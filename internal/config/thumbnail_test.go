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
