package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"skillhub/internal/config"
	"skillhub/internal/models"
	"skillhub/internal/repository"
	"skillhub/internal/utils"
	"strings"
	"sync"
)

// SkillService 业务逻辑层
type SkillService struct {
	repo      repository.SkillRepository
	uploadDir string
	mu        sync.RWMutex
}

// UploadRequest 上传请求
type UploadRequest struct {
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Description string `json:"description"`
	FileName    string `json:"file_name"`
}

// NewSkillService 创建服务实例
func NewSkillService(repo repository.SkillRepository, uploadDir string) *SkillService {
	// 确保上传目录存在
	os.MkdirAll(uploadDir, 0755)

	return &SkillService{
		repo:      repo,
		uploadDir: uploadDir,
	}
}

// GetAllSkills 获取所有技能
func (s *SkillService) GetAllSkills() ([]models.Skill, error) {
	return s.repo.FindAll(context.Background())
}

// GetTop50 获取 TOP 50
func (s *SkillService) GetTop50() []models.Skill {
	skills, err := s.repo.FindTopByDownloads(context.Background(), 50)
	if err != nil {
		return []models.Skill{}
	}

	// 更新排名（通过索引）
	for range skills {
		// 排名可以通过索引计算，不需要存储
	}

	return skills
}

// GetSkillByID 根据 ID 获取技能
func (s *SkillService) GetSkillByID(id int) (*models.Skill, error) {
	return s.repo.FindByID(context.Background(), uint(id))
}

// FilterSkills 过滤技能
func (s *SkillService) FilterSkills(page, perPage int, category, search string) (*models.SkillListResponse, error) {
	filter := &repository.SkillFilter{
		Page:     page,
		PerPage:  perPage,
		Category: category,
		Search:   search,
	}

	skills, total, err := s.repo.FindWithFilter(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	return &models.SkillListResponse{
		Total:   total,
		Skills:  skills,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// GetCategories 获取分类统计
func (s *SkillService) GetCategories() map[string]int64 {
	categories, err := s.repo.GetCategories(context.Background())
	if err != nil {
		return make(map[string]int64)
	}
	return categories
}

// UploadSkill 上传 Skill 文件
func (s *SkillService) UploadSkill(req *UploadRequest, fileData io.Reader) (*models.Skill, error) {
	// 验证输入
	if err := utils.ValidateSkillInput(req.Name, req.Category); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建上传目录
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %v", err)
	}

	// 净化文件名
	safeFilename := utils.SanitizeFilename(req.FileName)

	// 生成唯一文件名
	ext := filepath.Ext(safeFilename)
	if ext == "" {
		ext = ".zip"
	}
	baseName := strings.TrimSuffix(safeFilename, ext)
	baseName = strings.ReplaceAll(req.Name, " ", "_")
	baseName = strings.ReplaceAll(baseName, "/", "_")

	// 生成唯一文件名（使用时间戳）
	fileName := fmt.Sprintf("%d_%s%s", os.Getpid(), baseName, ext)

	// 检查文件是否存在，如果存在则添加计数器
	counter := 1
	originalFileName := fileName
	for {
		filePath := filepath.Join(s.uploadDir, fileName)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		fileName = fmt.Sprintf("%s_%d%s", strings.TrimSuffix(originalFileName, ext), counter, ext)
		counter++
	}

	filePath := filepath.Join(s.uploadDir, fileName)

	// 保存文件
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fileData); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save file: %v", err)
	}

	// 创建 Skill 记录
	skill := &models.Skill{
		Name:        req.Name,
		Icon:        req.Icon,
		Category:    req.Category,
		Description: req.Description,
		Downloads:   0,
		Rating:      0,
		Verified:    false,
		Accelerated: true,
		Safe:        true,
		FileName:    fileName,
	}

	if err := s.repo.Create(context.Background(), skill); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save skill: %v", err)
	}

	return skill, nil
}

// DownloadSkill 下载 Skill 文件
func (s *SkillService) DownloadSkill(id int) (string, error) {
	skill, err := s.repo.FindByID(context.Background(), uint(id))
	if err != nil {
		return "", errors.New("skill not found")
	}

	if skill.FileName == "" {
		return "", errors.New("skill has no file")
	}

	filePath := filepath.Join(s.uploadDir, skill.FileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", errors.New("file not found")
	}

	// 增加下载计数
	go s.repo.IncrementDownloads(context.Background(), uint(id))

	return filePath, nil
}

// UpdateSkill 更新 Skill
func (s *SkillService) UpdateSkill(id int, req *UploadRequest) (*models.Skill, error) {
	skill, err := s.repo.FindByID(context.Background(), uint(id))
	if err != nil {
		return nil, errors.New("skill not found")
	}

	if req.Name != "" {
		skill.Name = req.Name
	}
	if req.Icon != "" {
		skill.Icon = req.Icon
	}
	if req.Category != "" {
		skill.Category = req.Category
	}
	if req.Description != "" {
		skill.Description = req.Description
	}

	if err := s.repo.Update(context.Background(), skill); err != nil {
		return nil, err
	}

	return skill, nil
}

// DeleteSkill 删除 Skill
func (s *SkillService) DeleteSkill(id int) error {
	skill, err := s.repo.FindByID(context.Background(), uint(id))
	if err != nil {
		return errors.New("skill not found")
	}

	// 删除文件
	if skill.FileName != "" {
		filePath := filepath.Join(s.uploadDir, skill.FileName)
		os.Remove(filePath)
	}

	return s.repo.Delete(context.Background(), uint(id))
}

// GetStats 获取统计信息
func (s *SkillService) GetStats() map[string]interface{} {
	stats, err := s.repo.GetStats(context.Background())
	if err != nil {
		return map[string]interface{}{
			"total_skills":    0,
			"total_downloads": 0,
			"categories":      0,
			"upload_dir":      s.uploadDir,
		}
	}

	return map[string]interface{}{
		"total_skills":    stats.TotalSkills,
		"total_downloads": stats.TotalDownloads,
		"categories":      stats.Categories,
		"upload_dir":      s.uploadDir,
	}
}

// InitUploadDir 初始化上传目录（为现有 skill 创建示例文件）
func (s *SkillService) InitUploadDir() error {
	skills, err := s.repo.FindAll(context.Background())
	if err != nil {
		return err
	}

	os.MkdirAll(s.uploadDir, 0755)

	for _, skill := range skills {
		if skill.FileName == "" {
			fileName := fmt.Sprintf("%d_%s.zip", skill.ID, strings.ReplaceAll(skill.Name, " ", "_"))
			filePath := filepath.Join(s.uploadDir, fileName)

			// 创建示例文件
			content := fmt.Sprintf("# %s\n\n%s\n\nCategory: %s",
				skill.Name,
				skill.Description,
				skill.Category,
			)

			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return err
			}

			skill.FileName = fileName
			if err := s.repo.Update(context.Background(), &skill); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetSkillService 获取全局服务实例（向后兼容，不推荐使用）
var globalService *SkillService
var globalOnce sync.Once

// Deprecated: 使用 NewSkillService 代替
func GetSkillService() *SkillService {
	globalOnce.Do(func() {
		// 这种方式不推荐，仅用于向后兼容
		cfg := config.Get()
		if cfg == nil {
			// 如果配置未加载，使用默认值
			panic("config not loaded, please call config.Load() first")
		}
		// 这里需要一个数据库连接，在 main.go 中正确初始化
	})
	return globalService
}

// SetGlobalService 设置全局服务实例
func SetGlobalService(svc *SkillService) {
	globalService = svc
}
