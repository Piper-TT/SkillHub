package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// CalculateFileHash 计算文件的 SHA256 哈希
func CalculateFileHash(file io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CalculateDataHash 计算字节数据的 SHA256 哈希
func CalculateDataHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
