package bramble

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClient_SetWifiConfig(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"applied":"reboot_required"}}`)
	resp, err := c.SetWifiConfig(ctx, "my-network", "hunter22")
	if err != nil || !resp.OK || resp.Applied != "reboot_required" {
		t.Fatalf("SetWifiConfig failed: resp=%+v err=%v", resp, err)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.setWifiConfig"`) ||
		!strings.Contains(sent[0], `"ssid":"my-network"`) ||
		!strings.Contains(sent[0], `"password":"hunter22"`) {
		t.Fatalf("unexpected request payload: %s", sent[0])
	}
}

func TestClient_SetWifiConfig_OpenNetwork(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"applied":"reboot_required"}}`)
	resp, err := c.SetWifiConfig(ctx, "open-network", "")
	if err != nil || !resp.OK {
		t.Fatalf("SetWifiConfig(open) failed: resp=%+v err=%v", resp, err)
	}

	sent := mock.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0], `"password":""`) {
		t.Fatalf("expected empty password in request, got %v", sent)
	}
}

func TestClient_SetWifiConfig_ServerError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"unauthorized"}}`)
	if _, err := c.SetWifiConfig(ctx, "my-network", "hunter22"); err == nil {
		t.Fatal("expected error for unauthorized response")
	} else if !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected error to mention unauthorized, got %v", err)
	}
}

// TestClient_SetWifiConfig_InvalidParamsRejection covers the out-of-bounds
// ssid/password case: the SDK does no client-side length validation (it
// sends whatever the caller passes, same as SetNodeName/AddChannel), so an
// invalid ssid or password is rejected by the firmware's
// RPC_ERR_INVALID_PARAMS and must propagate as a normal error here.
func TestClient_SetWifiConfig_InvalidParamsRejection(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"invalid params"}}`)
	if _, err := c.SetWifiConfig(ctx, strings.Repeat("s", 33), "pw"); err == nil {
		t.Fatal("expected error propagated from firmware rejection")
	} else if !strings.Contains(err.Error(), "invalid params") {
		t.Fatalf("expected error to mention invalid params, got %v", err)
	}

	sent := mock.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0], `"method":"bramble.setWifiConfig"`) {
		t.Fatalf("expected the RPC call to be sent (no client-side gate), got %v", sent)
	}
}
