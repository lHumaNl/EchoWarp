package audio

import "strings"

// DetectVirtualDevice checks if a virtual audio driver is available.
// Returns the device name and whether it's installed.
func DetectVirtualDevice(dm DeviceEnumerator) (name string, found bool) {
	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return "", false
	}
	for _, d := range outputs {
		lower := strings.ToLower(d.Name)
		if strings.Contains(lower, "blackhole") ||
			strings.Contains(lower, "vb-audio") ||
			strings.Contains(lower, "cable") {
			return d.Name, true
		}
	}
	// Also check inputs (PulseAudio pipe sources appear as inputs)
	inputs, err := dm.ListInputDevices()
	if err != nil {
		return "", false
	}
	for _, d := range inputs {
		lower := strings.ToLower(d.Name)
		if strings.Contains(lower, "blackhole") ||
			strings.Contains(lower, "vb-audio") ||
			strings.Contains(lower, "cable") ||
			strings.Contains(lower, "echowarp") {
			return d.Name, true
		}
	}
	return "", false
}
