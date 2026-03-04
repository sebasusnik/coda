# Coda — Code Review

## Summary

Overall the project is well-structured and easy to follow. The README is excellent and the
package layout is clean. The issues below are grouped by severity.

---

## 🔴 Critical

### 1. `AlbumMode()` uses album name instead of album ID
**File:** `internal/client/client.go`

The `Album` struct only stores `Name`, so `AlbumMode()` constructs an invalid Spotify URI
using the album name instead of its ID. This will cause the API call to fail every time.

```go
// Bug: album name used instead of ID
"context_uri": "spotify:album:" + current.Item.Album.Name,
```

**Fix:** Add an `ID` field to the `Album` struct and use it when building the URI.

```go
type Album struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// Then in AlbumMode():
"context_uri": "spotify:album:" + current.Item.Album.ID,
```

---

### 2. `daemon serve` subcommand is referenced but never implemented
**File:** `internal/daemon/daemon.go`, `main.go`

Both the launchd plist and the systemd unit file reference `coda daemon serve` as the
process entry point, but this subcommand is not registered in `main.go` and has no
implementation. The daemon will fail to start after installation.

```xml
<!-- launchd plist -->
<string>daemon</string>
<string>serve</string>
```

**Fix:** Implement and register a `daemon serve` subcommand that runs whatever background
work the daemon is supposed to perform (e.g. a keep-alive token refresh loop).

---

### 3. `internal/config` package is empty
**File:** `internal/config/`

The `config.Config`, `config.Load()`, and `config.Save()` types and functions are
referenced in both `auth.go` and `client.go`, but the package directory contains no files.
This will cause a compilation error. It appears the file was accidentally omitted from the
repository.

**Fix:** Add `config.go` with at minimum:

```go
package config

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

type Config struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    ExpiresAt    time.Time `json:"expires_at"`
    ClientID     string    `json:"client_id"`
}

func configPath() (string, error) {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(homeDir, ".config", "coda", "config.json"), nil
}

func Load() (*Config, error) {
    path, err := configPath()
    if err != nil {
        return nil, err
    }
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    return &cfg, json.Unmarshal(data, &cfg)
}

func (c *Config) Save() error {
    path, err := configPath()
    if err != nil {
        return err
    }
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(c, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0600)
}
```

---

## 🟡 Warnings

### 4. `json.Marshal` errors silently ignored in multiple places
**File:** `internal/client/client.go`

Three functions — `playTrack()`, `RadioMode()`, and `AlbumMode()` — all discard the error
from `json.Marshal`:

```go
jsonBody, _ := json.Marshal(body)
```

While marshalling a `map[string]interface{}` is unlikely to fail in practice, ignoring
errors is a bad habit and can mask bugs.

**Fix:** Handle the error explicitly in each case:

```go
jsonBody, err := json.Marshal(body)
if err != nil {
    return fmt.Errorf("failed to marshal request body: %v", err)
}
```

---

### 5. Global `http.DefaultServeMux` used in the OAuth callback server
**File:** `internal/auth/auth.go`

The callback route is registered on the global default mux, which would panic on a
duplicate registration if `Authenticate` were called more than once in the same process
(e.g. in tests).

```go
server := &http.Server{Addr: ":8080"}
http.HandleFunc("/callback", func(...) { ... })
```

**Fix:** Use a local `ServeMux`:

```go
mux := http.NewServeMux()
mux.HandleFunc("/callback", func(...) { ... })
server := &http.Server{Addr: ":8080", Handler: mux}
```

---

### 6. `os.Setenv` for `CODA_CLIENT_ID` does not persist across processes
**File:** `internal/auth/auth.go`

When the user is prompted for their Client ID, it is saved with `os.Setenv`, which only
affects the current process. On the next invocation the value is gone.

```go
os.Setenv("CODA_CLIENT_ID", clientID)
```

The Client ID is already saved to `config.json` via `cfg.Save()`, so the env-var approach
is redundant and confusing. The `init()` function should also attempt to load the Client ID
from the saved config as a fallback.

**Fix:** In `init()`, fall back to the saved config when the env var is absent:

```go
func init() {
    clientID := os.Getenv("CODA_CLIENT_ID")
    if clientID == "" {
        if cfg, err := config.Load(); err == nil {
            clientID = cfg.ClientID
        }
    }
    oauthConfig = &oauth2.Config{
        ClientID: clientID,
        // ...
    }
}
```

---

### 7. Unnecessary in-memory global `lastSearchResults`
**File:** `internal/client/client.go`

Search results are stored in a package-level global and also persisted to disk. Since each
CLI invocation is a separate process, the in-memory cache is never actually shared between
`search` and `play` — `PlayByNumber` always ends up loading from disk. The global adds
confusion without providing any benefit.

```go
var lastSearchResults []Track
```

**Fix:** Remove the global. In `PlayByNumber`, load directly from disk. In `SearchTracks`,
save to disk and print without storing in memory.

---

### 8. `rand.Read` return value discarded
**File:** `internal/auth/auth.go`

```go
b := make([]byte, 16)
rand.Read(b)
```

`crypto/rand.Read` never returns an error in practice on supported platforms, but the
return value should still be checked for correctness and to satisfy linters.

**Fix:**

```go
if _, err := rand.Read(b); err != nil {
    return fmt.Errorf("failed to generate state: %v", err)
}
```

---

## 🟢 Minor / Style

### 9. Makefile ldflags inject variables not declared in `main.go`
**File:** `Makefile`, `main.go`

The Makefile injects `version`, `commit`, and `buildTime` via `-ldflags`:

```makefile
LDFLAGS := -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}"
```

However, none of these variables are declared in `main.go`, so the flags have no effect.

**Fix:** Declare the variables in `main.go` and expose them via a `version` command or
`--version` flag:

```go
var (
    version   = "dev"
    commit    = "unknown"
    buildTime = "unknown"
)
```

Then register a `--version` flag on the `cli.App`:

```go
app := &cli.App{
    Name:    "coda",
    Version: fmt.Sprintf("%s (%s) built %s", version, commit, buildTime),
    // ...
}
```

---

## Summary Table

| # | Severity | File | Issue |
|---|----------|------|-------|
| 1 | 🔴 Critical | `client.go` | `AlbumMode` uses album name instead of album ID |
| 2 | 🔴 Critical | `daemon.go` / `main.go` | `daemon serve` referenced but not implemented |
| 3 | 🔴 Critical | `internal/config/` | Config package is empty / missing from repo |
| 4 | 🟡 Warning | `client.go` | `json.Marshal` errors silently ignored in 3 places |
| 5 | 🟡 Warning | `auth.go` | Global `DefaultServeMux` used in callback handler |
| 6 | 🟡 Warning | `auth.go` | `os.Setenv` doesn't persist; Client ID loading is inconsistent |
| 7 | 🟡 Warning | `client.go` | Unnecessary in-memory global `lastSearchResults` |
| 8 | 🟡 Warning | `auth.go` | `rand.Read` error discarded |
| 9 | 🟢 Minor | `Makefile` / `main.go` | ldflags variables not declared in `main.go` |