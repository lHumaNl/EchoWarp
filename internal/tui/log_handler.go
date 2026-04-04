package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// LogEntry represents a single log record captured during streaming.
type LogEntry struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Attrs   string // pre-formatted "key=val key=val"
}

// LogMsg is a bubbletea message carrying a new log entry.
type LogMsg struct {
	Entry LogEntry
}

// Format formats a LogEntry as a single display line: "15:04:05 [INF] message  key=val"
func (e LogEntry) Format() string {
	ts := e.Time.Format("15:04:05")
	level := logLevelLabel(e.Level)
	if e.Attrs != "" {
		return fmt.Sprintf("%s %s %s  %s", ts, level, e.Message, e.Attrs)
	}
	return fmt.Sprintf("%s %s %s", ts, level, e.Message)
}

func logLevelLabel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "[ERR]"
	case l >= slog.LevelWarn:
		return "[WRN]"
	case l >= slog.LevelInfo:
		return "[INF]"
	default:
		return "[DBG]"
	}
}

// ChannelLogHandler is a slog.Handler that forwards log entries to a channel for TUI display.
// Implements slog.Handler interface.
type ChannelLogHandler struct {
	ch    chan<- LogEntry
	level slog.Level
	attrs []slog.Attr
}

// NewChannelLogHandler creates a ChannelLogHandler that sends records at or above minLevel to ch.
// ch should be buffered to avoid blocking the logging goroutine.
func NewChannelLogHandler(ch chan<- LogEntry, minLevel slog.Level) *ChannelLogHandler {
	return &ChannelLogHandler{ch: ch, level: minLevel}
}

func (h *ChannelLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *ChannelLogHandler) Handle(_ context.Context, r slog.Record) error {
	var parts []string
	for _, a := range h.attrs {
		parts = append(parts, fmt.Sprintf("%s=%v", a.Key, a.Value))
	}
	r.Attrs(func(a slog.Attr) bool {
		parts = append(parts, fmt.Sprintf("%s=%v", a.Key, a.Value))
		return true
	})
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level,
		Message: r.Message,
		Attrs:   strings.Join(parts, " "),
	}
	select {
	case h.ch <- entry:
	default: // drop if full — never block the caller
	}
	return nil
}

func (h *ChannelLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(combined, h.attrs)
	copy(combined[len(h.attrs):], attrs)
	return &ChannelLogHandler{ch: h.ch, level: h.level, attrs: combined}
}

func (h *ChannelLogHandler) WithGroup(_ string) slog.Handler {
	return h // groups not needed for TUI display
}

// ParseSlogLevel converts a string log level to slog.Level.
func ParseSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
