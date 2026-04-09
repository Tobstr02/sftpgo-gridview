package cache

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

type LocalCacheConfig struct {
	BasePath string
	TTL      time.Duration
}

type localCache struct {
	basePath string
	ttl      time.Duration
}

func NewLocalCache(cfg LocalCacheConfig) (ThumbnailCache, error) {
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, err
	}
	return &localCache{
		basePath: cfg.BasePath,
		ttl:      cfg.TTL,
	}, nil
}

func (c *localCache) Get(ctx context.Context, key string) ([]byte, error) {
	filePath := c.getFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

func (c *localCache) Set(ctx context.Context, key string, data []byte) error {
	filePath := c.getFilePath(key)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func (c *localCache) Delete(ctx context.Context, key string) error {
	filePath := c.getFilePath(key)
	err := os.Remove(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c *localCache) Exists(ctx context.Context, key string) (bool, error) {
	filePath := c.getFilePath(key)
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (c *localCache) getFilePath(key string) string {
	return filepath.Join(c.basePath, key[:3], key[3:6], key)
}
