package service

import (
	"context"
	"fmt"
	"net/url"
	"skillhub/internal/models"
	"skillhub/internal/repository"
	"skillhub/internal/utils"
	"sync"
)

// ServerService Server 服务
type ServerService struct {
	repo repository.ServerRepository
	mu   sync.RWMutex
}

var (
	serverServiceInstance *ServerService
	serverServiceOnce     sync.Once
)

// NewServerService 创建 Server 服务实例
func NewServerService(repo repository.ServerRepository) *ServerService {
	svc := &ServerService{repo: repo}
	serverServiceOnce.Do(func() {
		serverServiceInstance = svc
	})
	return svc
}

// GetServerService 获取全局 Server 服务实例
func GetServerService() *ServerService {
	return serverServiceInstance
}

// ServerUploadRequest Server 注册请求
type ServerUploadRequest struct {
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Description string `json:"description"`
	GitHubURL   string `json:"github_url"`
	URL         string `json:"url"`
}

// UploadServer 注册一个 Web 服务
func (s *ServerService) UploadServer(req *ServerUploadRequest) (*models.Server, error) {
	if err := utils.ValidateSkillInput(req.Name, req.Category); err != nil {
		return nil, err
	}
	if req.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	parsedURL, err := url.Parse(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, fmt.Errorf("invalid URL: must be http or https")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	server := &models.Server{
		Name:        req.Name,
		Icon:        req.Icon,
		Category:    req.Category,
		Description: req.Description,
		GitHubURL:   req.GitHubURL,
		ServiceURL:  req.URL,
		Verified:    false,
		Official:    false,
	}

	if err := s.repo.Create(context.Background(), server); err != nil {
		return nil, fmt.Errorf("failed to save server: %v", err)
	}

	return server, nil
}

// GetTopByStars 获取 Stars 最高的 N 个服务
func (s *ServerService) GetTopByStars(limit int) []models.Server {
	servers, _ := s.repo.FindTopByStars(context.Background(), limit)
	return servers
}

// GetAll 获取所有服务
func (s *ServerService) GetAll() ([]models.Server, error) {
	return s.repo.FindAll(context.Background())
}

// GetByID 根据 ID 获取服务
func (s *ServerService) GetByID(id int) (*models.Server, error) {
	return s.repo.FindByID(context.Background(), uint(id))
}

// FilterServers 过滤服务
func (s *ServerService) FilterServers(page, perPage int, category, search, sortBy string) (*models.ServerListResponse, error) {
	filter := &repository.ServerFilter{
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

	return &models.ServerListResponse{
		Total:   total,
		Servers: servers,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// GetCategories 获取分类列表
func (s *ServerService) GetCategories() (map[string]int64, error) {
	return s.repo.GetCategories(context.Background())
}

// GetStats 获取统计数据
func (s *ServerService) GetStats() (*models.ServerStatsResponse, error) {
	stats, err := s.repo.GetStats(context.Background())
	if err != nil {
		return nil, err
	}

	return &models.ServerStatsResponse{
		TotalServers: stats.TotalServers,
		TotalStars:   stats.TotalStars,
		Categories:   stats.Categories,
	}, nil
}
