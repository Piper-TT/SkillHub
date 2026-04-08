package service

import (
	"fmt"
)

// MCPToolAdapter MCP 工具适配器
// 将 MCP 服务的工具转换为 LLM 可调用的工具
type MCPToolAdapter struct {
	client   *MCPClient
	toolDefs []*ToolDefinition
}

// NewMCPToolAdapter 创建 MCP 工具适配器
func NewMCPToolAdapter(client *MCPClient) *MCPToolAdapter {
	adapter := &MCPToolAdapter{
		client: client,
	}

	// 初始化工具定义
	adapter.toolDefs = adapter.getDefaultToolDefinitions()

	return adapter
}

// getDefaultToolDefinitions 获取默认的 IDA-Pro-MCP 工具定义
// 对齐实际 MCP 工具的参数 schema
func (a *MCPToolAdapter) getDefaultToolDefinitions() []*ToolDefinition {
	return []*ToolDefinition{
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "analyze_binary",
				Description: "对已加载的二进制文件进行全面的初步分析（对应 MCP 的 survey_binary），返回文件元数据（架构、MD5、SHA256、入口点）、节区信息、导入函数分类（crypto/network/file_io/process/registry）、按交叉引用排序的 top 15 有趣字符串和函数、调用图摘要。这是分析的第一步，必须首先调用。",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_functions",
				Description: "获取二进制文件中的函数列表，包括函数名、地址、大小等信息。支持分页，默认返回前 200 个函数。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"offset": map[string]interface{}{
							"type":        "number",
							"description": "分页偏移量，默认 0",
						},
						"count": map[string]interface{}{
							"type":        "number",
							"description": "返回数量，默认 200",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_strings",
				Description: "提取二进制文件中的所有可读字符串（使用 find_regex 搜索），用于发现可疑的 URL、IP、路径、注册表键、命令行等 IOC 指标。",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_imports",
				Description: "获取导入表，显示该程序依赖的外部 DLL 和 API 函数。导入函数是判断恶意行为的关键（如 CreateRemoteThread、VirtualAllocEx、RegSetValueEx 等）。",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "decompile_function",
				Description: "反编译指定地址或函数名的函数，返回 Hex-Rays 伪代码。参数可以是地址（如 '0x401000'）或函数名（如 'sub_401000'、'main'）。用于深入分析可疑函数的具体逻辑。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"addr": map[string]interface{}{
							"type":        "string",
							"description": "函数地址（如 '0x401000'）或函数名（如 'sub_401000'、'main'）",
						},
					},
					"required": []string{"addr"},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_xrefs",
				Description: "获取指向指定地址或函数的交叉引用（xrefs_to），追踪谁调用了该函数或引用了该地址。用于追踪危险 API 的调用来源。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"addr": map[string]interface{}{
							"type":        "string",
							"description": "目标地址（如 '0x401000'）或函数名",
						},
					},
					"required": []string{"addr"},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "analyze_function",
				Description: "对指定函数进行综合分析（对应 MCP 的 analyze_function），返回伪代码、top 10 字符串、top 10 常量、调用者、被调用者、交叉引用、基本块摘要。比单独调用 decompile + xrefs 更高效。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"addr": map[string]interface{}{
							"type":        "string",
							"description": "函数地址（如 '0x401000'）或函数名",
						},
					},
					"required": []string{"addr"},
				},
			},
		},
	}
}

// GetToolDefinitions 获取所有工具定义
func (a *MCPToolAdapter) GetToolDefinitions() []*ToolDefinition {
	return a.toolDefs
}

// Execute 执行工具调用
func (a *MCPToolAdapter) Execute(toolName string, args map[string]interface{}) (string, error) {
	switch toolName {
	case "analyze_binary":
		return a.client.AnalyzeBinary()

	case "get_functions":
		offset := getIntArg(args, "offset", 0)
		count := getIntArg(args, "count", 200)
		return a.client.GetFunctions(offset, count)

	case "get_strings":
		return a.client.GetStrings()

	case "get_imports":
		return a.client.GetImports()

	case "decompile_function":
		addr, ok := args["addr"].(string)
		if !ok {
			// 兼容旧的 function_name 参数
			if fn, ok := args["function_name"].(string); ok {
				addr = fn
			} else {
				return "", fmt.Errorf("缺少 addr 参数")
			}
		}
		return a.client.DecompileFunction(addr)

	case "get_xrefs":
		addr, ok := args["addr"].(string)
		if !ok {
			// 兼容旧的 target 参数
			if t, ok := args["target"].(string); ok {
				addr = t
			} else {
				return "", fmt.Errorf("缺少 addr 参数")
			}
		}
		return a.client.GetXRefs(addr)

	case "analyze_function":
		addr, ok := args["addr"].(string)
		if !ok {
			return "", fmt.Errorf("缺少 addr 参数")
		}
		return a.client.AnalyzeFunction(addr)

	default:
		return "", fmt.Errorf("未知的 MCP 工具: %s", toolName)
	}
}

// getIntArg 从 args 中获取整数参数，支持 float64 (JSON 反序列化的默认类型)
func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return defaultVal
}

// ============== MCP Tool Executor (实现 ToolExecutor 接口) ==============

// MCPToolExecutor MCP 工具执行器
type MCPToolExecutor struct {
	adapter  *MCPToolAdapter
	toolName string
	toolDef  *ToolDefinition
}

// NewMCPToolExecutor 创建 MCP 工具执行器
func NewMCPToolExecutor(adapter *MCPToolAdapter, toolName string, toolDef *ToolDefinition) *MCPToolExecutor {
	return &MCPToolExecutor{
		adapter:  adapter,
		toolName: toolName,
		toolDef:  toolDef,
	}
}

// Name 返回工具名称
func (e *MCPToolExecutor) Name() string {
	return e.toolName
}

// Definition 返回工具定义
func (e *MCPToolExecutor) Definition() *ToolDefinition {
	return e.toolDef
}

// Execute 执行工具
func (e *MCPToolExecutor) Execute(args map[string]interface{}) (string, error) {
	return e.adapter.Execute(e.toolName, args)
}
