package repository

import (
	"context"
	"skillhub/internal/models"
	"strings"

	"gorm.io/gorm"
)

// MCPRepository MCP 数据访问接口
type MCPRepository interface {
	// Create 创建服务器
	Create(ctx context.Context, server *models.MCPServer) error
	// Update 更新服务器
	Update(ctx context.Context, server *models.MCPServer) error
	// Delete 软删除服务器
	Delete(ctx context.Context, id uint) error
	// FindByID 根据 ID 查找
	FindByID(ctx context.Context, id uint) (*models.MCPServer, error)
	// FindAll 获取所有服务器
	FindAll(ctx context.Context) ([]models.MCPServer, error)
	// FindWithFilter 带过滤条件的分页查询
	FindWithFilter(ctx context.Context, filter *MCPFilter) ([]models.MCPServer, int64, error)
	// GetCategories 获取所有分类及其计数
	GetCategories(ctx context.Context) (map[string]int64, error)
	// GetStats 获取统计数据
	GetStats(ctx context.Context) (*MCPStats, error)
	// FindTopByStars 获取 Stars 最高的 N 个
	FindTopByStars(ctx context.Context, limit int) ([]models.MCPServer, error)
	// ReplaceAll 替换所有数据
	ReplaceAll(ctx context.Context, servers []models.MCPServer) error
	// IncrementDownloads 增加下载计数
	IncrementDownloads(ctx context.Context, id uint) error
}

// MCPFilter 过滤条件
type MCPFilter struct {
	Page     int
	PerPage  int
	Category string
	Search   string
	SortBy   string // "stars", "downloads", "name"
}

// MCPStats 统计数据
type MCPStats struct {
	TotalServers int64
	TotalStars   int64
	Categories   int64
}

// mcpRepo 实现
type mcpRepo struct {
	db *gorm.DB
}

// NewMCPRepository 创建 Repository 实例
func NewMCPRepository(db *gorm.DB) MCPRepository {
	return &mcpRepo{db: db}
}

func (r *mcpRepo) Create(ctx context.Context, server *models.MCPServer) error {
	return r.db.WithContext(ctx).Create(server).Error
}

func (r *mcpRepo) Update(ctx context.Context, server *models.MCPServer) error {
	return r.db.WithContext(ctx).Save(server).Error
}

func (r *mcpRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.MCPServer{}, id).Error
}

func (r *mcpRepo) FindByID(ctx context.Context, id uint) (*models.MCPServer, error) {
	var server models.MCPServer
	err := r.db.WithContext(ctx).First(&server, id).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *mcpRepo) FindAll(ctx context.Context) ([]models.MCPServer, error) {
	var servers []models.MCPServer
	err := r.db.WithContext(ctx).Find(&servers).Error
	return servers, err
}

func (r *mcpRepo) FindWithFilter(ctx context.Context, filter *MCPFilter) ([]models.MCPServer, int64, error) {
	var servers []models.MCPServer
	var total int64

	query := r.db.WithContext(ctx).Model(&models.MCPServer{})

	// 分类过滤
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}

	// 搜索过滤
	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(description) LIKE ?",
			search, search,
		)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	orderBy := "stars DESC"
	switch filter.SortBy {
	case "downloads":
		orderBy = "downloads DESC"
	case "name":
		orderBy = "name ASC"
	}
	query = query.Order(orderBy)

	// 分页
	offset := (filter.Page - 1) * filter.PerPage
	if err := query.Offset(offset).Limit(filter.PerPage).Find(&servers).Error; err != nil {
		return nil, 0, err
	}

	return servers, total, nil
}

func (r *mcpRepo) GetCategories(ctx context.Context) (map[string]int64, error) {
	type categoryCount struct {
		Category string
		Count    int64
	}

	var results []categoryCount
	err := r.db.WithContext(ctx).
		Model(&models.MCPServer{}).
		Select("category, count(*) as count").
		Group("category").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	categories := make(map[string]int64)
	for _, r := range results {
		categories[r.Category] = r.Count
	}

	return categories, nil
}

func (r *mcpRepo) GetStats(ctx context.Context) (*MCPStats, error) {
	var stats MCPStats

	// 总服务器数
	if err := r.db.WithContext(ctx).Model(&models.MCPServer{}).Count(&stats.TotalServers).Error; err != nil {
		return nil, err
	}

	// 总 Stars
	if err := r.db.WithContext(ctx).Model(&models.MCPServer{}).
		Select("COALESCE(SUM(stars), 0)").
		Scan(&stats.TotalStars).Error; err != nil {
		return nil, err
	}

	// 分类数
	if err := r.db.WithContext(ctx).Model(&models.MCPServer{}).
		Distinct("category").
		Count(&stats.Categories).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

func (r *mcpRepo) FindTopByStars(ctx context.Context, limit int) ([]models.MCPServer, error) {
	var servers []models.MCPServer
	err := r.db.WithContext(ctx).
		Order("stars DESC").
		Limit(limit).
		Find(&servers).Error
	return servers, err
}

func (r *mcpRepo) ReplaceAll(ctx context.Context, servers []models.MCPServer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 只删除外部导入的数据（FileName 为空），保留本地上传的 MCP Server
		if err := tx.Exec("DELETE FROM mcp_servers WHERE file_name = '' OR file_name IS NULL").Error; err != nil {
			return err
		}

		// 批量插入
		batchSize := 100
		for i := 0; i < len(servers); i += batchSize {
			end := i + batchSize
			if end > len(servers) {
				end = len(servers)
			}
			batch := servers[i:end]
			if err := tx.Create(&batch).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *mcpRepo) IncrementDownloads(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.MCPServer{}).
		Where("id = ?", id).
		UpdateColumn("downloads", gorm.Expr("downloads + 1")).
		Error
}
