//go:build linux

package audio

// DetectBlackHole returns nil on Linux — BlackHole is a macOS-only concept.
// PulseAudio monitor sources serve the same purpose and are discovered automatically.
func DetectBlackHole(_ DeviceEnumerator) []AudioDevice { return nil }

// ListLoopbackDevices on Linux: for each output device, the corresponding
// PulseAudio/PipeWire monitor source can be used for loopback capture.
// The monitor source has the same name as the sink with ".monitor" appended.
func ListLoopbackDevices(dm DeviceEnumerator) ([]LoopbackDevice, error) {
	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, err
	}
	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil, err
	}

	// Build input device lookup by name for monitor matching.
	inputByName := make(map[string]AudioDevice, len(inputs))
	for _, d := range inputs {
		inputByName[d.Name] = d
	}

	var devices []LoopbackDevice
	for _, out := range outputs {
		// PulseAudio monitor sources appear as input devices with ".monitor" suffix.
		monitorName := out.Name + ".monitor"
		if monitor, ok := inputByName[monitorName]; ok {
			devices = append(devices, LoopbackDevice{
				OutputDevice: out,
				BlackHole:    monitor, // On Linux the monitor source acts as the capture device.
				Channels:     out.Channels,
			})
		}
	}
	return devices, nil
}

// CleanupOrphanedAggregateDevices is a no-op on Linux — aggregate devices are a macOS concept.
func CleanupOrphanedAggregateDevices() {}

// linuxLoopbackSession holds the monitor source device ID used for capture.
type linuxLoopbackSession struct {
	captureDeviceID uint32
}

// NewLoopbackSession on Linux finds the monitor source for the given output device
// and returns a session that captures from it.
func NewLoopbackSession(outputDeviceName, blackholeInputDeviceName string) (LoopbackSession, error) {
	dm, err := NewDeviceManager()
	if err != nil {
		return nil, err
	}
	defer func() { _ = dm.Close() }() //nolint:errcheck

	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil, err
	}

	// The "blackholeInputDeviceName" on Linux is the monitor source name
	// (e.g. "alsa_output.xxx.monitor"). Look it up directly.
	for _, inp := range inputs {
		if inp.Name == blackholeInputDeviceName {
			return &linuxLoopbackSession{captureDeviceID: inp.ID}, nil
		}
	}

	// Fallback: try to find "<outputDeviceName>.monitor".
	monitorName := outputDeviceName + ".monitor"
	for _, inp := range inputs {
		if inp.Name == monitorName {
			return &linuxLoopbackSession{captureDeviceID: inp.ID}, nil
		}
	}

	return nil, &noMonitorError{outputDeviceName: outputDeviceName}
}

// noMonitorError is returned when no PulseAudio monitor source is found.
type noMonitorError struct {
	outputDeviceName string
}

func (e *noMonitorError) Error() string {
	return "no PulseAudio monitor source found for output device: " + e.outputDeviceName
}

func (s *linuxLoopbackSession) CaptureDeviceID() uint32 { return s.captureDeviceID }

func (s *linuxLoopbackSession) IsNativeLoopback() bool { return false }

func (s *linuxLoopbackSession) Close() error { return nil }
