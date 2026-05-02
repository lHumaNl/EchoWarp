package views

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/virtualstate"
)

const defaultVirtualBaseName = "EchoWarp"

var unsafePulseAudioNameChars = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// CollectPresetDevices returns a DevicePreset containing all currently selected devices.
// It iterates over input and output device lists in order, including any device
// that has an entry in the multiSelect map (i.e. has at least one role assigned).
func (m *SetupModel) CollectPresetDevices() recent.DevicePreset {
	var devices []recent.PresetDevice

	for _, d := range m.inputDevices {
		if _, ok := m.multiSelect[d.selectKey()]; ok {
			pd := recent.PresetDevice{
				ID:      d.ID,
				Name:    d.Name,
				IsInput: true,
				Virtual: d.IsVirtual,
				Volume:  d.Volume,
				AGC:     d.AGC,
			}
			if d.IsVirtual {
				pd.VirtualSink = m.virtualSinkPresetForDevice(d)
			}
			devices = append(devices, pd)
		}
	}

	for _, d := range m.outputDevices {
		if _, ok := m.multiSelect[d.selectKey()]; ok {
			pd := recent.PresetDevice{
				ID:      d.ID,
				Name:    d.Name,
				IsInput: false,
				Virtual: d.IsVirtual,
				Volume:  d.Volume,
				AGC:     d.AGC,
			}
			// Save virtual sink preset for virtual output devices.
			if d.IsVirtual {
				pd.VirtualSink = m.virtualSinkPresetForDevice(d)
			}
			// Save mix input for virtual output devices.
			if d.IsVirtual {
				if mixSet, ok := m.mixInputs[d.selectKey()]; ok {
					for _, inp := range m.inputDevices {
						if mixSet[inp.selectKey()] {
							id := inp.ID
							pd.MixInputID = &id
							pd.MixInputName = inp.Name
							break
						}
					}
				}
			}
			devices = append(devices, pd)
		}
	}

	return recent.DevicePreset{Devices: devices, VirtualSinks: m.CollectVirtualSinkPresets()}
}

// CollectVirtualSinkPresets returns app-managed virtual sink lifecycle settings.
// It is independent from device selection so unchecking EchoWarp does not drop
// Delete/Recreate lifecycle state from persisted presets.
func (m SetupModel) CollectVirtualSinkPresets() []recent.VirtualSinkPreset {
	if len(m.trackedVirtualSinks) > 0 {
		result := make([]recent.VirtualSinkPreset, 0, len(m.trackedVirtualSinks))
		for _, sinkName := range m.sortedTrackedVirtualSinkNames() {
			result = append(result, m.trackedVirtualSinks[sinkName].Preset)
		}
		return result
	}
	if !m.shouldPersistVirtualSinkLifecycle() {
		return nil
	}
	return []recent.VirtualSinkPreset{*m.defaultVirtualSinkPreset()}
}

func (m SetupModel) shouldPersistVirtualSinkLifecycle() bool {
	return m.virtualSinkLifecycleConfigured || m.virtualMicCreated
}

func isEchoWarpMonitorDevice(d deviceRow) bool {
	return d.IsVirtual && d.IsInput && d.Name == echowarpMonitorName
}

func (m SetupModel) virtualSinkPresetForDevice(d deviceRow) *recent.VirtualSinkPreset {
	for _, sinkName := range m.sortedTrackedVirtualSinkNames() {
		vs := m.trackedVirtualSinks[sinkName].Preset
		if d.IsInput && d.Name == virtualSinkCaptureName(vs) {
			return &vs
		}
		if !d.IsInput && d.Name == virtualSinkPlaybackName(vs) {
			return &vs
		}
	}
	if isEchoWarpMonitorDevice(d) || (!d.IsInput && d.Name == echowarpSinkName) {
		return m.defaultVirtualSinkPreset()
	}
	return nil
}

func (m SetupModel) defaultVirtualSinkPreset() *recent.VirtualSinkPreset {
	onStop := m.virtualSinkOnStop
	if onStop == "" {
		onStop = recent.SinkDelete
	}
	onStart := m.virtualSinkOnStart
	if onStart == "" {
		onStart = recent.SinkRecreate
	}
	return virtualSinkPresetWithLifecycle(onStop, onStart)
}

