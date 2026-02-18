# Proxy Delegation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove CORS, rate limiting, JWT secret, and trusted proxies from the application and replace them with nginx and Traefik example configurations, plus startup warnings for stale config keys.

**Architecture:** All four features are deleted from Go source; the `internal/ratelimit/` package is removed entirely. A `warnDeprecatedKeys` function in `internal/config/config.go` performs a second YAML decode on startup to detect stale operator configs and emit `slog.Warn` messages. Reverse-proxy sample configs in `examples/nginx/` and `examples/traefik/` provide operators with drop-in replacements.

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v3`, `log/slog`, nginx, Traefik v3

**Design doc:** `docs/plans/2026-02-18-proxy-delegation-design.md`

**Branch:** Create a new branch from `main` before starting — do not implement on `docs/reconcile-and-autogen`.

```bash
git checkout main
git pull
git checkout -b feat/proxy-delegation
```

---

### Task 1: Delete `internal/ratelimit/` and remove its wiring from `cmd/updater/updater.go`

**Files:**
- Delete: `internal/ratelimit/` (entire directory — limiter.go, memory.go, memory_test.go, middleware.go, middleware_test.go)
- Modify: `cmd/updater/updater.go`

**Step 1: Delete the ratelimit package**

```bash
git rm -r internal/ratelimit/
```

Expected: 5 files staged for deletion.

**Step 2: Remove the import and wiring from `cmd/updater/updater.go`**

Remove the import line:
```go
"updater/internal/ratelimit"
```

Remove the entire block at lines 107–127 (the rate limiter initialization):
```go
// Initialize rate limiter if enabled
if cfg.Security.RateLimit.Enabled {
    rlCfg := cfg.Security.RateLimit
    ...
    routeOpts = append(routeOpts, api.WithRateLimiter(ratelimit.Middleware(anonLimiter, authLimiter)))
}
```

The `routeOpts` slice declaration and the OTel append above it are unaffected — leave them.

**Step 3: Run tests**

```bash
make test
```

Expected: compilation fails because `api.WithRateLimiter` still exists and references `net/http` but takes no ratelimit argument. That is fine — `WithRateLimiter` will be removed in Task 4. For now, the compilation error is from `updater.go` referencing the deleted package, which should now be clean. If `WithRateLimiter` is referenced nowhere else, the build should succeed. Verify with:

```bash
make build
```

Expected: BUILD SUCCESS (the ratelimit package is gone and no import references it).

**Step 4: Commit**

```bash
git add cmd/updater/updater.go
git commit -m "feat: remove internal/ratelimit package and wiring"
```

---

### Task 2: Remove `CORSConfig` from models and environment loader

**Files:**
- Modify: `internal/models/config.go`
- Modify: `internal/models/config_test.go`
- Modify: `internal/config/config.go`

**Step 1: Remove `CORSConfig` type and `CORS` field from `internal/models/config.go`**

Delete the entire `CORSConfig` struct (lines 64–70):
```go
type CORSConfig struct {
    Enabled        bool     `yaml:"enabled" json:"enabled"`
    AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
    AllowedMethods []string `yaml:"allowed_methods" json:"allowed_methods"`
    AllowedHeaders []string `yaml:"allowed_headers" json:"allowed_headers"`
    MaxAge         int      `yaml:"max_age" json:"max_age"`
}
```

Remove the `CORS` field from `ServerConfig`:
```go
CORS         CORSConfig    `yaml:"cors" json:"cors"`
```

Remove the `CORS` default block from `NewDefaultConfig()`:
```go
CORS: CORSConfig{
    Enabled:        true,
    AllowedOrigins: []string{"*"},
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"*"},
    MaxAge:         86400,
},
```

Also update the comment in `NewDefaultConfig()` — remove the line "- Rate limiting enabled: Prevent abuse from the start".

**Step 2: Remove CORS env var handling from `internal/config/config.go`**

Delete this entire block (lines 102–123):
```go
// CORS configuration
if cors := os.Getenv("UPDATER_CORS_ENABLED"); cors != "" {
    config.Server.CORS.Enabled = strings.ToLower(cors) == "true"
}
if origins := os.Getenv("UPDATER_CORS_ALLOWED_ORIGINS"); origins != "" {
    config.Server.CORS.AllowedOrigins = strings.Split(origins, ",")
}
if methods := os.Getenv("UPDATER_CORS_ALLOWED_METHODS"); methods != "" {
    config.Server.CORS.AllowedMethods = strings.Split(methods, ",")
}
if headers := os.Getenv("UPDATER_CORS_ALLOWED_HEADERS"); headers != "" {
    config.Server.CORS.AllowedHeaders = strings.Split(headers, ",")
}
if maxAge := os.Getenv("UPDATER_CORS_MAX_AGE"); maxAge != "" {
    if age, err := strconv.Atoi(maxAge); err == nil {
        config.Server.CORS.MaxAge = age
    }
}
```

**Step 3: Remove CORS tests from `internal/models/config_test.go`**

Search for any test cases, table rows, or struct literals that reference `CORSConfig` or `Server.CORS`. Remove them. The test functions themselves may survive if they test other fields — only remove the CORS-specific assertions and struct fields.

**Step 4: Run tests**

```bash
make test
```

Expected: PASS (CORS middleware still exists in routes.go but doesn't reference `CORSConfig` directly at this point — it will be removed in Task 4).

**Step 5: Commit**

```bash
git add internal/models/config.go internal/models/config_test.go internal/config/config.go
git commit -m "feat: remove CORSConfig from models and environment loader"
```

---

### Task 3: Remove `RateLimitConfig`, `JWTSecret`, and `TrustedProxies` from models and environment loader

**Files:**
- Modify: `internal/models/config.go`
- Modify: `internal/models/config_test.go`
- Modify: `internal/config/config.go`

**Step 1: Remove fields from `SecurityConfig` in `internal/models/config.go`**

The current struct:
```go
type SecurityConfig struct {
    BootstrapKey string          `yaml:"bootstrap_key" json:"bootstrap_key"`
    RateLimit    RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
    JWTSecret    string          `yaml:"jwt_secret" json:"jwt_secret"`
    EnableAuth     bool     `yaml:"enable_auth" json:"enable_auth"`
    TrustedProxies []string `yaml:"trusted_proxies" json:"trusted_proxies"`
}
```

After removal:
```go
// SecurityConfig holds authentication and authorisation settings.
type SecurityConfig struct {
    // BootstrapKey is the initial admin API key seeded into storage on first startup
    // when the api_keys table is empty. Required when EnableAuth is true.
    // Set via the UPDATER_BOOTSTRAP_KEY environment variable or security.bootstrap_key
    // in the config file. After the first startup, keys are managed via the REST API.
    BootstrapKey string `yaml:"bootstrap_key" json:"bootstrap_key"`
    // EnableAuth toggles API key authentication. When false all endpoints are public.
    EnableAuth bool `yaml:"enable_auth" json:"enable_auth"`
}
```

**Step 2: Delete `RateLimitConfig` type entirely**

Remove the entire `RateLimitConfig` struct (lines 102–116).

**Step 3: Remove defaults from `NewDefaultConfig()`**

Delete this block from `NewDefaultConfig()`:
```go
Security: SecurityConfig{
    RateLimit: RateLimitConfig{
        Enabled:                        true,
        RequestsPerMinute:              60,
        BurstSize:                      10,
        AuthenticatedRequestsPerMinute: 120,
        AuthenticatedBurstSize:         20,
        CleanupInterval:                5 * time.Minute,
    },
    EnableAuth:     false,
    TrustedProxies: []string{},
},
```

Replace with:
```go
Security: SecurityConfig{
    EnableAuth: false,
},
```

**Step 4: Remove from `internal/config/config.go`**

Delete the JWT secret env var block:
```go
if secret := os.Getenv("UPDATER_JWT_SECRET"); secret != "" {
    config.Security.JWTSecret = secret
}
```

Delete the rate limit env var blocks:
```go
if rateLimit := os.Getenv("UPDATER_RATE_LIMIT_ENABLED"); rateLimit != "" {
    config.Security.RateLimit.Enabled = strings.ToLower(rateLimit) == "true"
}
if rpm := os.Getenv("UPDATER_RATE_LIMIT_RPM"); rpm != "" {
    if r, err := strconv.Atoi(rpm); err == nil {
        config.Security.RateLimit.RequestsPerMinute = r
    }
}
if burst := os.Getenv("UPDATER_RATE_LIMIT_BURST"); burst != "" {
    if b, err := strconv.Atoi(burst); err == nil {
        config.Security.RateLimit.BurstSize = b
    }
}
```

In `SaveExample`, remove:
```go
config.Security.JWTSecret = "your-jwt-secret-here"
```

**Step 5: Remove rate limit tests from `internal/models/config_test.go`**

Delete `TestRateLimitConfig_Structure` (line 864 onward). Remove any table rows that set `RateLimitConfig` fields in other test functions.

**Step 6: Run tests**

```bash
make test
```

Expected: PASS. The build may still reference `WithRateLimiter` in routes.go — that is removed in Task 4.

**Step 7: Commit**

```bash
git add internal/models/config.go internal/models/config_test.go internal/config/config.go
git commit -m "feat: remove RateLimitConfig, JWTSecret, and TrustedProxies from config"
```

---

### Task 4: Remove `corsMiddleware` and `WithRateLimiter` from the API router

**Files:**
- Modify: `internal/api/routes.go`
- Modify: `internal/api/security_test.go`

**Step 1: Remove from `internal/api/routes.go`**

Delete the `corsMiddleware` function (lines 176–201) in its entirety.

Delete the `WithRateLimiter` function:
```go
// WithRateLimiter adds rate limiting middleware to the router.
func WithRateLimiter(middleware func(http.Handler) http.Handler) RouteOption {
    return func(r *mux.Router) {
        r.Use(middleware)
    }
}
```

Remove the CORS opt-in block from `SetupRoutes`:
```go
if config.Server.CORS.Enabled {
    router.Use(corsMiddleware(config.Server.CORS))
}
```

Remove the now-unused helper functions `contains` and `joinStrings` at the bottom of the file if nothing else references them. Verify with `make vet`.

**Step 2: Remove CORS tests from `internal/api/security_test.go`**

Remove all test cases or sub-tests that set `CORS: models.CORSConfig{...}` on the test config or assert on `Access-Control-Allow-*` response headers. The `TestCORSHeaders` sub-test (around line 762) should be deleted entirely.

For any test that builds a `models.Config` with a `Server.CORS` field, remove that field from the struct literal — the surrounding test itself may remain if it tests something else.

**Step 3: Run tests**

```bash
make test
```

Expected: PASS and no compilation errors.

**Step 4: Run vet to catch unused imports**

```bash
make vet
```

Fix any unused imports (e.g., `fmt` if it was only used by `corsMiddleware`).

**Step 5: Commit**

```bash
git add internal/api/routes.go internal/api/security_test.go
git commit -m "feat: remove corsMiddleware and WithRateLimiter from router"
```

---

### Task 5: Add deprecation warning for stale config keys

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write the failing test first**

Add this test to `internal/config/config_test.go`:

```go
func TestWarnDeprecatedKeys(t *testing.T) {
    tests := []struct {
        name     string
        yaml     string
        wantWarn []string
    }{
        {
            name:     "no deprecated keys",
            yaml:     "server:\n  port: 8080\n",
            wantWarn: nil,
        },
        {
            name:     "cors key warns",
            yaml:     "server:\n  cors:\n    enabled: true\n",
            wantWarn: []string{"server.cors"},
        },
        {
            name:     "rate_limit key warns",
            yaml:     "security:\n  rate_limit:\n    enabled: true\n",
            wantWarn: []string{"security.rate_limit"},
        },
        {
            name:     "jwt_secret key warns",
            yaml:     "security:\n  jwt_secret: \"abc\"\n",
            wantWarn: []string{"security.jwt_secret"},
        },
        {
            name:     "trusted_proxies key warns",
            yaml:     "security:\n  trusted_proxies:\n    - \"10.0.0.1\"\n",
            wantWarn: []string{"security.trusted_proxies"},
        },
        {
            name: "all deprecated keys warn",
            yaml: "server:\n  cors:\n    enabled: true\nsecurity:\n  jwt_secret: \"x\"\n  rate_limit:\n    enabled: true\n  trusted_proxies: []\n",
            wantWarn: []string{"server.cors", "security.jwt_secret", "security.rate_limit", "security.trusted_proxies"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var warnings []string
            handler := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})
            // Replace with a capturing handler
            capturingHandler := &capturingSlogHandler{warned: &warnings}
            logger := slog.New(capturingHandler)
            orig := slog.Default()
            slog.SetDefault(logger)
            defer slog.SetDefault(orig)

            _ = handler // suppress unused var

            warnDeprecatedKeys([]byte(tt.yaml))

            if len(tt.wantWarn) == 0 && len(warnings) > 0 {
                t.Errorf("expected no warnings, got %v", warnings)
            }
            for _, want := range tt.wantWarn {
                found := false
                for _, w := range warnings {
                    if strings.Contains(w, want) {
                        found = true
                        break
                    }
                }
                if !found {
                    t.Errorf("expected warning containing %q, got %v", want, warnings)
                }
            }
        })
    }
}

