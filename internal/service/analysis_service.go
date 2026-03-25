package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillhub/internal/models"
	"skillhub/internal/repository"
	"skillhub/internal/utils"
)

// AnalysisService 分析服务
type AnalysisService struct {
	repo       *repository.AnalysisRepository
	uploadDir  string
	maxSize    int64 // 最大文件大小 (bytes)
	mcpClient  *MCPClient
	llmSvc     *LLMService
}

// NewAnalysisService 创建分析服务
func NewAnalysisService(repo *repository.AnalysisRepository, uploadDir string, maxSize int64) *AnalysisService {
	// 初始化 MCP 客户端 (IDA-Pro-MCP)
	mcpClient := NewMCPClient("http://127.0.0.1:8745/mcp")

	return &AnalysisService{
		repo:      repo,
		uploadDir: uploadDir,
		maxSize:   maxSize,
		mcpClient: mcpClient,
		llmSvc:    NewLLMService(),
	}
}

// SetLLMService 设置 LLM 服务
func (s *AnalysisService) SetLLMService(llmSvc *LLMService) {
	s.llmSvc = llmSvc
}

// UploadFile 上传文件并创建分析任务
func (s *AnalysisService) UploadFile(userID string, agentID uint, file *multipart.FileHeader, description string) (*models.AnalysisTask, error) {
	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("无法打开上传文件: %w", err)
	}
	defer src.Close()

	// 检查文件大小
	if file.Size > s.maxSize {
		return nil, fmt.Errorf("文件大小超过限制 (最大 %d MB)", s.maxSize/1024/1024)
	}

	// 读取文件内容用于哈希计算和类型检测
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 计算文件哈希
	hash := sha256.Sum256(data)
	fileHash := hex.EncodeToString(hash[:])

	// 检查是否已存在相同文件的任务
	existingTask, err := s.repo.GetTaskByHash(fileHash)
	if err == nil && existingTask != nil {
		// 返回已存在的任务
		return existingTask, nil
	}

	// 检测文件类型
	fileType := utils.DetectFileTypeFromBytes(data)
	if fileType == utils.FileTypeUnknown {
		return nil, fmt.Errorf("不支持的文件类型，仅支持 PE/ELF/Mach-O 可执行文件")
	}

	// 创建存储目录
	taskDir := filepath.Join(s.uploadDir, fileHash[:2], fileHash[2:4])
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 保存文件 (使用哈希作为文件名)
	filePath := filepath.Join(taskDir, fileHash+filepath.Ext(file.Filename))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	// 创建分析任务
	task := &models.AnalysisTask{
		UserID:   userID,
		AgentID:  agentID,
		FileName: file.Filename,
		FilePath: filePath,
		FileHash: fileHash,
		FileType: string(fileType),
		FileSize: file.Size,
		Status:   "pending",
		Progress: 0,
	}

	if err := s.repo.CreateTask(task); err != nil {
		// 清理已保存的文件
		os.Remove(filePath)
		return nil, fmt.Errorf("创建分析任务失败: %w", err)
	}

	// 异步启动分析
	go s.runAnalysis(task.ID, filePath, file.Filename)

	return task, nil
}

// runAnalysis 执行分析任务
func (s *AnalysisService) runAnalysis(taskID uint, filePath, fileName string) {
	startTime := time.Now()

	// 更新状态为运行中
	s.repo.UpdateTaskStatus(taskID, "running", 10)

	fmt.Printf("[Analysis] Starting analysis for task %d: %s\n", taskID, fileName)

	// 1. 调用 IDA-Pro-MCP 分析二进制文件
	s.repo.UpdateTaskProgress(taskID, 20)

	analysisResult, err := s.mcpClient.AnalyzeBinary(filePath)
	if err != nil {
		fmt.Printf("[Analysis] MCP analyze error: %v\n", err)
		s.FailAnalysis(taskID, fmt.Sprintf("IDA Pro 分析失败: %v", err))
		return
	}

	s.repo.UpdateTaskProgress(taskID, 50)
	fmt.Printf("[Analysis] Binary analysis completed, result length: %d\n", len(analysisResult))

	// 2. 获取函数列表
	functions, _ := s.mcpClient.GetFunctions()
	s.repo.UpdateTaskProgress(taskID, 60)

	// 3. 获取字符串
	strings, _ := s.mcpClient.GetStrings()
	s.repo.UpdateTaskProgress(taskID, 70)

	// 4. 获取导入表
	imports, _ := s.mcpClient.GetImports()
	s.repo.UpdateTaskProgress(taskID, 80)

	// 5. 获取导出表
	exports, _ := s.mcpClient.GetExports()
	s.repo.UpdateTaskProgress(taskID, 85)

	// 6. 生成报告
	reportMD := s.GenerateReportFromMCP(fileName, analysisResult, functions, strings, imports, exports)

	s.repo.UpdateTaskProgress(taskID, 95)

	// 7. 保存结果
	result := &models.AnalysisResult{
		FileName:     fileName,
		FileSize:     0,
		FileType:     "PE",
		Architecture: "x86_64",
		Bits:         64,
		Endianness:   "Little",
		EntryPoint:   0,
		BaseAddress:  0,
		AnalysisTime: time.Since(startTime),
	}

	s.CompleteAnalysis(taskID, result, reportMD)

	fmt.Printf("[Analysis] Task %d completed in %v\n", taskID, time.Since(startTime))
}

