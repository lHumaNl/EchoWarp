package app

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestServerApp_EnsureCaptureHub_PublishesOnlyAfterReady(t *testing.T) {
	t.Parallel()

	startEntered := make(chan struct{})
	releaseStart := make(chan struct{})
	app, created := newSharedCaptureTestServer(t, func() *fakeSharedCapturer {
		return &fakeSharedCapturer{start: func(_ context.Context, _ uint32, _ chan<- []float32) error {
			close(startEntered)
			<-releaseStart
			return nil
		}}
	})

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	type ensureResult struct {
		hub *SharedCaptureHub
		err error
	}
	resultCh := make(chan ensureResult, 1)
	secondResultCh := make(chan ensureResult, 1)
	go func() {
		hub, err := app.ensureCaptureHub(serverCtx)
		resultCh <- ensureResult{hub: hub, err: err}
	}()

	select {
	case <-startEntered:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("shared capture startup was not entered")
	}

	go func() {
		hub, err := app.ensureCaptureHub(serverCtx)
		secondResultCh <- ensureResult{hub: hub, err: err}
	}()

	select {
	case res := <-resultCh:
		t.Fatalf("ensureCaptureHub returned before startup was released: %+v", res)
	default:
	}
	select {
	case res := <-secondResultCh:
		t.Fatalf("second ensureCaptureHub call returned before startup was released: %+v", res)
	default:
	}

	close(releaseStart)

	var firstRes ensureResult
	select {
	case firstRes = <-resultCh:
		require.NoError(t, firstRes.err)
		require.NotNil(t, firstRes.hub)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ensureCaptureHub did not return after startup succeeded")
	}
	select {
	case secondRes := <-secondResultCh:
		require.NoError(t, secondRes.err)
		require.NotNil(t, secondRes.hub)
		assert.Same(t, firstRes.hub, secondRes.hub)
		assert.Same(t, firstRes.hub, currentCaptureHub(app))
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second ensureCaptureHub call did not return after startup succeeded")
	}

	assert.Equal(t, int32(1), created.Load())
	cancelServer()
	require.Eventually(t, func() bool { return currentCaptureHub(app) == nil }, time.Second, 10*time.Millisecond)
}

func TestServerApp_EnsureCaptureHub_DoesNotPublishFailedHub(t *testing.T) {
	t.Parallel()

	app, created := newSharedCaptureTestServer(t, func() *fakeSharedCapturer {
		return &fakeSharedCapturer{start: failCaptureStart(assert.AnError)}
	})

	hub, err := app.ensureCaptureHub(context.Background())

	assert.Nil(t, hub)
	require.Error(t, err)
	assert.ErrorContains(t, err, assert.AnError.Error())
	hubState, gainState, agcState := currentCaptureHubState(app)
	assert.Nil(t, hubState, "failed startup must not publish a hub")
	assert.Nil(t, gainState)
	assert.Nil(t, agcState)
	assert.Equal(t, int32(1), created.Load())
}

func TestServerApp_SetupAudioPipelineMultiSend_DoesNotEmitImmediateSubscriptionClosed(t *testing.T) {
	t.Parallel()

	app, _ := newSharedCaptureTestServer(t, func() *fakeSharedCapturer {
		return &fakeSharedCapturer{start: continuousFrameStart(fullPCMFrame(48000, 1, 0.25))}
	})

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientCtx, cancelClient := context.WithCancel(serverCtx)
	defer cancelClient()

	encoded := make(chan []byte, 8)
	peer := &serverMockPeerManager{
		addAudioTrackFunc: func(sampleRate, channels uint32) (chan<- []byte, error) {
			assert.Equal(t, uint32(48000), sampleRate)
			assert.Equal(t, uint32(1), channels)
			return encoded, nil
		},
	}

	audioDone, err := app.setupAudioPipelineMulti(serverCtx, clientCtx, peer, transport.DirectionSend, "client-1", &multiClient{id: "client-1"}, nil, nil, nil)
	require.NoError(t, err)
	hub := currentCaptureHub(app)
	require.NotNil(t, hub)

	packet := waitForEncodedPacket(t, audioDone, encoded)
	require.NotEmpty(t, packet)
	assert.Equal(t, 1, hub.SubscriberCount())

	select {
	case err := <-audioDone:
		t.Fatalf("audio pipeline exited unexpectedly after startup: %v", err)
	default:
	}

	cancelClient()
	require.Eventually(t, func() bool { return hub.SubscriberCount() == 0 }, time.Second, 10*time.Millisecond)
	require.ErrorIs(t, <-audioDone, context.Canceled)
}

