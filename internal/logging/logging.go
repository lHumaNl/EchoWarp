// Package logging provides structured logging setup for EchoWarp.
// It supports console output with charmbracelet/log and file output with JSON format.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	charmlog "github.com/charmbracelet/log"
)

// GetDefaultLogFile returns the default log file path for the given mode.
// Mode should be "server" or "client".
// Uses the unified EchoWarp config directory: ~/.config/echowarp/logs/{mode}.log
// Falls back to ./.echowarp/logs/{mode}.log if the home directory cannot be determined.
func GetDefaultLogFile(mode string) string {
	return filepath.Join(echoWarpDir(), "logs", mode+".log")
}

// echoWarpDir returns the base directory for EchoWarp files.
// Uses ECHOWARP_CONFIG_DIR env var if set, otherwise ~/.config/echowarp.
func echoWarpDir() string {
	if dir := os.Getenv("ECHOWARP_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".echowarp")
	}
	return filepath.Join(home, ".config", "echowarp")
}

// noopCloser is an io.Closer that does nothing.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// NopCloser is a no-op closer for use when no file logging is configured.
var NopCloser io.Closer = noopCloser{}

// NewConsoleLogger creates a console logger with the specified level.
// Uses charmbracelet/log for colorful, human-readable output.
func NewConsoleLogger(level string) *slog.Logger {
	charmLogger := charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		Level:           parseLevel(level),
	})
	return slog.New(charmLogger)
}

// NewFileLogger creates a JSON file logger with the specified level.
// The file is created with 0600 permissions. Parent directories are created with 0700.
// Returns the logger, a Closer for the file, and any error.
func NewFileLogger(path string, level string) (*slog.Logger, io.Closer, error) {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, nil, fmt.Errorf("create log dir %s: %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %s: %w", path, err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: parseSlogLevel(level),
	})
	return slog.New(handler), f, nil
}

// NewCombinedLogger creates a logger that writes to both console and file.
// If filePath is empty, returns a console-only logger with NopCloser.
// File logging errors are logged to console but don't prevent console logging.
func NewCombinedLogger(level string, filePath string) (*slog.Logger, io.Closer, error) {
	consoleLogger := NewConsoleLogger(level)
	if filePath == "" {
		return consoleLogger, NopCloser, nil
	}

	fileLogger, closer, err := NewFileLogger(filePath, level)
	if err != nil {
		return consoleLogger, NopCloser, err
	}
	combined := slog.New(newMultiHandler(consoleLogger.Handler(), fileLogger.Handler()))
	return combined, closer, nil
}

// NewTUILogger creates a logger that writes to both a TUI channel handler and a log file.
// The tuiHandler should be a ChannelLogHandler for live TUI display.
// If filePath is empty, returns a TUI-only logger with NopCloser.
func NewTUILogger(level string, filePath string, tuiHandler slog.Handler) (*slog.Logger, io.Closer, error) {
	if filePath == "" {
		return slog.New(tuiHandler), NopCloser, nil
	}

	fileLogger, closer, err := NewFileLogger(filePath, level)
	if err != nil {
		return slog.New(tuiHandler), NopCloser, err
	}
	combined := slog.New(newMultiHandler(tuiHandler, fileLogger.Handler()))
	return combined, closer, nil
}

func parseLevel(level string) charmlog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return charmlog.DebugLevel
	case "info":
		return charmlog.InfoLevel
	case "warn":
		return charmlog.WarnLevel
	case "error":
		return charmlog.ErrorLevel
	default:
		return charmlog.InfoLevel
	}
}

func parseSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return newMultiHandler(newHandlers...)
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return newMultiHandler(newHandlers...)
}
