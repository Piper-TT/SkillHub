package main

import (
	"context"
	"embed"
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

	return db, nil
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
