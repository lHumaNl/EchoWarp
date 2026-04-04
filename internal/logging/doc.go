// Package logging provides structured logging utilities for EchoWarp.
//
// This package offers:
//   - Console logging with charmbracelet/log (colored, human-readable)
//   - File logging with JSON format for machine processing
//   - Combined logging to both console and file
//   - Configurable log levels (debug, info, warn, error)
//   - Multi-handler support for simultaneous outputs
//
// All loggers use Go's structured logging (log/slog) interface.
package logging
