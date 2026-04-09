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
