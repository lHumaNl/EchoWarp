package probe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

func TestApplyProbeToConfig_Normal(t *testing.T) {
	cfg := &config.Config{Reverse: true, Duplex: false}
	probe := &ProbeServerResult{
		Mode:        "normal",
		SampleRate:  44100,
		Channels:    2,
		OpusBitrate: 128000,
		TLSRequired: true,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.False(t, cfg.Reverse)
	assert.False(t, cfg.Duplex)
	assert.False(t, cfg.Conference)
	assert.Equal(t, uint32(44100), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, 128000, cfg.OpusBitrate)
	assert.True(t, cfg.TLS)
}

func TestApplyProbeToConfig_Reverse(t *testing.T) {
	cfg := &config.Config{}
	probe := &ProbeServerResult{
		Mode:       "reverse",
		SampleRate: 48000,
		Channels:   1,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.True(t, cfg.Reverse)
	assert.False(t, cfg.Duplex)
	assert.False(t, cfg.Conference)
}

func TestApplyProbeToConfig_Duplex(t *testing.T) {
	cfg := &config.Config{}
	probe := &ProbeServerResult{Mode: "duplex"}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.False(t, cfg.Reverse)
	assert.True(t, cfg.Duplex)
	assert.False(t, cfg.Conference)
}

func TestApplyProbeToConfig_Conference(t *testing.T) {
	cfg := &config.Config{}
	probe := &ProbeServerResult{Mode: "conference"}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.False(t, cfg.Reverse)
	assert.False(t, cfg.Duplex)
	assert.True(t, cfg.Conference)
}

func TestApplyProbeToConfig_ZeroValues(t *testing.T) {
	cfg := &config.Config{
		SampleRate:  48000,
		Channels:    2,
		OpusBitrate: 64000,
		MaxClients:  3,
	}
	probe := &ProbeServerResult{
		Mode:        "normal",
		SampleRate:  0,
		Channels:    0,
		OpusBitrate: 0,
		MaxClients:  0,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	// Zero values should not override existing config.
	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, 64000, cfg.OpusBitrate)
	assert.Equal(t, 3, cfg.MaxClients)
}

func TestApplyProbeToConfig_MaxClients(t *testing.T) {
	cfg := &config.Config{MaxClients: 1}
	probe := &ProbeServerResult{Mode: "normal", MaxClients: 4}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.MaxClients)
}

func TestApplyProbeToConfig_PasswordRequired(t *testing.T) {
	cfg := &config.Config{}
	probe := &ProbeServerResult{
		Mode:             "normal",
		PasswordRequired: true,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestApplyProbeToConfig_PasswordProvided(t *testing.T) {
	cfg := &config.Config{Password: "secret"}
	probe := &ProbeServerResult{
		Mode:             "normal",
		PasswordRequired: true,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
}

func TestApplyProbeToConfig_TLSSelfSigned(t *testing.T) {
	cfg := &config.Config{}
	probe := &ProbeServerResult{
		Mode:          "normal",
		TLSRequired:   true,
		TLSSelfSigned: true,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.True(t, cfg.TLS)
	assert.True(t, cfg.TLSInsecure)
	assert.True(t, cfg.TLSSelfSigned)
}

func TestApplyProbeToConfig_NonCriticalValuesApplied(t *testing.T) {
	cfg := &config.Config{
		SampleRate:  44100,
		Channels:    1,
		OpusBitrate: 64000,
	}
	probe := &ProbeServerResult{
		Mode:        "normal",
		SampleRate:  48000,
		Channels:    2,
		OpusBitrate: 128000,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, 128000, cfg.OpusBitrate)
}

func TestApplyProbeToConfig_TLSNoSelfSigned(t *testing.T) {
	cfg := &config.Config{}
	probe := &ProbeServerResult{
		Mode:          "normal",
		TLSRequired:   true,
		TLSSelfSigned: false,
	}

	err := ApplyProbeToConfig(cfg, probe)
	require.NoError(t, err)
	assert.True(t, cfg.TLS)
	assert.False(t, cfg.TLSInsecure)
	assert.False(t, cfg.TLSSelfSigned)
}
