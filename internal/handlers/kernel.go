package handlers

import (
	"io"
	"net/http"
	"skillhub/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
)

// KernelHandler 内核适配服务反向代理
type KernelHandler struct {
	serviceURL string
	client     *http.Client
}

// NewKernelHandler 创建内核适配代理处理器
func NewKernelHandler(cfg *config.Config) *KernelHandler {
	return &KernelHandler{
		serviceURL: strings.TrimRight(cfg.Kernel.ServiceURL, "/"),
		client: &http.Client{
			Timeout: 0, // 不超时，支持大文件下载
		},
	}
}

// Proxy 通用反向代理，将 /api/kernel/* 转发到 kernel-build 服务
func (h *KernelHandler) Proxy(c *gin.Context) {
	// 构建目标 URL：去掉 /api/kernel 前缀，拼到 service_url
	// /api/kernel/health -> service_url/health
	// /api/kernel/api/v1/check-adaptation -> service_url/api/v1/check-adaptation
	targetPath := strings.TrimPrefix(c.Request.URL.Path, "/api/kernel")
	if targetPath == "" {
		targetPath = "/"
	}
	targetURL := h.serviceURL + targetPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// 创建新请求
	var body io.Reader
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		body = c.Request.Body
		defer c.Request.Body.Close()
	}

	req, err := http.NewRequest(c.Request.Method, targetURL, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
		return
	}

	// 复制请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 发送请求
	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "kernel service unavailable",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	// 复制状态码和响应体
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
