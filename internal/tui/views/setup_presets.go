package views

import (
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

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
			if isEchoWarpMonitorDevice(d) {
				pd.VirtualSink = m.defaultVirtualSinkPreset()
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
				pd.VirtualSink = m.defaultVirtualSinkPreset()
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

	return recent.DevicePreset{Devices: devices}
}

func isEchoWarpMonitorDevice(d deviceRow) bool {
	return d.IsVirtual && d.IsInput && d.Name == echowarpMonitorName
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

// CollectModePreset returns a full ModePreset snapshot for the given mode,
// including both device selection and server-side settings (port, password,
// max_clients, tls*). The mode parameter documents the target mode; default-
// omission happens later at Save time via preset.DefaultsFor(mode).
func (m *SetupModel) CollectModePreset(mode string) preset.ModePreset {
	_ = mode // mode is part of the API surface; default-omission happens in preset.Save.
	devices := m.CollectPresetDevices().Devices
	mp := preset.ModePreset{Devices: devices}

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
