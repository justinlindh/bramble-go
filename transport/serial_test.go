package transport

import (
	"bufio"
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
	if err := s.Send(context.Background(), []byte(`{"id":1}`)); !errors.Is(err, ErrReconnecting) {
		t.Fatalf("expected ErrReconnecting, got %v", err)
	}
}

func TestSerialTransport_SetAuthToken(t *testing.T) {
	s := NewSerial("/dev/fake")

	if s.authToken != "" {
		t.Fatalf("expected empty token by default, got %q", s.authToken)
	}

	s.SetAuthToken("token-1")
	if s.authToken != "token-1" {
		t.Fatalf("expected token-1, got %q", s.authToken)
	}

	s.SetAuthToken("token-2")
	if s.authToken != "token-2" {
		t.Fatalf("expected overwritten token-2, got %q", s.authToken)
	}

	s.SetAuthToken("")
	if s.authToken != "" {
		t.Fatalf("expected empty token, got %q", s.authToken)
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

func TestSerialSendSuccessAndNotConnected(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s := NewSerial("/dev/fake")
		conn := &fakeSerialPort{}
		s.conn = conn

		if err := s.Send(context.Background(), []byte(`{"id":1}`)); err != nil {
			t.Fatalf("Send error: %v", err)
		}

		conn.mu.Lock()
		defer conn.mu.Unlock()
		if len(conn.writes) != 1 {
			t.Fatalf("expected 1 write, got %d", len(conn.writes))
		}
		if got := string(conn.writes[0]); got != `{"id":1}`+"\n" {
			t.Fatalf("unexpected wire payload %q", got)
		}
	})

	t.Run("not connected", func(t *testing.T) {
		s := NewSerial("/dev/fake")
		if err := s.Send(context.Background(), []byte(`{"id":1}`)); !errors.Is(err, ErrNotConnected) {
			t.Fatalf("expected ErrNotConnected, got %v", err)
		}
	})
}

func TestSerialReceiveErrorAndClosed(t *testing.T) {
	t.Run("read error", func(t *testing.T) {
		s := NewSerial("/dev/fake")
		expected := errors.New("boom")
		s.errCh <- expected

		got, err := s.Receive(context.Background())
		if got != nil {
			t.Fatalf("expected nil payload, got %q", string(got))
		}
		if !errors.Is(err, expected) {
			t.Fatalf("expected %v, got %v", expected, err)
		}
	})

	t.Run("closed", func(t *testing.T) {
		s := NewSerial("/dev/fake")
		if err := s.Close(); err != nil {
			t.Fatalf("Close error: %v", err)
		}
		if _, err := s.Receive(context.Background()); !errors.Is(err, ErrClosed) {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	})
}

func TestSerialReader_ReconnectFailurePropagatesError(t *testing.T) {
	origOpen := serialOpenFunc
	origSleep := serialSleep
	defer func() {
		serialOpenFunc = origOpen
		serialSleep = origSleep
	}()

	s := NewSerial("/dev/fake")
	s.conn = &fakeSerialPort{}
	s.scanner = bufio.NewScanner(s.conn)
	serialOpenFunc = func(_ string, _ *serial.Mode) (serial.Port, error) {
		return nil, errors.New("port down")
	}
	serialSleep = func(_ time.Duration) {
		_ = s.Close()
	}

	done := make(chan struct{})
	go func() {
		s.reader()
		close(done)
	}()

	select {
	case err := <-s.errCh:
		if err == nil || !strings.Contains(err.Error(), "reconnect failed") {
			t.Fatalf("expected reconnect failure, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reconnect failure")
	}
	<-done
}

func TestBuildAuthRequest(t *testing.T) {
	req, err := buildAuthRequest("abc123")
	if err != nil {
		t.Fatalf("buildAuthRequest: %v", err)
	}
	got := string(req)
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
func (p *pipeSerialPort) Read(b []byte) (int, error)   { return p.pr.Read(b) }
func (p *pipeSerialPort) Drain() error                 { return nil }
func (p *pipeSerialPort) ResetInputBuffer() error      { return nil }
func (p *pipeSerialPort) ResetOutputBuffer() error     { return nil }
func (p *pipeSerialPort) SetDTR(_ bool) error          { return nil }
func (p *pipeSerialPort) SetRTS(_ bool) error          { return nil }
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

func TestSerialConnect_OpenFailure(t *testing.T) {
	origOpen := serialOpenFunc
	defer func() { serialOpenFunc = origOpen }()

	serialOpenFunc = func(_ string, _ *serial.Mode) (serial.Port, error) {
		return nil, errors.New("open failure")
	}

	s := NewSerial("/dev/missing")
	if err := s.Connect(context.Background()); err == nil || !strings.Contains(err.Error(), "open /dev/missing") {
		t.Fatalf("expected open error, got %v", err)
	}
}

func TestSerialConnect_WithAuthAndReader(t *testing.T) {
	origOpen := serialOpenFunc
	defer func() { serialOpenFunc = origOpen }()

	port := newPipeSerialPort()
	serialOpenFunc = func(_ string, _ *serial.Mode) (serial.Port, error) {
		return port, nil
	}

	s := NewSerial("/dev/fake", WithAuthToken("sekret"))
	if s.authToken != "sekret" {
		t.Fatalf("AuthToken: got %q, want sekret", s.authToken)
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
	defer func() { _ = s.Close() }()

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

func TestSerialConnect_AuthFailure(t *testing.T) {
	origOpen := serialOpenFunc
	defer func() { serialOpenFunc = origOpen }()

	port := newPipeSerialPort()
	serialOpenFunc = func(_ string, _ *serial.Mode) (serial.Port, error) {
		return port, nil
	}

	go func() {
		port.feed(`{"jsonrpc":"2.0","id":0,"error":{"code":-32001,"message":"unauthorized"}}` + "\n")
	}()

	s := NewSerial("/dev/fake", WithAuthToken("bad-token"))
	err := s.Connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "auth validate") {
		t.Fatalf("expected auth validation error, got %v", err)
	}
	if s.conn != nil {
		t.Fatal("connection should be cleared after auth failure")
	}
}
