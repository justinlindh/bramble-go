package transport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"
)

func TestBLEOnNotificationAssemblesLines(t *testing.T) {
	b := NewBLE("")
	b.onNotification([]byte("{\"a\":1"))
	b.onNotification([]byte("}\nnoise\n{\"b\":2}\n"))

	ctx := context.Background()
	msg1, err := b.Receive(ctx)
	if err != nil {
		t.Fatalf("receive1: %v", err)
	}
	msg2, err := b.Receive(ctx)
	if err != nil {
		t.Fatalf("receive2: %v", err)
	}
	if string(msg1) != `{"a":1}` {
		t.Fatalf("unexpected first msg: %q", msg1)
	}
	if string(msg2) != "noise" {
		t.Fatalf("unexpected second msg: %q", msg2)
	}
}

func TestBLESendNotConnected(t *testing.T) {
	b := NewBLE("")
	if err := b.Send([]byte("{}")); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestBLEReceiveClosed(t *testing.T) {
	b := NewBLE("")
	close(b.closeCh)
	if _, err := b.Receive(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestBLEConnectAlreadyConnected(t *testing.T) {
	b := NewBLE("")
	b.connected = true
	if err := b.Connect(context.Background()); err == nil {
		t.Fatal("expected already connected error")
	}
}

func TestBLEInfo(t *testing.T) {
	if got := NewBLE("").Info(); got != "ble:auto-scan" {
		t.Fatalf("unexpected info: %q", got)
	}
	if got := NewBLE("Bramble").Info(); got != "ble:Bramble" {
		t.Fatalf("unexpected info with name: %q", got)
	}
}

func TestBLETransport_SetAuthToken(t *testing.T) {
	b := NewBLE("")

	if b.cfg.AuthToken != "" {
		t.Fatalf("expected empty token by default, got %q", b.cfg.AuthToken)
	}

	b.SetAuthToken("token-1")
	if b.cfg.AuthToken != "token-1" {
		t.Fatalf("expected token-1, got %q", b.cfg.AuthToken)
	}

	b.SetAuthToken("token-2")
	if b.cfg.AuthToken != "token-2" {
		t.Fatalf("expected overwritten token-2, got %q", b.cfg.AuthToken)
	}

	b.SetAuthToken("")
	if b.cfg.AuthToken != "" {
		t.Fatalf("expected empty token, got %q", b.cfg.AuthToken)
	}
}

func TestBLENewBLE_WithAuthToken(t *testing.T) {
	b := NewBLE("Bramble", WithAuthToken("secret"))
	if b.cfg.AuthToken != "secret" {
		t.Fatalf("expected auth token 'secret', got %q", b.cfg.AuthToken)
	}
	if b.cfg.DeviceName != "Bramble" {
		t.Fatalf("expected device name 'Bramble', got %q", b.cfg.DeviceName)
	}
}

func TestBLENewBLE_WithScanTimeout(t *testing.T) {
	b := NewBLE("", WithBLEScanTimeout(30*time.Second))
	if b.cfg.ScanTimeout != 30*time.Second {
		t.Fatalf("expected 30s scan timeout, got %v", b.cfg.ScanTimeout)
	}
}

func TestBLENewBLE_DefaultScanTimeout(t *testing.T) {
	b := NewBLE("")
	if b.cfg.ScanTimeout != 10*time.Second {
		t.Fatalf("expected default 10s scan timeout, got %v", b.cfg.ScanTimeout)
	}
}

func TestBLENewBLE_MultipleOptions(t *testing.T) {
	b := NewBLE("MyDevice", WithAuthToken("tok"), WithBLEScanTimeout(5*time.Second))
	if b.cfg.AuthToken != "tok" {
		t.Fatalf("expected auth token 'tok', got %q", b.cfg.AuthToken)
	}
	if b.cfg.DeviceName != "MyDevice" {
		t.Fatalf("expected device name 'MyDevice', got %q", b.cfg.DeviceName)
	}
	if b.cfg.ScanTimeout != 5*time.Second {
		t.Fatalf("expected 5s scan timeout, got %v", b.cfg.ScanTimeout)
	}
}

func TestValidateAuthResponse(t *testing.T) {
	if err := validateAuthResponse([]byte(`{"jsonrpc":"2.0","id":0,"result":{"ok":true}}`)); err != nil {
		t.Fatalf("expected valid auth response, got %v", err)
	}
	if err := validateAuthResponse([]byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-32001,"message":"unauthorized"}}`)); err == nil {
		t.Fatal("expected auth error response to fail")
	}
}

// TestBLEAuthenticate_NoToken verifies that authenticate is a no-op when no
// auth token is configured — it must return nil without touching BLE hardware.
func TestBLEAuthenticate_NoToken(t *testing.T) {
	b := NewBLE("")
	if err := b.authenticate(context.Background()); err != nil {
		t.Fatalf("expected nil with no auth token, got %v", err)
	}
}

// TestBLEAuthenticate_SendFails verifies that authenticate wraps a Send error
// as an "auth write" error. The transport is not connected, so Send returns
// ErrNotConnected before any BLE hardware is accessed.
func TestBLEAuthenticate_SendFails(t *testing.T) {
	b := NewBLE("", WithAuthToken("secret-token"))
	// b.connected is false — Send() returns ErrNotConnected
	err := b.authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "auth write") {
		t.Fatalf("expected 'auth write' in error, got: %v", err)
	}
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected wrapped in error, got: %v", err)
	}
}

// TestBLEConnect_AdapterEnableFails verifies that Connect() propagates adapter
// Enable() errors wrapped as "enable adapter". A non-existent adapter ID is
// injected so Enable() fails deterministically without real BLE hardware.
func TestBLEConnect_AdapterEnableFails(t *testing.T) {
	b := NewBLE("")
	b.adapter = bluetooth.NewAdapter("nonexistent-ble-adapter-test-99")
	err := b.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error from Connect() with non-existent adapter")
	}
	if !strings.Contains(err.Error(), "enable adapter") {
		t.Fatalf("expected 'enable adapter' in error, got: %v", err)
	}
}

// TestBLEReceive_ContextCancel verifies that Receive returns context.Canceled
// when the context is already cancelled on entry.
func TestBLEReceive_ContextCancel(t *testing.T) {
	b := NewBLE("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Receive(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestBLEOnNotification_DropsWhenFull verifies that onNotification silently
// drops complete lines when recvCh is at capacity (no panic, no block).
func TestBLEOnNotification_DropsWhenFull(t *testing.T) {
	b := NewBLE("")
	// Fill the channel to capacity.
	for i := 0; i < cap(b.recvCh); i++ {
		b.recvCh <- []byte("msg")
	}
	// Sending another complete line should not block or panic.
	b.onNotification([]byte("overflow\n"))
	// Channel must remain exactly at capacity (message was dropped).
	if got := len(b.recvCh); got != cap(b.recvCh) {
		t.Fatalf("expected channel len %d == cap %d after drop, got %d", cap(b.recvCh), cap(b.recvCh), got)
	}
}
