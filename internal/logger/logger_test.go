package logger

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"updater/internal/models"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  slog.Level
		expectErr bool
	}{
		{name: "debug", input: "debug", expected: slog.LevelDebug},
		{name: "info", input: "info", expected: slog.LevelInfo},
		{name: "warn", input: "warn", expected: slog.LevelWarn},
		{name: "error", input: "error", expected: slog.LevelError},
		{name: "uppercase", input: "DEBUG", expected: slog.LevelDebug},
		{name: "mixed case", input: "Info", expected: slog.LevelInfo},
		{name: "invalid", input: "invalid", expectErr: true},
		{name: "empty", input: "", expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := parseLevel(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}
			if level != tt.expected {
				t.Errorf("expected level %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestSetupJSONFormat(t *testing.T) {
	cfg := models.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	logger, closer, err := Setup(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closer != nil {
		t.Error("expected nil closer for stdout")
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestSetupTextFormat(t *testing.T) {
	cfg := models.LoggingConfig{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	}

	logger, closer, err := Setup(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closer != nil {
		t.Error("expected nil closer for stdout")
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestSetupStderrOutput(t *testing.T) {
	cfg := models.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stderr",
	}

	logger, closer, err := Setup(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closer != nil {
		t.Error("expected nil closer for stderr")
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestSetupFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := models.LoggingConfig{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, closer, err := Setup(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closer == nil {
		t.Fatal("expected non-nil closer for file output")
	}
	defer closer.Close()

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Write a log message and verify it appears in the file
	logger.Info("test message", "key", "value")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "test message") {
		t.Errorf("log file does not contain expected message, got: %s", content)
	}
	if !strings.Contains(content, "value") {
		t.Errorf("log file does not contain expected key-value, got: %s", content)
	}
}

func TestSetupFileOutputMissingPath(t *testing.T) {
	cfg := models.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "file",
	}

	_, _, err := Setup(cfg)
	if err == nil {
		t.Error("expected error for file output without path")
	}
}

func TestSetupInvalidFilePath(t *testing.T) {
	cfg := models.LoggingConfig{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: "/nonexistent/directory/path/test.log",
	}

	_, _, err := Setup(cfg)
	if err == nil {
		t.Error("expected error for invalid file path")
	}
}

func TestSetupInvalidLevel(t *testing.T) {
	cfg := models.LoggingConfig{
		Level:  "invalid",
		Format: "json",
		Output: "stdout",
	}

	_, _, err := Setup(cfg)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestOpenWriter(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		filePath  string
		expectErr bool
	}{
		{name: "stdout", output: "stdout"},
		{name: "stderr", output: "stderr"},
		{name: "default fallback", output: "anything"},
		{name: "file missing path", output: "file", expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, closer, err := openWriter(tt.output, tt.filePath)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if writer == nil {
				t.Error("expected non-nil writer")
			}
			if closer != nil {
				closer.Close()
			}
		})
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	// Verify that the logger respects level filtering
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)

	logger.Info("should not appear")
	logger.Warn("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("info message should have been filtered by warn level")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("warn message should have appeared")
	}
}
