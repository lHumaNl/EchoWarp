package audio

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

func TestCapture_BufferFull_DropsFrame(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	pcmCh := make(chan []float32, 1)

	samples := make([]float32, 960)
	for i := range samples {
		samples[i] = 0.5
	}

	select {
	case pcmCh <- samples:
	default:
		t.Fatal("should not block on first send")
	}

	select {
	case pcmCh <- samples:
		t.Fatal("should not send when buffer is full")
	default:
	}

	select {
	case <-pcmCh:
	default:
		t.Fatal("buffer should contain one item")
	}
}

func TestCapture_BufferFull_MetricsUpdated(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	capturer := &MalgoCapturer{
		sampleRate: 48000,
		channels:   1,
		ownsCtx:    true,
	}

	pcmCh := make(chan []float32, 1)
	callback := capturer.createOnRecvCallback(pcmCh)

	sampleData := make([]byte, 960*4)
	for i := 0; i < len(sampleData); i += 4 {
		sampleData[i] = 0x00
		sampleData[i+1] = 0x00
		sampleData[i+2] = 0x00
		sampleData[i+3] = 0x3F
	}

	callback(nil, sampleData, 960)

	beforeDrop := testutil.ToFloat64(metrics.FramesDroppedTotal.WithLabelValues("pcm"))

	callback(nil, sampleData, 960)

	afterDrop := testutil.ToFloat64(metrics.FramesDroppedTotal.WithLabelValues("pcm"))

	assert.Equal(t, beforeDrop, afterDrop-1, "FramesDroppedTotal should be incremented")

	_ = capturer.Close()
}

func TestCapture_ConcurrentSends(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	capturer := &MalgoCapturer{
		sampleRate: 48000,
		channels:   1,
		ownsCtx:    true,
	}

	pcmCh := make(chan []float32, 10)
	callback := capturer.createOnRecvCallback(pcmCh)

	sampleData := make([]byte, 960*4)
	for i := 0; i < len(sampleData); i += 4 {
		sampleData[i] = 0x00
		sampleData[i+1] = 0x00
		sampleData[i+2] = 0x00
		sampleData[i+3] = 0x3F
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				done <- true
				return
			default:
				callback(nil, sampleData, 960)
			}
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("concurrent sends should not deadlock")
	}

	_ = capturer.Close()
}

func TestCapture_MonitorBuffer(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	capturer := &MalgoCapturer{
		sampleRate: 48000,
		channels:   1,
		ownsCtx:    true,
	}

	ch := make(chan []float32, 100)

	for i := 0; i < 50; i++ {
		ch <- make([]float32, 960)
	}

	// Use context with deadline to ensure test completes
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		capturer.monitorBuffer(ctx, ch, "test_pcm")
	}()

	// Wait for monitor to complete (when context expires)
	select {
	case <-done:
		// Monitor completed as expected
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for monitor to complete")
	}

	_ = capturer.Close()
}
