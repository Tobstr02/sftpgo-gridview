package thumbnail

import (
	"context"
	"errors"
	"io"
	"time"

	cachepkg "github.com/drakkan/sftpgo/v2/internal/thumbnail/cache"
)

var (
	ErrSourceNotAccessible = errors.New("source file not accessible")
	ErrGenerationFailed    = errors.New("thumbnail generation failed")
)

type ThumbRequest struct {
	Provider string
	Bucket   string
	Path     string
	Size     int
	Mtime    int64
	Size_    int64
}

type ThumbResponse struct {
	CacheKey  string `json:"cache_key"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

type FileReader interface {
	GetFileReader(ctx context.Context, path string) (io.ReadCloser, error)
}

type ThumbnailService struct {
	generator *ImageGenerator
	cache     cachepkg.ThumbnailCache
	reader    FileReader
	keyGen    *cachepkg.KeyGenerator
	ttl       time.Duration
}

func NewThumbnailService(gen *ImageGenerator, cache cachepkg.ThumbnailCache, reader FileReader, ttl time.Duration) *ThumbnailService {
	return &ThumbnailService{
		generator: gen,
		cache:     cache,
		reader:    reader,
		keyGen:    cachepkg.NewKeyGenerator(),
		ttl:       ttl,
	}
}

func (s *ThumbnailService) GetThumbnail(ctx context.Context, req ThumbRequest) (*ThumbResponse, error) {
	key := s.keyGen.Generate(req.Provider, req.Bucket, req.Path, req.Mtime, req.Size_)

	_, err := s.cache.Get(ctx, key)
	if err == nil {
		return &ThumbResponse{
			CacheKey:  key,
			URL:       "/thumb/" + key,
			ExpiresAt: time.Now().Add(s.ttl).Format(time.RFC3339),
		}, nil
	}

	if !errors.Is(err, cachepkg.ErrNotFound) {
		return nil, err
	}

	file, err := s.reader.GetFileReader(ctx, req.Path)
	if err != nil {
		return nil, ErrSourceNotAccessible
	}
	defer file.Close()

	thumbData, err := s.generator.Generate(ctx, file)
	if err != nil {
		return nil, ErrGenerationFailed
	}

	if err := s.cache.Set(ctx, key, thumbData); err != nil {
		return nil, err
	}

	return &ThumbResponse{
		CacheKey:  key,
		URL:       "/thumb/" + key,
		ExpiresAt: time.Now().Add(s.ttl).Format(time.RFC3339),
	}, nil
}

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

func (s *ThumbnailService) DeleteThumbnail(ctx context.Context, key string) error {
	return s.cache.Delete(ctx, key)
}
