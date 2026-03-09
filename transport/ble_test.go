package transport

import (
	"context"
	"errors"
	"testing"
)

func TestBLEOnNotificationAssemblesLines(t *testing.T) {
	b := NewBLE(BLEConfig{})
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
	b := NewBLE(BLEConfig{})
	if err := b.Send([]byte("{}")); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestBLEReceiveClosed(t *testing.T) {
	b := NewBLE(BLEConfig{})
	close(b.closeCh)
	if _, err := b.Receive(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestBLEConnectAlreadyConnected(t *testing.T) {
	b := NewBLE(BLEConfig{})
	b.connected = true
	if err := b.Connect(context.Background()); err == nil {
		t.Fatal("expected already connected error")
	}
}

func TestBLEInfo(t *testing.T) {
	if got := NewBLE(BLEConfig{}).Info(); got != "ble:auto-scan" {
		t.Fatalf("unexpected info: %q", got)
	}
	if got := NewBLE(BLEConfig{DeviceName: "Bramble"}).Info(); got != "ble:Bramble" {
		t.Fatalf("unexpected info with name: %q", got)
	}
}

func TestBLETransport_SetAuthToken(t *testing.T) {
	b := NewBLE(BLEConfig{})

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

func TestValidateAuthResponse(t *testing.T) {
	if err := validateAuthResponse([]byte(`{"jsonrpc":"2.0","id":0,"result":{"ok":true}}`)); err != nil {
		t.Fatalf("expected valid auth response, got %v", err)
	}
	if err := validateAuthResponse([]byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-32001,"message":"unauthorized"}}`)); err == nil {
		t.Fatal("expected auth error response to fail")
	}
}
