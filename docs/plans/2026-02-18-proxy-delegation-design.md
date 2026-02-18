# Proxy Delegation Design

**Date:** 2026-02-18
**Status:** Approved

## Problem

The updater service currently implements CORS, rate limiting, trusted proxy handling, and a JWT secret field in its own application layer. These concerns are better owned by a reverse proxy (nginx, Traefik) that sits in front of the service in every production deployment. Keeping them in the application adds configuration surface, code to maintain, and test coverage obligations without providing value that the proxy layer does not already supply.

## Decision

Remove the four features from the application entirely. The service will document which proxy configuration replaces each removed feature and will emit startup warnings if deprecated config keys are still present in the YAML.

## Scope

### Removed from the application

| Feature | Location | Notes |
|---------|----------|-------|
| `CORSConfig` type + `server.cors` block | `internal/models/config.go` | Proxy sets `Access-Control-Allow-*` headers |
| `RateLimitConfig` type + `security.rate_limit` block | `internal/models/config.go` | Proxy enforces request limits |
| `SecurityConfig.TrustedProxies` | `internal/models/config.go` | Proxy is unconditionally trusted |
| `SecurityConfig.JWTSecret` | `internal/models/config.go` | Unused in the current codebase |
| `corsMiddleware` + `WithRateLimiter` wiring | `internal/api/routes.go` | ~30 lines |
| `internal/ratelimit/` package | entire directory | `limiter.go`, `memory.go`, `middleware.go` + tests |
| Rate limiter wiring | `cmd/updater/updater.go` | ~8 lines |
| CORS + rate limit tests | `internal/api/security_test.go`, `internal/models/config_test.go` | Remove only the cases for removed features |

### Unchanged

- API key authentication (`authMiddleware`, `OptionalAuth`)
- Permission enforcement (`RequirePermission`, read/write/admin hierarchy)
- Admin cookie session middleware (`adminSessionMiddleware`)
- `EnableAuth` config toggle
- All HTTP handlers, OpenAPI spec, request/response models
- Logging middleware, panic recovery middleware

## Deprecation Warning

On startup, `LoadConfig` will perform a second decode into a `deprecatedConfig` struct that retains the old fields:

```go
type deprecatedConfig struct {
    Server   struct { CORS interface{} `yaml:"cors"` }         `yaml:"server"`
    Security struct {
        JWTSecret      string      `yaml:"jwt_secret"`
        TrustedProxies interface{} `yaml:"trusted_proxies"`
        RateLimit      interface{} `yaml:"rate_limit"`
    } `yaml:"security"`
}
```

If any field is non-zero, a `slog.Warn` is emitted naming the offending key and pointing to `docs/reverse-proxy.md`. The service continues to start normally — no fatal error.

## New Files

```
examples/
  nginx/
    nginx.conf          # TLS termination, CORS, rate limiting, X-Forwarded-For
    docker-compose.yml  # nginx + updater compose
  traefik/
    docker-compose.yml  # Traefik v3 + updater with CORS middleware, rate limiting, Let's Encrypt TLS
docs/
  reverse-proxy.md      # Overview: what the proxy must handle, links to examples
```

## Updated Files

| File | Change |
|------|--------|
| `docker/nginx/nginx.conf` | Add `Access-Control-Allow-*` headers; update comments |
| `examples/config.yaml` | Remove `cors`, `jwt_secret`, `rate_limit`, `trusted_proxies`, stale `api_keys` |
| `docs/SECURITY.md` | Remove CORS and rate limiting sections; add "Proxy Layer" section |
| `docs/ARCHITECTURE.md` | Update security features list |
| `mkdocs.yml` | Add `reverse-proxy.md` to nav |
| `internal/models/config.go` | Remove four config types/fields and their defaults |
| `internal/models/config_test.go` | Remove `RateLimitConfig` and `CORSConfig` test cases |
| `internal/api/routes.go` | Remove `corsMiddleware`, `WithRateLimiter`; remove CORS opt-in block |
| `internal/api/security_test.go` | Remove CORS header assertion tests |
| `cmd/updater/updater.go` | Remove `ratelimit` import and limiter construction |
| `internal/config/` | Add `checkDeprecatedKeys` function |

## Proxy Responsibilities

### CORS

nginx sets `Access-Control-Allow-*` response headers on all proxied requests. The service no longer handles preflight OPTIONS responses for CORS purposes (the existing OPTIONS handler for unknown routes returns 204 and is unrelated to CORS).

### Rate limiting

nginx uses two `limit_req_zone` zones keyed on `$binary_remote_addr` to approximate the app's two-tier behaviour:

- Anonymous zone: lower rate (equivalent to the former `requests_per_minute` / `burst_size`)
- A separate zone for the `/api/` path at a higher rate reflects the former `authenticated_requests_per_minute`

A true per-key authenticated rate limit requires custom logic (e.g., `auth_request` + Redis) and is out of scope.

### TLS

Both nginx and Traefik examples terminate TLS externally and forward plain HTTP to the service on port 8080.

### Real client IP

`X-Forwarded-For` and `X-Real-IP` are forwarded by the proxy. The service's request logging reads `r.RemoteAddr` which will be the proxy's IP. If real client IPs are needed in logs, handlers should read `X-Real-IP`. This is a known trade-off — no logging change is in scope for this PR.

## Breaking Config Change

Operators using `server.cors`, `security.rate_limit`, `security.jwt_secret`, or `security.trusted_proxies` in their YAML files will see startup warnings but no crash. They should migrate those concerns to their reverse proxy and remove the deprecated keys from their config.

## Out of Scope

- Updating request logging to use the forwarded real IP
- Distributed rate limiting (Redis-backed) for HA deployments — this is tracked in roadmap item 3.2
- Multi-tenancy changes