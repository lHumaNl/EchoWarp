package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

func roomTestRule(scope, source, recipient string, muted bool) echowarp.AudioRouteRule {
	return echowarp.AudioRouteRule{Scope: scope, Source: source, Recipient: recipient, Muted: muted}
}

func roomTestAllowed(r *ConferenceRoom, source, recipient string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.allowedLocked(source, recipient)
}

func TestConferenceRoutingDenyLayers(t *testing.T) {
	rules := []echowarp.AudioRouteRule{
		roomTestRule(conferenceReceive, "alice", conferenceSelf, true), roomTestRule(conferenceReceive, "*", conferenceSelf, true),
		roomTestRule(conferenceSend, conferenceSelf, "bob", true), roomTestRule(conferenceSend, conferenceSelf, "*", true),
		roomTestRule(conferenceAdmin, "alice", "bob", true), roomTestRule(conferenceAdmin, "alice", "*", true),
		roomTestRule(conferenceAdmin, "*", "bob", true), roomTestRule(conferenceAdmin, "*", "*", true),
	}
	for _, rule := range rules {
		t.Run(conferenceRuleOrder(rule), func(t *testing.T) { roomTestDenyLayer(t, rule) })
	}
}

func roomTestDenyLayer(t *testing.T, rule echowarp.AudioRouteRule) {
	t.Helper()
	r := newRoomTest(t)
	addRoomTestPeer(t, r, "alice")
	addRoomTestPeer(t, r, "bob")
	actor := map[string]string{conferenceReceive: "bob", conferenceSend: "alice", conferenceAdmin: ConferenceServerID}[rule.Scope]
	require.True(t, roomTestAllowed(r, "alice", "bob"))
	require.False(t, roomTestAllowed(r, "alice", "alice"))
	require.NoError(t, r.SetRule(actor, actor == ConferenceServerID, rule))
	require.False(t, roomTestAllowed(r, "alice", "bob"))
	reverseAllowed := rule.Scope != conferenceAdmin || rule.Source != "*" || rule.Recipient != "*"
	require.Equal(t, reverseAllowed, roomTestAllowed(r, "bob", "alice"))
	other := roomTestRule(conferenceReceive, "alice", conferenceSelf, false)
	if rule.Scope == conferenceReceive {
		other = roomTestRule(conferenceSend, conferenceSelf, "bob", false)
		actor = "alice"
	} else {
		actor = "bob"
	}
	require.NoError(t, r.SetRule(actor, false, other))
	require.False(t, roomTestAllowed(r, "alice", "bob"))
	actor = map[string]string{conferenceReceive: "bob", conferenceSend: "alice", conferenceAdmin: ConferenceServerID}[rule.Scope]
	rule.Muted = false
	require.NoError(t, r.SetRule(actor, actor == ConferenceServerID, rule))
	require.True(t, roomTestAllowed(r, "alice", "bob"))
}

func TestConferenceRoutingExactRemovalAndLegacyGates(t *testing.T) {
	r := newRoomTest(t)
	addRoomTestPeer(t, r, "alice")
	addRoomTestPeer(t, r, "bob")
	require.NoError(t, r.SetRule("bob", false, roomTestRule(conferenceReceive, "*", conferenceSelf, true)))
	require.NoError(t, r.SetRule("bob", false, roomTestRule(conferenceReceive, "alice", conferenceSelf, false)))
	require.False(t, roomTestAllowed(r, "alice", "bob"))
	r.SetSourceBlocked("alice", true)
	r.SetSourcePaused("alice", true)
	r.SetRecipientBlocked("bob", true)
	require.NoError(t, r.SetRule("bob", false, roomTestRule(conferenceReceive, "*", conferenceSelf, false)))
	r.SetSourceBlocked("alice", false)
	require.False(t, roomTestAllowed(r, "alice", "bob"))
	r.SetSourcePaused("alice", false)
	require.False(t, roomTestAllowed(r, "alice", "bob"))
	r.SetRecipientBlocked("bob", false)
	require.True(t, roomTestAllowed(r, "alice", "bob"))
}

