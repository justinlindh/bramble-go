package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var bleSleep = time.Sleep

// bleAuthWriteRetries and bleAuthWriteRetryDelay bound the retry of the auth
// token write against a transient bluez "In Progress" error. bluez returns
// this immediately after EnableNotifications, while the ATT link is still
// settling, on the very first write of a fresh connection.
const (
	bleAuthWriteRetries    = 5
	bleAuthWriteRetryDelay = 250 * time.Millisecond
)

// bleAuthAckTimeout bounds how long Connect waits for the firmware's auth
// ack after writing the token.
const bleAuthAckTimeout = 5 * time.Second

// bleDiscoveryRetryDelay is how long Connect waits before retrying filtered
// service discovery once. A freshly connected bluez adapter can race its own
// GATT cache population against a filtered DiscoverServices call, returning
// an empty result even though the service is present.
const bleDiscoveryRetryDelay = time.Second

// bleChunkSize is the write size Send splits payloads into, sized for a
// common ATT payload limit (default MTU 247 leaves ~240 bytes after protocol
// overhead).
const bleChunkSize = 240

// bleWait blocks for delay, or until ctx is cancelled, whichever comes first,
// returning ctx.Err() in the latter case. Connect's retry delays go through
// this rather than a bare sleep so that cancelling the context actually
// interrupts a connect in progress instead of being noticed only once every
// delay has run to completion.
func bleWait(ctx context.Context, delay time.Duration) error {
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// NUS (Nordic UART Service) UUIDs, must match firmware ble_server.c.
//
// bluetooth.NewUUID takes its [16]byte argument in big-endian (string)
// order: byte 0 is the first two hex digits of the canonical dashed form,
// byte 15 is the last two. Do not byte-reverse these.
var (
	nusServiceUUID = bluetooth.NewUUID([16]byte{ // 6e400001-b5a3-f393-e0a9-e50e24dcca9e
		0x6e, 0x40, 0x00, 0x01, 0xb5, 0xa3, 0xf3, 0x93,
		0xe0, 0xa9, 0xe5, 0x0e, 0x24, 0xdc, 0xca, 0x9e,
	})
	nusTXUUID = bluetooth.NewUUID([16]byte{ // 6e400002-..., write (host to device)
		0x6e, 0x40, 0x00, 0x02, 0xb5, 0xa3, 0xf3, 0x93,
		0xe0, 0xa9, 0xe5, 0x0e, 0x24, 0xdc, 0xca, 0x9e,
	})
	nusRXUUID = bluetooth.NewUUID([16]byte{ // 6e400003-..., notify (device to host)
		0x6e, 0x40, 0x00, 0x03, 0xb5, 0xa3, 0xf3, 0x93,
		0xe0, 0xa9, 0xe5, 0x0e, 0x24, 0xdc, 0xca, 0x9e,
	})
)

// BLE implements Transport over BLE using the Nordic UART Service.
//
// Prerequisite: pairing. The firmware declares the NUS TX/RX characteristics
// with BLE_GATT_CHR_F_*_ENC (components/ble/ble_server.c), so bluez requires
// an encrypted link before it will let a write or notification through. The
// host must already be paired and bonded with the target device before
// calling Connect; this package does not attempt to pair. Pair once with:
//
//	bluetoothctl
//	  agent NoInputNoOutput
//	  default-agent
//	  scan on
//	  pair <MAC>
//
// tinygo's bluez pairing support is not reliable enough to drive
// automatically from here, so this is a documented manual prerequisite, not
// something Connect works around.
//
// Symptom to watch for: if the bond is later destroyed on either side (a
// firmware NVS erase, a `bluetoothctl remove`, a factory reset), writes on
// the TX characteristic fail SILENTLY: WriteWithoutResponse returns no
// error and the device never responds, because it is a write without
// response over a link bluez has silently declined to use unencrypted. Seeing
// Connect succeed (scan and GATT discovery do not require encryption) but
// every Send appear to vanish with no error and no reply is the signature of
// a stale or missing bond; re-pair to fix it.
type BLE struct {
	// deviceName to scan for (e.g. "Bramble"). Empty = connect to first NUS device.
	deviceName string
	// scanTimeout is how long to scan before giving up. Default: 10s.
	scanTimeout  time.Duration
	authToken    string
	adapter      *bluetooth.Adapter
	device       bluetooth.Device
	txChar       bluetooth.DeviceCharacteristic
	connected    bool
	mu           sync.Mutex
	recvCh       chan []byte
	lineBuf      strings.Builder
	closeCh      chan struct{}
	reconnecting bool
	closing      bool

	// OnDisconnect is called (if non-nil) when the BLE link is lost unexpectedly.
	OnDisconnect func()

	// OnReconnect is called (if non-nil) after a successful automatic reconnect.
	OnReconnect func()
}

// NewBLE creates a new BLE transport. The deviceName parameter specifies the
// BLE device name to scan for (e.g. "Bramble"). Pass an empty string to
// connect to the first device advertising the Nordic UART Service.
// Use functional options (e.g. WithAuthToken) to configure the transport.
func NewBLE(deviceName string, opts ...Option) *BLE {
	b := &BLE{
		deviceName:  deviceName,
		scanTimeout: 10 * time.Second,
		adapter:     bluetooth.DefaultAdapter,
		recvCh:      make(chan []byte, 32),
		closeCh:     make(chan struct{}),
	}
	for _, o := range opts {
		o.applyBLE(b)
	}
	return b
}

// Connect scans for a Bramble device advertising NUS and connects.
func (b *BLE) Connect(ctx context.Context) error {
	b.mu.Lock()
	if b.connected {
		b.mu.Unlock()
		return errors.New("bramble/transport/ble: already connected")
	}
	b.closing = false
	b.mu.Unlock()

	return b.connect(ctx)
}

func (b *BLE) connect(ctx context.Context) error {
	// Enable the BLE adapter.
	if err := b.adapter.Enable(); err != nil {
		return fmt.Errorf("bramble/transport/ble: enable adapter: %w", err)
	}

	var foundAddr bluetooth.Address
	var foundName string
	scanDone := make(chan struct{})

	scanCtx, scanCancel := context.WithTimeout(ctx, b.scanTimeout)
	defer scanCancel()

	go func() {
		_ = b.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			name := result.LocalName()

			if b.deviceName != "" {
				if !strings.Contains(strings.ToLower(name), strings.ToLower(b.deviceName)) {
					return
				}
			} else {
				hasNUS := false
				for _, uuid := range result.ServiceUUIDs() {
					if uuid == nusServiceUUID {
						hasNUS = true
						break
					}
				}
				if !hasNUS && name == "" {
					return
				}
			}

			foundAddr = result.Address
			foundName = name
			_ = adapter.StopScan()
			select {
			case <-scanDone:
			default:
				close(scanDone)
			}
		})
	}()

	select {
	case <-scanDone:
	case <-scanCtx.Done():
		_ = b.adapter.StopScan()
		return fmt.Errorf("bramble/transport/ble: scan timeout (no device found in %v)", b.scanTimeout)
	}

	device, err := b.adapter.Connect(foundAddr, bluetooth.ConnectionParams{})
	if err != nil {
		return fmt.Errorf("bramble/transport/ble: connect to %s (%s): %w", foundName, foundAddr.String(), err)
	}

	// Install connection state handler for disconnect/reconnect hooks.
	b.adapter.SetConnectHandler(func(d bluetooth.Device, connected bool) {
		if d.Address != foundAddr {
			return
		}
		b.handleConnectionStateChange(connected)
	})

	svcs, err := device.DiscoverServices([]bluetooth.UUID{nusServiceUUID})
	if err != nil || len(svcs) == 0 {
		// A fresh bluez GATT cache can race a filtered DiscoverServices call
		// immediately after connect, returning an empty result even though
		// the service is present. Retry once before giving up.
		if werr := bleWait(ctx, bleDiscoveryRetryDelay); werr != nil {
			_ = device.Disconnect()
			return werr
		}
		svcs, err = device.DiscoverServices([]bluetooth.UUID{nusServiceUUID})
	}
	if err != nil || len(svcs) == 0 {
		_ = device.Disconnect()
		return fmt.Errorf("bramble/transport/ble: NUS service not found on %s", foundName)
	}

	chars, err := svcs[0].DiscoverCharacteristics([]bluetooth.UUID{nusTXUUID, nusRXUUID})
	if err != nil || len(chars) < 2 {
		_ = device.Disconnect()
		return fmt.Errorf("bramble/transport/ble: NUS characteristics not found (got %d)", len(chars))
	}

	var tx bluetooth.DeviceCharacteristic
	for _, c := range chars {
		switch c.UUID() {
		case nusTXUUID:
			tx = c
		case nusRXUUID:
			if err := c.EnableNotifications(b.onNotification); err != nil {
				_ = device.Disconnect()
				return fmt.Errorf("bramble/transport/ble: enable RX notifications: %w", err)
			}
		}
	}

	b.mu.Lock()
	b.device = device
	b.txChar = tx
	b.connected = true
	if b.isCloseChClosed() {
		b.closeCh = make(chan struct{})
	}
	b.mu.Unlock()

	if err := b.authenticate(ctx); err != nil {
		_ = device.Disconnect()
		b.handleConnectionStateChange(false)
		return err
	}

	return nil
}

