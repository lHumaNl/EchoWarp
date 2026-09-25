package startup

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/recent"
)

const maxPort = 65535

// historyMu serializes this package's writers only, not legacy recent.Save callers.
var historyMu sync.Mutex

// LatestRecent selects the newest valid, nonzero successful-connection timestamp.
// Missing and corrupt history both produce an error because recent.Load conflates them.
func LatestRecent() (recent.Server, error) {
	historyMu.Lock()
	defer historyMu.Unlock()
	servers, err := recent.Load()
	if err != nil {
		return recent.Server{}, fmt.Errorf("load recent servers: %w", err)
	}
	var latest recent.Server
	for _, server := range servers {
		if validEndpoint(server.Address, server.Port) && !server.LastConnected.IsZero() &&
			(latest.LastConnected.IsZero() || server.LastConnected.After(latest.LastConnected)) {
			latest = server
		}
	}
	if latest.LastConnected.IsZero() {
		return latest, errors.New("no valid recent connection found; history is empty or unreadable")
	}
	return latest, nil
}

// SaveSuccessful records a confirmed client connection, never an attempted startup.
// Callers must invoke it only after connection success. No credentials are persisted.
func SaveSuccessful(cfg config.Config, info *probe.ProbeServerResult) error {
	if cfg.Mode != config.ModeClient || !validEndpoint(cfg.Address, cfg.Port) {
		return nil
	}
	historyMu.Lock()
	defer historyMu.Unlock()
	servers, err := recent.Load()
	if err != nil {
		return fmt.Errorf("load recent servers: %w", err)
	}
	server := successfulServer(cfg, info)
	cfg.StreamMode = config.AudioMode(server.LastMode)
	cfg.SyncFromStreamMode()
	mergeHistory(&server, servers, PresetFromConfig(cfg))
	if err = recent.Save(recent.Add(servers, server)); err != nil {
		return fmt.Errorf("save successful connection: %w", err)
	}
	return nil
}

func validEndpoint(address string, port int) bool {
	return strings.TrimSpace(address) != "" && port > 0 && port <= maxPort
}

func successfulServer(cfg config.Config, info *probe.ProbeServerResult) recent.Server {
	server := recent.Server{
		Address: cfg.Address, Port: cfg.Port, LastConnected: time.Now(),
		LastMode: cfg.AudioMode(), LogLevel: cfg.LogLevel,
	}
	if info != nil {
		server.ServerID, server.Hostname = info.ServerID, info.ServerName
		server.LastMode = effectiveProbeMode(info)
	}
	return server
}

func mergeHistory(server *recent.Server, servers []recent.Server, preset recent.DevicePreset) {
	server.Presets = make(map[string]recent.DevicePreset)
	for _, existing := range servers {
		if !recent.MatchesServer(existing, server.Address, server.Port, server.ServerID) {
			continue
		}
		for mode, previous := range existing.Presets {
			server.Presets[mode] = previous
		}
		if server.Hostname == "" {
			server.Hostname = existing.Hostname
		}
		break
	}
	preserveVirtualMetadata(&preset, server.Presets[server.LastMode])
	server.Presets[server.LastMode] = preset
}

func preserveVirtualMetadata(preset *recent.DevicePreset, previous recent.DevicePreset) {
	// Config has no virtual-sink lifecycle fields; absence must not erase TUI metadata.
	preset.VirtualSinks = previous.VirtualSinks
	for i := range preset.Devices {
		device := &preset.Devices[i]
		for _, old := range previous.Devices {
			if device.Name != "" && device.Name == old.Name && device.IsInput == old.IsInput {
				device.Virtual, device.VirtualSink = old.Virtual, old.VirtualSink
				break
			}
		}
	}
}
