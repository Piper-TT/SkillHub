package service

import (
	"context"
	"skillhub/internal/models"
	"skillhub/internal/repository"
	"sync"
)

// MCPService MCP 服务
type MCPService struct {
	repo repository.MCPRepository
	mu   sync.RWMutex
}

var (
	mcpServiceInstance *MCPService
	mcpServiceOnce     sync.Once
)

// NewMCPService 创建 MCP 服务实例
func NewMCPService(repo repository.MCPRepository) *MCPService {
	svc := &MCPService{repo: repo}
	mcpServiceOnce.Do(func() {
		mcpServiceInstance = svc
	})
	return svc
}

// GetMCPService 获取全局 MCP 服务实例
func GetMCPService() *MCPService {
	return mcpServiceInstance
}

// GetTopByStars 获取 Stars 最高的 N 个服务器
func (s *MCPService) GetTopByStars(limit int) []models.MCPServer {
	servers, _ := s.repo.FindTopByStars(context.Background(), limit)
	return servers
}

// GetAll 获取所有服务器
func (s *MCPService) GetAll() ([]models.MCPServer, error) {
	return s.repo.FindAll(context.Background())
}

// GetByID 根据 ID 获取服务器
func (s *MCPService) GetByID(id int) (*models.MCPServer, error) {
	return s.repo.FindByID(context.Background(), uint(id))
}

// FilterServers 过滤服务器
func (s *MCPService) FilterServers(page, perPage int, category, search, sortBy string) (*models.MCPServerListResponse, error) {
	filter := &repository.MCPFilter{
		Page:     page,
		PerPage:  perPage,
		Category: category,
		Search:   search,
		SortBy:   sortBy,
	}

	servers, total, err := s.repo.FindWithFilter(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	return &models.MCPServerListResponse{
		Total:   total,
		Servers: servers,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// GetCategories 获取分类列表
func (s *MCPService) GetCategories() (map[string]int64, error) {
	return s.repo.GetCategories(context.Background())
}

// GetStats 获取统计数据
func (s *MCPService) GetStats() (*models.MCPStatsResponse, error) {
	stats, err := s.repo.GetStats(context.Background())
	if err != nil {
		return nil, err
	}

	return &models.MCPStatsResponse{
		TotalServers: stats.TotalServers,
		TotalStars:   stats.TotalStars,
		Categories:   stats.Categories,
	}, nil
}
