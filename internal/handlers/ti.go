package handlers

import (
	"io"
	"net/http"
	"skillhub/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
)

// TIHandler 威胁情报查询服务反向代理
type TIHandler struct {
	serviceURL string
	client     *http.Client
}

// NewTIHandler 创建威胁情报代理处理器
func NewTIHandler(cfg *config.Config) *TIHandler {
	return &TIHandler{
		serviceURL: strings.TrimRight(cfg.TI.ServiceURL, "/"),
		client: &http.Client{
			Timeout: 0,
		},
	}
}

// Proxy 通用反向代理，将 /api/ti/* 转发到 tiserver 服务
func (h *TIHandler) Proxy(c *gin.Context) {
	targetPath := strings.TrimPrefix(c.Request.URL.Path, "/api/ti")
	if targetPath == "" {
		targetPath = "/"
	}
	targetURL := h.serviceURL + targetPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

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

	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ti service unavailable",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
