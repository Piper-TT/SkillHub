package models

import (
	"time"

	"gorm.io/gorm"
)

// Session 会话模型
type Session struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   string `gorm:"index;size:100" json:"user_id"`  // 用户标识 (UUID from localStorage)
	AgentID  uint   `gorm:"index" json:"agent_id"`          // 关联的 Agent
	Title    string `gorm:"size:255" json:"title"`          // 会话标题 (自动从首条消息生成)
	Messages string `gorm:"type:text" json:"messages"`      // 消息历史 (JSON array)
	Status   string `gorm:"size:20;default:active" json:"status"` // active, archived
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}

// SessionListResponse 会话列表响应
type SessionListResponse struct {
	Total    int64     `json:"total"`
	Sessions []Session `json:"sessions"`
	Page     int       `json:"page"`
	PerPage  int       `json:"per_page"`
}