// authenticate performs the BLE auth handshake when an auth token is
// configured.
//
// Unlike Serial, which negotiates auth with a "bramble.auth" JSON-RPC call,
// the firmware's BLE transport (components/ble/ble_server.c) treats the very
// first NUS write on a connection as a bare token, with no JSON envelope. A
// successful handshake gets back {"jsonrpc":"2.0","result":{"ok":true},
// "id":null}; a bad token gets a -32603 error with id:null and a 100ms
// throttle before the next attempt is accepted. Send appends the newline
// delimiter, so the token is written as-is here.
func (b *BLE) authenticate(ctx context.Context) error {
	if b.authToken == "" {
		return nil
	}

	token := b.authToken

	attempts := bleAuthAttempts(token)

	err := retrySend(ctx, func() error {
		return b.Send(ctx, []byte(token))
	}, bleWait, attempts, bleAuthWriteRetryDelay)
	if err != nil {
		return fmt.Errorf("bramble/transport/ble: auth write: %w", err)
	}

	authCtx, cancel := context.WithTimeout(ctx, bleAuthAckTimeout)
	defer cancel()

	resp, err := b.Receive(authCtx)
	if err != nil {
		return fmt.Errorf("bramble/transport/ble: auth receive: %w", err)
	}
	if err := validateBLEAuthAck(resp); err != nil {
		return fmt.Errorf("bramble/transport/ble: auth validate: %w", err)
	}
	return nil
}