func TestServerApp_SetupAudioPipelineMultiSend_ReusesSharedHubForTwoClients(t *testing.T) {
	t.Parallel()

	app, created := newSharedCaptureTestServer(t, func() *fakeSharedCapturer {
		return &fakeSharedCapturer{start: continuousFrameStart(fullPCMFrame(48000, 1, 0.25))}
	})

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientACtx, cancelA := context.WithCancel(serverCtx)
	defer cancelA()
	clientBCtx, cancelB := context.WithCancel(serverCtx)
	defer cancelB()

	encodedA := make(chan []byte, 8)
	encodedB := make(chan []byte, 8)
	peerA := &serverMockPeerManager{addAudioTrackFunc: func(uint32, uint32) (chan<- []byte, error) { return encodedA, nil }}
	peerB := &serverMockPeerManager{addAudioTrackFunc: func(uint32, uint32) (chan<- []byte, error) { return encodedB, nil }}
	mcA := &multiClient{id: "client-a"}
	mcB := &multiClient{id: "client-b"}

	audioDoneA, err := app.setupAudioPipelineMulti(serverCtx, clientACtx, peerA, transport.DirectionSend, "client-a", mcA, nil, nil, nil)
	require.NoError(t, err)
	firstHub := currentCaptureHub(app)
	require.NotNil(t, firstHub)

	audioDoneB, err := app.setupAudioPipelineMulti(serverCtx, clientBCtx, peerB, transport.DirectionSend, "client-b", mcB, nil, nil, nil)
	require.NoError(t, err)

	assert.Same(t, firstHub, currentCaptureHub(app))
	assert.Equal(t, int32(1), created.Load(), "shared hub should be created once")
	require.NotNil(t, mcA.getClientGain())
	require.NotNil(t, mcB.getClientGain())
	require.Eventually(t, func() bool { return firstHub.SubscriberCount() == 2 }, time.Second, 10*time.Millisecond)

	require.NotEmpty(t, waitForEncodedPacket(t, audioDoneA, encodedA))
	require.NotEmpty(t, waitForEncodedPacket(t, audioDoneB, encodedB))

	select {
	case err := <-audioDoneA:
		t.Fatalf("client A pipeline exited unexpectedly: %v", err)
	default:
	}
	select {
	case err := <-audioDoneB:
		t.Fatalf("client B pipeline exited unexpectedly: %v", err)
	default:
	}

	cancelA()
	cancelB()
	require.ErrorIs(t, <-audioDoneA, context.Canceled)
	require.ErrorIs(t, <-audioDoneB, context.Canceled)
	require.Eventually(t, func() bool { return firstHub.SubscriberCount() == 0 }, time.Second, 10*time.Millisecond)
}

func newSharedCaptureTestServer(t *testing.T, newCapturer func() *fakeSharedCapturer) (*ServerApp, *atomic.Int32) {
	t.Helper()

	cfg := testServerConfig()
	cfg.MaxClients = 4
	cfg.SampleRate = 48000
	cfg.Channels = 1
	cfg.Devices = []config.DeviceEntry{{ID: 1, Role: config.RoleCapture, Volume: 1.0}}

	app := NewServerApp(cfg, testHubLogger(), nil, nil, nil)
	created := &atomic.Int32{}
	app.newSharedCaptureHub = func(cfg CapturePipelineConfig, logger *slog.Logger) *SharedCaptureHub {
		created.Add(1)
		hub := NewSharedCaptureHub(cfg, logger)
		capturer := newCapturer()
		hub.newCapturer = func(uint32, uint32, ...audio.CapturerOption) (sharedCapturer, error) {
			return capturer, nil
		}
		return hub
	}

	return app, created
}

func fullPCMFrame(sampleRate, channels uint32, sample float32) []float32 {
	frame := make([]float32, int(sampleRate/50*channels))
	for i := range frame {
		frame[i] = sample
	}
	return frame
}

func continuousFrameStart(frame []float32) func(context.Context, uint32, chan<- []float32) error {
	return func(ctx context.Context, _ uint32, outCh chan<- []float32) error {
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				copied := append([]float32(nil), frame...)
				select {
				case outCh <- copied:
				case <-ctx.Done():
					return
				}
				select {
				case <-ticker.C:
				case <-ctx.Done():
					return
				}
			}
		}()
		return nil
	}
}

func waitForEncodedPacket(t *testing.T, audioDone <-chan error, encoded <-chan []byte) []byte {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		select {
		case err := <-audioDone:
			t.Fatalf("audio pipeline exited before producing encoded audio: %v", err)
		case packet := <-encoded:
			return packet
		case <-deadline:
			t.Fatal("timed out waiting for encoded audio")
		}
	}
}

func currentCaptureHub(app *ServerApp) *SharedCaptureHub {
	app.captureHubMu.Lock()
	defer app.captureHubMu.Unlock()
	return app.captureHub
}

func currentCaptureHubState(app *ServerApp) (*SharedCaptureHub, *DeviceGainControl, map[uint32]*audio.AGCProcessor) {
	app.captureHubMu.Lock()
	defer app.captureHubMu.Unlock()
	return app.captureHub, app.captureHubGain, app.captureHubAGC
}
