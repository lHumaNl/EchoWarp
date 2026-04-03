package app

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recvWithTimeout reads a ChatMessage from ch or fails after timeout.
func recvWithTimeout(t *testing.T, ch <-chan ChatMessage, timeout time.Duration) ChatMessage {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for ChatMessage")
		return ChatMessage{}
	}
}

// e2eClient wires a ChatClient to a ChatHub via an in-memory byte channel,
// simulating a DataChannel. It collects all messages delivered to onMessage.
type e2eClient struct {
	client *ChatClient
	dc     chan []byte // simulated DataChannel
	msgs   chan ChatMessage
	done   chan struct{}
}

func newE2EClient(nickname string) *e2eClient {
	msgCh := make(chan ChatMessage, 64)
	c := &e2eClient{
		client: NewChatClient(nickname, func(msg ChatMessage) {
			msgCh <- msg
		}),
		dc:   make(chan []byte, 64),
		msgs: msgCh,
		done: make(chan struct{}),
	}
	// Wire the client's send function to write raw JSON into dc (simulates DC.Send).
	c.client.SetSendFn(func(data []byte) error {
		c.dc <- data
		return nil
	})
	return c
}

// startReader reads from the simulated DC and feeds bytes into ChatClient.HandleIncoming.
func (e *e2eClient) startReader() {
	go func() {
		for {
			select {
			case raw := <-e.dc:
				e.client.HandleIncoming(raw)
			case <-e.done:
				return
			}
		}
	}()
}

func (e *e2eClient) stop() {
	close(e.done)
}

// drainAll collects all messages available in msgs within a short window.
func (e *e2eClient) drainAll(timeout time.Duration) []ChatMessage {
	var out []ChatMessage
	deadline := time.After(timeout)
	for {
		select {
		case m := <-e.msgs:
			out = append(out, m)
		case <-deadline:
			return out
		}
	}
}

// ── Test 1 ─────────────────────────────────────────────────────────────────

func TestChatE2E_HubToClient(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	alice := newE2EClient("Alice")
	bob := newE2EClient("Bob")
	defer alice.stop()
	defer bob.stop()

	// Start readers that pipe DC bytes into ChatClient.HandleIncoming.
	alice.startReader()
	bob.startReader()

	// Register both — sendFn writes JSON into their DC channel (hub → DC → client).
	hub.Register("alice", "Alice", func(data []byte) error {
		alice.dc <- data
		return nil
	})
	hub.Register("bob", "Bob", func(data []byte) error {
		bob.dc <- data
		return nil
	})

	// Give readers time to process registration messages (history + join notifications).
	time.Sleep(100 * time.Millisecond)

	// Drain registration messages so they don't interfere.
	alice.drainAll(100 * time.Millisecond)
	bob.drainAll(100 * time.Millisecond)

	// Alice sends a message via the hub.
	raw, err := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: "Hello Bob!"})
	require.NoError(t, err)
	hub.HandleIncoming("alice", raw)

	// Bob should receive it.
	bobMsg := recvWithTimeout(t, bob.msgs, 2*time.Second)
	assert.Equal(t, ChatActionMsg, bobMsg.Action)
	assert.Equal(t, "Alice", bobMsg.From)
	assert.Equal(t, "Hello Bob!", bobMsg.Text)

	// Alice should NOT receive her own message. Wait briefly and check.
	select {
	case m := <-alice.msgs:
		t.Fatalf("Alice should not receive her own message, got: %+v", m)
	case <-time.After(200 * time.Millisecond):
		// expected
	}
}

// ── Test 2 ─────────────────────────────────────────────────────────────────

func TestChatE2E_HistoryOnJoin(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	alice := newE2EClient("Alice")
	defer alice.stop()
	alice.startReader()

	hub.Register("alice", "Alice", func(data []byte) error {
		alice.dc <- data
		return nil
	})

	// Drain Alice's registration messages.
	alice.drainAll(100 * time.Millisecond)

	// Alice sends 5 messages.
	for i := 0; i < 5; i++ {
		raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: "msg"})
		hub.HandleIncoming("alice", raw)
	}

	// Now Bob joins.
	bob := newE2EClient("Bob")
	defer bob.stop()
	bob.startReader()

	hub.Register("bob", "Bob", func(data []byte) error {
		bob.dc <- data
		return nil
	})

	// Bob receives: history messages (via onMessage for each) + "Bob joined" system message.
	msgs := bob.drainAll(500 * time.Millisecond)

	// Count history chat messages and look for "Bob joined".
	chatCount := 0
	joinFound := false
	for _, m := range msgs {
		if m.Action == ChatActionMsg && m.From == "Alice" {
			chatCount++
		}
		if m.Action == ChatActionSystem && m.Text == "Bob joined" {
			joinFound = true
		}
	}

	assert.Equal(t, 5, chatCount, "Bob should receive 5 history chat messages")
	assert.True(t, joinFound, "Bob should receive 'Bob joined' system message")
}

