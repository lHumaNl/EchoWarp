package views

import (
	"strings"

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
			devices = append(devices, recent.PresetDevice{
				ID:      d.ID,
				Name:    d.Name,
				IsInput: true,
				Virtual: d.IsVirtual,
			})
		}
	}

	for _, d := range m.outputDevices {
		if _, ok := m.multiSelect[d.selectKey()]; ok {
			pd := recent.PresetDevice{
				ID:      d.ID,
				Name:    d.Name,
				IsInput: false,
				Virtual: d.IsVirtual,
			}
			// Save virtual sink preset for virtual output devices.
			if d.IsVirtual {
				onStop := m.virtualSinkOnStop
				if onStop == "" {
					onStop = recent.SinkDelete
				}
				onStart := m.virtualSinkOnStart
				if onStart == "" {
					onStart = recent.SinkRecreate
				}
				pd.VirtualSink = &recent.VirtualSinkPreset{
					ModuleType: "module-null-sink",
					SinkName:   echowarpSinkName,
					OnStop:     onStop,
					OnStart:    onStart,
				}
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

// CollectServerSettings returns the current server configuration from the TUI fields.
func (m *SetupModel) CollectServerSettings() preset.ServerSettings {
	var s preset.ServerSettings
	for _, f := range m.Fields {
		switch f.Label {
		case "Mode":
			s.LastMode = strings.SplitN(f.Value, " ", 2)[0]
		case "Port":
			s.Port = f.IntValue()
		case "Password":
			s.Password = f.Value
		case "Max clients":
			s.MaxClients = f.IntValue()
		}
	}
	for _, f := range m.AdvancedFields {
		switch f.Label {
		case "TLS":
			s.TLS = f.Value == "on"
		case "TLS Cert":
			s.TLSCert = f.Value
		case "TLS Key":
			s.TLSKey = f.Value
		}
	}
	return s
}
