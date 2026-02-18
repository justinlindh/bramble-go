package bramble

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/justinlindh/bramble-go/transport"
)

func setupClient(t *testing.T) (*Client, *transport.MockTransport) {
	t.Helper()
	mock := transport.NewMock()
	client := NewClient(mock)
	return client, mock
}

func connectClient(t *testing.T, client *Client, mock *transport.MockTransport) {
	t.Helper()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"firmware_version":"0.1.0-dev","protocol_version":"0.1.0","hardware":"heltec_v3"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

func TestClientConnect(t *testing.T) {
	client, mock := setupClient(t)
	defer client.Close()
	connectClient(t, client, mock)
}

func TestClientConnectIncompatible(t *testing.T) {
	client, mock := setupClient(t)
	defer client.Close()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"firmware_version":"2.0.0","protocol_version":"2.0.0","hardware":"heltec_v3"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for incompatible version")
	}
}

func TestClientStatus(t *testing.T) {
	client, mock := setupClient(t)
	defer client.Close()
	connectClient(t, client, mock)

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"address":"1191C6E0","firmware_version":"0.1.0-dev","protocol_version":"0.1.0","hardware":"heltec_v3","radio_ok":true,"peers":2,"beacon_tx":10,"beacon_rx":8,"packets_tx":5,"packets_rx":3,"uptime_s":120}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Address != "1191C6E0" {
		t.Errorf("expected address 1191C6E0, got %s", status.Address)
	}
	if status.Peers != 2 {
		t.Errorf("expected 2 peers, got %d", status.Peers)
	}
	if !status.RadioOK {
		t.Error("expected radio_ok true")
	}
}

func TestClientPing(t *testing.T) {
	client, mock := setupClient(t)
	defer client.Close()
	connectClient(t, client, mock)

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"pong":true}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestClientSend(t *testing.T) {
	client, mock := setupClient(t)
	defer client.Close()
	connectClient(t, client, mock)

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"message_id":"abc123","status":"sent"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := client.Send(ctx, "6EEA8967", "hello")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if result.MessageID != "abc123" {
		t.Errorf("expected message_id abc123, got %s", result.MessageID)
	}

	sent := mock.Sent()
	if len(sent) < 2 {
		t.Fatalf("expected at least 2 sent messages, got %d", len(sent))
	}
	var req rpcRequest
	json.Unmarshal([]byte(sent[len(sent)-1]), &req)
	if req.Method != "bramble.sendMessage" {
		t.Errorf("expected method bramble.sendMessage, got %s", req.Method)
	}
}

func TestClientRPCError(t *testing.T) {
	client, mock := setupClient(t)
	defer client.Close()
	connectClient(t, client, mock)

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"error":{"code":-1001,"message":"Radio not ready"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Status(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	// Verify we got an error (the exact type is internal)
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
