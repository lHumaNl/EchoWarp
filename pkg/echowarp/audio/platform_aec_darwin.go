//go:build darwin

package audio

import (
	"context"
	"fmt"
	"sync"
)

// CoreAudioAEC is a stub for macOS CoreAudio echo cancellation.
// CoreAudio provides AEC via AudioUnit's kAudioUnitSubType_VoiceProcessingIO,
// which handles echo cancellation at the HAL level with access to exact
// speaker output timing.
//
// TODO: Implement using CGO + AudioToolbox framework:
//   - Create VoiceProcessingIO AudioUnit
//   - Set input/output device via kAudioOutputUnitProperty_CurrentDevice
//   - Enable AEC via kAUVoiceIOProperty_VoiceProcessingEnableAGC
//   - Route captured audio through the unit's render callback
type CoreAudioAEC struct {
	mu         sync.Mutex
	started    bool
	sampleRate uint32
	channels   uint32
}

func getPlatformAEC() PlatformAEC {
	return &CoreAudioAEC{}
}

func (c *CoreAudioAEC) Name() string {
	return "CoreAudio"
}

// Available returns false — this is a stub awaiting CGO implementation.
func (c *CoreAudioAEC) Available() bool {
	return false
}

func (c *CoreAudioAEC) Start(sampleRate uint32, channels uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.Available() {
		return fmt.Errorf("CoreAudio AEC not yet implemented")
	}

	c.sampleRate = sampleRate
	c.channels = channels
	c.started = true
	return nil
}

func (c *CoreAudioAEC) Process(_ context.Context, samples []float32) ([]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return samples, nil
	}

	// TODO: Pass samples through VoiceProcessingIO AudioUnit render callback.
	return samples, nil
}

func (c *CoreAudioAEC) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.started = false
	// TODO: Dispose AudioUnit.
	return nil
}
