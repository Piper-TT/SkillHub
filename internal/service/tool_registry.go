package service

import (
	"encoding/json"
	"fmt"
)

// ToolDefinition 工具定义 (OpenAI 兼容格式)
type ToolDefinition struct {
	Type     string            `json:"type"` // "function"
	Function *FunctionDef      `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
}

// ExecuteToolResult 执行工具的详细结果
type ExecuteToolResult struct {
	Result string
	Error  string
}

// ToolExecutor 工具执行器接口
type ToolExecutor interface {
	// Name 工具名称
	Name() string

	// Definition 返回工具定义
	Definition() *ToolDefinition

	// Execute 执行工具
	Execute(args map[string]interface{}) (string, error)
}

// ToolRegistry 工具注册中心
type ToolRegistry struct {
	tools map[string]ToolExecutor
}

// NewToolRegistry 创建工具注册中心
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolExecutor),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(executor ToolExecutor) {
	r.tools[executor.Name()] = executor
}

// GetTool 获取工具
func (r *ToolRegistry) GetTool(name string) (ToolExecutor, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// GetAllTools 获取所有工具定义
func (r *ToolRegistry) GetAllTools() []*ToolDefinition {
	definitions := make([]*ToolDefinition, 0, len(r.tools))
	for _, executor := range r.tools {
		definitions = append(definitions, executor.Definition())
	}
	return definitions
}

// ExecuteTool 执行工具
func (r *ToolRegistry) ExecuteTool(name string, args map[string]interface{}) *ExecuteToolResult {
	result := &ExecuteToolResult{}

	executor, ok := r.tools[name]
	if !ok {
		result.Error = fmt.Sprintf("工具 %s 不存在", name)
		return result
	}

	output, err := executor.Execute(args)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Result = output
	return result
}

// ExecuteToolCalls 批量执行工具调用
func (r *ToolRegistry) ExecuteToolCalls(calls []ToolCall) []*ToolResult {
	results := make([]*ToolResult, 0, len(calls))
	for _, call := range calls {
		var args map[string]interface{}
		if call.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				results = append(results, &ToolResult{
					ToolCallID: call.ID,
					Name:       call.Name,
					Error:      fmt.Sprintf("解析参数失败: %v", err),
				})
				continue
			}
		}

		execResult := r.ExecuteTool(call.Name, args)
		results = append(results, &ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Result:     execResult.Result,
			Error:      execResult.Error,
		})
	}
	return results
}

// ToolCall LLM 返回的工具调用
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
