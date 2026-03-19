package models

import (
	"time"

	"gorm.io/gorm"
)

// Skill GORM 模型
type Skill struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"index;not null;size:255" json:"name"`
	Icon        string         `gorm:"size:500" json:"icon"`
	Category    string         `gorm:"index;size:100" json:"category"`
	Description string         `gorm:"size:1000" json:"description"`
	Downloads   int            `gorm:"default:0" json:"downloads"`
	Rating      int            `gorm:"default:0" json:"rating"`
	Verified    bool           `gorm:"default:false" json:"verified"`
	Accelerated bool           `gorm:"default:true" json:"accelerated"`
	Safe        bool           `gorm:"default:true" json:"safe"`
	FileName    string         `gorm:"size:255" json:"file_name,omitempty"`
}

// TableName 指定表名
func (Skill) TableName() string {
	return "skills"
}

// SkillListResponse 技能列表响应
type SkillListResponse struct {
	Total   int64   `json:"total"`
	Skills  []Skill `json:"skills"`
	Page    int     `json:"page"`
	PerPage int     `json:"per_page"`
}

// UploadResponse 上传响应
type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Skill   *Skill `json:"skill,omitempty"`
}
