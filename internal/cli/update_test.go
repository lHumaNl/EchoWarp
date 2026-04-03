package cli

import (
	"testing"
)

func TestUpdateCommand_Use(t *testing.T) {
	cmd := newUpdateCmd()
	if cmd.Use != "update" {
		t.Errorf("expected Use 'update', got %q", cmd.Use)
	}
}

func TestUpdateCommand_Short(t *testing.T) {
	cmd := newUpdateCmd()
	if cmd.Short == "" {
		t.Error("update command should have a short description")
	}
}

func TestUpdateCommand_Long(t *testing.T) {
	cmd := newUpdateCmd()
	if cmd.Long == "" {
		t.Error("update command should have a long description")
	}
}

func TestUpdateCommand_HasFlags(t *testing.T) {
	cmd := newUpdateCmd()

	flags := []string{"check", "version", "dry-run", "force"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing flag: %s", flag)
		}
	}
}

func TestUpdateCommand_CheckFlag(t *testing.T) {
	cmd := newUpdateCmd()
	checkFlag := cmd.Flags().Lookup("check")
	if checkFlag == nil {
		t.Fatal("check flag not found")
	}
	if checkFlag.DefValue != "false" {
		t.Errorf("expected default value 'false', got %q", checkFlag.DefValue)
	}
}

func TestUpdateCommand_VersionFlag(t *testing.T) {
	cmd := newUpdateCmd()
	versionFlag := cmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Fatal("version flag not found")
	}
	if versionFlag.DefValue != "" {
		t.Errorf("expected default value '', got %q", versionFlag.DefValue)
	}
}

func TestUpdateCommand_DryRunFlag(t *testing.T) {
	cmd := newUpdateCmd()
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Fatal("dry-run flag not found")
	}
	if dryRunFlag.DefValue != "false" {
		t.Errorf("expected default value 'false', got %q", dryRunFlag.DefValue)
	}
}

func TestUpdateCommand_ForceFlag(t *testing.T) {
	cmd := newUpdateCmd()
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("force flag not found")
	}
	if forceFlag.DefValue != "false" {
		t.Errorf("expected default value 'false', got %q", forceFlag.DefValue)
	}
}
