package oss

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// ── Alibaba OSS (阿里云对象存储) ─────────────────────────────────────────────

// AliyunDriver implements ProviderDriver for Alibaba Cloud OSS.
type AliyunDriver struct{}

func (d *AliyunDriver) client(cfg ProviderConfig) (*oss.Client, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		region := strings.TrimPrefix(cfg.Region, "oss-")
		endpoint = fmt.Sprintf("https://oss-%s.aliyuncs.com", region)
	}
	return oss.New(endpoint, cfg.AccessKey, cfg.SecretKey)
}

func (d *AliyunDriver) bucket(cfg ProviderConfig) (*oss.Bucket, error) {
	c, err := d.client(cfg)
	if err != nil {
		return nil, err
	}
	return c.Bucket(cfg.Bucket)
}

func (d *AliyunDriver) Upload(ctx context.Context, cfg ProviderConfig, key string, body io.Reader, size int64, contentType string) (string, error) {
	b, err := d.bucket(cfg)
	if err != nil {
		return "", err
	}
	err = b.PutObject(key, body, oss.ContentType(contentType))
	if err != nil {
		return "", fmt.Errorf("aliyun PutObject: %w", err)
	}
	return d.publicURL(cfg, key), nil
}

func (d *AliyunDriver) Download(ctx context.Context, cfg ProviderConfig, key string) (io.ReadCloser, error) {
	b, err := d.bucket(cfg)
	if err != nil {
		return nil, err
	}
	body, err := b.GetObject(key)
	if err != nil {
		return nil, fmt.Errorf("aliyun GetObject: %w", err)
	}
	return body, nil
}

func (d *AliyunDriver) Delete(ctx context.Context, cfg ProviderConfig, key string) error {
	b, err := d.bucket(cfg)
	if err != nil {
		return err
	}
	return b.DeleteObject(key)
}

func (d *AliyunDriver) ListKeys(ctx context.Context, cfg ProviderConfig, prefix string, limit int) ([]string, error) {
	b, err := d.bucket(cfg)
	if err != nil {
		return nil, err
	}
	marker := ""
	var keys []string
	for {
		lor, err := b.ListObjects(oss.Prefix(prefix), oss.Marker(marker), oss.MaxKeys(limit))
		if err != nil {
			return nil, fmt.Errorf("aliyun ListObjects: %w", err)
		}
		for _, obj := range lor.Objects {
			keys = append(keys, obj.Key)
		}
		if !lor.IsTruncated {
			break
		}
		marker = lor.NextMarker
	}
	return keys, nil
}

func (d *AliyunDriver) PublicURL(cfg ProviderConfig, key string) string { return d.publicURL(cfg, key) }
func (d *AliyunDriver) publicURL(cfg ProviderConfig, key string) string {
	if cfg.CustomDomain != "" {
		return fmt.Sprintf("https://%s/%s", cfg.CustomDomain, key)
	}
	region := strings.TrimPrefix(cfg.Region, "oss-")
	return fmt.Sprintf("https://%s.oss-%s.aliyuncs.com/%s", cfg.Bucket, region, key)
}

// ── Tencent COS (腾讯云对象存储) ────────────────────────────────────────────

// TencentCOSDriver implements ProviderDriver for Tencent Cloud COS.
type TencentCOSDriver struct{}

func (d *TencentCOSDriver) cosClient(cfg ProviderConfig) *cos.Client {
	u, _ := cos.NewBucketURL(cfg.Bucket, cfg.Region, true)
	return cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		},
	})
}

func (d *TencentCOSDriver) Upload(ctx context.Context, cfg ProviderConfig, key string, body io.Reader, size int64, contentType string) (string, error) {
	client := d.cosClient(cfg)
	opt := &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: contentType}}
	_, err := client.Object.Put(ctx, key, body, opt)
	if err != nil {
		return "", fmt.Errorf("cos PutObject: %w", err)
	}
	return d.PublicURL(cfg, key), nil
}

func (d *TencentCOSDriver) Download(ctx context.Context, cfg ProviderConfig, key string) (io.ReadCloser, error) {
	client := d.cosClient(cfg)
	resp, err := client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("cos GetObject: %w", err)
	}
	return resp.Body, nil
}

func (d *TencentCOSDriver) Delete(ctx context.Context, cfg ProviderConfig, key string) error {
	client := d.cosClient(cfg)
	_, err := client.Object.Delete(ctx, key)
	return err
}

func (d *TencentCOSDriver) ListKeys(ctx context.Context, cfg ProviderConfig, prefix string, limit int) ([]string, error) {
	client := d.cosClient(cfg)
	opt := &cos.BucketGetOptions{Prefix: prefix, MaxKeys: limit}
	result, _, err := client.Bucket.Get(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("cos GetBucket: %w", err)
	}
	keys := make([]string, 0, len(result.Contents))
	for _, obj := range result.Contents {
		keys = append(keys, obj.Key)
	}
	return keys, nil
}

func (d *TencentCOSDriver) PublicURL(cfg ProviderConfig, key string) string {
	if cfg.CustomDomain != "" {
		return fmt.Sprintf("https://%s/%s", cfg.CustomDomain, key)
	}
	u, _ := cos.NewBucketURL(cfg.Bucket, cfg.Region, true)
	return fmt.Sprintf("%s/%s", strings.TrimRight(u.String(), "/"), key)
}

// ── Huawei OBS / Baidu BOS / Volcengine TOS / S3-compatible ─────────────────
// These providers all support the S3 API protocol. They share the S3Driver.
//
// Register with:
//   ossSvc.RegisterDriver("huawei_obs", &oss.S3Driver{})
//   ossSvc.RegisterDriver("baidu_bos", &oss.S3Driver{})
//   ossSvc.RegisterDriver("volcengine_tos", &oss.S3Driver{})
//   ossSvc.RegisterDriver("s3_compatible", &oss.S3Driver{})  // MinIO, R2, B2, etc.
//   ossSvc.RegisterDriver("minio", &oss.S3Driver{})
