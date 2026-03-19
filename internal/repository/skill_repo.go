package repository

import (
	"context"
	"skillhub/internal/models"
	"strings"

	"gorm.io/gorm"
)

// SkillRepository 数据访问接口
type SkillRepository interface {
	// Create 创建技能
	Create(ctx context.Context, skill *models.Skill) error
	// Update 更新技能
	Update(ctx context.Context, skill *models.Skill) error
	// Delete 软删除技能
	Delete(ctx context.Context, id uint) error
	// FindByID 根据 ID 查找
	FindByID(ctx context.Context, id uint) (*models.Skill, error)
	// FindAll 获取所有技能
	FindAll(ctx context.Context) ([]models.Skill, error)
	// FindWithFilter 带过滤条件的分页查询
	FindWithFilter(ctx context.Context, filter *SkillFilter) ([]models.Skill, int64, error)
	// IncrementDownloads 增加下载次数
	IncrementDownloads(ctx context.Context, id uint) error
	// GetCategories 获取所有分类及其计数
	GetCategories(ctx context.Context) (map[string]int64, error)
	// GetStats 获取统计数据
	GetStats(ctx context.Context) (*SkillStats, error)
	// FindTopByDownloads 获取下载量最高的 N 个
	FindTopByDownloads(ctx context.Context, limit int) ([]models.Skill, error)
}

// SkillFilter 过滤条件
type SkillFilter struct {
	Page     int
	PerPage  int
	Category string
	Search   string
}

// SkillStats 统计数据
type SkillStats struct {
	TotalSkills    int64
	TotalDownloads int64
	Categories     int64
}

// skillRepo 实现
type skillRepo struct {
	db *gorm.DB
}

// NewSkillRepository 创建 Repository 实例
func NewSkillRepository(db *gorm.DB) SkillRepository {
	return &skillRepo{db: db}
}

func (r *skillRepo) Create(ctx context.Context, skill *models.Skill) error {
	return r.db.WithContext(ctx).Create(skill).Error
}

func (r *skillRepo) Update(ctx context.Context, skill *models.Skill) error {
	return r.db.WithContext(ctx).Save(skill).Error
}

func (r *skillRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Skill{}, id).Error
}

func (r *skillRepo) FindByID(ctx context.Context, id uint) (*models.Skill, error) {
	var skill models.Skill
	err := r.db.WithContext(ctx).First(&skill, id).Error
	if err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *skillRepo) FindAll(ctx context.Context) ([]models.Skill, error) {
	var skills []models.Skill
	err := r.db.WithContext(ctx).Find(&skills).Error
	return skills, err
}

func (r *skillRepo) FindWithFilter(ctx context.Context, filter *SkillFilter) ([]models.Skill, int64, error) {
	var skills []models.Skill
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Skill{})

	// 分类过滤
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}

	// 搜索过滤
	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(category) LIKE ?",
			search, search, search,
		)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	offset := (filter.Page - 1) * filter.PerPage
	if err := query.Offset(offset).Limit(filter.PerPage).Find(&skills).Error; err != nil {
		return nil, 0, err
	}

	return skills, total, nil
}

func (r *skillRepo) IncrementDownloads(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Skill{}).
		Where("id = ?", id).
		UpdateColumn("downloads", gorm.Expr("downloads + 1")).Error
}

func (r *skillRepo) GetCategories(ctx context.Context) (map[string]int64, error) {
	type categoryCount struct {
		Category string
		Count    int64
	}

	var results []categoryCount
	err := r.db.WithContext(ctx).
		Model(&models.Skill{}).
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

func (r *skillRepo) GetStats(ctx context.Context) (*SkillStats, error) {
	var stats SkillStats

	// 总技能数
	if err := r.db.WithContext(ctx).Model(&models.Skill{}).Count(&stats.TotalSkills).Error; err != nil {
		return nil, err
	}

	// 总下载量
	if err := r.db.WithContext(ctx).Model(&models.Skill{}).
		Select("COALESCE(SUM(downloads), 0)").
		Scan(&stats.TotalDownloads).Error; err != nil {
		return nil, err
	}

	// 分类数
	if err := r.db.WithContext(ctx).Model(&models.Skill{}).
		Distinct("category").
		Count(&stats.Categories).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

func (r *skillRepo) FindTopByDownloads(ctx context.Context, limit int) ([]models.Skill, error) {
	var skills []models.Skill
	err := r.db.WithContext(ctx).
		Order("downloads DESC").
		Limit(limit).
		Find(&skills).Error
	return skills, err
}
