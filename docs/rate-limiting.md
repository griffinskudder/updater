# Rate Limiting

Rate limiting is no longer handled by the updater service itself.
It is the responsibility of the reverse proxy that sits in front of the service.

See [Reverse Proxy](reverse-proxy.md) for nginx and Traefik examples that configure
IP-based rate limiting at the proxy layer.