// GenerateReportFromMCP 从 MCP 结果生成报告
func (s *AnalysisService) GenerateReportFromMCP(fileName, analysis, functions, stringsData, imports, exports string) string {
	var sb strings.Builder

	sb.WriteString("# 恶意文件分析报告\n\n")
	sb.WriteString(fmt.Sprintf("**生成时间**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**文件名**: %s\n\n", fileName))

	// 基本信息
	sb.WriteString("## 基本信息\n\n")
	if analysis != "" {
		sb.WriteString(analysis)
		sb.WriteString("\n\n")
	}

	// 导入函数
	if imports != "" {
		sb.WriteString("## 导入函数\n\n")
		sb.WriteString("```\n")
		sb.WriteString(imports)
		sb.WriteString("\n```\n\n")
	}

	// 导出函数
	if exports != "" {
		sb.WriteString("## 导出函数\n\n")
		sb.WriteString("```\n")
		sb.WriteString(exports)
		sb.WriteString("\n```\n\n")
	}

	// 函数列表
	if functions != "" {
		sb.WriteString("## 函数列表\n\n")
		sb.WriteString("```\n")
		sb.WriteString(functions)
		sb.WriteString("\n```\n\n")
	}

	// 字符串
	if stringsData != "" {
		sb.WriteString("## 字符串\n\n")
		sb.WriteString("```\n")
		sb.WriteString(stringsData)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("---\n\n")
	sb.WriteString("*报告由 SkillHub 恶意文件分析系统生成*\n")

	return sb.String()
}

// GetTask 获取任务详情
func (s *AnalysisService) GetTask(id uint, userID string) (*models.AnalysisTask, error) {
	return s.repo.GetTaskByIDAndUser(id, userID)
}

// GetTaskList 获取任务列表
func (s *AnalysisService) GetTaskList(userID string, page, perPage int) (*models.AnalysisTaskListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return s.repo.GetTasksByUserID(userID, page, perPage)
}

// CancelTask 取消任务
func (s *AnalysisService) CancelTask(id uint, userID string) error {
	task, err := s.repo.GetTaskByIDAndUser(id, userID)
	if err != nil {
		return err
	}

	if task.Status == "completed" {
		return fmt.Errorf("已完成的任务无法取消")
	}

	if task.Status == "running" {
		// TODO: 通知 IDA 服务器停止分析
	}

	return s.repo.UpdateTaskStatus(id, "cancelled", 0)
}

// DeleteTask 删除任务
func (s *AnalysisService) DeleteTask(id uint, userID string) error {
	task, err := s.repo.GetTaskByIDAndUser(id, userID)
	if err != nil {
		return err
	}

	// 删除关联的文件
	if task.FilePath != "" {
		os.Remove(task.FilePath)
	}
	if task.ReportPDF != "" {
		os.Remove(task.ReportPDF)
	}

	return s.repo.DeleteTask(id)
}

// GetTaskResult 获取任务分析结果
func (s *AnalysisService) GetTaskResult(id uint, userID string) (*models.AnalysisResult, error) {
	task, err := s.repo.GetTaskByIDAndUser(id, userID)
	if err != nil {
		return nil, err
	}

	if task.Status != "completed" {
		return nil, fmt.Errorf("任务尚未完成")
	}

	var result models.AnalysisResult
	if err := json.Unmarshal([]byte(task.ResultJSON), &result); err != nil {
		return nil, fmt.Errorf("解析分析结果失败: %w", err)
	}

	return &result, nil
}

// GetTaskReport 获取任务报告 (Markdown)
func (s *AnalysisService) GetTaskReport(id uint, userID string) (string, error) {
	task, err := s.repo.GetTaskByIDAndUser(id, userID)
	if err != nil {
		return "", err
	}

	if task.Status != "completed" {
		return "", fmt.Errorf("任务尚未完成")
	}

	return task.ReportMD, nil
}

// StartAnalysis 开始分析任务 (由调度器调用)
func (s *AnalysisService) StartAnalysis(taskID uint, serverID uint) error {
	// 更新任务状态
	if err := s.repo.UpdateTaskStatus(taskID, "running", 5); err != nil {
		return err
	}

	// 更新 IDA 服务器状态
	if err := s.repo.UpdateIDAServerStatus(serverID, "busy", taskID); err != nil {
		return err
	}

	return nil
}

// UpdateProgress 更新分析进度
func (s *AnalysisService) UpdateProgress(taskID uint, progress int) error {
	return s.repo.UpdateTaskProgress(taskID, progress)
}

// CompleteAnalysis 完成分析任务
func (s *AnalysisService) CompleteAnalysis(taskID uint, result *models.AnalysisResult, reportMD string) error {
	// 序列化分析结果
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("序列化分析结果失败: %w", err)
	}

	// 生成 PDF 报告路径
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		return err
	}

	reportPDF := ""
	if task.FilePath != "" {
		reportPDF = strings.TrimSuffix(task.FilePath, filepath.Ext(task.FilePath)) + ".pdf"
	}

	// 更新任务状态
	if err := s.repo.SetTaskResult(taskID, string(resultJSON), reportMD, reportPDF); err != nil {
		return err
	}

	// 释放 IDA 服务器
	if task.IDAServerID > 0 {
		s.repo.UpdateIDAServerStatus(task.IDAServerID, "online", 0)
	}

	return nil
}

