// Package daemon provides process lifecycle management for EchoWarp background operation.
// It handles PID file management, process status checking, and graceful shutdown.
package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// defaultPIDFile is the default path for the PID file.
const defaultPIDFile = "/tmp/echowarp.pid"

// Daemon manages the lifecycle of a background EchoWarp process.
// It uses a PID file to track the running process and enable control commands.
type Daemon struct {
	pidFile string
}

// New creates a Daemon with the given PID file path.
// If pidFile is empty, uses /tmp/echowarp.pid.
func New(pidFile string) *Daemon {
	if pidFile == "" {
		pidFile = defaultPIDFile
	}
	return &Daemon{pidFile: pidFile}
}

// WritePID creates the PID file containing the current process ID.
// Creates parent directories if needed. Returns an error if the file cannot be written.
func (d *Daemon) WritePID() error {
	dir := filepath.Dir(d.pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	return os.WriteFile(d.pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// ReadPID reads and parses the PID from the PID file.
// Returns an error if the file doesn't exist or contains invalid data.
func (d *Daemon) ReadPID() (int, error) {
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID: %w", err)
	}
	return pid, nil
}

// RemovePID deletes the PID file. Typically called during graceful shutdown.
func (d *Daemon) RemovePID() error {
	return os.Remove(d.pidFile)
}

// IsRunning checks if the daemon process is currently running.
// Returns (true, pid) if running, (false, 0) if not running or PID file missing.
func (d *Daemon) IsRunning() (bool, int) {
	pid, err := d.ReadPID()
	if err != nil {
		return false, 0
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false, 0
	}
	return true, pid
}

// Stop sends SIGTERM to the daemon process for graceful shutdown.
// Returns an error if the daemon is not running or the signal cannot be sent.
func (d *Daemon) Stop() error {
	running, pid := d.IsRunning()
	if !running {
		return fmt.Errorf("daemon is not running")
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}
	return nil
}

// PIDFile returns the path to the PID file.
func (d *Daemon) PIDFile() string {
	return d.pidFile
}
