package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"skillhub/internal/models"
	"skillhub/internal/repository"
	"skillhub/internal/utils"
	"strings"
	"time"
)

// AgentService 智能体服务
type AgentService struct {
	agentRepo   repository.AgentRepository
	sessionRepo repository.SessionRepository
	apiKeyRepo  repository.APIKeyRepository
}

// NewAgentService 创建服务实例
func NewAgentService(
	agentRepo repository.AgentRepository,
	sessionRepo repository.SessionRepository,
	apiKeyRepo repository.APIKeyRepository,
) *AgentService {
	return &AgentService{
		agentRepo:   agentRepo,
		sessionRepo: sessionRepo,
		apiKeyRepo:  apiKeyRepo,
	}
}

// GetAgents 获取智能体列表
func (s *AgentService) GetAgents(page, perPage int, category, search string) (*models.AgentListResponse, error) {
	filter := &repository.AgentFilter{
		Page:     page,
		PerPage:  perPage,
		Category: category,
		Search:   search,
	}

	agents, total, err := s.agentRepo.FindWithFilter(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	return &models.AgentListResponse{
		Total:   total,
		Agents:  agents,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// GetAgentByID 根据 ID 获取智能体
func (s *AgentService) GetAgentByID(id uint) (*models.Agent, error) {
	return s.agentRepo.FindByID(context.Background(), id)
}

// GetCategories 获取分类统计
func (s *AgentService) GetCategories() map[string]int64 {
	categories, err := s.agentRepo.GetCategories(context.Background())
	if err != nil {
		return make(map[string]int64)
	}
	return categories
}

// GetStats 获取统计信息
func (s *AgentService) GetStats() *models.AgentStatsResponse {
	stats, err := s.agentRepo.GetStats(context.Background())
	if err != nil {
		return &models.AgentStatsResponse{}
	}
	return &models.AgentStatsResponse{
		TotalAgents: stats.TotalAgents,
		TotalUsage:  stats.TotalUsage,
		Categories:  stats.Categories,
	}
}

// CreateAgent 创建智能体
func (s *AgentService) CreateAgent(agent *models.Agent) error {
	if agent.Name == "" {
		return errors.New("name is required")
	}
	agent.Slug = s.generateSlug(agent.Name)
	if agent.Category == "" {
		agent.Category = "Other"
	}
	if agent.Icon == "" {
		agent.Icon = "🤖"
	}
	if agent.Model == "" {
		agent.Model = "claude-3-opus-20240229"
	}
	if agent.Temperature == 0 {
		agent.Temperature = 0.7
	}
	if agent.MaxTokens == 0 {
		agent.MaxTokens = 4096
	}
	return s.agentRepo.Create(context.Background(), agent)
}

// UpdateAgent 更新智能体
func (s *AgentService) UpdateAgent(id uint, req *models.UpdateAgentRequest) (*models.Agent, error) {
	agent, err := s.agentRepo.FindByID(context.Background(), id)
	if err != nil {
		return nil, errors.New("agent not found")
	}

	if req.Name != "" {
		agent.Name = req.Name
		agent.Slug = s.generateSlug(req.Name)
	}
	if req.Icon != "" {
		agent.Icon = req.Icon
	}
	if req.Category != "" {
		agent.Category = req.Category
	}
	if req.Description != "" {
		agent.Description = req.Description
	}
	if req.SystemPrompt != "" {
		agent.SystemPrompt = req.SystemPrompt
	}
	if req.Model != "" {
		agent.Model = req.Model
	}
	if req.Temperature > 0 {
		agent.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		agent.MaxTokens = req.MaxTokens
	}
	agent.RedirectURL = req.RedirectURL

	if err := s.agentRepo.Update(context.Background(), agent); err != nil {
		return nil, err
	}
	return agent, nil
}

// DeleteAgent 删除智能体
func (s *AgentService) DeleteAgent(id uint) error {
	return s.agentRepo.Delete(context.Background(), id)
}

// generateSlug 从名称生成唯一 slug
func (s *AgentService) generateSlug(name string) string {
	slug := strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug = re.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if slug == "" {
		slug = "agent"
	}

	base := slug
	counter := 1
	for {
		existing, err := s.agentRepo.FindBySlug(context.Background(), slug)
		if err != nil || existing == nil {
			break
		}
		counter++
		slug = fmt.Sprintf("%s-%d", base, counter)
	}
	return slug
}

// SaveAPIKey 保存用户 API Key
func (s *AgentService) SaveAPIKey(userID string, req *models.APIKeyRequest) error {
	// 加密 API Key
	encryptedKey, err := utils.EncryptAPIKey(req.APIKey)
	if err != nil {
		return err
	}

	// 查找是否已存在
	existing, err := s.apiKeyRepo.FindByUserAndProvider(context.Background(), userID, req.Provider)
	if err == nil {
		// 更新现有记录
		existing.EncryptedKey = encryptedKey
		existing.BaseModel = req.BaseModel
		existing.CustomEndpoint = req.CustomEndpoint
		return s.apiKeyRepo.Update(context.Background(), existing)
	}

	// 创建新记录
	apiKey := &models.UserAPIKey{
		UserID:         userID,
		Provider:       req.Provider,
		EncryptedKey:   encryptedKey,
		BaseModel:      req.BaseModel,
		CustomEndpoint: req.CustomEndpoint,
	}
	return s.apiKeyRepo.Create(context.Background(), apiKey)
}

// GetAPIKeys 获取用户已配置的 API Key 列表
func (s *AgentService) GetAPIKeys(userID string) ([]models.APIKeyResponse, error) {
	keys, err := s.apiKeyRepo.FindByUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}

	responses := make([]models.APIKeyResponse, len(keys))
	for i, key := range keys {
		responses[i] = models.APIKeyResponse{
			Provider:       key.Provider,
			BaseModel:      key.BaseModel,
			CustomEndpoint: key.CustomEndpoint,
			HasKey:         key.EncryptedKey != "",
		}
	}
	return responses, nil
}

// GetDecryptedAPIKey 获取解密后的 API Key
func (s *AgentService) GetDecryptedAPIKey(userID, provider string) (string, error) {
	key, err := s.apiKeyRepo.FindByUserAndProvider(context.Background(), userID, provider)
	if err != nil {
		return "", errors.New("API key not found for provider: " + provider)
	}
	return utils.DecryptAPIKey(key.EncryptedKey)
}

// DeleteAPIKey 删除 API Key
func (s *AgentService) DeleteAPIKey(userID, provider string) error {
	return s.apiKeyRepo.Delete(context.Background(), userID, provider)
}

// GetOrCreateSession 获取或创建会话
func (s *AgentService) GetOrCreateSession(userID string, agentID uint, sessionID uint) (*models.Session, error) {
	// 如果提供了 sessionID，尝试获取现有会话
	if sessionID > 0 {
		session, err := s.sessionRepo.FindByID(context.Background(), sessionID)
		if err == nil && session.UserID == userID {
			return session, nil
		}
	}

	// 创建新会话
	session := &models.Session{
		UserID:   userID,
		AgentID:  agentID,
		Title:    "新对话",
		Messages: "[]",
		Status:   "active",
	}
	if err := s.sessionRepo.Create(context.Background(), session); err != nil {
		return nil, err
	}
	return session, nil
}

// GetSessionMessages 获取会话消息
func (s *AgentService) GetSessionMessages(session *models.Session) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	if session.Messages == "" || session.Messages == "[]" {
		return messages, nil
	}
	err := json.Unmarshal([]byte(session.Messages), &messages)
	return messages, err
}

// SaveSessionMessages 保存会话消息
func (s *AgentService) SaveSessionMessages(session *models.Session, messages []models.ChatMessage) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	session.Messages = string(data)
	return s.sessionRepo.Update(context.Background(), session)
}

