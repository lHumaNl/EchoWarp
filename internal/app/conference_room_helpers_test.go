package app

import (
	"errors"
	"testing"
)

func roomTestServer(t *testing.T) (*ConferenceRoom, *roomTestPeer) {
	t.Helper()
	r := newRoomTest(t)
	r.AddServerSource()
	p := addRoomTestPeer(t, r, "alice")
	roomTestStart(t, r, "alice", p)
	return r, p
}

func roomTestPair(t *testing.T) (*ConferenceRoom, *roomTestPeer, *roomTestPeer) {
	t.Helper()
	r := newRoomTest(t)
	a := addRoomTestPeer(t, r, "alice")
	b := addRoomTestPeer(t, r, "bob")
	roomTestStart(t, r, "alice", a)
	roomTestStart(t, r, "bob", b)
	return r, a, b
}

func roomTestBlockWrites(p *roomTestPeer) chan struct{} {
	gate := make(chan struct{})
	p.mu.Lock()
	p.writeGate = gate
	p.mu.Unlock()
	return gate
}

func roomTestAwaitGate(gate, closed <-chan struct{}) error {
	if gate == nil {
		return nil
	}
	select {
	case <-gate:
		return nil
	case <-closed:
		return errors.New("closed")
	}
}
