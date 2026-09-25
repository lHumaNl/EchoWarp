package echowarp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

type audioRouteFakeRunner struct {
	set func(context.Context, AudioRouteRule) error
	get func(context.Context) (AudioRouteState, error)
}

func (r *audioRouteFakeRunner) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (r *audioRouteFakeRunner) SetAudioRoute(ctx context.Context, rule AudioRouteRule) error {
	return r.set(ctx, rule)
}

func (r *audioRouteFakeRunner) AudioRoutes(ctx context.Context) (AudioRouteState, error) {
	return r.get(ctx)
}

var _ AudioRouteController = (*audioRouteFakeRunner)(nil)

func audioRouteTestNode(runner Runner, mode Mode) *Node {
	n := &Node{runner: runner, config: NodeConfig{Mode: mode}}
	n.setState(StatusStreaming)
	return n
}

var validAudioRouteRules = []AudioRouteRule{
	{Scope: "receive", Source: "client-2", Recipient: "self", Muted: true},
	{Scope: "receive", Source: "*", Recipient: "self", Muted: true},
	{Scope: "receive", Source: "server", Recipient: "self", Muted: false},
	{Scope: "send", Source: "self", Recipient: "client-2", Muted: true},
	{Scope: "send", Source: "self", Recipient: "*", Muted: false},
	{Scope: "admin", Source: "client-1", Recipient: "client-2", Muted: true},
	{Scope: "admin", Source: "server", Recipient: "*", Muted: true},
	{Scope: "admin", Source: "*", Recipient: "server", Muted: false},
	{Scope: "admin", Source: "*", Recipient: "*", Muted: false},
	{Scope: "receive", Source: strings.Repeat("a", audioRouteMaxID), Recipient: "self"},
	{Scope: "receive", Source: "A_1.b:c-d", Recipient: "self"},
}

func TestAudioRouteNodeDelegatesExactRule(t *testing.T) {
	for _, rule := range validAudioRouteRules {
		t.Run(rule.Scope+"/"+rule.Source+"/"+rule.Recipient, func(t *testing.T) {
			calls := 0
			runner := &audioRouteFakeRunner{set: func(ctx context.Context, got AudioRouteRule) error {
				assert.Equal(t, rule, got)
				assert.Equal(t, t.Context(), ctx)
				calls++
				return nil
			}}
			node := audioRouteTestNode(runner, ModeServer)
			require.NoError(t, node.SetAudioRoute(t.Context(), rule))
			require.NoError(t, node.SetAudioRoute(t.Context(), rule))
			assert.Equal(t, 2, calls, "idempotence and exact removal belong to the runtime")
		})
	}
}

var invalidAudioRouteRules = []struct {
	name string
	rule AudioRouteRule
	code string
}{
	{"empty", AudioRouteRule{}, ewerrors.ErrConfigValidation},
	{"unknown scope", AudioRouteRule{"all", "*", "*", true}, ewerrors.ErrConfigValidation},
	{"scope case", AudioRouteRule{"Receive", "*", "self", true}, ewerrors.ErrConfigValidation},
	{"empty source", AudioRouteRule{"receive", "", "self", true}, ewerrors.ErrConfigValidation},
	{"empty recipient", AudioRouteRule{"send", "self", "", true}, ewerrors.ErrConfigValidation},
	{"long source", AudioRouteRule{"receive", strings.Repeat("x", audioRouteMaxID+1), "self", true}, ewerrors.ErrConfigValidation},
	{"long recipient", AudioRouteRule{"send", "self", strings.Repeat("x", audioRouteMaxID+1), true}, ewerrors.ErrConfigValidation},
	{"space", AudioRouteRule{"receive", "client 1", "self", true}, ewerrors.ErrConfigValidation},
	{"control", AudioRouteRule{"receive", "client\n1", "self", true}, ewerrors.ErrConfigValidation},
	{"slash", AudioRouteRule{"send", "self", "client/1", true}, ewerrors.ErrConfigValidation},
	{"unicode", AudioRouteRule{"receive", "\u00e9", "self", true}, ewerrors.ErrConfigValidation},
	{"partial wildcard", AudioRouteRule{"receive", "client*", "self", true}, ewerrors.ErrConfigValidation},
	{"receive spoof", AudioRouteRule{"receive", "*", "client-2", true}, ewerrors.ErrAuthFailed},
	{"receive wildcard owner", AudioRouteRule{"receive", "*", "*", true}, ewerrors.ErrAuthFailed},
	{"send spoof", AudioRouteRule{"send", "client-2", "*", true}, ewerrors.ErrAuthFailed},
	{"send wildcard owner", AudioRouteRule{"send", "*", "*", true}, ewerrors.ErrAuthFailed},
}

