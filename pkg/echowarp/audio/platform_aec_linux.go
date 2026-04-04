//go:build linux

package audio

import (
	"context"
	"fmt"
	"sync"
)

// PulseAudioAEC is a stub for Linux PulseAudio echo cancellation.
// PulseAudio provides AEC via the module-echo-cancel module, which uses
// WebRTC's AEC algorithm internally. It can be loaded dynamically or
// accessed via the PulseAudio async API.
//
// TODO: Implement using CGO + libpulse:
//   - Load module-echo-cancel via pa_context_load_module()
//   - Or create a virtual source/sink pair with echo cancellation
//   - Route capture through the echo-canceled source
//   - Alternatively, use PipeWire's built-in echo cancellation filter
type PulseAudioAEC struct {
	mu         sync.Mutex
	started    bool
	sampleRate uint32
	channels   uint32
}

func getPlatformAEC() PlatformAEC {
	return &PulseAudioAEC{}
}

func (p *PulseAudioAEC) Name() string {
	return "PulseAudio"
}

// Available returns false — this is a stub awaiting CGO implementation.
func (p *PulseAudioAEC) Available() bool {
	return false
}

func (p *PulseAudioAEC) Start(sampleRate uint32, channels uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.Available() {
		return fmt.Errorf("PulseAudio AEC not yet implemented")
	}

	p.sampleRate = sampleRate
	p.channels = channels
	p.started = true
	return nil
}

func (p *PulseAudioAEC) Process(_ context.Context, samples []float32) ([]float32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return samples, nil
	}

	// TODO: Read from echo-canceled PulseAudio source.
	return samples, nil
}

func (p *PulseAudioAEC) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.started = false
	// TODO: Unload module-echo-cancel.
	return nil
}
