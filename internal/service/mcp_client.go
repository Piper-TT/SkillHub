package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// MCPClient MCP 客户端 - 支持启动和管理 idalib-mcp 进程
type MCPClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex

	// 进程管理
	cmd         *exec.Cmd
	processFile string // 当前分析的文件
	sessionID   string // MCP Session ID
}

// NewMCPClient 创建 MCP 客户端
func NewMCPClient(baseURL string) *MCPClient {
	return &MCPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // 5 分钟超时
		},
	}
}

// MCPToolResult MCP 工具调用结果
type MCPToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// StartServer 启动 MCP 服务器
func (c *MCPClient) StartServer(idalibPath, targetFile string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已经有进程在运行，先停止
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd = nil
	}

	fmt.Printf("[MCP] Starting server for: %s\n", targetFile)

	// 启动 idalib-mcp.exe
	cmd := exec.Command(idalibPath,
		"--isolated-contexts",
		"--host", "127.0.0.1",
		"--port", "8745",
		targetFile,
	)

	// 设置输出
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 MCP 服务器失败: %w", err)
	}

	c.cmd = cmd
	c.processFile = targetFile
	c.sessionID = "" // 重置 session

	// 等待服务器就绪并初始化
	fmt.Printf("[MCP] Waiting for server to be ready (timeout: 10m)...\n")
	if err := c.waitForReadyAndInit(10 * time.Minute); err != nil {
		c.StopServer()
		return fmt.Errorf("MCP 服务器初始化失败: %w", err)
	}

	fmt.Printf("[MCP] Server ready with session: %s\n", c.sessionID)
	return nil
}

// StopServer 停止 MCP 服务器
func (c *MCPClient) StopServer() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && c.cmd.Process != nil {
		fmt.Printf("[MCP] Stopping server...\n")
		c.cmd.Process.Kill()
		c.cmd.Wait()
		c.cmd = nil
		c.processFile = ""
		c.sessionID = ""
	}
}

// waitForReadyAndInit 等待服务器就绪并初始化
func (c *MCPClient) waitForReadyAndInit(timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		// 尝试初始化连接
		reqBody := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"clientInfo": map[string]interface{}{
					"name":    "skillhub",
					"version": "1.0",
				},
				"capabilities": map[string]interface{}{},
			},
		}

		body, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// 获取 Session ID
		sessionID := resp.Header.Get("Mcp-Session-Id")
		resp.Body.Close()

		if resp.StatusCode == 200 && sessionID != "" {
			c.sessionID = sessionID
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for MCP server")
}

// CallTool 调用 MCP 工具
func (c *MCPClient) CallTool(toolName string, args map[string]interface{}) (*MCPToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionID == "" {
		return nil, fmt.Errorf("MCP session not initialized")
	}

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", c.sessionID) // 添加 Session ID

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析 JSON-RPC 响应
	var rpcResp struct {
		Result *MCPToolResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w, body: %s", err, string(respBody))
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// ListTools 列出可用工具
func (c *MCPClient) ListTools() ([]map[string]interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionID == "" {
		return nil, fmt.Errorf("MCP session not initialized")
	}

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", c.sessionID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp struct {
		Result struct {
			Tools []map[string]interface{} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", rpcResp.Error.Message)
	}

	return rpcResp.Result.Tools, nil
}

// AnalyzeBinary 分析二进制文件
func (c *MCPClient) AnalyzeBinary() (string, error) {
	result, err := c.CallTool("analyze_binary", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if result.IsError {
		return "", fmt.Errorf("analysis failed")
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", nil
}

// GetFunctions 获取函数列表
func (c *MCPClient) GetFunctions() (string, error) {
	result, err := c.CallTool("get_functions", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", nil
}

// GetStrings 获取字符串列表
func (c *MCPClient) GetStrings() (string, error) {
	result, err := c.CallTool("get_strings", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", nil
}

// GetImports 获取导入表
func (c *MCPClient) GetImports() (string, error) {
	result, err := c.CallTool("get_imports", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", nil
}

// GetExports 获取导出表
func (c *MCPClient) GetExports() (string, error) {
	result, err := c.CallTool("get_exports", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", nil
}

// GetSegments 获取节区信息
func (c *MCPClient) GetSegments() (string, error) {
	result, err := c.CallTool("get_segments", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", nil
}

// GetMetadata 获取文件元数据
func (c *MCPClient) GetMetadata() (string, error) {
	result, err := c.CallTool("get_metadata", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", nil
}

// DecompileFunction 反编译函数
func (c *MCPClient) DecompileFunction(funcName string) (string, error) {
	result, err := c.CallTool("decompile_function", map[string]interface{}{
		"function_name": funcName,
	})
	if err != nil {
		return "", err
	}
	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", nil
}

// GetXRefs 获取交叉引用
func (c *MCPClient) GetXRefs(target string) (string, error) {
	result, err := c.CallTool("get_xrefs", map[string]interface{}{
		"target": target,
	})
	if err != nil {
		return "", err
	}
	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", nil
}

// IsRunning 检查服务器是否在运行
func (c *MCPClient) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cmd != nil && c.cmd.Process != nil
}
