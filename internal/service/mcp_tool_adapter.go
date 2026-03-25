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
func (a *MCPToolAdapter) getDefaultToolDefinitions() []*ToolDefinition {
	return []*ToolDefinition{
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "analyze_binary",
				Description: "分析二进制文件，返回基本文件信息、架构、入口点等元数据。这是分析的第一步，必须首先调用。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "要分析的二进制文件路径",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_functions",
				Description: "获取二进制文件中的所有函数列表，包括函数名、地址、大小等信息",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_strings",
				Description: "提取二进制文件中的所有可读字符串，用于发现可疑的 URL、IP、路径等信息",
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
				Description: "获取导入表，显示该程序依赖的外部 DLL 和 API 函数",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_exports",
				Description: "获取导出表，显示该程序对外提供的函数接口",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_segments",
				Description: "获取节区(段)信息，包括代码段、数据段等的地址、大小和权限",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_metadata",
				Description: "获取详细的文件元数据，包括编译时间、节区数量等",
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
				Description: "反编译指定函数，返回伪代码",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"function_name": map[string]interface{}{
							"type":        "string",
							"description": "要反编译的函数名",
						},
					},
					"required": []string{"function_name"},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_xrefs",
				Description: "获取指定地址或函数的交叉引用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"target": map[string]interface{}{
							"type":        "string",
							"description": "目标地址或函数名",
						},
					},
					"required": []string{"target"},
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
		// 文件已在启动 MCP 服务器时加载，无需再传递路径
		return a.client.AnalyzeBinary()

	case "get_functions":
		return a.client.GetFunctions()

	case "get_strings":
		return a.client.GetStrings()

	case "get_imports":
		return a.client.GetImports()

	case "get_exports":
		return a.client.GetExports()

	case "get_segments":
		return a.client.GetSegments()

	case "get_metadata":
		return a.client.GetMetadata()

	case "decompile_function":
		funcName, ok := args["function_name"].(string)
		if !ok {
			return "", fmt.Errorf("缺少 function_name 参数")
		}
		return a.client.DecompileFunction(funcName)

	case "get_xrefs":
		target, ok := args["target"].(string)
		if !ok {
			return "", fmt.Errorf("缺少 target 参数")
		}
		return a.client.GetXRefs(target)

	default:
		return "", fmt.Errorf("未知的 MCP 工具: %s", toolName)
	}
}

// ============== MCP Tool Executor (实现 ToolExecutor 接口) ==============

// MCPToolExecutor MCP 工具执行器
type MCPToolExecutor struct {
	adapter   *MCPToolAdapter
	toolName  string
	toolDef   *ToolDefinition
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
