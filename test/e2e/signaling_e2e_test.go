//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
	"github.com/stretchr/testify/require"
)

func TestE2E_TCPSignaler_ServerClientConnect_Loopback(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	server := transport.NewTCPSignaler(transport.RoleServer, addr)
	client := transport.NewTCPSignaler(transport.RoleClient, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("server start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.ListenAddr() != ""
	}, 2*time.Second, 50*time.Millisecond, "server listen addr")

	go func() {
		if err := client.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("client start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.RemoteAddr() != ""
	}, 5*time.Second, 50*time.Millisecond, "server remote addr")

	if server.RemoteAddr() == "" {
		t.Fatal("server remote addr is empty after connection")
	}

	msg := transport.SignalingMessage{Type: "ping"}
	payload, _ := json.Marshal(map[string]string{"data": "hello"})
	msg.Payload = payload

	if err := client.Send(msg); err != nil {
		t.Fatalf("client send error: %v", err)
	}

	select {
	case received := <-server.Receive():
		if received.Type != "ping" {
			t.Errorf("expected message type 'ping', got '%s'", received.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message on server")
	}

	responseMsg := transport.SignalingMessage{Type: "pong"}
	responsePayload, _ := json.Marshal(map[string]string{"data": "world"})
	responseMsg.Payload = responsePayload

	if err := server.Send(responseMsg); err != nil {
		t.Fatalf("server send error: %v", err)
	}

	select {
	case received := <-client.Receive():
		if received.Type != "pong" {
			t.Errorf("expected message type 'pong', got '%s'", received.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for response on client")
	}

	_ = server.Close()
	_ = client.Close()
}

func TestE2E_TCPSignaler_MultipleMessages_OrderPreserved(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	server := transport.NewTCPSignaler(transport.RoleServer, addr)
	client := transport.NewTCPSignaler(transport.RoleClient, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("server start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.ListenAddr() != ""
	}, 2*time.Second, 50*time.Millisecond, "server listen addr")

	go func() {
		if err := client.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("client start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.RemoteAddr() != ""
	}, 5*time.Second, 50*time.Millisecond, "server remote addr")

	numMessages := 20
	for i := 0; i < numMessages; i++ {
		msg := transport.SignalingMessage{Type: "seq"}
		payload, _ := json.Marshal(map[string]int{"seq": i})
		msg.Payload = payload
		if err := client.Send(msg); err != nil {
			t.Fatalf("client send error at message %d: %v", i, err)
		}
	}

	receivedSeqs := make([]int, 0, numMessages)
	timeout := time.After(10 * time.Second)

	for len(receivedSeqs) < numMessages {
		select {
		case msg := <-server.Receive():
			if msg.Type != "seq" {
				t.Errorf("unexpected message type: %s", msg.Type)
				continue
			}
			var payload struct{ Seq int }
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				t.Errorf("failed to unmarshal payload: %v", err)
				continue
			}
			receivedSeqs = append(receivedSeqs, payload.Seq)
		case <-timeout:
			t.Fatalf("timeout: received only %d/%d messages", len(receivedSeqs), numMessages)
		}
	}

	if len(receivedSeqs) != numMessages {
		t.Errorf("expected %d messages, got %d", numMessages, len(receivedSeqs))
	}

	for i, seq := range receivedSeqs {
		if seq != i {
			t.Errorf("message out of order: expected index %d to have seq %d, got %d", i, i, seq)
		}
	}

	_ = server.Close()
	_ = client.Close()
}

func TestE2E_TCPSignaler_TLS_EstablishesSecureConnection(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	certPEM, keyPEM := generateSelfSignedCert(t, "localhost")

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	server := transport.NewTCPSignaler(transport.RoleServer, addr, transport.WithTLS(serverTLSConfig))
	client := transport.NewTCPSignaler(transport.RoleClient, addr, transport.WithTLS(clientTLSConfig))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("server start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.ListenAddr() != ""
	}, 2*time.Second, 50*time.Millisecond, "server listen addr")

	go func() {
		if err := client.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("client start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.RemoteAddr() != ""
	}, 5*time.Second, 50*time.Millisecond, "server remote addr")

	msg := transport.SignalingMessage{Type: "secure_test"}
	payload, _ := json.Marshal(map[string]string{"data": "encrypted"})
	msg.Payload = payload

	if err := client.Send(msg); err != nil {
		t.Fatalf("client send error: %v", err)
	}

	select {
	case received := <-server.Receive():
		if received.Type != "secure_test" {
			t.Errorf("expected message type 'secure_test', got '%s'", received.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message over TLS connection")
	}

	_ = server.Close()
	_ = client.Close()
}

func TestE2E_TCPSignaler_ServerShutdown_WhileClientConnected(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	server := transport.NewTCPSignaler(transport.RoleServer, addr)
	client := transport.NewTCPSignaler(transport.RoleClient, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("server start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.ListenAddr() != ""
	}, 2*time.Second, 50*time.Millisecond, "server listen addr")

	go func() {
		if err := client.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("client start error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.RemoteAddr() != ""
	}, 5*time.Second, 50*time.Millisecond, "server remote addr")

	var wg sync.WaitGroup
	wg.Add(1)
	receiveDone := make(chan struct{})
	go func() {
		defer wg.Done()
		defer close(receiveDone)
		for {
			select {
			case _, ok := <-client.Receive():
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()

	if err := server.Close(); err != nil {
		t.Errorf("server close error: %v", err)
	}

	select {
	case <-receiveDone:
	case <-time.After(2 * time.Second):
		t.Log("receive loop did not exit cleanly within timeout")
	}

	_ = client.Close()
	wg.Wait()
}

func generateSelfSignedCert(t *testing.T, host string) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{host},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPEM, keyPEM
}
