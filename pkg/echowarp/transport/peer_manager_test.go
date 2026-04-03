package transport

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

func TestMultiPeerManager_AddPeer_Success(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	peer, err := m.AddPeer("client1", ICEConfig{})
	if err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}
	if peer == nil {
		t.Fatal("expected peer to be returned")
	}
	if m.PeerCount() != 1 {
		t.Errorf("expected PeerCount=1, got %d", m.PeerCount())
	}
}

func TestMultiPeerManager_AddPeer_DuplicateID_Fails(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	_, err := m.AddPeer("client1", ICEConfig{})
	if err != nil {
		t.Fatalf("first AddPeer failed: %v", err)
	}

	_, err = m.AddPeer("client1", ICEConfig{})
	if err != ErrPeerExists {
		t.Errorf("expected ErrPeerExists, got %v", err)
	}
}

func TestMultiPeerManager_RemovePeer_ClosesConnection(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	_, err := m.AddPeer("client1", ICEConfig{})
	if err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}

	err = m.RemovePeer("client1")
	if err != nil {
		t.Fatalf("RemovePeer failed: %v", err)
	}

	if m.PeerCount() != 0 {
		t.Errorf("expected PeerCount=0, got %d", m.PeerCount())
	}
}

func TestMultiPeerManager_RemovePeer_NotFound_Error(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	err := m.RemovePeer("nonexistent")
	if err != ErrPeerNotFound {
		t.Errorf("expected ErrPeerNotFound, got %v", err)
	}
}

func TestMultiPeerManager_MaxPeers_RejectsExcess(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 2)
	defer m.Close()

	_, err := m.AddPeer("client1", ICEConfig{})
	if err != nil {
		t.Fatalf("AddPeer client1 failed: %v", err)
	}

	_, err = m.AddPeer("client2", ICEConfig{})
	if err != nil {
		t.Fatalf("AddPeer client2 failed: %v", err)
	}

	_, err = m.AddPeer("client3", ICEConfig{})
	if err != ErrMaxPeers {
		t.Errorf("expected ErrMaxPeers, got %v", err)
	}
}

func TestMultiPeerManager_Peers_ReturnsSortedIDs(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	_, _ = m.AddPeer("clientC", ICEConfig{})
	_, _ = m.AddPeer("clientA", ICEConfig{})
	_, _ = m.AddPeer("clientB", ICEConfig{})

	peers := m.Peers()
	expected := []string{"clientA", "clientB", "clientC"}

	if len(peers) != len(expected) {
		t.Fatalf("expected %d peers, got %d", len(expected), len(peers))
	}

	for i, id := range expected {
		if peers[i] != id {
			t.Errorf("expected peers[%d]=%s, got %s", i, id, peers[i])
		}
	}
}

func TestMultiPeerManager_Close_ClosesAll(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)

	_, _ = m.AddPeer("client1", ICEConfig{})
	_, _ = m.AddPeer("client2", ICEConfig{})
	_, _ = m.AddPeer("client3", ICEConfig{})

	if m.PeerCount() != 3 {
		t.Fatalf("expected PeerCount=3, got %d", m.PeerCount())
	}

	err := m.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if m.PeerCount() != 0 {
		t.Errorf("expected PeerCount=0 after Close, got %d", m.PeerCount())
	}
}

func TestMultiPeerManager_ConcurrentAccess(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	var wg sync.WaitGroup
	numOps := 100

	for i := 0; i < numOps; i++ {
		wg.Add(3)

		go func(idx int) {
			defer wg.Done()
			clientID := "client" + string(rune('0'+idx%10))
			_, _ = m.AddPeer(clientID, ICEConfig{})
		}(i)

		go func(idx int) {
			defer wg.Done()
			clientID := "client" + string(rune('0'+idx%10))
			_ = m.RemovePeer(clientID)
		}(i)

		go func() {
			defer wg.Done()
			_ = m.PeerCount()
			_ = m.Peers()
		}()
	}

	wg.Wait()
}

func TestMultiPeerManager_Metrics_AddPeer_UpdatesPoolSize(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	before := testutil.ToFloat64(metrics.ConnectionPoolSize)
	_, err := m.AddPeer("client1", ICEConfig{})
	if err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}
	after := testutil.ToFloat64(metrics.ConnectionPoolSize)

	if after != before+1 {
		t.Errorf("expected ConnectionPoolSize to increase by 1, got before=%v, after=%v", before, after)
	}
}

func TestMultiPeerManager_Metrics_RemovePeer_UpdatesPoolSize(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)
	defer m.Close()

	_, _ = m.AddPeer("client1", ICEConfig{})
	before := testutil.ToFloat64(metrics.ConnectionPoolSize)

	err := m.RemovePeer("client1")
	if err != nil {
		t.Fatalf("RemovePeer failed: %v", err)
	}
	after := testutil.ToFloat64(metrics.ConnectionPoolSize)

	if after != before-1 {
		t.Errorf("expected ConnectionPoolSize to decrease by 1, got before=%v, after=%v", before, after)
	}
}

func TestMultiPeerManager_Metrics_Close_UpdatesBothMetrics(t *testing.T) {
	m := NewMultiPeerManager(DirectionSend, 0)

	_, _ = m.AddPeer("client1", ICEConfig{})
	_, _ = m.AddPeer("client2", ICEConfig{})
	_, _ = m.AddPeer("client3", ICEConfig{})

	beforeSize := testutil.ToFloat64(metrics.ConnectionPoolSize)
	if beforeSize < 3 {
		t.Logf("Warning: ConnectionPoolSize before Close is less than expected: %v", beforeSize)
	}

	err := m.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	afterSize := testutil.ToFloat64(metrics.ConnectionPoolSize)
	afterActive := testutil.ToFloat64(metrics.ConnectionPoolActive)

	if afterSize != beforeSize-3 {
		t.Errorf("expected ConnectionPoolSize to decrease by 3, got before=%v, after=%v", beforeSize, afterSize)
	}
	if afterActive != 0 {
		t.Errorf("expected ConnectionPoolActive to be 0 after Close, got %v", afterActive)
	}
}
