package probe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

func baseConfig() config.Config {
	return config.Config{
		SampleRate:  48000,
		Channels:    1,
		OpusBitrate: 64000,
		MaxClients:  1,
	}
}

func baseProbe() *ProbeServerResult {
	return &ProbeServerResult{
		Mode:        "normal",
		SampleRate:  48000,
		Channels:    1,
		OpusBitrate: 64000,
		MaxClients:  1,
	}
}

func TestCompareServerParams_NoChanges(t *testing.T) {
	changes := CompareServerParams(baseConfig(), baseProbe())
	assert.Empty(t, changes)
}

func TestCompareServerParams_ModeChange_Critical(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		probe   string
		oldMode string
	}{
		{"normal→reverse", baseConfig(), "reverse", "normal"},
		{"normal→duplex", baseConfig(), "duplex", "normal"},
		{"reverse→normal", func() config.Config { c := baseConfig(); c.Reverse = true; return c }(), "normal", "reverse"},
		{"duplex→conference", func() config.Config { c := baseConfig(); c.Duplex = true; return c }(), "conference", "duplex"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := baseProbe()
			probe.Mode = tt.probe
			changes := CompareServerParams(tt.cfg, probe)
			require.NotEmpty(t, changes)
			found := false
			for _, c := range changes {
				if c.Field == "Mode" {
					found = true
					assert.True(t, c.Critical)
					assert.Equal(t, tt.oldMode, c.OldValue)
					assert.Equal(t, tt.probe, c.NewValue)
				}
			}
			assert.True(t, found, "expected Mode change")
		})
	}
}

func TestCompareServerParams_TLSChange_Critical(t *testing.T) {
	// off→on
	cfg := baseConfig()
	probe := baseProbe()
	probe.TLSRequired = true
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "TLS", changes[0].Field)
	assert.True(t, changes[0].Critical)
	assert.Equal(t, "off", changes[0].OldValue)
	assert.Equal(t, "on", changes[0].NewValue)

	// on→off
	cfg.TLS = true
	probe.TLSRequired = false
	changes = CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "TLS", changes[0].Field)
	assert.True(t, changes[0].Critical)
	assert.Equal(t, "on", changes[0].OldValue)
	assert.Equal(t, "off", changes[0].NewValue)
}

func TestCompareServerParams_TLSEnabled_ViaKeyFields(t *testing.T) {
	// TLS enabled via cert/key fields (no TLS bool)
	cfg := baseConfig()
	cfg.TLSCert = "/path/to/cert.pem"
	probe := baseProbe()
	probe.TLSRequired = false
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "TLS", changes[0].Field)
	assert.True(t, changes[0].Critical)
}

func TestCompareServerParams_HWID_OffToOn_Critical(t *testing.T) {
	cfg := baseConfig()
	probe := baseProbe()
	probe.HWIDRequired = true
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "HWID", changes[0].Field)
	assert.True(t, changes[0].Critical)
}

func TestCompareServerParams_HWID_OnToOff_NonCritical(t *testing.T) {
	cfg := baseConfig()
	cfg.HWIDRequired = true
	probe := baseProbe()
	probe.HWIDRequired = false
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "HWID", changes[0].Field)
	assert.False(t, changes[0].Critical)
}

func TestCompareServerParams_Password_NoneToRequired_Critical(t *testing.T) {
	cfg := baseConfig()
	probe := baseProbe()
	probe.PasswordRequired = true
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "Password", changes[0].Field)
	assert.True(t, changes[0].Critical)
	assert.Equal(t, "none", changes[0].OldValue)
	assert.Equal(t, "required", changes[0].NewValue)
}

func TestCompareServerParams_Password_RequiredToNone_Critical(t *testing.T) {
	cfg := baseConfig()
	cfg.Password = "secret"
	probe := baseProbe()
	probe.PasswordRequired = false
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "Password", changes[0].Field)
	assert.True(t, changes[0].Critical)
	assert.Equal(t, "required", changes[0].OldValue)
	assert.Equal(t, "none", changes[0].NewValue)
}

func TestCompareServerParams_SampleRate_NonCritical(t *testing.T) {
	cfg := baseConfig()
	probe := baseProbe()
	probe.SampleRate = 44100
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "SampleRate", changes[0].Field)
	assert.False(t, changes[0].Critical)
	assert.Equal(t, "48000", changes[0].OldValue)
	assert.Equal(t, "44100", changes[0].NewValue)
}

