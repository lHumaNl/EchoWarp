package views

import (
	"os"
	"testing"
)

// TestMain isolates the views test package from the developer's real
// EchoWarp config directory. Without this, preset.Load() reads
// ~/.config/echowarp/server_presets.yaml from the user's home dir,
// and any stale state (e.g. max_clients=2 from a prior conference
// session) leaks into tests that construct a SetupModel.
//
// config.EchoWarpDir() honors ECHOWARP_CONFIG_DIR when set, so we
// point it at an empty throwaway temp dir for the duration of the
// test binary. The dir is removed on exit.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "echowarp-views-test-*")
	if err != nil {
		panic("failed to create test config dir: " + err.Error())
	}
	if err := os.Setenv("ECHOWARP_CONFIG_DIR", tmp); err != nil {
		panic("failed to set ECHOWARP_CONFIG_DIR: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
