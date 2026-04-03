package audio

import (
	"context"
	"time"
)

// DeviceChangeType describes what changed in the device list.
type DeviceChangeType int

const (
	// DeviceAdded means a new device appeared.
	DeviceAdded DeviceChangeType = iota
	// DeviceRemoved means a device disappeared.
	DeviceRemoved
)

// DeviceChangeEvent is emitted when the system device list changes.
type DeviceChangeEvent struct {
	Type    DeviceChangeType
	Device  AudioDevice
	NewList []AudioDevice // full current device list after the change
}

// DeviceMonitor polls for device list changes at a configurable interval.
// It compares snapshots by device name (since IDs are unstable indices).
type DeviceMonitor struct {
	enumerator DeviceEnumerator
	interval   time.Duration
}

// NewDeviceMonitor creates a monitor that polls every interval.
func NewDeviceMonitor(enumerator DeviceEnumerator, interval time.Duration) *DeviceMonitor {
	return &DeviceMonitor{
		enumerator: enumerator,
		interval:   interval,
	}
}

// Watch starts polling and sends change events to the returned channel.
// It monitors both input and output devices. Blocks until ctx is canceled.
// The returned channel is closed when monitoring stops.
func (dm *DeviceMonitor) Watch(ctx context.Context) <-chan DeviceChangeEvent {
	ch := make(chan DeviceChangeEvent, 8)

	go func() {
		defer close(ch)

		prevInputs := dm.snapshot(true)
		prevOutputs := dm.snapshot(false)

		ticker := time.NewTicker(dm.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currInputs := dm.snapshot(true)
				currOutputs := dm.snapshot(false)
				dm.diff(ctx, prevInputs, currInputs, currInputs, ch)
				dm.diff(ctx, prevOutputs, currOutputs, currOutputs, ch)
				prevInputs = currInputs
				prevOutputs = currOutputs
			}
		}
	}()

	return ch
}

func (dm *DeviceMonitor) snapshot(input bool) map[string]AudioDevice {
	var devices []AudioDevice
	var err error
	if input {
		devices, err = dm.enumerator.ListInputDevices()
	} else {
		devices, err = dm.enumerator.ListOutputDevices()
	}
	if err != nil {
		return nil
	}
	m := make(map[string]AudioDevice, len(devices))
	for _, d := range devices {
		m[d.Name] = d
	}
	return m
}

func (dm *DeviceMonitor) diff(ctx context.Context, prev, curr map[string]AudioDevice, fullList map[string]AudioDevice, ch chan<- DeviceChangeEvent) {
	currentList := make([]AudioDevice, 0, len(fullList))
	for _, d := range fullList {
		currentList = append(currentList, d)
	}

	// Detect removed
	for name, dev := range prev {
		if _, ok := curr[name]; !ok {
			select {
			case ch <- DeviceChangeEvent{Type: DeviceRemoved, Device: dev, NewList: currentList}:
			case <-ctx.Done():
				return
			}
		}
	}
	// Detect added
	for name, dev := range curr {
		if _, ok := prev[name]; !ok {
			select {
			case ch <- DeviceChangeEvent{Type: DeviceAdded, Device: dev, NewList: currentList}:
			case <-ctx.Done():
				return
			}
		}
	}
}
