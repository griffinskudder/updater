# Graceful Shutdown Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the graceful shutdown drain timeout configurable via `shutdown_timeout` config field and `UPDATER_SHUTDOWN_TIMEOUT` env var, and add subprocess integration tests that verify clean exit on SIGTERM and SIGINT.

**Architecture:** Most of the graceful shutdown already exists in `cmd/updater/updater.go` — signal handling, `http.Server.Shutdown`, metrics drain, and storage cleanup via `defer`. This plan adds the missing config field and wires it in, then adds integration tests that compile the binary and send real OS signals to verify clean exit. Tests are subprocess-based because there is only one entrypoint and no plans for a second; a `run()` abstraction refactor is not warranted.

**Tech Stack:** Go stdlib (`os/signal`, `syscall`, `net/http`, `context`), testify, existing `//go:build integration` test suite in `internal/integration/`.

---

### Task 1: Create a new branch

**Files:**
- No file changes — branch setup only.

**Step 1: Check out a new branch from main**

```bash
git checkout main
git pull
git checkout -b feat/issue-70-graceful-shutdown
```

Expected: you are now on `feat/issue-70-graceful-shutdown`.

---

### Task 2: Add ShutdownTimeout to ServerConfig (TDD)

**Files:**
- Modify: `internal/models/config.go`
- Modify: `internal/models/config_test.go`

**Step 1: Write the failing tests**

In `internal/models/config_test.go`, add the following two test cases.

In `TestNewDefaultConfig`, add an assertion after the existing `IdleTimeout` assertion:

```go
assert.Equal(t, 30*time.Second, config.Server.ShutdownTimeout)
```

Add a new table-driven test for the negative-value validation. Find `TestConfig_Validate` (the table-driven test) and add a new entry to its `tests` slice:

```go
{
    name: "negative shutdown timeout",
    config: func() *Config {
        c := NewDefaultConfig()
        c.Storage.Type = "memory"
        c.Server.ShutdownTimeout = -1 * time.Second
        return c
    }(),
    expectError: true,
    errorMsg:    "shutdown timeout",
},
```

**Step 2: Run to verify failure**

```bash
make test
```

Expected: FAIL — `config.Server.ShutdownTimeout` is zero, not `30s`; validation does not reject negative values.

**Step 3: Add the field, default, and validation**

In `internal/models/config.go`, add `ShutdownTimeout` to `ServerConfig`:

```go
type ServerConfig struct {
	Port            int           `yaml:"port" json:"port"`
	Host            string        `yaml:"host" json:"host"`
	ReadTimeout     time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`
	TLSEnabled      bool          `yaml:"tls_enabled" json:"tls_enabled"`
	TLSCertFile     string        `yaml:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile      string        `yaml:"tls_key_file" json:"tls_key_file"`
}
```

In `NewDefaultConfig()`, set the default inside the `Server` block:

```go
ShutdownTimeout: 30 * time.Second,
```

In `ServerConfig.Validate()`, add after the `IdleTimeout` check:

```go
if sc.ShutdownTimeout < 0 {
    return errors.New("shutdown timeout cannot be negative")
}
```

**Step 4: Run to verify pass**

```bash
make test
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/models/config.go internal/models/config_test.go
git commit -m "feat: add ShutdownTimeout field to ServerConfig"
```

---

### Task 3: Parse UPDATER_SHUTDOWN_TIMEOUT env var (TDD)

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write the failing test**

In `internal/config/config_test.go`, find `TestLoad_WithEnvironmentVariables`. That test saves and restores env vars manually with a `defer`. Add `UPDATER_SHUTDOWN_TIMEOUT` to the `originalEnv` map at the top of the test:

```go
"UPDATER_SHUTDOWN_TIMEOUT": os.Getenv("UPDATER_SHUTDOWN_TIMEOUT"),
```

Add the `os.Setenv` call alongside the other `os.Setenv` calls in that test:

```go
os.Setenv("UPDATER_SHUTDOWN_TIMEOUT", "45s")
```

Add an assertion after the existing server config assertions:

```go
assert.Equal(t, 45*time.Second, config.Server.ShutdownTimeout)
```

**Step 2: Run to verify failure**

```bash
make test
```

Expected: FAIL — `ShutdownTimeout` stays at default `30s` because the env var is not parsed yet.

**Step 3: Add env var parsing**

In `internal/config/config.go`, inside `loadFromEnvironment`, add after the `UPDATER_IDLE_TIMEOUT` block:

```go
if timeout := os.Getenv("UPDATER_SHUTDOWN_TIMEOUT"); timeout != "" {
    if d, err := time.ParseDuration(timeout); err == nil {
        config.Server.ShutdownTimeout = d
    }
}
```

**Step 4: Run to verify pass**

```bash
make test
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: parse UPDATER_SHUTDOWN_TIMEOUT env var"
```

---

### Task 4: Wire ShutdownTimeout into the server shutdown sequence

**Files:**
- Modify: `cmd/updater/updater.go`

**Step 1: Replace the hardcoded timeout and improve shutdown logging**

Find the shutdown block (after `<-quit`). Replace it with:

