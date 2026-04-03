package audio

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessorChain_Empty(t *testing.T) {
	chain := NewProcessorChain()
	samples := []float32{0.5, -0.5, 0.3}
	result, err := chain.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != len(samples) {
		t.Errorf("expected %d samples, got %d", len(samples), len(result))
	}
}

func TestProcessorChain_Single(t *testing.T) {
	chain := NewProcessorChain(NewGainProcessor(2.0))
	samples := []float32{0.25, -0.25, 0.1}
	result, err := chain.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []float32{0.5, -0.5, 0.2}
	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("sample[%d]: expected %f, got %f", i, expected[i], result[i])
		}
	}
}

func TestProcessorChain_Multiple(t *testing.T) {
	chain := NewProcessorChain(
		NewGainProcessor(2.0),
		NewGainProcessor(0.5),
	)
	samples := []float32{0.4, -0.4, 0.2}
	result, err := chain.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range samples {
		if result[i] != samples[i] {
			t.Errorf("sample[%d]: expected %f (unchanged), got %f", i, samples[i], result[i])
		}
	}
}

func TestProcessorChain_Add(t *testing.T) {
	chain := NewProcessorChain()
	chain.Add(NewGainProcessor(2.0))
	samples := []float32{0.25}
	result, err := chain.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0] != 0.5 {
		t.Errorf("expected 0.5, got %f", result[0])
	}
}

func TestGainProcessor_Clamping(t *testing.T) {
	p := NewGainProcessor(10.0)
	samples := []float32{0.5, -0.5}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0] != 1.0 {
		t.Errorf("expected clamped to 1.0, got %f", result[0])
	}
	if result[1] != -1.0 {
		t.Errorf("expected clamped to -1.0, got %f", result[1])
	}
}

func TestGainProcessor_SetGet(t *testing.T) {
	p := NewGainProcessor(1.0)
	p.SetGain(2.5)
	if p.Gain() != 2.5 {
		t.Errorf("expected gain 2.5, got %f", p.Gain())
	}
}

func TestGainProcessor_Name(t *testing.T) {
	p := NewGainProcessor(1.0)
	if p.Name() != "gain" {
		t.Errorf("expected name 'gain', got %s", p.Name())
	}
}

func TestSilenceProcessor_Quiet(t *testing.T) {
	p := NewSilenceProcessor(0.01)
	samples := []float32{0.001, 0.001, 0.001}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for quiet samples, got %v", result)
	}
}

func TestSilenceProcessor_Loud(t *testing.T) {
	p := NewSilenceProcessor(0.01)
	samples := []float32{0.5, 0.5, 0.5}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil for loud samples")
	}
}

func TestSilenceProcessor_Empty(t *testing.T) {
	p := NewSilenceProcessor(0.01)
	result, err := p.Process(context.Background(), []float32{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty samples, got %v", result)
	}
}

func TestSilenceProcessor_SetGet(t *testing.T) {
	p := NewSilenceProcessor(0.01)
	p.SetThreshold(0.05)
	if p.Threshold() != 0.05 {
		t.Errorf("expected threshold 0.05, got %f", p.Threshold())
	}
}

func TestNormalizeProcessor_Basic(t *testing.T) {
	p := NewNormalizeProcessor(0.5)
	samples := []float32{0.25, -0.25, 0.1}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0] != 0.5 {
		t.Errorf("expected first sample 0.5, got %f", result[0])
	}
	if result[1] != -0.5 {
		t.Errorf("expected second sample -0.5, got %f", result[1])
	}
}

func TestNormalizeProcessor_TooQuiet(t *testing.T) {
	p := NewNormalizeProcessor(0.5)
	samples := []float32{0.0001, -0.0001}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0] != 0.0001 {
		t.Errorf("expected unchanged sample, got %f", result[0])
	}
}

