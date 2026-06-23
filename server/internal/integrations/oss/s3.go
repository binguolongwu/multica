package oss

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Driver implements ProviderDriver for any S3-compatible object storage
// (AWS S3, MinIO, R2, B2, Wasabi, Tencent COS via S3 API, etc.).
type S3Driver struct{}

func (d *S3Driver) s3Client(ctx context.Context, cfg ProviderConfig) (*s3.Client, error) {
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))
	opts = append(opts, config.WithCredentialsProvider(
		credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	))

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})
	return client, nil
}

func (d *S3Driver) Upload(ctx context.Context, cfg ProviderConfig, key string, body io.Reader, size int64, contentType string) (string, error) {
	client, err := d.s3Client(ctx, cfg)
	if err != nil {
		return "", err
	}
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 PutObject: %w", err)
	}
	return d.publicURL(cfg, key), nil
}

func (d *S3Driver) Download(ctx context.Context, cfg ProviderConfig, key string) (io.ReadCloser, error) {
	client, err := d.s3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 GetObject: %w", err)
	}
	return out.Body, nil
}

func (d *S3Driver) Delete(ctx context.Context, cfg ProviderConfig, key string) error {
	client, err := d.s3Client(ctx, cfg)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	return err
}

func (d *S3Driver) ListKeys(ctx context.Context, cfg ProviderConfig, prefix string, limit int) ([]string, error) {
	client, err := d.s3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(cfg.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 ListObjectsV2: %w", err)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, obj := range out.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

func (d *S3Driver) PublicURL(cfg ProviderConfig, key string) string {
	if cfg.CustomDomain != "" {
		return fmt.Sprintf("https://%s/%s", cfg.CustomDomain, key)
	}
	if cfg.Endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", cfg.Endpoint, cfg.Bucket, key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.Bucket, cfg.Region, key)
}

func (d *S3Driver) publicURL(cfg ProviderConfig, key string) string {
	return d.PublicURL(cfg, key)
}
