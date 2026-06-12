package bramble

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClient_SendBroadcast_ReturnsBroadcastID(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"broadcast_id":"BCAST-001","status":"queued"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := c.SendBroadcast(ctx, "hello everyone")
	if err != nil {
		t.Fatalf("SendBroadcast error: %v", err)
	}
	if result.BroadcastID != "BCAST-001" {
		t.Fatalf("BroadcastID: got %q, want BCAST-001", result.BroadcastID)
	}
}

func TestClient_OnBroadcastDelivery_TypedPayload(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan BroadcastDelivery, 1)
	c.OnBroadcastDelivery(func(evt BroadcastDelivery) { received <- evt })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onBroadcastDelivery","params":{"broadcast_id":"BCAST-123","recipient":"A1B2C3D4","status":"delivered","timestamp_ms":1730000000123}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case evt := <-received:
		if evt.BroadcastID != "BCAST-123" {
			t.Fatalf("broadcast_id: got %q, want BCAST-123", evt.BroadcastID)
		}
		if evt.Recipient != "A1B2C3D4" {
			t.Fatalf("recipient: got %q, want A1B2C3D4", evt.Recipient)
		}
		if evt.Status != "delivered" {
			t.Fatalf("status: got %q, want delivered", evt.Status)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for OnBroadcastDelivery callback")
	}
}

func TestClient_OnBroadcastDelivery_UnknownFieldTolerance(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan BroadcastDelivery, 1)
	c.OnBroadcastDelivery(func(evt BroadcastDelivery) { received <- evt })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onBroadcastDelivery","params":{"broadcast_id":"BCAST-999","recipient":"FFFFFFFF","status":"failed","timestamp_ms":1730000000999,"unknown_field":"ignored","nested":{"extra":true}}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case evt := <-received:
		if evt.BroadcastID != "BCAST-999" {
			t.Fatalf("broadcast_id: got %q, want BCAST-999", evt.BroadcastID)
		}
		if evt.Status != "failed" {
			t.Fatalf("status: got %q, want failed", evt.Status)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for OnBroadcastDelivery callback with unknown fields")
	}
}

func TestClient_BroadcastOnChannelCritical(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"packet_id":"CAFEBABE","status":"sent"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := c.BroadcastOnChannelCritical(ctx, 7, "urgent channel")
	if err != nil {
		t.Fatalf("BroadcastOnChannelCritical error: %v", err)
	}
	if result.MessageID != "CAFEBABE" {
		t.Fatalf("MessageID: got %q, want CAFEBABE", result.MessageID)
	}
	if result.Status != "sent" {
		t.Fatalf("Status: got %q, want sent", result.Status)
	}
	if result.BroadcastID != "" {
		t.Fatalf("BroadcastID: got %q, want empty for sendMessage path", result.BroadcastID)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.sendMessage"`) {
		t.Fatalf("expected bramble.sendMessage request, got: %s", sent[0])
	}
	if !strings.Contains(sent[0], `"dest":"FFFFFFFE"`) || !strings.Contains(sent[0], `"channel":7`) {
		t.Fatalf("expected channel broadcast destination in request, got: %s", sent[0])
	}
	if !strings.Contains(sent[0], `"critical":true`) {
		t.Fatalf("expected critical=true in request, got: %s", sent[0])
	}
}
