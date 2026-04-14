package storage

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage S3 兼容存储实现（支持 AWS S3、MinIO 等）
type S3Storage struct {
	client      *s3.Client
	bucket      string // S3 存储桶名称
	cdnDomain   string // CDN 域名（如果设置，返回的 URL 使用此域名而非桶名）
	endpointURL string // 自定义端点（如果设置，使用路径样式 URL，例如 MinIO）
}

// NewS3StorageFromEnv 从环境变量创建 S3 存储实例
// 如果未设置 S3_BUCKET 则返回 nil
//
// 环境变量：
//   - S3_BUCKET（必需）
//   - S3_REGION（默认：us-west-2）
//   - AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY（可选；使用默认凭证链回退）
func NewS3StorageFromEnv() *S3Storage {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		slog.Info("S3_BUCKET not set, cloud upload disabled")
		return nil
	}

	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "us-west-2"
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey != "" && secretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		slog.Error("failed to load AWS config", "error", err)
		return nil
	}

	cdnDomain := os.Getenv("CLOUDFRONT_DOMAIN")

	endpointURL := os.Getenv("AWS_ENDPOINT_URL")
	s3Opts := []func(*s3.Options){}
	if endpointURL != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpointURL)
			o.UsePathStyle = true
		})
	}

	slog.Info("S3 storage initialized", "bucket", bucket, "region", region, "cdn_domain", cdnDomain, "endpoint_url", endpointURL)
	return &S3Storage{
		client:      s3.NewFromConfig(cfg, s3Opts...),
		bucket:      bucket,
		cdnDomain:   cdnDomain,
		endpointURL: endpointURL,
	}
}

// storageClass 返回适当的 S3 存储类别
// 自定义端点（例如 MinIO）仅支持 STANDARD；真实 AWS 默认使用 INTELLIGENT_TIERING
func (s *S3Storage) storageClass() types.StorageClass {
	if s.endpointURL != "" {
		return types.StorageClassStandard
	}
	return types.StorageClassIntelligentTiering
}

// KeyFromURL 从 CDN 或桶 URL 中提取 S3 对象键
// 例如："https://multica-static.copilothub.ai/abc123.png" → "abc123.png"
func (s *S3Storage) KeyFromURL(rawURL string) string {
	if s.endpointURL != "" {
		prefix := strings.TrimRight(s.endpointURL, "/") + "/" + s.bucket + "/"
		if strings.HasPrefix(rawURL, prefix) {
			return strings.TrimPrefix(rawURL, prefix)
		}
	}

	// Strip the "https://domain/" prefix.
	for _, prefix := range []string{
		"https://" + s.cdnDomain + "/",
		"https://" + s.bucket + "/",
	} {
		if strings.HasPrefix(rawURL, prefix) {
			return strings.TrimPrefix(rawURL, prefix)
		}
	}
	// Fallback: take everything after the last "/".
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		return rawURL[i+1:]
	}
	return rawURL
}

// Delete 从 S3 删除对象。错误会被记录但不会导致致命错误。
func (s *S3Storage) Delete(ctx context.Context, key string) {
	if key == "" {
		return
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Error("s3 DeleteObject failed", "key", key, "error", err)
	}
}

// DeleteKeys 从 S3 批量删除多个对象。尽力而为，错误会被记录。
func (s *S3Storage) DeleteKeys(ctx context.Context, keys []string) {
	for _, key := range keys {
		s.Delete(ctx, key)
	}
}

func (s *S3Storage) Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error) {
	safe := sanitizeFilename(filename)
	disposition := "attachment"
	if isInlineContentType(contentType) {
		disposition = "inline"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(s.bucket),
		Key:                aws.String(key),
		Body:               bytes.NewReader(data),
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(fmt.Sprintf(`%s; filename="%s"`, disposition, safe)),
		CacheControl:       aws.String("max-age=432000,public"),
		StorageClass:       s.storageClass(),
	})
	if err != nil {
		return "", fmt.Errorf("s3 PutObject: %w", err)
	}

	if s.endpointURL != "" {
		link := fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.endpointURL, "/"), s.bucket, key)
		return link, nil
	}
	domain := s.bucket
	if s.cdnDomain != "" {
		domain = s.cdnDomain
	}
	link := fmt.Sprintf("https://%s/%s", domain, key)
	return link, nil
}
