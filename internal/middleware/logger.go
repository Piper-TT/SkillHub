package middleware

import (
	"time"

	"skillhub/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

// InitLogger 初始化日志系统
func InitLogger(cfg *config.LoggingConfig) error {
	var zapConfig zap.Config

	if cfg.Format == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// 设置日志级别
	switch cfg.Level {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	l, err := zapConfig.Build()
	if err != nil {
		return err
	}

	logger = l.Sugar()
	return nil
}

// GetLogger 获取日志实例
func GetLogger() *zap.SugaredLogger {
	return logger
}

// Sync 同步日志缓冲
func Sync() {
	if logger != nil {
		logger.Sync()
	}
}

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 请求完成后记录日志
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		if query != "" {
			path = path + "?" + query
		}

		// 根据状态码选择日志级别
		switch {
		case status >= 500:
			logger.Errorf("[%s] %s %d %v %s | Errors: %s",
				method, path, status, latency, clientIP, c.Errors.String())
		case status >= 400:
			logger.Warnf("[%s] %s %d %v %s",
				method, path, status, latency, clientIP)
		default:
			logger.Infof("[%s] %s %d %v %s",
				method, path, status, latency, clientIP)
		}
	}
}
