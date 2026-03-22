package transport

import (
	"context"
	"errors"
	"testing"
	"time"
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

func TestBLESendReconnecting(t *testing.T) {
	b := NewBLE("")
	b.reconnecting = true
	if err := b.Send([]byte("{}")); !errors.Is(err, ErrReconnecting) {
		t.Fatalf("expected ErrReconnecting, got %v", err)
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

func TestBLEDisconnectReconnectCallbacks(t *testing.T) {
	b := NewBLE("")
	b.connected = true

	disconnectCalled := 0
	reconnectCalled := 0
	b.OnDisconnect = func() { disconnectCalled++ }
	b.OnReconnect = func() { reconnectCalled++ }

	b.handleConnectionStateChange(false)
	if disconnectCalled != 1 {
		t.Fatalf("expected disconnect callback once, got %d", disconnectCalled)
	}

	b.reconnecting = false
	b.handleConnectionStateChange(true)
	if reconnectCalled != 1 {
		t.Fatalf("expected reconnect callback once, got %d", reconnectCalled)
	}
}

func TestBLEHandleDisconnectClosesTransport(t *testing.T) {
	b := NewBLE("")
	b.connected = true

	b.handleConnectionStateChange(false)

	if b.connected {
		t.Fatal("expected transport disconnected")
	}
	if _, err := b.Receive(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after disconnect, got %v", err)
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
