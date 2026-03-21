package models

import (
	"time"

	"gorm.io/gorm"
)

// MCPServer MCP 服务器模型
type MCPServer struct {
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

	// 来源信息
	GitHubURL  string `gorm:"size:500" json:"github_url"`
	NPMPackage string `gorm:"size:255" json:"npm_package"`
	PyPIPkg    string `gorm:"size:255" json:"pypi_package"`

	// 统计数据
	Stars     int `gorm:"default:0" json:"stars"`
	Downloads int `gorm:"default:0" json:"downloads"`

	// 安装信息
	InstallCmd string `gorm:"size:500" json:"install_cmd"`
	Config     string `gorm:"size:2000" json:"config"` // JSON 配置示例

	// 状态标记
	Verified bool `gorm:"default:false" json:"verified"`
	Official bool `gorm:"default:false" json:"official"`
}

// TableName 指定表名
func (MCPServer) TableName() string {
	return "mcp_servers"
}

// MCPServerListResponse 服务器列表响应
type MCPServerListResponse struct {
	Total   int64       `json:"total"`
	Servers []MCPServer `json:"servers"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
}

// MCPStatsResponse 统计数据响应
type MCPStatsResponse struct {
	TotalServers int64 `json:"total_servers"`
	TotalStars   int64 `json:"total_stars"`
	Categories   int64 `json:"categories"`
}
