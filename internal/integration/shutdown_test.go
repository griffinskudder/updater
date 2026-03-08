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

// moduleRoot returns the absolute path to the module root by walking up from the
// current working directory until a go.mod file is found.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

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

	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find module root: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/updater")
	cmd.Dir = root
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
