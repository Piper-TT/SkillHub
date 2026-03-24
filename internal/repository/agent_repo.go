package repository

import (
	"context"
	"skillhub/internal/models"
	"strings"

	"gorm.io/gorm"
)

// AgentRepository 智能体数据访问接口
type AgentRepository interface {
	Create(ctx context.Context, agent *models.Agent) error
	Update(ctx context.Context, agent *models.Agent) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*models.Agent, error)
	FindBySlug(ctx context.Context, slug string) (*models.Agent, error)
	FindWithFilter(ctx context.Context, filter *AgentFilter) ([]models.Agent, int64, error)
	GetCategories(ctx context.Context) (map[string]int64, error)
	GetStats(ctx context.Context) (*AgentStats, error)
	IncrementUsage(ctx context.Context, id uint) error
}

// AgentFilter 过滤条件
type AgentFilter struct {
	Page     int
	PerPage  int
	Category string
	Search   string
}

// AgentStats 统计数据
type AgentStats struct {
	TotalAgents int64
	TotalUsage  int64
	Categories  int64
}

type agentRepo struct {
	db *gorm.DB
}

func NewAgentRepository(db *gorm.DB) AgentRepository {
	return &agentRepo{db: db}
}

func (r *agentRepo) Create(ctx context.Context, agent *models.Agent) error {
	return r.db.WithContext(ctx).Create(agent).Error
}

func (r *agentRepo) Update(ctx context.Context, agent *models.Agent) error {
	return r.db.WithContext(ctx).Save(agent).Error
}

func (r *agentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Agent{}, id).Error
}

func (r *agentRepo) FindByID(ctx context.Context, id uint) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).First(&agent, id).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *agentRepo) FindBySlug(ctx context.Context, slug string) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *agentRepo) FindWithFilter(ctx context.Context, filter *AgentFilter) ([]models.Agent, int64, error) {
	var agents []models.Agent
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Agent{})

	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}

	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(category) LIKE ?",
			search, search, search,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (filter.Page - 1) * filter.PerPage
	if err := query.Offset(offset).Limit(filter.PerPage).Order("id ASC").Find(&agents).Error; err != nil {
		return nil, 0, err
	}

	return agents, total, nil
}

func (r *agentRepo) GetCategories(ctx context.Context) (map[string]int64, error) {
	type categoryCount struct {
		Category string
		Count    int64
	}

	var results []categoryCount
	err := r.db.WithContext(ctx).
		Model(&models.Agent{}).
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

func (r *agentRepo) GetStats(ctx context.Context) (*AgentStats, error) {
	var stats AgentStats

	if err := r.db.WithContext(ctx).Model(&models.Agent{}).Count(&stats.TotalAgents).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&models.Agent{}).
		Select("COALESCE(SUM(usage_count), 0)").
		Scan(&stats.TotalUsage).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&models.Agent{}).
		Distinct("category").
		Count(&stats.Categories).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

func (r *agentRepo) IncrementUsage(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Agent{}).
		Where("id = ?", id).
		UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
}
