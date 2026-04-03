//go:build darwin

package audio

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type darwinVirtualMic struct {
	name       string
	sampleRate uint32
	channels   uint32
	player     *MalgoPlayer
	inputCh    chan []float32
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
}

func newDarwinVirtualMic(_ string, sampleRate, channels uint32) (*darwinVirtualMic, error) {
	dm, err := NewDeviceManager()
	if err != nil {
		return nil, fmt.Errorf("init device manager: %w", err)
	}
	defer func() { _ = dm.Close() }() //nolint:errcheck

	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	var blackholeDevice *AudioDevice
	for i := range outputs {
		if strings.Contains(strings.ToLower(outputs[i].Name), "blackhole") {
			blackholeDevice = &outputs[i]
			break
		}
	}

	if blackholeDevice == nil {
		return nil, fmt.Errorf("BlackHole virtual audio device not found; install from https://existential.audio/blackhole/ or run 'brew install blackhole-2ch'")
	}

	player, err := NewPlayer(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("create player: %w", err)
	}

	inputCh := make(chan []float32, 100)
	ctx, cancel := context.WithCancel(context.Background())

	vm := &darwinVirtualMic{
		name:       blackholeDevice.Name,
		sampleRate: sampleRate,
		channels:   channels,
		player:     player,
		inputCh:    inputCh,
		ctx:        ctx,
		cancel:     cancel,
	}

	if err := player.Start(ctx, blackholeDevice.ID, inputCh); err != nil {
		cancel()
		_ = player.Close() //nolint:errcheck
		return nil, fmt.Errorf("start player on BlackHole device: %w", err)
	}

	return vm, nil
}

func (v *darwinVirtualMic) Write(samples []float32) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.inputCh == nil {
		return fmt.Errorf("virtual mic closed")
	}

	select {
	case v.inputCh <- samples:
		return nil
	default:
		return nil
	}
}

func (v *darwinVirtualMic) DeviceName() string {
	return v.name
}

func (v *darwinVirtualMic) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}

	if v.inputCh != nil {
		close(v.inputCh)
		v.inputCh = nil
	}

	if v.player != nil {
		err := v.player.Close()
		v.player = nil
		return err
	}

	return nil
}
