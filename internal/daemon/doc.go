// Package daemon provides daemon process management for EchoWarp.
//
// This package handles:
//   - PID file management for daemon processes
//   - Process lifecycle control (start, stop, status)
//   - Process existence checking via signal(0)
//   - Graceful shutdown via SIGTERM
//
// The daemon package enables background operation of EchoWarp
// with proper process tracking and control.
package daemon
