package service

import (
	"context"
	"errors"
	"fmt"
	"skillhub/internal/models"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// LLMService LLM 调用服务
type LLMService struct {
	openAIClients sync.Map // 缓存 OpenAI 兼容客户端
}

// NewLLMService 创建 LLM 服务
func NewLLMService() *LLMService {
	return &LLMService{}
}

// ChatChunk 流式响应块
type ChatChunk struct {
	Content string
	Done    bool
	Error   error
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Provider       string
	APIKey         string
	Model          string
	SystemPrompt   string
	Messages       []models.ChatMessage
	Temperature    float64
	MaxTokens      int
	CustomEndpoint string // 仅 openai-compatible 使用
}

// StreamChat 流式聊天
func (s *LLMService) StreamChat(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error) {
	switch req.Provider {
	case "anthropic", "openai", "deepseek", "glm", "openai-compatible":
		return s.streamOpenAI(ctx, req)
	default:
		return nil, errors.New("unsupported provider: " + req.Provider)
	}
}

// ValidateKey 验证 API Key
func (s *LLMService) ValidateKey(ctx context.Context, provider, apiKey, customEndpoint string) error {
	return s.validateOpenAIKey(ctx, provider, apiKey, customEndpoint)
}

// ============== OpenAI Compatible (支持 Anthropic/OpenAI/DeepSeek) ==============

func (s *LLMService) getOpenAIClient(provider, apiKey, customEndpoint string) *openai.Client {
	key := provider + ":" + apiKey + ":" + customEndpoint
	if client, ok := s.openAIClients.Load(key); ok {
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
	s.openAIClients.Store(key, client)
	return client
}

func (s *LLMService) validateOpenAIKey(ctx context.Context, provider, apiKey, customEndpoint string) error {
	client := s.getOpenAIClient(provider, apiKey, customEndpoint)

	// 发送一个简单的测试请求
	model := "gpt-3.5-turbo"
	if provider == "anthropic" {
		model = "claude-3-haiku-20240307"
	} else if provider == "deepseek" {
		model = "deepseek-chat"
	} else if provider == "glm" {
		model = "glm-4-flash"
	}

	_, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     model,
		MaxTokens: 10,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Hi"},
		},
	})
	return err
}

func (s *LLMService) streamOpenAI(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error) {
	client := s.getOpenAIClient(req.Provider, req.APIKey, req.CustomEndpoint)

	// 构建消息
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages)+1)

	// 添加系统提示
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
		default:
			continue
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// 确定模型
	model := req.Model

	// 检查模型是否与 provider 匹配，如果不匹配则使用 provider 的默认模型
	if model != "" && !s.isModelCompatibleWithProvider(model, req.Provider) {
		// 日志：模型不兼容，将使用默认模型
		fmt.Printf("[LLM] Model %q incompatible with provider %q, will use default model\n", model, req.Provider)
		model = "" // 重置为空，使用默认模型
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
		fmt.Printf("[LLM] Using default model for provider %q: %s\n", req.Provider, model)
	}

	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	})
	if err != nil {
		fmt.Printf("[LLM] Stream creation failed: %v\n", err)
		return nil, err
	}
	fmt.Printf("[LLM] Stream created successfully, waiting for response...\n")

	ch := make(chan ChatChunk, 100)

	go func() {
		defer close(ch)
		defer stream.Close()

		chunkCount := 0
		for {
			response, err := stream.Recv()
			if err != nil {
				fmt.Printf("[LLM] Stream recv error: %v\n", err)
				if err.Error() == "EOF" || strings.Contains(err.Error(), "stream ended") {
					fmt.Printf("[LLM] Stream ended normally, total chunks: %d\n", chunkCount)
					ch <- ChatChunk{Done: true}
					return
				}
				ch <- ChatChunk{Error: err}
				return
			}

			chunkCount++
			if len(response.Choices) > 0 {
				delta := response.Choices[0].Delta
				if delta.Content != "" {
					ch <- ChatChunk{Content: delta.Content}
				}
				if response.Choices[0].FinishReason == "stop" {
					fmt.Printf("[LLM] Stream finished with stop reason, total chunks: %d\n", chunkCount)
					ch <- ChatChunk{Done: true}
					return
				}
			}
		}
	}()

	return ch, nil
}

// GetDefaultModel 获取 Provider 的默认模型
func (s *LLMService) GetDefaultModel(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-3-opus-20240229"
	case "openai":
		return "gpt-4o"
	case "deepseek":
		return "deepseek-chat"
	case "glm":
		return "glm-4-flash"
	default:
		return "gpt-4o"
	}
}

// GetSupportedModels 获取 Provider 支持的模型列表
func (s *LLMService) GetSupportedModels(provider string) []string {
	switch provider {
	case "anthropic":
		return []string{
			"claude-3-opus-20240229",
			"claude-3-sonnet-20240229",
			"claude-3-haiku-20240307",
			"claude-3-5-sonnet-20241022",
		}
	case "openai":
		return []string{
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
			"o1-preview",
			"o1-mini",
		}
	case "deepseek":
		return []string{
			"deepseek-chat",
			"deepseek-coder",
		}
	case "glm":
		return []string{
			"glm-4-flash",
			"glm-4",
			"glm-4-plus",
			"glm-4-air",
			"glm-4-airx",
			"glm-4-long",
			"glm-4v-flash",
			"glm-4v",
		}
	default:
		return []string{"gpt-4o"}
	}
}

// StreamChatWithTimeout 带超时的流式聊天
func (s *LLMService) StreamChatWithTimeout(ctx context.Context, req *ChatRequest, timeout time.Duration) (<-chan ChatChunk, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return s.StreamChat(ctx, req)
}

// isModelCompatibleWithProvider 检查模型是否与 provider 兼容
func (s *LLMService) isModelCompatibleWithProvider(model, provider string) bool {
	if model == "" {
		return true
	}

	switch provider {
	case "anthropic":
		return strings.HasPrefix(model, "claude-")
	case "openai":
		return strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1-")
	case "deepseek":
		return strings.HasPrefix(model, "deepseek-")
	case "glm":
		return strings.HasPrefix(model, "glm-")
	default:
		return true // 未知 provider 允许任意模型
	}
}
