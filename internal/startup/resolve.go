package startup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

// Network allows readiness tests without opening sockets or audio devices.
type Network struct {
	Probe    func(context.Context, string, int) (*probe.ProbeServerResult, error)
	Discover func(context.Context, time.Duration) ([]discovery.ServiceInfo, error)
}

func DefaultNetwork() Network {
	return Network{Probe: probe.ProbeServerWithContext, Discover: discoverServers}
}

func discoverServers(ctx context.Context, timeout time.Duration) ([]discovery.ServiceInfo, error) {
	stream, err := discovery.NewDiscoverer().Discover(ctx, timeout)
	if err != nil {
		return nil, err
	}
	var servers []discovery.ServiceInfo
	for server := range stream {
		servers = append(servers, server)
	}
	return servers, nil
}

// Resolve performs network readiness, then pure device validation. It never
// opens a capture/playback device or mutates persisted history.
func Resolve(ctx context.Context, cfg config.Config, inventory []audio.AudioDevice, intent Intent, network Network) (config.Config, *probe.ProbeServerResult, error) {
	original := cfg
	if intent.Problem != "" {
		return cfg, nil, fmt.Errorf("%s", intent.Problem)
	}
	if err := ctx.Err(); err != nil {
		return cfg, nil, err
	}
	if err := ValidateServerTLS(cfg); err != nil {
		return cfg, nil, err
	}
	var info *probe.ProbeServerResult
	if cfg.Mode == config.ModeClient {
		if cfg.Address == "" {
			var err error
			cfg, err = discoverTarget(ctx, cfg, intent, network)
			if err != nil {
				return cfg, nil, err
			}
		}
		var err error
		info, err = network.Probe(ctx, cfg.Address, cfg.Port)
		if err != nil {
			return cfg, nil, fmt.Errorf("server not reachable: %w", err)
		}
		if err := prepareProbe(&cfg, intent.Recent, info); err != nil {
			return cfg, info, err
		}
	}
	var err error
	cfg, err = overrideLegacyDevices(cfg, inventory, intent.ExplicitFlags)
	if err != nil {
		return cfg, info, err
	}
	if intent.DeviceName != "" {
		cfg, err = applyNamedSelection(cfg, inventory, intent)
		if err != nil {
			return cfg, info, err
		}
	}
	applyExplicitOptions(&cfg, original, intent.ExplicitFlags)
	prepared, err := prepareWithOverrides(cfg, inventory, intent.Recent, info, intent.ExplicitFlags)
	return prepared, info, err
}

func applyNamedSelection(cfg config.Config, inventory []audio.AudioDevice, intent Intent) (config.Config, error) {
	role := primaryRole(cfg)
	if cfg.Mode == config.ModeServer && !cfg.Reverse {
		role = config.RoleCapture
	}
	for _, flag := range intent.ExplicitFlags {
		if flag == "device" || flag == "capture-device" && role == config.RoleCapture || flag == "playback-device" && role == config.RolePlayback {
			return cfg, fmt.Errorf("--device-name conflicts with the explicit %s selector", flag)
		}
	}
	var matches []audio.AudioDevice
	for _, device := range inventory {
		if device.IsInput == (role == config.RoleCapture) && strings.Contains(strings.ToLower(device.Name), strings.ToLower(intent.DeviceName)) {
			matches = append(matches, device)
		}
	}
	if len(matches) != 1 {
		return cfg, fmt.Errorf("device name %q: expected one %s match, found %d", intent.DeviceName, role, len(matches))
	}
	device := matches[0]
	roles, err := requiredRoles(cfg)
	if err != nil {
		return cfg, err
	}
	kept := make([]config.DeviceEntry, 0, len(cfg.Devices)+1)
	for _, entry := range cfg.Devices {
		effectiveRole, err := entryRole(entry, inventory, roles)
		if err != nil {
			return cfg, fmt.Errorf("cannot safely replace named device selection: %w", err)
		}
		if effectiveRole != role {
			kept = append(kept, entry)
		}
	}
	kept = append(kept, config.DeviceEntry{ID: device.ID, Name: device.Name, Role: role, Type: deviceType(role), Volume: 1})
	cfg.Devices = kept
	if role == config.RoleCapture {
		cfg.InputDeviceID = &device.ID
	} else {
		cfg.OutputDeviceID = &device.ID
	}
	cfg.DeviceID = nil // Explicit role entries now own selection; avoid a stale legacy fallback.
	return cfg, nil
}

func discoverTarget(ctx context.Context, cfg config.Config, intent Intent, network Network) (config.Config, error) {
	if !intent.Discover {
		return cfg, fmt.Errorf("server address is required; use --address or --recent")
	}
	timeout := intent.DiscoveryTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	servers, err := network.Discover(ctx, timeout)
	if err != nil {
		return cfg, fmt.Errorf("server discovery failed: %w", err)
	}
	targets := make(map[string]discovery.ServiceInfo)
	for _, server := range servers {
		if len(server.AddrIPv4) > 0 {
			targets[fmt.Sprintf("%s:%d", server.AddrIPv4[0], server.Port)] = server
		}
	}
	if len(targets) != 1 {
		return cfg, fmt.Errorf("found %d servers; select a server or use --address", len(targets))
	}
	for _, server := range targets {
		cfg.Address, cfg.Port = server.AddrIPv4[0].String(), server.Port
	}
	return cfg, nil
}
