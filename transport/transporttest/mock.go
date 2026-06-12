// Package transporttest provides in-memory transport.Transport implementations
// for testing code that consumes the bramble-go SDK.
package transporttest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/justinlindh/bramble-go/transport"
)

// Mock is an in-memory transport.Transport implementation for testing.
// Responses are queued via QueueResponse and delivered in FIFO order.
type Mock struct {
	mu        sync.Mutex
	sent      [][]byte
	connected bool
	recvCh    chan []byte
	queueSeq  int
	authToken string
}

// NewMock creates a new Mock transport ready for use in tests.
func NewMock() *Mock {
	return &Mock{
		recvCh: make(chan []byte, 64),
	}
}

// Connect marks the transport as connected.
func (m *Mock) Connect(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	return nil
}

// Send records the outgoing payload for later inspection via Sent().
func (m *Mock) Send(_ context.Context, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return transport.ErrNotConnected
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.sent = append(m.sent, cp)
	return nil
}

// Receive blocks until a queued response is available or the context is cancelled.
func (m *Mock) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-m.recvCh:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close marks the transport as disconnected and drains the receive channel.
func (m *Mock) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	// Drain without blocking
	for {
		select {
		case <-m.recvCh:
		default:
			return nil
		}
	}
}

// Info returns a description of the mock transport.
func (m *Mock) Info() string { return "mock" }

// QueueResponse enqueues a raw JSON string to be returned by the next Receive call.
func (m *Mock) QueueResponse(js string) {
	m.mu.Lock()
	m.queueSeq++
	delay := time.Duration(m.queueSeq) * time.Millisecond
	m.mu.Unlock()

	go func() {
		time.Sleep(delay)
		m.recvCh <- []byte(js)
	}()
}

// InjectAsync enqueues raw bytes from a goroutine, simulating asynchronous inbound data.
func (m *Mock) InjectAsync(data []byte) {
	cp := make([]byte, len(data))
	copy(cp, data)
	go func() {
		m.recvCh <- cp
	}()
}

// Sent returns all payloads that have been sent via Send(), as strings.
func (m *Mock) Sent() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.sent))
	for i, b := range m.sent {
		out[i] = string(b)
	}
	return out
}

// IsConnected reports whether the mock transport is currently connected.
func (m *Mock) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// SetAuthToken updates the mock transport auth token.
func (m *Mock) SetAuthToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authToken = token
}

// AuthToken returns the current mock transport auth token.
func (m *Mock) AuthToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.authToken
}

// Ensure Mock implements Transport at compile time.
var _ transport.Transport = (*Mock)(nil)

// MockError is a Mock transport that returns an error on Send.
// Useful for testing error paths.
type MockError struct {
	*Mock
	SendErr error
}

// NewMockError creates a Mock transport that returns sendErr on every Send call.
func NewMockError(sendErr error) *MockError {
	return &MockError{Mock: NewMock(), SendErr: sendErr}
}

// Send always returns the configured error.
func (m *MockError) Send(_ context.Context, _ []byte) error {
	return fmt.Errorf("mock send error: %w", m.SendErr)
}
