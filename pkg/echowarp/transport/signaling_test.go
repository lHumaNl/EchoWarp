package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPSignaler_Server_AcceptsConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	addr := server.ListenAddr()
	require.NotEmpty(t, addr)

	client := NewTCPSignaler(RoleClient, addr)
	go func() {
		_ = client.Start(ctx)
	}()

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client to be ready")
	}

	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server connection to be ready")
	}

	assert.NotEmpty(t, server.RemoteAddr())

	cancel()
	server.Close()
	client.Close()
}

func TestTCPSignaler_Send_DeliversMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	client := NewTCPSignaler(RoleClient, server.ListenAddr())
	go client.Start(ctx)

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client to be ready")
	}

	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server connection to be ready")
	}

	payload, _ := json.Marshal(map[string]string{"nonce": "abc123"})
	msg := SignalingMessage{
		Type:    "auth_challenge",
		Payload: payload,
	}
	err := client.Send(msg)
	require.NoError(t, err)

	select {
	case received := <-server.Receive():
		assert.Equal(t, "auth_challenge", received.Type)
		var p map[string]string
		json.Unmarshal(received.Payload, &p)
		assert.Equal(t, "abc123", p["nonce"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	cancel()
	server.Close()
	client.Close()
}

func TestTCPSignaler_Receive_ReadsMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	client := NewTCPSignaler(RoleClient, server.ListenAddr())
	go client.Start(ctx)

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client to be ready")
	}

	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server connection to be ready")
	}

	payload, _ := json.Marshal(map[string]bool{"success": true})
	err := server.Send(SignalingMessage{Type: "auth_result", Payload: payload})
	require.NoError(t, err)

	select {
	case received := <-client.Receive():
		assert.Equal(t, "auth_result", received.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	cancel()
	server.Close()
	client.Close()
}

func TestTCPSignaler_Close_ClosesConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	client := NewTCPSignaler(RoleClient, server.ListenAddr())
	go client.Start(ctx)

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client to be ready")
	}

	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server connection to be ready")
	}

	err := server.Close()
	assert.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)

	cancel()
}

func TestTCPSignaler_MultipleMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	client := NewTCPSignaler(RoleClient, server.ListenAddr())
	go client.Start(ctx)

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client to be ready")
	}

	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server connection to be ready")
	}

	for i := 0; i < 10; i++ {
		payload, _ := json.Marshal(map[string]int{"seq": i})
		err := client.Send(SignalingMessage{Type: "test", Payload: payload})
		require.NoError(t, err)
	}

	for i := 0; i < 10; i++ {
		select {
		case msg := <-server.Receive():
			assert.Equal(t, "test", msg.Type)
			var p map[string]int
			json.Unmarshal(msg.Payload, &p)
			assert.Equal(t, i, p["seq"])
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}

	cancel()
	server.Close()
	client.Close()
}