func TestConferenceRoutingAuthorization(t *testing.T) {
	r := newRoomTest(t)
	addRoomTestPeer(t, r, "alice")
	addRoomTestPeer(t, r, "bob")
	for _, rule := range []echowarp.AudioRouteRule{
		roomTestRule(conferenceAdmin, "alice", "bob", true), roomTestRule(conferenceSend, "bob", "alice", true),
		roomTestRule(conferenceReceive, "alice", "bob", true), roomTestRule(conferenceReceive, "missing", conferenceSelf, true),
		roomTestRule(conferenceSend, conferenceSelf, "bad id", true), roomTestRule("bad", conferenceSelf, "bob", true),
		roomTestRule(conferenceSend, conferenceSelf, strings.Repeat("a", conferenceMaxID+1), true),
	} {
		require.Error(t, r.SetRule("alice", false, rule))
	}
	require.Error(t, r.SetRule("alice", true, roomTestRule(conferenceAdmin, "*", "*", true)))
	require.Error(t, r.SetRule("missing", false, roomTestRule(conferenceSend, conferenceSelf, "bob", true)))
	require.NoError(t, r.SetRule(ConferenceServerID, true, roomTestRule(conferenceSend, conferenceSelf, "*", true)))
	require.NoError(t, r.SetRule(ConferenceServerID, true, roomTestRule(conferenceReceive, "*", conferenceSelf, true)))
}

func TestConferenceRoutingStatePrivacyAndLeave(t *testing.T) {
	r := newRoomTest(t)
	addRoomTestPeer(t, r, "alice")
	addRoomTestPeer(t, r, "bob")
	addRoomTestPeer(t, r, "carol")
	require.NoError(t, r.SetRule("bob", false, roomTestRule(conferenceReceive, "alice", conferenceSelf, true)))
	require.NoError(t, r.SetRule("alice", false, roomTestRule(conferenceSend, conferenceSelf, "*", true)))
	require.NoError(t, r.SetRule(ConferenceServerID, true, roomTestRule(conferenceAdmin, "bob", "carol", true)))
	require.NoError(t, r.SetRule(ConferenceServerID, true, roomTestRule(conferenceAdmin, "*", "*", true)))
	require.Len(t, r.State("alice", false).Rules, 2)
	require.Len(t, r.State(ConferenceServerID, true).Rules, 4)
	state := r.State("alice", false)
	state.Rules[0].Muted = false
	require.True(t, r.State("alice", false).Rules[0].Muted)
	r.RemovePeer("alice")
	require.Len(t, r.State(ConferenceServerID, true).Rules, 2)
	addRoomTestPeer(t, r, "alice")
	require.Len(t, r.State("alice", false).Rules, 1)
}

func TestConferenceRoutingStrictControl(t *testing.T) {
	r, p := roomTestServer(t)
	for _, payload := range []string{`null`, `{}`, `{"id":1}`, `{"id":1,"rule":{"scope":"receive","source":"server","recipient":"self"}}`,
		`{"id":1,"trusted":true}`, `{"id":1,"rule":{"scope":"admin","source":"*","recipient":"*","muted":true}}`,
		`{"id":1,"rule":{"scope":"receive","source":"server","recipient":"self","muted":null}}`, `{"id":1} {}`} {
		_, err := r.HandleControl("alice", ActionConferenceRoute, json.RawMessage(payload))
		require.Error(t, err)
		var result ConferenceRouteResult
		require.NoError(t, json.Unmarshal(roomTestReceive(t, p, ActionConferenceRouteResult), &result))
		require.NotEmpty(t, result.Error)
	}
	roomTestRouteReadback(t, r, p)
}

