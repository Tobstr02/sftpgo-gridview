# Grid View with Thumbnails Implementation Plan

**Goal:** Implement grid view with thumbnails feature for SFTPGo WebClient

**Architecture:** Server-side thumbnail generation with multi-backend cache (S3 + local disk), grid view frontend with lazy loading

**Design:** `thoughts/shared/designs/2026-04-09-gridview-thumbnails-design.md`

---

## Dependency Graph

```
Batch 1 (parallel): 1.1, 1.2, 1.3 [foundation - no deps]
Batch 2 (parallel): 2.1, 2.2, 2.3, 2.4 [core - depends on batch 1]
Batch 3 (parallel): 3.1, 3.2, 3.3, 3.4, 3.5 [frontend - no deps on batch 2]
Batch 4 (parallel): 4.1 [integration - depends on batch 2 and 3]
```

---

## Batch 1: Foundation (parallel - 3 implementers)

### Task 1.1: Thumbnail Config
**File:** `internal/config/thumbnail.go`
**Test:** `internal/config/thumbnail_test.go`
**Depends:** none

```go
// config/thumbnail.go
package config

import "time"

type ThumbnailConfig struct {
    Enabled bool `json:"enabled" mapstructure:"enabled"`
    Size    int  `json:"size" mapstructure:"size"`
    Cache   ThumbnailCacheConfig `json:"cache" mapstructure:"cache"`
}

type ThumbnailCacheConfig struct {
    Backend string            `json:"backend" mapstructure:"backend"`
    S3      S3ThumbnailConfig `json:"s3" mapstructure:"s3"`
    Local   LocalThumbnailConfig `json:"local" mapstructure:"local"`
    TTL     time.Duration     `json:"ttl" mapstructure:"ttl"`
}

type S3ThumbnailConfig struct {
    Bucket  string `json:"bucket" mapstructure:"bucket"`
    Prefix  string `json:"prefix" mapstructure:"prefix"`
    Region  string `json:"region" mapstructure:"region"`
}

type LocalThumbnailConfig struct {
    Path string `json:"path" mapstructure:"path"`
}

const (
    defaultThumbnailSize = 256
    defaultThumbnailTTL  = 720 * time.Hour // 30 days
    defaultThumbnailBackend = "local"
)
```

```go
// config/thumbnail_test.go
package config

import (
    "testing"
    "time"
)

func TestThumbnailConfig(t *testing.T) {
    cfg := ThumbnailConfig{
        Enabled: true,
        Size:    256,
        Cache: ThumbnailCacheConfig{
            Backend: "local",
            TTL:     720 * time.Hour,
        },
    }
    if !cfg.Enabled {
        t.Error("Expected Enabled to be true")
    }
    if cfg.Size != 256 {
        t.Errorf("Expected Size to be 256, got %d", cfg.Size)
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/config/... -run Thumbnail`
**Commit:** `feat(thumbnail): add thumbnail configuration`

---

### Task 1.2: Cache Interface
**File:** `internal/thumbnail/cache/cache.go`
**Test:** `internal/thumbnail/cache/cache_test.go`
**Depends:** none

```go
// thumbnail/cache/cache.go
package cache

import (
    "context"
    "errors"
)

var (
    ErrNotFound = errors.New("thumbnail not found")
    ErrInvalidKey = errors.New("invalid cache key")
)

type ThumbnailCache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, data []byte) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}

type MultiBackendCache struct {
    primary   ThumbnailCache
    fallback ThumbnailCache
}

func NewMultiBackendCache(primary, fallback ThumbnailCache) *MultiBackendCache {
    return &MultiBackendCache{
        primary:   primary,
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
```

