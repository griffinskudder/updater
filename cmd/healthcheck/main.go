// Package main is a minimal HTTP health check binary for use in distroless
// containers. It exits 0 when the /health endpoint returns HTTP 200, and 1
// otherwise. Compile with CGO_ENABLED=0 for a fully static binary.
package main

import (
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		os.Exit(1)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
