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

func TestClient_SetWifiConfig_ValidationErrors(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := c.SetWifiConfig(ctx, "", "pw"); err == nil {
		t.Fatal("expected error for empty ssid")
	}
	if _, err := c.SetWifiConfig(ctx, strings.Repeat("s", 33), "pw"); err == nil {
		t.Fatal("expected error for oversized ssid")
	}
	if _, err := c.SetWifiConfig(ctx, "net", strings.Repeat("p", 65)); err == nil {
		t.Fatal("expected error for oversized password")
	}

	if sent := mock.Sent(); len(sent) != 0 {
		t.Fatalf("expected no RPC calls for invalid input, got %d", len(sent))
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
