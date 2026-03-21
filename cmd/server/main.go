package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"skillhub/internal/config"
	"skillhub/internal/handlers"
	"skillhub/internal/middleware"
	"skillhub/internal/models"
	"skillhub/internal/repository"
	"skillhub/internal/service"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed templates/* static/*
var webFS embed.FS

func main() {
	// 设置工作目录
	if err := os.Chdir(getWorkDir()); err != nil {
		fmt.Printf("Warning: failed to change working directory: %v\n", err)
	}

	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	if err := middleware.InitLogger(&cfg.Logging); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer middleware.Sync()

	log := middleware.GetLogger()
	log.Info("Starting SkillHub server...")

	// 3. 初始化数据库
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	log.Infof("Database initialized: %s", cfg.Database.Path)

	// 4. 初始化依赖
	skillRepo := repository.NewSkillRepository(db)
	skillService := service.NewSkillService(skillRepo, cfg.Server.UploadDir)
	service.SetGlobalService(skillService)

	// 初始化 MCP 依赖
	mcpRepo := repository.NewMCPRepository(db)
	mcpService := service.NewMCPService(mcpRepo)
	mcpHandler := handlers.NewMCPHandler(mcpService)

	// 初始化刷新服务
	refreshService := service.NewRefreshService(skillRepo)
	refreshService.SetLogger(log)

	skillHandler := handlers.NewSkillHandler(skillService)

	// 5. 设置 Gin
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 中间件
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.Security())
	r.Use(middleware.RequestSizeLimit(cfg.Server.MaxUploadSize))
	r.Use(middleware.UploadSecurity())

	// 设置最大上传大小
	r.MaxMultipartMemory = cfg.Server.MaxUploadSize

	// 静态文件
	staticFS, _ := fs.Sub(webFS, "static")
	r.StaticFS("/static", http.FS(staticFS))

	// 上传文件目录（用于下载）
	r.Static("/uploads", cfg.Server.UploadDir)

	// API 路由
	api := r.Group("/api")
	{
		// 健康检查
		api.GET("/health", skillHandler.HealthCheck)

		// 查询接口
		api.GET("/top50", skillHandler.GetTop50)
		api.GET("/skills", skillHandler.GetAllSkills)
		api.GET("/categories", skillHandler.GetCategories)
		api.GET("/skills/:id", skillHandler.GetSkillByID)
		api.GET("/stats", skillHandler.GetStats)

		// 上传/下载接口
		api.POST("/skills/upload", skillHandler.UploadSkill)
		api.GET("/skills/:id/download", skillHandler.DownloadSkill)

		// 管理接口
		api.PUT("/skills/:id", skillHandler.UpdateSkill)
		api.DELETE("/skills/:id", skillHandler.DeleteSkill)

		// 初始化（仅用于开发测试）
		api.POST("/init", skillHandler.InitUploadDir)

		// MCP API 路由
		api.GET("/mcp", mcpHandler.GetServers)
		api.GET("/mcp/categories", mcpHandler.GetCategories)
		api.GET("/mcp/stats", mcpHandler.GetStats)
		api.GET("/mcp/:id", mcpHandler.GetServerByID)
	}

	// Portal 主页
	r.GET("/", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/portal.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load portal template")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// SkillHub 界面
	r.GET("/skills", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load template")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// MCPHub 界面
	r.GET("/mcp", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/mcp.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load mcp template")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// 6. 启动服务器（优雅关闭）
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器（在 goroutine 中）
	go func() {
		log.Infof("Server listening on http://localhost%s", addr)
		printBanner(cfg.Server.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 启动后台数据刷新服务（每24小时刷新一次）
	ctxRefresh, cancelRefresh := context.WithCancel(context.Background())
	defer cancelRefresh()
	refreshService.StartBackgroundRefresh(ctxRefresh, 24*time.Hour)

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// 优雅关闭，最多等待 5 秒
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited")
}

// initDatabase 初始化数据库
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	gormConfig := &gorm.Config{}
	if cfg.Logging.Level != "debug" {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	switch cfg.Database.Type {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(cfg.Database.Path), gormConfig)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&models.Skill{}, &models.MCPServer{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 初始化种子数据
	if err := seedData(db); err != nil {
		return nil, fmt.Errorf("failed to seed data: %w", err)
	}

	// 初始化 MCP 种子数据
	if err := seedMCPData(db); err != nil {
		return nil, fmt.Errorf("failed to seed MCP data: %w", err)
	}

	return db, nil
}

// seedData 初始化种子数据 (从 JSON 文件加载)
func seedData(db *gorm.DB) error {
	var count int64
	db.Model(&models.Skill{}).Count(&count)
	if count > 0 {
		return nil // 已有数据，跳过
	}

	// 从 JSON 文件读取技能数据
	skills, err := loadSkillsFromJSON()
	if err != nil {
		return err
	}

	return db.Create(&skills).Error
}

// loadSkillsFromJSON 从 JSON 文件加载技能数据
func loadSkillsFromJSON() ([]models.Skill, error) {
	// 尝试多个可能的路径
	paths := []string{
		"internal/data/skills.json",
		"./internal/data/skills.json",
		filepath.Join(getWorkDir(), "internal/data/skills.json"),
	}

	var data []byte
	var err error

	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read skills.json: %w", err)
	}

	// 解析 JSON
	var jsonSkills []struct {
		Slug    string  `json:"slug"`
		Name    string  `json:"name"`
		Summary string  `json:"summary"`
		Score   float64 `json:"score"`
	}

	if err := json.Unmarshal(data, &jsonSkills); err != nil {
		return nil, fmt.Errorf("failed to parse skills.json: %w", err)
	}

	// 转换为 Skill 模型
	skills := make([]models.Skill, 0, len(jsonSkills))
	icons := []string{"🤖", "⚡", "🚀", "💡", "🔧", "📦", "🎯", "💻", "🔥", "⭐", "🌟", "💎", "🎨", "📊", "🔐", "📱", "🌐", "🔍", "📝", "🛠️"}

	for i, js := range jsonSkills {
		category := categorizeSkill(js.Name, js.Summary)
		icon := icons[len(js.Name)%len(icons)]
		downloads := 50000 - (i * 30)
		if downloads < 1000 {
			downloads = 1000
		}
		rating := 5
		if i >= 50 {
			rating = 4
		}
		if i >= 200 {
			rating = 3
		}

		skills = append(skills, models.Skill{
			Name:        js.Name,
			Slug:        js.Slug,
			Icon:        icon,
			Category:    category,
			Description: js.Summary,
			Downloads:   downloads,
			Rating:      rating,
			Verified:    i < 200,
			Accelerated: true,
			Safe:        true,
		})
	}

	return skills, nil
}

// categorizeSkill 根据名称和摘要分类技能
func categorizeSkill(name, summary string) string {
	text := strings.ToLower(name + " " + summary)

	if strings.Contains(text, "agent") || strings.Contains(text, "ai ") || strings.Contains(text, "智能") || strings.Contains(text, "llm") {
		return "AI智能"
	}
	if strings.Contains(text, "code") || strings.Contains(text, "dev") || strings.Contains(text, "开发") || strings.Contains(text, "git") || strings.Contains(text, "编程") {
		return "开发工具"
	}
	if strings.Contains(text, "browser") || strings.Contains(text, "web") || strings.Contains(text, "scrape") || strings.Contains(text, "浏览器") {
		return "浏览器自动化"
	}
	if strings.Contains(text, "security") || strings.Contains(text, "audit") || strings.Contains(text, "安全") || strings.Contains(text, "shield") {
		return "安全工具"
	}
	if strings.Contains(text, "data") || strings.Contains(text, "database") || strings.Contains(text, "数据") || strings.Contains(text, "memory") {
		return "数据管理"
	}
	if strings.Contains(text, "doc") || strings.Contains(text, "file") || strings.Contains(text, "文档") || strings.Contains(text, "pdf") {
		return "文档处理"
	}
	if strings.Contains(text, "search") || strings.Contains(text, "信息") || strings.Contains(text, "query") {
		return "信息处理"
	}
	if strings.Contains(text, "email") || strings.Contains(text, "chat") || strings.Contains(text, "办公") || strings.Contains(text, "slack") || strings.Contains(text, "discord") {
		return "办公协同"
	}
	if strings.Contains(text, "image") || strings.Contains(text, "video") || strings.Contains(text, "media") || strings.Contains(text, "多媒体") {
		return "多媒体"
	}
	if strings.Contains(text, "claude") || strings.Contains(text, "cursor") || strings.Contains(text, "copilot") {
		return "编程助手"
	}

	return "其他"
}

func printBanner(port int) {
	fmt.Println("")
	fmt.Println("=================================")
	fmt.Println("  SkillHub Server Started")
	fmt.Println("=================================")
	fmt.Printf("  Web UI:    http://localhost:%d\n", port)
	fmt.Printf("  Health:    http://localhost:%d/api/health\n", port)
	fmt.Printf("  Upload:    POST /api/skills/upload\n")
	fmt.Printf("  Download:  GET  /api/skills/:id/download\n")
	fmt.Println("=================================")
	fmt.Println("")
}

func getWorkDir() string {
	execPath, _ := os.Executable()
	dir := filepath.Dir(execPath)

	// 检查是否在 build 目录下
	if filepath.Base(dir) == "server" || filepath.Base(dir) == "build" {
		dir = filepath.Dir(dir)
	}

	// 查找 go.mod 文件
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "."
}

// seedMCPData 初始化 MCP 种子数据
func seedMCPData(db *gorm.DB) error {
	var count int64
	db.Model(&models.MCPServer{}).Count(&count)
	if count > 0 {
		return nil // 已有数据，跳过
	}

	// 从 JSON 文件读取 MCP 数据
	servers, err := loadMCPServersFromJSON()
	if err != nil {
		// 如果文件不存在，使用默认数据
		servers = getDefaultMCPServers()
	}

	return db.Create(&servers).Error
}

// loadMCPServersFromJSON 从 JSON 文件加载 MCP 数据
func loadMCPServersFromJSON() ([]models.MCPServer, error) {
	paths := []string{
		"internal/data/mcp_servers.json",
		"./internal/data/mcp_servers.json",
		filepath.Join(getWorkDir(), "internal/data/mcp_servers.json"),
	}

	var data []byte
	var err error

	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, err
	}

	var servers []models.MCPServer
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, err
	}

	return servers, nil
}

// getDefaultMCPServers 获取默认 MCP 服务器数据
func getDefaultMCPServers() []models.MCPServer {
	return []models.MCPServer{
		{
			Name:        "Filesystem MCP",
			Slug:        "filesystem",
			Icon:        "📁",
			Category:    "File System",
			Description: "安全文件系统操作，支持读写、搜索和管理文件",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem",
			Stars:       8500,
			Downloads:   50000,
			InstallCmd:  "npx @anthropic/mcp-server-filesystem",
			Config:      `{"mcpServers": {"filesystem": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-filesystem", "/path/to/allowed/dir"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "PostgreSQL MCP",
			Slug:        "postgresql",
			Icon:        "🗄️",
			Category:    "Database",
			Description: "只读 PostgreSQL 数据库访问，支持模式检查",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/postgres",
			Stars:       6200,
			Downloads:   35000,
			InstallCmd:  "npx @anthropic/mcp-server-postgres",
			Config:      `{"mcpServers": {"postgres": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-postgres", "postgresql://user:pass@localhost/db"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "GitHub MCP",
			Slug:        "github",
			Icon:        "🛠️",
			Category:    "Developer",
			Description: "GitHub API 集成，支持仓库、Issue、PR 等操作",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/github",
			Stars:       7800,
			Downloads:   42000,
			InstallCmd:  "npx @anthropic/mcp-server-github",
			Config:      `{"mcpServers": {"github": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-github"], "env": {"GITHUB_TOKEN": "your-token"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Puppeteer MCP",
			Slug:        "puppeteer",
			Icon:        "🌐",
			Category:    "Browser",
			Description: "浏览器自动化，支持截图、表单填写、页面导航",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/puppeteer",
			Stars:       5400,
			Downloads:   28000,
			InstallCmd:  "npx @anthropic/mcp-server-puppeteer",
			Config:      `{"mcpServers": {"puppeteer": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-puppeteer"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Brave Search MCP",
			Slug:        "brave-search",
			Icon:        "🔍",
			Category:    "Web Search",
			Description: "Brave 搜索 API 集成，支持网络搜索",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/brave-search",
			Stars:       4800,
			Downloads:   25000,
			InstallCmd:  "npx @anthropic/mcp-server-brave-search",
			Config:      `{"mcpServers": {"brave-search": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-brave-search"], "env": {"BRAVE_API_KEY": "your-key"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Slack MCP",
			Slug:        "slack",
			Icon:        "💬",
			Category:    "Communication",
			Description: "Slack API 集成，支持消息发送和频道管理",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/slack",
			Stars:       3200,
			Downloads:   18000,
			InstallCmd:  "npx @anthropic/mcp-server-slack",
			Config:      `{"mcpServers": {"slack": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-slack"], "env": {"SLACK_BOT_TOKEN": "your-token"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Google Maps MCP",
			Slug:        "google-maps",
			Icon:        "🗺️",
			Category:    "Data",
			Description: "Google Maps API 集成，支持地址搜索和路线规划",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/google-maps",
			Stars:       2800,
			Downloads:   15000,
			InstallCmd:  "npx @anthropic/mcp-server-google-maps",
			Config:      `{"mcpServers": {"google-maps": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-google-maps"], "env": {"GOOGLE_MAPS_API_KEY": "your-key"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Memory MCP",
			Slug:        "memory",
			Icon:        "🧠",
			Category:    "AI Tools",
			Description: "持久化记忆存储，让 AI 记住对话历史",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/memory",
			Stars:       4500,
			Downloads:   22000,
			InstallCmd:  "npx @anthropic/mcp-server-memory",
			Config:      `{"mcpServers": {"memory": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-memory"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Fetch MCP",
			Slug:        "fetch",
			Icon:        "📡",
			Category:    "Web Search",
			Description: "HTTP 请求工具，支持获取网页内容",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/fetch",
			Stars:       3600,
			Downloads:   19000,
			InstallCmd:  "npx @anthropic/mcp-server-fetch",
			Config:      `{"mcpServers": {"fetch": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-fetch"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "SQLite MCP",
			Slug:        "sqlite",
			Icon:        "🗃️",
			Category:    "Database",
			Description: "SQLite 数据库操作，支持查询和修改",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/sqlite",
			Stars:       2900,
			Downloads:   16000,
			InstallCmd:  "npx @anthropic/mcp-server-sqlite",
			Config:      `{"mcpServers": {"sqlite": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-sqlite", "--db-path", "/path/to/db.sqlite"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Git MCP",
			Slug:        "git",
			Icon:        "📝",
			Category:    "Developer",
			Description: "Git 操作工具，支持提交、分支、差异对比",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/git",
			Stars:       4100,
			Downloads:   21000,
			InstallCmd:  "npx @anthropic/mcp-server-git",
			Config:      `{"mcpServers": {"git": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-git", "--repository", "/path/to/repo"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Sequential Thinking MCP",
			Slug:        "sequential-thinking",
			Icon:        "💭",
			Category:    "AI Tools",
			Description: "结构化思考工具，帮助 AI 进行复杂推理",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/sequentialthinking",
			Stars:       3800,
			Downloads:   17000,
			InstallCmd:  "npx @anthropic/mcp-server-sequential-thinking",
			Config:      `{"mcpServers": {"sequential-thinking": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-sequential-thinking"]}}}`,
			Verified:    true,
			Official:    true,
		},
	}
}