```go
// thumbnail/cache/cache_test.go
package cache

import (
    "context"
    "testing"
)

type mockCache struct {
    data map[string][]byte
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
    if val, ok := m.data[key]; ok {
        return val, nil
    }
    return nil, ErrNotFound
}

func (m *mockCache) Set(ctx context.Context, key string, data []byte) error {
    m.data[key] = data
    return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
    delete(m.data, key)
    return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
    _, ok := m.data[key]
    return ok, nil
}

func TestMultiBackendCache(t *testing.T) {
    primary := &mockCache{data: make(map[string][]byte)}
    fallback := &mockCache{data: make(map[string][]byte)}
    cache := NewMultiBackendCache(primary, fallback)

    ctx := context.Background()
    testData := []byte("test thumbnail")

    err := cache.Set(ctx, "key1", testData)
    if err != nil {
        t.Fatalf("Set failed: %v", err)
    }

    result, err := cache.Get(ctx, "key1")
    if err != nil {
        t.Fatalf("Get failed: %v", err)
    }
    if string(result) != string(testData) {
        t.Errorf("Expected %s, got %s", testData, result)
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/thumbnail/cache/...`
**Commit:** `feat(thumbnail): add multi-backend cache interface`

---

### Task 1.3: Cache Key Generator
**File:** `internal/thumbnail/cache/key.go`
**Test:** `internal/thumbnail/cache/key_test.go`
**Depends:** none

```go
// thumbnail/cache/key.go
package cache

import (
    "crypto/sha256"
    "fmt"
)

type KeyGenerator struct{}

func NewKeyGenerator() *KeyGenerator {
    return &KeyGenerator{}
}

func (g *KeyGenerator) Generate(provider, bucket, filePath string, mtime int64, size int64) string {
    input := fmt.Sprintf("%s:%s%s:%d:%d", provider, bucket, filePath, mtime, size)
    hash := sha256.Sum256([]byte(input))
    return fmt.Sprintf("sha256:%x", hash)
}

func (g *KeyGenerator) ValidateKey(key string) bool {
    if len(key) < 7 || key[:7] != "sha256:" {
        return false
    }
    return len(key[7:]) == 64
}
```

```go
// thumbnail/cache/key_test.go
package cache

import (
    "testing"
)

func TestKeyGenerator(t *testing.T) {
    gen := NewKeyGenerator()

    key := gen.Generate("s3", "files", "/photos/vacation.jpg", 1709234567, 2048000)
    if !gen.ValidateKey(key) {
        t.Errorf("Key should be valid: %s", key)
    }

    if gen.ValidateKey("invalid") {
        t.Error("Invalid key should fail validation")
    }

    key2 := gen.Generate("s3", "files", "/photos/vacation.jpg", 1709234567, 2048000)
    if key != key2 {
        t.Error("Same inputs should produce same key")
    }

    key3 := gen.Generate("s3", "files", "/photos/vacation.jpg", 9999999999, 2048000)
    if key == key3 {
        t.Error("Different mtime should produce different key")
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/thumbnail/cache/... -run Key`
**Commit:** `feat(thumbnail): add cache key generator`

---

## Batch 2: Core Modules (parallel - 4 implementers)

All tasks depend on Batch 1 completing.

### Task 2.1: Local Disk Cache Backend
**File:** `internal/thumbnail/cache/local.go`
**Test:** `internal/thumbnail/cache/local_test.go`
**Depends:** 1.1, 1.2, 1.3

```go
// thumbnail/cache/local.go
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
        ttl:     cfg.TTL,
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
```

```go
// thumbnail/cache/local_test.go
package cache

import (
    "context"
    "os"
    "testing"
    "time"
)

func TestLocalCache(t *testing.T) {
    tmpDir := t.TempDir()
    cfg := LocalCacheConfig{
        BasePath: tmpDir,
        TTL:      720 * time.Hour,
    }
    cache, err := NewLocalCache(cfg)
    if err != nil {
        t.Fatalf("NewLocalCache failed: %v", err)
    }

    ctx := context.Background()
    testData := []byte("test thumbnail data")

    err = cache.Set(ctx, "testkey123", testData)
    if err != nil {
        t.Fatalf("Set failed: %v", err)
    }

    result, err := cache.Get(ctx, "testkey123")
    if err != nil {
        t.Fatalf("Get failed: %v", err)
    }
    if string(result) != string(testData) {
        t.Errorf("Expected %s, got %s", testData, result)
    }

    exists, err := cache.Exists(ctx, "testkey123")
    if err != nil {
        t.Fatalf("Exists failed: %v", err)
    }
    if !exists {
        t.Error("Key should exist")
    }

    err = cache.Delete(ctx, "testkey123")
    if err != nil {
        t.Fatalf("Delete failed: %v", err)
    }

    _, err = cache.Get(ctx, "testkey123")
    if err == nil {
        t.Error("Expected ErrNotFound after delete")
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/thumbnail/cache/... -run Local`
**Commit:** `feat(thumbnail): add local disk cache backend`

