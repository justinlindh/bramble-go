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

const defaultBaudRate = 115200

// SerialOption is a functional option for configuring a Serial transport.
type SerialOption func(*Serial)

// WithBaudRate sets the baud rate for the serial connection.
func WithBaudRate(baud int) SerialOption {
	return func(s *Serial) {
		s.baud = baud
	}
}

// Serial is a Transport that communicates with a Bramble node over a UART/serial port.
// Messages are newline-delimited JSON. Lines not starting with '{' (e.g., ESP-IDF log
// output and console prompts) are skipped transparently.
type Serial struct {
	port    string
	baud    int
	mu      sync.Mutex
	conn    serial.Port
	scanner *bufio.Scanner
	recvCh  chan []byte
	errCh   chan error
	done    chan struct{}
	once    sync.Once
}

// NewSerial creates a new Serial transport for the given port.
// Default baud rate is 115200; override with WithBaudRate.
func NewSerial(port string, opts ...SerialOption) *Serial {
	s := &Serial{
		port:   port,
		baud:   defaultBaudRate,
		recvCh: make(chan []byte, 32),
		errCh:  make(chan error, 1),
		done:   make(chan struct{}),
	}
	for _, o := range opts {
		o(s)
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

	conn, err := serial.Open(s.port, mode)
	if err != nil {
		return fmt.Errorf("bramble/transport/serial: open %s: %w", s.port, err)
	}
	s.conn = conn
	s.scanner = bufio.NewScanner(conn)

	go s.reader()
	return nil
}

// reader is the background goroutine that reads lines from the serial port.
// Lines that don't start with '{' are ESP-IDF logs or prompts and are discarded.
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
				select {
				case s.errCh <- fmt.Errorf("bramble/transport/serial: read: %w", err):
				default:
				}
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

// Send writes a JSON payload followed by a newline to the serial port.
func (s *Serial) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// Ensure Serial implements Transport at compile time.
var _ Transport = (*Serial)(nil)
