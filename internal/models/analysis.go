package models

import (
	"time"

	"gorm.io/gorm"
)

// AnalysisTask 分析任务
type AnalysisTask struct {
	gorm.Model
	UserID       string     `gorm:"index;size:100" json:"user_id"`
	AgentID      uint       `gorm:"index" json:"agent_id"`
	FileName     string     `gorm:"size:255" json:"file_name"`
	FilePath     string     `gorm:"size:500" json:"-"`         // 内部存储路径，不暴露给前端
	FileHash     string     `gorm:"index;size:64" json:"file_hash"` // SHA256
	FileType     string     `gorm:"size:20" json:"file_type"`  // PE/ELF/Mach-O/Unknown
	FileSize     int64      `gorm:"default:0" json:"file_size"`
	Status       string     `gorm:"index;size:20;default:pending" json:"status"` // pending/running/completed/failed/cancelled
	Progress     int        `gorm:"default:0" json:"progress"` // 0-100
	IDAServerID  uint       `gorm:"index" json:"ida_server_id"`
	ResultJSON   string     `gorm:"type:text" json:"-"`        // IDA 分析结果 (JSON)
	ReportMD     string     `gorm:"type:text" json:"report_md"` // Markdown 报告
	ReportPDF    string     `gorm:"size:255" json:"-"`         // PDF 文件路径
	ErrorMessage string     `gorm:"size:500" json:"error_message,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// TableName 指定表名
func (AnalysisTask) TableName() string {
	return "analysis_tasks"
}

// AnalysisTaskListResponse 任务列表响应
type AnalysisTaskListResponse struct {
	Total int64           `json:"total"`
	Tasks []AnalysisTask  `json:"tasks"`
	Page  int             `json:"page"`
	PerPage int           `json:"per_page"`
}

// AnalysisResult IDA 分析结果
type AnalysisResult struct {
	// 基本信息
	FileName     string `json:"file_name"`
	FilePath     string `json:"file_path"`
	FileSize     int64  `json:"file_size"`
	FileType     string `json:"file_type"`
	Architecture string `json:"architecture"`
	Bits         int    `json:"bits"`
	Endianness   string `json:"endianness"`
	EntryPoint   uint64 `json:"entry_point"`
	BaseAddress  uint64 `json:"base_address"`
	CompileTime  string `json:"compile_time"`

	// 节区信息
	Sections []SectionInfo `json:"sections"`

	// 导入函数
	Imports []ImportInfo `json:"imports"`

	// 导出函数
	Exports []ExportInfo `json:"exports"`

	// 函数列表
	Functions []FunctionInfo `json:"functions"`

	// 字符串
	Strings []StringInfo `json:"strings"`

	// 分析时间
	AnalysisTime time.Duration `json:"analysis_time"`
}

// SectionInfo 节区信息
type SectionInfo struct {
	Name      string `json:"name"`
	VirtualAddress uint64 `json:"virtual_address"`
	VirtualSize    uint64 `json:"virtual_size"`
	RawSize        uint64 `json:"raw_size"`
	Entropy        float64 `json:"entropy"`
	Permissions    string `json:"permissions"` // rwx
}

// ImportInfo 导入函数信息
type ImportInfo struct {
	DLL      string `json:"dll"`
	Name     string `json:"name"`
	Address  uint64 `json:"address"`
}

// ExportInfo 导出函数信息
type ExportInfo struct {
	Name    string `json:"name"`
	Address uint64 `json:"address"`
	Ordinal uint   `json:"ordinal"`
}

// FunctionInfo 函数信息
type FunctionInfo struct {
	Name      string `json:"name"`
	Address   uint64 `json:"address"`
	Size      uint64 `json:"size"`
	Signature string `json:"signature,omitempty"`
}

// StringInfo 字符串信息
type StringInfo struct {
	Value  string `json:"value"`
	Address uint64 `json:"address"`
	Type   string `json:"type"` // ascii/unicode
}

// IDAServer IDA 服务器实例
type IDAServer struct {
	gorm.Model
	Name        string    `gorm:"size:100" json:"name"`
	Endpoint    string    `gorm:"size:255" json:"endpoint"`    // MCP WebSocket URL
	Status      string    `gorm:"size:20;default:offline" json:"status"` // online/offline/busy
	CurrentTask uint      `gorm:"default:0" json:"current_task"`
	LastPing    time.Time `json:"last_ping"`
}

// TableName 指定表名
func (IDAServer) TableName() string {
	return "ida_servers"
}

// UploadRequest 上传请求
type UploadRequest struct {
	AgentID     uint   `form:"agent_id" binding:"required"`
	Description string `form:"description"`
}

// TaskStatusResponse 任务状态响应
type TaskStatusResponse struct {
	ID          uint       `json:"id"`
	FileName    string     `json:"file_name"`
	FileHash    string     `json:"file_hash"`
	FileType    string     `json:"file_type"`
	FileSize    int64      `json:"file_size"`
	Status      string     `json:"status"`
	Progress    int        `json:"progress"`
	ErrorMessage string    `json:"error_message,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
