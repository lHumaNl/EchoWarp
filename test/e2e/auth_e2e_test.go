//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
	"github.com/stretchr/testify/require"
)

func TestE2E_Auth_FullFlow_Success(t *testing.T) {
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

	password := "test_password"
	authenticator := auth.NewChallengeAuth()

	serverSend := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return server.Send(transport.SignalingMessage{Type: msgType, Payload: data})
	}

	serverRecv := func() (string, []byte, error) {
		select {
		case msg := <-server.Receive():
			return msg.Type, msg.Payload, nil
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}

	clientSend := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return client.Send(transport.SignalingMessage{Type: msgType, Payload: data})
	}

	clientRecv := func() (string, []byte, error) {
		select {
		case msg := <-client.Receive():
			return msg.Type, msg.Payload, nil
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- authenticator.AuthenticateServer(serverSend, serverRecv, password)
	}()

	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- authenticator.AuthenticateClient(clientSend, clientRecv, password)
	}()

	select {
	case err := <-serverErrCh:
		if err != nil {
			t.Errorf("server authentication failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for server authentication")
	}

	select {
	case err := <-clientErrCh:
		if err != nil {
			t.Errorf("client authentication failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for client authentication")
	}

	_ = server.Close()
	_ = client.Close()
}

func TestE2E_Auth_WrongPassword_Failure(t *testing.T) {
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

	authenticator := auth.NewChallengeAuth()

	serverSend := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return server.Send(transport.SignalingMessage{Type: msgType, Payload: data})
	}

	serverRecv := func() (string, []byte, error) {
		select {
		case msg := <-server.Receive():
			return msg.Type, msg.Payload, nil
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}

	clientSend := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return client.Send(transport.SignalingMessage{Type: msgType, Payload: data})
	}

	clientRecv := func() (string, []byte, error) {
		select {
		case msg := <-client.Receive():
			return msg.Type, msg.Payload, nil
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- authenticator.AuthenticateServer(serverSend, serverRecv, "correct_password")
	}()

	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- authenticator.AuthenticateClient(clientSend, clientRecv, "wrong_password")
	}()

	select {
	case err := <-serverErrCh:
		if err == nil {
			t.Error("expected server authentication to fail with wrong password")
		} else if !errors.Is(err, auth.ErrAuthFailed) {
			t.Errorf("expected ErrAuthFailed, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for server authentication")
	}

	select {
	case err := <-clientErrCh:
		if err == nil {
			t.Error("expected client authentication to fail with wrong password")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for client authentication")
	}

	_ = server.Close()
	_ = client.Close()
}

func TestE2E_Auth_EmptyPassword_SkipsAuth(t *testing.T) {
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

	authenticator := auth.NewChallengeAuth()

	serverSend := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return server.Send(transport.SignalingMessage{Type: msgType, Payload: data})
	}

	serverRecv := func() (string, []byte, error) {
		select {
		case msg := <-server.Receive():
			return msg.Type, msg.Payload, nil
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}

	clientSend := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return client.Send(transport.SignalingMessage{Type: msgType, Payload: data})
	}

	clientRecv := func() (string, []byte, error) {
		select {
		case msg := <-client.Receive():
			return msg.Type, msg.Payload, nil
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- authenticator.AuthenticateServer(serverSend, serverRecv, "")
	}()

	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- authenticator.AuthenticateClient(clientSend, clientRecv, "")
	}()

	select {
	case err := <-serverErrCh:
		if err != nil {
			t.Errorf("server authentication should succeed with empty password, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for server authentication")
	}

	select {
	case err := <-clientErrCh:
		if err != nil {
			t.Errorf("client authentication should succeed with empty password, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for client authentication")
	}

	_ = server.Close()
	_ = client.Close()
}
