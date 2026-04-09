package thumbnail

import (
	"context"
	"errors"
	"io"
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
