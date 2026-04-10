package thumbnail

import (
	"context"
	"time"

	cachepkg "github.com/drakkan/sftpgo/v2/internal/thumbnail/cache"
)

// ThumbnailService provides thumbnail generation and caching.
// It is a singleton that manages cache and generator, but file reading
// is done externally (by the HTTP handler using SFTPGo's Connection).
type ThumbnailService struct {
	generator *ImageGenerator
	cache     cachepkg.ThumbnailCache
	keyGen    *cachepkg.KeyGenerator
	ttl       time.Duration
}

// NewThumbnailService creates a new ThumbnailService.
func NewThumbnailService(gen *ImageGenerator, cache cachepkg.ThumbnailCache, ttl time.Duration) *ThumbnailService {
	return &ThumbnailService{
		generator: gen,
		cache:     cache,
		keyGen:    cachepkg.NewKeyGenerator(),
		ttl:       ttl,
	}
}

// GenerateCacheKey creates a cache key from file metadata.
func (s *ThumbnailService) GenerateCacheKey(provider, bucket, path string, mtime int64, size int64) string {
	return s.keyGen.Generate(provider, bucket, path, mtime, size)
}

// ValidateCacheKey checks if a cache key is valid.
func (s *ThumbnailService) ValidateCacheKey(key string) bool {
	return s.keyGen.ValidateKey(key)
}

// GetCachedThumbnail retrieves a cached thumbnail by key.
// Returns (thumbnailData, contentType, error).
func (s *ThumbnailService) GetCachedThumbnail(ctx context.Context, key string) ([]byte, string, error) {
	if !s.keyGen.ValidateKey(key) {
		return nil, "", cachepkg.ErrInvalidKey
	}

	data, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, "", err
	}

	return data, "image/jpeg", nil
}

// CacheThumbnail stores a generated thumbnail in the cache.
func (s *ThumbnailService) CacheThumbnail(ctx context.Context, key string, data []byte) error {
	return s.cache.Set(ctx, key, data)
}

// DeleteThumbnail removes a thumbnail from the cache.
func (s *ThumbnailService) DeleteThumbnail(ctx context.Context, key string) error {
	return s.cache.Delete(ctx, key)
}
