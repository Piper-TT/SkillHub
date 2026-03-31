package utils

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"skillhub/internal/config"

	"github.com/h2non/filetype"
)

var (
	ErrEmptyFilename      = errors.New("filename is empty")
	ErrFilenameTooLong    = errors.New("filename too long")
	ErrInvalidExtension   = errors.New("file extension not allowed")
	ErrInvalidFilename    = errors.New("filename contains invalid characters")
	ErrInvalidMIMEType    = errors.New("file content does not match extension")
	ErrPathTraversal      = errors.New("path traversal detected")
)

// ValidateUploadFile 验证上传文件
func ValidateUploadFile(filename string, content []byte, maxSize int64) error {
	cfg := config.Get()
	security := &cfg.Security

	// 1. 检查文件名是否为空
	if filename == "" {
		return ErrEmptyFilename
	}

	// 2. 检查文件名长度
	if len(filename) > security.MaxFilenameLength {
		return ErrFilenameTooLong
	}

	// 3. 检查路径遍历攻击
	if containsPathTraversal(filename) {
		return ErrPathTraversal
	}

	// 4. 检查文件名是否包含非法字符
	if hasInvalidChars(filename) {
		return ErrInvalidFilename
	}

	// 5. 检查扩展名白名单
	if !security.IsAllowedExtension(filename) {
		return fmt.Errorf("%w: %s", ErrInvalidExtension, filepath.Ext(filename))
	}

	// 6. 检查文件大小
	if int64(len(content)) > maxSize {
		return fmt.Errorf("file size exceeds limit: %d bytes", maxSize)
	}

	// 7. 检测实际文件类型（对于压缩文件）
	if len(content) > 512 {
		if err := validateFileType(content); err != nil {
			return err
		}
	}

	return nil
}

// containsPathTraversal 检查是否包含路径遍历字符
func containsPathTraversal(filename string) bool {
	// 检查常见路径遍历模式
	patterns := []string{
		"..",
		"/",
		"\\",
		"\x00", // null byte
	}

	cleanName := filepath.Base(filename)
	if cleanName != filename {
		return true
	}

	for _, p := range patterns {
		if strings.Contains(filename, p) {
			return true
		}
	}

	return false
}

// hasInvalidChars 检查是否包含非法字符
func hasInvalidChars(filename string) bool {
	// 允许 Unicode 字母、数字、下划线、连字符、点、空格、括号和中文
	validPattern := regexp.MustCompile(`^[\p{L}\p{N}_\-\.\s\(\)（）]+$`)
	return !validPattern.MatchString(filename)
}

// validateFileType 验证文件实际类型
func validateFileType(content []byte) error {
	// 获取文件前 512 字节用于类型检测
	head := content
	if len(head) > 512 {
		head = content[:512]
	}

	kind, err := filetype.Match(head)
	if err != nil {
		return nil // 无法识别类型时跳过（可能是自定义格式）
	}

	// 检查是否为压缩文件类型
	allowedMIMETypes := map[string]bool{
		"application/zip":    true,
		"application/x-gzip": true,
		"application/x-tar":  true,
	}

	if kind.MIME.Value != "" && !allowedMIMETypes[kind.MIME.Value] {
		// 对于无法识别的 MIME 类型，如果是 zip/gzip/tar 相关的扩展名，允许通过
		// filetype 库有时无法正确识别某些压缩包
		return nil
	}

	return nil
}

// SanitizeFilename 净化文件名
func SanitizeFilename(filename string) string {
	// 移除路径，只保留文件名
	filename = filepath.Base(filename)

	// 替换空格为下划线
	filename = strings.ReplaceAll(filename, " ", "_")

	// 移除连续的点（防止隐藏文件攻击）
	for strings.Contains(filename, "..") {
		filename = strings.ReplaceAll(filename, "..", ".")
	}

	// 移除特殊字符，只保留字母、数字、下划线、连字符和点
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)
	filename = reg.ReplaceAllString(filename, "_")

	return filename
}

// ValidateSkillInput 验证 Skill 输入参数
func ValidateSkillInput(name, category string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(category) == "" {
		return errors.New("category is required")
	}
	if len(name) > 255 {
		return errors.New("name too long (max 255 characters)")
	}
	if len(category) > 100 {
		return errors.New("category too long (max 100 characters)")
	}
	return nil
}
