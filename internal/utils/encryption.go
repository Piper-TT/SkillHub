package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

var (
	ErrInvalidKey = errors.New("invalid encryption key")
	ErrDecryptFailed = errors.New("decryption failed")
)

// getEncryptionKey 获取加密密钥
// 优先从环境变量读取，否则使用默认密钥（仅开发环境）
func getEncryptionKey() ([]byte, error) {
	keyHex := os.Getenv("AGENTHUB_ENCRYPTION_KEY")
	if keyHex == "" {
		// 默认密钥（仅开发环境使用，生产环境必须设置环境变量）
		keyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, ErrInvalidKey
	}

	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	return key, nil
}

// EncryptAPIKey 加密 API Key
func EncryptAPIKey(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// DecryptAPIKey 解密 API Key
func DecryptAPIKey(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", ErrDecryptFailed
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrDecryptFailed
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptFailed
	}

	return string(plaintext), nil
}
