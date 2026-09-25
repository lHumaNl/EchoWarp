package startup

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

func TestResolveDiscoversOnlyUnambiguousTarget(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Mode = config.ModeClient
			cfg.Devices = []config.DeviceEntry{{ID: 0, Role: config.RolePlayback, Volume: 1}}
			probes := 0
			network := Network{
				Discover: func(context.Context, time.Duration) ([]discovery.ServiceInfo, error) {
					var found []discovery.ServiceInfo
					for i := range count {
						found = append(found, discovery.ServiceInfo{Port: 4500, AddrIPv4: []net.IP{net.IPv4(127, 0, 0, byte(i+1))}})
					}
					return found, nil
				},
				Probe: func(_ context.Context, address string, port int) (*probe.ProbeServerResult, error) {
					probes++
					require.Equal(t, "127.0.0.1", address)
					require.Equal(t, 4500, port)
					return &probe.ProbeServerResult{Mode: "normal"}, nil
				},
			}
			prepared, _, err := Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 0, Name: "Speakers"}}, Intent{Discover: true}, network)
			if count == 1 {
				require.NoError(t, err)
				require.Equal(t, uint32(0), *prepared.DeviceID)
				require.Equal(t, 1, probes)
			} else {
				require.Error(t, err)
				require.Zero(t, probes)
			}
		})
	}
}

func TestDeviceNameSelectorsAreUniqueCaseInsensitiveSubstrings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "host"
	network := Network{Probe: func(context.Context, string, int) (*probe.ProbeServerResult, error) {
		return &probe.ProbeServerResult{Mode: "normal"}, nil
	}}
	for _, selector := range []string{"usb", "USB AUDIO", "Speakers"} {
		prepared, _, err := Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 3, Name: "USB Audio Speakers"}}, Intent{DeviceName: selector}, network)
		require.NoError(t, err)
		require.Equal(t, uint32(3), *prepared.DeviceID)
	}
	_, _, err := Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 1, Name: "USB A"}, {ID: 2, Name: "USB B"}}, Intent{DeviceName: "usb"}, network)
	require.ErrorContains(t, err, "found 2")
	_, _, err = Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 1, Name: "USB A"}}, Intent{DeviceName: "usb", ExplicitFlags: []string{"playback-device"}}, network)
	require.ErrorContains(t, err, "conflicts")
}

func TestResolveExplicitCLIOptionsBeatModeProfile(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "host"
	cfg.Devices = []config.DeviceEntry{{ID: 0, Role: config.RolePlayback, Volume: 1}}
	cfg.AEC = false
	cfg.LogLevel = "warn"
	cfg.TLSInsecure = false
	cfg.ClientModes = map[string]interface{}{"normal": map[string]interface{}{"aec": true, "logging": map[string]interface{}{"level": "debug"}}}
	network := Network{Probe: func(context.Context, string, int) (*probe.ProbeServerResult, error) {
		return &probe.ProbeServerResult{Mode: "normal", TLSRequired: true, TLSSelfSigned: true}, nil
	}}
	prepared, _, err := Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 0, Name: "Speakers"}}, Intent{ExplicitFlags: []string{"aec", "log-level", "tls-insecure"}}, network)
	require.NoError(t, err)
	require.False(t, prepared.AEC)
	require.False(t, prepared.TLSInsecure)
	require.Equal(t, "warn", prepared.LogLevel)
}

func TestResolveNoPasswordAndCancellationNeverStart(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "host"
	network := Network{Probe: func(context.Context, string, int) (*probe.ProbeServerResult, error) {
		return &probe.ProbeServerResult{PasswordRequired: true}, nil
	}}
	_, _, err := Resolve(t.Context(), cfg, nil, Intent{}, network)
	require.ErrorContains(t, err, "password")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err = Resolve(ctx, cfg, nil, Intent{}, Network{})
	require.ErrorIs(t, err, context.Canceled)
}
