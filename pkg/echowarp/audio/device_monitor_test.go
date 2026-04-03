package audio

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEnumerator struct {
	mu      sync.Mutex
	inputs  []AudioDevice
	outputs []AudioDevice
}

func (m *mockEnumerator) ListInputDevices() ([]AudioDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]AudioDevice, len(m.inputs))
	copy(cp, m.inputs)
	return cp, nil
}

func (m *mockEnumerator) ListOutputDevices() ([]AudioDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]AudioDevice, len(m.outputs))
	copy(cp, m.outputs)
	return cp, nil
}

func (m *mockEnumerator) setInputs(devs []AudioDevice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputs = devs
}

func TestDeviceMonitor_DetectsAddedDevice(t *testing.T) {
	enum := &mockEnumerator{
		inputs: []AudioDevice{{ID: 0, Name: "Mic1", IsInput: true}},
	}
	mon := NewDeviceMonitor(enum, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ch := mon.Watch(ctx)

	// Add a device after a short delay
	time.AfterFunc(50*time.Millisecond, func() {
		enum.setInputs([]AudioDevice{
			{ID: 0, Name: "Mic1", IsInput: true},
			{ID: 1, Name: "Mic2", IsInput: true},
		})
	})

	var events []DeviceChangeEvent
	for ev := range ch {
		events = append(events, ev)
		if len(events) >= 1 {
			cancel()
		}
	}

	require.GreaterOrEqual(t, len(events), 1)
	assert.Equal(t, DeviceAdded, events[0].Type)
	assert.Equal(t, "Mic2", events[0].Device.Name)
}

func TestDeviceMonitor_DetectsRemovedDevice(t *testing.T) {
	enum := &mockEnumerator{
		inputs: []AudioDevice{
			{ID: 0, Name: "Mic1", IsInput: true},
			{ID: 1, Name: "Mic2", IsInput: true},
		},
	}
	mon := NewDeviceMonitor(enum, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ch := mon.Watch(ctx)

	// Remove a device
	time.AfterFunc(50*time.Millisecond, func() {
		enum.setInputs([]AudioDevice{{ID: 0, Name: "Mic1", IsInput: true}})
	})

	var events []DeviceChangeEvent
	for ev := range ch {
		events = append(events, ev)
		if len(events) >= 1 {
			cancel()
		}
	}

	require.GreaterOrEqual(t, len(events), 1)
	assert.Equal(t, DeviceRemoved, events[0].Type)
	assert.Equal(t, "Mic2", events[0].Device.Name)
}

func TestDeviceMonitor_NoEventsWhenUnchanged(t *testing.T) {
	enum := &mockEnumerator{
		inputs: []AudioDevice{{ID: 0, Name: "Mic1", IsInput: true}},
	}
	mon := NewDeviceMonitor(enum, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch := mon.Watch(ctx)

	var events []DeviceChangeEvent
	for ev := range ch {
		events = append(events, ev)
	}

	assert.Empty(t, events)
}
