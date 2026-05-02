package views

import (
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/virtualstate"
)

type managedVirtualAliasContext struct {
	presets           []recent.VirtualSinkPreset
	deviceNames       map[string]bool
	moduleBackedSinks map[string]bool
}

func (m SetupModel) managedVirtualAliasContext() managedVirtualAliasContext {
	return managedVirtualAliasContext{
		presets:           m.managedVirtualSinkPresets(),
		deviceNames:       m.currentDeviceListNames(),
		moduleBackedSinks: m.moduleBackedVirtualSinkNames(),
	}
}

func (m SetupModel) currentDeviceListNames() map[string]bool {
	names := make(map[string]bool)
	for _, item := range m.DeviceList.Items() {
		if device, ok := item.(interface{ FilterValue() string }); ok {
			names[device.FilterValue()] = true
		}
	}
	return names
}

func (m SetupModel) moduleBackedVirtualSinkNames() map[string]bool {
	names := make(map[string]bool)
	for sinkName, tracked := range m.trackedVirtualSinks {
		if tracked.Manageable && tracked.ModuleID != "" {
			names[sinkName] = true
		}
	}
	return appendStateModuleBackedVirtualSinkNames(names)
}

func appendStateModuleBackedVirtualSinkNames(names map[string]bool) map[string]bool {
	state, err := virtualstate.Load()
	if err != nil {
		return names
	}
	for _, device := range state.Devices {
		if stateDeviceClassifiesVirtual(device) && device.State.ModuleID != "" {
			names[device.SinkName] = true
		}
	}
	return names
}
