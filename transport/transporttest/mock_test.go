package transporttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/justinlindh/bramble-go/transport"
)

func TestMockConnectAndClose(t *testing.T) {
	m := NewMock()
	if m.IsConnected() {
		t.Fatal("expected disconnected initially")
	}
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !m.IsConnected() {
		t.Fatal("expected connected after Connect")
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if m.IsConnected() {
		t.Fatal("expected disconnected after Close")
	}
}

func TestMockSendWhenDisconnected(t *testing.T) {
	m := NewMock()
	if err := m.Send(context.Background(), []byte(`{"x":1}`)); !errors.Is(err, transport.ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestMockReceiveTimeout(t *testing.T) {
	m := NewMock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := m.Receive(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestMockQueueResponseOrderingAndSent(t *testing.T) {
	m := NewMock()
	_ = m.Connect(context.Background())

	if err := m.Send(context.Background(), []byte(`one`)); err != nil {
		t.Fatalf("Send one: %v", err)
	}
	if err := m.Send(context.Background(), []byte(`two`)); err != nil {
		t.Fatalf("Send two: %v", err)
	}
	sent := m.Sent()
	if len(sent) != 2 || sent[0] != "one" || sent[1] != "two" {
		t.Fatalf("unexpected sent data: %v", sent)
	}

	m.QueueResponse(`first`)
	m.QueueResponse(`second`)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	b1, err := m.Receive(ctx)
	if err != nil || string(b1) != "first" {
		t.Fatalf("first receive: data=%q err=%v", string(b1), err)
	}
	b2, err := m.Receive(ctx)
	if err != nil || string(b2) != "second" {
		t.Fatalf("second receive: data=%q err=%v", string(b2), err)
	}
}

func TestMockInjectAsync(t *testing.T) {
	m := NewMock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	m.InjectAsync([]byte(`async`))
	b, err := m.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive async: %v", err)
	}
	if string(b) != "async" {
		t.Fatalf("unexpected async payload: %q", string(b))
	}
}

func TestMockInfoAndMockError(t *testing.T) {
	m := NewMock()
	if m.Info() != "mock" {
		t.Fatalf("unexpected info: %q", m.Info())
	}

	sentinel := errors.New("boom")
	me := NewMockError(sentinel)
	err := me.Send(context.Background(), []byte("x"))
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel error, got %v", err)
	}
}

func TestMock_SetAuthToken(t *testing.T) {
	m := NewMock()

	if m.authToken != "" {
		t.Fatalf("expected empty token by default, got %q", m.authToken)
	}

	m.SetAuthToken("token-1")
	if m.authToken != "token-1" {
		t.Fatalf("expected token-1, got %q", m.authToken)
	}

	m.SetAuthToken("token-2")
	if m.authToken != "token-2" {
		t.Fatalf("expected overwritten token-2, got %q", m.authToken)
	}

	m.SetAuthToken("")
	if m.authToken != "" {
		t.Fatalf("expected empty token, got %q", m.authToken)
	}
}
