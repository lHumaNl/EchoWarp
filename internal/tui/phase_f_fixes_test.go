package tui

// Tests for Phase F Bug 9 fix:
//   - Duplex mode: capture and playback spectrum use separate analyzer instances
//   - spectrumTickMsg populates both capture and playback bands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
)

// TestDuplexSpectrumTickPopulatesBothSets verifies that spectrumTickMsg
// populates both playback (spectrumBands/vuLevels) and capture
// (captureSpectrumBands/captureVULevels) when both funcs are set.
func TestDuplexSpectrumTickPopulatesBothSets(t *testing.T) {
	t.Parallel()

	playbackBands := []float64{0.1, 0.2, 0.3, 0.4}
	playbackLevels := []float64{0.5, 0.6}
	captureBands := []float64{0.7, 0.8, 0.9, 1.0}
	captureLevels := []float64{0.3, 0.4}

	cfg := config.Config{Duplex: true}
	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.spectrumTickActive = true
	m.spectrumFunc = func() []float64 { return playbackBands }
	m.levelFunc = func() []float64 { return playbackLevels }
	m.captureSpectrumFunc = func() []float64 { return captureBands }
	m.captureLevelFunc = func() []float64 { return captureLevels }

	updated, _ := m.Update(spectrumTickMsg{})
	um := updated.(Model)

	assert.Equal(t, playbackBands, um.spectrumBands, "playback spectrum bands should be populated")
	assert.Equal(t, playbackLevels, um.vuLevels, "playback VU levels should be populated")
	assert.Equal(t, captureBands, um.captureSpectrumBands, "capture spectrum bands should be populated")
	assert.Equal(t, captureLevels, um.captureVULevels, "capture VU levels should be populated")
}

// TestDuplexSpectrumTickNilCaptureFuncs verifies that nil capture funcs
// do not panic and leave capture fields unchanged.
func TestDuplexSpectrumTickNilCaptureFuncs(t *testing.T) {
	t.Parallel()

	playbackBands := []float64{0.1, 0.2}

	cfg := config.Config{}
	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.spectrumTickActive = true
	m.spectrumFunc = func() []float64 { return playbackBands }
	// captureSpectrumFunc and captureLevelFunc are nil

	updated, _ := m.Update(spectrumTickMsg{})
	um := updated.(Model)

	assert.Equal(t, playbackBands, um.spectrumBands)
	assert.Nil(t, um.captureSpectrumBands, "capture bands should remain nil when func is nil")
	assert.Nil(t, um.captureVULevels, "capture levels should remain nil when func is nil")
}

// TestCaptureAndPlaybackUseDifferentInstances verifies the builder methods
// wire separate funcs for capture and playback.
func TestCaptureAndPlaybackUseDifferentInstances(t *testing.T) {
	t.Parallel()

	playbackCalled := false
	captureCalled := false

	cfg := config.Config{Duplex: true}
	m := NewModel(cfg, nil).
		WithSpectrumFunc(func() []float64 { playbackCalled = true; return []float64{1.0} }).
		WithLevelFunc(func() []float64 { return []float64{0.5} }).
		WithCaptureSpectrumFunc(func() []float64 { captureCalled = true; return []float64{2.0} }).
		WithCaptureLevelFunc(func() []float64 { return []float64{0.3} })

	m.screen = ScreenStreaming
	m.spectrumTickActive = true

	updated, _ := m.Update(spectrumTickMsg{})
	um := updated.(Model)

	assert.True(t, playbackCalled, "playback spectrum func should have been called")
	assert.True(t, captureCalled, "capture spectrum func should have been called")
	assert.Equal(t, []float64{1.0}, um.spectrumBands)
	assert.Equal(t, []float64{2.0}, um.captureSpectrumBands)
}