---

### Task 2.2: S3 Cache Backend
**File:** `internal/thumbnail/cache/s3.go`
**Test:** `internal/thumbnail/cache/s3_test.go`
**Depends:** 1.1, 1.2, 1.3

```go
// thumbnail/cache/s3.go
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
)

type S3CacheConfig struct {
    Bucket   string
    Prefix   string
    Region   string
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
        var notFoundErr *s3.Types.NoSuchKey
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
    var notFoundErr *s3.Types.NoSuchKey
    if errors.As(err, &notFoundErr) {
        return false, nil
    }
    return false, err
}
```

```go
// thumbnail/cache/s3_test.go
package cache

import (
    "context"
    "testing"
)

type mockS3Cache struct {
    data map[string][]byte
}

func (m *mockS3Cache) Get(ctx context.Context, key string) ([]byte, error) {
    if val, ok := m.data[key]; ok {
        return val, nil
    }
    return nil, ErrNotFound
}

func (m *mockS3Cache) Set(ctx context.Context, key string, data []byte) error {
    m.data[key] = data
    return nil
}

func (m *mockS3Cache) Delete(ctx context.Context, key string) error {
    delete(m.data, key)
    return nil
}

func (m *mockS3Cache) Exists(ctx context.Context, key string) (bool, error) {
    _, ok := m.data[key]
    return ok, nil
}

func TestS3CacheMock(t *testing.T) {
    cache := &mockS3Cache{data: make(map[string][]byte)}
    ctx := context.Background()
    testData := []byte("s3 thumbnail")

    err := cache.Set(ctx, "s3key", testData)
    if err != nil {
        t.Fatalf("Set failed: %v", err)
    }

    result, err := cache.Get(ctx, "s3key")
    if err != nil {
        t.Fatalf("Get failed: %v", err)
    }
    if string(result) != string(testData) {
        t.Errorf("Expected %s, got %s", testData, result)
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/thumbnail/cache/... -run S3`
**Commit:** `feat(thumbnail): add S3 cache backend`

---

### Task 2.3: Image Generator
**File:** `internal/thumbnail/generator.go`
**Test:** `internal/thumbnail/generator_test.go`
**Depends:** 1.1, 1.2

```go
// thumbnail/generator.go
package thumbnail

import (
    "bytes"
    "context"
    "errors"
    "io"
    "mime"
    "net/http"
    "strings"

    "github.com/disintegration/imaging"
)

var (
    ErrUnsupportedFormat = errors.New("unsupported image format")
    ErrImageDecode       = errors.New("failed to decode image")
)

type ImageGenerator struct {
    maxSize int
}

func NewImageGenerator(maxSize int) *ImageGenerator {
    return &ImageGenerator{maxSize: maxSize}
}

var supportedFormats = map[string]imaging.Format{
    ".jpeg": imaging.JPEG,
    ".jpg":  imaging.JPEG,
    ".png":  imaging.PNG,
    ".gif":  imaging.GIF,
    ".webp": imaging.WEBP,
    ".bmp":  imaging.BMP,
}

func (g *ImageGenerator) Generate(ctx context.Context, input io.Reader) ([]byte, error) {
    data, err := io.ReadAll(input)
    if err != nil {
        return nil, err
    }

    format := g.detectFormat(data)
    if format == imaging.UNKNOWN {
        return nil, ErrUnsupportedFormat
    }

    img, err := imaging.Decode(bytes.NewReader(data))
    if err != nil {
        return nil, ErrImageDecode
    }

    thumb := imaging.Thumbnail(img, g.maxSize, g.maxSize, imaging.Lanczos)

    var buf bytes.Buffer
    err = imaging.Encode(&buf, thumb, imaging.JPEG, imaging.JPEGQuality(85))
    if err != nil {
        return nil, err
    }

    return buf.Bytes(), nil
}

func (g *ImageGenerator) detectFormat(data []byte) imaging.Format {
    contentType := http.DetectContentType(data)
    ext := mime.ExtensionsByType(contentType)
    if len(ext) == 0 {
        return imaging.UNKNOWN
    }
    format, ok := supportedFormats[strings.ToLower(ext[0])]
    if !ok {
        return imaging.UNKNOWN
    }
    return format
}

func (g *ImageGenerator) IsFormatSupported(filename string) bool {
    idx := strings.LastIndex(filename, ".")
    if idx < 0 {
        return false
    }
    ext := strings.ToLower(filename[idx:])
    _, ok := supportedFormats[ext]
    return ok
}
```

