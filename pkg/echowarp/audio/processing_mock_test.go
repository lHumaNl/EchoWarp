package audio

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCapturer struct {
	sampleRate uint32
	channels   uint32
	closed     bool
}

func (m *mockCapturer) Start(ctx context.Context, deviceID uint32, outCh chan<- []float32) error {
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case outCh <- make([]float32, 960):
				default:
				}
			}
		}
	}()
	return nil
}

func (m *mockCapturer) SampleRate() uint32 { return m.sampleRate }
func (m *mockCapturer) Channels() uint32   { return m.channels }
func (m *mockCapturer) Close() error {
	m.closed = true
	return nil
}

type mockPlayer struct {
	closed bool
}

func (m *mockPlayer) Start(ctx context.Context, deviceID uint32, inCh <-chan []float32) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-inCh:
				if !ok {
					return
				}
			}
		}
	}()
	return nil
}

func (m *mockPlayer) Close() error {
	m.closed = true
	return nil
}

func TestProcessingCapturer_Basic(t *testing.T) {
	mock := &mockCapturer{sampleRate: 48000, channels: 1}
	pc := NewProcessingCapturer(mock, NewGainProcessor(1.0))

	assert.Equal(t, uint32(48000), pc.SampleRate())
	assert.Equal(t, uint32(1), pc.Channels())
}

func TestProcessingCapturer_AddProcessor(t *testing.T) {
	mock := &mockCapturer{sampleRate: 48000, channels: 1}
	pc := NewProcessingCapturer(mock)

	pc.AddProcessor(NewGainProcessor(2.0))
	procs := pc.Processors()
	assert.Len(t, procs.processors, 1)
}

func TestProcessingCapturer_Close(t *testing.T) {
	mock := &mockCapturer{sampleRate: 48000, channels: 1}
	pc := NewProcessingCapturer(mock)

	err := pc.Close()
	assert.NoError(t, err)
	assert.True(t, mock.closed)
}

func TestProcessingPlayer_Basic(t *testing.T) {
	mock := &mockPlayer{}
	pp := NewProcessingPlayer(mock, NewGainProcessor(1.0))

	assert.NotNil(t, pp.Processors())
}

func TestProcessingPlayer_AddProcessor(t *testing.T) {
	mock := &mockPlayer{}
	pp := NewProcessingPlayer(mock)

	pp.AddProcessor(NewGainProcessor(0.5))
	procs := pp.Processors()
	assert.Len(t, procs.processors, 1)
}

func TestProcessingPlayer_Close(t *testing.T) {
	mock := &mockPlayer{}
	pp := NewProcessingPlayer(mock)

	err := pp.Close()
	assert.NoError(t, err)
	assert.True(t, mock.closed)
}

func TestProcessingCapturer_StartWithProcessor(t *testing.T) {
	mock := &mockCapturer{sampleRate: 48000, channels: 1}
	pc := NewProcessingCapturer(mock, NewGainProcessor(2.0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	outCh := make(chan []float32, 10)
	err := pc.Start(ctx, 0, outCh)
	require.NoError(t, err)

	select {
	case samples := <-outCh:
		assert.NotNil(t, samples)
	case <-time.After(100 * time.Millisecond):
		t.Error("expected output from processing capturer")
	}
}

func TestProcessingPlayer_StartWithProcessor(t *testing.T) {
	mock := &mockPlayer{}
	pp := NewProcessingPlayer(mock, NewGainProcessor(0.5))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inCh := make(chan []float32, 10)
	err := pp.Start(ctx, 0, inCh)
	require.NoError(t, err)

	// Send data through the pipeline
	inCh <- []float32{0.5, 0.5, 0.5}

	// Wait for the data to be processed
	// The mock player will consume the data asynchronously
	time.Sleep(50 * time.Millisecond)
}
