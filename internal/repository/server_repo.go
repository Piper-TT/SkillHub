package repository

import (
	"context"
	"skillhub/internal/models"
	"strings"

	"gorm.io/gorm"
)

// ServerRepository Server 数据访问接口
type ServerRepository interface {
	Create(ctx context.Context, server *models.Server) error
	Update(ctx context.Context, server *models.Server) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*models.Server, error)
	FindAll(ctx context.Context) ([]models.Server, error)
	FindWithFilter(ctx context.Context, filter *ServerFilter) ([]models.Server, int64, error)
	GetCategories(ctx context.Context) (map[string]int64, error)
	GetStats(ctx context.Context) (*ServerStats, error)
	FindTopByStars(ctx context.Context, limit int) ([]models.Server, error)
	ReplaceAll(ctx context.Context, servers []models.Server) error
	IncrementDownloads(ctx context.Context, id uint) error
}

// ServerFilter 过滤条件
type ServerFilter struct {
	Page     int
	PerPage  int
	Category string
	Search   string
	SortBy   string
}

// ServerStats 统计数据
type ServerStats struct {
	TotalServers int64
	TotalStars   int64
	Categories   int64
}

type serverRepo struct {
	db *gorm.DB
}

// NewServerRepository 创建 Repository 实例
func NewServerRepository(db *gorm.DB) ServerRepository {
	return &serverRepo{db: db}
}

func (r *serverRepo) Create(ctx context.Context, server *models.Server) error {
	return r.db.WithContext(ctx).Create(server).Error
}

func (r *serverRepo) Update(ctx context.Context, server *models.Server) error {
	return r.db.WithContext(ctx).Save(server).Error
}

func (r *serverRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Server{}, id).Error
}

func (r *serverRepo) FindByID(ctx context.Context, id uint) (*models.Server, error) {
	var server models.Server
	err := r.db.WithContext(ctx).First(&server, id).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *serverRepo) FindAll(ctx context.Context) ([]models.Server, error) {
	var servers []models.Server
	err := r.db.WithContext(ctx).Find(&servers).Error
	return servers, err
}

func (r *serverRepo) FindWithFilter(ctx context.Context, filter *ServerFilter) ([]models.Server, int64, error) {
	var servers []models.Server
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Server{})

	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}

	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(description) LIKE ?",
			search, search,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderBy := "stars DESC"
	switch filter.SortBy {
	case "downloads":
		orderBy = "downloads DESC"
	case "name":
		orderBy = "name ASC"
	}
	query = query.Order(orderBy)

	offset := (filter.Page - 1) * filter.PerPage
	if err := query.Offset(offset).Limit(filter.PerPage).Find(&servers).Error; err != nil {
		return nil, 0, err
	}

	return servers, total, nil
}

func (r *serverRepo) GetCategories(ctx context.Context) (map[string]int64, error) {
	type categoryCount struct {
		Category string
		Count    int64
	}

	var results []categoryCount
	err := r.db.WithContext(ctx).
		Model(&models.Server{}).
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

func (r *serverRepo) GetStats(ctx context.Context) (*ServerStats, error) {
	var stats ServerStats

	if err := r.db.WithContext(ctx).Model(&models.Server{}).Count(&stats.TotalServers).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&models.Server{}).
		Select("COALESCE(SUM(stars), 0)").
		Scan(&stats.TotalStars).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&models.Server{}).
		Distinct("category").
		Count(&stats.Categories).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

func (r *serverRepo) FindTopByStars(ctx context.Context, limit int) ([]models.Server, error) {
	var servers []models.Server
	err := r.db.WithContext(ctx).
		Order("stars DESC").
		Limit(limit).
		Find(&servers).Error
	return servers, err
}

func (r *serverRepo) ReplaceAll(ctx context.Context, servers []models.Server) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM servers WHERE file_name = '' OR file_name IS NULL").Error; err != nil {
			return err
		}

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

func (r *serverRepo) IncrementDownloads(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.Server{}).
		Where("id = ?", id).
		UpdateColumn("downloads", gorm.Expr("downloads + 1")).
		Error
}
