// Package oss provides workspace-level object storage integration.
//
// Supports multiple OSS providers (Qiniu, Aliyun OSS, Volcengine TOS,
// Tencent COS, S3-compatible) through a unified ProviderDriver interface.
// Credentials are stored encrypted with secretbox; agents never see them
// — they interact with OSS exclusively through the multica CLI.
package oss

import (
	"context"
	"io"
)

// ProviderDriver is the interface each OSS provider must implement.
type ProviderDriver interface {
	// Upload stores a file and returns its public URL.
	Upload(ctx context.Context, cfg ProviderConfig, key string, body io.Reader, size int64, contentType string) (publicURL string, err error)

	// Download retrieves a file by key.
	Download(ctx context.Context, cfg ProviderConfig, key string) (io.ReadCloser, error)

	// Delete removes a single object.
	Delete(ctx context.Context, cfg ProviderConfig, key string) error

	// ListKeys returns object keys with an optional prefix filter.
	ListKeys(ctx context.Context, cfg ProviderConfig, prefix string, limit int) ([]string, error)

	// PublicURL returns the public-facing URL for a given key.
	PublicURL(cfg ProviderConfig, key string) string
}

// ProviderConfig holds the decrypted configuration needed to dial a provider.
type ProviderConfig struct {
	Provider     string // qiniu / aliyun_oss / volcengine_tos / tencent_cos / s3_compatible / minio
	Bucket       string
	Region       string
	Endpoint     string
	AccessKey    string
	SecretKey    string // decrypted
	CustomDomain string
	FolderPrefix string
}
