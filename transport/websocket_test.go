package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestWebSocketReconnectBackoffAndCallbacks(t *testing.T) {
	origDial := websocketDialFunc
	origSleep := websocketSleep
	defer func() {
		websocketDialFunc = origDial
		websocketSleep = origSleep
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "ok")
		<-r.Context().Done()
	}))
	defer srv.Close()
	wsURL := "ws" + srv.URL[len("http"):]

	var calls int32
	websocketDialFunc = func(ctx context.Context, u string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
		if atomic.AddInt32(&calls, 1) < 3 {
			return nil, nil, errors.New("dial failed")
		}
		return websocket.Dial(ctx, u, opts)
	}

	var delays []time.Duration
	websocketSleep = func(d time.Duration) { delays = append(delays, d) }

	w := NewWebSocket(wsURL)
	dc, rc := 0, 0
	w.OnDisconnect = func() { dc++ }
	w.OnReconnect = func() { rc++ }

	if err := w.reconnect(); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	defer w.Close()

	if calls != 3 {
		t.Fatalf("expected 3 dial attempts, got %d", calls)
	}
	if len(delays) < 3 || delays[0] != time.Second || delays[1] != 2*time.Second || delays[2] != 4*time.Second {
		t.Fatalf("unexpected delays: %v", delays)
	}
	if dc != 1 || rc != 1 {
		t.Fatalf("unexpected callbacks disconnect=%d reconnect=%d", dc, rc)
	}
}

func TestWebSocketSendWhenReconnecting(t *testing.T) {
	w := NewWebSocket("ws://example.invalid")
	w.reconnecting = true
	if err := w.Send([]byte("{}")); !errors.Is(err, ErrReconnecting) {
		t.Fatalf("expected ErrReconnecting, got %v", err)
	}
}

func TestWebSocketReceiveNotConnected(t *testing.T) {
	w := NewWebSocket("ws://example.invalid")
	if _, err := w.Receive(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestWebSocketReconnectStopsOnClose(t *testing.T) {
	origDial := websocketDialFunc
	origSleep := websocketSleep
	defer func() {
		websocketDialFunc = origDial
		websocketSleep = origSleep
	}()

	w := NewWebSocket("ws://example.invalid")
	websocketDialFunc = func(context.Context, string, *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
		return nil, nil, errors.New("down")
	}
	websocketSleep = func(_ time.Duration) { _ = w.Close() }

	if err := w.reconnect(); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}