// capturingSlogHandler captures Warn-level log messages for testing.
type capturingSlogHandler struct {
    warned *[]string
}

func (h *capturingSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
    return level >= slog.LevelWarn
}

func (h *capturingSlogHandler) Handle(_ context.Context, r slog.Record) error {
    *h.warned = append(*h.warned, r.Message)
    return nil
}

func (h *capturingSlogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingSlogHandler) WithGroup(_ string) slog.Handler      { return h }
```

Add imports to `config_test.go` if missing: `"context"`, `"io"`, `"log/slog"`, `"strings"`.

**Step 2: Run the test to confirm it fails**

```bash
make test
```

Expected: FAIL — `warnDeprecatedKeys` is not defined yet.

**Step 3: Implement `warnDeprecatedKeys` in `internal/config/config.go`**

Add this type and function before `loadFromFile`:

```go
// deprecatedConfig mirrors removed config fields for detecting stale operator configs.
type deprecatedConfig struct {
    Server struct {
        CORS interface{} `yaml:"cors"`
    } `yaml:"server"`
    Security struct {
        JWTSecret      string      `yaml:"jwt_secret"`
        TrustedProxies interface{} `yaml:"trusted_proxies"`
        RateLimit      interface{} `yaml:"rate_limit"`
    } `yaml:"security"`
}

