package repository

import (
	"time"

	"skillhub/internal/models"

	"gorm.io/gorm"
)

// AnalysisRepository 分析任务仓库
type AnalysisRepository struct {
	db *gorm.DB
}

// NewAnalysisRepository 创建分析任务仓库
func NewAnalysisRepository(db *gorm.DB) *AnalysisRepository {
	return &AnalysisRepository{db: db}
}

// CreateTask 创建分析任务
func (r *AnalysisRepository) CreateTask(task *models.AnalysisTask) error {
	return r.db.Create(task).Error
}

// GetTaskByID 根据ID获取任务
func (r *AnalysisRepository) GetTaskByID(id uint) (*models.AnalysisTask, error) {
	var task models.AnalysisTask
	err := r.db.First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTaskByIDAndUser 根据ID和用户ID获取任务（确保用户只能访问自己的任务）
func (r *AnalysisRepository) GetTaskByIDAndUser(id uint, userID string) (*models.AnalysisTask, error) {
	var task models.AnalysisTask
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTasksByUserID 获取用户的所有任务
func (r *AnalysisRepository) GetTasksByUserID(userID string, page, perPage int) (*models.AnalysisTaskListResponse, error) {
	var tasks []models.AnalysisTask
	var total int64

	query := r.db.Model(&models.AnalysisTask{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * perPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&tasks).Error; err != nil {
		return nil, err
	}

	return &models.AnalysisTaskListResponse{
		Total:   total,
		Tasks:   tasks,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// UpdateTask 更新任务
func (r *AnalysisRepository) UpdateTask(task *models.AnalysisTask) error {
	return r.db.Save(task).Error
}

// UpdateTaskStatus 更新任务状态
func (r *AnalysisRepository) UpdateTaskStatus(id uint, status string, progress int) error {
	updates := map[string]interface{}{
		"status":   status,
		"progress": progress,
	}

	if status == "running" {
		now := time.Now()
		updates["started_at"] = &now
	} else if status == "completed" || status == "failed" || status == "cancelled" {
		now := time.Now()
		updates["completed_at"] = &now
	}

	return r.db.Model(&models.AnalysisTask{}).Where("id = ?", id).Updates(updates).Error
}

// SetTaskError 设置任务错误
func (r *AnalysisRepository) SetTaskError(id uint, errMsg string) error {
	now := time.Now()
	return r.db.Model(&models.AnalysisTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": errMsg,
		"completed_at":  &now,
	}).Error
}

// SetTaskResult 设置任务结果
func (r *AnalysisRepository) SetTaskResult(id uint, resultJSON, reportMD, reportPDF string) error {
	now := time.Now()
	return r.db.Model(&models.AnalysisTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       "completed",
		"progress":     100,
		"result_json":  resultJSON,
		"report_md":    reportMD,
		"report_pdf":   reportPDF,
		"completed_at": &now,
	}).Error
}

// DeleteTask 删除任务
func (r *AnalysisRepository) DeleteTask(id uint) error {
	return r.db.Delete(&models.AnalysisTask{}, id).Error
}

// GetPendingTasks 获取待处理的任务
func (r *AnalysisRepository) GetPendingTasks(limit int) ([]models.AnalysisTask, error) {
	var tasks []models.AnalysisTask
	err := r.db.Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

// GetTaskByHash 根据文件哈希获取任务（去重）
func (r *AnalysisRepository) GetTaskByHash(hash string) (*models.AnalysisTask, error) {
	var task models.AnalysisTask
	err := r.db.Where("file_hash = ?", hash).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ===================== IDA Server =====================

// CreateIDAServer 创建IDA服务器
func (r *AnalysisRepository) CreateIDAServer(server *models.IDAServer) error {
	return r.db.Create(server).Error
}

// GetIDAServerByID 根据ID获取IDA服务器
func (r *AnalysisRepository) GetIDAServerByID(id uint) (*models.IDAServer, error) {
	var server models.IDAServer
	err := r.db.First(&server, id).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// GetAllIDAServers 获取所有IDA服务器
func (r *AnalysisRepository) GetAllIDAServers() ([]models.IDAServer, error) {
	var servers []models.IDAServer
	err := r.db.Find(&servers).Error
	return servers, err
}

// GetAvailableIDAServer 获取可用的IDA服务器
func (r *AnalysisRepository) GetAvailableIDAServer() (*models.IDAServer, error) {
	var server models.IDAServer
	err := r.db.Where("status = ?", "online").First(&server).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// UpdateIDAServer 更新IDA服务器
func (r *AnalysisRepository) UpdateIDAServer(server *models.IDAServer) error {
	return r.db.Save(server).Error
}

// UpdateIDAServerStatus 更新IDA服务器状态
func (r *AnalysisRepository) UpdateIDAServerStatus(id uint, status string, currentTask uint) error {
	return r.db.Model(&models.IDAServer{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"current_task": currentTask,
		"last_ping":    time.Now(),
	}).Error
}

// DeleteIDAServer 删除IDA服务器
func (r *AnalysisRepository) DeleteIDAServer(id uint) error {
	return r.db.Delete(&models.IDAServer{}, id).Error
}

// UpdateTaskProgress 更新任务进度
func (r *AnalysisRepository) UpdateTaskProgress(id uint, progress int) error {
	return r.db.Model(&models.AnalysisTask{}).Where("id = ?", id).Update("progress", progress).Error
}
