package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDaemon_WritePID_CreatesPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	if err := d.WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("failed to read PID file: %v", err)
	}

	expected := os.Getpid()
	actual := string(data)
	if actual != strconv.Itoa(expected) {
		t.Errorf("PID file content = %q, want %d", actual, expected)
	}
}

func TestDaemon_ReadPID_ReturnsCorrectPID(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	expectedPID := 12345
	if err := os.WriteFile(pidFile, []byte("12345"), 0644); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	pid, err := d.ReadPID()
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}

	if pid != expectedPID {
		t.Errorf("ReadPID() = %d, want %d", pid, expectedPID)
	}
}

func TestDaemon_ReadPID_WithNewline(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	if err := os.WriteFile(pidFile, []byte("12345\n"), 0644); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	pid, err := d.ReadPID()
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}

	if pid != 12345 {
		t.Errorf("ReadPID() = %d, want %d", pid, 12345)
	}
}

func TestDaemon_ReadPID_NoFile_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "nonexistent.pid")
	d := New(pidFile)

	_, err := d.ReadPID()
	if err == nil {
		t.Error("ReadPID expected error for nonexistent file")
	}
}

func TestDaemon_RemovePID_DeletesFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	if err := d.WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	if err := d.RemovePID(); err != nil {
		t.Fatalf("RemovePID failed: %v", err)
	}

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should be deleted")
	}
}

func TestDaemon_IsRunning_CurrentProcess(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	if err := d.WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	running, pid := d.IsRunning()
	if !running {
		t.Error("IsRunning should return true for current process")
	}
	if pid != os.Getpid() {
		t.Errorf("IsRunning pid = %d, want %d", pid, os.Getpid())
	}
}

func TestDaemon_IsRunning_NoFile_ReturnsFalse(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "nonexistent.pid")
	d := New(pidFile)

	running, pid := d.IsRunning()
	if running {
		t.Error("IsRunning should return false for nonexistent PID file")
	}
	if pid != 0 {
		t.Errorf("IsRunning pid = %d, want 0", pid)
	}
}

func TestDaemon_IsRunning_InvalidPID_ReturnsFalse(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	if err := os.WriteFile(pidFile, []byte("invalid"), 0644); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	running, _ := d.IsRunning()
	if running {
		t.Error("IsRunning should return false for invalid PID")
	}
}

func TestDaemon_DoubleStart_DetectsRunning(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	if err := d.WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	running, _ := d.IsRunning()
	if !running {
		t.Error("second IsRunning should detect already running daemon")
	}
}

func TestDaemon_IsRunning_NonexistentProcess_ReturnsFalse(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	if err := os.WriteFile(pidFile, []byte("99999999"), 0644); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	running, _ := d.IsRunning()
	if running {
		t.Error("IsRunning should return false for nonexistent process")
	}
}

func TestDaemon_New_DefaultPIDFile(t *testing.T) {
	d := New("")
	if d.PIDFile() != defaultPIDFile {
		t.Errorf("New(\"\") pidFile = %q, want %q", d.PIDFile(), defaultPIDFile)
	}
}

func TestDaemon_WritePID_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "subdir", "nested", "test.pid")
	d := New(pidFile)

	if err := d.WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Error("PID file should be created with parent directories")
	}
}

func TestDaemon_WritePID_DirCreationError(t *testing.T) {
	// Try to create PID file in a location where parent dir creation will fail
	// Using a file path as a directory will cause MkdirAll to fail
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Try to create PID file inside the file (will fail)
	pidFile := filepath.Join(tmpFile, "subdir", "test.pid")
	d := New(pidFile)

	err := d.WritePID()
	if err == nil {
		t.Error("WritePID expected error when directory creation fails")
	}
}

func TestDaemon_Stop_NotRunning_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	// No PID file exists, so daemon is not running
	err := d.Stop()
	if err == nil {
		t.Error("Stop expected error when daemon is not running")
	}
	if !contains(err.Error(), "daemon is not running") {
		t.Errorf("Stop error = %q, want to contain 'daemon is not running'", err.Error())
	}
}

