package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

func GenerateFileHash(hashType string,file io.Reader) (string, error) {
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
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
