package cache

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3CacheConfig struct {
	Bucket string
	Prefix string
	Region string
}

type s3Cache struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
	prefix   string
}

func NewS3Cache(ctx context.Context, cfg S3CacheConfig) (ThumbnailCache, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg)
	return &s3Cache{
		client:   client,
		uploader: manager.NewUploader(client),
		bucket:   cfg.Bucket,
		prefix:   cfg.Prefix,
	}, nil
}

func (c *s3Cache) Get(ctx context.Context, key string) ([]byte, error) {
	key = c.prefix + key
	result, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFoundErr *types.NoSuchKey
		if errors.As(err, &notFoundErr) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(result.Body)
}

func (c *s3Cache) Set(ctx context.Context, key string, data []byte) error {
	key = c.prefix + key
	_, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (c *s3Cache) Delete(ctx context.Context, key string) error {
	key = c.prefix + key
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *s3Cache) Exists(ctx context.Context, key string) (bool, error) {
	key = c.prefix + key
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	var notFoundErr *types.NoSuchKey
	if errors.As(err, &notFoundErr) {
		return false, nil
	}
	return false, err
}
