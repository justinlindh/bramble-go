package bramble

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/justinlindh/bramble-go/transport"
)

func newConnectedProtocol(t *testing.T) (*Protocol, *transport.MockTransport) {
	t.Helper()
	mock := transport.NewMock()
	ctx := context.Background()
	if err := mock.Connect(ctx); err != nil {
		t.Fatalf("mock connect: %v", err)
	}
	p := NewProtocol(mock)
	p.Start()
	return p, mock
}

// TestProtocol_Call verifies that Call serialises the request and deserialises the response.
func TestProtocol_Call(t *testing.T) {
	p, mock := newConnectedProtocol(t)
	defer p.Stop()

	// Queue a response before calling so the reader goroutine can pick it up.
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"pong":true,"address":"AABBCCDD","protocol_version":"0.1.0"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	raw, err := p.Call(ctx, "bramble.ping", nil)
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}

	var result PingResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.Pong {
		t.Error("expected pong=true")
	}
	if result.Address != "AABBCCDD" {
		t.Errorf("address: got %q, want %q", result.Address, "AABBCCDD")
	}

	// Verify the request was sent correctly.
	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	var req rpcRequest
	if err := json.Unmarshal([]byte(sent[0]), &req); err != nil {
		t.Fatalf("unmarshal sent request: %v", err)
	}
	if req.Method != "bramble.ping" {
		t.Errorf("method: got %q, want %q", req.Method, "bramble.ping")
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("jsonrpc: got %q, want %q", req.JSONRPC, "2.0")
	}
}

// TestProtocol_RPCError verifies that RPC error responses are propagated as errors.
func TestProtocol_RPCError(t *testing.T) {
	p, mock := newConnectedProtocol(t)
	defer p.Stop()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := p.Call(ctx, "bramble.unknown", nil)
	if err == nil {
		t.Fatal("expected error for RPC error response")
	}
}

// TestProtocol_Timeout verifies that Call returns an error when the context times out.
func TestProtocol_Timeout(t *testing.T) {
	p, mock := newConnectedProtocol(t)
	defer p.Stop()
	_ = mock // no response queued

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Call(ctx, "bramble.ping", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// TestProtocol_Notification verifies that unsolicited notifications reach the channel.
func TestProtocol_Notification(t *testing.T) {
	p, mock := newConnectedProtocol(t)
	defer p.Stop()

	// Queue a notification (no "id" field).
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onMessage","params":{"from":"1191C6E0","to":"6EEA8967","text":"hi","timestamp":1000}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case n := <-p.Notifications():
		if n.Method != "bramble.onMessage" {
			t.Errorf("method: got %q, want %q", n.Method, "bramble.onMessage")
		}
		var m Message
		if err := json.Unmarshal(n.Params, &m); err != nil {
			t.Fatalf("unmarshal notification params: %v", err)
		}
		if m.Text != "hi" {
			t.Errorf("text: got %q, want %q", m.Text, "hi")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for notification")
	}
}

// TestProtocol_SequentialCalls verifies multiple calls are correctly correlated by ID.
func TestProtocol_SequentialCalls(t *testing.T) {
	p, mock := newConnectedProtocol(t)
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 1; i <= 3; i++ {
		mock.QueueResponse(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"idx":%d}}`, i, i))
		_, err := p.Call(ctx, "bramble.getStatus", nil)
		if err != nil {
			t.Errorf("call %d error: %v", i, err)
		}
	}
}