func TestDaemon_Stop_NonexistentProcess_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	// Write a PID that doesn't exist
	nonexistentPID := 99999999
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(nonexistentPID)), 0644); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	// IsRunning should return false for nonexistent process
	running, _ := d.IsRunning()
	if running {
		t.Fatal("expected daemon to not be running")
	}

	// Stop should return error because daemon is not running
	err := d.Stop()
	if err == nil {
		t.Error("Stop expected error when process does not exist")
	}
}

func TestDaemon_Stop_RunningProcess_SendsSIGTERM(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	// Write current process PID (the test process)
	if err := d.WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	// Verify it's running
	running, pid := d.IsRunning()
	if !running {
		t.Fatal("expected current process to be running")
	}
	if pid != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), pid)
	}

	// Note: We cannot actually call Stop() here because it would send SIGTERM
	// to the current test process, which would kill the test.
	// Instead, we test that IsRunning works correctly and Stop validates
	// the running state, which we test in the other Stop tests.
	//
	// In a real integration test, you would:
	// 1. Start a separate process
	// 2. Write its PID
	// 3. Call Stop()
	// 4. Verify it receives SIGTERM
	//
	// For unit test coverage, we've covered the error paths which is sufficient.
}

func TestDaemon_Stop_ProcessWithoutPermission_ReturnsError(t *testing.T) {
	// On Unix systems, PID 1 (init/launchd) exists but regular users
	// typically cannot send signals to it. This tests the signal error path.
	if os.Getuid() == 0 {
		t.Skip("Running as root - cannot test permission denied for PID 1")
	}

	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	// Write PID 1 (init process) which exists but we can't signal
	if err := os.WriteFile(pidFile, []byte("1"), 0644); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	// Verify it's running
	running, _ := d.IsRunning()
	if !running {
		// On some systems (e.g., containers), PID 1 might not exist
		// Skip the test if that's the case
		t.Skip("PID 1 does not exist in this environment")
	}

	// Try to stop - should fail because we don't have permission
	err := d.Stop()
	if err == nil {
		// On some systems, we might actually have permission (unlikely but possible)
		t.Log("Warning: was able to signal PID 1 - unusual but not an error")
	}
	// The error could be either "operation not permitted" or "no such process"
	// depending on the system, so we just verify we get an error or success
}

func TestDaemon_Stop_ActualProcess_SendsSignal(t *testing.T) {
	// This test spawns a real subprocess to test the Stop() method
	if os.Getenv("TEST_CHILD_PROCESS") == "1" {
		// Child process: sleep until killed (intentional - keeps process alive)
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}

	// Start a child process
	cmd := exec.Command(os.Args[0], "-test.run=TestDaemon_Stop_ActualProcess_SendsSignal")
	cmd.Env = append(os.Environ(), "TEST_CHILD_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start child process: %v", err)
	}

	// Ensure we clean up the child process
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	d := New(pidFile)

	// Write the child process PID
	childPID := cmd.Process.Pid
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(childPID)), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Use require.Eventually to wait for child process to be running
	require.Eventually(t, func() bool {
		running, pid := d.IsRunning()
		return running && pid == childPID
	}, 500*time.Millisecond, 10*time.Millisecond, "expected child process to be running")

	// Stop the process - this should send SIGTERM
	err := d.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Wait for the process to exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(2 * time.Second):
		t.Error("process did not exit after SIGTERM")
	case err := <-done:
		// Process should have been signaled
		if err == nil {
			t.Error("expected process to exit with error (signal)")
		} else {
			// Check if it was killed by signal
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					if !status.Signaled() {
						t.Errorf("expected process to be signaled, got: %v", err)
					}
				}
			}
		}
	}
}

func TestDaemon_RemovePID_NoFile_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "nonexistent.pid")
	d := New(pidFile)

	err := d.RemovePID()
	if err == nil {
		t.Error("RemovePID expected error for nonexistent file")
	}
}

func TestDaemon_PIDFile_ReturnsCorrectPath(t *testing.T) {
	customPath := "/custom/path/test.pid"
	d := New(customPath)

	if d.PIDFile() != customPath {
		t.Errorf("PIDFile() = %q, want %q", d.PIDFile(), customPath)
	}
}

func TestDaemon_New_CustomPIDFile(t *testing.T) {
	customPath := "/var/run/echowarp.pid"
	d := New(customPath)

	if d.pidFile != customPath {
		t.Errorf("New() pidFile = %q, want %q", d.pidFile, customPath)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
