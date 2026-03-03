package transport

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

var (
	websocketDialFunc = websocket.Dial
	websocketSleep    = time.Sleep
)

// WebSocket is a Transport that communicates with a Bramble node over a WebSocket connection.
// Each WebSocket message frame carries exactly one JSON-RPC message; no newline framing is used.
//
// If the connection drops unexpectedly, WebSocket will automatically attempt to reconnect
// using exponential backoff (1s, 2s, 4s, 8s, … up to 30s). During reconnection, Send
// returns ErrReconnecting. The Receive loop detects disconnects and drives reconnection.
type WebSocket struct {
	url       string
	AuthToken string
	mu        sync.Mutex
	conn      *websocket.Conn
	done      chan struct{}
	once      sync.Once

	// reconnecting is true while a reconnect attempt is in progress.
	reconnecting bool

	// OnDisconnect is called (if non-nil) when the connection is lost unexpectedly.
	OnDisconnect func()

	// OnReconnect is called (if non-nil) after a successful reconnection.
	OnReconnect func()
}

// NewWebSocket creates a new WebSocket transport for the given URL.
// The URL should use the ws:// or wss:// scheme (e.g. "ws://192.168.4.1/ws").
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

	conn, _, err := websocketDialFunc(ctx, w.url, w.dialOptions())
	if err != nil {
		return fmt.Errorf("bramble/transport/websocket: dial %s: %w", w.url, err)
	}
	conn.SetReadLimit(512 * 1024)
	w.conn = conn
	return nil
}

// Send encodes a JSON payload as a single WebSocket text message.
func (w *WebSocket) Send(data []byte) error {
	w.mu.Lock()
	conn := w.conn
	reconnecting := w.reconnecting
	w.mu.Unlock()

	if reconnecting {
		return ErrReconnecting
	}
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
// On unexpected disconnect, it triggers automatic reconnection with exponential backoff.
func (w *WebSocket) Receive(ctx context.Context) ([]byte, error) {
	w.mu.Lock()
	conn := w.conn
	w.mu.Unlock()

	if conn == nil {
		return nil, ErrNotConnected
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		// Check if we're intentionally closed.
		select {
		case <-w.done:
			return nil, ErrClosed
		default:
		}

		// Check if context was cancelled (not a disconnect).
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Unexpected disconnect — attempt reconnect.
		if rerr := w.reconnect(); rerr != nil {
			return nil, fmt.Errorf("bramble/transport/websocket: reconnect failed: %w", rerr)
		}

		// Reconnected — return a sentinel so the caller retries.
		return nil, ErrReconnecting
	}
	return data, nil
}

// reconnect attempts to re-establish the WebSocket connection with exponential backoff.
// Backoff: 1s, 2s, 4s, 8s, 16s, 30s (max). Stops if Close() is called.
func (w *WebSocket) reconnect() error {
	w.mu.Lock()
	if w.reconnecting {
		w.mu.Unlock()
		return nil // another goroutine is already reconnecting
	}
	w.reconnecting = true
	onDisconnect := w.OnDisconnect
	w.mu.Unlock()

	if onDisconnect != nil {
		onDisconnect()
	}

	delay := time.Second
	const maxDelay = 30 * time.Second

	for {
		select {
		case <-w.done:
			w.mu.Lock()
			w.reconnecting = false
			w.mu.Unlock()
			return ErrClosed
		default:
		}

		websocketSleep(delay)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, _, err := websocketDialFunc(ctx, w.url, w.dialOptions())
		cancel()

		if err == nil {
			conn.SetReadLimit(512 * 1024)
			w.mu.Lock()
			w.conn = conn
			w.reconnecting = false
			onReconnect := w.OnReconnect
			w.mu.Unlock()

			if onReconnect != nil {
				onReconnect()
			}
			return nil
		}

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

func (w *WebSocket) dialOptions() *websocket.DialOptions {
	if w.AuthToken == "" {
		return nil
	}

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+w.AuthToken)
	return &websocket.DialOptions{HTTPHeader: headers}
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
