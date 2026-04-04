package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCmd_Bash(t *testing.T) {
	rootCmd := NewRootCmd()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "bash"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "bash") {
		t.Error("expected bash completion output to contain 'bash'")
	}
	if !strings.Contains(output, "echowarp") {
		t.Error("expected bash completion output to contain 'echowarp'")
	}
}

func TestCompletionCmd_Zsh(t *testing.T) {
	rootCmd := NewRootCmd()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "zsh"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "echowarp") {
		t.Error("expected zsh completion output to contain 'echowarp'")
	}
	if !strings.Contains(output, "compdef") {
		t.Error("expected zsh completion output to contain 'compdef'")
	}
}

func TestCompletionCmd_Fish(t *testing.T) {
	rootCmd := NewRootCmd()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "fish"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "complete") {
		t.Error("expected fish completion output to contain 'complete'")
	}
	if !strings.Contains(output, "echowarp") {
		t.Error("expected fish completion output to contain 'echowarp'")
	}
}

func TestCompletionCmd_PowerShell(t *testing.T) {
	rootCmd := NewRootCmd()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "powershell"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "echowarp") {
		t.Error("expected powershell completion output to contain 'echowarp'")
	}
	if !strings.Contains(output, "Register-ArgumentCompleter") {
		t.Error("expected powershell completion output to contain 'Register-ArgumentCompleter'")
	}
}

func TestCompletionCmd_InvalidShell(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"completion", "invalid"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid shell, got nil")
	}

	if !strings.Contains(err.Error(), "invalid argument") {
		t.Errorf("expected error about invalid argument, got: %v", err)
	}
}

func TestCompletionCmd_NoArgs(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"completion"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided, got nil")
	}

	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Errorf("expected error about missing argument, got: %v", err)
	}
}
