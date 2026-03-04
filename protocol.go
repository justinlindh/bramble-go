package bramble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/justinlindh/bramble-go/transport"
)

// rpcRequest is the JSON-RPC 2.0 request envelope sent to the node.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcResponse is the JSON-RPC 2.0 response envelope received from the node.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError holds a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// Notification is an unsolicited JSON-RPC notification from the node.
// The Method field identifies the event (e.g. "bramble.onMessage").
type Notification struct {
	// Method is the notification method name.
	Method string
	// Params is the raw JSON payload.
	Params json.RawMessage
}

// Protocol implements JSON-RPC 2.0 framing over a Transport.
// It multiplexes concurrent calls and dispatches unsolicited notifications.
type Protocol struct {
	t        transport.Transport
	nextID   atomic.Int64
	pending  sync.Map       // int64 -> chan *rpcResponse
	notifyCh chan Notification
	done     chan struct{}
	once     sync.Once
}

// NewProtocol creates a new Protocol wrapping the given transport.
// Call Start() to begin processing incoming messages.
func NewProtocol(t transport.Transport) *Protocol {
	return &Protocol{
		t:        t,
		notifyCh: make(chan Notification, 64),
		done:     make(chan struct{}),
	}
}

// Start launches the background reader goroutine that routes incoming JSON-RPC
// messages to the appropriate pending callers or the notification channel.
func (p *Protocol) Start() {
	go p.reader()
}

// reader is the background goroutine that reads from the transport and routes messages.
func (p *Protocol) reader() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-p.done
		cancel()
	}()

	for {
		select {
		case <-p.done:
			return
		default:
		}

		data, err := p.t.Receive(ctx)
		if err != nil {
			select {
			case <-p.done:
				return
			default:
				continue
			}
		}

		var msg rpcResponse
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		isResponse := msg.ID != nil && msg.Method == "" // result or error reply
		isNotification := msg.Method != "" && msg.ID == nil // server-pushed event

		if isResponse {
			// Response to a pending Call.
			if ch, ok := p.pending.Load(*msg.ID); ok {
				resp := msg
				ch.(chan *rpcResponse) <- &resp
			}
			continue
		}

		if isNotification {
			// Unsolicited notification.
			n := Notification{Method: msg.Method, Params: msg.Params}
			select {
			case p.notifyCh <- n:
			default:
				// Drop if consumer is slow.
			}
		}
		// Otherwise: message has both ID and Method — this is the serial echo of our
		// own request reflecting back from the device's console. Discard silently.
	}
}

// Call sends a JSON-RPC 2.0 request and waits for the response.
// It returns the raw JSON result field on success.
func (p *Protocol) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := p.nextID.Add(1)

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("bramble: marshal %s: %w", method, err)
	}

	ch := make(chan *rpcResponse, 1)
	p.pending.Store(id, ch)
	defer p.pending.Delete(id)

	for {
		err := p.t.Send(data)
		if err == nil {
			break
		}
		if !errors.Is(err, transport.ErrReconnecting) {
			return nil, fmt.Errorf("bramble: send %s: %w", method, err)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("bramble: %s: %w", method, ctx.Err())
		case <-p.done:
			return nil, fmt.Errorf("bramble: %s: %w", method, transport.ErrClosed)
		case <-time.After(50 * time.Millisecond):
		}
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("bramble: %s: %w", method, resp.Error)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("bramble: %s: %w", method, ctx.Err())
	case <-p.done:
		return nil, fmt.Errorf("bramble: %s: %w", method, transport.ErrClosed)
	}
}

// Notifications returns a read-only channel of unsolicited notifications from the node.
func (p *Protocol) Notifications() <-chan Notification {
	return p.notifyCh
}

// Stop signals the reader goroutine to exit. Safe to call multiple times.
func (p *Protocol) Stop() {
	p.once.Do(func() {
		close(p.done)
	})
}
