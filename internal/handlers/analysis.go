package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"skillhub/internal/models"
	"skillhub/internal/service"
	"skillhub/internal/utils"

	"github.com/gin-gonic/gin"
)

// AnalysisHandler 分析任务处理器
type AnalysisHandler struct {
	svc *service.AnalysisService
}

// NewAnalysisHandler 创建分析任务处理器
func NewAnalysisHandler(svc *service.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{svc: svc}
}

// UploadFile 上传文件
// POST /api/analysis/upload
func (h *AnalysisHandler) UploadFile(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.Unauthorized(c, "未授权")
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请上传文件")
		return
	}

	description := c.PostForm("description")

	// 上传文件并创建任务 (agentID=0 表示使用默认 MCP 分析)
	task, err := h.svc.UploadFile(userID, 0, file, description)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"task_id":  task.ID,
		"status":   task.Status,
		"message":  "文件上传成功，已加入分析队列",
	})
}

// GetTask 获取任务状态
// GET /api/analysis/:id
func (h *AnalysisHandler) GetTask(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.Unauthorized(c, "未授权")
		return
	}

	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	task, err := h.svc.GetTask(uint(taskID), userID)
	if err != nil {
		// 如果按 userID 查不到，尝试只按 ID 查询（兼容性处理）
		task, err = h.svc.GetTaskByID(uint(taskID))
		if err != nil {
			utils.NotFound(c, "任务不存在")
			return
		}
	}

	utils.Success(c, models.TaskStatusResponse{
		ID:           task.ID,
		FileName:     task.FileName,
		FileHash:     task.FileHash,
		FileType:     task.FileType,
		FileSize:     task.FileSize,
		Status:       task.Status,
		Progress:     task.Progress,
		ErrorMessage: task.ErrorMessage,
		StartedAt:    task.StartedAt,
		CompletedAt:  task.CompletedAt,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	})
}

// GetTaskList 获取任务列表
// GET /api/analysis/tasks
func (h *AnalysisHandler) GetTaskList(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.Unauthorized(c, "未授权")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	result, err := h.svc.GetTaskList(userID, page, perPage)
	if err != nil {
		utils.InternalError(c, "获取任务列表失败")
		return
	}

	// 转换为响应格式
	tasks := make([]models.TaskStatusResponse, len(result.Tasks))
	for i, task := range result.Tasks {
		tasks[i] = models.TaskStatusResponse{
			ID:           task.ID,
			FileName:     task.FileName,
			FileHash:     task.FileHash,
			FileType:     task.FileType,
			FileSize:     task.FileSize,
			Status:       task.Status,
			Progress:     task.Progress,
			ErrorMessage: task.ErrorMessage,
			StartedAt:    task.StartedAt,
			CompletedAt:  task.CompletedAt,
			CreatedAt:    task.CreatedAt,
			UpdatedAt:    task.UpdatedAt,
		}
	}

	utils.Success(c, gin.H{
		"total":    result.Total,
		"tasks":    tasks,
		"page":     result.Page,
		"per_page": result.PerPage,
	})
}

// GetTaskResult 获取分析结果
// GET /api/analysis/:id/result
func (h *AnalysisHandler) GetTaskResult(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.Unauthorized(c, "未授权")
		return
	}

	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	result, err := h.svc.GetTaskResult(uint(taskID), userID)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, result)
}

// GetTaskReport 获取报告 (Markdown)
// GET /api/analysis/:id/report
func (h *AnalysisHandler) GetTaskReport(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.Unauthorized(c, "未授权")
		return
	}

	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	report, err := h.svc.GetTaskReport(uint(taskID), userID)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	// 返回 Markdown 内容
	c.Header("Content-Type", "text/markdown; charset=utf-8")
	c.String(http.StatusOK, report)
}

// DownloadPDF 下载 PDF 报告
// GET /api/analysis/:id/pdf
func (h *AnalysisHandler) DownloadPDF(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.Unauthorized(c, "未授权")
		return
	}

	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	task, err := h.svc.GetTask(uint(taskID), userID)
	if err != nil {
		utils.NotFound(c, "任务不存在")
		return
	}

	if task.Status != "completed" {
		utils.BadRequest(c, "任务尚未完成")
		return
	}

	if task.ReportPDF == "" {
		utils.NotFound(c, "PDF 报告尚未生成")
		return
	}

	// 下载文件
	c.FileAttachment(task.ReportPDF, fmt.Sprintf("analysis_report_%d.pdf", taskID))
}

// CancelTask 取消任务
// DELETE /api/analysis/:id
func (h *AnalysisHandler) CancelTask(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.Unauthorized(c, "未授权")
		return
	}

	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	if err := h.svc.CancelTask(uint(taskID), userID); err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "任务已取消"})
}

// ===================== IDA Server Management =====================

// GetIDAServers 获取 IDA 服务器列表
// GET /api/ida/servers
func (h *AnalysisHandler) GetIDAServers(c *gin.Context) {
	servers, err := h.svc.GetIDAServers()
	if err != nil {
		utils.InternalError(c, "获取服务器列表失败")
		return
	}

	utils.Success(c, servers)
}

// RegisterIDAServer 注册 IDA 服务器
// POST /api/ida/servers
func (h *AnalysisHandler) RegisterIDAServer(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	server, err := h.svc.RegisterIDAServer(req.Name, req.Endpoint)
	if err != nil {
		utils.InternalError(c, "注册服务器失败")
		return
	}

	utils.Success(c, server)
}

// RemoveIDAServer 移除 IDA 服务器
// DELETE /api/ida/servers/:id
func (h *AnalysisHandler) RemoveIDAServer(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "无效的服务器ID")
		return
	}

	if err := h.svc.RemoveIDAServer(uint(serverID)); err != nil {
		utils.InternalError(c, "移除服务器失败")
		return
	}

	utils.Success(c, gin.H{"message": "服务器已移除"})
}

// ServerHeartbeat 服务器心跳
// POST /api/ida/servers/:id/heartbeat
func (h *AnalysisHandler) ServerHeartbeat(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "无效的服务器ID")
		return
	}

	if err := h.svc.UpdateServerHeartbeat(uint(serverID)); err != nil {
		utils.InternalError(c, "更新心跳失败")
		return
	}

	utils.Success(c, gin.H{"message": "ok"})
}
