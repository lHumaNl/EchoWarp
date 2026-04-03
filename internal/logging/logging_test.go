package logging

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewConsoleLogger_SetsLevel(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantLevel slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"unknown defaults to info", "unknown", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewConsoleLogger(tt.level)
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
			if !logger.Enabled(nil, tt.wantLevel) {
				t.Errorf("expected level %v to be enabled", tt.wantLevel)
			}
		})
	}
}

func TestNewFileLogger_WritesJSONLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, closer, err := NewFileLogger(logPath, "info")
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	logger.Info("test message", "key", "value")

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), `"msg":"test message"`) {
		t.Errorf("log file should contain message, got: %s", string(data))
	}
	if !strings.Contains(string(data), `"key":"value"`) {
		t.Errorf("log file should contain key-value pair, got: %s", string(data))
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("each line should be valid JSON, got error: %v, line: %s", err, line)
		}
	}
}

func TestNewCombinedLogger_WritesBoth(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "combined.log")

	logger, closer, err := NewCombinedLogger("info", logPath)
	if err != nil {
		t.Fatalf("failed to create combined logger: %v", err)
	}
	defer closer.Close()

	logger.Info("combined test message", "test", "value")

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), `"msg":"combined test message"`) {
		t.Errorf("log file should contain message, got: %s", string(data))
	}
}

func TestNewCombinedLogger_NoFile(t *testing.T) {
	logger, closer, err := NewCombinedLogger("info", "")
	if err != nil {
		t.Fatalf("expected no error when filePath is empty, got: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if closer == nil {
		t.Fatal("expected non-nil closer")
	}
	if err := closer.Close(); err != nil {
		t.Errorf("expected no error closing nop closer, got: %v", err)
	}
}

func TestMultiHandler_WithAttrs(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "attrs.log")

	logger, closer, err := NewFileLogger(logPath, "debug")
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	childLogger := logger.With("service", "test")
	childLogger.Info("message with attr")

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), `"service":"test"`) {
		t.Errorf("log file should contain service attr, got: %s", string(data))
	}
}

func TestParseSlogLevel_AllLevels(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantLevel slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"unknown defaults to info", "unknown", slog.LevelInfo},
		{"case insensitive", "DEBUG", slog.LevelDebug},
		{"mixed case", "WaRn", slog.LevelWarn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "test.log")

			logger, closer, err := NewFileLogger(logPath, tt.level)
			if err != nil {
				t.Fatalf("failed to create file logger: %v", err)
			}
			defer closer.Close()

			// Verify the level is set correctly
			if !logger.Enabled(context.Background(), tt.wantLevel) {
				t.Errorf("expected level %v to be enabled", tt.wantLevel)
			}
		})
	}
}

func TestNewFileLogger_DirCreationError(t *testing.T) {
	// Create a file, then try to create a directory inside it — fails on all platforms.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(blocker, "subdir", "test.log")
	_, _, err := NewFileLogger(logPath, "info")
	if err == nil {
		t.Error("expected error when creating log directory inside a file")
	}
	if !strings.Contains(err.Error(), "create log dir") {
		t.Errorf("expected error to contain 'create log dir', got: %v", err)
	}
}

func TestNewFileLogger_FileOpenError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "readonly.log")

	// Create a directory with the same name as the log file to cause open error
	if err := os.Mkdir(logPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	_, _, err := NewFileLogger(logPath, "info")
	if err == nil {
		t.Error("expected error when opening file that is a directory")
	}
	if !strings.Contains(err.Error(), "open log file") {
		t.Errorf("expected error to contain 'open log file', got: %v", err)
	}
}

func TestMultiHandler_Enabled(t *testing.T) {
	consoleLogger := NewConsoleLogger("warn")
	fileLogger, closer, err := NewFileLogger(os.DevNull, "info")
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}
	defer closer.Close()

	multi := newMultiHandler(consoleLogger.Handler(), fileLogger.Handler())

	// Warn level should be enabled (console is warn, file is info)
	if !multi.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected warn level to be enabled")
	}

	// Info level should be enabled (file logger allows it)
	if !multi.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected info level to be enabled by file logger")
	}

	// Debug level should not be enabled (both handlers require higher level)
	if multi.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug level to be disabled")
	}
}

func TestMultiHandler_Handle(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "handle.log")

	logger, closer, err := NewFileLogger(logPath, "debug")
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}
	defer closer.Close()

	// Test that Handle works correctly
	logger.Debug("debug message", "key", "value")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), `"msg":"debug message"`) {
		t.Errorf("log file should contain debug message, got: %s", string(data))
	}
}

