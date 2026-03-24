package models

import (
	"time"

	"gorm.io/gorm"
)

// UserAPIKey 用户 API Key 模型
type UserAPIKey struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID        string `gorm:"index;size:100" json:"user_id"`           // 用户标识
	Provider      string `gorm:"index;size:50" json:"provider"`           // anthropic, openai, deepseek, openai-compatible
	EncryptedKey  string `gorm:"size:500" json:"-"`                       // AES-256-GCM 加密后的 API Key
	BaseModel     string `gorm:"size:100" json:"base_model"`              // 默认模型
	CustomEndpoint string `gorm:"size:255" json:"custom_endpoint"`        // 仅 openai-compatible 使用
}

// TableName 指定表名
func (UserAPIKey) TableName() string {
	return "user_api_keys"
}

// APIKeyRequest API Key 请求
type APIKeyRequest struct {
	Provider       string `json:"provider" binding:"required"`
	APIKey         string `json:"api_key" binding:"required"`
	BaseModel      string `json:"base_model"`
	CustomEndpoint string `json:"custom_endpoint"` // 仅 openai-compatible
}

// APIKeyResponse API Key 响应 (不包含实际 key)
type APIKeyResponse struct {
	Provider       string `json:"provider"`
	BaseModel      string `json:"base_model"`
	CustomEndpoint string `json:"custom_endpoint,omitempty"`
	HasKey         bool   `json:"has_key"` // 是否已配置
}

// ValidateKeyRequest 验证 API Key 请求
type ValidateKeyRequest struct {
	Provider       string `json:"provider" binding:"required"`
	APIKey         string `json:"api_key" binding:"required"`
	CustomEndpoint string `json:"custom_endpoint"`
}

// ValidateKeyResponse 验证 API Key 响应
type ValidateKeyResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}
