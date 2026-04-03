package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderDualSpectrumWithVU_Normal(t *testing.T) {
	capBands := []float64{0.3, 0.5, 0.8, 1.0, 0.4}
	capVU := []float64{0.7, 0.6}
	playBands := []float64{0.2, 0.6, 0.9, 0.5}
	playVU := []float64{0.5, 0.8}

	result := RenderDualSpectrumWithVU(capBands, capVU, playBands, playVU, 6, 100)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "capture")
	assert.Contains(t, result, "playback")
	assert.Contains(t, result, "▲")
	assert.Contains(t, result, "▼")
}

func TestRenderDualSpectrumWithVU_BothEmpty(t *testing.T) {
	result := RenderDualSpectrumWithVU(nil, nil, nil, nil, 5, 100)
	assert.Empty(t, result)
}

func TestRenderDualSpectrumWithVU_OnlyCaptureEmpty(t *testing.T) {
	playBands := []float64{0.5, 0.7, 0.9}
	playVU := []float64{0.6, 0.8}

	result := RenderDualSpectrumWithVU(nil, nil, playBands, playVU, 5, 100)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "playback")
	assert.Contains(t, result, "▼")
}

func TestRenderDualSpectrumWithVU_OnlyPlaybackEmpty(t *testing.T) {
	capBands := []float64{0.5, 0.7, 0.9}
	capVU := []float64{0.6, 0.8}

	result := RenderDualSpectrumWithVU(capBands, capVU, nil, nil, 5, 100)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "capture")
	assert.Contains(t, result, "▲")
}

func TestRenderDualSpectrumWithVU_NarrowFallback(t *testing.T) {
	capBands := []float64{0.3, 0.5}
	playBands := []float64{0.6, 0.8}

	// Width < 60 should fallback to single spectrum (playback preferred)
	result := RenderDualSpectrumWithVU(capBands, nil, playBands, nil, 5, 50)
	assert.NotEmpty(t, result)
	// Should NOT contain dual labels
	assert.NotContains(t, result, "capture")
	assert.NotContains(t, result, "▲")
}

func TestRenderDualSpectrumWithVU_NarrowFallbackCaptureOnly(t *testing.T) {
	capBands := []float64{0.3, 0.5}

	result := RenderDualSpectrumWithVU(capBands, nil, nil, nil, 5, 50)
	assert.NotEmpty(t, result)
}

func TestRenderDualSpectrumWithVU_HasCorrectLineCount(t *testing.T) {
	capBands := []float64{0.5, 0.8}
	playBands := []float64{0.3, 0.6}

	height := 6
	result := RenderDualSpectrumWithVU(capBands, nil, playBands, nil, height, 80)
	lines := strings.Split(result, "\n")
	assert.Equal(t, height, len(lines))
}

func TestRenderDualSpectrumWithVU_VULabels(t *testing.T) {
	capVU := []float64{0.5, 0.6}
	playVU := []float64{0.7, 0.8}

	result := RenderDualSpectrumWithVU(nil, capVU, nil, playVU, 5, 80)
	assert.Contains(t, result, "L")
	assert.Contains(t, result, "R")
}
