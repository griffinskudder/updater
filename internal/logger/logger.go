// Package logger provides structured logging initialization for the updater service.
// It configures Go's built-in log/slog package based on the service's LoggingConfig,
// supporting JSON and text output formats, configurable log levels, and multiple
// output destinations (stdout, stderr, file).
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"updater/internal/models"
	"updater/internal/version"
)

// Setup creates and configures a structured logger based on the provided LoggingConfig.
// It returns the configured logger with global version fields, an io.Closer for file
// handles (nil for stdout/stderr), and any error encountered during setup.
//
// The caller is responsible for closing the returned Closer when done (if non-nil).
func Setup(cfg models.LoggingConfig, ver version.Info) (*slog.Logger, io.Closer, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid log level: %w", err)
	}

	writer, closer, err := openWriter(cfg.Output, cfg.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log output: %w", err)
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	// Add global version fields to all log messages
	logger := slog.New(handler).With(
		slog.String("version", ver.Version),
		slog.String("git_commit", ver.GitCommit),
		slog.String("build_date", ver.BuildDate),
	)

	return logger, closer, nil
}

// parseLevel converts a level string to an slog.Level.
// Supported values: debug, info, warn, error (case-insensitive).
func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level: %s", level)
	}
}

// openWriter returns the appropriate io.Writer based on the output configuration.
// For file output, it returns the file as the closer. For stdout/stderr, closer is nil.
func openWriter(output, filePath string) (io.Writer, io.Closer, error) {
	switch strings.ToLower(output) {
	case "stderr":
		return os.Stderr, nil, nil
	case "file":
		if filePath == "" {
			return nil, nil, fmt.Errorf("file path is required when output is file")
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
		}
		return f, f, nil
	default:
		return os.Stdout, nil, nil
	}
}
