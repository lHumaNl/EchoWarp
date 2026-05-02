package views

import "github.com/lHumaNl/echowarp/internal/recent"

type managedVirtualAliasContext struct {
	presets           []recent.VirtualSinkPreset
	deviceNames       map[string]bool
	deviceNameCounts  map[string]int
	moduleBackedSinks map[string]bool
}

func (m SetupModel) managedVirtualAliasContext() managedVirtualAliasContext {
	return managedVirtualAliasContext{
		presets:           m.managedVirtualSinkPresets(),
		deviceNames:       m.currentDeviceListNames(),
		deviceNameCounts:  m.currentDeviceListNameCounts(),
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

func (m SetupModel) currentDeviceListNameCounts() map[string]int {
	counts := make(map[string]int)
	for _, item := range m.DeviceList.Items() {
		if device, ok := item.(interface{ FilterValue() string }); ok {
			counts[device.FilterValue()]++
		}
	}
	return counts
}

func (m SetupModel) moduleBackedVirtualSinkNames() map[string]bool {
	names := make(map[string]bool)
	for sinkName, tracked := range m.trackedVirtualSinks {
		if tracked.isLiveModuleBacked() {
			names[sinkName] = true
		}
	}
	return names
}