func TestMultiHandler_WithGroup(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "group.log")

	logger, closer, err := NewFileLogger(logPath, "debug")
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}
	defer closer.Close()

	// Create a logger with a group
	groupedLogger := logger.WithGroup("mygroup")
	groupedLogger.Info("grouped message", "key", "value")

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// The group should wrap the attributes
	if !strings.Contains(string(data), `"msg":"grouped message"`) {
		t.Errorf("log file should contain grouped message, got: %s", string(data))
	}
}

func TestMultiHandler_WithAttrsDirect(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "attrs_direct.log")

	logger, closer, err := NewFileLogger(logPath, "debug")
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	// Test WithAttrs directly on the handler
	handler := newMultiHandler(logger.Handler())
	attrHandler := handler.WithAttrs([]slog.Attr{slog.String("custom", "attr")})
	newLogger := slog.New(attrHandler)

	newLogger.Info("message with custom attr")

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), `"custom":"attr"`) {
		t.Errorf("log file should contain custom attr, got: %s", string(data))
	}
}

func TestMultiHandler_WithGroupDirect(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "group_direct.log")

	logger, closer, err := NewFileLogger(logPath, "debug")
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	// Test WithGroup directly on the handler
	handler := newMultiHandler(logger.Handler())
	groupHandler := handler.WithGroup("directgroup")
	newLogger := slog.New(groupHandler)

	newLogger.Info("message with direct group")

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), `"msg":"message with direct group"`) {
		t.Errorf("log file should contain message, got: %s", string(data))
	}
}

func TestNopCloser(t *testing.T) {
	var closer io.Closer = NopCloser
	if err := closer.Close(); err != nil {
		t.Errorf("expected no error from NopCloser.Close(), got: %v", err)
	}
}

func TestNewCombinedLogger_FileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	// Create a directory where the file should be to cause an error
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "readonly.log")

	// Create a directory with the same name as the log file
	if err := os.Mkdir(logPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// NewCombinedLogger should return console logger even when file logging fails
	logger, closer, err := NewCombinedLogger("info", logPath)
	if err == nil {
		t.Error("expected error when file path is a directory")
	}
	if logger == nil {
		t.Fatal("expected non-nil console logger even on file error")
	}
	if closer == nil {
		t.Fatal("expected non-nil closer")
	}

	// Should still be able to log to console
	logger.Info("test message")

	if err := closer.Close(); err != nil {
		t.Errorf("expected no error closing nop closer, got: %v", err)
	}
}

func TestNewFileLogger_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a nested path that doesn't exist
	logPath := filepath.Join(tmpDir, "level1", "level2", "test.log")

	logger, closer, err := NewFileLogger(logPath, "info")
	if err != nil {
		t.Fatalf("expected no error creating nested directories, got: %v", err)
	}
	defer closer.Close()

	// Verify the directory was created
	dir := filepath.Dir(logPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("expected parent directory to be created: %s", dir)
	}

	// Verify we can write to the file
	logger.Info("test message")
}

func TestGetDefaultLogFile_Server(t *testing.T) {
	path := GetDefaultLogFile("server")
	if !strings.Contains(path, "server.log") {
		t.Errorf("expected path to contain 'server.log', got: %s", path)
	}
	if !strings.Contains(path, "echowarp") {
		t.Errorf("expected path to contain 'echowarp', got: %s", path)
	}
}

func TestGetDefaultLogFile_Client(t *testing.T) {
	path := GetDefaultLogFile("client")
	if !strings.Contains(path, "client.log") {
		t.Errorf("expected path to contain 'client.log', got: %s", path)
	}
	if !strings.Contains(path, "echowarp") {
		t.Errorf("expected path to contain 'echowarp', got: %s", path)
	}
}

func TestGetDefaultLogFile_CustomMode(t *testing.T) {
	path := GetDefaultLogFile("custom")
	if !strings.Contains(path, "custom.log") {
		t.Errorf("expected path to contain 'custom.log', got: %s", path)
	}
	if !strings.Contains(path, "echowarp") {
		t.Errorf("expected path to contain 'echowarp', got: %s", path)
	}
}

func TestGetDefaultLogFile_UsesConfigDir(t *testing.T) {
	path := GetDefaultLogFile("test")
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, it should use fallback directory
		if !strings.HasPrefix(path, ".") {
			t.Errorf("expected path to start with '.' when home dir unavailable, got: %s", path)
		}
		return
	}
	expectedPrefix := filepath.Join(home, ".config", "echowarp", "logs")
	if !strings.HasPrefix(path, expectedPrefix) {
		t.Errorf("expected path to start with %s, got: %s", expectedPrefix, path)
	}
}
