//go:build darwin

package audio

import (
	"fmt"
	"strings"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio/coreaudio"
)

// DetectBlackHole searches for installed BlackHole audio devices among input devices.
func DetectBlackHole(dm DeviceEnumerator) []AudioDevice {
	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil
	}
	var result []AudioDevice
	for _, d := range inputs {
		if strings.Contains(strings.ToLower(d.Name), "blackhole") {
			result = append(result, d)
		}
	}
	return result
}

// ListLoopbackDevices returns output devices available for loopback capture via BlackHole.
func ListLoopbackDevices(dm DeviceEnumerator) ([]LoopbackDevice, error) {
	blackholes := DetectBlackHole(dm)
	if len(blackholes) == 0 {
		return nil, nil
	}

	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, err
	}

	var result []LoopbackDevice
	for _, out := range outputs {
		if strings.Contains(strings.ToLower(out.Name), "blackhole") {
			continue
		}
		bh := bestBlackHole(blackholes, out.Channels)
		if bh == nil {
			continue
		}
		channels := out.Channels
		if bh.Channels < channels {
			channels = bh.Channels
		}
		result = append(result, LoopbackDevice{
			OutputDevice: out,
			BlackHole:    *bh,
			Channels:     channels,
		})
	}
	return result, nil
}

func bestBlackHole(blackholes []AudioDevice, maxCh uint32) *AudioDevice {
	var best *AudioDevice
	for i := range blackholes {
		bh := &blackholes[i]
		if bh.Channels <= maxCh {
			if best == nil || bh.Channels > best.Channels {
				best = bh
			}
		}
	}
	if best != nil {
		return best
	}
	for i := range blackholes {
		bh := &blackholes[i]
		if best == nil || bh.Channels < best.Channels {
			best = bh
		}
	}
	return best
}

// CleanupOrphanedAggregateDevices removes stale EchoWarp loopback aggregate devices.
func CleanupOrphanedAggregateDevices() {
	coreaudio.CleanupOrphaned()
}

// darwinLoopbackSession manages a CoreAudio aggregate device for loopback capture.
type darwinLoopbackSession struct {
	aggregateDeviceID uint32
	prevDefaultOutput uint32
	captureDeviceID   uint32
}

// NewLoopbackSession creates a CoreAudio Aggregate Device combining the output device
// and BlackHole, sets it as the system default output, and returns a session.
func NewLoopbackSession(outputDeviceName, blackholeInputDeviceName string) (LoopbackSession, error) {
	caDevices := coreaudio.ListDevices()

	outputCA := coreaudio.FindDeviceByName(caDevices, outputDeviceName)
	if outputCA == nil {
		return nil, fmt.Errorf("CoreAudio output device %q not found", outputDeviceName)
	}

	bhCA := coreaudio.FindDeviceByName(caDevices, blackholeInputDeviceName)
	if bhCA == nil {
		return nil, fmt.Errorf("CoreAudio BlackHole device %q not found", blackholeInputDeviceName)
	}

	// Find BlackHole input device ID in malgo
	dm, err := NewDeviceManager()
	if err != nil {
		return nil, fmt.Errorf("init device manager for loopback: %w", err)
	}
	defer func() { _ = dm.Close() }() //nolint:errcheck

	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil, fmt.Errorf("list input devices: %w", err)
	}

	bhInputID, found := findBlackHoleInput(inputs, blackholeInputDeviceName)
	if !found {
		return nil, fmt.Errorf("BlackHole input device not found in malgo device list")
	}

	prevDefault := coreaudio.GetDefaultOutputDevice()

	uid := fmt.Sprintf("EchoWarp_Loopback_%d", time.Now().UnixNano())
	aggID, err := coreaudio.CreateAggregateDevice(uid, "EchoWarp Loopback", outputCA.UID, outputCA.UID, bhCA.UID)
	if err != nil {
		return nil, fmt.Errorf("create aggregate device: %w", err)
	}

	if err := coreaudio.SetDefaultOutputDevice(aggID); err != nil {
		coreaudio.DestroyAggregateDevice(aggID)
		return nil, fmt.Errorf("set aggregate as default output: %w", err)
	}

	return &darwinLoopbackSession{
		aggregateDeviceID: aggID,
		prevDefaultOutput: prevDefault,
		captureDeviceID:   bhInputID,
	}, nil
}

func findBlackHoleInput(inputs []AudioDevice, name string) (uint32, bool) {
	lower := strings.ToLower(name)
	for _, inp := range inputs {
		if strings.EqualFold(inp.Name, lower) {
			return inp.ID, true
		}
	}
	// Fallback: first BlackHole input
	for _, inp := range inputs {
		if strings.Contains(strings.ToLower(inp.Name), "blackhole") {
			return inp.ID, true
		}
	}
	return 0, false
}

func (s *darwinLoopbackSession) CaptureDeviceID() uint32 {
	return s.captureDeviceID
}

func (s *darwinLoopbackSession) IsNativeLoopback() bool {
	return false
}

func (s *darwinLoopbackSession) Close() error {
	if s.prevDefaultOutput != 0 {
		_ = coreaudio.SetDefaultOutputDevice(s.prevDefaultOutput) //nolint:errcheck
	}
	if s.aggregateDeviceID != 0 {
		coreaudio.DestroyAggregateDevice(s.aggregateDeviceID)
		s.aggregateDeviceID = 0
	}
	return nil
}
