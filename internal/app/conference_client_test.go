package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

const clientTestTimeout = 10 * time.Second

type clientTestPeer struct {
	transport.PeerManager
	transport.ConferenceMedia
	onTrack func(transport.RTPTrackInfo, <-chan *rtp.Packet)
	send    func(string, any) error
	answer  func(webrtc.SessionDescription) (webrtc.SessionDescription, error)
}

func (p *clientTestPeer) OnRTPTrack(fn func(transport.RTPTrackInfo, <-chan *rtp.Packet)) {
	p.onTrack = fn
}
func (p *clientTestPeer) SendControl(action string, data any) error { return p.send(action, data) }
func (p *clientTestPeer) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	return p.answer(offer)
}

func clientTestConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.SampleRate, cfg.Channels, cfg.AudioBufferFrames = 48000, 1, 1
	return cfg
}

func newClientTest(t *testing.T) (*conferenceClientSession, *clientTestPeer) {
	t.Helper()
	p := &clientTestPeer{send: func(string, any) error { return nil }}
	p.answer = func(webrtc.SessionDescription) (webrtc.SessionDescription, error) {
		return webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: "answer"}, nil
	}
	s, err := newConferenceClientSession(t.Context(), p, clientTestConfig(), nil)
	require.NoError(t, err)
	require.NotNil(t, p.onTrack)
	t.Cleanup(s.Close)
	return s, p
}

func clientTestControl(t *testing.T, s *conferenceClientSession, action string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	handled, err := s.HandleControl(action, data)
	require.True(t, handled)
	require.NoError(t, err)
}

func clientTestHello(t *testing.T, s *conferenceClientSession) {
	t.Helper()
	clientTestControl(t, s, ActionConferenceHello, ConferenceHello{Version: ConferenceVersion, SelfID: "alice"})
}

func clientTestState(t *testing.T, s *conferenceClientSession, sources ...ConferenceSourceState) {
	t.Helper()
	clientTestControl(t, s, ActionConferenceState, ConferenceStateMessage{Sources: sources, Routes: echowarp.AudioRouteState{SelfID: "alice"}})
}

func TestConferenceClientHelloAndOffer(t *testing.T) {
	s, p := newClientTest(t)
	var actions []string
	p.send = func(action string, data any) error {
		s.mu.Lock()
		s.mu.Unlock() // Reentrant transport calls must not find the lock held.
		actions = append(actions, action)
		if action == ActionConferenceAnswer {
			require.Equal(t, uint64(7), data.(ConferenceDescription).ID)
		}
		return nil
	}
	clientTestHello(t, s)
	clientTestControl(t, s, ActionConferenceOffer, ConferenceDescription{ID: 7, SDP: "offer"})
	require.Equal(t, []string{ActionConferenceHelloAck, ActionConferenceAnswer}, actions)
	_, err := s.HandleControl(ActionConferenceOffer, json.RawMessage(`{"id":7,"sdp":"stale"}`))
	require.Error(t, err)
	require.Error(t, context.Cause(s.ctx))
}

func TestConferenceClientInvalidControlsFail(t *testing.T) {
	cases := []struct{ action, data string }{
		{ActionConferenceHello, `{"version":2,"self_id":"alice"}`},
		{ActionConferenceHello, `{"version":1,"self_id":"server"}`},
		{ActionConferenceHello, `{"version":1,"self_id":"self"}`},
		{ActionConferenceHello, `{"version":1,"self_id":"bad id"}`},
		{ActionConferenceHello, `null`}, {ActionConferenceHello, `{"version":1,"extra":1}`},
		{ActionConferenceOffer, `{"id":1,"sdp":"offer"}`},
		{ActionConferenceState, `{"sources":[],"routes":{"self_id":"alice"}}`},
		{"conference_future", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.action+tc.data, func(t *testing.T) {
			s, _ := newClientTest(t)
			handled, err := s.HandleControl(tc.action, json.RawMessage(tc.data))
			require.True(t, handled)
			require.Error(t, err)
			require.Error(t, s.ctx.Err())
		})
	}
}

func TestConferenceClientHelloTimeoutStartsAtReady(t *testing.T) {
	s, _ := newClientTest(t)
	s.timeout = time.Millisecond
	require.NoError(t, s.ctx.Err())
	s.Ready()
	s.Ready()
	select {
	case <-s.ctx.Done():
	case <-time.After(clientTestTimeout):
		t.Fatal("hello deadline missing")
	}
	require.Contains(t, context.Cause(s.ctx).Error(), "hello timeout")
	compatible, _ := newClientTest(t)
	clientTestHello(t, compatible)
	compatible.timeout = time.Millisecond
	compatible.Ready()
	require.Never(t, func() bool { return compatible.ctx.Err() != nil }, 20*time.Millisecond, time.Millisecond)
}

