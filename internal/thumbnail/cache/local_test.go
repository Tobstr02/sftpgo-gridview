package cache

import (
	"context"
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
