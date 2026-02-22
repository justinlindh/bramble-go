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
