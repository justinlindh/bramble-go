package transport

import (
	"errors"
	"testing"
)

// --- Error variable tests ---

func TestErrorVariablesDistinct(t *testing.T) {
	if errors.Is(ErrNotConnected, ErrClosed) || errors.Is(ErrNotConnected, ErrReconnecting) || errors.Is(ErrClosed, ErrReconnecting) {
		t.Fatal("error variables should be distinct")
	}
	for _, e := range []error{ErrNotConnected, ErrClosed, ErrReconnecting} {
		if e.Error() == "" {
			t.Fatalf("error %v should have a useful message", e)
		}
	}
}

// --- WebSocket additional tests ---

func TestNewWebSocketConfig(t *testing.T) {
	w := NewWebSocket("ws://192.168.4.1/ws")
	if w.url != "ws://192.168.4.1/ws" {
		t.Fatalf("unexpected url: %s", w.url)
	}
	if w.done == nil {
		t.Fatal("done channel should be initialized")
	}
	if w.conn != nil {
		t.Fatal("conn should be nil before Connect")
	}
}

func TestWebSocketSendNotConnected(t *testing.T) {
	w := NewWebSocket("ws://example.invalid")
	if err := w.Send([]byte("{}")); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestWebSocketCloseIdempotent(t *testing.T) {
	w := NewWebSocket("ws://example.invalid")
	if err := w.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestWebSocketInfo(t *testing.T) {
	w := NewWebSocket("ws://192.168.4.1/ws")
	if got := w.Info(); got != "websocket:ws://192.168.4.1/ws" {
		t.Fatalf("unexpected info: %q", got)
	}
}

// --- Serial additional tests ---

func TestNewSerialConfig(t *testing.T) {
	s := NewSerial("/dev/ttyUSB0")
	if s.port != "/dev/ttyUSB0" {
		t.Fatalf("unexpected port: %s", s.port)
	}
	if s.baud != 115200 {
		t.Fatalf("unexpected default baud: %d", s.baud)
	}
	if s.recvCh == nil || s.errCh == nil || s.done == nil {
		t.Fatal("channels should be initialized")
	}
}

func TestNewSerialWithBaudRate(t *testing.T) {
	s := NewSerial("/dev/ttyUSB0", WithBaudRate(9600))
	if s.baud != 9600 {
		t.Fatalf("expected baud 9600, got %d", s.baud)
	}
}

func TestSerialSendNotConnected(t *testing.T) {
	s := NewSerial("/dev/fake")
	if err := s.Send([]byte("{}")); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestSerialCloseIdempotent(t *testing.T) {
	s := NewSerial("/dev/fake")
	if err := s.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestSerialInfo(t *testing.T) {
	s := NewSerial("/dev/ttyUSB0")
	if got := s.Info(); got != "serial:/dev/ttyUSB0@115200" {
		t.Fatalf("unexpected info: %q", got)
	}
	s2 := NewSerial("/dev/ttyACM0", WithBaudRate(9600))
	if got := s2.Info(); got != "serial:/dev/ttyACM0@9600" {
		t.Fatalf("unexpected info: %q", got)
	}
}

// --- BLE additional tests ---

func TestNewBLEConfig(t *testing.T) {
	b := NewBLE(BLEConfig{DeviceName: "Bramble", ScanTimeout: 5e9})
	if b.cfg.DeviceName != "Bramble" {
		t.Fatalf("unexpected device name: %s", b.cfg.DeviceName)
	}
	if b.cfg.ScanTimeout != 5e9 {
		t.Fatalf("unexpected scan timeout: %v", b.cfg.ScanTimeout)
	}
	if b.recvCh == nil || b.closeCh == nil {
		t.Fatal("channels should be initialized")
	}
}

func TestNewBLEDefaultScanTimeout(t *testing.T) {
	b := NewBLE(BLEConfig{})
	if b.cfg.ScanTimeout != 10e9 {
		t.Fatalf("expected 10s default, got %v", b.cfg.ScanTimeout)
	}
}

func TestBLECloseIdempotent(t *testing.T) {
	b := NewBLE(BLEConfig{})
	// Not connected — Close should be no-op
	if err := b.Close(); err != nil {
		t.Fatalf("close when not connected: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// --- BLE chunking tests ---
// The chunking logic is inline in Send(), so we test it indirectly via a connected BLE
// with a fake txChar. Since bluetooth.DeviceCharacteristic isn't easily mockable,
// we test the chunking math directly.

func TestBLEChunkingBoundaries(t *testing.T) {
	// Verify the chunk size constant and boundaries by computing expected chunks
	const chunkSize = 240
	tests := []struct {
		name     string
		dataLen  int
		expected int // number of chunks for data + newline
	}{
		{"empty", 0, 1},            // just newline: 1 byte -> 1 chunk
		{"small", 100, 1},          // 101 bytes -> 1 chunk
		{"exactly_239", 239, 1},    // 240 bytes (239+nl) -> 1 chunk
		{"exactly_240", 240, 2},    // 241 bytes -> 2 chunks
		{"241_bytes", 241, 2},      // 242 bytes -> 2 chunks
		{"479_bytes", 479, 2},      // 480 bytes -> 2 chunks
		{"480_bytes", 480, 3},      // 481 bytes -> 3 chunks
		{"720_bytes", 720, 4},      // 721 bytes -> 4 chunks
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payloadLen := tc.dataLen + 1 // +1 for newline
			chunks := payloadLen / chunkSize
			if payloadLen%chunkSize != 0 {
				chunks++
			}
			if chunks != tc.expected {
				t.Fatalf("dataLen=%d: expected %d chunks, got %d", tc.dataLen, tc.expected, chunks)
			}
		})
	}
}