// warnDeprecatedKeys logs a warning for each removed config key found in the YAML data.
// The service continues to start normally — these keys are silently ignored by the main decoder.
func warnDeprecatedKeys(data []byte) {
    var dep deprecatedConfig
    if err := yaml.Unmarshal(data, &dep); err != nil {
        return
    }
    if dep.Server.CORS != nil {
        slog.Warn("Config key 'server.cors' is no longer supported; configure CORS at your reverse proxy. See docs/reverse-proxy.md.")
    }
    if dep.Security.JWTSecret != "" {
        slog.Warn("Config key 'security.jwt_secret' is no longer used and can be removed from your config file.")
    }
    if dep.Security.TrustedProxies != nil {
        slog.Warn("Config key 'security.trusted_proxies' is no longer supported; configure proxy trust at your reverse proxy. See docs/reverse-proxy.md.")
    }
    if dep.Security.RateLimit != nil {
        slog.Warn("Config key 'security.rate_limit' is no longer supported; configure rate limiting at your reverse proxy. See docs/reverse-proxy.md.")
    }
}
```

Call it from `loadFromFile`, after reading the file and before unmarshaling into the main config:

```go
func loadFromFile(config *models.Config, filePath string) error {
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        return fmt.Errorf("config file not found: %s", filePath)
    }
    data, err := os.ReadFile(filePath)
    if err != nil {
        return fmt.Errorf("failed to read config file: %w", err)
    }
    warnDeprecatedKeys(data)  // <-- add this line
    if err := yaml.Unmarshal(data, config); err != nil {
        return fmt.Errorf("failed to parse YAML config: %w", err)
    }
    return nil
}
```

**Step 4: Run tests**

```bash
make test
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: warn on deprecated config keys at startup"
```

---

### Task 6: Add reverse-proxy example files and clean up existing configs

**Files:**
- Create: `examples/nginx/nginx.conf`
- Create: `examples/nginx/docker-compose.yml`
- Create: `examples/traefik/docker-compose.yml`
- Modify: `docker/nginx/nginx.conf`
- Modify: `examples/config.yaml`
- Delete: `configs/security-examples.yaml` (entirely obsolete — uses removed config keys and old API key format)

**Step 1: Delete the obsolete security examples config**

```bash
git rm configs/security-examples.yaml
```

**Step 2: Create `examples/nginx/nginx.conf`**

```nginx
# Updater Service — nginx Reverse Proxy
# Handles TLS termination, CORS, rate limiting, and real-IP forwarding.
# The updater service runs on port 8080 and performs no TLS, CORS, or rate limiting itself.

