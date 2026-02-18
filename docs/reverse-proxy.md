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
|-----------|-------|------------------|
| `examples/nginx/` | nginx | TLS (manual certs), CORS, rate limiting, security headers |
| `examples/traefik/` | Traefik v3 | TLS (Let's Encrypt), CORS middleware, rate limiting, security headers |

Each directory contains an `nginx.conf` or `docker-compose.yml` ready to use with minor substitution of your domain and certificate paths.

## Real client IP in logs

When running behind a proxy, `r.RemoteAddr` in the service will be the proxy IP, not the client IP.
The nginx and Traefik examples forward `X-Real-IP` and `X-Forwarded-For`. If you need the client IP in service logs, read the `X-Real-IP` header in your application or configure your proxy to replace `RemoteAddr` directly.

## TLS

Both example configurations terminate TLS at the proxy and forward plain HTTP to the service on port 8080.
Do not expose port 8080 directly to the internet.

## Migrating from previous config

If your `config.yaml` contains any of the following keys, they are no longer used.
The service logs a warning on startup and continues running.
Remove them from your config and configure the equivalent at your proxy instead.

| Removed config key | Proxy equivalent |
|--------------------|------------------|
| `server.cors` | `add_header Access-Control-*` (nginx) or `headers` middleware (Traefik) |
| `security.rate_limit` | `limit_req_zone` (nginx) or `rateLimit` middleware (Traefik) |
| `security.trusted_proxies` | Proxy trust is unconditional -- remove this key |
| `security.jwt_secret` | Not used -- remove this key |
