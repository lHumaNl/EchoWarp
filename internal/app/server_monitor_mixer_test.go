package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestServerMonitorMixer_MixesMultipleClientSources(t *testing.T) {
	t.Parallel()
	mixer := newTestServerMonitorMixer()
	mixer.AddSource("client-a")
	mixer.AddSource("client-b")

	mixer.SubmitSourceFrame("client-a", []float32{0.10, -0.20})
	mixer.SubmitSourceFrame("client-b", []float32{0.20, 0.50})

	got := mixer.MixOnce(context.Background())
	assertFloatFrame(t, []float32{tanh32(0.30), tanh32(0.30)}, got)
}

func TestServerMonitorMixer_FeedsAECReferenceFromMixedOutput(t *testing.T) {
	t.Parallel()
	var references [][]float32
	mixer := NewServerMonitorMixer(ServerMonitorMixerConfig{
		SampleRate: 50,
		Channels:   1,
		FrameSize:  2,
		ReferenceFeeder: func(frame []float32) {
			references = append(references, append([]float32(nil), frame...))
		},
	})
	mixer.AddSource("client-a")
	mixer.AddSource("client-b")
	mixer.SubmitSourceFrame("client-a", []float32{0.25, 0.10})
	mixer.SubmitSourceFrame("client-b", []float32{0.25, -0.30})

	got := mixer.MixOnce(context.Background())
	require.Len(t, references, 1)
	assertFloatFrame(t, got, references[0])
}

func TestServerMonitorMixer_RemoveSourceStopsIncludingIt(t *testing.T) {
	t.Parallel()
	mixer := newTestServerMonitorMixer()
	mixer.AddSource("client-a")
	mixer.AddSource("client-b")
	mixer.RemoveSource("client-b")

	mixer.SubmitSourceFrame("client-a", []float32{0.40, 0.20})
	mixer.SubmitSourceFrame("client-b", []float32{0.40, 0.20})

	got := mixer.MixOnce(context.Background())
	assertFloatFrame(t, []float32{tanh32(0.40), tanh32(0.20)}, got)
}