events {
    worker_connections 1024;
}

http {
    sendfile on;
    server_tokens off;

    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # Rate limiting: two zones keyed on client IP.
    # Adjust rates to match your traffic profile.
    limit_req_zone $binary_remote_addr zone=api_anon:10m rate=60r/m;
    limit_req_zone $binary_remote_addr zone=api_auth:10m rate=300r/m;

    # Logging
    log_format main '$remote_addr - [$time_local] "$request" $status "$http_user_agent"';
    access_log /var/log/nginx/access.log main;
    error_log  /var/log/nginx/error.log warn;

    upstream updater {
        server updater:8080;
        keepalive 32;
    }

    # HTTP -> HTTPS redirect
    server {
        listen 80;
        server_name _;

        location /.well-known/acme-challenge/ {
            root /var/www/certbot;
        }

        location / {
            return 301 https://$host$request_uri;
        }
    }

    server {
        listen 443 ssl http2;
        server_name api.example.com;  # Replace with your domain

        ssl_certificate     /etc/nginx/ssl/fullchain.pem;
        ssl_certificate_key /etc/nginx/ssl/privkey.pem;
        ssl_protocols       TLSv1.2 TLSv1.3;
        ssl_prefer_server_ciphers off;
        ssl_session_cache   shared:SSL:10m;
        ssl_session_timeout 10m;

        # Security headers
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
        add_header X-Content-Type-Options    nosniff always;
        add_header X-Frame-Options           DENY always;
        add_header Referrer-Policy           "strict-origin-when-cross-origin" always;

        # CORS — adjust allowed origins for your frontend domain(s)
        add_header Access-Control-Allow-Origin  "*" always;
        add_header Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS" always;
        add_header Access-Control-Allow-Headers "Authorization, Content-Type" always;
        add_header Access-Control-Max-Age       86400 always;

        # Health check (higher rate limit; no auth required)
        location = /health {
            limit_req zone=api_anon burst=10 nodelay;
            proxy_pass         http://updater;
            proxy_set_header   Host              $host;
            proxy_set_header   X-Real-IP         $remote_addr;
            proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto $scheme;
        }

        # Public update-check endpoints (anonymous rate limit)
        location ~ ^/api/v1/(updates|check|latest) {
            if ($request_method = OPTIONS) { return 204; }
            limit_req zone=api_anon burst=20 nodelay;
            client_max_body_size 64k;
            proxy_pass         http://updater;
            proxy_set_header   Host              $host;
            proxy_set_header   X-Real-IP         $remote_addr;
            proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto $scheme;
            proxy_connect_timeout 5s;
            proxy_read_timeout    10s;
        }

        # Authenticated API endpoints (higher rate limit)
        location /api/ {
            if ($request_method = OPTIONS) { return 204; }
            limit_req zone=api_auth burst=50 nodelay;
            client_max_body_size 1M;
            proxy_pass         http://updater;
            proxy_set_header   Host              $host;
            proxy_set_header   X-Real-IP         $remote_addr;
            proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto $scheme;
            proxy_connect_timeout 5s;
            proxy_read_timeout    30s;
        }

        # Admin UI
        location /admin/ {
            limit_req zone=api_auth burst=20 nodelay;
            proxy_pass         http://updater;
            proxy_set_header   Host              $host;
            proxy_set_header   X-Real-IP         $remote_addr;
            proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto $scheme;
        }

        location / {
            return 404;
        }
    }
}
```

**Step 3: Create `examples/nginx/docker-compose.yml`**

```yaml
# Updater Service with nginx reverse proxy
# Suitable for production deployments where TLS certificates are already present on the host.
# Copy your fullchain.pem and privkey.pem into ./ssl/ before starting.
#
# Usage:
#   cp /etc/letsencrypt/live/api.example.com/fullchain.pem ssl/
#   cp /etc/letsencrypt/live/api.example.com/privkey.pem   ssl/
#   UPDATER_BOOTSTRAP_KEY=$(openssl rand -base64 32) docker compose up -d

