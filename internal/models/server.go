package models

import (
	"time"

	"gorm.io/gorm"
)

// Server 服务模型
type Server struct {
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

	// Web 服务地址
	ServiceURL string `gorm:"size:500" json:"service_url"`

	// 安装信息
	InstallCmd string `gorm:"size:500" json:"install_cmd"`
	Config     string `gorm:"size:2000" json:"config"`

	// 状态标记
	Verified bool `gorm:"default:false" json:"verified"`
	Official bool `gorm:"default:false" json:"official"`
}

// TableName 指定表名
func (Server) TableName() string {
	return "servers"
}

// ServerListResponse 服务器列表响应
type ServerListResponse struct {
	Total   int64    `json:"total"`
	Servers []Server `json:"servers"`
	Page    int      `json:"page"`
	PerPage int      `json:"per_page"`
}

// ServerStatsResponse 统计数据响应
type ServerStatsResponse struct {
	TotalServers int64 `json:"total_servers"`
	TotalStars   int64 `json:"total_stars"`
	Categories   int64 `json:"categories"`
}