func roomTestRouteReadback(t *testing.T, r *ConferenceRoom, p *roomTestPeer) {
	t.Helper()
	payload := json.RawMessage(`{"id":7,"rule":{"scope":"receive","source":"server","recipient":"self","muted":true}}`)
	handled, err := r.HandleControl("alice", ActionConferenceRoute, payload)
	require.True(t, handled)
	require.NoError(t, err)
	require.False(t, roomTestAllowed(r, ConferenceServerID, "alice"))
	roomTestReceive(t, p, ActionConferenceRouteResult)
	roomTestHandle(t, r, "alice", ActionConferenceRoutes, map[string]uint64{"id": 8})
	var result ConferenceRouteResult
	require.NoError(t, json.Unmarshal(roomTestReceive(t, p, ActionConferenceRouteResult), &result))
	require.Equal(t, uint64(8), result.ID)
	require.Empty(t, result.Error)
	require.Len(t, result.State.Rules, 1)
}

func TestConferenceRoutingSlowRecipientAndInvalidation(t *testing.T) {
	r := newRoomTest(t)
	r.AddServerSource()
	slow := addRoomTestPeer(t, r, "slow")
	fast := addRoomTestPeer(t, r, "fast")
	roomTestStart(t, r, "slow", slow)
	roomTestStart(t, r, "fast", fast)
	gate := roomTestBlockWrites(slow)
	r.WriteSource(ConferenceServerID, roomTestRTP(1))
	roomTestReadPacket(t, fast)
	roomTestWait(t, slow.entered)
	r.WriteSource(ConferenceServerID, roomTestRTP(2))
	roomTestReadPacket(t, fast)
	require.NoError(t, r.SetRule("slow", false, roomTestRule(conferenceReceive, ConferenceServerID, conferenceSelf, true)))
	require.NoError(t, r.SetRule("slow", false, roomTestRule(conferenceReceive, ConferenceServerID, conferenceSelf, false)))
	close(gate)
	require.Equal(t, uint16(1), roomTestReadPacket(t, slow).packet.SequenceNumber)
	r.WriteSource(ConferenceServerID, roomTestRTP(3))
	require.Equal(t, uint16(3), roomTestReadPacket(t, slow).packet.SequenceNumber)
	require.Empty(t, slow.written)
}

func TestConferenceRoutingPeerValidation(t *testing.T) {
	r := newRoomTest(t)
	for _, id := range []string{"", ConferenceServerID, conferenceSelf, "*", "bad id", strings.Repeat("a", conferenceMaxID+1)} {
		require.Error(t, r.AddPeer(context.Background(), id, newRoomTestPeer()))
	}
	require.Error(t, r.AddPeer(context.Background(), "alice", nil))
	addRoomTestPeer(t, r, "alice")
	require.Error(t, r.AddPeer(context.Background(), "alice", newRoomTestPeer()))
	r.Close()
	require.Error(t, r.AddPeer(context.Background(), "bob", newRoomTestPeer()))
}

func TestConferenceRoutingPolicyChangePreservesOtherRecipientsQueue(t *testing.T) {
	r := newRoomTest(t)
	r.AddServerSource()
	slow := addRoomTestPeer(t, r, "slow")
	other := addRoomTestPeer(t, r, "other")
	roomTestStart(t, r, "slow", slow)
	roomTestStart(t, r, "other", other)
	gate := roomTestBlockWrites(slow)
	r.WriteSource(ConferenceServerID, roomTestRTP(1))
	roomTestWait(t, slow.entered)
	r.WriteSource(ConferenceServerID, roomTestRTP(2))
	for range 20 {
		require.NoError(t, r.SetRule("other", false, roomTestRule(conferenceReceive, ConferenceServerID, conferenceSelf, true)))
		require.NoError(t, r.SetRule("other", false, roomTestRule(conferenceReceive, ConferenceServerID, conferenceSelf, false)))
	}
	close(gate)
	require.Equal(t, uint16(1), roomTestReadPacket(t, slow).packet.SequenceNumber)
	require.Equal(t, uint16(2), roomTestReadPacket(t, slow).packet.SequenceNumber)
}