services:
  updater:
    image: ghcr.io/griffinskudder/updater:latest
    environment:
      UPDATER_BOOTSTRAP_KEY: "${UPDATER_BOOTSTRAP_KEY}"
      UPDATER_ENABLE_AUTH: "true"
      UPDATER_STORAGE_TYPE: "sqlite"
      UPDATER_DATABASE_DSN: "/data/updater.db"
      UPDATER_LOG_FORMAT: "json"
    volumes:
      - updater_data:/data
    expose:
      - "8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      updater:
        condition: service_healthy
    restart: unless-stopped

volumes:
  updater_data:
```

**Step 4: Create `examples/traefik/docker-compose.yml`**

```yaml
# Updater Service with Traefik v3 reverse proxy
# Traefik handles TLS (Let's Encrypt), CORS middleware, and rate limiting via labels.
#
# Usage:
#   Replace api.example.com and admin@example.com with your values.
#   UPDATER_BOOTSTRAP_KEY=$(openssl rand -base64 32) docker compose up -d

services:
  traefik:
    image: traefik:v3
    command:
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--entrypoints.web.http.redirections.entrypoint.to=websecure"
      - "--entrypoints.web.http.redirections.entrypoint.scheme=https"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge=true"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "traefik_certs:/letsencrypt"
    restart: unless-stopped

  updater:
    image: ghcr.io/griffinskudder/updater:latest
    environment:
      UPDATER_BOOTSTRAP_KEY: "${UPDATER_BOOTSTRAP_KEY}"
      UPDATER_ENABLE_AUTH: "true"
      UPDATER_STORAGE_TYPE: "sqlite"
      UPDATER_DATABASE_DSN: "/data/updater.db"
      UPDATER_LOG_FORMAT: "json"
    volumes:
      - updater_data:/data
    expose:
      - "8080"
    restart: unless-stopped
    labels:
      - "traefik.enable=true"

      # Router: HTTPS with Let's Encrypt
      - "traefik.http.routers.updater.rule=Host(`api.example.com`)"
      - "traefik.http.routers.updater.entrypoints=websecure"
      - "traefik.http.routers.updater.tls.certresolver=letsencrypt"
      - "traefik.http.routers.updater.middlewares=updater-cors,updater-ratelimit,updater-security"

      # CORS middleware — adjust Access-Control-Allow-Origin for your frontend domain
      - "traefik.http.middlewares.updater-cors.headers.accesscontrolallowmethods=GET,POST,PUT,DELETE,OPTIONS"
      - "traefik.http.middlewares.updater-cors.headers.accesscontrolalloworiginlist=*"
      - "traefik.http.middlewares.updater-cors.headers.accesscontrolallowheaders=Authorization,Content-Type"
      - "traefik.http.middlewares.updater-cors.headers.accesscontrolmaxage=86400"

      # Rate limit middleware (per IP; adjust average and burst to match your traffic)
      - "traefik.http.middlewares.updater-ratelimit.ratelimit.average=60"
      - "traefik.http.middlewares.updater-ratelimit.ratelimit.burst=20"
      - "traefik.http.middlewares.updater-ratelimit.ratelimit.period=1m"

      # Security headers
      - "traefik.http.middlewares.updater-security.headers.stsSeconds=31536000"
      - "traefik.http.middlewares.updater-security.headers.stsIncludeSubdomains=true"
      - "traefik.http.middlewares.updater-security.headers.contentTypeNosniff=true"
      - "traefik.http.middlewares.updater-security.headers.frameDeny=true"

