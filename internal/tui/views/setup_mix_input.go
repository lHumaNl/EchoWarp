package views

import "github.com/lHumaNl/echowarp/internal/recent"

// outputRow represents a single navigable row in the Output device section.
// It can be either an output device or a mix-input sub-item under a virtual output.
type outputRow struct {
	device      deviceRow // the device (output device or input device for mix)
	isMixItem   bool      // true if this is a mix sub-item (input device)
	parentKey   string    // selectKey of the parent virtual output (only when isMixItem)
	mixSelected bool      // true if this mix input is enabled
}

// buildOutputRows constructs the flat list of output rows including mix sub-items.
// For each selected virtual output device, eligible input devices are shown as sub-items.
func (m SetupModel) buildOutputRows() []outputRow {
	var rows []outputRow
	for _, dev := range m.outputDevices {
		rows = append(rows, outputRow{device: dev})

		// If this is a selected virtual output, show mix input sub-items
		if !dev.IsVirtual {
			continue
		}
		roles := m.multiSelect[dev.selectKey()]
		if !roles.Playback {
			continue
		}

		parentKey := dev.selectKey()
		mixSet := m.mixInputs[parentKey]
		for _, inp := range m.inputDevices {
			// Skip virtual/loopback inputs — only real microphones make sense for mix.
			if inp.IsVirtual || inp.IsLoopback {
				continue
			}
			selected := mixSet != nil && mixSet[inp.selectKey()]
			rows = append(rows, outputRow{
				device:      inp,
				isMixItem:   true,
				parentKey:   parentKey,
				mixSelected: selected,
			})
		}
	}
	return rows
}

// toggleMixInput toggles a mix input sub-item in the mixInputs map.
func (m *SetupModel) toggleMixInput(parentKey, inputKey string) {
	if m.mixInputs == nil {
		m.mixInputs = make(map[string]map[string]bool)
	}
	if m.mixInputs[parentKey] == nil {
		m.mixInputs[parentKey] = make(map[string]bool)
	}
	if m.mixInputs[parentKey][inputKey] {
		delete(m.mixInputs[parentKey], inputKey)
		if len(m.mixInputs[parentKey]) == 0 {
			delete(m.mixInputs, parentKey)
		}
	} else {
		m.mixInputs[parentKey][inputKey] = true
	}
}

// restoreMixInputsFromPreset restores mixInputs map from preset devices that have MixInputID set.
func (m *SetupModel) restoreMixInputsFromPreset(preset recent.DevicePreset, matched []deviceRow) {
	for _, pd := range preset.Devices {
		if pd.IsInput || pd.MixInputID == nil {
			continue
		}
		// Find the matched output device row for this preset device.
		var outputKey string
		for _, d := range matched {
			if !d.IsInput && (d.ID == pd.ID || d.Name == pd.Name) {
				outputKey = d.selectKey()
				break
			}
		}
		if outputKey == "" {
			continue
		}
		m.restoreMixInput(outputKey, *pd.MixInputID, pd.MixInputName)
	}
}

// cleanupMixInputs removes mix entries for output devices that are no longer selected.
func (m *SetupModel) cleanupMixInputs() {
	for outKey := range m.mixInputs {
		roles, ok := m.multiSelect[outKey]
		if !ok || !roles.Playback {
			delete(m.mixInputs, outKey)
		}
	}
}
