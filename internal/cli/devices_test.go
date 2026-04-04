package cli

import (
	"testing"
)

func TestDevicesCommand_Execute(t *testing.T) {
	cmd := newDevicesCmd()
	cmd.SetArgs([]string{})

	_ = cmd.Execute()
}

func TestDevicesCommand_Use(t *testing.T) {
	cmd := newDevicesCmd()
	if cmd.Use != "devices" {
		t.Errorf("expected Use 'devices', got %q", cmd.Use)
	}
}

func TestDevicesCommand_RunE(t *testing.T) {
	cmd := newDevicesCmd()
	if cmd.RunE == nil {
		t.Error("devices command should have RunE function")
	}
}

func TestDevicesCommand_Short(t *testing.T) {
	cmd := newDevicesCmd()
	if cmd.Short == "" {
		t.Error("devices command should have a short description")
	}
}

func TestNewDevicesCmd(t *testing.T) {
	cmd := newDevicesCmd()
	if cmd == nil {
		t.Error("newDevicesCmd should return non-nil command")
	}
}

func TestDevicesCommand_NoArgs(t *testing.T) {
	cmd := newDevicesCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("devices command should not error with no args: %v", err)
	}
}
