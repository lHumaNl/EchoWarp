package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// runtimeDevState captures the runtime values we persist back into presets.
type runtimeDevState struct {
	volume float64
	agc    bool
}

// persistRuntimeDeviceStateCmd saves the current runtime device Volume and AGC
// values back to the user's preset (client recent or server preset), so that
// the next launch restores what the user tuned during streaming instead of the
// stale setup-screen snapshot.
//
// Only Volume and AGC are updated — other preset fields (mix inputs, virtual
// sinks, device identity, port/TLS for server) remain untouched.
//
// Matching is by device ID. Devices in the preset that are not present in
// deviceStates are left as-is.
func persistRuntimeDeviceStateCmd(cfgSnapshot config.Config, deviceStates []DeviceState) tea.Cmd {
	if len(deviceStates) == 0 {
		return nil
	}
	byID := make(map[uint32]runtimeDevState, len(deviceStates))
	for _, ds := range deviceStates {
		byID[ds.ID] = runtimeDevState{volume: ds.Volume, agc: ds.AGC}
	}

	return func() tea.Msg {
		switch cfgSnapshot.Mode {
		case config.ModeClient:
			persistClientRuntime(cfgSnapshot, byID)
		case config.ModeServer:
			persistServerRuntime(cfgSnapshot, byID)
		}
		return nil
	}
}

func persistClientRuntime(cfg config.Config, byID map[uint32]runtimeDevState) {
	servers, err := recent.Load()
	if err != nil {
		return
	}
	mode := cfg.AudioMode()
	changed := false
	for si := range servers {
		s := &servers[si]
		if s.Address != cfg.Address || s.Port != cfg.Port {
			continue
		}
		mp, ok := s.Presets[mode]
		if !ok {
			return
		}
		for di := range mp.Devices {
			if st, ok := byID[mp.Devices[di].ID]; ok {
				if mp.Devices[di].Volume != st.volume || mp.Devices[di].AGC != st.agc {
					mp.Devices[di].Volume = st.volume
					mp.Devices[di].AGC = st.agc
					changed = true
				}
			}
		}
		s.Presets[mode] = mp
		break
	}
	if changed {
		_ = recent.Save(servers) //nolint:errcheck // best-effort persist
	}
}

func persistServerRuntime(cfg config.Config, byID map[uint32]runtimeDevState) {
	sp := preset.Load()
	mode := cfg.AudioMode()
	mpPtr := sp.Get(mode)
	if mpPtr == nil {
		return
	}
	mp := *mpPtr
	changed := false
	for di := range mp.Devices {
		if st, ok := byID[mp.Devices[di].ID]; ok {
			if mp.Devices[di].Volume != st.volume || mp.Devices[di].AGC != st.agc {
				mp.Devices[di].Volume = st.volume
				mp.Devices[di].AGC = st.agc
				changed = true
			}
		}
	}
	if changed {
		sp.Set(mode, mp)
		_ = preset.Save(sp) //nolint:errcheck // best-effort persist
	}
}
