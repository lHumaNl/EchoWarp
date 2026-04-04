package cli

import (
	"testing"
)

func TestVersionCommand_Execute(t *testing.T) {
	cmd := newVersionCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("version command should not error: %v", err)
	}
}

func TestVersionCommand_Use(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Use != "version" {
		t.Errorf("expected Use 'version', got %q", cmd.Use)
	}
}

func TestVersionCommand_Short(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Short == "" {
		t.Error("version command should have a short description")
	}
}

func TestVersionCommand_Run(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Run == nil {
		t.Error("version command should have Run function")
	}
}

func TestNewVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	if cmd == nil {
		t.Error("newVersionCmd should return non-nil command")
	}
}
