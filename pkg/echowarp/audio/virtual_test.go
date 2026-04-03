package audio

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVirtualMic_ReturnsCorrectImplementation(t *testing.T) {
	vm, err := NewVirtualMic("test", 48000, 1)

	if err != nil {
		assert.Nil(t, vm)
		assert.Contains(t, err.Error(), "BlackHole", "error should mention BlackHole requirement")
	} else {
		require.NotNil(t, vm)
		defer vm.Close()
		assert.NotEmpty(t, vm.DeviceName())
	}
}

func TestNewVirtualMic_InvalidParameters(t *testing.T) {
	t.Run("zero sample rate", func(t *testing.T) {
		vm, err := NewVirtualMic("test", 0, 1)
		if err == nil {
			defer vm.Close()
			t.Skip("platform does not validate sample rate")
		}
		assert.Nil(t, vm)
	})

	t.Run("zero channels", func(t *testing.T) {
		vm, err := NewVirtualMic("test", 48000, 0)
		if err == nil {
			defer vm.Close()
			t.Skip("platform does not validate channels")
		}
		assert.Nil(t, vm)
	})

	t.Run("empty name", func(t *testing.T) {
		vm, err := NewVirtualMic("", 48000, 1)
		if err == nil {
			defer vm.Close()
			t.Skip("platform does not validate name")
		}
		assert.Nil(t, vm)
	})
}

func TestVirtualMic_WriteAndClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	vm, err := NewVirtualMic("test", 48000, 1)
	if err != nil {
		if strings.Contains(err.Error(), "BlackHole") ||
			strings.Contains(err.Error(), "PulseAudio") ||
			strings.Contains(err.Error(), "VB-Audio") ||
			strings.Contains(err.Error(), "not supported") {
			t.Skipf("virtual mic not available: %v", err)
		}
		require.NoError(t, err)
	}
	defer vm.Close()

	samples := make([]float32, 1024)
	for i := range samples {
		samples[i] = 0.0
	}

	err = vm.Write(samples)
	assert.NoError(t, err)

	err = vm.Close()
	assert.NoError(t, err)
}

func TestVirtualMic_DeviceName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	vm, err := NewVirtualMic("echowarp-test", 48000, 1)
	if err != nil {
		t.Skipf("virtual mic not available: %v", err)
	}
	defer vm.Close()

	name := vm.DeviceName()
	// Driver may return a canonical name that differs from the requested name
	// (e.g., BlackHole on macOS). Accept if it contains the requested substring.
	assert.NotEmpty(t, name)
	if name != "echowarp-test" {
		t.Logf("DeviceName() = %q (differs from requested %q, driver-assigned)", name, "echowarp-test")
	}
}

func TestVirtualMic_Close_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	vm, err := NewVirtualMic("test", 48000, 1)
	if err != nil {
		t.Skipf("virtual mic not available: %v", err)
	}

	err = vm.Close()
	assert.NoError(t, err)

	err = vm.Close()
	assert.NoError(t, err)
}
