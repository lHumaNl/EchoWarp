//go:build windows

package audio

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type windowsVirtualMic struct {
	name       string
	sampleRate uint32
	channels   uint32
	player     *MalgoPlayer
	inputCh    chan []float32
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
}

func newWindowsVirtualMic(name string, sampleRate, channels uint32) (*windowsVirtualMic, error) {
	dm, err := NewDeviceManager()
	if err != nil {
		return nil, fmt.Errorf("init device manager: %w", err)
	}
	defer dm.Close()

	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	cableDevice := findCableDevice(outputs)
	if cableDevice == nil {
		return nil, fmt.Errorf("VB-Audio Virtual Cable not found; install from https://vb-audio.com/Cable/")
	}

	player, inputCh, ctx, cancel := createVirtualMicComponents(sampleRate, channels)

	vm := &windowsVirtualMic{
		name:       cableDevice.Name,
		sampleRate: sampleRate,
		channels:   channels,
		player:     player,
		inputCh:    inputCh,
		ctx:        ctx,
		cancel:     cancel,
	}

	if err := player.Start(ctx, cableDevice.ID, inputCh); err != nil {
		cancel()
		_ = player.Close()
		return nil, fmt.Errorf("start player on VB-Audio device: %w", err)
	}

	return vm, nil
}

func findCableDevice(outputs []AudioDevice) *AudioDevice {
	searchTerms := []string{"cable", "vb-audio", "virtual"}
	for i := range outputs {
		nameLower := strings.ToLower(outputs[i].Name)
		for _, term := range searchTerms {
			if strings.Contains(nameLower, term) {
				return &outputs[i]
			}
		}
	}
	return nil
}

func createVirtualMicComponents(sampleRate, channels uint32) (*MalgoPlayer, chan []float32, context.Context, context.CancelFunc) {
	player, _ := NewPlayer(sampleRate, channels)
	inputCh := make(chan []float32, 100)
	ctx, cancel := context.WithCancel(context.Background())
	return player, inputCh, ctx, cancel
}

func (v *windowsVirtualMic) Write(samples []float32) error {
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

func (v *windowsVirtualMic) DeviceName() string {
	return v.name
}

func (v *windowsVirtualMic) Close() error {
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
