//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
	"github.com/stretchr/testify/require"
)

func TestE2E_AuthBan_FailedAttempts_TriggersBan(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	banFilePath := filepath.Join(tempDir, "banned_ips.json")

	banMgr, err := ban.NewFileBanManager(3, banFilePath)
	if err != nil {
		t.Fatalf("failed to create ban manager: %v", err)
	}

	for i := 0; i < 3; i++ {
		newlyBanned := banMgr.RecordFailure("127.0.0.1:12345")
		if i < 2 && newlyBanned {
			t.Errorf("unexpected ban on attempt %d", i+1)
		}
		if i == 2 && !newlyBanned {
			t.Error("expected ban on 3rd attempt")
		}
	}

	if !banMgr.IsBanned("127.0.0.1:12345") {
		t.Error("expected IP to be banned")
	}

	if err := banMgr.Close(); err != nil {
		t.Fatalf("failed to close ban manager: %v", err)
	}

	if _, err := os.Stat(banFilePath); os.IsNotExist(err) {
		t.Error("expected ban file to be created")
	}
}

func TestE2E_AuthBan_BannedClient_RejectedImmediately(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	tempDir := t.TempDir()
	banFilePath := filepath.Join(tempDir, "banned_ips.json")

	banMgr, err := ban.NewFileBanManager(1, banFilePath)
	if err != nil {
		t.Fatalf("failed to create ban manager: %v", err)
	}
	defer banMgr.Close()

	banMgr.RecordFailure("127.0.0.1")

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

	remoteAddr := server.RemoteAddr()
	if banMgr.IsBanned(remoteAddr) {
		authResultPayload, _ := json.Marshal(auth.AuthResultPayload{
			Success: false,
			Message: "IP banned",
			Code:    403,
		})
		_ = server.Send(transport.SignalingMessage{Type: "auth_result", Payload: authResultPayload})

		select {
		case msg := <-client.Receive():
			if msg.Type != "auth_result" {
				t.Errorf("expected auth_result message, got %s", msg.Type)
			}
			var result auth.AuthResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err != nil {
				t.Fatalf("failed to unmarshal auth result: %v", err)
			}
			if result.Success {
				t.Error("expected auth result to be unsuccessful")
			}
			if result.Code != 403 {
				t.Errorf("expected code 403, got %d", result.Code)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for rejection message")
		}
	}

	_ = server.Close()
	_ = client.Close()
}

func TestE2E_AuthBan_SuccessResetsFailureCount(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	banFilePath := filepath.Join(tempDir, "banned_ips.json")

	banMgr, err := ban.NewFileBanManager(5, banFilePath)
	if err != nil {
		t.Fatalf("failed to create ban manager: %v", err)
	}
	defer banMgr.Close()

	for i := 0; i < 3; i++ {
		newlyBanned := banMgr.RecordFailure("127.0.0.1:12345")
		if newlyBanned {
			t.Errorf("unexpected ban on attempt %d", i+1)
		}
	}

	if banMgr.IsBanned("127.0.0.1:12345") {
		t.Error("should not be banned after 3 attempts with max=5")
	}

	banMgr.RecordSuccess("127.0.0.1:12345")

	for i := 0; i < 4; i++ {
		newlyBanned := banMgr.RecordFailure("127.0.0.1:12345")
		if newlyBanned {
			t.Errorf("unexpected ban on attempt %d after reset", i+1)
		}
	}

	if banMgr.IsBanned("127.0.0.1:12345") {
		t.Error("should not be banned: 4 failures after reset < 5 max")
	}

	newlyBanned := banMgr.RecordFailure("127.0.0.1:12345")
	if !newlyBanned {
		t.Error("expected ban on 5th consecutive failure after reset")
	}
}