volumes:
  updater_data:
  traefik_certs:
```

**Step 5: Update `docker/nginx/nginx.conf`**

Add CORS headers to the existing `location /api/` block:

```nginx
# CORS — the updater service does not set CORS headers itself
add_header Access-Control-Allow-Origin  "*" always;
add_header Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS" always;
add_header Access-Control-Allow-Headers "Authorization, Content-Type" always;
add_header Access-Control-Max-Age       86400 always;
```

Add before the `proxy_pass` line in the `/api/` location. Also add an OPTIONS preflight handler:
```nginx
if ($request_method = OPTIONS) {
    return 204;
}
```

**Step 6: Update `examples/config.yaml`**

Remove the `cors`, `rate_limit`, `trusted_proxies`, `jwt_secret`, and old `api_keys` sections. The file should become:

```yaml
# Example configuration for the updater service.
# Rate limiting, CORS, and TLS are handled by the reverse proxy (see examples/nginx/ or examples/traefik/).

server:
  port: 8080
  host: "0.0.0.0"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s

storage:
  type: "json"
  path: "./data/releases.json"
  database:
    driver: "sqlite3"
    dsn: ""
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 300s
    conn_max_idle_time: 300s

security:
  enable_auth: true
  bootstrap_key: "${UPDATER_BOOTSTRAP_KEY}"

