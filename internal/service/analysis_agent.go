package service

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"skillhub/internal/models"
)

// AnalysisAgent 恶意文件分析智能体
type AnalysisAgent struct {
	llmService   *LLMToolService
	toolRegistry *ToolRegistry
	mcpAdapter   *MCPToolAdapter
	apiKey       string
	provider     string
	model        string
}

// AnalysisAgentConfig 智能体配置
type AnalysisAgentConfig struct {
	APIKey   string
	Provider string // "anthropic", "openai", "deepseek", "glm"
	Model    string
	MCPURL   string
}

// NewAnalysisAgent 创建分析智能体
func NewAnalysisAgent(config *AnalysisAgentConfig) *AnalysisAgent {
	// 创建工具注册中心
	toolRegistry := NewToolRegistry()

	// 创建 MCP 客户端和适配器
	mcpClient := NewMCPClient(config.MCPURL)
	mcpAdapter := NewMCPToolAdapter(mcpClient)

	// 注册 MCP 工具
	for _, toolDef := range mcpAdapter.GetToolDefinitions() {
		executor := NewMCPToolExecutor(mcpAdapter, toolDef.Function.Name, toolDef)
		toolRegistry.Register(executor)
	}

	// 创建 LLM 服务
	llmService := NewLLMToolService(toolRegistry)

	return &AnalysisAgent{
		llmService:   llmService,
		toolRegistry: toolRegistry,
		mcpAdapter:   mcpAdapter,
		apiKey:       config.APIKey,
		provider:     config.Provider,
		model:        config.Model,
	}
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	Summary     string         // 分析摘要
	ThreatLevel string         // 威胁等级: low, medium, high, critical
	Indicators  []string       // 威胁指标
	ToolCalls   []ToolCallInfo // 工具调用记录
	Duration    time.Duration  // 分析耗时
	RawReport   string         // 原始报告内容
}

// Analyze 分析文件
func (a *AnalysisAgent) Analyze(ctx context.Context, filePath, fileName string) (*AnalysisResult, error) {
	startTime := time.Now()

	// 首先设置文件路径到 MCP
	// 通过调用 analyze_binary 来初始化 IDA 分析
	_, err := a.mcpAdapter.Execute("analyze_binary", map[string]interface{}{
		"file_path": filePath,
	})
	if err != nil {
		fmt.Printf("[Agent] Failed to initialize MCP analysis: %v\n", err)
		// 继续尝试，LLM 可能会重试
	}

	// 构建系统提示
	systemPrompt := a.buildSystemPrompt(fileName)

	// 构建用户消息
	userMessage := fmt.Sprintf(`请分析这个可疑文件: %s

文件路径: %s

请按照以下步骤进行分析:
1. 首先调用 analyze_binary 获取文件基本信息
2. 获取函数列表和字符串
3. 分析导入导出表
4. 识别可疑的代码模式和行为
5. 生成详细的恶意软件分析报告

请使用可用的工具进行全面分析，然后给出你的专业判断。`, fileName, filePath)

	// 获取可用工具
	tools := a.toolRegistry.GetAllTools()

	// 调用 LLM with Tools
	req := &ChatWithToolsRequest{
		Provider:     a.provider,
		APIKey:       a.apiKey,
		Model:        a.model,
		SystemPrompt: systemPrompt,
		Messages: []models.ChatMessage{
			{Role: "user", Content: userMessage},
		},
		Tools:    tools,
		MaxTurns: 15,
	}

	resp, err := a.llmService.ChatWithTools(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM 分析失败: %w", err)
	}

	// 构建结果
	result := &AnalysisResult{
		RawReport: resp.Content,
		ToolCalls: resp.ToolCalls,
		Duration:  time.Since(startTime),
	}

	// 解析威胁等级和指标
	a.parseAnalysisResult(result, resp.Content)

	fmt.Printf("[Agent] Analysis completed in %v with %d tool calls\n", result.Duration, len(result.ToolCalls))

	return result, nil
}