// FailAnalysis 标记任务失败
func (s *AnalysisService) FailAnalysis(taskID uint, errMsg string) error {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		return err
	}

	// 更新任务状态
	if err := s.repo.SetTaskError(taskID, errMsg); err != nil {
		return err
	}

	// 释放 IDA 服务器
	if task.IDAServerID > 0 {
		s.repo.UpdateIDAServerStatus(task.IDAServerID, "online", 0)
	}

	return nil
}

// ===================== IDA Server Management =====================

// RegisterIDAServer 注册 IDA 服务器
func (s *AnalysisService) RegisterIDAServer(name, endpoint string) (*models.IDAServer, error) {
	server := &models.IDAServer{
		Name:     name,
		Endpoint: endpoint,
		Status:   "online",
		LastPing: time.Now(),
	}

	if err := s.repo.CreateIDAServer(server); err != nil {
		return nil, err
	}

	return server, nil
}

// GetIDAServers 获取所有 IDA 服务器
func (s *AnalysisService) GetIDAServers() ([]models.IDAServer, error) {
	return s.repo.GetAllIDAServers()
}

// GetAvailableServer 获取可用的 IDA 服务器
func (s *AnalysisService) GetAvailableServer() (*models.IDAServer, error) {
	return s.repo.GetAvailableIDAServer()
}

// UpdateServerHeartbeat 更新服务器心跳
func (s *AnalysisService) UpdateServerHeartbeat(serverID uint) error {
	return s.repo.UpdateIDAServerStatus(serverID, "online", 0)
}

// RemoveIDAServer 移除 IDA 服务器
func (s *AnalysisService) RemoveIDAServer(serverID uint) error {
	return s.repo.DeleteIDAServer(serverID)
}

// ===================== Report Generation =====================