func TestNormalizeProcessor_Empty(t *testing.T) {
	p := NewNormalizeProcessor(0.5)
	result, err := p.Process(context.Background(), []float32{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d samples", len(result))
	}
}

func TestNormalizeProcessor_SetGet(t *testing.T) {
	p := NewNormalizeProcessor(0.5)
	p.SetTargetLevel(0.8)
	if p.TargetLevel() != 0.8 {
		t.Errorf("expected target level 0.8, got %f", p.TargetLevel())
	}
}

func TestLoggingProcessor_Name(t *testing.T) {
	p := NewLoggingProcessor("test", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if p.Name() != "logging:test" {
		t.Errorf("expected name 'logging:test', got %s", p.Name())
	}
}

func TestLoggingProcessor_NilLogger(t *testing.T) {
	p := NewLoggingProcessor("test", nil)
	samples := []float32{0.5}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0] != 0.5 {
		t.Errorf("expected sample unchanged, got %f", result[0])
	}
}

func TestProcessorChain_ContextCancellation(t *testing.T) {
	chain := NewProcessorChain(NewGainProcessor(1.0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := chain.Process(ctx, []float32{0.5})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestProcessorChain_SilenceFilterReturns(t *testing.T) {
	chain := NewProcessorChain(
		NewSilenceProcessor(0.01),
		NewGainProcessor(2.0),
	)

	quietSamples := []float32{0.001, 0.001}
	result, err := chain.Process(context.Background(), quietSamples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when silence filter returns nil")
	}
}

func TestProcessorChain_Name(t *testing.T) {
	chain := NewProcessorChain()
	if chain.Name() != "chain" {
		t.Errorf("expected name 'chain', got %s", chain.Name())
	}
}

func TestProcessorChain_Processors(t *testing.T) {
	gain := NewGainProcessor(1.0)
	silence := NewSilenceProcessor(0.01)
	chain := NewProcessorChain(gain, silence)

	procs := chain.Processors()
	if len(procs) != 2 {
		t.Errorf("expected 2 processors, got %d", len(procs))
	}
}

func TestSilenceProcessor_Name(t *testing.T) {
	p := NewSilenceProcessor(0.01)
	if p.Name() != "silence_filter" {
		t.Errorf("expected name 'silence_filter', got %s", p.Name())
	}
}

func TestNormalizeProcessor_Name(t *testing.T) {
	p := NewNormalizeProcessor(0.5)
	if p.Name() != "normalize" {
		t.Errorf("expected name 'normalize', got %s", p.Name())
	}
}

func TestLoggingProcessor_TriggerStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	p := NewLoggingProcessor("test", logger)

	for i := 0; i < 200; i++ {
		_, err := p.Process(context.Background(), []float32{0.5, -0.5})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestLoggingProcessor_TriggerStats_WithNegativeSamples(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	p := NewLoggingProcessor("test", logger)

	samples := []float32{0.5, -0.8, 0.3, -0.9}
	for i := 0; i < 100; i++ {
		_, err := p.Process(context.Background(), samples)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestNormalizeProcessor_NegativeSamples(t *testing.T) {
	p := NewNormalizeProcessor(1.0)
	samples := []float32{-0.5, 0.25, -0.1}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0] != -1.0 {
		t.Errorf("expected first sample -1.0, got %f", result[0])
	}
	if result[1] != 0.5 {
		t.Errorf("expected second sample 0.5, got %f", result[1])
	}
}

func TestGainProcessor_ZeroGain(t *testing.T) {
	p := NewGainProcessor(0.0)
	samples := []float32{0.5, -0.5, 0.3}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range result {
		if result[i] != 0.0 {
			t.Errorf("expected 0.0 at index %d, got %f", i, result[i])
		}
	}
}

func TestLoggingProcessor_99thFrame(t *testing.T) {
	p := NewLoggingProcessor("test", nil)

	for i := 0; i < 99; i++ {
		_, err := p.Process(context.Background(), []float32{0.5})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestNormalizeProcessor_AllNegative(t *testing.T) {
	p := NewNormalizeProcessor(1.0)
	samples := []float32{-0.5, -0.3, -0.1}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, -1.0, float64(result[0]))
}

func TestSilenceProcessor_BoundaryThreshold(t *testing.T) {
	p := NewSilenceProcessor(0.1)

	samples := []float32{0.09, 0.09, 0.09}
	result, err := p.Process(context.Background(), samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Nil(t, result)

	loudSamples := []float32{0.2, 0.2, 0.2}
	result, err = p.Process(context.Background(), loudSamples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.NotNil(t, result)
}

func TestProcessorChain_MultipleSilenceFilters(t *testing.T) {
	chain := NewProcessorChain(
		NewSilenceProcessor(0.01),
		NewSilenceProcessor(0.02),
	)

	loudSamples := []float32{0.5, 0.5}
	result, err := chain.Process(context.Background(), loudSamples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.NotNil(t, result)
}
