# SFTPGo Architecture

Full-featured and highly configurable event-driven file transfer server supporting SFTP, HTTP/S, FTP/S, and WebDAV protocols with multiple storage backends.

## Tech Stack

- **Language**: Go 1.25
- **CLI Framework**: spf13/cobra + spf13/viper
- **HTTP Router**: go-chi/chi/v5
- **Database**: SQLite, PostgreSQL, MySQL, CockroachDB, Bolt, Memory (via database/sql + drivers)
- **Logging**: rs/zerolog + gopkg.in/natefinch/lumberjack.v2
- **JWT**: go-jose/go-jose/v4
- **Testing**: Go stdlib `testing` + stretchr/testify
- **Linting**: golangci-lint (revive, bodyclose, dogsled, dupl, goconst, gocyclo, misspell, rowserrcheck, unconvert, unparam, whitespace)

## Directory Structure

```
├── main.go                    # Entry point: calls cmd.Execute()
├── go.mod / go.sum            # Go module definition
├── sftpgo.json                # Default configuration file
├── internal/                  # Core application code (27 packages)
│   ├── cmd/                   # CLI commands (Cobra-based)
│   ├── config/                # Configuration loading (Viper)
│   ├── dataprovider/          # Database abstraction layer
│   ├── httpd/                 # HTTP server (REST API + WebUI)
│   ├── sftpd/                 # SFTP protocol server
│   ├── ftpd/                  # FTP/S protocol server
│   ├── webdavd/               # WebDAV protocol server
│   ├── service/               # Service orchestration
│   ├── vfs/                   # Virtual filesystem layer
│   ├── kms/                   # Key management service
│   ├── jwt/                   # JWT authentication
│   ├── mfa/                   # Multi-factor authentication
│   ├── smtp/                  # Email/SMTP support
│   ├── logger/                # Logging infrastructure
│   ├── common/                # Shared types and utilities
│   ├── util/                  # Utility functions
│   ├── telemetry/             # OpenTelemetry integration
│   ├── plugin/                # Plugin system
│   └── ...
├── static/                    # Frontend assets (CSS, JS, images, vendor bundles)
├── templates/                 # HTML templates (WebAdmin, WebClient, email)
├── openapi/                    # OpenAPI/Swagger specifications
├── examples/                  # Example programs (OTP, LDAP, backup)
├── tests/                     # Test utilities and integration tests
├── pkgs/                      # Packaging scripts
├── docker/                    # Docker-related files
└── init/                      # Init scripts and service definitions
```

## Core Components

### Entry Point (`main.go`)
```
main() → cmd.Execute() → rootCmd.Execute()
```

### CLI (`internal/cmd/`)
- **Root command** (`root.go`): `sftpgo` - defines shared flags via Viper
- **Subcommands**:
  - `serve` - Start the SFTPGo service
  - `gen` - Generators (shell completions, man pages)
  - `acme` - ACME/Let's Encrypt certificate management
  - `initprovider` - Initialize/update data provider
  - `resetprovider` - Reset data provider
  - `resetpwd` - Reset admin password
  - `ping` - Health check
  - `smtptest` - Test SMTP configuration
  - `portable` - Serve single directory (build-tagged)
  - `service` - Windows service management

### HTTP Server (`internal/httpd/`)
- Router: `go-chi/chi/v5`
- Authentication: JWT (API key + OIDC support)
- Three route groups:
  1. **REST API** (`/api/v2/...`) - programmatic access
  2. **WebAdmin** (`/web/admin/...`) - admin UI
  3. **WebClient** (`/web/client/...`) - client file browser
- Middleware chain: RequestID → parseHeaders → Logger → Recoverer → SecurityHeaders → CORS → ConnectionCheck
- Token managers: in-memory (single instance) or database (HA mode)

### Protocol Servers
- **SFTP** (`internal/sftpd/`): SSH-based file transfer
- **FTP** (`internal/ftpd/`): FTP/S with TLS
- **WebDAV** (`internal/webdavd/`): HTTP-based file access
- **HTTP** (`internal/httpd/`): REST API + WebUIs

### Data Layer (`internal/dataprovider/`)
- **Provider interface**: abstract database operations
- **SQL implementations**: SQLite, PostgreSQL, MySQL, CockroachDB (share `sqlcommon.go`)
- **Non-SQL**: Memory (maps), Bolt (bbolt key-value store)
- **Key operations**: User/Admin/Group CRUD, Quota management, API keys, Shares, Event rules, IP lists

### Virtual Filesystem (`internal/vfs/`)
- Supports: local filesystem, encrypted filesystem, S3, GCS, Azure Blob, SFTP backend
- Unified interface for file operations across backends

## Data Flow

### Startup Flow
```
main() → cmd.Execute() → rootCmd.Execute()
  → serveCmd.Run() → service.Start()
    → config.LoadConfig()
    → logger.InitLogger()
    → kms.Initialize()
    → dataprovider.Initialize()
    → httpd.Conf.Initialize()
    → sftpd/ftpd/webdavd servers start
    → service.Wait() (signal handling)
```

### HTTP Request Flow
```
Request → Middleware Chain → Router
  → /api/v2/* (REST API)
    → JWT/API Key Auth → Handler → dataprovider → JSON Response
  → /web/admin/* (WebAdmin)
    → Session Auth → Handler → Template → HTML Response
  → /web/client/* (WebClient)
    → Session Auth → Handler → Template → HTML Response
```

### File Transfer Flow
```
Client → Protocol Server (SFTP/FTP/WebDAV)
  → Authenticate (User DB or external)
  → Authorize (Permissions check)
  → VFS Operation (via storage backend)
  → Transfer Log → Response
```

## Configuration

- **Config file**: `sftpgo.json` (or YAML/TOML/HCL/properties)
- **Environment variables**: All config options via `SFTPGO_*` prefix
- **Flags**: `--config-dir`, `--config-file`, `--log-file-path`, etc.
- **Loading**: Viper with env var binding

### Key Config Sections
- `httpd` - HTTP server binding, TLS, CORS, OIDC
- `httpd.binding` - per-listener settings (ports, login methods)
- `dataprovider` - database driver, connection string
- `sftpd`, `ftpd`, `webdavd` - protocol-specific settings
- `kms` - encryption key management
- `common` - password policy, rate limiting, defender

## External Integrations

| Service | Integration Package |
|---------|---------------------|
| AWS S3 | `github.com/aws/aws-sdk-go-v2/service/s3` |
| Google Cloud Storage | `cloud.google.com/go/storage` |
| Azure Blob | `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob` |
| SFTP Backend | `github.com/pkg/sftp` |
| OIDC/OAuth2 | `github.com/coreos/go-oidc/v3` |
| SMTP | `github.com/wneessen/go-mail` |
| ACME/Let's Encrypt | `github.com/go-acme/lego/v4` |
| OpenTelemetry | `go.opentelemetry.io/otel/*` |

## Build & Deploy

### Build
```bash
go build -o sftpgo .
```

### Run
```bash
./sftpgo serve
```

### Docker
```bash
docker build -t sftpgo .
docker run -v /path/to/config:/etc/sftpgo sftpgo
```

### Test
```bash
go test ./...
golangci-lint run
```
