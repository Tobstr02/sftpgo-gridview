package config

import "time"

type ThumbnailConfig struct {
	Enabled bool                 `json:"enabled" mapstructure:"enabled"`
	Size    int                  `json:"size" mapstructure:"size"`
	Cache   ThumbnailCacheConfig `json:"cache" mapstructure:"cache"`
}

type ThumbnailCacheConfig struct {
	Backend string               `json:"backend" mapstructure:"backend"`
	S3      S3ThumbnailConfig    `json:"s3" mapstructure:"s3"`
	Local   LocalThumbnailConfig `json:"local" mapstructure:"local"`
	TTL     time.Duration        `json:"ttl" mapstructure:"ttl"`
}

type S3ThumbnailConfig struct {
	Bucket string `json:"bucket" mapstructure:"bucket"`
	Prefix string `json:"prefix" mapstructure:"prefix"`
	Region string `json:"region" mapstructure:"region"`
}

type LocalThumbnailConfig struct {
	Path string `json:"path" mapstructure:"path"`
}

const (
	defaultThumbnailSize    = 256
	defaultThumbnailTTL     = 720 * time.Hour // 30 days
	defaultThumbnailBackend = "local"
)