// AddMessageToSession 添加消息到会话
func (s *AgentService) AddMessageToSession(session *models.Session, role, content string) error {
	messages, err := s.GetSessionMessages(session)
	if err != nil {
		return err
	}

	messages = append(messages, models.ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})

	// 限制消息数量（保留最近 10 轮 = 20 条消息）
	const maxMessages = 20
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}

	// 更新标题（如果是第一条用户消息）
	if len(messages) == 1 && role == "user" {
		if len(content) > 50 {
			session.Title = content[:50] + "..."
		} else {
			session.Title = content
		}
	}

	return s.SaveSessionMessages(session, messages)
}

// GetUserSessions 获取用户的会话列表
func (s *AgentService) GetUserSessions(userID string, agentID uint, limit int) ([]models.Session, error) {
	if agentID > 0 {
		return s.sessionRepo.FindByUserAndAgent(context.Background(), userID, agentID, limit)
	}
	return s.sessionRepo.FindByUser(context.Background(), userID, limit)
}

// DeleteSession 删除会话
func (s *AgentService) DeleteSession(userID string, sessionID uint) error {
	session, err := s.sessionRepo.FindByID(context.Background(), sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return errors.New("unauthorized")
	}
	return s.sessionRepo.Delete(context.Background(), sessionID)
}

// IncrementAgentUsage 增加智能体使用次数
func (s *AgentService) IncrementAgentUsage(agentID uint) error {
	return s.agentRepo.IncrementUsage(context.Background(), agentID)
}