```go
// thumbnail/generator_test.go
package thumbnail

import (
    "bytes"
    "context"
    "testing"
)

func TestImageGenerator(t *testing.T) {
    gen := NewImageGenerator(256)

    if !gen.IsFormatSupported("test.jpg") {
        t.Error("jpg should be supported")
    }
    if !gen.IsFormatSupported("test.png") {
        t.Error("png should be supported")
    }
    if gen.IsFormatSupported("test.txt") {
        t.Error("txt should not be supported")
    }

    testImg := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
    _, err := gen.Generate(context.Background(), bytes.NewReader(testImg))
    if err == nil {
        t.Error("Should fail for invalid image data")
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/thumbnail/... -run Generator`
**Commit:** `feat(thumbnail): add image generator`

---

### Task 2.4: Thumbnail Service
**File:** `internal/thumbnail/service.go`
**Test:** `internal/thumbnail/service_test.go`
**Depends:** 2.1, 2.2, 2.3

```go
// thumbnail/service.go
package thumbnail

import (
    "context"
    "errors"
    "io"
    "time"

    "github.com/drakkan/sftpgo/v2/internal/thumbnail/cache"
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
    CacheKey string `json:"cache_key"`
    URL      string `json:"url"`
    ExpiresAt string `json:"expires_at"`
}

type FileReader interface {
    GetFileReader(ctx context.Context, path string) (io.ReadCloser, error)
}

type ThumbnailService struct {
    generator *ImageGenerator
    cache     cache.ThumbnailCache
    reader    FileReader
    keyGen    *cache.KeyGenerator
    ttl       time.Duration
}

func NewThumbnailService(gen *ImageGenerator, cache cache.ThumbnailCache, reader FileReader, ttl time.Duration) *ThumbnailService {
    return &ThumbnailService{
        generator: gen,
        cache:    cache,
        reader:   reader,
        keyGen:   cache.NewKeyGenerator(),
        ttl:      ttl,
    }
}

func (s *ThumbnailService) GetThumbnail(ctx context.Context, req ThumbRequest) (*ThumbResponse, error) {
    key := s.keyGen.Generate(req.Provider, req.Bucket, req.Path, req.Mtime, req.Size_)

    data, err := s.cache.Get(ctx, key)
    if err == nil {
        return &ThumbResponse{
            CacheKey: key,
            URL:      "/thumb/" + key,
            ExpiresAt: time.Now().Add(s.ttl).Format(time.RFC3339),
        }, nil
    }

    if !errors.Is(err, cache.ErrNotFound) {
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
        CacheKey: key,
        URL:      "/thumb/" + key,
        ExpiresAt: time.Now().Add(s.ttl).Format(time.RFC3339),
    }, nil
}

func (s *ThumbnailService) GetCachedThumbnail(ctx context.Context, key string) ([]byte, string, error) {
    if !s.keyGen.ValidateKey(key) {
        return nil, "", cache.ErrInvalidKey
    }

    data, err := s.cache.Get(ctx, key)
    if err != nil {
        return nil, "", err
    }

    return data, "image/jpeg", nil
}
```

