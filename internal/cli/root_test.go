package cli

import (
	"strings"
	"testing"
)

func TestRootCommand_HasSubcommands(t *testing.T) {
	rootCmd := NewRootCmd()
	cmds := rootCmd.Commands()
	if len(cmds) == 0 {
		t.Error("root command should have subcommands")
	}

	expectedCmds := []string{"server", "client", "devices", "version", "config", "daemon", "update"}
	foundCmds := make(map[string]bool)
	for _, cmd := range cmds {
		foundCmds[cmd.Name()] = true
	}

	for _, expected := range expectedCmds {
		if !foundCmds[expected] {
			t.Errorf("missing subcommand: %s", expected)
		}
	}
}

func TestRootCommand_Help(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("help should not error: %v", err)
	}
}

func TestRootCommand_Use(t *testing.T) {
	rootCmd := NewRootCmd()
	if rootCmd.Use != "echowarp" {
		t.Errorf("expected Use 'echowarp', got %q", rootCmd.Use)
	}
}

func TestRootCommand_Short(t *testing.T) {
	rootCmd := NewRootCmd()
	if rootCmd.Short == "" {
		t.Error("root command should have a short description")
	}
}

func TestRootCommand_Long(t *testing.T) {
	rootCmd := NewRootCmd()
	if rootCmd.Long == "" {
		t.Error("root command should have a long description")
	}
}

func TestRootCommand_SilenceUsage(t *testing.T) {
	rootCmd := NewRootCmd()
	if !rootCmd.SilenceUsage {
		t.Error("root command should have SilenceUsage set to true")
	}
}

func TestRootCommand_SilenceErrors(t *testing.T) {
	rootCmd := NewRootCmd()
	if !rootCmd.SilenceErrors {
		t.Error("root command should have SilenceErrors set to true")
	}
}

func TestNewRootCmd(t *testing.T) {
	rootCmd := NewRootCmd()
	if rootCmd == nil {
		t.Error("NewRootCmd should return non-nil command")
	}
}

func TestRootCommand_SubcommandCount(t *testing.T) {
	rootCmd := NewRootCmd()
	cmds := rootCmd.Commands()
	// 9 subcommands: version, devices, server, client, config, daemon, update, completion, doctor
	if len(cmds) != 9 {
		t.Errorf("expected 9 subcommands, got %d", len(cmds))
	}
}

func TestRenderHelpContent(t *testing.T) {
	content := renderHelpContent(80)
	if content == "" {
		t.Error("renderHelpContent should return non-empty content")
	}
	if !strings.Contains(content, "server") {
		t.Error("help content should mention 'server' command")
	}
	if !strings.Contains(content, "client") {
		t.Error("help content should mention 'client' command")
	}
}

func TestRenderConfigContent_NoConfig(t *testing.T) {
	// Should not panic when config file doesn't exist
	content := renderConfigContent("/nonexistent/path/config.yaml", 80)
	if content == "" {
		t.Error("renderConfigContent should return non-empty content even on error")
	}
}

func TestQuickStartModel_Init(t *testing.T) {
	m := newQuickStartModel(false, "/nonexistent/config.yaml")
	if m.screen != qsScreenMenu {
		t.Error("initial screen should be menu")
	}
	if m.quitting {
		t.Error("model should not be quitting initially")
	}
}

func TestQuickStartModel_InitWithConfig(t *testing.T) {
	m := newQuickStartModel(true, "/tmp/test_config.yaml")
	if m.screen != qsScreenMenu {
		t.Error("initial screen should be menu")
	}
	if !m.hasConfig {
		t.Error("hasConfig should be true")
	}
}

func TestShortenHome(t *testing.T) {
	result := shortenHome("/nonexistent/absolute/path")
	// Should return original path since it doesn't start with home dir
	if result == "" {
		t.Error("shortenHome should return non-empty string")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := defaultConfigPath()
	if path == "" {
		t.Error("defaultConfigPath should return non-empty path")
	}
	if !strings.Contains(path, "echowarp") {
		t.Error("default config path should contain 'echowarp'")
	}
}
