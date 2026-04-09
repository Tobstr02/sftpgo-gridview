# SFTPGo Code Style

## Naming Conventions

### Files
- **Go source**: `lowercase_underscore.go` (e.g., `dataprovider.go`, `server.go`)
- **Test files**: `*_test.go` (e.g., `user_test.go`)
- **Platform-specific**: `name_windows.go`, `name_unix.go`
- **Build-tagged**: `name_portable.go`, `name_disabled.go`

### Packages
- Short, lowercase: `cmd`, `httpd`, `vfs`, `kms`
- No underscores, no mixed case

### Functions/Methods
- **Exportable**: `PascalCase` (e.g., `GetUserByUsername`, `AddFolder`)
- **Unexport**: `camelCase` (e.g., `getUsers`, `addFolder`)
- **Test helpers**: often prefixed with test utility name (e.g., `TestGetUserFromDB`)

### Variables/Constants
- **Exportable**: `PascalCase` (e.g., `DefaultLogLevel`, `MaxConnections`)
- **Unexport**: `camelCase` or `PascalCase` with prefix indicating type
- **Constants**: `PascalCase` or `ALL_CAPS` for true constants
- **Private globals**: `camelCase` with type prefix (e.g., `userCache map[string]User`)

### Interfaces
- `PascalCase` ending with `er` or `Interface`: `Provider`, `Saver`, `FileSystem`

### Error Types
- Sentinel errors: `Err` prefix (e.g., `ErrValidation`, `ErrNotFound`)
- Custom error types: `PascalCase` ending with `Error` (e.g., `ValidationError`)

## File Organization

### Go Source Files
```
1. Package declaration
2. Copyright header (AGPL-3 notice)
3. imports (blank line between stdlib and third-party)
4. Constants
5. Variables
6. Types (structs, interfaces)
7. Functions/Methods
```

### Import Organization
```go
import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    "github.com/drakkan/sftpgo/v2/internal/logger"
    "github.com/drakkan/sftpgo/v2/internal/version"
)
```

### Common File Patterns
- `xxx.go` - main implementation
- `xxx_test.go` - tests
- `internal_xxx.go` - internal helpers (not exported)
- `xxx_xxx.go` - platform-specific code

## Code Patterns

### Error Handling
```go
// Sentinel errors
var ErrValidation = errors.New("validation error")

// Custom error types with Is() method
type ValidationError struct {
    err string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("Validation error: %s", e.err)
}

func (e *ValidationError) Is(target error) bool {
    _, ok := target.(*ValidationError)
    return ok
}

// Wrapping with context
return nil, fmt.Errorf("unable to add user: %w", err)

// Error checking
if errors.Is(err, util.ErrNotFound) {
    // handle not found
}
```

### Logging
```go
// Component logger with connection ID tracking
logger.Info(logSender, connectionID, "user %q logged in", username)
logger.Error(logSender, connectionID, "transfer failed: %v", err)

// Transfer logging
logger.TransferLog(operation, path, elapsed, size, user, connectionID, protocol, localAddr, remoteAddr, ftpMode, err)

// Login logging
logger.LoginLog(user, ip, loginMethod, protocol, connectionID, clientVersion, encrypted, info)

// Console output (for CLI commands)
logger.ErrorToConsole("configuration error: %v", err)
```

### Configuration (Viper + Cobra)
```go
// Define constants for flag names
const (
    configDirFlag = "config-dir"
    configDirKey  = "config_dir"
)

// Bind flag to viper
viper.SetDefault(configDirKey, defaultConfigDir)
viper.BindEnv(configDirKey, "SFTPGO_CONFIG_DIR")
cmd.Flags().StringVarP(&configDir, configDirFlag, "c", viper.GetString(configDirKey), "...")
viper.BindPFlag(configDirKey, cmd.Flags().Lookup(configDirFlag))
```

### HTTP Handlers (Chi)
```go
// Route registration
router.MethodFunc("GET", "/api/v2/users", s.getUsers)

// Middleware chain
router.Group(func(r chi.Router) {
    r.Use(jwtAuthenticatorAPI)
    r.Use(s.checkPerms("admin"))
    r.Get("/users", s.getUsers)
})

// Response
func sendAPIResponse(w http.ResponseWriter, r *http.Request, err error, message string, code int) {
    resp := apiResponse{Error: errorString, Message: message}
    ctx := context.WithValue(r.Context(), render.StatusCtxKey, code)
    render.JSON(w, r.WithContext(ctx), resp)
}
```

### Database Patterns
```go
// SQL placeholder strategy
func getSQLPlaceholders() []string {
    for i := 1; i <= 100; i++ {
        if config.Driver == PGSQLDataProviderName {
            placeholders = append(placeholders, fmt.Sprintf("$%d", i))
        } else {
            placeholders = append(placeholders, "?")
        }
    }
}

// Query function
func getUsersQuery(order, role string) string {
    return fmt.Sprintf(`SELECT %s FROM %s u ... ORDER BY u.username %s LIMIT %s OFFSET %s`,
        selectUserFields, sqlTableUsers, order, sqlPlaceholders[0], sqlPlaceholders[1])
}
```

### Test Patterns
```go
// Table-driven tests
func TestUserValidation(t *testing.T) {
    tests := []struct {
        name    string
        user    User
        wantErr bool
    }{
        {"valid user", User{Username: "test"}, false},
        {"empty username", User{}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateUser(tt.user)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateUser() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

// Error assertions
assert.ErrorIs(t, err, util.ErrNotFound)
assert.ErrorAs(t, err, &validationErr)
```

## Do's and Don'ts

### Do
- Use `errors.Is()` and `errors.As()` for error checking
- Use `logger.Info/Warn/Error` with sender and connection ID
- Use Viper for configuration with env var binding
- Use table-driven tests with `t.Run()`
- Use `//nolint:errcheck` only when explicitly ignoring is safe
- Use `_ =` prefix for intentionally ignored returns
- Use custom error types with `Is()` method for sentinel errors
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Use `zerolog.Level` for log levels

### Don't
- Don't use `panic()` for normal error handling
- Don't use `log.Println()` - use the `logger` package
- Don't ignore errors silently: `_ = something()` only when intentional
- Don't use `//nolint:errcheck` without a comment explaining why
- Don't create deeply nested error types
- Don't use `time.Sleep()` in tests (use mocks instead)
- Don't use global variables for mutable state
- Don't skip the copyright header on new files

## Linter Configuration

The project uses `.golangci.yml` with these enabled linters:
- `bodyclose` - checks response body closing
- `dogsled` - checks for excessive blank identifiers
- `dupl` - detects code duplication (threshold: 150)
- `goconst` - finds repeated constants (min 3 chars, 3 occurrences)
- `gocyclo` - checks cyclomatic complexity (min complexity: 15)
- `misspell` - catches spelling mistakes
- `revive` - configurable rule set (var-naming enabled as warning)
- `rowserrcheck` - checks for unchecked errors in database operations
- `unconvert` - detects unnecessary type conversions
- `unparam` - finds unused function parameters
- `whitespace` - detects leading/trailing whitespace

Formatters enabled: `gofmt`, `goimports`

Excluded paths: `third_party`, `builtin`, `examples`