func TestConferenceClientRouteAcknowledgmentAndReadback(t *testing.T) {
	s, p := newClientTest(t)
	clientTestHello(t, s)
	rule := echowarp.AudioRouteRule{Scope: conferenceReceive, Source: "bob", Recipient: conferenceSelf, Muted: true}
	authoritative := echowarp.AudioRouteState{SelfID: "alice", Rules: []echowarp.AudioRouteRule{{Scope: conferenceReceive, Source: "bob", Recipient: "alice", Muted: true}}}
	p.send = func(action string, payload any) error {
		r := payload.(conferenceClientRequest)
		if action == ActionConferenceRoute {
			require.Equal(t, rule, *r.Rule)
		} else {
			require.Nil(t, r.Rule)
		}
		clientTestControl(t, s, ActionConferenceRouteResult, ConferenceRouteResult{ID: r.ID, State: authoritative})
		return nil
	}
	require.NoError(t, s.SetAudioRoute(t.Context(), rule))
	state, err := s.AudioRoutes(t.Context())
	require.NoError(t, err)
	require.Equal(t, authoritative, state)
	state.Rules[0].Muted = false
	require.True(t, authoritative.Rules[0].Muted)
}

func TestConferenceClientRouteTimeoutAndCancellation(t *testing.T) {
	s, _ := newClientTest(t)
	clientTestHello(t, s)
	s.timeout = time.Millisecond
	_, err := s.AudioRoutes(t.Context())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Empty(t, s.pending)
	clientTestControl(t, s, ActionConferenceRouteResult, ConferenceRouteResult{ID: 1, State: echowarp.AudioRouteState{SelfID: "alice"}})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = s.AudioRoutes(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestConferenceClientPendingBoundAndClose(t *testing.T) {
	s, _ := newClientTest(t)
	clientTestHello(t, s)
	for range conferenceClientPendingLimit {
		_, _, err := s.reserveRequest()
		require.NoError(t, err)
	}
	_, _, err := s.reserveRequest()
	require.ErrorContains(t, err, "limit")
	reply := s.pending[1]
	done := make(chan error, 1)
	go func() { _, waitErr := s.awaitRouteResult(t.Context(), reply); done <- waitErr }()
	s.Close()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Empty(t, s.pending)
}

func TestConferenceClientRouteOwnership(t *testing.T) {
	s, p := newClientTest(t)
	clientTestHello(t, s)
	p.send = func(string, any) error { t.Error("unauthorized route was sent"); return nil }
	for _, rule := range []echowarp.AudioRouteRule{
		{Scope: conferenceAdmin, Source: "bob", Recipient: "alice"},
		{Scope: conferenceReceive, Source: "bob", Recipient: "carol"},
		{Scope: conferenceSend, Source: "bob", Recipient: "carol"},
	} {
		err := s.SetAudioRoute(t.Context(), rule)
		var structured *ewerrors.EchoWarpError
		require.ErrorAs(t, err, &structured)
		require.Equal(t, ewerrors.ErrAuthFailed, structured.Code)
	}
}

func TestConferenceClientNetworkFailures(t *testing.T) {
	s, p := newClientTest(t)
	clientTestHello(t, s)
	failure := errors.New("send failed")
	p.send = func(string, any) error { return failure }
	_, err := s.AudioRoutes(t.Context())
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, context.Cause(s.ctx), failure)
	other, peer := newClientTest(t)
	clientTestHello(t, other)
	peer.answer = func(webrtc.SessionDescription) (webrtc.SessionDescription, error) {
		return webrtc.SessionDescription{}, failure
	}
	_, err = other.HandleControl(ActionConferenceOffer, json.RawMessage(`{"id":1,"sdp":"offer"}`))
	require.ErrorIs(t, err, failure)
}

func TestConferenceClientRunBoundedAndClose(t *testing.T) {
	s, _ := newClientTest(t)
	out := make(chan []float32, 5)
	var calls atomic.Int32
	done := make(chan error, 1)
	go func() { done <- s.Run(out, func(pcm []float32) { pcm[0] = 1; calls.Add(1) }) }()
	require.Eventually(t, func() bool { return calls.Load() > int32(cap(out)) }, clientTestTimeout, time.Millisecond)
	require.Equal(t, cap(out), len(out))
	s.Close()
	require.ErrorIs(t, <-done, context.Canceled)
	for pcm := range out {
		require.Len(t, pcm, 960)
		require.Equal(t, float32(1), pcm[0])
	}
}