```go
// thumbnail/service_test.go
package thumbnail

import (
    "context"
    "errors"
    "testing"
    "time"
)

type mockFileReader struct{}

func (m *mockFileReader) GetFileReader(ctx context.Context, path string) (io.ReadCloser, error) {
    return nil, errors.New("not implemented")
}

type mockCache struct {
    data map[string][]byte
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
    if val, ok := m.data[key]; ok {
        return val, nil
    }
    return nil, errors.New("not found")
}

func (m *mockCache) Set(ctx context.Context, key string, data []byte) error {
    m.data[key] = data
    return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
    delete(m.data, key)
    return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
    _, ok := m.data[key]
    return ok, nil
}

func TestThumbnailServiceCache(t *testing.T) {
    gen := NewImageGenerator(256)
    mockCache := &mockCache{data: make(map[string][]byte)}
    svc := NewThumbnailService(gen, mockCache, &mockFileReader{}, 720*time.Hour)

    req := ThumbRequest{
        Provider: "local",
        Bucket:   "",
        Path:     "/test.jpg",
        Mtime:    1234567890,
        Size_:    1024,
    }

    ctx := context.Background()
    resp, err := svc.GetThumbnail(ctx, req)
    if err == nil {
        t.Log("Got response:", resp)
    }

    data, contentType, err := svc.GetCachedThumbnail(ctx, "sha256:abc123")
    if err != nil {
        t.Log("GetCachedThumbnail failed as expected for invalid key")
    } else {
        t.Log("Got cached thumbnail:", contentType, len(data))
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/thumbnail/... -run Service`
**Commit:** `feat(thumbnail): add thumbnail service`

---

## Batch 3: Frontend Components (parallel - 5 implementers)

Tasks in this batch do NOT depend on Batch 2 completing - they can start immediately.

### Task 3.1: Grid View CSS
**File:** `static/webclient/css/grid-view.css`
**Test:** none
**Depends:** none

```css
/* webclient/css/grid-view.css */

.grid-view {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 16px;
    padding: 16px;
}

.thumbnail-cell {
    aspect-ratio: 1;
    border-radius: 8px;
    overflow: hidden;
    background: var(--kt-gray-100);
    position: relative;
    cursor: pointer;
    transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.thumbnail-cell:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.thumbnail-cell img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.thumbnail-cell .filename {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 8px;
    background: linear-gradient(transparent, rgba(0, 0, 0, 0.7));
    color: white;
    font-size: 12px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.thumbnail-cell .skeleton {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: linear-gradient(90deg, var(--kt-gray-100) 25%, var(--kt-gray-200) 50%, var(--kt-gray-100) 75%);
    background-size: 200% 100%;
    animation: skeleton-shimmer 1.5s infinite;
}

@keyframes skeleton-shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
}

.thumbnail-cell .error-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--kt-gray-500);
}

.thumbnail-cell .error-state i {
    font-size: 32px;
    margin-bottom: 4px;
}

@media (max-width: 576px) {
    .grid-view {
        grid-template-columns: repeat(2, 1fr);
        gap: 8px;
        padding: 8px;
    }
}

@media (min-width: 576px) and (max-width: 768px) {
    .grid-view {
        grid-template-columns: repeat(3, 1fr);
    }
}

@media (min-width: 768px) and (max-width: 1024px) {
    .grid-view {
        grid-template-columns: repeat(4, 1fr);
    }
}
```

**Verify:** File created, no verification command needed
**Commit:** `feat(thumbnail): add grid view CSS styles`

---

### Task 3.2: View Toggle Component
**File:** `static/webclient/js/view-toggle.js`
**Test:** none
**Depends:** none

