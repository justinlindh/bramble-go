package transport

import (
	"context"
	"errors"
)

// ErrBLENotImplemented is returned by all BLE transport methods.
// BLE support is planned for a future release.
var ErrBLENotImplemented = errors.New("bramble/transport: BLE not implemented")

// BLE is a stub BLE transport that returns ErrBLENotImplemented for every operation.
// It satisfies the Transport interface for compile-time compatibility.
type BLE struct{}

// NewBLE creates a new BLE transport stub.
func NewBLE() *BLE {
	return &BLE{}
}

// Connect always returns ErrBLENotImplemented.
func (b *BLE) Connect(_ context.Context) error { return ErrBLENotImplemented }

// Send always returns ErrBLENotImplemented.
func (b *BLE) Send(_ []byte) error { return ErrBLENotImplemented }

// Receive always returns ErrBLENotImplemented.
func (b *BLE) Receive(_ context.Context) ([]byte, error) { return nil, ErrBLENotImplemented }

// Close always returns ErrBLENotImplemented.
func (b *BLE) Close() error { return ErrBLENotImplemented }

// Info returns a description of the BLE transport.
func (b *BLE) Info() string { return "ble (not implemented)" }
