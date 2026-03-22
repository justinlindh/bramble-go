package transport

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

var (
	serialOpenFunc = serial.Open
	serialSleep    = time.Sleep
)

const defaultBaudRate = 115200

// Serial is a Transport that communicates with a Bramble node over a UART/serial port.
// Messages are newline-delimited JSON. Lines not starting with '{' (e.g., ESP-IDF log
// output and console prompts) are skipped transparently.
//
// If the connection drops unexpectedly (e.g., USB unplug), Serial will automatically
// attempt to reconnect using exponential backoff (1s, 2s, 4s, 8s, … up to 30s).
// During reconnection, Send returns ErrReconnecting.
type Serial struct {
	port      string
	baud      int
	authToken string
	mu        sync.Mutex
	conn      serial.Port
	scanner   *bufio.Scanner
	recvCh    chan []byte
	errCh     chan error
	done      chan struct{}
	once      sync.Once

	// reconnecting is true while a reconnect attempt is in progress.
	reconnecting bool

	// OnDisconnect is called (if non-nil) when the connection is lost unexpectedly.
	OnDisconnect func()

	// OnReconnect is called (if non-nil) after a successful reconnection.
	OnReconnect func()
}

// NewSerial creates a new Serial transport for the given port.
// Default baud rate is 115200; override with WithBaudRate.
func NewSerial(port string, opts ...Option) *Serial {
	s := &Serial{
		port:   port,
		baud:   defaultBaudRate,
		recvCh: make(chan []byte, 32),
		errCh:  make(chan error, 1),
		done:   make(chan struct{}),
	}
	for _, o := range opts {
		o.applySerial(s)
	}
	return s
}

// Connect opens the serial port and starts the background reader goroutine.
func (s *Serial) Connect(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mode := &serial.Mode{
		BaudRate: s.baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	conn, err := serialOpenFunc(s.port, mode)
	if err != nil {
		return fmt.Errorf("bramble/transport/serial: open %s: %w", s.port, err)
	}
	s.conn = conn
	s.scanner = bufio.NewScanner(conn)

	if err := s.authenticate(); err != nil {
		_ = conn.Close()
		s.conn = nil
		return err
	}

	go s.reader()
	return nil
}

func (s *Serial) authenticate() error {
	if s.authToken == "" {
		return nil
	}

	if err := s.sendAuth(); err != nil {
		return err
	}
	return s.readAuthResponse()
}

func (s *Serial) sendAuth() error {
	msg := append(buildAuthRequest(s.authToken), '\n')
	_, err := s.conn.Write(msg)
	if err != nil {
		return fmt.Errorf("bramble/transport/serial: auth write: %w", err)
	}
	return nil
}

func (s *Serial) readAuthResponse() error {
	deadlineConn, ok := s.conn.(interface{ SetReadDeadline(time.Time) error })
	if ok {
		_ = deadlineConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		defer func() { _ = deadlineConn.SetReadDeadline(time.Time{}) }()
	}

	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		if err := validateAuthResponse([]byte(line)); err != nil {
			return fmt.Errorf("bramble/transport/serial: auth validate: %w", err)
		}
		return nil
	}
	if err := s.scanner.Err(); err != nil {
		return fmt.Errorf("bramble/transport/serial: auth read: %w", err)
	}
	return fmt.Errorf("bramble/transport/serial: auth read: empty response")
}

// reader is the background goroutine that reads lines from the serial port.
// Lines that don't start with '{' are ESP-IDF logs or prompts and are discarded.
// On fatal read errors (e.g., USB disconnect), it triggers automatic reconnection.
func (s *Serial) reader() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		// Set a generous read deadline so we can check done periodically.
		if dl, ok := s.conn.(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = dl.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		}

		if !s.scanner.Scan() {
			err := s.scanner.Err()
			select {
			case <-s.done:
				return
			default:
			}
			if err != nil {
				// Fatal read error (USB disconnect, etc.) — attempt reconnect.
				if rerr := s.reconnect(); rerr != nil {
					select {
					case s.errCh <- fmt.Errorf("bramble/transport/serial: reconnect failed: %w", rerr):
					default:
					}
					return
				}
				// Reconnected — continue reading with new scanner.
				continue
			}
			// If Scan returned false with no error it may be a timeout — retry.
			s.scanner = bufio.NewScanner(s.conn)
			continue
		}

		line := strings.TrimSpace(s.scanner.Text())
		if !strings.HasPrefix(line, "{") {
			// Skip ESP-IDF log lines, prompts, blank lines, etc.
			continue
		}

		cp := []byte(line)
		select {
		case s.recvCh <- cp:
		case <-s.done:
			return
		}
	}
}

// reconnect attempts to re-open the serial port with exponential backoff.
// Backoff: 1s, 2s, 4s, 8s, 16s, 30s (max). Stops if Close() is called.
func (s *Serial) reconnect() error {
	s.mu.Lock()
	if s.reconnecting {
		s.mu.Unlock()
		return nil
	}
	s.reconnecting = true
	// Close the old connection.
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	onDisconnect := s.OnDisconnect
	s.mu.Unlock()

	if onDisconnect != nil {
		onDisconnect()
	}

	delay := time.Second
	const maxDelay = 30 * time.Second

	mode := &serial.Mode{
		BaudRate: s.baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	for {
		select {
		case <-s.done:
			s.mu.Lock()
			s.reconnecting = false
			s.mu.Unlock()
			return ErrClosed
		default:
		}

		serialSleep(delay)

		conn, err := serialOpenFunc(s.port, mode)
		if err == nil {
			s.mu.Lock()
			s.conn = conn
			s.scanner = bufio.NewScanner(conn)
			s.mu.Unlock()

			if err := s.authenticate(); err != nil {
				_ = conn.Close()
				s.mu.Lock()
				s.conn = nil
				s.scanner = nil
				s.mu.Unlock()
			} else {
				s.mu.Lock()
				s.reconnecting = false
				onReconnect := s.OnReconnect
				s.mu.Unlock()

				if onReconnect != nil {
					onReconnect()
				}
				return nil
			}
		}

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// Send writes a JSON payload followed by a newline to the serial port.
func (s *Serial) Send(_ context.Context, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reconnecting {
		return ErrReconnecting
	}
	if s.conn == nil {
		return ErrNotConnected
	}

	msg := append(data, '\n')
	_, err := s.conn.Write(msg)
	if err != nil {
		return fmt.Errorf("bramble/transport/serial: write: %w", err)
	}
	return nil
}

// Receive blocks until a complete JSON message is available, the context is
// cancelled, or a read error occurs.
func (s *Serial) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-s.recvCh:
		return data, nil
	case err := <-s.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrClosed
	}
}

// Close stops the background reader and closes the serial port.
func (s *Serial) Close() error {
	var err error
	s.once.Do(func() {
		close(s.done)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.conn != nil {
			err = s.conn.Close()
		}
	})
	return err
}

// Info returns a human-readable description of the serial endpoint.
func (s *Serial) Info() string {
	return fmt.Sprintf("serial:%s@%d", s.port, s.baud)
}

// SetAuthToken sets the authentication token for the serial transport.
func (s *Serial) SetAuthToken(token string) {
	s.authToken = token
}

// GetAuthToken returns the current authentication token for the serial transport.
func (s *Serial) GetAuthToken() string {
	return s.authToken
}

// Ensure Serial implements Transport at compile time.
var _ Transport = (*Serial)(nil)
