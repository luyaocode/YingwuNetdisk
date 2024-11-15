package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"time"
)

func GenerateFileHash(hashType string, file io.Reader) (string, error) {
	// 获取当前时间戳
	currentTime := time.Now().Format(time.RFC3339) // 采用RFC3339格式的时间戳
	// 选择哈希算法
	var h hash.Hash
	switch hashType {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	default:
		return "", fmt.Errorf("unsupported hash type")
	}
	// 将当前时间戳写入哈希计算中
	if _, err := h.Write([]byte(currentTime)); err != nil {
		return "", fmt.Errorf("failed to write timestamp to hash: %v", err)
	}

	// 计算文件的哈希
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	// 返回计算出的哈希值
	return hex.EncodeToString(h.Sum(nil)), nil
}
