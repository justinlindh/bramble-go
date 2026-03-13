package transport

import "time"

// Option is a functional option shared by transport constructors.
type Option interface {
	applySerial(*Serial)
	applyWebSocket(*WebSocket)
	applyBLE(*BLE)
}

type option struct {
	serial    func(*Serial)
	websocket func(*WebSocket)
	ble       func(*BLE)
}

func (o option) applySerial(s *Serial) {
	if o.serial != nil {
		o.serial(s)
	}
}

func (o option) applyWebSocket(w *WebSocket) {
	if o.websocket != nil {
		o.websocket(w)
	}
}

func (o option) applyBLE(b *BLE) {
	if o.ble != nil {
		o.ble(b)
	}
}

// WithBaudRate sets the baud rate for serial connections.
func WithBaudRate(baud int) Option {
	return option{serial: func(s *Serial) { s.baud = baud }}
}

// WithAuthToken sets the auth token used by transports that support authentication.
func WithAuthToken(token string) Option {
	return option{
		serial:    func(s *Serial) { s.authToken = token },
		websocket: func(w *WebSocket) { w.authToken = token },
		ble:       func(b *BLE) { b.cfg.AuthToken = token },
	}
}

// WithBLEScanTimeout sets the BLE scan timeout duration.
func WithBLEScanTimeout(d time.Duration) Option {
	return option{ble: func(b *BLE) { b.cfg.ScanTimeout = d }}
}
