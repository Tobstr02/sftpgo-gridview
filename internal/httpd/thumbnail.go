package httpd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/drakkan/sftpgo/v2/internal/common"
	"github.com/drakkan/sftpgo/v2/internal/dataprovider"
	"github.com/drakkan/sftpgo/v2/internal/jwt"
	"github.com/drakkan/sftpgo/v2/internal/logger"
	"github.com/drakkan/sftpgo/v2/internal/thumbnail"
	"github.com/drakkan/sftpgo/v2/internal/thumbnail/cache"
	"github.com/drakkan/sftpgo/v2/internal/util"
	"github.com/drakkan/sftpgo/v2/internal/vfs"
	"github.com/rs/xid"
	"github.com/sftpgo/sdk"
)

const (
	// thumbPath is the URL path for thumbnail requests
	thumbPath = "/web/client/thumb"
)

var (
	// ErrThumbnailNotSupported is returned when the file is not a supported image
	ErrThumbnailNotSupported = errors.New("thumbnail not supported for this file type")
	// ErrThumbnailGenerationFailed is returned when thumbnail generation fails
	ErrThumbnailGenerationFailed = errors.New("thumbnail generation failed")
)

// thumbHandler handles thumbnail generation and caching.
// It generates thumbnails on-demand (synchronously) and caches them.
type thumbHandler struct {
	service *thumbnail.ThumbnailService
}

// handleThumbnail generates or retrieves a cached thumbnail for the requested file.
// Query params:
//   - path: the virtual file path (required)
//   - mtime: file modification time as Unix timestamp (required)
//   - size: file size in bytes (required)
//
// The handler follows SFTPGo's per-request pattern:
// 1. Extract user from JWT
// 2. Create temporary connection
// 3. Verify file access permissions
// 4. Check cache, generate if needed
// 5. Return thumbnail image
func (h *thumbHandler) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	// Extract user from JWT
	claims, err := jwt.FromContext(r.Context())
	if err != nil || claims.Username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := dataprovider.GetUserWithGroupSettings(claims.Username, "")
	if err != nil {
		http.Error(w, "User not found", http.StatusForbidden)
		return
	}

	// Parse query parameters
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	mtimeStr := r.URL.Query().Get("mtime")
	sizeStr := r.URL.Query().Get("size")

	mtime, err := strconv.ParseInt(mtimeStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid mtime parameter", http.StatusBadRequest)
		return
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid size parameter", http.StatusBadRequest)
		return
	}

	// Create temporary connection for file access
	connID := xid.New().String()
	protocol := getProtocolFromRequest(r)
	connectionID := fmt.Sprintf("%v_%v", protocol, connID)
	if err := checkHTTPClientUser(&user, r, connectionID, false, false); err != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	baseConn := common.NewBaseConnection(connID, protocol, util.GetHTTPLocalAddress(r), r.RemoteAddr, user)
	connection := newConnection(baseConn, w, r)
	if err = common.Connections.Add(connection); err != nil {
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}
	defer common.Connections.Remove(connection.GetID())

	// Get the cleaned file path
	cleanPath := connection.User.GetCleanedPath(filePath)

	// Verify the file exists and get its info
	info, err := connection.Stat(cleanPath, 0)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		http.Error(w, "Cannot generate thumbnail for directory", http.StatusBadRequest)
		return
	}

	// Determine provider and bucket for cache key
	// Use the provider number and bucket from user's fs config
	provider := connection.User.FsConfig.Provider
	bucket := getBucketFromFsConfig(connection.User.FsConfig)

	// Generate cache key
	cacheKey := h.service.GenerateCacheKey(
		strconv.Itoa(int(provider)), // use provider number as string
		bucket,
		cleanPath,
		mtime,
		size,
	)

	// Check cache first
	ctx := context.Background()
	thumbData, contentType, err := h.service.GetCachedThumbnail(ctx, cacheKey)
	if err == nil {
		// Cache hit - return cached thumbnail
		h.serveThumbnail(w, r, thumbData, contentType, cacheKey)
		return
	}

	if !errors.Is(err, cache.ErrNotFound) {
		logger.Warn("thumbnail", connectionID, "Error checking cache: %v", err)
	}

	// Cache miss - generate thumbnail
	thumbData, err = h.generateThumbnail(connection, cleanPath)
	if err != nil {
		if errors.Is(err, ErrThumbnailNotSupported) {
			http.Error(w, "Unsupported image format", http.StatusUnsupportedMediaType)
			return
		}
		logger.Warn("thumbnail", connectionID, "Error generating thumbnail: %v", err)
		http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
		return
	}

	// Cache the thumbnail
	if err := h.service.CacheThumbnail(ctx, cacheKey, thumbData); err != nil {
		logger.Warn("thumbnail", connectionID, "Error caching thumbnail: %v", err)
		// Continue anyway - we have the thumbnail to serve
	}

	// Serve the thumbnail
	h.serveThumbnail(w, r, thumbData, "image/jpeg", cacheKey)
}

// generateThumbnail reads the file and generates a thumbnail.
// It uses the connection's getFileReader to properly handle permissions and quotas.
func (h *thumbHandler) generateThumbnail(connection *Connection, filePath string) ([]byte, error) {
	// Get file reader through connection (handles permissions, quotas, etc.)
	reader, err := connection.getFileReader(filePath, 0, http.MethodGet)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	defer reader.Close()

	// Read the file data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read file data: %w", err)
	}

	// Create an image generator for thumbnail creation
	generator := thumbnail.NewImageGenerator(256) // 256px max size

	// Generate thumbnail
	thumbData, err := generator.Generate(context.Background(), bytes.NewReader(data))
	if err != nil {
		if errors.Is(err, thumbnail.ErrUnsupportedFormat) {
			return nil, ErrThumbnailNotSupported
		}
		if errors.Is(err, thumbnail.ErrImageDecode) {
			return nil, ErrThumbnailNotSupported
		}
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	return thumbData, nil
}

// serveThumbnail writes the thumbnail data to the response.
func (h *thumbHandler) serveThumbnail(w http.ResponseWriter, r *http.Request, data []byte, contentType string, cacheKey string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year cache
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, cacheKey))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.Write(data)
}

// getBucketFromFsConfig extracts the bucket name from the filesystem config.
// For S3, GCS, Azure, it returns the bucket name. For local storage, it returns empty string.
func getBucketFromFsConfig(fsConfig vfs.Filesystem) string {
	switch fsConfig.Provider {
	case sdk.S3FilesystemProvider:
		return fsConfig.S3Config.Bucket
	case sdk.GCSFilesystemProvider:
		return fsConfig.GCSConfig.Bucket
	case sdk.AzureBlobFilesystemProvider:
		return fsConfig.AzBlobConfig.Container
	}
	return ""
}
