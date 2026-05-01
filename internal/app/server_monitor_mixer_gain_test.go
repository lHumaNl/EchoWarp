package app

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestServerMonitorMixer_SourceGainMutesOnlySelectedSource(t *testing.T) {
	t.Parallel()
	mixer := newTestServerMonitorMixer()
	mixer.AddSource("client-a")
	mixer.AddSource("client-b")
	require.True(t, mixer.SetSourceGain("client-a", 0))

	mixer.SubmitSourceFrame("client-a", []float32{0.40, -0.40})
	mixer.SubmitSourceFrame("client-b", []float32{0.25, 0.50})

	got := mixer.MixOnce(context.Background())
	assertFloatFrame(t, []float32{tanh32(0.25), tanh32(0.50)}, got)
}

func TestServerMonitorMixer_PausedSourceIsOmittedFromMix(t *testing.T) {
	t.Parallel()
	mixer := newTestServerMonitorMixer()
	paused := &atomic.Bool{}
	paused.Store(true)
	mixer.AddSource("client-a", paused)
	mixer.AddSource("client-b")

	mixer.SubmitSourceFrame("client-a", []float32{0.40, -0.40})
	mixer.SubmitSourceFrame("client-b", []float32{0.25, 0.50})

	got := mixer.MixOnce(context.Background())
	assertFloatFrame(t, []float32{tanh32(0.25), tanh32(0.50)}, got)
}

func TestSetupAudioPipelineMultiReverse_WiresIncomingClientGain(t *testing.T) {
	t.Parallel()
	app, _ := newTestReverseMultiServer(t, nil)
	clientA := &multiClient{id: "client-a", nickname: "Client A"}
	clientB := &multiClient{id: "client-b", nickname: "Client B"}
	app.mu.Lock()
	app.clients[clientA.id] = clientA
	app.clients[clientB.id] = clientB
	app.mu.Unlock()
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	clientACtx, cancelA := context.WithCancel(serverCtx)
	clientBCtx, cancelB := context.WithCancel(serverCtx)
	defer cancelA()
	defer cancelB()

	_, err := app.setupAudioPipelineMulti(serverCtx, clientACtx, &serverMockPeerManager{}, transport.DirectionReceive, clientA.id, clientA, nil, nil, nil)
	require.NoError(t, err)
	_, err = app.setupAudioPipelineMulti(serverCtx, clientBCtx, &serverMockPeerManager{}, transport.DirectionReceive, clientB.id, clientB, nil, nil, nil)
	require.NoError(t, err)
	gainA := clientA.getIncomingGain()
	gainB := clientB.getIncomingGain()
	require.NotNil(t, gainA)
	require.NotNil(t, gainB)
	require.Nil(t, clientA.getClientGain())

	app.adjustClientVolume(clientA.id, -1.0)

	assert.Equal(t, float32(0), gainA.Gain())
	assert.Equal(t, float32(1), gainB.Gain())
}

func newTestReverseMultiServer(t *testing.T, logger *slog.Logger) (*ServerApp, *atomic.Int32) {
	t.Helper()
	cfg := testServerConfig()
	cfg.Reverse = true
	cfg.MaxClients = 4
	cfg.Devices = []config.DeviceEntry{{ID: 2, Role: config.RolePlayback, Volume: 1.0}}
	if logger == nil {
		logger = testAppLogger()
	}
	app := NewServerApp(cfg, logger, nil, nil, nil)
	starts := &atomic.Int32{}
	app.monitorPlayerStart = func(ctx context.Context, _ *slog.Logger, _ config.Config, ch <-chan []float32, _ chan<- error, _ <-chan struct{}, startupCh chan<- error) {
		starts.Add(1)
		startupCh <- nil
		go drainMonitorPlayback(ctx, ch)
	}
	return app, starts
}
