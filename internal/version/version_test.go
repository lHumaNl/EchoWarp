package version

import "testing"

func TestVersionNoVPrefix(t *testing.T) {
	if len(Version) > 0 && Version[0] == 'v' {
		t.Errorf("Version should not start with 'v', got %q", Version)
	}
}

func TestTrimPrefix(t *testing.T) {
	Version = "v1.2.3"
	initVersion()
	if Version != "1.2.3" {
		t.Errorf("expected 1.2.3, got %s", Version)
	}
}