func TestCompareServerParams_Channels_NonCritical(t *testing.T) {
	cfg := baseConfig()
	probe := baseProbe()
	probe.Channels = 2
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "Channels", changes[0].Field)
	assert.False(t, changes[0].Critical)
}

func TestCompareServerParams_OpusBitrate_NonCritical(t *testing.T) {
	cfg := baseConfig()
	probe := baseProbe()
	probe.OpusBitrate = 128000
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 1)
	assert.Equal(t, "OpusBitrate", changes[0].Field)
	assert.False(t, changes[0].Critical)
}

// TestCompareServerParams_MaxClients_FromDefault_NoChange verifies that a
// MaxClients change from the default value (1) to any other value is NOT
// reported. The first mDNS probe doesn't carry MaxClients so the client
// defaults to 1; reporting this as a change every connect is misleading.
func TestCompareServerParams_MaxClients_FromDefault_NoChange(t *testing.T) {
	cfg := baseConfig() // MaxClients = 1 (default)
	p := baseProbe()
	p.MaxClients = 2
	changes := CompareServerParams(cfg, p)
	for _, ch := range changes {
		assert.NotEqual(t, "MaxClients", ch.Field, "MaxClients change from default (1) should not be reported")
	}
}

// TestCompareServerParams_MaxClients_FromDefault_ToLarger_NoChange verifies
// default→N suppression for values larger than 2.
func TestCompareServerParams_MaxClients_FromDefault_ToLarger_NoChange(t *testing.T) {
	cfg := baseConfig()
	p := baseProbe()
	p.MaxClients = 10
	changes := CompareServerParams(cfg, p)
	for _, ch := range changes {
		assert.NotEqual(t, "MaxClients", ch.Field)
	}
}

// TestCompareServerParams_MaxClients_RealChange_Reported verifies that a
// MaxClients change from an explicit non-default value (e.g. 2 → 3) IS still
// reported as a non-critical change.
func TestCompareServerParams_MaxClients_RealChange_Reported(t *testing.T) {
	cfg := baseConfig()
	cfg.MaxClients = 2 // explicit, non-default
	p := baseProbe()
	p.MaxClients = 3
	changes := CompareServerParams(cfg, p)
	found := false
	for _, ch := range changes {
		if ch.Field == "MaxClients" {
			found = true
			assert.False(t, ch.Critical)
			assert.Equal(t, "2", ch.OldValue)
			assert.Equal(t, "3", ch.NewValue)
		}
	}
	assert.True(t, found, "MaxClients change from explicit non-default value should be reported")
}

// TestCompareServerParams_MaxClients_NonCritical verifies that a MaxClients
// change from a non-default old value is non-critical.
func TestCompareServerParams_MaxClients_NonCritical(t *testing.T) {
	cfg := baseConfig()
	cfg.MaxClients = 5
	p := baseProbe()
	p.MaxClients = 10
	changes := CompareServerParams(cfg, p)
	require.Len(t, changes, 1)
	assert.Equal(t, "MaxClients", changes[0].Field)
	assert.False(t, changes[0].Critical)
}

func TestCompareServerParams_MultipleMixed(t *testing.T) {
	cfg := baseConfig()
	probe := baseProbe()
	probe.Mode = "reverse"        // critical
	probe.SampleRate = 44100      // non-critical
	probe.PasswordRequired = true // critical
	changes := CompareServerParams(cfg, probe)
	require.Len(t, changes, 3)

	criticalCount := 0
	for _, c := range changes {
		if c.Critical {
			criticalCount++
		}
	}
	assert.Equal(t, 2, criticalCount)
}

func TestCompareServerParams_ConferenceMode(t *testing.T) {
	cfg := baseConfig()
	cfg.Conference = true
	probe := baseProbe()
	probe.Mode = "conference"
	changes := CompareServerParams(cfg, probe)
	assert.Empty(t, changes)
}

func TestCompareServerParams_EmptyProbeMode(t *testing.T) {
	// Empty probe mode defaults to "normal".
	cfg := baseConfig()
	probe := baseProbe()
	probe.Mode = ""
	changes := CompareServerParams(cfg, probe)
	assert.Empty(t, changes)
}
