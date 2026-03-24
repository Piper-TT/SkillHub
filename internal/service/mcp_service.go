package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"skillhub/internal/models"
	"skillhub/internal/repository"
	"skillhub/internal/utils"
	"strings"
	"sync"
)

// MCPService MCP 服务
type MCPService struct {
	repo      repository.MCPRepository
	uploadDir string
	mu        sync.RWMutex
}

var (
	mcpServiceInstance *MCPService
	mcpServiceOnce     sync.Once
)

// NewMCPService 创建 MCP 服务实例
func NewMCPService(repo repository.MCPRepository) *MCPService {
	uploadDir := "./uploads/mcp"
	os.MkdirAll(uploadDir, 0755)

	svc := &MCPService{
		repo:      repo,
		uploadDir: uploadDir,
	}
	mcpServiceOnce.Do(func() {
		mcpServiceInstance = svc
	})
	return svc
}

// GetMCPService 获取全局 MCP 服务实例
func GetMCPService() *MCPService {
	return mcpServiceInstance
}

// UploadRequest MCP 上传请求
type MCPUploadRequest struct {
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Description string `json:"description"`
	GitHubURL   string `json:"github_url"`
	FileName    string `json:"file_name"`
}

// UploadServer 上传 MCP 服务器文件
func (s *MCPService) UploadServer(req *MCPUploadRequest, fileData io.Reader) (*models.MCPServer, error) {
	if err := utils.ValidateSkillInput(req.Name, req.Category); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建上传目录
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %v", err)
	}

	// 净化文件名
	safeFilename := utils.SanitizeFilename(req.FileName)
	ext := filepath.Ext(safeFilename)
	if ext == "" {
		ext = ".zip"
	}
	baseName := strings.TrimSuffix(safeFilename, ext)
	baseName = strings.ReplaceAll(req.Name, " ", "_")
	baseName = strings.ReplaceAll(baseName, "/", "_")

	// 生成唯一文件名
	fileName := fmt.Sprintf("mcp_%d_%s%s", os.Getpid(), baseName, ext)
	counter := 1
	originalFileName := fileName
	for {
		filePath := filepath.Join(s.uploadDir, fileName)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		fileName = fmt.Sprintf("%s_%d%s", strings.TrimSuffix(originalFileName, ext), counter, ext)
		counter++
	}

	filePath := filepath.Join(s.uploadDir, fileName)

	// 保存文件
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fileData); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save file: %v", err)
	}

	// 创建 MCPServer 记录
	server := &models.MCPServer{
		Name:        req.Name,
		Icon:        req.Icon,
		Category:    req.Category,
		Description: req.Description,
		GitHubURL:   req.GitHubURL,
		FileName:    fileName,
		Verified:    false,
		Official:    false,
	}

	if err := s.repo.Create(context.Background(), server); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save server: %v", err)
	}

	return server, nil
}

// DownloadServer 下载 MCP 服务器文件
func (s *MCPService) DownloadServer(id int) (string, error) {
	server, err := s.repo.FindByID(context.Background(), uint(id))
	if err != nil {
		return "", errors.New("server not found")
	}

	if server.FileName == "" {
		return "", errors.New("server has no local file")
	}

	filePath := filepath.Join(s.uploadDir, server.FileName)
	if _, err := os.Stat(filePath); err != nil {
		return "", errors.New("file not found")
	}

	// 增加下载计数
	go s.repo.IncrementDownloads(context.Background(), uint(id))

	return filePath, nil
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
