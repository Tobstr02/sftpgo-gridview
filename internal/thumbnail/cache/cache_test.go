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
