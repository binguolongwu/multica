package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
)

// defaultJWTSecret 默认 JWT 密钥（仅开发环境使用，生产环境必须更改）
const defaultJWTSecret = "multica-dev-secret-change-in-production"

// jwtSecret JWT 密钥（懒加载，只初始化一次）
var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

// JWTSecret 获取 JWT 签名密钥
// 优先级：环境变量 JWT_SECRET > 默认密钥
// 使用 sync.Once 确保线程安全的单例模式
func JWTSecret() []byte {
	jwtSecretOnce.Do(func() {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = defaultJWTSecret
		}
		jwtSecret = []byte(secret)
	})

	return jwtSecret
}

// GeneratePATToken 生成个人访问令牌（PAT）
// 格式："mul_" + 40 个随机十六进制字符
// 使用加密安全的随机数生成器
func GeneratePATToken() (string, error) {
	b := make([]byte, 20) // 20 bytes = 40 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate PAT token: %w", err)
	}
	return "mul_" + hex.EncodeToString(b), nil
}

// GenerateDaemonToken 生成 Daemon 认证令牌
// 格式："mdt_" + 40 个随机十六进制字符
// 用于 Agent 运行时与服务器通信认证
func GenerateDaemonToken() (string, error) {
	b := make([]byte, 20) // 20 bytes = 40 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate daemon token: %w", err)
	}
	return "mdt_" + hex.EncodeToString(b), nil
}

// HashToken 计算令牌的 SHA-256 哈希值（十六进制编码）
// 数据库中存储的是哈希值而非原始令牌，增强安全性
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