logging:
  level: "info"
  format: "json"
  output: "stdout"

cache:
  enabled: true
  type: "memory"
  ttl: 300s

metrics:
  enabled: true
  path: "/metrics"
  port: 9090
```

**Step 7: Run tests**

```bash
make test
```

Expected: PASS.

**Step 8: Commit**

```bash
git add examples/nginx/ examples/traefik/ docker/nginx/nginx.conf examples/config.yaml
git commit -m "feat: add nginx and Traefik reverse proxy examples"
```

---

### Task 7: Update documentation

**Files:**
- Create: `docs/reverse-proxy.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `mkdocs.yml`

**Step 1: Create `docs/reverse-proxy.md`**

```markdown
# Reverse Proxy

The updater service does not enforce CORS headers, rate limits, or TLS itself.
These concerns must be configured at the reverse proxy layer for every production deployment.

## Why a reverse proxy?

A reverse proxy (nginx, Traefik, Caddy, Cloudflare) provides these features with
more flexibility and less operational overhead than embedding them in the service:

- TLS termination and certificate renewal (Let's Encrypt)
- CORS header management per route and origin
- Rate limiting with IP-based and token-bucket strategies
- Security headers (HSTS, CSP, X-Frame-Options)
- Load balancing across multiple service replicas

## Example configurations

Ready-to-use configurations are provided in the `examples/` directory:

| Directory | Proxy | What it provides |
|-----------|-------|-----------------|
| `examples/nginx/` | nginx | TLS (manual certs), CORS, rate limiting, security headers |
| `examples/traefik/` | Traefik v3 | TLS (Let's Encrypt), CORS middleware, rate limiting, security headers |

Each directory contains an `nginx.conf` or `docker-compose.yml` ready to use with minor substitution of your domain and certificate paths.

## Real client IP in logs

When running behind a proxy, `r.RemoteAddr` in the service will be the proxy's IP, not the client's IP.
The nginx and Traefik examples forward `X-Real-IP` and `X-Forwarded-For`. If you need the client IP in service logs, read the `X-Real-IP` header in your application or configure your proxy to replace `RemoteAddr` directly.

## TLS

Both example configurations terminate TLS at the proxy and forward plain HTTP to the service on port 8080.
Do not expose port 8080 directly to the internet.

## Migrating from previous config

If your `config.yaml` contains any of the following keys, they are no longer used.
The service will log a warning on startup and continue running.
Remove them from your config and configure the equivalent at your proxy instead.

| Removed config key | Proxy equivalent |
|--------------------|-----------------|
| `server.cors` | `add_header Access-Control-*` (nginx) or `headers` middleware (Traefik) |
| `security.rate_limit` | `limit_req_zone` (nginx) or `rateLimit` middleware (Traefik) |
| `security.trusted_proxies` | Proxy trust is unconditional — remove this key |
| `security.jwt_secret` | Not used — remove this key |
```