// ── Test 3 ─────────────────────────────────────────────────────────────────

func TestChatE2E_ServerMessage(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	alice := newE2EClient("Alice")
	bob := newE2EClient("Bob")
	defer alice.stop()
	defer bob.stop()
	alice.startReader()
	bob.startReader()

	hub.Register("alice", "Alice", func(data []byte) error {
		alice.dc <- data
		return nil
	})
	hub.Register("bob", "Bob", func(data []byte) error {
		bob.dc <- data
		return nil
	})

	// Drain registration messages.
	alice.drainAll(100 * time.Millisecond)
	bob.drainAll(100 * time.Millisecond)

	hub.SendFromServer("Hello everyone")

	aliceMsg := recvWithTimeout(t, alice.msgs, 2*time.Second)
	bobMsg := recvWithTimeout(t, bob.msgs, 2*time.Second)

	for _, msg := range []ChatMessage{aliceMsg, bobMsg} {
		assert.Equal(t, ChatActionMsg, msg.Action)
		assert.Equal(t, "Server", msg.From)
		assert.Equal(t, "Hello everyone", msg.Text)
	}
}

// ── Test 4 ─────────────────────────────────────────────────────────────────

// TestChatE2E_FullDCFlow simulates the full DataChannel round-trip using
// in-memory channels (same logical flow as real WebRTC DCs, without pion dependency).
//
// Flow: ClientSend → DC → Hub.HandleIncoming → Hub.broadcast → DC → Client.HandleIncoming
func TestChatE2E_FullDCFlow(t *testing.T) {
	// Simulated DCs: serverInbox receives from client, clientInbox receives from server.
	serverInbox := make(chan []byte, 64)
	clientInbox := make(chan []byte, 64)

	hub := NewChatHub(chatTestLogger(), 5, nil)

	// Client-side ChatClient — onMessage collects received messages.
	received := make(chan ChatMessage, 64)
	client := NewChatClient("Tester", func(msg ChatMessage) {
		received <- msg
	})

	// Client's sendFn writes to serverInbox (simulates DC.Send from client → server).
	client.SetSendFn(func(data []byte) error {
		serverInbox <- data
		return nil
	})

	// Register the "peer" in the hub; hub's sendFn writes to clientInbox (server → client DC).
	hub.Register("peer1", "Tester", func(data []byte) error {
		clientInbox <- data
		return nil
	})

	// Goroutine: reads from clientInbox → client.HandleIncoming (simulates client DC.OnMessage).
	var wg sync.WaitGroup
	done := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case raw := <-clientInbox:
				client.HandleIncoming(raw)
			case <-done:
				return
			}
		}
	}()

	// Goroutine: reads from serverInbox → hub.HandleIncoming (simulates server DC.OnMessage).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case raw := <-serverInbox:
				hub.HandleIncoming("peer1", raw)
			case <-done:
				return
			}
		}
	}()

	defer func() {
		close(done)
		wg.Wait()
	}()

	// Drain registration messages (history + "Tester joined").
	drainTimeout := time.After(300 * time.Millisecond)
	for {
		select {
		case <-received:
		case <-drainTimeout:
			goto sendPhase
		}
	}

sendPhase:
	// Client sends a message through the full loop.
	err := client.Send("round-trip test")
	require.NoError(t, err)

	// The hub sees this from serverInbox, broadcasts back through clientInbox.
	// But the sender is excluded from broadcast — so we need a second participant
	// OR we verify the hub processes it. Since there's only one participant and
	// the hub excludes the sender, the message won't come back to the same client.
	// Let's add a second "observer" to verify the message went through the hub.

	observerMsgs := make(chan ChatMessage, 64)
	observer := newE2EClient("Observer")
	defer observer.stop()

	hub.Register("observer", "Observer", func(data []byte) error {
		observer.dc <- data
		return nil
	})
	observer.startReader()

	// Drain observer's registration messages.
	observer.drainAll(200 * time.Millisecond)

	// Client sends another message.
	err = client.Send("hello observer")
	require.NoError(t, err)

	// Observer should receive it via the hub broadcast.
	obsMsg := recvWithTimeout(t, observer.msgs, 2*time.Second)
	assert.Equal(t, ChatActionMsg, obsMsg.Action)
	assert.Equal(t, "Tester", obsMsg.From)
	assert.Equal(t, "hello observer", obsMsg.Text)

	// Verify original client did NOT get its own message echoed back.
	select {
	case m := <-received:
		// Could be "Observer joined" system message — that's fine, skip it.
		if m.Action == ChatActionMsg && m.Text == "hello observer" {
			t.Fatal("client should not receive its own message")
		}
	case <-time.After(200 * time.Millisecond):
		// expected
	}

	_ = observerMsgs // suppress unused warning
}
