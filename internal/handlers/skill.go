package handlers

import (
	"fmt"
	"io"
	"net/http"
	"skillhub/internal/middleware"
	"skillhub/internal/models"
	"skillhub/internal/service"
	"skillhub/internal/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type SkillHandler struct {
	svc *service.SkillService
}

func NewSkillHandler(svc *service.SkillService) *SkillHandler {
	return &SkillHandler{
		svc: svc,
	}
}

// GetTop50 获取 TOP 50 技能
func (h *SkillHandler) GetTop50(c *gin.Context) {
	skills := h.svc.GetTop50()
	c.JSON(http.StatusOK, models.SkillListResponse{
		Total:   int64(len(skills)),
		Skills:  skills,
		Page:    1,
		PerPage: 50,
	})
}

// GetAllSkills 获取所有技能（分页）
func (h *SkillHandler) GetAllSkills(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	perPage, err := strconv.Atoi(c.DefaultQuery("per_page", "12"))
	if err != nil || perPage < 1 {
		perPage = 12
	}
	if perPage > 100 {
		perPage = 100 // 限制最大每页数量
	}

	category := c.Query("category")
	search := c.Query("search")

	result, err := h.svc.FilterSkills(page, perPage, category, search)
	if err != nil {
		utils.InternalError(c, "Failed to filter skills")
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetCategories 获取所有分类
func (h *SkillHandler) GetCategories(c *gin.Context) {
	categories := h.svc.GetCategories()
	c.JSON(http.StatusOK, categories)
}

// GetSkillByID 根据ID获取技能详情
func (h *SkillHandler) GetSkillByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	skill, err := h.svc.GetSkillByID(id)
	if err != nil {
		utils.NotFound(c, "skill not found")
		return
	}

	c.JSON(http.StatusOK, skill)
}

// GetStats 获取统计数据
func (h *SkillHandler) GetStats(c *gin.Context) {
	stats := h.svc.GetStats()
	c.JSON(http.StatusOK, stats)
}

// UploadSkill 上传 Skill 文件
func (h *SkillHandler) UploadSkill(c *gin.Context) {
	// 获取表单数据
	name := c.PostForm("name")
	icon := c.PostForm("icon")
	category := c.PostForm("category")
	description := c.PostForm("description")

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

	// 读取文件内容进行验证
	fileContent, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "failed to read file")
		return
	}

	// 验证文件（使用配置中的最大大小）
	cfg := middleware.GetLogger() // 这里需要配置，暂时使用固定值
	_ = cfg                       // 避免未使用警告

	// 使用默认的 100MB 限制
	maxSize := int64(100 * 1024 * 1024)
	if err := utils.ValidateUploadFile(header.Filename, fileContent, maxSize); err != nil {
		utils.BadRequest(c, "file validation failed: "+err.Error())
		return
	}

	req := &service.UploadRequest{
		Name:        name,
		Icon:        icon,
		Category:    category,
		Description: description,
		FileName:    header.Filename,
	}

	// 使用读取的文件内容上传
	skill, err := h.uploadWithContent(req, fileContent)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, models.UploadResponse{
		Success: true,
		Message: "upload successful",
		Skill:   skill,
	})
}

// uploadWithContent 使用文件内容上传
func (h *SkillHandler) uploadWithContent(req *service.UploadRequest, content []byte) (*models.Skill, error) {
	// 创建一个简单的 reader 包装
	return h.svc.UploadSkill(req, &byteReader{data: content})
}

// byteReader 简单的字节 reader 实现
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// DownloadSkill 下载 Skill 文件
func (h *SkillHandler) DownloadSkill(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	filePath, err := h.svc.DownloadSkill(id)
	if err != nil {
		// 检查是否包含 ClawHub 直接链接
		errMsg := err.Error()
		if strings.Contains(errMsg, "clawhub.ai") {
			// 返回 JSON 包含直接链接
			skill, _ := h.svc.GetSkillByID(id)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":         503,
				"message":      "ClawHub rate limit reached, please use direct link",
				"direct_url":   fmt.Sprintf("https://clawhub.ai/api/v1/download?slug=%s", skill.Slug),
				"install_cmd":  fmt.Sprintf("npx clawhub@latest install %s", skill.Slug),
				"skill_name":   skill.Name,
				"retry_later":  "Local cache will be available after first successful download",
			})
			return
		}
		utils.NotFound(c, errMsg)
		return
	}

	// 获取文件名用于下载
	skill, _ := h.svc.GetSkillByID(id)
	downloadName := "skill.zip"
	if skill != nil {
		if skill.FileName != "" {
			downloadName = skill.FileName
		} else {
			downloadName = skill.Name + ".zip"
		}
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+downloadName)
	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(filePath, downloadName)
}

// UpdateSkill 更新 Skill
func (h *SkillHandler) UpdateSkill(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	var req service.UploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	skill, err := h.svc.UpdateSkill(id, &req)
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, skill)
}

// DeleteSkill 删除 Skill
func (h *SkillHandler) DeleteSkill(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		utils.BadRequest(c, "invalid id")
		return
	}

	if err := h.svc.DeleteSkill(id); err != nil {
		utils.NotFound(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "skill deleted"})
}

// InitUploadDir 初始化上传目录（为现有 skill 创建示例文件）
func (h *SkillHandler) InitUploadDir(c *gin.Context) {
	if err := h.svc.InitUploadDir(); err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "upload directory initialized"})
}

// HealthCheck 健康检查
func (h *SkillHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
