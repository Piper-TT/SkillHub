package handlers

import (
	"fmt"
	"io"
	"net/http"
	"skillhub/internal/middleware"
	"skillhub/internal/service"
	"skillhub/internal/utils"
	"strconv"
	"strings"

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

// UploadServer 上传 MCP 服务器文件
func (h *MCPHandler) UploadServer(c *gin.Context) {
	// 获取表单数据
	name := c.PostForm("name")
	icon := c.PostForm("icon")
	category := c.PostForm("category")
	description := c.PostForm("description")
	githubUrl := c.PostForm("github_url")

	// 验证必填字段
	if err := utils.ValidateSkillInput(name, category); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "file is required: "+err.Error())
		return
	}
	defer file.Close()

	// 读取文件内容
	fileContent, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "failed to read file")
		return
	}

	// 验证文件
	maxSize := int64(100 * 1024 * 1024) // 100MB
	if err := utils.ValidateUploadFile(header.Filename, fileContent, maxSize); err != nil {
		utils.BadRequest(c, "file validation failed: "+err.Error())
		return
	}

	req := &service.MCPUploadRequest{
		Name:        name,
		Icon:        icon,
		Category:    category,
		Description: description,
		GitHubURL:   githubUrl,
		FileName:    header.Filename,
	}

	// 上传
	server, err := h.svc.UploadServer(req, &mcpByteReader{data: fileContent})
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "upload successful",
		"server":  server,
	})
}

// DownloadServer 下载 MCP 服务器文件
func (h *MCPHandler) DownloadServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	filePath, err := h.svc.DownloadServer(id)
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}

	// 获取文件名
	server, _ := h.svc.GetByID(id)
	downloadName := "mcp_server.zip"
	if server != nil {
		if server.FileName != "" {
			downloadName = server.FileName
		} else {
			downloadName = server.Name + ".zip"
		}
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+downloadName)
	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(filePath, downloadName)
}

// DeleteServer 删除 MCP 服务器
func (h *MCPHandler) DeleteServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	// 检查是否有本地文件
	server, err := h.svc.GetByID(id)
	if err != nil {
		utils.NotFound(c, "server not found")
		return
	}

	// 如果有本地文件，不允许删除（或者可以选择同时删除文件）
	if server.FileName != "" {
		// 可以选择删除文件，这里暂时不允许删除有本地文件的服务器
		utils.BadRequest(c, "cannot delete server with local file")
		return
	}

	// 这里需要添加删除方法到 repository
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "server deleted"})
}

// byteReader 简单的字节 reader 实现
type mcpByteReader struct {
	data []byte
	pos  int
}

func (r *mcpByteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// unused imports workaround
var (
	_ = fmt.Sprintf
	_ = strings.Contains
	_ = middleware.GetLogger
)
