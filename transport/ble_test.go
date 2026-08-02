package transport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"
)

func TestBLEOnNotificationAssemblesLines(t *testing.T) {
	b := NewBLE("")
	b.onNotification([]byte("{\"a\":1"))
	b.onNotification([]byte("}\nnoise\n{\"b\":2}\n"))

	ctx := context.Background()
	msg1, err := b.Receive(ctx)
	if err != nil {
		t.Fatalf("receive1: %v", err)
	}
	msg2, err := b.Receive(ctx)
	if err != nil {
		t.Fatalf("receive2: %v", err)
	}
	if string(msg1) != `{"a":1}` {
		t.Fatalf("unexpected first msg: %q", msg1)
	}
	if string(msg2) != "noise" {
		t.Fatalf("unexpected second msg: %q", msg2)
	}
}

func TestBLESendNotConnected(t *testing.T) {
	b := NewBLE("")
	if err := b.Send(context.Background(), []byte("{}")); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestBLESendReconnecting(t *testing.T) {
	b := NewBLE("")
	b.reconnecting = true
	if err := b.Send(context.Background(), []byte("{}")); !errors.Is(err, ErrReconnecting) {
		t.Fatalf("expected ErrReconnecting, got %v", err)
	}
}

func TestBLEReceiveClosed(t *testing.T) {
	b := NewBLE("")
	close(b.closeCh)
	if _, err := b.Receive(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestBLEConnectAlreadyConnected(t *testing.T) {
	b := NewBLE("")
	b.connected = true
	if err := b.Connect(context.Background()); err == nil {
		t.Fatal("expected already connected error")
	}
}

func TestBLEDisconnectReconnectCallbacks(t *testing.T) {
	b := NewBLE("")
	b.connected = true

	disconnectCalled := 0
	reconnectCalled := 0
	b.OnDisconnect = func() { disconnectCalled++ }
	b.OnReconnect = func() { reconnectCalled++ }

	b.handleConnectionStateChange(false)
	if disconnectCalled != 1 {
		t.Fatalf("expected disconnect callback once, got %d", disconnectCalled)
	}

	b.reconnecting = false
	b.handleConnectionStateChange(true)
	if reconnectCalled != 1 {
		t.Fatalf("expected reconnect callback once, got %d", reconnectCalled)
	}
}

func TestBLEHandleDisconnectClosesTransport(t *testing.T) {
	b := NewBLE("")
	b.connected = true

	b.handleConnectionStateChange(false)

	if b.connected {
		t.Fatal("expected transport disconnected")
	}
	if _, err := b.Receive(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after disconnect, got %v", err)
	}
}

func TestBLEInfo(t *testing.T) {
	if got := NewBLE("").Info(); got != "ble:auto-scan" {
		t.Fatalf("unexpected info: %q", got)
	}
	if got := NewBLE("Bramble").Info(); got != "ble:Bramble" {
		t.Fatalf("unexpected info with name: %q", got)
	}
}

func TestBLETransport_SetAuthToken(t *testing.T) {
	b := NewBLE("")

	if b.authToken != "" {
		t.Fatalf("expected empty token by default, got %q", b.authToken)
	}

	b.SetAuthToken("token-1")
	if b.authToken != "token-1" {
		t.Fatalf("expected token-1, got %q", b.authToken)
	}

	b.SetAuthToken("token-2")
	if b.authToken != "token-2" {
		t.Fatalf("expected overwritten token-2, got %q", b.authToken)
	}

	b.SetAuthToken("")
	if b.authToken != "" {
		t.Fatalf("expected empty token, got %q", b.authToken)
	}
}

func TestBLETransport_AuthToken(t *testing.T) {
	b := NewBLE("")

	if got := b.AuthToken(); got != "" {
		t.Fatalf("expected empty token by default, got %q", got)
	}

	b.SetAuthToken("ble-token")
	if got := b.AuthToken(); got != "ble-token" {
		t.Fatalf("expected ble-token, got %q", got)
	}
}

func TestBLENewBLE_WithAuthToken(t *testing.T) {
	b := NewBLE("Bramble", WithAuthToken("secret"))
	if b.authToken != "secret" {
		t.Fatalf("expected auth token 'secret', got %q", b.authToken)
	}
	if b.deviceName != "Bramble" {
		t.Fatalf("expected device name 'Bramble', got %q", b.deviceName)
	}
}

func TestBLENewBLE_WithScanTimeout(t *testing.T) {
	b := NewBLE("", WithBLEScanTimeout(30*time.Second))
	if b.scanTimeout != 30*time.Second {
		t.Fatalf("expected 30s scan timeout, got %v", b.scanTimeout)
	}
}

func TestBLENewBLE_DefaultScanTimeout(t *testing.T) {
	b := NewBLE("")
	if b.scanTimeout != 10*time.Second {
		t.Fatalf("expected default 10s scan timeout, got %v", b.scanTimeout)
	}
}

func TestBLENewBLE_MultipleOptions(t *testing.T) {
	b := NewBLE("MyDevice", WithAuthToken("tok"), WithBLEScanTimeout(5*time.Second))
	if b.authToken != "tok" {
		t.Fatalf("expected auth token 'tok', got %q", b.authToken)
	}
	if b.deviceName != "MyDevice" {
		t.Fatalf("expected device name 'MyDevice', got %q", b.deviceName)
	}
	if b.scanTimeout != 5*time.Second {
		t.Fatalf("expected 5s scan timeout, got %v", b.scanTimeout)
	}
}

// TestNUSUUIDs pins the string form of each NUS UUID against the values the
// firmware advertises (components/ble/ble_server.c). bluetooth.NewUUID takes
// its [16]byte argument in big-endian (string) order; a byte-reversed
// literal compiles fine but produces a UUID that never matches anything
// advertised over the air. This test would have caught that.
func TestNUSUUIDs(t *testing.T) {
	cases := []struct {
		name string
		got  bluetooth.UUID
		want string
	}{
		{"service", nusServiceUUID, "6e400001-b5a3-f393-e0a9-e50e24dcca9e"},
		{"tx", nusTXUUID, "6e400002-b5a3-f393-e0a9-e50e24dcca9e"},
		{"rx", nusRXUUID, "6e400003-b5a3-f393-e0a9-e50e24dcca9e"},
	}
	for _, c := range cases {
		if got := c.got.String(); got != c.want {
			t.Errorf("%s UUID = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestValidateBLEAuthAck(t *testing.T) {
	if err := validateBLEAuthAck([]byte(`{"jsonrpc":"2.0","result":{"ok":true},"id":null}`)); err != nil {
		t.Fatalf("expected valid ack, got %v", err)
	}
	if err := validateBLEAuthAck([]byte(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"unauthorized: first BLE write must be auth token"},"id":null}`)); err == nil {
		t.Fatal("expected error ack to fail")
	}
	if err := validateBLEAuthAck([]byte(`{"jsonrpc":"2.0","result":{"ok":false},"id":null}`)); err == nil {
		t.Fatal("expected ok:false to fail")
	}
	if err := validateBLEAuthAck([]byte(`{"jsonrpc":"2.0","id":null}`)); err == nil {
		t.Fatal("expected missing result to fail")
	}
	if err := validateBLEAuthAck([]byte(`not json`)); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestIsTransientBLEWriteError(t *testing.T) {
	if isTransientBLEWriteError(nil) {
		t.Fatal("nil error must not be transient")
	}
	if !isTransientBLEWriteError(errors.New("org.bluez.Error.InProgress: In Progress")) {
		t.Fatal("expected bluez In Progress error to be treated as transient")
	}
	if !isTransientBLEWriteError(errors.New("operation already in progress")) {
		t.Fatal("expected lowercase 'in progress' to be treated as transient")
	}
	if isTransientBLEWriteError(ErrNotConnected) {
		t.Fatal("ErrNotConnected must not be treated as transient")
	}
}

func TestRetrySend_SucceedsFirstTry(t *testing.T) {
	calls := 0
	sleeps := 0
	err := retrySend(func() error {
		calls++
		return nil
	}, func(time.Duration) { sleeps++ }, 5, time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	if sleeps != 0 {
		t.Fatalf("expected no sleeps, got %d", sleeps)
	}
}

func TestRetrySend_RetriesTransientThenSucceeds(t *testing.T) {
	calls := 0
	sleeps := 0
	err := retrySend(func() error {
		calls++
		if calls < 3 {
			return errors.New("org.bluez.Error.InProgress: In Progress")
		}
		return nil
	}, func(time.Duration) { sleeps++ }, 5, time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	if sleeps != 2 {
		t.Fatalf("expected 2 sleeps between 3 calls, got %d", sleeps)
	}
}

func TestRetrySend_StopsImmediatelyOnNonTransientError(t *testing.T) {
	calls := 0
	sleeps := 0
	wantErr := errors.New("permanent failure")
	err := retrySend(func() error {
		calls++
		return wantErr
	}, func(time.Duration) { sleeps++ }, 5, time.Millisecond)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	if sleeps != 0 {
		t.Fatalf("expected no sleeps, got %d", sleeps)
	}
}

func TestRetrySend_GivesUpAfterMaxAttempts(t *testing.T) {
	calls := 0
	sleeps := 0
	err := retrySend(func() error {
		calls++
		return errors.New("in progress")
	}, func(time.Duration) { sleeps++ }, 5, time.Millisecond)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != 5 {
		t.Fatalf("expected 5 calls, got %d", calls)
	}
	if sleeps != 4 {
		t.Fatalf("expected 4 sleeps between 5 calls, got %d", sleeps)
	}
}

func TestValidateAuthResponse(t *testing.T) {
	if err := validateAuthResponse([]byte(`{"jsonrpc":"2.0","id":0,"result":{"ok":true}}`)); err != nil {
		t.Fatalf("expected valid auth response, got %v", err)
	}
	if err := validateAuthResponse([]byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-32001,"message":"unauthorized"}}`)); err == nil {
		t.Fatal("expected auth error response to fail")
	}
}

// TestBLEAuthenticate_NoToken verifies that authenticate is a no-op when no
// auth token is configured — it must return nil without touching BLE hardware.
func TestBLEAuthenticate_NoToken(t *testing.T) {
	b := NewBLE("")
	if err := b.authenticate(context.Background()); err != nil {
		t.Fatalf("expected nil with no auth token, got %v", err)
	}
}

// TestBLEAuthenticate_SendFails verifies that authenticate wraps a Send error
// as an "auth write" error. The transport is not connected, so Send returns
// ErrNotConnected before any BLE hardware is accessed.
func TestBLEAuthenticate_SendFails(t *testing.T) {
	b := NewBLE("", WithAuthToken("secret-token"))
	// b.connected is false — Send() returns ErrNotConnected
	err := b.authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "auth write") {
		t.Fatalf("expected 'auth write' in error, got: %v", err)
	}
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected wrapped in error, got: %v", err)
	}
}

// TestBLEConnect_AdapterEnableFails verifies that Connect() propagates adapter
// Enable() errors wrapped as "enable adapter". A non-existent adapter ID is
// injected so Enable() fails deterministically without real BLE hardware.
func TestBLEConnect_AdapterEnableFails(t *testing.T) {
	b := NewBLE("")
	b.adapter = bluetooth.NewAdapter("nonexistent-ble-adapter-test-99")
	err := b.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error from Connect() with non-existent adapter")
	}
	if !strings.Contains(err.Error(), "enable adapter") {
		t.Fatalf("expected 'enable adapter' in error, got: %v", err)
	}
}

// TestBLEReceive_ContextCancel verifies that Receive returns context.Canceled
// when the context is already cancelled on entry.
func TestBLEReceive_ContextCancel(t *testing.T) {
	b := NewBLE("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Receive(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestBLEOnNotification_DropsWhenFull verifies that onNotification silently
// drops complete lines when recvCh is at capacity (no panic, no block).
func TestBLEOnNotification_DropsWhenFull(t *testing.T) {
	b := NewBLE("")
	// Fill the channel to capacity.
	for i := 0; i < cap(b.recvCh); i++ {
		b.recvCh <- []byte("msg")
	}
	// Sending another complete line should not block or panic.
	b.onNotification([]byte("overflow\n"))
	// Channel must remain exactly at capacity (message was dropped).
	if got := len(b.recvCh); got != cap(b.recvCh) {
		t.Fatalf("expected channel len %d == cap %d after drop, got %d", cap(b.recvCh), cap(b.recvCh), got)
	}
}
