package handlers

import (
	"io"
	"net/http"
	"skillhub/internal/models"
	"skillhub/internal/service"
	"skillhub/internal/utils"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// AgentHandler 智能体处理器
type AgentHandler struct {
	agentSvc *service.AgentService
	llmSvc   *service.LLMService
}

// NewAgentHandler 创建处理器
func NewAgentHandler(agentSvc *service.AgentService, llmSvc *service.LLMService) *AgentHandler {
	return &AgentHandler{
		agentSvc: agentSvc,
		llmSvc:   llmSvc,
	}
}

// GetAgents 获取智能体列表
func (h *AgentHandler) GetAgents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "12"))
	if perPage > 100 {
		perPage = 100
	}

	category := c.Query("category")
	search := c.Query("search")

	result, err := h.agentSvc.GetAgents(page, perPage, category, search)
	if err != nil {
		utils.InternalError(c, "Failed to get agents")
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetAgentByID 获取智能体详情
func (h *AgentHandler) GetAgentByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}

	agent, err := h.agentSvc.GetAgentByID(uint(id))
	if err != nil {
		utils.NotFound(c, "agent not found")
		return
	}

	c.JSON(http.StatusOK, agent)
}

// GetCategories 获取分类列表
func (h *AgentHandler) GetCategories(c *gin.Context) {
	categories := h.agentSvc.GetCategories()
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// GetStats 获取统计信息
func (h *AgentHandler) GetStats(c *gin.Context) {
	stats := h.agentSvc.GetStats()
	c.JSON(http.StatusOK, stats)
}

// Chat 与智能体聊天 (SSE 流式响应)
func (h *AgentHandler) Chat(c *gin.Context) {
	agentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid agent id")
		return
	}

	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// 获取用户 ID (从 localStorage 传入的 header)
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.BadRequest(c, "X-User-ID header is required")
		return
	}

	// 获取智能体
	agent, err := h.agentSvc.GetAgentByID(uint(agentID))
	if err != nil {
		utils.NotFound(c, "agent not found")
		return
	}

	// 获取用户的 API Key - 按优先级尝试所有支持的 provider
	var apiKey string
	var provider string
	providers := []string{"anthropic", "openai", "deepseek", "glm"}

	for _, p := range providers {
		key, err := h.agentSvc.GetDecryptedAPIKey(userID, p)
		if err == nil && key != "" {
			apiKey = key
			provider = p
			break
		}
	}

	if apiKey == "" {
		utils.BadRequest(c, "请先配置 LLM API Key")
		return
	}

	// 获取或创建会话
	session, err := h.agentSvc.GetOrCreateSession(userID, uint(agentID), req.SessionID)
	if err != nil {
		utils.InternalError(c, "Failed to create session")
		return
	}

	// 添加用户消息到会话
	if err := h.agentSvc.AddMessageToSession(session, "user", req.Message); err != nil {
		utils.InternalError(c, "Failed to save message")
		return
	}

	// 获取会话消息
	messages, err := h.agentSvc.GetSessionMessages(session)
	if err != nil {
		utils.InternalError(c, "Failed to get messages")
		return
	}

	// 构建 LLM 请求
	llmReq := &service.ChatRequest{
		Provider:     provider,
		APIKey:       apiKey,
		Model:        agent.Model,
		SystemPrompt: agent.SystemPrompt,
		Messages:     messages,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
	}

	// 流式调用 LLM
	stream, err := h.llmSvc.StreamChat(c.Request.Context(), llmReq)
	if err != nil {
		utils.InternalError(c, "LLM error: "+err.Error())
		return
	}

	// 设置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

	// 收集完整响应用于保存
	var fullResponse string

	// 流式响应
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		utils.InternalError(c, "Streaming not supported")
		return
	}

	for chunk := range stream {
		if chunk.Error != nil {
			c.SSEvent("error", gin.H{"message": chunk.Error.Error()})
			flusher.Flush()
			return
		}

		if chunk.Done {
			// 保存助手响应到会话
			if fullResponse != "" {
				h.agentSvc.AddMessageToSession(session, "assistant", fullResponse)
				h.agentSvc.IncrementAgentUsage(uint(agentID))
			}
			c.SSEvent("done", gin.H{
				"session_id": session.ID,
				"message":    "[DONE]",
			})
			flusher.Flush()
			return
		}

		fullResponse += chunk.Content
		c.SSEvent("message", gin.H{"content": chunk.Content})
		flusher.Flush()
	}
}