func TestAudioRouteNodeRejectsInvalidRules(t *testing.T) {
	node := audioRouteTestNode(&audioRouteFakeRunner{}, ModeClient)
	for _, tc := range invalidAudioRouteRules {
		t.Run(tc.name, func(t *testing.T) {
			err := node.SetAudioRoute(t.Context(), tc.rule)
			assert.Equal(t, tc.code, ewerrors.GetCode(err))
		})
	}
}

func TestAudioRouteNodeRejectsClientAdmin(t *testing.T) {
	node := audioRouteTestNode(&audioRouteFakeRunner{}, ModeClient)
	for _, muted := range []bool{true, false} {
		err := node.SetAudioRoute(t.Context(), AudioRouteRule{"admin", "*", "*", muted})
		assert.Equal(t, ewerrors.ErrAuthFailed, ewerrors.GetCode(err))
	}
}

func assertAudioRouteNodeError(t *testing.T, node *Node, code string) {
	t.Helper()
	err := node.SetAudioRoute(t.Context(), validAudioRouteRules[0])
	assert.Equal(t, code, ewerrors.GetCode(err))
	state, err := node.AudioRoutes(t.Context())
	assert.Equal(t, code, ewerrors.GetCode(err))
	assert.Equal(t, AudioRouteState{}, state)
}

func TestAudioRouteNodeLifecycleErrors(t *testing.T) {
	for _, status := range []NodeStatus{StatusIdle, StatusStopped} {
		node := audioRouteTestNode(&audioRouteFakeRunner{}, ModeClient)
		node.setState(status)
		assertAudioRouteNodeError(t, node, ewerrors.ErrNotRunning)
	}
	assertAudioRouteNodeError(t, audioRouteTestNode(nil, ModeClient), ewerrors.ErrNotRunning)
	plain := &plainToggleRunner{}
	assertAudioRouteNodeError(t, audioRouteTestNode(plain, ModeClient), ewerrors.ErrInternalState)
}

func TestAudioRouteNodeAllowsPausedController(t *testing.T) {
	runner := &audioRouteFakeRunner{set: func(context.Context, AudioRouteRule) error { return nil }}
	node := audioRouteTestNode(runner, ModeClient)
	node.setState(StatusPaused)
	require.NoError(t, node.SetAudioRoute(t.Context(), validAudioRouteRules[0]))
}

func TestAudioRouteNodeReadbackSnapshot(t *testing.T) {
	want := AudioRouteState{SelfID: "client-1", Rules: append([]AudioRouteRule{}, validAudioRouteRules...)}
	runner := &audioRouteFakeRunner{get: func(ctx context.Context) (AudioRouteState, error) {
		assert.Equal(t, t.Context(), ctx)
		return want, nil
	}}
	got, err := audioRouteTestNode(runner, ModeClient).AudioRoutes(t.Context())
	require.NoError(t, err)
	assert.Equal(t, want, got)
	got.Rules[0].Source = "changed"
	assert.NotEqual(t, want.Rules[0], got.Rules[0])
}