```javascript
// webclient/js/view-toggle.js
const ViewToggle = {
    STORAGE_KEY: 'filesViewMode',
    VIEW_LIST: 'list',
    VIEW_GRID: 'grid',
    currentView: null,

    init() {
        this.currentView = localStorage.getItem(this.STORAGE_KEY) || this.VIEW_GRID;
        this.render();
        this.bindEvents();
    },

    render() {
        const container = document.getElementById('view-toggle');
        if (!container) return;

        container.innerHTML = `
            <div class="btn-group" role="group">
                <button type="button" class="btn btn-icon ${this.currentView === this.VIEW_LIST ? 'btn-primary' : 'btn-light'}" 
                        data-view="${this.VIEW_LIST}" title="List view">
                    <i class="ki-duotone ki-row-vertical fs-2"></i>
                </button>
                <button type="button" class="btn btn-icon ${this.currentView === this.VIEW_GRID ? 'btn-primary' : 'btn-light'}" 
                        data-view="${this.VIEW_GRID}" title="Grid view">
                    <i class="ki-duotone ki-grid fs-2"></i>
                </button>
            </div>
        `;
    },

    bindEvents() {
        const container = document.getElementById('view-toggle');
        if (!container) return;

        container.addEventListener('click', (e) => {
            const btn = e.target.closest('[data-view]');
            if (!btn) return;
            this.setView(btn.dataset.view);
        });
    },

    setView(view) {
        if (this.currentView === view) return;
        this.currentView = view;
        localStorage.setItem(this.STORAGE_KEY, view);
        this.render();
        document.dispatchEvent(new CustomEvent('viewChanged', { detail: { view } }));
    },

    getView() {
        return this.currentView;
    }
};

document.addEventListener('DOMContentLoaded', () => ViewToggle.init());
```

**Verify:** File created
**Commit:** `feat(thumbnail): add view toggle component`

---

### Task 3.3: Thumbnail Loader (Lazy Loading)
**File:** `static/webclient/js/thumbnail-loader.js`
**Test:** none
**Depends:** none

```javascript
// webclient/js/thumbnail-loader.js
const ThumbnailLoader = {
    observer: null,
    thumbURL: '/thumb/generate',
    cacheURL: '/thumb/',

    init() {
        this.setupIntersectionObserver();
    },

    setupIntersectionObserver() {
        this.observer = new IntersectionObserver(
            (entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        this.loadThumbnail(entry.target);
                        this.observer.unobserve(entry.target);
                    }
                });
            },
            { rootMargin: '200px' }
        );
    },

    observeAll() {
        const cells = document.querySelectorAll('.thumbnail-cell[data-thumb-key]');
        cells.forEach(cell => {
            if (!cell.querySelector('img[src]') && !cell.querySelector('.error-state')) {
                this.observer.observe(cell);
            }
        });
    },

    loadThumbnail(cell) {
        const key = cell.dataset.thumbKey;
        const cacheKey = cell.dataset.cacheKey;
        const url = cell.dataset.url;

        if (cacheKey) {
            this.showImage(cell, this.cacheURL + cacheKey);
            return;
        }

        this.requestGeneration(cell, url, key);
    },

    showImage(cell, src) {
        const skeleton = cell.querySelector('.skeleton');
        if (skeleton) skeleton.remove();

        let img = cell.querySelector('img');
        if (!img) {
            img = document.createElement('img');
            img.alt = cell.dataset.filename || 'Thumbnail';
            cell.appendChild(img);
        }

        img.onload = () => cell.classList.add('loaded');
        img.onerror = () => this.showError(cell, 'Failed to load image');
        img.src = src;
    },

    showError(cell, message) {
        cell.innerHTML = `
            <div class="error-state">
                <i class="ki-duotone ki-warning text-danger fs-2"></i>
                <span class="fs-7">${message}</span>
            </div>
        `;
    },

    async requestGeneration(cell, fileURL, key) {
        try {
            const response = await fetch(this.thumbURL, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-TOKEN': window csrfToken 
                },
                body: JSON.stringify({ url: fileURL })
            });

            if (!response.ok) {
                this.showError(cell, 'Generation failed');
                return;
            }

            const data = await response.json();
            cell.dataset.cacheKey = data.cache_key;
            this.showImage(cell, this.cacheURL + data.cache_key);
        } catch (err) {
            this.showError(cell, 'Request failed');
        }
    }
};

document.addEventListener('DOMContentLoaded', () => ThumbnailLoader.init());
```

