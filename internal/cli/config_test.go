package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigInitCommand_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	cmd := newConfigCmd()
	cmd.SetArgs([]string{"init", "--output", configFile})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("config init should not error: %v", err)
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file should be created")
	}
}

func TestConfigCommand_HasInitSubcommand(t *testing.T) {
	cmd := newConfigCmd()
	cmds := cmd.Commands()

	found := false
	for _, c := range cmds {
		if c.Name() == "init" {
			found = true
			break
		}
	}

	if !found {
		t.Error("config command should have 'init' subcommand")
	}
}

func TestConfigInitCommand_HasOutputFlag(t *testing.T) {
	initCmd := newConfigInitCmd()
	flags := initCmd.Flags()
	if flags == nil {
		t.Error("config init command should have flags")
	}

	if flags.Lookup("output") == nil {
		t.Error("config init command should have --output flag")
	}
}

func TestConfigCommand_Use(t *testing.T) {
	cmd := newConfigCmd()
	if cmd.Use != "config" {
		t.Errorf("expected Use 'config', got %q", cmd.Use)
	}
}

func TestConfigInitCommand_Use(t *testing.T) {
	cmd := newConfigInitCmd()
	if cmd.Use != "init" {
		t.Errorf("expected Use 'init', got %q", cmd.Use)
	}
}

func TestConfigInitCommand_RunE(t *testing.T) {
	cmd := newConfigInitCmd()
	if cmd.RunE == nil {
		t.Error("config init command should have RunE function")
	}
}

func TestConfigInitCommand_DefaultOutput(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "echowarp")
	_ = os.MkdirAll(configDir, 0755)

	cmd := newConfigInitCmd()
	cmd.SetArgs([]string{"--output", filepath.Join(configDir, "config.yaml")})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("config init should not error: %v", err)
	}
}

func TestConfigInitCommand_OutputFlagShorthand(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	cmd := newConfigInitCmd()
	cmd.SetArgs([]string{"-o", configFile})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("config init should not error: %v", err)
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file should be created")
	}
}

func TestNewConfigCmd(t *testing.T) {
	cmd := newConfigCmd()
	if cmd == nil {
		t.Error("newConfigCmd should return non-nil command")
	}
}

func TestNewConfigInitCmd(t *testing.T) {
	cmd := newConfigInitCmd()
	if cmd == nil {
		t.Error("newConfigInitCmd should return non-nil command")
	}
}
