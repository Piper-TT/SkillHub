package handlers

import (
	"net/http"
	"skillhub/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MCPHandler MCP 处理器
type MCPHandler struct {
	svc *service.MCPService
}

// NewMCPHandler 创建 MCP Handler
func NewMCPHandler(svc *service.MCPService) *MCPHandler {
	return &MCPHandler{svc: svc}
}

// GetServers 获取服务器列表
func (h *MCPHandler) GetServers(c *gin.Context) {
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

// GetServerByID 获取单个服务器详情
func (h *MCPHandler) GetServerByID(c *gin.Context) {
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
func (h *MCPHandler) GetCategories(c *gin.Context) {
	categories, err := h.svc.GetCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// GetStats 获取统计数据
func (h *MCPHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetTop50 获取 TOP 50
func (h *MCPHandler) GetTop50(c *gin.Context) {
	servers := h.svc.GetTopByStars(50)
	c.JSON(http.StatusOK, gin.H{"servers": servers})
}
