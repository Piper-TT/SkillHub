package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// MCPClient MCP 客户端
type MCPClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
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

// CallTool 调用 MCP 工具
func (c *MCPClient) CallTool(toolName string, args map[string]interface{}) (*MCPToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

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

// AnalyzeBinary 使用 IDA Pro 分析二进制文件
func (c *MCPClient) AnalyzeBinary(filePath string) (string, error) {
	result, err := c.CallTool("analyze_binary", map[string]interface{}{
		"file_path": filePath,
	})
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
