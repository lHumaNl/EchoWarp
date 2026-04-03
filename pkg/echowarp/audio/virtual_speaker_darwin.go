//go:build darwin

package audio

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type darwinVirtualSpeaker struct {
	name     string
	capturer *MalgoCapturer
	outCh    chan []float32
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
}

func newDarwinVirtualSpeaker(_ string, sampleRate, channels uint32) (*darwinVirtualSpeaker, error) {
	dm, err := NewDeviceManager()
	if err != nil {
		return nil, fmt.Errorf("init device manager: %w", err)
	}
	defer func() { _ = dm.Close() }() //nolint:errcheck

	// Find BlackHole as an input device (loopback side)
	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	var blackholeInput *AudioDevice
	for i := range inputs {
		if strings.Contains(strings.ToLower(inputs[i].Name), "blackhole") {
			blackholeInput = &inputs[i]
			break
		}
	}

	if blackholeInput == nil {
		return nil, fmt.Errorf("BlackHole virtual audio device not found; install from https://existential.audio/blackhole/ or run 'brew install blackhole-2ch'")
	}

	capturer, err := NewCapturer(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("create capturer: %w", err)
	}

	outCh := make(chan []float32, 100)
	ctx, cancel := context.WithCancel(context.Background())

	vs := &darwinVirtualSpeaker{
		name:     blackholeInput.Name,
		capturer: capturer,
		outCh:    outCh,
		ctx:      ctx,
		cancel:   cancel,
	}

	if err := capturer.Start(ctx, blackholeInput.ID, outCh); err != nil {
		cancel()
		_ = capturer.Close() //nolint:errcheck
		return nil, fmt.Errorf("start capture on BlackHole device: %w", err)
	}

	return vs, nil
}

func (v *darwinVirtualSpeaker) Read() ([]float32, error) {
	select {
	case samples, ok := <-v.outCh:
		if !ok {
			return nil, fmt.Errorf("virtual speaker closed")
		}
		return samples, nil
	case <-v.ctx.Done():
		return nil, v.ctx.Err()
	}
}

func (v *darwinVirtualSpeaker) DeviceName() string {
	return v.name
}

func (v *darwinVirtualSpeaker) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}

	if v.capturer != nil {
		err := v.capturer.Close()
		v.capturer = nil
		return err
	}

	return nil
}