func TestTCPSignaler_BidirectionalCommunication(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	client := NewTCPSignaler(RoleClient, server.ListenAddr())
	go client.Start(ctx)

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client to be ready")
	}

	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server connection to be ready")
	}

	err := server.Send(SignalingMessage{Type: "ping", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)

	select {
	case msg := <-client.Receive():
		assert.Equal(t, "ping", msg.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	err = client.Send(SignalingMessage{Type: "pong", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)

	select {
	case msg := <-server.Receive():
		assert.Equal(t, "pong", msg.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	cancel()
	server.Close()
	client.Close()
}

func TestTCPSignaler_TLS_EstablishesSecureConnection(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	cert := tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: priv}
	serverTLSConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0", WithTLS(serverTLSConfig))
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	clientTLSConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewTCPSignaler(RoleClient, server.ListenAddr(), WithTLS(clientTLSConfig))
	go client.Start(ctx)

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client to be ready")
	}

	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server connection to be ready")
	}

	assert.NotEmpty(t, server.RemoteAddr())

	payload, _ := json.Marshal(map[string]string{"secure": "yes"})
	err = client.Send(SignalingMessage{Type: "secure_test", Payload: payload})
	require.NoError(t, err)

	select {
	case received := <-server.Receive():
		assert.Equal(t, "secure_test", received.Type)
		var p map[string]string
		json.Unmarshal(received.Payload, &p)
		assert.Equal(t, "yes", p["secure"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for secure message")
	}

	cancel()
	server.Close()
	client.Close()
}

func TestWithMaxMessageSize(t *testing.T) {
	signaler := NewTCPSignaler(RoleServer, "127.0.0.1:0", WithMaxMessageSize(1024*1024))
	assert.NotNil(t, signaler)
	assert.Equal(t, 1024*1024, signaler.maxMsgSize)
}

func TestNewTCPSignalerFromConn(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := NewTCPSignalerFromConn(server)
	assert.NotNil(t, signaler)
	assert.Equal(t, server.RemoteAddr().String(), signaler.RemoteAddr())
}

func TestTCPSignaler_StartWithConn(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signaler.StartWithConn(ctx)

	go func() {
		data, _ := json.Marshal(SignalingMessage{Type: "test", Payload: json.RawMessage(`{}`)})
		data = append(data, '\n')
		client.Write(data)
	}()

	select {
	case msg := <-signaler.Receive():
		assert.Equal(t, "test", msg.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestWithIdleTimeout(t *testing.T) {
	signaler := NewTCPSignaler(RoleServer, "127.0.0.1:0", WithIdleTimeout(5*time.Second))
	assert.NotNil(t, signaler)
	assert.Equal(t, 5*time.Second, signaler.idleTimeout)
}

func TestWithIdleTimeout_Zero(t *testing.T) {
	signaler := NewTCPSignaler(RoleServer, "127.0.0.1:0", WithIdleTimeout(0))
	assert.NotNil(t, signaler)
	assert.Equal(t, time.Duration(0), signaler.idleTimeout)
}

func TestWithCertificatePinning(t *testing.T) {
	fingerprints := []string{"a1b2c3d4e5f6"}
	signaler := NewTCPSignaler(RoleServer, "127.0.0.1:0", WithCertificatePinning(fingerprints))
	assert.NotNil(t, signaler)
	assert.Equal(t, fingerprints, signaler.pinnedFingerprints)
}

func TestTCPSignaler_CertificatePinning_TLSMismatch(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	cert := tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: priv}
	serverTLSConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0", WithTLS(serverTLSConfig))
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server listener to be ready")
	}

	wrongFingerprint := "0000000000000000000000000000000000000000000000000000000000000000"
	clientTLSConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewTCPSignaler(RoleClient, server.ListenAddr(),
		WithTLS(clientTLSConfig),
		WithCertificatePinning([]string{wrongFingerprint}),
	)

	err = client.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")

	server.Close()
	client.Close()
}

func TestTCPSignaler_CertificatePinning_NonTLS(t *testing.T) {
	fingerprints := []string{"a1b2c3d4e5f6"}
	server := NewTCPSignaler(RoleServer, "127.0.0.1:0", WithCertificatePinning(fingerprints))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	select {
	case <-server.ListenerReady():
		client := NewTCPSignaler(RoleClient, server.ListenAddr(), WithCertificatePinning(fingerprints))
		err := client.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "certificate pinning requires TLS")
		client.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	cancel()
	server.Close()
}

func TestTCPSignaler_Send_NotReady(t *testing.T) {
	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")

	sendDone := make(chan error, 1)
	go func() {
		err := server.Send(SignalingMessage{Type: "test"})
		sendDone <- err
	}()

	select {
	case <-sendDone:
		t.Fatal("Send should block until connection is ready")
	case <-time.After(100 * time.Millisecond):
	}

	server.Close()
}

func TestTCPSignaler_ReadLoop_InvalidJSON(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signaler.StartWithConn(ctx)

	_, err := client.Write([]byte("invalid json\n"))
	require.NoError(t, err)

	select {
	case <-signaler.Receive():
		t.Fatal("should not receive invalid JSON")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestTCPSignaler_ReadLoop_ContextCancel(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithCancel(context.Background())

	signaler.StartWithConn(ctx)

	// Cancel the context to stop the read loop
	cancel()

	// Verify the read loop has stopped by checking that the Receive channel
	// is either closed or won't receive the message we're about to send.
	// Use Eventually to poll for the expected state.
	require.Eventually(t, func() bool {
		// Write should succeed but nothing should be received
		_, err := client.Write([]byte(`{"type":"test","payload":{}}` + "\n"))
		if err != nil {
			return false
		}
		// Check that nothing is received (read loop has stopped)
		select {
		case <-signaler.Receive():
			return false // Read loop is still running
		default:
			return true // Read loop has stopped
		}
	}, 500*time.Millisecond, 20*time.Millisecond, "Read loop should stop after context cancellation")
}

func TestTCPSignaler_Client_ConnectionRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := NewTCPSignaler(RoleClient, "127.0.0.1:1")
	err := client.Start(ctx)
	assert.Error(t, err)
	client.Close()
}

func TestTCPSignaler_Server_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	cancel()

	select {
	case err := <-errCh:
		assert.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for context cancel")
	}

	server.Close()
}

func TestTCPSignaler_Close_WithConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")
	go server.Start(ctx)

	select {
	case <-server.ListenerReady():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	client := NewTCPSignaler(RoleClient, server.ListenAddr())
	go client.Start(ctx)

	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	err := server.Close()
	assert.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}

func TestTCPSignaler_Close_MultipleCalls(t *testing.T) {
	server := NewTCPSignaler(RoleServer, "127.0.0.1:0")

	err := server.Close()
	assert.NoError(t, err)

	err = server.Close()
	assert.NoError(t, err)
}
