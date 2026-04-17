package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/glebarez/go-sqlite"
)

// MultiVulnDBManager 多漏洞数据库管理器
type MultiVulnDBManager struct {
	dataDir string
	dbs     map[string]*sql.DB
	mu      sync.RWMutex
}

// NewMultiVulnDBManager 创建多数据库管理器
// dataDir 为存放 .db 文件的目录（如 ./data/vuln/）
func NewMultiVulnDBManager(dataDir string) *MultiVulnDBManager {
	mgr := &MultiVulnDBManager{
		dataDir: dataDir,
		dbs:     make(map[string]*sql.DB),
	}
	os.MkdirAll(dataDir, 0755)
	mgr.tryOpenAll()
	return mgr
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

// dataDir 实际指向项目根目录的 data/ 目录
const vulnDataDir = "./data"

// tryOpenAll 尝试打开所有已存在的数据库
func (m *MultiVulnDBManager) tryOpenAll() {
	fmt.Printf("[VulnDB] dataDir: %s\n", m.dataDir)
	for name, filename := range dbFiles {
		dbPath := filepath.Join(m.dataDir, filename)
		if _, err := os.Stat(dbPath); err != nil {
			fmt.Printf("[VulnDB] %s: file not found (%s)\n", name, dbPath)
			continue
		}
		fmt.Printf("[VulnDB] %s: found (%s)\n", name, dbPath)
		db, err := sql.Open("sqlite", dbPath+"?mode=ro")
		if err != nil {
			continue
		}
		if err := db.Ping(); err != nil {
			db.Close()
			continue
		}
		m.dbs[name] = db
	}
}

// IsAvailable 检查指定数据库是否可用
func (m *MultiVulnDBManager) IsAvailable(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.dbs[name]
	return ok
}

// AnyAvailable 检查是否至少有一个数据库可用
func (m *MultiVulnDBManager) AnyAvailable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.dbs) > 0
}

// GetDB 获取指定数据库连接
func (m *MultiVulnDBManager) GetDB(name string) *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dbs[name]
}

// Reload 重新加载指定数据库
func (m *MultiVulnDBManager) Reload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if old, ok := m.dbs[name]; ok {
		old.Close()
		delete(m.dbs, name)
	}

	filename, ok := dbFiles[name]
	if !ok {
		return fmt.Errorf("未知数据库: %s", name)
	}

	dbPath := filepath.Join(m.dataDir, filename)
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("数据库文件不存在: %s", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	m.dbs[name] = db
	return nil
}

// ReloadAll 重新加载所有数据库
func (m *MultiVulnDBManager) ReloadAll() {
	for name := range dbFiles {
		m.Reload(name)
	}
}

// Close 关闭所有数据库连接
func (m *MultiVulnDBManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, db := range m.dbs {
		db.Close()
	}
	m.dbs = make(map[string]*sql.DB)
}

// GetStatus 获取所有数据库状态
func (m *MultiVulnDBManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]interface{})
	for name := range dbFiles {
		db, ok := m.dbs[name]
		if !ok {
			status[name] = map[string]interface{}{
				"available": false,
			}
			continue
		}
		var tableCount int
		db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
		status[name] = map[string]interface{}{
			"available":   true,
			"table_count": tableCount,
		}
	}

	// 读取漏洞库版本号
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
