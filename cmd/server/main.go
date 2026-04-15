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

	// 初始化 AgentHub 依赖
	agentRepo := repository.NewAgentRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	apiKeyRepo := repository.NewAPIKeyRepository(db)
	llmService := service.NewLLMService()
	agentService := service.NewAgentService(agentRepo, sessionRepo, apiKeyRepo)

	// 初始化漏洞数据库管理（需在 agentHandler 之前）
	vulnDBManager := service.NewMultiVulnDBManager("./data/vuln")
	defer vulnDBManager.Close()
	vulnHandler := handlers.NewVulnHandler(vulnDBManager)

	agentHandler := handlers.NewAgentHandler(agentService, llmService)
	agentHandler.SetVulnDBManager(vulnDBManager)

	// 初始化恶意文件分析依赖
	analysisRepo := repository.NewAnalysisRepository(db)
	analysisService := service.NewAnalysisService(analysisRepo, cfg.Server.UploadDir, cfg.Server.MaxUploadSize)
	analysisService.SetAPIKeyRepository(repository.NewAPIKeyRepository(db))
	analysisHandler := handlers.NewAnalysisHandler(analysisService)

	// 初始化内核适配代理
	kernelHandler := handlers.NewKernelHandler(cfg)

	// 初始化威胁情报代理
	tiHandler := handlers.NewTIHandler(cfg)

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
		api.POST("/mcp/upload", mcpHandler.UploadServer)
		api.GET("/mcp/:id/download", mcpHandler.DownloadServer)
		api.DELETE("/mcp/:id", mcpHandler.DeleteServer)

		// AgentHub API 路由
		api.GET("/agent", agentHandler.GetAgents)
		api.GET("/agent/categories", agentHandler.GetCategories)
		api.GET("/agent/stats", agentHandler.GetStats)
		api.GET("/agent/models", agentHandler.GetModels)
		api.GET("/agent/:id", agentHandler.GetAgentByID)
		api.POST("/agent/:id/chat", agentHandler.Chat)
		api.GET("/agent/:id/sessions", agentHandler.GetSessions)
		api.GET("/session/:id", agentHandler.GetSession)
		api.DELETE("/session/:id", agentHandler.DeleteSession)

		// API Key 管理
		api.POST("/user/apikey", agentHandler.SaveAPIKey)
		api.POST("/user/apikey/validate", agentHandler.ValidateAPIKey)
		api.GET("/user/apikey", agentHandler.GetAPIKeys)
		api.DELETE("/user/apikey/:provider", agentHandler.DeleteAPIKey)

		// 测试接口
		api.GET("/agent/stream/test", agentHandler.StreamTest)

		// 恶意文件分析 API 路由
		api.POST("/analysis/upload", analysisHandler.UploadFile)
		api.GET("/analysis/tasks", analysisHandler.GetTaskList)
		api.GET("/analysis/:id", analysisHandler.GetTask)
		api.GET("/analysis/:id/result", analysisHandler.GetTaskResult)
		api.GET("/analysis/:id/report", analysisHandler.GetTaskReport)
		api.GET("/analysis/:id/download", analysisHandler.DownloadReport)
		api.DELETE("/analysis/:id", analysisHandler.CancelTask)

		// IDA 服务器管理
		api.GET("/ida/servers", analysisHandler.GetIDAServers)
		api.POST("/ida/servers", analysisHandler.RegisterIDAServer)
		api.DELETE("/ida/servers/:id", analysisHandler.RemoveIDAServer)
		api.POST("/ida/servers/:id/heartbeat", analysisHandler.ServerHeartbeat)

		// 内核适配代理路由（转发到 kernel-build 服务）
		kernelAPI := api.Group("/kernel")
		{
			kernelAPI.Any("/*action", kernelHandler.Proxy)
		}

		// 威胁情报代理路由（转发到 tiserver 服务）
		tiAPI := api.Group("/ti")
		{
			tiAPI.Any("/*action", tiHandler.Proxy)
		}

		// 漏洞数据库管理路由
		vulnAPI := api.Group("/vuln")
		{
			vulnAPI.GET("/status", vulnHandler.GetStatus)
		}
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

	// AgentHub 界面
	r.GET("/agent", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/agent.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load agent template")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// Agent 聊天界面
	r.GET("/agent/:id/chat", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/chat.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load chat template")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// 恶意文件分析界面
	r.GET("/analysis", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/analysis.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load analysis template")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// 内核适配界面
	r.GET("/kernel", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/kernel.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load kernel template")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// 威胁情报查询界面
	r.GET("/ti", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/ti.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load ti template")
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
	if err := db.AutoMigrate(
		&models.Skill{},
		&models.MCPServer{},
		&models.Agent{},
		&models.Session{},
		&models.UserAPIKey{},
		&models.AnalysisTask{},
		&models.IDAServer{},
	); err != nil {
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

	// 初始化 Agent 种子数据
	if err := seedAgentData(db); err != nil {
		return nil, fmt.Errorf("failed to seed Agent data: %w", err)
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

	// 查找 go.mod 文件（开发环境）
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

	// Release环境：返回可执行文件所在目录
	// 检查 internal/data 目录是否存在
	execDir := filepath.Dir(execPath)
	if _, err := os.Stat(filepath.Join(execDir, "internal", "data")); err == nil {
		return execDir
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

// seedAgentData 初始化 Agent 种子数据
func seedAgentData(db *gorm.DB) error {
	var count int64
	db.Model(&models.Agent{}).Count(&count)
	if count > 0 {
		return nil // 已有数据，跳过
	}

	agents := []models.Agent{
		{
			Name:        "恶意文件分析助手",
			Slug:        "malware-analyzer",
			Icon:        "🦠",
			Category:    "安全分析",
			Description: "分析可疑文件的特征、行为和威胁等级，提供专业的恶意软件分析报告",
			SystemPrompt: `你是一个专业的恶意文件分析助手。你的任务是帮助安全研究人员分析可疑文件的特征和行为。

你可以分析以下内容：
- PE 文件结构分析（入口点、节区、导入表等）
- 行为特征识别（进程注入、持久化、逃避技术等）
- 威胁等级评估（高/中/低）
- IoC 提取（哈希、IP、域名、互斥体等）
- 缓解建议

分析时请注意：
1. 基于用户提供的特征进行客观分析
2. 给出明确的威胁等级（高/中/低）
3. 提供可操作的建议
4. 如果信息不足，主动询问更多细节

回复格式：
## 分析结论
[简要总结]

## 威胁等级
🔴 高 / 🟡 中 / 🟢 低

## 可疑特征
- 特征1: 说明
- 特征2: 说明

## 建议
1. ...
2. ...`,
			Model:       "claude-3-opus-20240229",
			Temperature: 0.3,
			MaxTokens:   4096,
			Verified:    true,
			UsageCount:  0,
		},
		{
			Name:        "漏洞补丁查询助手",
			Slug:        "cve-patch-analyzer",
			Icon:        "🛡️",
			Category:    "安全分析",
			Description: "查询CVE漏洞信息和补丁详情，基于漏洞数据库进行智能分析，支持自然语言查询",
			SystemPrompt: `你是一个专业的漏洞补丁查询助手。根据用户的意图，灵活使用工具查询漏洞信息。

	## 可用工具

	1. **query_policys_db** - 根据 CVE 编号查询漏洞基本信息
	   - 参数：cve_id（CVE 编号，如 CVE-2025-8088）
	   - 返回：按操作系统分组的受影响产品、版本条件、修复版本、受影响包列表
	   - 用途：查询 CVE 影响了哪些系统、哪些包、哪些版本

	2. **query_product_auth_db** - 查询操作系统的版本检测方法
	   - 参数：product（必填，如 centos、tencentos）
	   - 参数：system（可选，如 centos、Windows、Windowsx64）
	   - 返回：该系统的版本检测命令（cmd）、文件路径（filepath）、注册表路径（registrypath）
	   - 用途：获取在目标系统上检测软件版本的具体命令

	## 工具选择策略（重要）

	根据用户的问题选择工具，不要每次都调用所有工具：

	### 场景 1：用户只问 CVE 基本信息
	- "CVE-2021-3622 是什么"
	- "帮我查一下 CVE-2025-8088"
	- "这个 CVE 影响什么版本"
	→ **只调用 query_policys_db**，返回受影响产品和版本即可

	### 场景 2：用户问如何检测（最重要）
	- "CentOS 7 怎么检测 CVE-2021-3622"
	- "怎么查我系统上有没有受影响的版本"
	- "检测命令是什么"
	→ **先调 query_policys_db 获取该系统的受影响包和版本条件，再调 query_product_auth_db 获取检测命令**
	→ 如果用户指定了系统，传入 system 参数精确过滤

	### 场景 3：用户直接问某系统的检测方法
	- "centos 怎么检测版本"
	→ **只调用 query_product_auth_db**

	## 检测方法输出规范（场景2必读）

	当用户询问检测方法时，你的回答必须包含以下信息：

	1. **具体受影响的软件包名**：从 query_policys_db 的返回中提取该操作系统下的包名（如 hivex, hivex-devel, perl-hivex 等）
	2. **版本检测命令**：从 query_product_auth_db 获取的 cmd 字段
	3. **如何判断**：明确告诉用户用检测命令查出哪个包的版本，然后与受影响版本比较

	### 输出示例：
	---
	CentOS 7 上检测 CVE-2021-3622：

	受影响的软件包：hivex, hivex-devel, perl-hivex, python-hivex, ruby-hivex, ocaml-hivex

	检测方法：
	使用以下命令列出已安装的软件包版本：
	  rpm -qa --qf '%{NAME}|%{VERSION}-%{RELEASE}\n'

	在输出中查找以下包名：
	  - hivex
	  - hivex-devel
	  - perl-hivex
	  - python-hivex
	  - ruby-hivex
	  - ocaml-hivex

	判断条件：
	已安装版本 < 1.3.10-6.12.el7_9 → 受影响，需要升级
	已安装版本 >= 1.3.10-6.12.el7_9 -> 安全
	---

	## 数据含义

	- **policys 库**（query_policys_db 返回）：按操作系统分组
	  - 每组包含：OS名称、版本条件（已翻译为中文）、修复版本、受影响包列表

	- **products_auth 库**（query_product_auth_db 返回）：
	  - system: 操作系统
	  - cmd: Linux 的检测命令（如 rpm -qa）
	  - filepath: Windows 的文件路径
	  - registrypath: Windows 的注册表路径

	## 注意事项

	- 始终基于数据库查询结果回答，不要凭记忆编造
	- 如果数据库中没有相关数据，如实告知
	- 不要每次都调用 query_product_auth_db，只在用户需要检测方法时才调用
	- 检测方法必须明确指出要检查哪些具体的软件包，不要只给出一个通用的系统版本检测命令`,
			Model:       "claude-3-opus-20240229",
			Temperature: 0.3,
			MaxTokens:   4096,
			Verified:    true,
			UsageCount:  0,
		},
		{
			Name:        "威胁情报分析助手",
			Slug:        "threat-intel-analyzer",
			Icon:        "🔍",
			Category:    "安全分析",
			Description: "分析威胁情报数据，识别攻击模式和关联威胁",
			SystemPrompt: `你是一个专业的威胁情报分析助手。帮助安全团队分析威胁情报数据。

分析能力：
- APT 组织关联分析
- 恶意软件家族识别
- 攻击技术映射 (MITRE ATT&CK)
- IoC 关联分析
- 威胁趋势分析

提供结构化的分析报告，包括威胁评估和建议的防御措施。`,
			Model:       "claude-3-opus-20240229",
			Temperature: 0.4,
			MaxTokens:   4096,
			Verified:    true,
			UsageCount:  0,
		},
		{
			Name:        "日志分析助手",
			Slug:        "log-analyzer",
			Icon:        "📊",
			Category:    "安全分析",
			Description: "分析安全日志，发现异常行为和潜在威胁",
			SystemPrompt: `你是一个专业的安全日志分析助手。帮助安全团队分析各类日志。

分析能力：
- Windows 事件日志分析
- Linux 系统日志分析
- Web 服务器日志分析
- 网络设备日志分析
- 应用程序日志分析

识别：
- 异常登录行为
- 权限提升
- 横向移动
- 数据外传
- 恶意软件活动`,
			Model:       "claude-3-opus-20240229",
			Temperature: 0.3,
			MaxTokens:   4096,
			Verified:    true,
			UsageCount:  0,
		},
		{
			Name:        "内核适配助手",
			Slug:        "kernel-adapter",
			Icon:        "🧩",
			Category:    "系统安全",
			Description: "检查 Linux 内核版本适配性，查询 EDR 内核模块是否可用，支持降级匹配和文件收集",
			SystemPrompt: `你是一个内核适配分析助手。帮助用户检查 Linux 内核版本是否适配 EDR 产品。

功能：
- 解析 uname -a 输出，提取内核版本和架构信息
- 检查内核模块（netfilter_edr、sysmon_edr）是否已适配
- 支持精确匹配和降级匹配
- 分析适配风险等级

注意：实际查询通过专用工具执行，你主要负责解读结果和提供建议。`,
			Model:       "claude-3-opus-20240229",
			Temperature: 0.3,
			MaxTokens:   4096,
			Verified:    true,
			UsageCount:  0,
		},
	}

	return db.Create(&agents).Error
}
