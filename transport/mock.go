package transport

import (
	"context"
	"fmt"
	"sync"
)

// MockTransport is an in-memory transport for testing.
// Responses are queued via QueueResponse and delivered in FIFO order.
type MockTransport struct {
	mu        sync.Mutex
	responses [][]byte
	sent      [][]byte
	connected bool
	recvCh    chan []byte
}

// NewMock creates a new MockTransport ready for use in tests.
func NewMock() *MockTransport {
	return &MockTransport{
		recvCh: make(chan []byte, 64),
	}
}

// Connect marks the transport as connected.
func (m *MockTransport) Connect(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	return nil
}

// Send records the outgoing payload for later inspection via Sent().
func (m *MockTransport) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return ErrNotConnected
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.sent = append(m.sent, cp)
	return nil
}

// Receive blocks until a queued response is available or the context is cancelled.
func (m *MockTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-m.recvCh:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close marks the transport as disconnected and drains the receive channel.
func (m *MockTransport) Close() error {
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
func (m *MockTransport) Info() string { return "mock" }

// QueueResponse enqueues a raw JSON string to be returned by the next Receive call.
func (m *MockTransport) QueueResponse(js string) {
	m.recvCh <- []byte(js)
}

// Sent returns all payloads that have been sent via Send(), as strings.
func (m *MockTransport) Sent() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.sent))
	for i, b := range m.sent {
		out[i] = string(b)
	}
	return out
}

// IsConnected reports whether the mock transport is currently connected.
func (m *MockTransport) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// Ensure MockTransport implements Transport at compile time.
var _ Transport = (*MockTransport)(nil)

// MockTransportError is a MockTransport that returns an error on Send.
// Useful for testing error paths.
type MockTransportError struct {
	*MockTransport
	SendErr error
}

// NewMockError creates a MockTransport that returns sendErr on every Send call.
func NewMockError(sendErr error) *MockTransportError {
	return &MockTransportError{MockTransport: NewMock(), SendErr: sendErr}
}

// Send always returns the configured error.
func (m *MockTransportError) Send(_ []byte) error {
	return fmt.Errorf("mock send error: %w", m.SendErr)
}
