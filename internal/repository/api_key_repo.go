package repository

import (
	"context"
	"skillhub/internal/models"

	"gorm.io/gorm"
)

// APIKeyRepository API Key 数据访问接口
type APIKeyRepository interface {
	Create(ctx context.Context, apiKey *models.UserAPIKey) error
	Update(ctx context.Context, apiKey *models.UserAPIKey) error
	Delete(ctx context.Context, userID, provider string) error
	FindByUserAndProvider(ctx context.Context, userID, provider string) (*models.UserAPIKey, error)
	FindByUser(ctx context.Context, userID string) ([]models.UserAPIKey, error)
}

type apiKeyRepo struct {
	db *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) APIKeyRepository {
	return &apiKeyRepo{db: db}
}

func (r *apiKeyRepo) Create(ctx context.Context, apiKey *models.UserAPIKey) error {
	return r.db.WithContext(ctx).Create(apiKey).Error
}

func (r *apiKeyRepo) Update(ctx context.Context, apiKey *models.UserAPIKey) error {
	return r.db.WithContext(ctx).Save(apiKey).Error
}

func (r *apiKeyRepo) Delete(ctx context.Context, userID, provider string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		Delete(&models.UserAPIKey{}).Error
}

func (r *apiKeyRepo) FindByUserAndProvider(ctx context.Context, userID, provider string) (*models.UserAPIKey, error) {
	var apiKey models.UserAPIKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		First(&apiKey).Error
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (r *apiKeyRepo) FindByUser(ctx context.Context, userID string) ([]models.UserAPIKey, error) {
	var apiKeys []models.UserAPIKey
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&apiKeys).Error
	return apiKeys, err
}