// GetSessions 获取用户的会话列表
func (h *AgentHandler) GetSessions(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.BadRequest(c, "X-User-ID header is required")
		return
	}

	agentID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	sessions, err := h.agentSvc.GetUserSessions(userID, uint(agentID), limit)
	if err != nil {
		utils.InternalError(c, "Failed to get sessions")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetSession 获取会话详情
func (h *AgentHandler) GetSession(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.BadRequest(c, "X-User-ID header is required")
		return
	}

	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid session id")
		return
	}

	session, err := h.agentSvc.GetOrCreateSession(userID, 0, uint(sessionID))
	if err != nil {
		utils.NotFound(c, "session not found")
		return
	}

	if session.UserID != userID {
		utils.NotFound(c, "session not found")
		return
	}

	messages, err := h.agentSvc.GetSessionMessages(session)
	if err != nil {
		utils.InternalError(c, "Failed to get messages")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session":  session,
		"messages": messages,
	})
}

// DeleteSession 删除会话
func (h *AgentHandler) DeleteSession(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.BadRequest(c, "X-User-ID header is required")
		return
	}

	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid session id")
		return
	}

	if err := h.agentSvc.DeleteSession(userID, uint(sessionID)); err != nil {
		utils.NotFound(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "session deleted"})
}

// ============== API Key 管理 ==============

// SaveAPIKey 保存 API Key
func (h *AgentHandler) SaveAPIKey(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.BadRequest(c, "X-User-ID header is required")
		return
	}

	var req models.APIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// 验证 API Key
	if err := h.llmSvc.ValidateKey(c.Request.Context(), req.Provider, req.APIKey, req.CustomEndpoint); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "API Key 验证失败: " + err.Error(),
		})
		return
	}

	// 保存 API Key
	if err := h.agentSvc.SaveAPIKey(userID, &req); err != nil {
		utils.InternalError(c, "Failed to save API key")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API Key 保存成功",
	})
}

// ValidateAPIKey 验证 API Key
func (h *AgentHandler) ValidateAPIKey(c *gin.Context) {
	var req models.ValidateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	err := h.llmSvc.ValidateKey(c.Request.Context(), req.Provider, req.APIKey, req.CustomEndpoint)

	c.JSON(http.StatusOK, models.ValidateKeyResponse{
		Valid:   err == nil,
		Message: errorMessage(err),
	})
}

// GetAPIKeys 获取用户已配置的 API Key 列表
func (h *AgentHandler) GetAPIKeys(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.BadRequest(c, "X-User-ID header is required")
		return
	}

	keys, err := h.agentSvc.GetAPIKeys(userID)
	if err != nil {
		utils.InternalError(c, "Failed to get API keys")
		return
	}

	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// DeleteAPIKey 删除 API Key
func (h *AgentHandler) DeleteAPIKey(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.BadRequest(c, "X-User-ID header is required")
		return
	}

	provider := c.Param("provider")
	if provider == "" {
		utils.BadRequest(c, "provider is required")
		return
	}

	if err := h.agentSvc.DeleteAPIKey(userID, provider); err != nil {
		utils.NotFound(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "API key deleted"})
}

// GetModels 获取支持的模型列表
func (h *AgentHandler) GetModels(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		provider = "anthropic"
	}

	models := h.llmSvc.GetSupportedModels(provider)
	c.JSON(http.StatusOK, gin.H{
		"provider": provider,
		"models":   models,
		"default":  h.llmSvc.GetDefaultModel(provider),
	})
}

// ============== 辅助函数 ==============

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ============== SSE 客户端 (用于测试) ==============

// StreamTest 测试 SSE 流式响应
func (h *AgentHandler) StreamTest(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		utils.InternalError(c, "Streaming not supported")
		return
	}

	messages := []string{"Hello", " from", " AgentHub", "!", " This", " is", " a", " test", " stream."}

	for _, msg := range messages {
		c.SSEvent("message", gin.H{"content": msg})
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)
	}

	c.SSEvent("done", gin.H{"message": "[DONE]"})
	flusher.Flush()

	// 等待客户端断开
	<-c.Request.Context().Done()
}

// 确保实现 io.Reader 接口
var _ io.Reader = (*byteReader)(nil)
