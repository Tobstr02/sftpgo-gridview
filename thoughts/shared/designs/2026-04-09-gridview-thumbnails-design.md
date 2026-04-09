---
date: 2026-04-09
topic: "Grid View with Thumbnails"
status: draft
---

## Problem Statement

The WebClient currently displays files in a **list-only view** using DataTables. Image files show only a generic icon with a preview button - users cannot see image thumbnails directly in the file listing. This is a standard feature in modern file browsers (filebrowser, Nextcloud) and is expected by users.

**Key requirements:**
1. Grid view default for WebClient showing actual image thumbnails
2. Thumbnails must be **cached** (S3-compatible storage required)
3. Mobile-responsive layout
4. Works with SFTP, S3, FTP, and local filesystem backends
5. Performance-conscious (lazy loading, virtual scrolling consideration)

---

## Constraints

- **SFTPGo is Go-based**: Thumbnails must be generated server-side in Go
- **S3-compatible**: Must cache thumbnails in S3 (or similar object storage) for scalability
- **Minimal dependencies**: Leverage existing libraries, avoid heavy deps like FFmpeg for v1
- **CSP compliance**: JavaScript must be CSP-safe (no inline eval) - already a project goal
- **Mobile support**: Must work on tablet/mobile browsers
- **No existing infrastructure**: No thumbnail service, no file content cache

---

## Approach

### Architecture Decision: Server-Side Thumbnail Generation with Distributed Cache

**Why not client-side thumbnails?**
- Client-side canvas-based thumbnailing is complex and CSP-restricted
- S3-backed storage means thumbnails must be server-generated anyway
- Enables consistent experience across devices

**Why not just-in-time generation?**
- S3 cost optimization: regenerating on every request is expensive
- Performance: cached thumbnails load instantly
- Consistency: same thumbnail everywhere

**Chosen approach:**
1. **Thumbnail generation service** in Go using a minimal imaging library (`github.com/disintegration/imaging` - pure Go, no C dependencies)
2. **Multi-backend cache**: S3 primary → local disk fallback
3. **Grid view component** in the WebClient replacing DataTables
4. **Virtual scrolling** for large directories

