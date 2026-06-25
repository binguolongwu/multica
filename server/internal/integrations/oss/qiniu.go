package oss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

// QiniuDriver implements ProviderDriver for Qiniu Kodo (七牛云对象存储).
type QiniuDriver struct{}

func (d *QiniuDriver) mac(cfg ProviderConfig) *qbox.Mac {
	return qbox.NewMac(cfg.AccessKey, cfg.SecretKey)
}

func (d *QiniuDriver) bucketManager(cfg ProviderConfig) *storage.BucketManager {
	mac := d.mac(cfg)
	region := cfg.Region
	if region == "" {
		region = "z0" // default
	}
	reg, ok := storage.GetRegionByID(storage.RegionID(region))
	if !ok {
		reg, _ = storage.GetRegionByID("z0") // fallback
	}
	cfgObj := storage.Config{Region: &reg, UseHTTPS: true}
	return storage.NewBucketManager(mac, &cfgObj)
}

func (d *QiniuDriver) formUploader(cfg ProviderConfig) *storage.FormUploader {
	region := cfg.Region
	if region == "" {
		region = "z0"
	}
	reg, ok := storage.GetRegionByID(storage.RegionID(region))
	if !ok {
		reg, _ = storage.GetRegionByID("z0")
	}
	cfgObj := storage.Config{Region: &reg, UseHTTPS: true}
	return storage.NewFormUploader(&cfgObj)
}

func (d *QiniuDriver) Upload(ctx context.Context, cfg ProviderConfig, key string, body io.Reader, size int64, contentType string) (string, error) {
	upToken := d.token(cfg, cfg.Bucket, key)

	data, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("qiniu read body: %w", err)
	}

	uploader := d.formUploader(cfg)
	ret := storage.PutRet{}
	extra := storage.PutExtra{MimeType: contentType}
	err = uploader.Put(ctx, &ret, upToken, key, bytes.NewReader(data), size, &extra)
	if err != nil {
		return "", fmt.Errorf("qiniu upload: %w", err)
	}
	return d.PublicURL(cfg, key), nil
}

func (d *QiniuDriver) Download(ctx context.Context, cfg ProviderConfig, key string) (io.ReadCloser, error) {
	publicURL := d.PublicURL(cfg, key)
	return io.NopCloser(strings.NewReader(publicURL)), nil
}

func (d *QiniuDriver) Delete(ctx context.Context, cfg ProviderConfig, key string) error {
	bm := d.bucketManager(cfg)
	return bm.Delete(cfg.Bucket, key)
}

func (d *QiniuDriver) ListKeys(ctx context.Context, cfg ProviderConfig, prefix string, limit int) ([]string, error) {
	bm := d.bucketManager(cfg)
	entries, _, _, _, err := bm.ListFiles(cfg.Bucket, prefix, "", "", limit)
	if err != nil {
		return nil, fmt.Errorf("qiniu list: %w", err)
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.Key)
	}
	return keys, nil
}

func (d *QiniuDriver) PublicURL(cfg ProviderConfig, key string) string {
	domain := cfg.CustomDomain
	if domain == "" {
		domain = cfg.Bucket
	}
	return fmt.Sprintf("https://%s/%s", domain, key)
}

func (d *QiniuDriver) token(cfg ProviderConfig, bucket, key string) string {
	mac := d.mac(cfg)
	putPolicy := storage.PutPolicy{Scope: fmt.Sprintf("%s:%s", bucket, key)}
	return putPolicy.UploadToken(mac)
}
