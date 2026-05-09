package handlers

import (
	"net/http"
	"skillhub/internal/service"
	"skillhub/internal/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ServerHandler Server 处理器
type ServerHandler struct {
	svc *service.ServerService
}

// NewServerHandler 创建 Server Handler
func NewServerHandler(svc *service.ServerService) *ServerHandler {
	return &ServerHandler{svc: svc}
}

// GetServers 获取服务列表
func (h *ServerHandler) GetServers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	category := c.Query("category")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "stars")

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	response, err := h.svc.FilterServers(page, perPage, category, search, sortBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetServerByID 获取单个服务详情
func (h *ServerHandler) GetServerByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	server, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	c.JSON(http.StatusOK, server)
}

// GetCategories 获取分类列表
func (h *ServerHandler) GetCategories(c *gin.Context) {
	categories, err := h.svc.GetCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// GetStats 获取统计数据
func (h *ServerHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetTop50 获取 TOP 50
func (h *ServerHandler) GetTop50(c *gin.Context) {
	servers := h.svc.GetTopByStars(50)
	c.JSON(http.StatusOK, gin.H{"servers": servers})
}

// UploadServer 注册 Web 服务
func (h *ServerHandler) UploadServer(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Icon        string `json:"icon"`
		Category    string `json:"category" binding:"required"`
		Description string `json:"description"`
		GitHubURL   string `json:"github_url"`
		URL         string `json:"url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	uploadReq := &service.ServerUploadRequest{
		Name:        req.Name,
		Icon:        req.Icon,
		Category:    req.Category,
		Description: req.Description,
		GitHubURL:   req.GitHubURL,
		URL:         req.URL,
	}

	server, err := h.svc.UploadServer(uploadReq)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "注册成功",
		"server":  server,
	})
}

// DownloadServer 已移除 — 服务通过 ServiceURL 直接访问
func (h *ServerHandler) DownloadServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	server, err := h.svc.GetByID(id)
	if err != nil {
		utils.NotFound(c, "server not found")
		return
	}

	if server.ServiceURL == "" {
		utils.NotFound(c, "server has no service URL")
		return
	}

	c.Redirect(http.StatusFound, server.ServiceURL)
}

// DeleteServer 删除服务
func (h *ServerHandler) DeleteServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "server deleted"})
}