// bleAuthAttempts reports how many times the auth token write may be tried.
//
// Send issues one write per bleChunkSize chunk, so retrying a payload that
// spans more than one chunk would re-send chunks that already landed and
// corrupt the newline-delimited framing on the device. A token that fits in a
// single chunk (counting the newline Send appends) has no such split to
// half-apply and is safe to repeat; anything larger gets one attempt.
func bleAuthAttempts(token string) int {
	if len(token)+1 > bleChunkSize {
		return 1
	}
	return bleAuthWriteRetries
}

// retrySend calls send, retrying up to attempts times (waiting delay between
// attempts) as long as the returned error looks like a transient bluez write
// failure. It returns immediately on success or on a non-transient error.
//
// send must be safe to repeat: a partially applied send cannot be retried
// without corrupting device-side framing, so callers whose payload spans more
// than one write are responsible for passing attempts=1 (see authenticate).
//
// The wait is cancellable so that a caller abandoning the connect does not
// have to sit through the remaining backoff; ctx.Err() is returned in that
// case, since the cancellation is the reason the retry stopped.
func retrySend(ctx context.Context, send func() error, wait func(context.Context, time.Duration) error, attempts int, delay time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = send()
		if err == nil || !isTransientBLEWriteError(err) {
			return err
		}
		if i < attempts-1 {
			if werr := wait(ctx, delay); werr != nil {
				return werr
			}
		}
	}
	return err
}

// isTransientBLEWriteError reports whether err looks like bluez's
// "org.bluez.Error.InProgress" response, returned when a GATT write lands
// while the ATT link is still settling right after EnableNotifications.
func isTransientBLEWriteError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "in progress")
}

