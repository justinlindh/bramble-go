//nolint:staticcheck // SA1019: tests intentionally exercise legacy nhooyr/websocket API used by production transport until migration is completed.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWebSocketConnectInitialDialSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "ok")
		<-r.Context().Done()
	}))
	defer srv.Close()

	w := NewWebSocket("ws" + srv.URL[len("http"):])
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer w.Close()
}

func TestWebSocketConnectSetsAuthorizationHeader(t *testing.T) {
	origDial := websocketDialFunc
	defer func() { websocketDialFunc = origDial }()

	var gotAuth string
	websocketDialFunc = func(ctx context.Context, u string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
		if opts != nil && opts.HTTPHeader != nil {
			gotAuth = opts.HTTPHeader.Get("Authorization")
		}
		return nil, nil, errors.New("expected dial failure")
	}

	w := NewWebSocket("ws://example.invalid")
	w.AuthToken = "secret-token"
	err := w.Connect(context.Background())
	if err == nil {
		t.Fatal("expected Connect error")
	}
	if !strings.Contains(gotAuth, "Bearer secret-token") {
		t.Fatalf("expected Authorization header to include bearer token, got %q", gotAuth)
	}
}

func TestWebSocketConnectInvalidURL(t *testing.T) {
	w := NewWebSocket("://bad-url")
	err := w.Connect(context.Background())
	if err == nil {
		t.Fatal("expected Connect error for invalid URL")
	}
}

func TestWebSocketConnectHandshakeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a websocket handshake"))
	}))
	defer srv.Close()

	w := NewWebSocket("ws" + srv.URL[len("http"):])
	err := w.Connect(context.Background())
	if err == nil {
		t.Fatal("expected Connect handshake error")
	}
}

func TestWebSocketSendReturnsReconnectingOnClosedConnection(t *testing.T) {
	origWrite := websocketWriteFunc
	defer func() { websocketWriteFunc = origWrite }()

	websocketWriteFunc = func(_ *websocket.Conn, _ context.Context, _ []byte) error {
		return fmt.Errorf("write failed: %w", net.ErrClosed)
	}

	w := NewWebSocket("ws://example.invalid")
	w.conn = &websocket.Conn{}
	if err := w.Send([]byte(`{"hello":"world"}`)); !errors.Is(err, ErrReconnecting) {
		t.Fatalf("expected ErrReconnecting, got %v", err)
	}
}

func TestWebSocketSendAndReceiveSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "ok")
		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if err := c.Write(r.Context(), websocket.MessageText, data); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	w := NewWebSocket("ws" + srv.URL[len("http"):])
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer w.Close()

	if err := w.Send([]byte(`{"hello":"world"}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	data, err := w.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if string(data) != `{"hello":"world"}` {
		t.Fatalf("unexpected echoed data: %s", string(data))
	}
}