// buildSystemPrompt 构建系统提示
func (a *AnalysisAgent) buildSystemPrompt(fileName string) string {
	return `你是一位专业的恶意软件分析师，拥有丰富的逆向工程和安全分析经验。

你的任务是分析可疑的二进制文件，识别潜在的恶意行为和威胁。

## 分析方法

你应该使用以下工具进行全面分析：

1. **analyze_binary** - 首先调用，获取文件基本信息
2. **get_functions** - 获取函数列表
3. **get_strings** - 提取字符串，寻找可疑 URL、IP、路径
4. **get_imports** - 分析导入的 API，识别危险函数
5. **get_exports** - 查看导出函数
6. **get_segments** - 检查节区权限和特征
7. **decompile_function** - 反编译可疑函数
8. **get_xrefs** - 追踪关键函数的调用

## 分析重点

请特别关注：

- **可疑 API 调用**: 进程注入、注册表操作、网络通信、文件操作
- **字符串特征**: URL、IP 地址、加密密钥、命令字符串
- **代码模式**: 反调试、反虚拟机、加密/解密例程
- **节区异常**: 可写+可执行节区、高熵值(可能加壳)

## 报告格式

请按以下格式输出分析报告：

### 文件概述
- 文件名、类型、架构、大小
- 编译时间(如有)

### 威胁评估
- **威胁等级**: [低/中/高/严重]
- **置信度**: [低/中/高]

### 行为分析
- 主要恶意行为
- 攻击向量
- 持久化机制(如有)

### 技术指标
- 可疑函数列表
- 字符串指标
- 网络指标(C2 地址等)

### 结论与建议
- 总结判断
- 缓解建议

请用中文输出报告。`
}

// parseAnalysisResult 解析分析结果
func (a *AnalysisAgent) parseAnalysisResult(result *AnalysisResult, content string) {
	// 提取威胁等级
	contentLower := strings.ToLower(content)

	if strings.Contains(contentLower, "威胁等级") || strings.Contains(contentLower, "严重") {
		if strings.Contains(contentLower, "严重") || strings.Contains(contentLower, "critical") {
			result.ThreatLevel = "critical"
		} else if strings.Contains(contentLower, "高") || strings.Contains(contentLower, "high") {
			result.ThreatLevel = "high"
		} else if strings.Contains(contentLower, "中") || strings.Contains(contentLower, "medium") {
			result.ThreatLevel = "medium"
		} else {
			result.ThreatLevel = "low"
		}
	} else {
		result.ThreatLevel = "unknown"
	}

	// 提取摘要 (取前 500 字符)
	if len(content) > 500 {
		result.Summary = content[:500] + "..."
	} else {
		result.Summary = content
	}

	// 提取指标 (简单实现，可以通过更复杂的解析改进)
	result.Indicators = a.extractIndicators(content)
}

// extractIndicators 提取威胁指标
func (a *AnalysisAgent) extractIndicators(content string) []string {
	var indicators []string

	// 提取 IP 地址、URL、文件路径等
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "http://") ||
			strings.Contains(line, "https://") ||
			strings.Contains(line, "\\") && strings.Contains(line, ".exe") ||
			strings.Contains(line, "HKEY_") ||
			strings.Contains(line, "CreateRemoteThread") ||
			strings.Contains(line, "VirtualAlloc") ||
			strings.Contains(line, "WriteProcessMemory") {
			indicators = append(indicators, line)
		}
	}

	// 限制数量
	if len(indicators) > 20 {
		indicators = indicators[:20]
	}

	return indicators
}

// GenerateReport 生成 Markdown 报告
func (a *AnalysisAgent) GenerateReport(fileName string, result *AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString("# 恶意文件分析报告\n\n")
	sb.WriteString(fmt.Sprintf("**生成时间**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**文件名**: %s\n", fileName))
	sb.WriteString(fmt.Sprintf("**分析耗时**: %v\n\n", result.Duration))

	// 威胁等级
	threatLevelMap := map[string]string{
		"low":      "🟢 低",
		"medium":   "🟡 中",
		"high":     "🟠 高",
		"critical": "🔴 严重",
		"unknown":  "⚪ 未知",
	}
	sb.WriteString(fmt.Sprintf("**威胁等级**: %s\n\n", threatLevelMap[result.ThreatLevel]))

	// 工具调用统计
	sb.WriteString(fmt.Sprintf("**使用工具**: %d 次调用\n\n", len(result.ToolCalls)))

	// 威胁指标
	if len(result.Indicators) > 0 {
		sb.WriteString("## 关键指标\n\n")
		for _, indicator := range result.Indicators {
			sb.WriteString(fmt.Sprintf("- %s\n", indicator))
		}
		sb.WriteString("\n")
	}

	// 详细分析
	sb.WriteString("## 详细分析\n\n")
	sb.WriteString(result.RawReport)

	// 工具调用记录
	if len(result.ToolCalls) > 0 {
		sb.WriteString("\n\n---\n\n")
		sb.WriteString("## 工具调用记录\n\n")
		sb.WriteString("| 工具 | 参数 | 结果 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, call := range result.ToolCalls {
			argsStr := fmt.Sprintf("%v", call.Arguments)
			if len(argsStr) > 50 {
				argsStr = argsStr[:50] + "..."
			}
			resultStr := "成功"
			if call.Error != "" {
				resultStr = "失败: " + call.Error
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", call.Name, argsStr, resultStr))
		}
	}

	sb.WriteString("\n\n---\n\n")
	sb.WriteString("*报告由 SkillHub 恶意文件分析系统 (LLM Agent) 生成*\n")

	return sb.String()
}