func TestAudioRouteNodeEmptyReadback(t *testing.T) {
	runner := &audioRouteFakeRunner{get: func(context.Context) (AudioRouteState, error) {
		return AudioRouteState{SelfID: "server"}, nil
	}}
	got, err := audioRouteTestNode(runner, ModeServer).AudioRoutes(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "server", got.SelfID)
	assert.NotNil(t, got.Rules)
	assert.Empty(t, got.Rules)
}

func TestAudioRouteNodePropagatesErrors(t *testing.T) {
	want := ewerrors.NewError(ewerrors.ErrAuthFailed, "runtime rejected identity")
	runner := &audioRouteFakeRunner{
		set: func(context.Context, AudioRouteRule) error { return want },
		get: func(context.Context) (AudioRouteState, error) { return AudioRouteState{}, want },
	}
	node := audioRouteTestNode(runner, ModeClient)
	assert.ErrorIs(t, node.SetAudioRoute(t.Context(), validAudioRouteRules[0]), want)
	_, err := node.AudioRoutes(t.Context())
	assert.ErrorIs(t, err, want)
}

func TestAudioRouteNodePreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	node := audioRouteTestNode(&audioRouteFakeRunner{}, ModeClient)
	assert.ErrorIs(t, node.SetAudioRoute(ctx, validAudioRouteRules[0]), context.Canceled)
	_, err := node.AudioRoutes(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func assertAudioRouteLockReleased(t *testing.T, node *Node) {
	t.Helper()
	if !node.mu.TryLock() {
		t.Error("Node lock held during controller callback")
		return
	}
	node.mu.Unlock()
}

func TestAudioRouteNodeCallbacksDoNotHoldLock(t *testing.T) {
	runner := &audioRouteFakeRunner{}
	node := audioRouteTestNode(runner, ModeClient)
	runner.set = func(context.Context, AudioRouteRule) error {
		assertAudioRouteLockReleased(t, node)
		return nil
	}
	runner.get = func(context.Context) (AudioRouteState, error) {
		assertAudioRouteLockReleased(t, node)
		return AudioRouteState{}, nil
	}
	require.NoError(t, node.SetAudioRoute(t.Context(), validAudioRouteRules[0]))
	_, err := node.AudioRoutes(t.Context())
	require.NoError(t, err)
}

func audioRouteWaiter(entered chan<- struct{}, ack <-chan error) func(context.Context, AudioRouteRule) error {
	return func(ctx context.Context, _ AudioRouteRule) error {
		close(entered)
		select {
		case err := <-ack:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func TestAudioRouteNodeWaitsForAcknowledgment(t *testing.T) {
	entered, ack := make(chan struct{}), make(chan error)
	runner := &audioRouteFakeRunner{set: audioRouteWaiter(entered, ack)}
	node := audioRouteTestNode(runner, ModeClient)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- node.SetAudioRoute(ctx, validAudioRouteRules[0]) }()
	<-entered
	select {
	case err := <-done:
		t.Fatalf("returned before acknowledgment: %v", err)
	default:
	}
	want := errors.New("server refused rule")
	ack <- want
	assert.ErrorIs(t, <-done, want)
}

func TestAudioRouteNodeCancellationDuringCallback(t *testing.T) {
	entered := make(chan struct{})
	runner := &audioRouteFakeRunner{set: audioRouteWaiter(entered, nil)}
	node := audioRouteTestNode(runner, ModeClient)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- node.SetAudioRoute(ctx, validAudioRouteRules[0]) }()
	<-entered
	cancel()
	assert.ErrorIs(t, <-done, context.Canceled)
}

func TestAudioRouteNodeReadCancellationDuringCallback(t *testing.T) {
	entered := make(chan struct{})
	runner := &audioRouteFakeRunner{get: func(ctx context.Context) (AudioRouteState, error) {
		close(entered)
		<-ctx.Done()
		return AudioRouteState{}, ctx.Err()
	}}
	node := audioRouteTestNode(runner, ModeClient)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := node.AudioRoutes(ctx); done <- err }()
	<-entered
	cancel()
	assert.ErrorIs(t, <-done, context.Canceled)
}
