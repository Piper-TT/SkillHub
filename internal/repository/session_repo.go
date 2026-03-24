package repository

import (
	"context"
	"skillhub/internal/models"

	"gorm.io/gorm"
)

// SessionRepository 会话数据访问接口
type SessionRepository interface {
	Create(ctx context.Context, session *models.Session) error
	Update(ctx context.Context, session *models.Session) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*models.Session, error)
	FindByUserAndAgent(ctx context.Context, userID string, agentID uint, limit int) ([]models.Session, error)
	FindByUser(ctx context.Context, userID string, limit int) ([]models.Session, error)
}

type sessionRepo struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepo{db: db}
}

func (r *sessionRepo) Create(ctx context.Context, session *models.Session) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *sessionRepo) Update(ctx context.Context, session *models.Session) error {
	return r.db.WithContext(ctx).Save(session).Error
}

func (r *sessionRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Session{}, id).Error
}

func (r *sessionRepo) FindByID(ctx context.Context, id uint) (*models.Session, error) {
	var session models.Session
	err := r.db.WithContext(ctx).First(&session, id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepo) FindByUserAndAgent(ctx context.Context, userID string, agentID uint, limit int) ([]models.Session, error) {
	var sessions []models.Session
	query := r.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ? AND status = ?", userID, agentID, "active").
		Order("updated_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&sessions).Error
	return sessions, err
}

func (r *sessionRepo) FindByUser(ctx context.Context, userID string, limit int) ([]models.Session, error) {
	var sessions []models.Session
	query := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, "active").
		Order("updated_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&sessions).Error
	return sessions, err
}