**Rejected alternatives:**
- **libvips binding**: Too heavy, C dependency
- **FFmpeg for video thumbnails**: Out of scope for v1, adds complexity
- **Client-side canvas**: CSP issues, inconsistent across browsers

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     WebClient Browser                        │
│  ┌─────────────────┐  ┌──────────────────────────────────┐ │
│  │  View Toggle     │  │     Grid View (CSS Grid)         │ │
│  │  List │ Grid     │  │  ┌────┐ ┌────┐ ┌────┐ ┌────┐   │ │
│  └─────────────────┘  │  │thumb│ │thumb│ │thumb│ │thumb│ │ │
│                        │  └────┘ └────┘ └────┘ └────┘   │ │
│                        │  filename   filename  ...        │ │
│                        └──────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    SFTPGo HTTP Server                        │
│  ┌─────────────────┐  ┌──────────────────────────────────┐ │
│  │  /thumb/*       │  │   ThumbnailController            │ │
│  │  endpoint       │  │   - Check cache first            │ │
│  └─────────────────┘  │   - Generate if miss            │ │
│                        │   - Store and serve             │ │
│                        └──────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────┐    ┌─────────────────────────────┐
│   S3/Object Storage     │    │   Local Disk Cache          │
│   (thumbnail-bucket)    │    │   ($SFTPGO_DATA/thumbs/)    │
└─────────────────────────┘    └─────────────────────────────┘
```

### Backend Components

| Component | Responsibility |
|-----------|----------------|
| `ThumbnailService` | Generates thumbnails, manages cache lifecycle |
| `ThumbnailCache` | Multi-backend interface (S3 + local disk) |
| `GridViewController` | Serves thumbnail requests, validates params |
| `ThumbRequest` / `ThumbResponse` | Request/response models |

### Frontend Components

| Component | Responsibility |
|-----------|----------------|
| `GridView` | CSS Grid layout, handles view toggle |
| `ThumbnailCell` | Individual thumbnail card with lazy loading |
| `ThumbnailLoader` | Intersection Observer for lazy loading |
| `ViewToggle` | Switch between list/grid views |

---

## Data Flow

### Thumbnail Request Flow

```
1. Browser requests directory listing (existing API)
2. Server returns file metadata (name, size, mime_type, path)
3. Frontend detects image files, renders GridView with placeholder
4. For each image thumbnail:
   a. Check if cached: GET /thumb/{hash}
   b. If cache HIT: Load from CDN/cache URL directly
   c. If cache MISS:
      - Request generation: POST /thumb/generate
      - Server generates thumbnail (256x256 fit)
      - Server stores in S3 + local cache
      - Return thumbnail URL
5. Thumbnails load via lazy loading (Intersection Observer)
```

### Cache Key Strategy

```
Thumbnail cache key: SHA256(provider + bucket + path + mtime + size)

Example:
- S3 provider, bucket "files", path "/photos/vacation.jpg"
- Key: sha256("s3:files/photos/vacation.jpg:1709234567:2048000")
```

This ensures:
- Same file = same thumbnail (deterministic)
- File change = new thumbnail (mtime-based)
- Cross-provider uniqueness

---

## API Endpoints

### GET /thumb/{cacheKey}

**Purpose:** Retrieve cached thumbnail

**Response:**
- `200 OK`: JPEG image with `Content-Type: image/jpeg`
- `404 Not Found`: Thumbnail not in cache (client should request generation)
- `400 Bad Request`: Invalid cache key format

**Headers:**
```
Cache-Control: public, max-age=31536000  (1 year)
ETag: "{hash}"
```

### POST /thumb/generate

**Purpose:** Generate thumbnail for a file

**Request:**
```json
{
  "provider": "s3",
  "bucket": "files",
  "path": "/photos/vacation.jpg",
  "size": 256
}
```

**Response:**
```json
{
  "cache_key": "sha256:abc123...",
  "url": "/thumb/sha256:abc123...",
  "expires_at": "2027-04-09T00:00:00Z"
}
```

### DELETE /thumb/{cacheKey}

**Purpose:** Invalidate thumbnail cache (for file deletion)

---

## Thumbnail Generation

### Supported Formats (v1)

| Category | Formats | Library |
|----------|---------|---------|
| Images | JPEG, PNG, GIF, WebP, BMP | `disintegration/imaging` |
| PDFs | PDF (first page only) | `github.com/unidoc/unipdf` (optional, v2) |

### Image Processing Pipeline

```
1. Read source file (stream for memory efficiency)
2. Detect format from magic bytes (not extension)
3. Decode image
4. Compute fit dimensions (256x256 max, preserve aspect)
5. Resize using Lanczos (quality + performance balance)
6. Encode as JPEG quality=85
7. Return bytes
```

### Sizing Strategy

| Size | Dimensions | Use Case |
|------|-------------|----------|
| Small | 256×256 | Grid view default |
| Large | 640×640 | Gallery/detail view (future) |

---

## Cache Strategy

### Multi-Backend Cache Interface

```go
type ThumbnailCache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, data []byte) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

### Backend Implementations

| Backend | Priority | Config |
|---------|----------|--------|
| S3 | Primary | `THUMBNAIL_S3_BUCKET`, `THUMBNAIL_S3_PREFIX`, AWS credentials |
| Local Disk | Fallback | `THUMBNAIL_CACHE_DIR` (default: `$SFTPGO_DATA/thumbnails/`) |

### Cache Invalidation

- **TTL-based**: Configurable max-age (default: 30 days)
- **Event-based**: Invalidate on file rename/delete
- **Manual**: Admin API to flush cache

---

## Frontend Grid View

### CSS Grid Layout

```css
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
}
```

### View Toggle

- Stored in `localStorage` as `filesViewMode` (list | grid)
- Default: **grid** (user requirement)
- Toggle button in toolbar next to sort options

### Mobile Responsiveness

| Breakpoint | Columns |
|------------|---------|
| < 576px | 2 columns |
| 576-768px | 3 columns |
| 768-1024px | 4 columns |
| > 1024px | auto-fill |

### Lazy Loading

```javascript
const observer = new IntersectionObserver(
  (entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        loadThumbnail(entry.target.dataset.key);
        observer.unobserve(entry.target);
      }
    });
  },
  { rootMargin: '200px' }
);
```

### Placeholder States

| State | Visual |
|-------|--------|
| Loading | Skeleton shimmer animation |
| Error | Broken image icon + filename |
| Unsupported | File type icon (existing) |

---

## File Structure

### New Backend Files

```
internal/
├── thumbnail/
│   ├── service.go        # ThumbnailService
│   ├── cache/
│   │   ├── cache.go      # Interface + multi-backend
│   │   ├── s3.go         # S3 cache backend
│   │   └── local.go      # Local disk cache
│   ├── generator.go      # Image processing
│   └── handlers.go       # HTTP handlers
```

### New Frontend Files

```
static/
├── webclient/
│   ├── js/
│   │   ├── grid-view.js      # Grid view component
│   │   └── thumbnail-loader.js
│   └── css/
│       ├── grid-view.css
│       └── thumbnail-cell.css
```

### Modified Files

| File | Changes |
|------|---------|
| `templates/webclient/files.html` | Add grid view toggle, replace DataTables with hybrid view |
| `internal/httpd/webclient.go` | Add thumb routes, serve thumb endpoints |
| `internal/config/config.go` | Add thumbnail config section |
| `ARCHITECTURE.md` | Document thumbnail service |
| `CODE_STYLE.md` | Add thumbnail coding conventions |

---

## Configuration

### New Config Keys

```yaml
# thumbnail.yaml
thumbnail:
  enabled: true
  size: 256
  cache:
    backend: "s3"  # "s3" | "local" | "multi"
    s3:
      bucket: "sftpgo-thumbnails"
      prefix: "thumbs/"
      region: "us-east-1"
    local:
      path: "./data/thumbnails"
    ttl: 720h  # 30 days
  formats:
    images: ["jpeg", "jpg", "png", "gif", "webp", "bmp"]
    video: []  # v2
```

---

## Error Handling

### Generation Failures

| Error | HTTP Code | Client Action |
|-------|-----------|---------------|
| Unsupported format | 415 | Show generic icon |
| Source file not found | 404 | Remove from grid |
| Source file inaccessible | 403 | Show lock icon |
| Generation failed | 500 | Retry with backoff (max 3) |
| Storage write failed | 500 | Log, serve without cache |

### Cache Miss Behavior

- Never fail silently - always attempt generation
- On generation failure, fall back to existing preview button
- Monitor failure rate for alerting

---

## Testing Strategy

### Unit Tests

| Component | Test Coverage |
|-----------|---------------|
| `generator.go` | All supported format conversions |
| `cache/s3.go` | Mock S3, verify Put/Get/Delete |
| `cache/local.go` | File creation, retrieval, TTL expiry |
| `service.go` | Cache hit/miss logic |

### Integration Tests

| Scenario | Steps |
|----------|-------|
| Thumbnail round-trip | Upload image → request thumb → verify matches |
| S3 cache | Configure S3 → generate thumb → verify in S3 |
| Invalid format | Request thumb for .txt → verify 415 |
| Large image | 50MB JPEG → verify resize + memory safety |

### Frontend Tests

| Component | Test |
|-----------|------|
| Grid view | Render 100 items, verify scroll performance |
| Lazy loading | Scroll, verify only visible thumbs load |
| View toggle | Switch views, verify state persists |
| Mobile | Test on mobile viewport |

---

## Open Questions

1. **Video thumbnails (v2)**: FileBrowser uses FFmpeg. Acceptable for v1 to skip, or should we target v1?
2. **PDF thumbnails**: Same question - muPDF dependency vs v2 scope
3. **Office docs (DOCX, XLSX)**: Out of scope, but nice-to-have. Same decision needed.
4. **Thumbnail尺寸**: 256px seems right for grid. Should we offer 128px for compact view?
5. **S3 bucket ownership**: Does user provide their own bucket, or create one automatically?
6. **Cache cleanup**: Background job to delete expired thumbnails? Or rely on TTL + eventual consistency?
7. **Animated GIFs**: Should thumbnails preserve animation? Performance concern.

---

## Effort Estimate

| Phase | Effort | Description |
|-------|--------|-------------|
| Backend thumbnail service | Medium | Go service, image processing, cache interface |
| S3 cache backend | Medium | AWS SDK integration |
| Local cache backend | Low | Disk I/O, simple |
| HTTP handlers | Low | Routes + request validation |
| Frontend grid view | High | CSS Grid, view toggle, lazy loading, mobile |
| Integration + testing | Medium | E2E tests, S3 integration tests |
| **Total** | **High** | ~3-4 weeks |

---

## Dependencies

### New Go Dependencies

| Library | Purpose | License |
|---------|---------|---------|
| `github.com/disintegration/imaging` | Pure Go image processing | MIT |
| `github.com/aws/aws-sdk-go-v2` | S3 SDK | Apache 2.0 |

### Already Available

- `github.com/go-chi/chi` - HTTP routing
- `github.com/spf13/viper` - Configuration
- `zerolog` - Logging

---

## Success Criteria

1. **Grid view default**: WebClient opens to grid view showing thumbnails
2. **Thumbnails visible**: All supported image formats show actual thumbnails, not icons
3. **Caching works**: Thumbnails stored in configured cache backend
4. **Performance**: Grid scrolling smooth with 100+ images (lazy loading)
5. **Mobile responsive**: Usable on mobile browsers (Chrome/Safari on iOS/Android)
6. **No regressions**: Existing list view, file operations still work
7. **S3 support**: Thumbnails stored in S3 when configured
