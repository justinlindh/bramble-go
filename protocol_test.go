package bramble

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
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

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"pong":true,"address":"AABBCCDD","protocol_version":"0.2.0"}}`)

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
		t.Errorf("address: got %q, want AABBCCDD", result.Address)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	var req rpcRequest
	if err := json.Unmarshal([]byte(sent[0]), &req); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if req.Method != "bramble.ping" {
		t.Errorf("method: got %q, want bramble.ping", req.Method)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("jsonrpc: got %q, want 2.0", req.JSONRPC)
	}
}

// TestProtocol_RPCError verifies that RPC error responses surface as errors.
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

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onMessage","params":{"from":"00000001","to":"00000002","text":"hi","timestamp":1000}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case n := <-p.Notifications():
		if n.Method != "bramble.onMessage" {
			t.Errorf("method: got %q, want bramble.onMessage", n.Method)
		}
		var m Message
		if err := json.Unmarshal(n.Params, &m); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if m.Text != "hi" {
			t.Errorf("text: got %q, want hi", m.Text)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for notification")
	}
}

// TestProtocol_ConcurrentCalls verifies multiple concurrent calls are correctly correlated.
func TestProtocol_ConcurrentCalls(t *testing.T) {
	p, mock := newConnectedProtocol(t)
	defer p.Stop()

	const n = 5
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := p.Call(ctx, "bramble.getStatus", nil)
			errs <- err
		}()
	}

	// Give goroutines time to register pending channels, then feed responses.
	time.Sleep(20 * time.Millisecond)
	for i := 1; i <= n; i++ {
		mock.QueueResponse(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"idx":%d}}`, i, i))
	}

	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent call error: %v", err)
		}
	}
}

type flakyReconnectTransport struct {
	mu    sync.Mutex
	sends int
	recv  chan []byte
}

func (f *flakyReconnectTransport) Connect(context.Context) error { return nil }
func (f *flakyReconnectTransport) Close() error                  { return nil }
func (f *flakyReconnectTransport) Info() string                  { return "flaky" }
func (f *flakyReconnectTransport) SetAuthToken(_ string)         {}
func (f *flakyReconnectTransport) GetAuthToken() string          { return "" }

func (f *flakyReconnectTransport) Send(_ context.Context, _ []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends++
	if f.sends == 1 {
		return transport.ErrReconnecting
	}
	return nil
}

func (f *flakyReconnectTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-f.recv:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestProtocol_CallRetriesWhileTransportReconnecting(t *testing.T) {
	tpt := &flakyReconnectTransport{recv: make(chan []byte, 1)}
	p := NewProtocol(tpt)
	p.Start()
	defer p.Stop()

	tpt.recv <- []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	raw, err := p.Call(ctx, "bramble.ping", nil)
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("unexpected result: %s", raw)
	}

	tpt.mu.Lock()
	defer tpt.mu.Unlock()
	if tpt.sends < 2 {
		t.Fatalf("expected retry send, got sends=%d", tpt.sends)
	}
}