**Verify:** File created
**Commit:** `feat(thumbnail): add thumbnail lazy loader`

---

### Task 3.4: Grid View Component
**File:** `static/webclient/js/grid-view.js`
**Test:** none
**Depends:** none

```javascript
// webclient/js/grid-view.js
const GridView = {
    container: null,
    items: [],

    init(containerSelector) {
        this.container = document.querySelector(containerSelector);
        if (!this.container) return;

        this.setupViewListener();
        this.render();
    },

    setupViewListener() {
        document.addEventListener('viewChanged', (e) => {
            if (e.detail.view === 'grid') {
                this.show();
            } else {
                this.hide();
            }
        });
    },

    render() {
        if (ViewToggle.getView() === 'grid') {
            this.show();
        }
    },

    show() {
        if (this.container) {
            this.container.classList.remove('d-none');
        }
        ThumbnailLoader.observeAll();
    },

    hide() {
        if (this.container) {
            this.container.classList.add('d-none');
        }
    },

    setItems(items) {
        this.items = items;
        this.renderItems();
    },

    renderItems() {
        if (!this.container || ViewToggle.getView() !== 'grid') return;

        this.container.innerHTML = this.items.map(item => this.createCellHTML(item)).join('');
        ThumbnailLoader.observeAll();
    },

    createCellHTML(item) {
        const filename = this.escapeHTML(item.name);
        const isImage = this.isImageFile(item.name);
        const cacheKey = item.thumb_cache_key || '';
        const url = item.url || '';

        if (!isImage) {
            return `
                <div class="thumbnail-cell" data-filename="${filename}">
                    <div class="error-state">
                        <i class="ki-duotone ki-file fs-2"></i>
                        <span class="fs-7">${filename}</span>
                    </div>
                </div>
            `;
        }

        return `
            <div class="thumbnail-cell" data-filename="${filename}" data-cache-key="${cacheKey}" data-url="${url}">
                <div class="skeleton"></div>
                <span class="filename">${filename}</span>
            </div>
        `;
    },

    isImageFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].includes(ext);
    },

    escapeHTML(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};
```

**Verify:** File created
**Commit:** `feat(thumbnail): add grid view component`

---

### Task 3.5: Files Page Integration
**File:** `templates/webclient/files.html` (modifications)
**Test:** none
**Depends:** 3.1, 3.2, 3.3, 3.4

Modifications to `templates/webclient/files.html`:

1. Add CSS link in `extra_css` block:
```html
{{- define "extra_css"}}
<link href="{{.StaticURL}}/assets/plugins/custom/datatables/datatables.bundle.css" rel="stylesheet" type="text/css"/>
<link href="{{.StaticURL}}/vendor/imageviewer/imageviewer.css" rel="stylesheet" type="text/css"/>
<link href="{{.StaticURL}}/webclient/css/grid-view.css" rel="stylesheet" type="text/css"/>
{{- end}}
```

2. Add view toggle button (in card-header toolbar after search):
```html
<div id="view-toggle" class="me-3"></div>
```

3. Add grid view container (after file_manager_list_container):
```html
<div id="grid_view_container" class="grid-view d-none"></div>
```

4. Add JS includes and initialization:
```html
{{- define "extra_js"}}
<script src="{{.StaticURL}}/assets/plugins/custom/datatables/datatables.bundle.js"></script>
<script src="{{.StaticURL}}/vendor/imageviewer/imageviewer.js"></script>
<script src="{{.StaticURL}}/vendor/pdfobject/pdfobject.min.js"></script>
<script src="{{.StaticURL}}/webclient/js/view-toggle.js"></script>
<script src="{{.StaticURL}}/webclient/js/thumbnail-loader.js"></script>
<script src="{{.StaticURL}}/webclient/js/grid-view.js"></script>
<script type="text/javascript">
    // Initialize grid view after page load
    document.addEventListener('DOMContentLoaded', function() {
        GridView.init('#grid_view_container');
    });
</script>
{{- end}}
```

