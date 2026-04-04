package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckVersionCompat(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()

	t.Run("exact match", func(t *testing.T) {
		Version = "1.0.0-rc1"
		assert.NoError(t, checkVersionCompat("1.0.0-rc1"))
	})

	t.Run("mismatch", func(t *testing.T) {
		Version = "1.0.0"
		err := checkVersionCompat("2.0.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "incompatible version")
	})

	t.Run("pre-release differs", func(t *testing.T) {
		Version = "1.0.0-rc1"
		assert.Error(t, checkVersionCompat("1.0.0-rc2"))
	})

	t.Run("empty local version skips", func(t *testing.T) {
		Version = ""
		assert.NoError(t, checkVersionCompat("2.0.0"))
	})

	t.Run("empty peer version skips", func(t *testing.T) {
		Version = "1.0.0"
		assert.NoError(t, checkVersionCompat(""))
	})
}