// validateBLEAuthAck checks the firmware's response to a BLE bare-token auth
// write: {"jsonrpc":"2.0","result":{"ok":true},"id":null} on success, or
// {"jsonrpc":"2.0","error":{...},"id":null} on a bad token. Unlike Serial's
// validateAuthResponse, the id here is always null, never an echoed request
// id, so it is not checked.
func validateBLEAuthAck(data []byte) error {
	var ack struct {
		Result *struct {
			OK bool `json:"ok"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &ack); err != nil {
		return fmt.Errorf("invalid auth ack: %w", err)
	}
	if len(ack.Error) > 0 && string(ack.Error) != "null" {
		return fmt.Errorf("auth failed: %s", string(ack.Error))
	}
	if ack.Result == nil || !ack.Result.OK {
		return errors.New("auth ack missing result.ok")
	}
	return nil
}

func (b *BLE) onNotification(data []byte) {
	for _, ch := range data {
		if ch == '\n' {
			line := b.lineBuf.String()
			b.lineBuf.Reset()
			if len(line) > 0 {
				select {
				case b.recvCh <- []byte(line):
				default:
					// Drop if channel full
				}
			}
		} else {
			b.lineBuf.WriteByte(ch)
		}
	}
}

func (b *BLE) isCloseChClosed() bool {
	select {
	case <-b.closeCh:
		return true
	default:
		return false
	}
}

func (b *BLE) handleConnectionStateChange(connected bool) {
	b.mu.Lock()
	wasConnected := b.connected
	if connected {
		b.connected = true
		reconnecting := b.reconnecting
		onReconnect := b.OnReconnect
		b.mu.Unlock()
		if onReconnect != nil && !wasConnected && !reconnecting {
			onReconnect()
		}
		return
	}

	if !wasConnected {
		b.mu.Unlock()
		return
	}

	b.connected = false
	shouldReconnect := !b.closing && !b.reconnecting
	onDisconnect := b.OnDisconnect
	if !b.isCloseChClosed() {
		close(b.closeCh)
	}
	if shouldReconnect {
		b.reconnecting = true
	}
	b.mu.Unlock()

	if onDisconnect != nil {
		onDisconnect()
	}
	if shouldReconnect {
		go b.reconnectLoop()
	}
}

func (b *BLE) reconnectLoop() {
	delay := time.Second
	const maxDelay = 30 * time.Second

	for {
		b.mu.Lock()
		if b.closing {
			b.reconnecting = false
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := b.connect(ctx)
		cancel()
		if err == nil {
			b.mu.Lock()
			b.reconnecting = false
			onReconnect := b.OnReconnect
			b.mu.Unlock()
			if onReconnect != nil {
				onReconnect()
			}
			return
		}

		bleSleep(delay)
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// Send writes a JSON-RPC message to the device, chunked to BLE MTU.
func (b *BLE) Send(_ context.Context, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.reconnecting {
		return ErrReconnecting
	}
	if !b.connected {
		return ErrNotConnected
	}

	// Append newline delimiter
	payload := append(data, '\n')

	// Write in chunks sized for a common ATT payload limit.
	// With default MTU 247, payload is typically 240 bytes after protocol overhead.
	const chunkSize = bleChunkSize
	for i := 0; i < len(payload); i += chunkSize {
		end := i + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		_, err := b.txChar.WriteWithoutResponse(payload[i:end])
		if err != nil {
			return fmt.Errorf("bramble/transport/ble: write: %w", err)
		}
	}
	return nil
}

// Receive blocks until a complete JSON-RPC message arrives from the device.
func (b *BLE) Receive(ctx context.Context) ([]byte, error) {
	select {
	case msg := <-b.recvCh:
		return msg, nil
	case <-b.closeCh:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close disconnects from the BLE device.
func (b *BLE) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closing = true
	b.reconnecting = false
	if !b.connected {
		if !b.isCloseChClosed() {
			close(b.closeCh)
		}
		return nil
	}

	b.connected = false
	if !b.isCloseChClosed() {
		close(b.closeCh)
	}
	return b.device.Disconnect()
}

// SetAuthToken sets the authentication token for the BLE transport.
func (b *BLE) SetAuthToken(token string) {
	b.authToken = token
}

// AuthToken returns the currently configured BLE authentication token.
// Unlike Serial, BLE sends this as a bare token on the first NUS write
// rather than as a JSON-RPC request; see authenticate.
func (b *BLE) AuthToken() string {
	return b.authToken
}

// Info returns a description of the BLE transport.
func (b *BLE) Info() string {
	if b.deviceName != "" {
		return fmt.Sprintf("ble:%s", b.deviceName)
	}
	return "ble:auto-scan"
}