**Step 2: Update `docs/SECURITY.md`**

Remove the entire "Rate Limiting" configuration section (including the yaml block with `requests_per_hour`).

Remove the CORS configuration section if present.

Replace them with a short "Proxy Layer" section:

```markdown
## Proxy Layer

Rate limiting, CORS, and TLS are enforced by the reverse proxy in front of the service.
See [Reverse Proxy](reverse-proxy.md) for nginx and Traefik configuration examples.
```

**Step 3: Update `docs/ARCHITECTURE.md`**

In the Security Features list under the API section, remove:
- "Configurable CORS with allowed origins, methods, and headers"
- "Rate limiting: token bucket, two-tier (anonymous vs authenticated)"
- Any reference to `TrustedProxies` or `JWTSecret`

Add instead:
- "CORS, rate limiting, and TLS delegated to the reverse proxy layer"

In the Configuration section, remove the `server.cors` and `security.rate_limit` env var entries from the environment variables table.

**Step 4: Update `mkdocs.yml`**

Add `reverse-proxy.md` to the nav. A sensible location is under the top-level section alongside `docs/storage.md`:

```yaml
- Reverse Proxy: reverse-proxy.md
```

**Step 5: Run tests (docs changes — no unit tests required)**

```bash
make test
```

Expected: PASS.

**Step 6: Commit**

```bash
git add docs/reverse-proxy.md docs/SECURITY.md docs/ARCHITECTURE.md mkdocs.yml
git commit -m "docs: add reverse-proxy guide, remove CORS and rate-limit config docs"
```

---

## Final verification

After all tasks, run the full check:

```bash
make check
```

Expected: fmt + vet + test all pass.

Optionally validate the docs build:

```bash
make docs-build
```

Expected: openapi-validate + docs-generate + docs-db + MkDocs all pass with no broken links.

Then open a PR from `feat/proxy-delegation` to `main`.