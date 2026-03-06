package transport

// Option is a functional option shared by transport constructors.
type Option interface {
	applySerial(*Serial)
	applyWebSocket(*WebSocket)
}

type option struct {
	serial    func(*Serial)
	websocket func(*WebSocket)
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

// WithBaudRate sets the baud rate for serial connections.
func WithBaudRate(baud int) Option {
	return option{serial: func(s *Serial) { s.baud = baud }}
}

// WithAuthToken sets the auth token used by transports that support authentication.
func WithAuthToken(token string) Option {
	return option{
		serial:    func(s *Serial) { s.AuthToken = token },
		websocket: func(w *WebSocket) { w.AuthToken = token },
	}
}
