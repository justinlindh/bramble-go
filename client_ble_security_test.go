package bramble

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClient_BleSecurity_StaticPasskey(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"mode":"static-passkey","staticPasskeySet":true}}`)
	resp, err := c.BleSecurity(ctx)
	if err != nil {
		t.Fatalf("BleSecurity failed: %v", err)
	}
	if resp.Mode != BleSecurityModeStaticPasskey || !resp.StaticPasskeySet {
		t.Fatalf("unexpected posture: %+v", resp)
	}

	sent := mock.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0], `"method":"bramble.getBleSecurity"`) {
		t.Fatalf("unexpected request payload: %v", sent)
	}
}

func TestClient_BleSecurity_DisplayBoard(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"mode":"passkey-display","staticPasskeySet":false}}`)
	resp, err := c.BleSecurity(ctx)
	if err != nil {
		t.Fatalf("BleSecurity failed: %v", err)
	}
	if resp.Mode != BleSecurityModePasskeyDisplay || resp.StaticPasskeySet {
		t.Fatalf("unexpected posture: %+v", resp)
	}
}

func TestClient_BleSecurity_ServerError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-1005,"message":"Unauthorized"}}`)
	if _, err := c.BleSecurity(ctx); err == nil {
		t.Fatal("expected error for unauthorized response")
	} else if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected error to mention Unauthorized, got %v", err)
	}
}

func TestClient_SetBlePasskey(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"mode":"static-passkey"}}`)
	resp, err := c.SetBlePasskey(ctx, "314159")
	if err != nil {
		t.Fatalf("SetBlePasskey failed: %v", err)
	}
	if !resp.OK || resp.Mode != BleSecurityModeStaticPasskey || resp.Error != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	sent := mock.Sent()
	if len(sent) != 1 ||
		!strings.Contains(sent[0], `"method":"bramble.setBlePasskey"`) ||
		!strings.Contains(sent[0], `"passkey":"314159"`) {
		t.Fatalf("unexpected request payload: %v", sent)
	}
}

// TestClient_SetBlePasskey_ClearSendsTheMember covers the wire contract's one
// booby trap: the node treats an omitted passkey member as an error, not as a
// clear, so clearing must send an explicit empty string rather than dropping
// the member from the params object.
func TestClient_SetBlePasskey_ClearSendsTheMember(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"mode":"just-works"}}`)
	resp, err := c.SetBlePasskey(ctx, "")
	if err != nil {
		t.Fatalf("SetBlePasskey(clear) failed: %v", err)
	}
	if !resp.OK || resp.Mode != BleSecurityModeJustWorks {
		t.Fatalf("unexpected response: %+v", resp)
	}

	sent := mock.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0], `"passkey":""`) {
		t.Fatalf("expected an explicit empty passkey member, got %v", sent)
	}
}

// TestClient_SetBlePasskey_RefusalIsNotAnError covers the other half of that
// contract: the node reports a refusal inside a successful result, so the SDK
// must hand the caller ok:false with the reason instead of a Go error.
func TestClient_SetBlePasskey_RefusalIsNotAnError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":false,"error":"board shows a random pairing code; static passkey unsupported"}}`)
	resp, err := c.SetBlePasskey(ctx, "314159")
	if err != nil {
		t.Fatalf("refusal must not surface as an error: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok false, got %+v", resp)
	}
	if !strings.Contains(resp.Error, "static passkey unsupported") {
		t.Fatalf("expected the refusal reason, got %q", resp.Error)
	}
}

// TestClient_SetBlePasskey_NoClientSideValidation documents that the SDK sends
// whatever the caller passes, same as SetWifiConfig and SetNodeName: a
// malformed passkey is the node's rejection to make, and it arrives as an
// ok:false refusal rather than being blocked here.
func TestClient_SetBlePasskey_NoClientSideValidation(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":false,"error":"passkey must be exactly 6 digits"}}`)
	resp, err := c.SetBlePasskey(ctx, "12345")
	if err != nil {
		t.Fatalf("SetBlePasskey failed: %v", err)
	}
	if resp.OK || !strings.Contains(resp.Error, "6 digits") {
		t.Fatalf("unexpected response: %+v", resp)
	}

	sent := mock.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0], `"passkey":"12345"`) {
		t.Fatalf("expected the call to be sent unvalidated, got %v", sent)
	}
}

func TestClient_SetBlePasskey_ServerError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-1005,"message":"Unauthorized"}}`)
	if _, err := c.SetBlePasskey(ctx, "314159"); err == nil {
		t.Fatal("expected error for unauthorized response")
	} else if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected error to mention Unauthorized, got %v", err)
	}
}
