# Graceful Shutdown Design

**Date:** 2026-03-08
**Issue:** #70
**Status:** Approved

## Background

Graceful shutdown on SIGTERM/SIGINT is already mostly implemented in
`cmd/updater/updater.go`. The signal handler, `http.Server.Shutdown`, metrics
server drain, and storage cleanup via `defer` are all in place. Two gaps
remain from the acceptance criteria:

1. The drain timeout is hardcoded at `30s` and cannot be configured.
2. There are no tests for the shutdown path.

## Approach

### Test strategy: subprocess (Option B)

The integration test compiles the binary and drives it as a subprocess, sending
real OS signals and asserting on exit code and elapsed time.

This approach was chosen because there is only one entrypoint for this service
and none is planned. Since this is not a library or multi-binary repository,
abstracting `main()` into an injectable `run()` function solely for testability
is not justified. The subprocess approach tests the actual compiled artifact
with real signal delivery, which is the most faithful test of the shutdown path.

## Design

### 1. Config field

Add `ShutdownTimeout time.Duration` to `ServerConfig` in
`internal/models/config.go`.

```go
ShutdownTimeout time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`
```

- **Default:** `30s` — matches the current hardcoded value; no behaviour change
  for existing deployments.
- **Validation:** reject negative values in `ServerConfig.Validate()`.
- **Env var:** `UPDATER_SHUTDOWN_TIMEOUT`, parsed with `time.ParseDuration` in
  `internal/config/config.go`.

### 2. Shutdown sequence (`cmd/updater/updater.go`)

Replace the hardcoded `30 * time.Second` with `cfg.Server.ShutdownTimeout` and
log the elapsed duration:

```go
slog.Info("Shutting down server", "timeout", cfg.Server.ShutdownTimeout)
start := time.Now()

ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
defer cancel()

if metricsServer != nil {
    if err := metricsServer.Shutdown(ctx); err != nil {
        slog.Error("Metrics server forced to shutdown", "error", err)
    }
}

if err := server.Shutdown(ctx); err != nil {
    slog.Error("Server forced to shutdown", "error", err)
}

slog.Info("Server shutdown complete", "elapsed", time.Since(start))
```

The existing shutdown ordering is correct and unchanged: metrics server drains
first, then the main HTTP server, then storage and OTel via `defer` (LIFO).

### 3. Integration test (`internal/integration/shutdown_test.go`)

Build tag: `//go:build integration`

**`TestMain`** builds the binary once into a `t.TempDir()` directory before
any test runs. This avoids rebuilding per test case.

**`TestGracefulShutdown_SIGTERM`** and **`TestGracefulShutdown_SIGINT`** each:

1. Find a free port via `net.Listen("tcp", ":0")`.
2. Start the compiled binary with environment variables:
   - `UPDATER_STORAGE_TYPE=memory`
   - `UPDATER_PORT=<port>`
   - `UPDATER_METRICS_ENABLED=false`
   - `UPDATER_LOG_FORMAT=text`
3. Poll `GET /api/v1/health` until ready (tight loop, 5s deadline).
4. Send `syscall.SIGTERM` or `syscall.SIGINT` via `cmd.Process.Signal(...)`.
5. Wait for `cmd.Wait()` with a 5s deadline.
6. Assert exit code 0.

In-flight request drain is not tested explicitly — that is `http.Server.Shutdown`
stdlib behaviour and is not changed by this issue.

### 4. Documentation

- **`examples/config.yaml`** — add `shutdown_timeout: 30s` under the `server:`
  block.
- **`docs/ARCHITECTURE.md`** — add `shutdown_timeout` to the server config
  reference table alongside `read_timeout`, `write_timeout`, and `idle_timeout`.

## Files to change

| File | Change |
|------|--------|
| `internal/models/config.go` | Add `ShutdownTimeout` field and default; validate |
| `internal/config/config.go` | Parse `UPDATER_SHUTDOWN_TIMEOUT` env var |
| `cmd/updater/updater.go` | Use `cfg.Server.ShutdownTimeout`; log elapsed time |
| `internal/integration/shutdown_test.go` | New subprocess integration tests |
| `examples/config.yaml` | Add `shutdown_timeout` |
| `docs/ARCHITECTURE.md` | Document `shutdown_timeout` config field |
