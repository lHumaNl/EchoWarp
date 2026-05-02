package audio

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
)

// MalgoDeviceManager manages audio device enumeration using the malgo library.
// It provides a unified interface for listing input and output audio devices
// across different platforms (macOS, Windows, Linux).
//
// Thread-safe: concurrent device enumeration is protected by internal mutex.
type MalgoDeviceManager struct {
	ctx *malgo.AllocatedContext
	mu  sync.Mutex
}

// NewDeviceManager creates a new device manager. This initializes the underlying
// audio context and should be called before any device enumeration.
func NewDeviceManager() (*MalgoDeviceManager, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("audio device manager: init context: %w", err)
	}
	return &MalgoDeviceManager{ctx: ctx}, nil
}

// ListInputDevices returns all available audio capture devices.
func (m *MalgoDeviceManager) ListInputDevices() ([]AudioDevice, error) {
	return m.listDevices(malgo.Capture)
}

// ListOutputDevices returns all available audio playback devices.
func (m *MalgoDeviceManager) ListOutputDevices() ([]AudioDevice, error) {
	return m.listDevices(malgo.Playback)
}

func (m *MalgoDeviceManager) listDevices(kind malgo.DeviceType) ([]AudioDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	infos, err := m.ctx.Devices(kind)
	if err != nil {
		return nil, fmt.Errorf("audio device manager: list devices: %w", err)
	}

	// NOTE: Device IDs are array indices into malgo's device list, not stable identifiers.
	// The device list is re-queried each time ListInputDevices/ListOutputDevices is called.
	// This matches malgo's API design where device enumeration returns a snapshot.
	// If devices are added/removed between calls, IDs may shift.
	devices := make([]AudioDevice, 0, len(infos))
	for i, info := range infos {
		fullInfo, err := m.ctx.DeviceInfo(kind, info.ID, malgo.Shared)
		if err != nil {
			continue
		}

		var channels, sampleRate, bitDepth uint32
		if len(fullInfo.Formats) > 0 {
			channels = fullInfo.Formats[0].Channels
			sampleRate = fullInfo.Formats[0].SampleRate
			switch fullInfo.Formats[0].Format {
			case malgo.FormatS16:
				bitDepth = 16
			case malgo.FormatS24:
				bitDepth = 24
			case malgo.FormatS32:
				bitDepth = 32
			case malgo.FormatF32:
				bitDepth = 32
			}
		}

		devices = append(devices, AudioDevice{
			ID:         uint32(i),
			Name:       info.Name(),
			BackendID:  malgoDeviceBackendID(info.ID),
			IsInput:    kind == malgo.Capture,
			Channels:   channels,
			SampleRate: sampleRate,
			BitDepth:   bitDepth,
		})
	}

	return devices, nil
}

func malgoDeviceBackendID(id malgo.DeviceID) string {
	raw := bytes.TrimRight(id[:], "\x00")
	if len(raw) == 0 {
		return ""
	}
	if isPrintableNativeID(raw) {
		return string(raw)
	}
	return id.String()
}

func isPrintableNativeID(raw []byte) bool {
	for _, b := range raw {
		if b < 0x20 || b > 0x7e {
			return false
		}
	}
	return true
}

// Context returns the underlying malgo context for use with other audio components.
// This allows sharing the context between capturers, players, and device enumeration.
func (m *MalgoDeviceManager) Context() *malgo.AllocatedContext {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ctx
}

// Close releases the malgo context and all associated resources.
// This should be called when the device manager is no longer needed.
func (m *MalgoDeviceManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctx != nil {
		if err := m.ctx.Uninit(); err != nil {
			return fmt.Errorf("audio device manager: close: %w", err)
		}
		m.ctx.Free()
		m.ctx = nil
	}
	return nil
}
