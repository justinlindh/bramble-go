package transport

import (
	"context"
	"fmt"
	"sync"

	"nhooyr.io/websocket"
)

// WebSocket is a Transport that communicates with a Bramble node over a WebSocket connection.
// Each WebSocket message frame carries exactly one JSON-RPC message; no newline framing is used.
type WebSocket struct {
	url    string
	mu     sync.Mutex
	conn   *websocket.Conn
	done   chan struct{}
	once   sync.Once
}

// NewWebSocket creates a new WebSocket transport for the given URL.
// The URL should use the ws:// or wss:// scheme (e.g. "ws://192.168.4.1/rpc").
func NewWebSocket(url string) *WebSocket {
	return &WebSocket{
		url:  url,
		done: make(chan struct{}),
	}
}

// Connect dials the WebSocket server and establishes a connection.
func (w *WebSocket) Connect(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	conn, _, err := websocket.Dial(ctx, w.url, nil)
	if err != nil {
		return fmt.Errorf("bramble/transport/websocket: dial %s: %w", w.url, err)
	}
	// Allow large payloads (default is 32KB which may be tight for full config dumps).
	conn.SetReadLimit(512 * 1024)
	w.conn = conn
	return nil
}

// Send encodes a JSON payload as a single WebSocket text message.
func (w *WebSocket) Send(data []byte) error {
	w.mu.Lock()
	conn := w.conn
	w.mu.Unlock()

	if conn == nil {
		return ErrNotConnected
	}

	ctx := context.Background()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("bramble/transport/websocket: write: %w", err)
	}
	return nil
}

// Receive blocks until a WebSocket message is available or the context is cancelled.
func (w *WebSocket) Receive(ctx context.Context) ([]byte, error) {
	w.mu.Lock()
	conn := w.conn
	w.mu.Unlock()

	if conn == nil {
		return nil, ErrNotConnected
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		select {
		case <-w.done:
			return nil, ErrClosed
		default:
		}
		return nil, fmt.Errorf("bramble/transport/websocket: read: %w", err)
	}
	return data, nil
}

// Close sends a WebSocket close frame and closes the connection.
func (w *WebSocket) Close() error {
	var err error
	w.once.Do(func() {
		close(w.done)
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.conn != nil {
			err = w.conn.Close(websocket.StatusNormalClosure, "client closing")
		}
	})
	return err
}

// Info returns a human-readable description of the WebSocket endpoint.
func (w *WebSocket) Info() string {
	return fmt.Sprintf("websocket:%s", w.url)
}

// Ensure WebSocket implements Transport at compile time.
var _ Transport = (*WebSocket)(nil)
