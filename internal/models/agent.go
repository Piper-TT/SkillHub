package models

import (
	"time"

	"gorm.io/gorm"
)

// Agent 智能体模型
type Agent struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 基本信息
	Name        string `gorm:"index;not null;size:255" json:"name"`
	Slug        string `gorm:"index;size:255" json:"slug"`
	Icon        string `gorm:"size:50" json:"icon"`
	Category    string `gorm:"index;size:100" json:"category"`
	Description string `gorm:"size:500" json:"description"`

	// LLM 配置
	SystemPrompt string  `gorm:"size:4000" json:"system_prompt"`
	Model        string  `gorm:"size:100" json:"model"`         // 推荐模型
	Temperature  float64 `gorm:"default:0.7" json:"temperature"` // 温度参数
	MaxTokens    int     `gorm:"default:4096" json:"max_tokens"` // 最大输出 tokens

	// 状态标记
	Verified    bool   `gorm:"default:false" json:"verified"`
	UsageCount  int    `gorm:"default:0" json:"usage_count"` // 使用次数
	RedirectURL string `gorm:"size:255" json:"redirect_url"` // 自定义跳转地址，为空则走默认聊天页
}

// TableName 指定表名
func (Agent) TableName() string {
	return "agents"
}

// AgentListResponse 智能体列表响应
type AgentListResponse struct {
	Total  int64   `json:"total"`
	Agents []Agent `json:"agents"`
	Page   int     `json:"page"`
	PerPage int    `json:"per_page"`
}

// AgentStatsResponse 统计数据响应
type AgentStatsResponse struct {
	TotalAgents int64 `json:"total_agents"`
	TotalUsage  int64 `json:"total_usage"`
	Categories  int64 `json:"categories"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message   string `json:"message" binding:"required"`
	SessionID uint   `json:"session_id"` // 可选，为空则创建新会话
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role      string    `json:"role"`      // user, assistant, system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}
