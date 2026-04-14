package storage

import (
	"context"
)

// Storage 文件存储接口
// 支持本地文件系统和 S3 兼容存储（MinIO、AWS S3 等）
type Storage interface {
	// Upload 上传文件数据，返回可访问的 URL
	Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error)
	// Delete 删除单个文件
	Delete(ctx context.Context, key string)
	// DeleteKeys 批量删除多个文件
	DeleteKeys(ctx context.Context, keys []string)
	// KeyFromURL 从完整 URL 中提取存储键（文件路径标识）
	KeyFromURL(rawURL string) string
}
