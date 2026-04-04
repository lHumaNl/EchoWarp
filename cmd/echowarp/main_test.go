package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/cli"
)

func TestMain_PackageCompiles(t *testing.T) {
	cmd := cli.NewRootCmd()
	require.NotNil(t, cmd, "NewRootCmd should not return nil")
	require.Equal(t, "echowarp", cmd.Use, "root command should be named 'echowarp'")
	require.NotEmpty(t, cmd.Short, "root command should have a short description")
	require.NotEmpty(t, cmd.Long, "root command should have a long description")
}

func TestMain_VersionFlag(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()

	// Restore stdout
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	require.NoError(t, err, "version command should not error")
	require.Contains(t, output, "EchoWarp", "version output should contain 'EchoWarp'")
}

func TestMain_HelpFlag(t *testing.T) {
	cmd := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	require.NoError(t, err, "help flag should not error")
	output := buf.String()
	require.Contains(t, output, "EchoWarp", "help output should contain 'EchoWarp'")
	require.Contains(t, output, "Available Commands:", "help output should list available commands")
}

func TestMain_HelpCommand(t *testing.T) {
	cmd := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"help"})
	err := cmd.Execute()
	require.NoError(t, err, "help command should not error")
	output := buf.String()
	require.Contains(t, output, "EchoWarp", "help output should contain 'EchoWarp'")
}

func TestMain_InvalidCommand(t *testing.T) {
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	require.Error(t, err, "invalid command should return error")
	require.Contains(t, err.Error(), "unknown command", "error should indicate unknown command")
}

func TestMain_InvalidFlag(t *testing.T) {
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"--invalid-flag-that-does-not-exist"})
	err := cmd.Execute()
	require.Error(t, err, "invalid flag should return error")
}

func TestMain_SubcommandHelp(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"server help", "server"},
		{"client help", "client"},
		{"devices help", "devices"},
		{"config help", "config"},
		{"daemon help", "daemon"},
		{"version help", "version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{tt.command, "--help"})
			err := cmd.Execute()
			require.NoError(t, err, "%s --help should not error", tt.command)
			require.NotEmpty(t, buf.String(), "help output should not be empty")
		})
	}
}

func TestMain_CompletionCommand(t *testing.T) {
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"completion", "bash"})
	err := cmd.Execute()
	require.NoError(t, err, "completion bash should not error")
}