```go
slog.Info("Shutting down server", "timeout", cfg.Server.ShutdownTimeout)
start := time.Now()

ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
defer cancel()

// Shutdown metrics server
if metricsServer != nil {
    if err := metricsServer.Shutdown(ctx); err != nil {
        slog.Error("Metrics server forced to shutdown", "error", err)
    }
}

// Attempt graceful shutdown
if err := server.Shutdown(ctx); err != nil {
    slog.Error("Server forced to shutdown", "error", err)
}

slog.Info("Server shutdown complete", "elapsed", time.Since(start))
```

The `start` variable declaration must come before `ctx, cancel := ...` so it measures the full drain window including the metrics server.

**Step 2: Build to verify no compile errors**

```bash
make build
```

Expected: binary produced at `bin/updater`, no errors.

**Step 3: Commit**

```bash
git add cmd/updater/updater.go
git commit -m "feat: use configurable ShutdownTimeout in server shutdown sequence"
```

---

### Task 5: Subprocess integration tests

**Files:**
- Create: `internal/integration/shutdown_test.go`

**Context:** The existing integration tests in `internal/integration/integration_test.go` use `httptest.NewServer` and never start the real binary. These new tests compile the binary with `go build` in `TestMain` and drive it as a subprocess. `TestMain` adds the build step once for the whole package — existing tests are unaffected.

The `//go:build integration` tag means these tests only run via `make integration-test`, not `make test`.

Note: `syscall.SIGTERM` and `syscall.SIGINT` are Linux signals. These tests run inside the Docker-based test environment where Linux is the OS, so signal delivery works correctly.

**Step 1: Create the test file**

```go
//go:build integration

package integration

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// binaryPath holds the path to the compiled updater binary, built once in TestMain.
var binaryPath string

// TestMain builds the updater binary once before running any integration tests.
// Existing tests in this package are unaffected — they use httptest.NewServer directly.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "updater-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "updater")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/updater")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build updater binary: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// getFreePort returns an available TCP port by binding and immediately releasing it.
func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// waitForServer polls the health endpoint until the server is ready or the deadline passes.
func waitForServer(t *testing.T, port int) {
	t.Helper()
	url := fmt.Sprintf("http://localhost:%d/api/v1/health", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:noctx
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server on port %d did not become ready within 5s", port)
}

// testGracefulShutdown is a shared helper for signal-specific test cases.
// It starts the binary, waits for readiness, sends the given signal, and
// asserts the process exits cleanly within 10 seconds.
func testGracefulShutdown(t *testing.T, sig syscall.Signal) {
	t.Helper()

	port := getFreePort(t)

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		"UPDATER_STORAGE_TYPE=memory",
		fmt.Sprintf("UPDATER_PORT=%d", port),
		"UPDATER_METRICS_ENABLED=false",
		"UPDATER_LOG_FORMAT=text",
		"UPDATER_SHUTDOWN_TIMEOUT=5s",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		// Kill the process if the test ends before clean shutdown (e.g. on failure).
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
	})

	waitForServer(t, port)

	require.NoError(t, cmd.Process.Signal(sig))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		assert.NoError(t, err, "expected clean exit (exit code 0) after signal %s", sig)
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("server did not shut down within 10s after signal %s", sig)
	}
}

func TestGracefulShutdown_SIGTERM(t *testing.T) {
	testGracefulShutdown(t, syscall.SIGTERM)
}

func TestGracefulShutdown_SIGINT(t *testing.T) {
	testGracefulShutdown(t, syscall.SIGINT)
}
```

**Step 2: Run integration tests to verify they pass**

```bash
make integration-test
```

Expected: PASS — both `TestGracefulShutdown_SIGTERM` and `TestGracefulShutdown_SIGINT` pass. The existing integration tests also continue to pass.

**Step 3: Commit**

```bash
git add internal/integration/shutdown_test.go
git commit -m "test: add subprocess integration tests for graceful shutdown"
```

---

### Task 6: Update documentation

**Files:**
- Modify: `examples/config.yaml`
- Modify: `docs/ARCHITECTURE.md`

**Step 1: Update examples/config.yaml**

Add `shutdown_timeout` after `idle_timeout` in the `server:` block:

```yaml
  idle_timeout: 60s
  # shutdown_timeout is the maximum time to wait for in-flight requests to
  # complete after receiving SIGTERM or SIGINT before connections are forcefully closed.
  shutdown_timeout: 30s
```

**Step 2: Find the server config reference table in docs/ARCHITECTURE.md**

Search for `idle_timeout` in `docs/ARCHITECTURE.md` to locate the server config table. Add a row for `shutdown_timeout` alongside the other timeout fields:

```markdown
| `shutdown_timeout` | `UPDATER_SHUTDOWN_TIMEOUT` | `30s` | Maximum time to drain in-flight requests on SIGTERM/SIGINT |
```

**Step 3: Run make check to verify nothing is broken**

```bash
make check
```

Expected: PASS.

**Step 4: Commit**

```bash
git add examples/config.yaml docs/ARCHITECTURE.md
git commit -m "docs: document shutdown_timeout config field"
```

---

### Task 7: Final verification and PR

**Step 1: Run the full test suite**

```bash
make check
```

Expected: PASS.

**Step 2: Run integration tests**

```bash
make integration-test
```

Expected: PASS.

**Step 3: Push and open PR**

```bash
git push -u origin feat/issue-70-graceful-shutdown
```

Then open a PR targeting `main`. Reference `Closes #70` in the PR description.
