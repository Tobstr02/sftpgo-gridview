package cache

import (
	"context"
	"errors"
)

var (
	ErrNotFound   = errors.New("thumbnail not found")
	ErrInvalidKey = errors.New("invalid cache key")
)

type ThumbnailCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

type MultiBackendCache struct {
	primary  ThumbnailCache
	fallback ThumbnailCache
}

func NewMultiBackendCache(primary, fallback ThumbnailCache) *MultiBackendCache {
	return &MultiBackendCache{
		primary:  primary,
		fallback: fallback,
	}
}

func (c *MultiBackendCache) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := c.primary.Get(ctx, key)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return c.fallback.Get(ctx, key)
	}
	return nil, ErrNotFound
}

func (c *MultiBackendCache) Set(ctx context.Context, key string, data []byte) error {
	if err := c.primary.Set(ctx, key, data); err != nil {
		return err
	}
	return c.fallback.Set(ctx, key, data)
}

func (c *MultiBackendCache) Delete(ctx context.Context, key string) error {
	if err := c.primary.Delete(ctx, key); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return c.fallback.Delete(ctx, key)
}

func (c *MultiBackendCache) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.primary.Exists(ctx, key)
	if err == nil && exists {
		return true, nil
	}
	return c.fallback.Exists(ctx, key)
}
