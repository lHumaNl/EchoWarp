package startup

import (
	"slices"

	"github.com/lHumaNl/echowarp/internal/config"
)

const defaultVolume = 1.0

func cloneConfig(cfg config.Config) config.Config {
	cfg.Devices = slices.Clone(cfg.Devices)
	for i := range cfg.Devices {
		cfg.Devices[i].MixInputID = copyID(cfg.Devices[i].MixInputID)
	}
	cfg.DeviceID = copyID(cfg.DeviceID)
	cfg.InputDeviceID = copyID(cfg.InputDeviceID)
	cfg.OutputDeviceID = copyID(cfg.OutputDeviceID)
	cfg.STUNServers = slices.Clone(cfg.STUNServers)
	cfg.TURNServers = slices.Clone(cfg.TURNServers)
	cfg.TrustedProxies = slices.Clone(cfg.TrustedProxies)
	// ClientModes is read-only throughout preparation; its raw YAML is never edited.
	return cfg
}

func copyID(id *uint32) *uint32 {
	if id == nil {
		return nil
	}
	value := *id
	return &value
}
