//go:build windows

package audio

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type windowsVirtualSpeaker struct {
	name     string
	capturer *MalgoCapturer
	outCh    chan []float32
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
}

func newWindowsVirtualSpeaker(name string, sampleRate, channels uint32) (*windowsVirtualSpeaker, error) {
	dm, err := NewDeviceManager()
	if err != nil {
		return nil, fmt.Errorf("init device manager: %w", err)
	}
	defer dm.Close()

	// Find VB-Cable as an input device (loopback side)
	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	var cableInput *AudioDevice
	searchTerms := []string{"cable", "vb-audio", "virtual"}
	for i := range inputs {
		nameLower := strings.ToLower(inputs[i].Name)
		for _, term := range searchTerms {
			if strings.Contains(nameLower, term) {
				cableInput = &inputs[i]
				break
			}
		}
		if cableInput != nil {
			break
		}
	}

	if cableInput == nil {
		return nil, fmt.Errorf("VB-Audio Virtual Cable input not found; install from https://vb-audio.com/Cable/")
	}

	capturer, err := NewCapturer(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("create capturer: %w", err)
	}

	outCh := make(chan []float32, 100)
	ctx, cancel := context.WithCancel(context.Background())

	vs := &windowsVirtualSpeaker{
		name:     cableInput.Name,
		capturer: capturer,
		outCh:    outCh,
		ctx:      ctx,
		cancel:   cancel,
	}

	if err := capturer.Start(ctx, cableInput.ID, outCh); err != nil {
		cancel()
		_ = capturer.Close()
		return nil, fmt.Errorf("start capture on VB-Cable device: %w", err)
	}

	return vs, nil
}

func (v *windowsVirtualSpeaker) Read() ([]float32, error) {
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

func (v *windowsVirtualSpeaker) DeviceName() string {
	return v.name
}

func (v *windowsVirtualSpeaker) Close() error {
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
