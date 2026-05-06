package service

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

// MultiVulnDBManager 多漏洞数据库管理器（每次查询打开新连接，用完即关，不缓存）
type MultiVulnDBManager struct {
	dataDir string
}

// NewMultiVulnDBManager 创建多数据库管理器
func NewMultiVulnDBManager(dataDir string) *MultiVulnDBManager {
	os.MkdirAll(dataDir, 0755)
	return &MultiVulnDBManager{dataDir: dataDir}
}

// 数据库名称常量
const (
	DBPolicys     = "policys"
	DBProductAuth = "products_auth"
)

// dbFiles 数据库文件名映射（相对于 data/ 目录）
var dbFiles = map[string]string{
	DBPolicys:     "vul-center\\vul\\policys.db",
	DBProductAuth: "vul-agent\\vul\\products_auth.db",
}

// IsAvailable 检查指定数据库文件是否存在且可打开
func (m *MultiVulnDBManager) IsAvailable(name string) bool {
	filename, ok := dbFiles[name]
	if !ok {
		return false
	}
	dbPath := filepath.Join(m.dataDir, filename)
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}

// AnyAvailable 检查是否至少有一个数据库可用
func (m *MultiVulnDBManager) AnyAvailable() bool {
	for name := range dbFiles {
		if m.IsAvailable(name) {
			return true
		}
	}
	return false
}

// GetDB 打开指定数据库的新连接（调用方必须 defer db.Close()）
func (m *MultiVulnDBManager) GetDB(name string) *sql.DB {
	filename, ok := dbFiles[name]
	if !ok {
		return nil
	}
	dbPath := filepath.Join(m.dataDir, filename)
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil
	}
	return db
}

// Close 无操作（不再缓存连接）
func (m *MultiVulnDBManager) Close() {}

// GetStatus 获取所有数据库状态
func (m *MultiVulnDBManager) GetStatus() map[string]interface{} {
	status := make(map[string]interface{})
	for name := range dbFiles {
		db := m.GetDB(name)
		if db == nil {
			status[name] = map[string]interface{}{
				"available": false,
			}
			continue
		}
		var tableCount int
		db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
		db.Close()
		status[name] = map[string]interface{}{
			"available":   true,
			"table_count": tableCount,
		}
	}

	status["vuln_version"] = m.readVulnVersion()
	return status
}

// readVulnVersion 读取漏洞库版本号
func (m *MultiVulnDBManager) readVulnVersion() string {
	ehashPath := filepath.Join(m.dataDir, "vul.pkg.ehash")
	data, err := os.ReadFile(ehashPath)
	if err != nil {
		return ""
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	return v.Version
}
