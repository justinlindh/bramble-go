package bramble

import (
	"context"
	"testing"
	"time"

	"github.com/justinlindh/bramble-go/transport/transporttest"
)

func TestClient_OnPeerLocation(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	fired := make(chan PeerLocationEvent, 1)
	c.OnPeerLocation(func(e PeerLocationEvent) { fired <- e })
	// Firmware sends null params for this notification.
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onPeerLocation","params":null}`)

	select {
	case <-fired:
		// callback fired — no fields to check on the empty struct
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnPeerLocation callback")
	}
}

func TestClient_OnPeerLocation_NilCallback(t *testing.T) {
	// Verify no panic when no callback is registered.
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onPeerLocation","params":null}`)
	time.Sleep(100 * time.Millisecond) // give notifyLoop time to process
	_ = c
}

func TestClient_OnIdentityChange(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan IdentityChangeEvent, 1)
	c.OnIdentityChange(func(e IdentityChangeEvent) { received <- e })
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onIdentityChange","params":{"new_address":"DEADBEEF","reason":"address_collision"}}`)

	select {
	case evt := <-received:
		if evt.NewAddress != "DEADBEEF" {
			t.Errorf("new_address: got %q, want DEADBEEF", evt.NewAddress)
		}
		if evt.Reason != "address_collision" {
			t.Errorf("reason: got %q, want address_collision", evt.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnIdentityChange callback")
	}
}

func TestClient_OnIdentityChange_NilCallback(t *testing.T) {
	// Verify no panic when no callback is registered.
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onIdentityChange","params":{"new_address":"CAFEBABE","reason":"address_collision"}}`)
	time.Sleep(100 * time.Millisecond)
	_ = c
}

func TestClient_WithOnPeerLocation_Option(t *testing.T) {
	mock := transporttest.NewMock()
	ctx := context.Background()
	if err := mock.Connect(ctx); err != nil {
		t.Fatalf("mock.Connect: %v", err)
	}

	fired := make(chan PeerLocationEvent, 1)
	c := NewClient(mock, WithOnPeerLocation(func(e PeerLocationEvent) { fired <- e }))
	c.proto = NewProtocol(mock)
	c.proto.Start()
	go c.notifyLoop()
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onPeerLocation","params":null}`)

	select {
	case <-fired:
		// success — WithOnPeerLocation option wired up correctly
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WithOnPeerLocation callback")
	}
}

func TestClient_WithOnIdentityChange_Option(t *testing.T) {
	mock := transporttest.NewMock()
	ctx := context.Background()
	if err := mock.Connect(ctx); err != nil {
		t.Fatalf("mock.Connect: %v", err)
	}

	received := make(chan IdentityChangeEvent, 1)
	c := NewClient(mock, WithOnIdentityChange(func(e IdentityChangeEvent) { received <- e }))
	c.proto = NewProtocol(mock)
	c.proto.Start()
	go c.notifyLoop()
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onIdentityChange","params":{"new_address":"12345678","reason":"address_collision"}}`)

	select {
	case evt := <-received:
		if evt.NewAddress != "12345678" || evt.Reason != "address_collision" {
			t.Errorf("unexpected event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WithOnIdentityChange callback")
	}
}
