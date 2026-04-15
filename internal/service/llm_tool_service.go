package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"skillhub/internal/models"

	"github.com/sashabaranov/go-openai"
)

// LLMToolService 支持 Tool Call 的 LLM 服务
type LLMToolService struct {
	*LLMService
	toolRegistry *ToolRegistry
	clients      sync.Map // OpenAI 客户端缓存
}

// NewLLMToolService 创建支持工具调用的 LLM 服务
func NewLLMToolService(toolRegistry *ToolRegistry) *LLMToolService {
	return &LLMToolService{
		LLMService:   NewLLMService(),
		toolRegistry: toolRegistry,
	}
}

// getOpenAIClient 获取 OpenAI 客户端
func (s *LLMToolService) getOpenAIClient(provider, apiKey, customEndpoint string) *openai.Client {
	key := provider + ":" + apiKey + ":" + customEndpoint
	if client, ok := s.clients.Load(key); ok {
		return client.(*openai.Client)
	}

	config := openai.DefaultConfig(apiKey)

	switch provider {
	case "anthropic":
		config.BaseURL = "https://api.anthropic.com/v1"
	case "deepseek":
		config.BaseURL = "https://api.deepseek.com/v1"
	case "glm":
		config.BaseURL = "https://open.bigmodel.cn/api/paas/v4"
	case "openai-compatible":
		if customEndpoint != "" {
			config.BaseURL = customEndpoint
		}
	}

	client := openai.NewClientWithConfig(config)
	s.clients.Store(key, client)
	return client
}

// ChatWithToolsRequest 带工具的聊天请求
type ChatWithToolsRequest struct {
	Provider     string
	APIKey       string
	Model        string
	SystemPrompt string
	Messages     []models.ChatMessage
	Tools        []*ToolDefinition // 可用工具
	MaxTurns     int               // 最大工具调用轮数
}

// ChatWithToolsResponse 带工具的聊天响应
type ChatWithToolsResponse struct {
	Content    string         // 最终回复内容
	ToolCalls  []ToolCallInfo // 工具调用记录
	TokensUsed int            // 消耗的 token
	Duration   time.Duration  // 总耗时
}

// ToolCallInfo 工具调用信息
type ToolCallInfo struct {
	Name      string
	Arguments map[string]interface{}
	Result    string
	Error     string
}

// ChatWithTools 执行带工具调用的聊天
func (s *LLMToolService) ChatWithTools(ctx context.Context, req *ChatWithToolsRequest) (*ChatWithToolsResponse, error) {
	startTime := time.Now()
	response := &ChatWithToolsResponse{
		ToolCalls: []ToolCallInfo{},
	}

	client := s.getOpenAIClient(req.Provider, req.APIKey, "")

	// 构建消息
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		var role string
		switch msg.Role {
		case "user":
			role = openai.ChatMessageRoleUser
		case "assistant":
			role = openai.ChatMessageRoleAssistant
		case "tool":
			role = openai.ChatMessageRoleTool
		default:
			continue
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// 转换工具定义
	var tools []openai.Tool
	for _, td := range req.Tools {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        td.Function.Name,
				Description: td.Function.Description,
				Parameters:  td.Function.Parameters,
			},
		})
	}

	// 确定模型（兼容性检查）
	model := req.Model
	if model != "" && !s.isModelCompatibleWithProvider(model, req.Provider) {
		fmt.Printf("[LLM ToolCall] Model %q incompatible with provider %q, using default\n", model, req.Provider)
		model = ""
	}
	if model == "" {
		switch req.Provider {
		case "anthropic":
			model = "claude-3-opus-20240229"
		case "deepseek":
			model = "deepseek-chat"
		case "glm":
			model = "glm-4-flash"
		default:
			model = "gpt-4o"
		}
		fmt.Printf("[LLM ToolCall] Using model: %s\n", model)
	}

	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 10 // 默认最多 10 轮工具调用
	}

	// 开始多轮对话
	for turn := 0; turn < maxTurns; turn++ {
		fmt.Printf("[LLM] Turn %d: sending request with %d messages\n", turn+1, len(messages))

		// 创建请求
		chatReq := openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
			Stream:   false,
		}
		if len(tools) > 0 {
			chatReq.Tools = tools
		}

		// 发送请求
		resp, err := client.CreateChatCompletion(ctx, chatReq)
		if err != nil {
			return nil, fmt.Errorf("LLM 请求失败: %w", err)
		}

		if len(resp.Choices) == 0 {
			return nil, errors.New("LLM 返回空响应")
		}

		choice := resp.Choices[0]
		response.TokensUsed += resp.Usage.TotalTokens

		// 检查是否有工具调用
		if len(choice.Message.ToolCalls) > 0 {
			// 添加助手消息到历史
			messages = append(messages, choice.Message)

			// 执行每个工具调用
			for _, toolCall := range choice.Message.ToolCalls {
				fmt.Printf("[LLM] Tool call: %s(%s)\n", toolCall.Function.Name, toolCall.Function.Arguments)

				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					args = make(map[string]interface{})
				}

				// 执行工具
				result := s.toolRegistry.ExecuteTool(toolCall.Function.Name, args)

				// 记录工具调用
				callInfo := ToolCallInfo{
					Name:      toolCall.Function.Name,
					Arguments: args,
				}
				if result.Error != "" {
					callInfo.Error = result.Error
				} else {
					callInfo.Result = result.Result
				}
				response.ToolCalls = append(response.ToolCalls, callInfo)

				// 添加工具结果到消息历史
				toolResultContent := result.Result
				if result.Error != "" {
					toolResultContent = "Error: " + result.Error
				}
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    toolResultContent,
					ToolCallID: toolCall.ID,
				})
			}

			// 继续下一轮
			continue
		}

		// 没有工具调用，返回最终结果
		response.Content = choice.Message.Content
		response.Duration = time.Since(startTime)
		fmt.Printf("[LLM] Completed after %d turns, %d tool calls, %v\n", turn+1, len(response.ToolCalls), response.Duration)
		return response, nil
	}

	return nil, errors.New("exceeded maximum tool call turns")
}

// StreamChatWithTools 流式带工具的聊天 (简化版，工具调用时不流式)
func (s *LLMToolService) StreamChatWithTools(ctx context.Context, req *ChatWithToolsRequest) (<-chan ChatChunk, error) {
	ch := make(chan ChatChunk, 100)

	go func() {
		defer close(ch)

		// 先执行完整的工具调用流程
		resp, err := s.ChatWithTools(ctx, req)
		if err != nil {
			ch <- ChatChunk{Error: err}
			return
		}

		// 流式发送最终结果
		if resp.Content != "" {
			// 模拟流式输出
			words := strings.Fields(resp.Content)
			for i, word := range words {
				select {
				case ch <- ChatChunk{Content: word + " "}:
				case <-ctx.Done():
					return
				}
				// 添加小延迟使流式效果更自然
				if i < len(words)-1 {
					time.Sleep(20 * time.Millisecond)
				}
			}
		}

		ch <- ChatChunk{Done: true}
	}()

	return ch, nil
}