**Verify:** Template renders correctly
**Commit:** `feat(thumbnail): integrate grid view into files page`

---

## Batch 4: HTTP Handlers & Integration (parallel - 1 implementer)

### Task 4.1: HTTP Handlers + Route Registration
**File:** `internal/thumbnail/handlers.go`
**Test:** `internal/thumbnail/handlers_test.go`
**Depends:** 2.4, 3.5

```go
// thumbnail/handlers.go
package thumbnail

import (
    "context"
    "net/http"
    "strings"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/render"

    "github.com/drakkan/sftpgo/v2/internal/logger"
)

const (
    thumbBasePath = "/thumb"
    thumbGetPath  = thumbBasePath + "/{key}"
    thumbGenPath  = thumbBasePath + "/generate"
)

type Handlers struct {
    service *ThumbnailService
}

func NewHandlers(svc *ThumbnailService) *Handlers {
    return &Handlers{service: svc}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
    r.Get(thumbGetPath, h.handleGetThumbnail)
    r.Post(thumbGenPath, h.handleGenerateThumbnail)
    r.Delete(thumbGetPath, h.handleDeleteThumbnail)
}

func (h *Handlers) handleGetThumbnail(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    key := chi.URLParam(r, "key")

    data, contentType, err := h.service.GetCachedThumbnail(ctx, key)
    if err != nil {
        if err.Error() == "thumbnail not found" {
            http.Error(w, "Not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", contentType)
    w.Header().Set("Cache-Control", "public, max-age=31536000")
    w.Header().Set("ETag", key)
    w.Write(data)
}

type generateRequest struct {
    URL string `json:"url"`
}

func (h *Handlers) handleGenerateThumbnail(w http.ResponseWriter, r *http.Request) {
    var req generateRequest
    if err := render.DecodeJSON(r, &req); err != nil {
        render.JSON(w, r, map[string]string{"error": "invalid request"})
        return
    }

    ctx := context.Background()
    resp, err := h.service.GetThumbnail(ctx, ThumbRequest{
        Path: req.URL,
    })
    if err != nil {
        render.JSON(w, r, map[string]string{"error": err.Error()})
        return
    }

    render.JSON(w, r, resp)
}

func (h *Handlers) handleDeleteThumbnail(w http.ResponseWriter, r *http.Request) {
    key := chi.URLParam(r, "key")
    ctx := context.Background()
    
    if err := h.service.DeleteThumbnail(ctx, key); err != nil {
        logger.Warn("thumbnail", "", "Failed to delete thumbnail: %v", err)
    }
    
    w.WriteHeader(http.StatusNoContent)
}
```

```go
// thumbnail/handlers_test.go
package thumbnail

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHandlers(t *testing.T) {
    // Test route registration
    handlers := NewHandlers(nil)
    if handlers == nil {
        t.Error("NewHandlers should not return nil")
    }

    // Test request/response structures
    req := generateRequest{URL: "/test/path"}
    if req.URL != "/test/path" {
        t.Errorf("Expected URL /test/path, got %s", req.URL)
    }
}
```

**Verify:** `cd /home/devuser/projects/sftpgo-gridview && go test ./internal/thumbnail/...`
**Commit:** `feat(thumbnail): add HTTP handlers and route registration`

---

## Summary

| Batch | Tasks | Dependencies |
|-------|-------|--------------|
| 1 | 1.1 Config, 1.2 Cache Interface, 1.3 Key Generator | None |
| 2 | 2.1 Local Cache, 2.2 S3 Cache, 2.3 Generator, 2.4 Service | 1.1, 1.2, 1.3 |
| 3 | 3.1 CSS, 3.2 ViewToggle, 3.3 Loader, 3.4 GridView, 3.5 Template | None |
| 4 | 4.1 HTTP Handlers | 2.4, 3.5 |

**Total:** 14 micro-tasks
**Parallel batches:** 4 batches
**Estimated implementers:** 8-14 (one per task in batches 1-3, one for batch 4)