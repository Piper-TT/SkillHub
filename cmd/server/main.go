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
	}

	// 首页
	r.GET("/", func(c *gin.Context) {
		data, err := webFS.ReadFile("templates/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load template")
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
	if err := db.AutoMigrate(&models.Skill{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 初始化种子数据
	if err := seedData(db); err != nil {
		return nil, fmt.Errorf("failed to seed data: %w", err)
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
