package transport

import (
	"context"
	"errors"
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
