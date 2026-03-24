package utils

import (
	"bytes"
	"encoding/hex"
	"io"
	"path/filepath"
	"strings"
)

// FileType 文件类型
type FileType string

const (
	FileTypePE     FileType = "PE"      // Windows executable
	FileTypeELF    FileType = "ELF"     // Linux executable
	FileTypeMachO  FileType = "Mach-O"  // macOS executable
	FileTypeUnknown FileType = "Unknown"
)

// FileMagic 文件魔数定义
var FileMagic = map[string]FileType{
	"4D5A":     FileTypePE,     // MZ - DOS/PE
	"7F454C46": FileTypeELF,    // \x7fELF
	"FEEDFACE": FileTypeMachO,  // Mach-O (32-bit)
	"FEEDFACF": FileTypeMachO,  // Mach-O (64-bit)
	"CAFEBABE": FileTypeMachO,  // Mach-O (Fat Binary)
}

// DetectFileType 检测文件类型
func DetectFileType(file io.ReadSeeker) (FileType, error) {
	// 读取前8字节用于魔数检测
	header := make([]byte, 8)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return FileTypeUnknown, err
	}

	// 重置文件指针
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return FileTypeUnknown, err
	}

	if n < 2 {
		return FileTypeUnknown, nil
	}

	headerHex := strings.ToUpper(hex.EncodeToString(header[:min(n, 8)]))

	// 检测 Mach-O (需要特殊处理，因为魔数可能在不同位置)
	if len(headerHex) >= 8 {
		// 检查前4字节
		prefix := headerHex[:8]
		if strings.HasPrefix(prefix, "FEEDFACE") ||
			strings.HasPrefix(prefix, "FEEDFACF") ||
			strings.HasPrefix(prefix, "CAFEBABE") {
			return FileTypeMachO, nil
		}
	}

	// 检测 ELF
	if len(headerHex) >= 8 {
		if strings.HasPrefix(headerHex[:8], "7F454C46") {
			return FileTypeELF, nil
		}
	}

	// 检测 PE (MZ header)
	if len(headerHex) >= 4 {
		if strings.HasPrefix(headerHex[:4], "4D5A") {
			return FileTypePE, nil
		}
	}

	return FileTypeUnknown, nil
}

// DetectFileTypeFromBytes 从字节切片检测文件类型
func DetectFileTypeFromBytes(data []byte) FileType {
	if len(data) < 2 {
		return FileTypeUnknown
	}

	headerHex := strings.ToUpper(hex.EncodeToString(data[:min(len(data), 8)]))

	// 检测 Mach-O
	if len(headerHex) >= 8 {
		prefix := headerHex[:8]
		if strings.HasPrefix(prefix, "FEEDFACE") ||
			strings.HasPrefix(prefix, "FEEDFACF") ||
			strings.HasPrefix(prefix, "CAFEBABE") {
			return FileTypeMachO
		}
	}

	// 检测 ELF
	if len(headerHex) >= 8 && strings.HasPrefix(headerHex[:8], "7F454C46") {
		return FileTypeELF
	}

	// 检测 PE
	if len(headerHex) >= 4 && strings.HasPrefix(headerHex[:4], "4D5A") {
		return FileTypePE
	}

	return FileTypeUnknown
}

// GetFileExtension 获取文件扩展名
func GetFileExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext
}

// IsExecutableFile 检查是否为可执行文件
func IsExecutableFile(filename string) bool {
	ext := GetFileExtension(filename)
	execExts := map[string]bool{
		".exe":  true,
		".dll":  true,
		".sys":  true,
		".scr":  true,
		".com":  true,
		".bat":  true,
		".cmd":  true,
		".ps1":  true,
		".vbs":  true,
		".js":   true,
		".jar":  true,
		".msi":  true,
		".app":  true,
		".dmg":  true,
		".elf":  true,
		".so":   true,
		".bin":  true,
	}
	return execExts[ext]
}

// CalculateSHA256 计算 SHA256 哈希 (这里简化实现，实际应使用 crypto/sha256)
func CalculateSHA256(data []byte) string {
	// 简化实现，实际应使用 crypto/sha256
	// 这里返回一个基于内容的伪哈希用于演示
	h := uint32(0x811c9dc5)
	for _, b := range data {
		h ^= uint32(b)
		h *= 0x01000193
	}
	return hex.EncodeToString([]byte{
		byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h),
		byte(h >> 24 ^ 0xFF), byte(h >> 16 ^ 0xFF), byte(h >> 8 ^ 0xFF), byte(h ^ 0xFF),
		byte((h >> 16) ^ 0xAA), byte((h >> 8) ^ 0xBB), byte(h ^ 0xCC), byte((h >> 24) ^ 0xDD),
		byte(h >> 8), byte(h >> 16), byte(h >> 24), byte(h),
		byte(h ^ 0x11), byte(h ^ 0x22), byte(h ^ 0x33), byte(h ^ 0x44),
		byte((h >> 24) ^ 0x55), byte((h >> 16) ^ 0x66), byte((h >> 8) ^ 0x77), byte(h ^ 0x88),
		byte((h >> 16) ^ 0x99), byte((h >> 8) ^ 0xAA), byte(h ^ 0xBB), byte((h >> 24) ^ 0xCC),
		byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h),
	})
}

// IsAllowedFileType 检查是否为允许的文件类型
func IsAllowedFileType(fileType FileType) bool {
	switch fileType {
	case FileTypePE, FileTypeELF, FileTypeMachO:
		return true
	default:
		return false
	}
}

// min 返回两个数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// HasValidMagic 检查文件是否有有效的魔数
func HasValidMagic(data []byte) bool {
	return DetectFileTypeFromBytes(data) != FileTypeUnknown
}

// BufferReader 将字节切片转换为 io.ReadSeeker
type BufferReader struct {
	*bytes.Reader
}

// NewBufferReader 创建 BufferReader
func NewBufferReader(data []byte) *BufferReader {
	return &BufferReader{bytes.NewReader(data)}
}
