// Package transport provides Transport implementations for connecting to Bramble mesh nodes.
// Supported transports: Serial (UART), WebSocket, and BLE (stub).
package transport

import (
	"context"
	"errors"
)

// Transport is the interface for connecting to a Bramble mesh node.
// All blocking operations accept a context for cancellation and timeout.
type Transport interface {
	// Connect establishes a connection to the node.
	Connect(ctx context.Context) error

	// Send sends a raw JSON byte payload to the node.
	Send(data []byte) error

	// Receive blocks until a complete JSON-RPC message is available,
	// the context is cancelled, or an error occurs.
	Receive(ctx context.Context) ([]byte, error)

	// Close closes the connection and releases any associated resources.
	Close() error

	// Info returns a human-readable description of the transport endpoint.
	Info() string
}

// ErrNotConnected is returned when an operation is attempted on an unconnected transport.
var ErrNotConnected = errors.New("bramble/transport: not connected")

// ErrClosed is returned when an operation is attempted on a closed transport.
var ErrClosed = errors.New("bramble/transport: transport closed")

// ErrReconnecting is returned when a method is called while a transport is reconnecting.
var ErrReconnecting = errors.New("bramble/transport: reconnecting")