func (m SetupModel) virtualSinkPresetForBaseName(baseName string) recent.VirtualSinkPreset {
	baseName = normalizeVirtualBaseName(baseName)
	id := virtualstate.NewVirtualDeviceID()
	sinkName := safePulseAudioSinkName(baseName, id)
	return recent.VirtualSinkPreset{
		ID: id, BaseName: baseName, ModuleType: "module-null-sink",
		SinkName: sinkName, MonitorName: sinkName + ".monitor",
		PlaybackName: "Playback " + baseName, CaptureName: "Capture " + baseName,
		OnStop: recent.SinkDelete, OnStart: recent.SinkRecreate,
	}
}

func normalizeVirtualBaseName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultVirtualBaseName
	}
	return strings.Join(strings.Fields(name), " ")
}

func safePulseAudioSinkName(baseName, id string) string {
	safeBase := unsafePulseAudioNameChars.ReplaceAllString(baseName, "_")
	safeBase = strings.Trim(safeBase, "_.-")
	if safeBase == "" || !isPulseAudioNameStart(rune(safeBase[0])) {
		safeBase = "device_" + safeBase
	}
	return "echowarp_" + safeBase + "_" + idSuffix(id)
}

func isPulseAudioNameStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func idSuffix(id string) string {
	parts := strings.Split(id, "-")
	return parts[len(parts)-1]
}

func defaultVirtualSinkPreset() *recent.VirtualSinkPreset {
	return virtualSinkPresetWithLifecycle(recent.SinkDelete, recent.SinkRecreate)
}

func virtualSinkPresetWithLifecycle(onStop, onStart recent.SinkLifecycle) *recent.VirtualSinkPreset {
	return &recent.VirtualSinkPreset{
		ModuleType: "module-null-sink",
		SinkName:   echowarpSinkName,
		OnStop:     onStop,
		OnStart:    onStart,
	}
}

func virtualSinkMonitorName(vs recent.VirtualSinkPreset) string {
	if vs.MonitorName != "" {
		return vs.MonitorName
	}
	if vs.BaseName != "" || vs.PlaybackName != "" || vs.CaptureName != "" {
		return vs.SinkName + ".monitor"
	}
	return "Monitor of " + vs.SinkName
}

func virtualSinkPlaybackName(vs recent.VirtualSinkPreset) string {
	if vs.PlaybackName != "" {
		return vs.PlaybackName
	}
	if vs.BaseName != "" {
		return "Playback " + vs.BaseName
	}
	return vs.SinkName
}

func virtualSinkCaptureName(vs recent.VirtualSinkPreset) string {
	if vs.CaptureName != "" {
		return vs.CaptureName
	}
	if vs.BaseName != "" {
		return "Capture " + vs.BaseName
	}
	return "Monitor of " + vs.SinkName
}

// CollectModePreset returns a full ModePreset snapshot for the given mode,
// including both device selection and server-side settings (port, password,
// max_clients, tls*). The mode parameter documents the target mode; default-
// omission happens later at Save time via preset.DefaultsFor(mode).
func (m *SetupModel) CollectModePreset(mode string) preset.ModePreset {
	_ = mode // mode is part of the API surface; default-omission happens in preset.Save.
	devices := m.CollectPresetDevices().Devices
	mp := preset.ModePreset{Devices: devices, VirtualSinks: m.CollectVirtualSinkPresets()}

	for _, f := range m.Fields {
		switch f.Key {
		case "port":
			mp.Port = f.IntValue()
		case "password":
			mp.Password = f.Value
		case "max_clients":
			mp.MaxClients = f.IntValue()
		}
	}
	for _, f := range m.AdvancedFields {
		switch f.Key {
		case "tls":
			mp.TLS = f.Value == "on"
		case "tls_cert":
			mp.TLSCert = f.Value
		case "tls_key":
			mp.TLSKey = f.Value
		case "log_level":
			mp.LogLevel = f.Value
		}
	}
	return mp
}