func TestSetupAudioPipelineMultiDuplex_UsesSharedCaptureAndMonitor(t *testing.T) {
	t.Parallel()
	app, starts := newTestDuplexMultiServer(t, nil)
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientCtx, cancelClient := context.WithCancel(serverCtx)
	defer cancelClient()

	_, err := app.setupAudioPipelineMulti(serverCtx, clientCtx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-1", &multiClient{id: "client-1"}, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, app.captureHub)
	require.NotNil(t, app.monitorMixer)
	assert.Equal(t, 1, app.monitorMixer.SourceCount())
	assert.Equal(t, int32(1), starts.Load())
}

func TestSetupAudioPipelineMultiDuplex_WiresPerClientGain(t *testing.T) {
	t.Parallel()
	const (
		clientID             = "client-1"
		clientNickname       = "Client 1"
		volumeDelta          = 0.25
		highVolumeDelta      = 10.0
		lowVolumeDelta       = -10.0
		expectedAdjustedGain = 1.25
		maxClientGain        = float32(1.5)
		minClientGain        = float32(0)
		gainTolerance        = 1e-6
	)
	app, _ := newTestDuplexMultiServer(t, nil)
	mc := &multiClient{id: clientID, nickname: clientNickname}
	app.mu.Lock()
	app.clients[clientID] = mc
	app.mu.Unlock()
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientCtx, cancelClient := context.WithCancel(serverCtx)
	defer cancelClient()

	_, err := app.setupAudioPipelineMulti(serverCtx, clientCtx, &serverMockPeerManager{}, transport.DirectionDuplex, clientID, mc, nil, nil, nil)
	require.NoError(t, err)
	gain := mc.getClientGain()
	require.NotNil(t, gain)

	app.adjustClientVolume(clientID, volumeDelta)
	assert.InDelta(t, expectedAdjustedGain, float64(gain.Gain()), gainTolerance)
	app.adjustClientVolume(clientID, highVolumeDelta)
	assert.Equal(t, maxClientGain, gain.Gain())
	app.adjustClientVolume(clientID, lowVolumeDelta)
	assert.Equal(t, minClientGain, gain.Gain())
}

func TestSetupAudioPipelineMultiDuplex_ReusesOneServerMonitorMixer(t *testing.T) {
	t.Parallel()
	app, starts := newTestDuplexMultiServer(t, nil)
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientACtx, cancelA := context.WithCancel(serverCtx)
	clientBCtx, cancelB := context.WithCancel(serverCtx)
	defer cancelA()
	defer cancelB()

	_, err := app.setupAudioPipelineMulti(serverCtx, clientACtx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-a", &multiClient{id: "client-a"}, nil, nil, nil)
	require.NoError(t, err)
	firstMixer := app.monitorMixer
	firstHub := app.captureHub
	_, err = app.setupAudioPipelineMulti(serverCtx, clientBCtx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-b", &multiClient{id: "client-b"}, nil, nil, nil)
	require.NoError(t, err)

	assert.Same(t, firstMixer, app.monitorMixer)
	assert.Same(t, firstHub, app.captureHub)
	assert.Equal(t, 2, app.monitorMixer.SourceCount())
	assert.Equal(t, int32(1), starts.Load())
}

func TestSetupAudioPipelineMultiDuplex_RemovesMonitorSourceOnDisconnect(t *testing.T) {
	t.Parallel()
	app, _ := newTestDuplexMultiServer(t, nil)
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientCtx, cancelClient := context.WithCancel(serverCtx)

	_, err := app.setupAudioPipelineMulti(serverCtx, clientCtx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-1", &multiClient{id: "client-1"}, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, app.monitorMixer.SourceCount())
	cancelClient()

	require.Eventually(t, func() bool {
		return app.monitorMixer.SourceCount() == 0
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestServerMonitorMixer_SourceContextCancelsOnMonitorCancel(t *testing.T) {
	t.Parallel()
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	monitorCtx, cancelMonitor := context.WithCancel(serverCtx)
	clientCtx, cancelClient := context.WithCancel(serverCtx)
	defer cancelClient()
	mixer := newTestServerMonitorMixer()
	mixer.setMonitorContext(monitorCtx)
	sourceCtx := mixer.sourceContext(clientCtx)

	cancelMonitor()

	select {
	case <-sourceCtx.Done():
		assert.NoError(t, clientCtx.Err())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("source context was not canceled by monitor context")
	}
}

func TestEnsureServerMonitorMixer_PlayerStartupFailureResetsAndCancels(t *testing.T) {
	t.Parallel()
	app, starts := newTestDuplexMultiServer(t, nil)
	startupErr := errors.New("player startup failed")
	var playerCtx context.Context
	app.monitorPlayerStart = func(ctx context.Context, _ *slog.Logger, _ config.Config, _ <-chan []float32, _ chan<- error, _ <-chan struct{}, startupCh chan<- error) {
		starts.Add(1)
		playerCtx = ctx
		startupCh <- startupErr
	}
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	mixer, err := app.ensureServerMonitorMixer(serverCtx)

	require.ErrorIs(t, err, startupErr)
	assert.Nil(t, mixer)
	assert.Nil(t, app.monitorMixer)
	assert.Nil(t, app.monitorPlaybackGain)
	assert.Nil(t, app.monitorPlaybackAGC)
	assert.Equal(t, int32(1), starts.Load())
	require.NotNil(t, playerCtx)
	require.Eventually(t, func() bool {
		return playerCtx.Err() != nil
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestEnsureServerMonitorMixer_PlayerRuntimeErrorAfterStartupResetsAndCancels(t *testing.T) {
	t.Parallel()
	app, starts := newTestDuplexMultiServer(t, nil)
	runtimeErr := errors.New("player runtime failed")
	sendRuntimeErr := make(chan struct{})
	monitorCanceled := make(chan struct{})
	var playerCtx context.Context

	app.monitorPlayerStart = func(ctx context.Context, _ *slog.Logger, _ config.Config, _ <-chan []float32, monitorDone chan<- error, _ <-chan struct{}, startupCh chan<- error) {
		starts.Add(1)
		playerCtx = ctx
		startupCh <- nil
		go func() {
			<-ctx.Done()
			close(monitorCanceled)
		}()
		go func() {
			<-sendRuntimeErr
			monitorDone <- runtimeErr
		}()
	}

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	mixer, err := app.ensureServerMonitorMixer(serverCtx)
	require.NoError(t, err)
	require.NotNil(t, mixer)
	require.NotNil(t, playerCtx)
	mixer.AddSource("client-1")
	require.Equal(t, 1, mixer.SourceCount())
	assert.Equal(t, int32(1), starts.Load())

	monitorState := func() (hasMixer, hasGain, hasAGC bool) {
		app.monitorMixerMu.Lock()
		defer app.monitorMixerMu.Unlock()
		return app.monitorMixer != nil, app.monitorPlaybackGain != nil, app.monitorPlaybackAGC != nil
	}

	hasMixer, hasGain, hasAGC := monitorState()
	require.True(t, hasMixer)
	require.True(t, hasGain)
	require.True(t, hasAGC)

	close(sendRuntimeErr)

	require.Eventually(t, func() bool {
		select {
		case <-monitorCanceled:
			return true
		default:
			return false
		}
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		hasMixer, hasGain, hasAGC := monitorState()
		return !hasMixer && !hasGain && !hasAGC && mixer.SourceCount() == 0
	}, 500*time.Millisecond, 10*time.Millisecond)
	assert.ErrorIs(t, playerCtx.Err(), context.Canceled)
}

func TestEnsureServerMonitorMixer_PublishesMixedOutputToStartedPlayer(t *testing.T) {
	t.Parallel()
	app, _ := newTestDuplexMultiServer(t, nil)
	app.cfg.SampleRate = 50
	app.cfg.Channels = 1
	played := make(chan []float32, 8)
	app.monitorPlayerStart = func(ctx context.Context, _ *slog.Logger, _ config.Config, ch <-chan []float32, _ chan<- error, _ <-chan struct{}, startupCh chan<- error) {
		startupCh <- nil
		go collectMonitorPlayback(ctx, ch, played)
	}
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	mixer, err := app.ensureServerMonitorMixer(serverCtx)
	require.NoError(t, err)
	mixer.AddSource("client-a")
	mixer.AddSource("client-b")
	mixer.SubmitSourceFrame("client-a", []float32{0.25})
	mixer.SubmitSourceFrame("client-b", []float32{0.25})

	assertEventuallyFrame(t, []float32{tanh32(0.50)}, played)
}

func TestSetupAudioPipelineMultiDuplex_MultiDeviceCaptureUsesLegacyFallback(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	app, _ := newTestDuplexMultiServer(t, logger)
	app.cfg.Devices = []config.DeviceEntry{
		{ID: 10, Role: config.RoleCapture, Volume: 1.0},
		{ID: 11, Role: config.RoleCapture, Volume: 1.0},
		{ID: 12, Role: config.RolePlayback, Volume: 1.0},
	}
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientCtx, cancelClient := context.WithCancel(serverCtx)
	defer cancelClient()

	_, err := app.setupAudioPipelineMulti(serverCtx, clientCtx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-1", &multiClient{id: "client-1"}, nil, nil, nil)
	require.NoError(t, err)

	assert.Nil(t, app.captureHub)
	assert.NotNil(t, app.monitorMixer)
	assert.True(t, strings.Contains(logs.String(), "legacy per-client capture"))
}

func TestSetupAudioPipelineMultiDuplex_MultiDeviceFallbackDoesNotConsumeDeviceCommands(t *testing.T) {
	t.Parallel()
	app, _ := newTestDuplexMultiServer(t, nil)
	app.cfg.Devices = []config.DeviceEntry{
		{ID: 10, Role: config.RoleCapture, Volume: 1.0},
		{ID: 11, Role: config.RoleCapture, Volume: 1.0},
		{ID: 12, Role: config.RolePlayback, Volume: 1.0},
	}
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientACtx, cancelA := context.WithCancel(serverCtx)
	clientBCtx, cancelB := context.WithCancel(serverCtx)
	defer cancelA()
	defer cancelB()

	_, err := app.setupAudioPipelineMulti(serverCtx, clientACtx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-a", &multiClient{id: "client-a"}, nil, nil, nil)
	require.NoError(t, err)
	_, err = app.setupAudioPipelineMulti(serverCtx, clientBCtx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-b", &multiClient{id: "client-b"}, nil, nil, nil)
	require.NoError(t, err)
	app.deviceCmdCh <- DeviceCommand{Action: DeviceVolumeUp, DeviceID: 10}
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1, len(app.deviceCmdCh))
}

func TestSetupAudioPipelineSingleDuplex_DoesNotUsePhase2SharedPaths(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Duplex = true
	cfg.MaxClients = 1
	cfg.DeviceID = nil
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	app.clients["client-1"] = &multiClient{id: "client-1"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := app.setupAudioPipeline(ctx, &serverMockPeerManager{}, transport.DirectionDuplex, "client-1")
	require.NoError(t, err)
	assert.Nil(t, app.captureHub)
	assert.Nil(t, app.monitorMixer)
}

func newTestServerMonitorMixer() *ServerMonitorMixer {
	return NewServerMonitorMixer(ServerMonitorMixerConfig{
		SampleRate:   50,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 2,
	})
}

func newTestDuplexMultiServer(t *testing.T, logger *slog.Logger) (*ServerApp, *atomic.Int32) {
	t.Helper()
	cfg := testServerConfig()
	cfg.Duplex = true
	cfg.MaxClients = 4
	cfg.Devices = []config.DeviceEntry{
		{ID: 1, Role: config.RoleCapture, Volume: 1.0},
		{ID: 2, Role: config.RolePlayback, Volume: 1.0},
	}
	if logger == nil {
		logger = testAppLogger()
	}
	app := NewServerApp(cfg, logger, nil, nil, nil)
	installFakeSharedCaptureHub(app, func() *fakeSharedCapturer {
		return &fakeSharedCapturer{}
	})
	starts := &atomic.Int32{}
	app.monitorPlayerStart = func(ctx context.Context, _ *slog.Logger, _ config.Config, ch <-chan []float32, _ chan<- error, _ <-chan struct{}, startupCh chan<- error) {
		starts.Add(1)
		startupCh <- nil
		go drainMonitorPlayback(ctx, ch)
	}
	return app, starts
}

func collectMonitorPlayback(ctx context.Context, ch <-chan []float32, played chan<- []float32) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-ch:
			played <- append([]float32(nil), frame...)
		}
	}
}

func drainMonitorPlayback(ctx context.Context, ch <-chan []float32) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
		}
	}
}

func tanh32(v float32) float32 {
	return float32(math.Tanh(float64(v)))
}

func assertFloatFrame(t *testing.T, want, got []float32) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		assert.InDelta(t, want[i], got[i], 5e-3)
	}
}

func assertEventuallyFrame(t *testing.T, want []float32, frames <-chan []float32) {
	t.Helper()
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case got := <-frames:
			if framesMatch(want, got) {
				return
			}
		case <-deadline:
			t.Fatalf("expected frame %v was not played", want)
		}
	}
}

func framesMatch(want, got []float32) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if math.Abs(float64(want[i]-got[i])) > 5e-3 {
			return false
		}
	}
	return true
}