// GenerateReport 生成分析报告 (Markdown)
func (s *AnalysisService) GenerateReport(result *models.AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString("# 恶意文件分析报告\n\n")
	sb.WriteString(fmt.Sprintf("**生成时间**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 基本信息
	sb.WriteString("## 基本信息\n\n")
	sb.WriteString(fmt.Sprintf("| 属性 | 值 |\n"))
	sb.WriteString(fmt.Sprintf("|------|----|\n"))
	sb.WriteString(fmt.Sprintf("| 文件名 | %s |\n", result.FileName))
	sb.WriteString(fmt.Sprintf("| 文件大小 | %d bytes |\n", result.FileSize))
	sb.WriteString(fmt.Sprintf("| 文件类型 | %s |\n", result.FileType))
	sb.WriteString(fmt.Sprintf("| 架构 | %s (%d-bit) |\n", result.Architecture, result.Bits))
	sb.WriteString(fmt.Sprintf("| 字节序 | %s |\n", result.Endianness))
	sb.WriteString(fmt.Sprintf("| 入口点 | 0x%X |\n", result.EntryPoint))
	sb.WriteString(fmt.Sprintf("| 基址 | 0x%X |\n", result.BaseAddress))
	if result.CompileTime != "" {
		sb.WriteString(fmt.Sprintf("| 编译时间 | %s |\n", result.CompileTime))
	}
	sb.WriteString("\n")

	// 节区信息
	if len(result.Sections) > 0 {
		sb.WriteString("## 节区信息\n\n")
		sb.WriteString("| 名称 | 虚拟地址 | 虚拟大小 | 原始大小 | 熵值 | 权限 |\n")
		sb.WriteString("|------|----------|----------|----------|------|------|\n")
		for _, sec := range result.Sections {
			sb.WriteString(fmt.Sprintf("| %s | 0x%X | %d | %d | %.2f | %s |\n",
				sec.Name, sec.VirtualAddress, sec.VirtualSize, sec.RawSize, sec.Entropy, sec.Permissions))
		}
		sb.WriteString("\n")
	}

	// 导入函数
	if len(result.Imports) > 0 {
		sb.WriteString("## 导入函数\n\n")
		sb.WriteString("| DLL | 函数名 | 地址 |\n")
		sb.WriteString("|-----|--------|------|\n")
		for _, imp := range result.Imports {
			sb.WriteString(fmt.Sprintf("| %s | %s | 0x%X |\n", imp.DLL, imp.Name, imp.Address))
		}
		sb.WriteString("\n")
	}

	// 导出函数
	if len(result.Exports) > 0 {
		sb.WriteString("## 导出函数\n\n")
		sb.WriteString("| 函数名 | 地址 | 序号 |\n")
		sb.WriteString("|--------|------|------|\n")
		for _, exp := range result.Exports {
			sb.WriteString(fmt.Sprintf("| %s | 0x%X | %d |\n", exp.Name, exp.Address, exp.Ordinal))
		}
		sb.WriteString("\n")
	}

	// 关键函数
	if len(result.Functions) > 0 {
		sb.WriteString("## 函数列表\n\n")
		fmt.Fprintf(&sb, "**总计**: %d 个函数\n\n", len(result.Functions))
		if len(result.Functions) <= 50 {
			sb.WriteString("| 函数名 | 地址 | 大小 |\n")
			sb.WriteString("|--------|------|------|\n")
			for _, fn := range result.Functions {
				sb.WriteString(fmt.Sprintf("| %s | 0x%X | %d |\n", fn.Name, fn.Address, fn.Size))
			}
		} else {
			sb.WriteString("前 50 个函数:\n\n")
			sb.WriteString("| 函数名 | 地址 | 大小 |\n")
			sb.WriteString("|--------|------|------|\n")
			for i := 0; i < 50 && i < len(result.Functions); i++ {
				fn := result.Functions[i]
				sb.WriteString(fmt.Sprintf("| %s | 0x%X | %d |\n", fn.Name, fn.Address, fn.Size))
			}
		}
		sb.WriteString("\n")
	}

	// 字符串
	if len(result.Strings) > 0 {
		sb.WriteString("## 字符串\n\n")
		fmt.Fprintf(&sb, "**总计**: %d 个字符串\n\n", len(result.Strings))
		interestingStrings := s.extractInterestingStrings(result.Strings)
		if len(interestingStrings) > 0 {
			sb.WriteString("### 可疑字符串\n\n")
			sb.WriteString("| 字符串 | 地址 |\n")
			sb.WriteString("|--------|------|\n")
			for _, str := range interestingStrings {
				sb.WriteString(fmt.Sprintf("| %s | 0x%X |\n", str.Value, str.Address))
			}
			sb.WriteString("\n")
		}
	}

	// 分析时间
	sb.WriteString(fmt.Sprintf("---\n\n**分析耗时**: %v\n", result.AnalysisTime))

	return sb.String()
}

// extractInterestingStrings 提取可疑字符串
func (s *AnalysisService) extractInterestingStrings(strs []models.StringInfo) []models.StringInfo {
	var interesting []models.StringInfo

	keywords := []string{
		"http://", "https://", "ftp://",
		"password", "passwd", "pwd",
		"admin", "login", "auth",
		"key", "secret", "token",
		"cmd", "exec", "shell",
		"registry", "HKEY",
		"socket", "connect", "download",
		"encrypt", "decrypt", "crypto",
		".exe", ".dll", ".bat",
		"Software\\", "CurrentVersion\\Run",
	}

	for _, str := range strs {
		lower := strings.ToLower(str.Value)
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				interesting = append(interesting, str)
				break
			}
		}
	}

	// 限制数量
	if len(interesting) > 50 {
		interesting = interesting[:50]
	}

	return interesting
}
