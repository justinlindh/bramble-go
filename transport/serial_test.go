package transport

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"
)

type fakeSerialPort struct {
	closed bool
	writes [][]byte
	mu     sync.Mutex
}

func (f *fakeSerialPort) SetMode(_ *serial.Mode) error     { return nil }
func (f *fakeSerialPort) Read(_ []byte) (n int, err error) { return 0, errors.New("read failure") }
func (f *fakeSerialPort) Drain() error                     { return nil }
func (f *fakeSerialPort) ResetInputBuffer() error          { return nil }
func (f *fakeSerialPort) ResetOutputBuffer() error         { return nil }
func (f *fakeSerialPort) SetDTR(_ bool) error              { return nil }
func (f *fakeSerialPort) SetRTS(_ bool) error              { return nil }
func (f *fakeSerialPort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}
func (f *fakeSerialPort) SetReadTimeout(_ time.Duration) error { return nil }
func (f *fakeSerialPort) Break(_ time.Duration) error          { return nil }

func (f *fakeSerialPort) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := append([]byte(nil), p...)
	f.writes = append(f.writes, cp)
	return len(p), nil
}

func (f *fakeSerialPort) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func TestSerialReconnectBackoffAndCallbacks(t *testing.T) {
	origOpen := serialOpenFunc
	origSleep := serialSleep
	defer func() {
		serialOpenFunc = origOpen
		serialSleep = origSleep
	}()

	s := NewSerial("/dev/fake")
	oldConn := &fakeSerialPort{}
	s.conn = oldConn

	var delays []time.Duration
	serialSleep = func(d time.Duration) { delays = append(delays, d) }

	attempts := 0
	newConn := &fakeSerialPort{}
	serialOpenFunc = func(_ string, _ *serial.Mode) (serial.Port, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("open failed")
		}
		return newConn, nil
	}

	disconnectCalled := 0
	reconnectCalled := 0
	s.OnDisconnect = func() { disconnectCalled++ }
	s.OnReconnect = func() { reconnectCalled++ }

	if err := s.reconnect(); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(delays) < 3 || delays[0] != time.Second || delays[1] != 2*time.Second || delays[2] != 4*time.Second {
		t.Fatalf("unexpected backoff delays: %v", delays)
	}
	if disconnectCalled != 1 || reconnectCalled != 1 {
		t.Fatalf("unexpected callbacks disconnect=%d reconnect=%d", disconnectCalled, reconnectCalled)
	}
	if !oldConn.closed {
		t.Fatal("old connection should be closed")
	}
	if s.conn != newConn {
		t.Fatal("serial should hold new connection after reconnect")
	}
}

func TestSerialSendReturnsErrReconnecting(t *testing.T) {
	s := NewSerial("/dev/fake")
	s.reconnecting = true
	if err := s.Send([]byte(`{"id":1}`)); !errors.Is(err, ErrReconnecting) {
		t.Fatalf("expected ErrReconnecting, got %v", err)
	}
}

func TestSerialReconnectStopsOnClose(t *testing.T) {
	origOpen := serialOpenFunc
	origSleep := serialSleep
	defer func() {
		serialOpenFunc = origOpen
		serialSleep = origSleep
	}()

	s := NewSerial("/dev/fake")
	serialOpenFunc = func(_ string, _ *serial.Mode) (serial.Port, error) { return nil, errors.New("still down") }
	serialSleep = func(_ time.Duration) {
		_ = s.Close()
	}

	if err := s.reconnect(); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestSerialReceiveContextCancel(t *testing.T) {
	s := NewSerial("/dev/fake")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Receive(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestBuildAuthRequest(t *testing.T) {
	got := string(buildAuthRequest("abc123"))
	want := `{"jsonrpc":"2.0","method":"bramble.auth","params":{"token":"abc123"},"id":0}`
	if got != want {
		t.Fatalf("unexpected auth request.\nwant: %s\n got: %s", want, got)
	}
}

type pipeSerialPort struct {
	pr     *io.PipeReader
	pw     *io.PipeWriter
	closed bool
	mu     sync.Mutex
	writes [][]byte
}

func newPipeSerialPort() *pipeSerialPort {
	pr, pw := io.Pipe()
	return &pipeSerialPort{pr: pr, pw: pw}
}

func (p *pipeSerialPort) SetMode(_ *serial.Mode) error { return nil }
func (p *pipeSerialPort) Read(b []byte) (int, error) { return p.pr.Read(b) }
func (p *pipeSerialPort) Drain() error { return nil }
func (p *pipeSerialPort) ResetInputBuffer() error { return nil }
func (p *pipeSerialPort) ResetOutputBuffer() error { return nil }
func (p *pipeSerialPort) SetDTR(_ bool) error { return nil }
func (p *pipeSerialPort) SetRTS(_ bool) error { return nil }
func (p *pipeSerialPort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}
func (p *pipeSerialPort) SetReadTimeout(_ time.Duration) error { return nil }
func (p *pipeSerialPort) Break(_ time.Duration) error          { return nil }
func (p *pipeSerialPort) SetReadDeadline(_ time.Time) error    { return nil }

func (p *pipeSerialPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := append([]byte(nil), b...)
	p.writes = append(p.writes, cp)
	return len(b), nil
}

func (p *pipeSerialPort) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	_ = p.pr.Close()
	return p.pw.Close()
}

func (p *pipeSerialPort) feed(line string) {
	_, _ = p.pw.Write([]byte(line))
}

func (p *pipeSerialPort) sent() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.writes))
	for _, w := range p.writes {
		out = append(out, string(w))
	}
	return out
}

func TestSerialConnect_WithAuthAndReader(t *testing.T) {
	origOpen := serialOpenFunc
	defer func() { serialOpenFunc = origOpen }()

	port := newPipeSerialPort()
	serialOpenFunc = func(_ string, _ *serial.Mode) (serial.Port, error) {
		return port, nil
	}

	s := NewSerial("/dev/fake", WithAuthToken("sekret"))
	if s.AuthToken != "sekret" {
		t.Fatalf("AuthToken: got %q, want sekret", s.AuthToken)
	}

	go func() {
		port.feed("boot log...\n")
		port.feed(`{"jsonrpc":"2.0","id":0,"result":{"ok":true}}` + "\n")
		port.feed("I (123) app: ready\n")
		port.feed(`{"jsonrpc":"2.0","method":"bramble.onMessage","params":{"text":"hello"}}` + "\n")
	}()

	if err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer s.Close()

	received, err := s.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	if got := string(received); !strings.Contains(got, `"method":"bramble.onMessage"`) {
		t.Fatalf("unexpected received payload: %s", got)
	}

	sent := port.sent()
	if len(sent) == 0 {
		t.Fatal("expected auth request to be written")
	}
	if !strings.Contains(sent[0], `"method":"bramble.auth"`) || !strings.Contains(sent[0], `"token":"sekret"`) {
		t.Fatalf("unexpected auth request write: %q", sent[0])
	}
}
