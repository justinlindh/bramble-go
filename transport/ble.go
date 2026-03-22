package transport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var bleSleep = time.Sleep

// NUS (Nordic UART Service) UUIDs — must match firmware ble_server.c
var (
	nusServiceUUID = bluetooth.NewUUID([16]byte{
		0x9e, 0xca, 0xdc, 0x24, 0x0e, 0xe5, 0xa9, 0xe0,
		0x93, 0xf3, 0xa3, 0xb5, 0x01, 0x00, 0x40, 0x6e,
	})
	nusTXUUID = bluetooth.NewUUID([16]byte{ // write (host → device)
		0x9e, 0xca, 0xdc, 0x24, 0x0e, 0xe5, 0xa9, 0xe0,
		0x93, 0xf3, 0xa3, 0xb5, 0x02, 0x00, 0x40, 0x6e,
	})
	nusRXUUID = bluetooth.NewUUID([16]byte{ // notify (device → host)
		0x9e, 0xca, 0xdc, 0x24, 0x0e, 0xe5, 0xa9, 0xe0,
		0x93, 0xf3, 0xa3, 0xb5, 0x03, 0x00, 0x40, 0x6e,
	})
)

// BLEConfig holds configuration for a BLE transport connection.
type BLEConfig struct {
	AuthConfig
	// DeviceName to scan for (e.g. "Bramble"). Empty = connect to first NUS device.
	DeviceName string
	// ScanTimeout is how long to scan before giving up. Default: 10s.
	ScanTimeout time.Duration
}

// BLE implements Transport over BLE using the Nordic UART Service.
type BLE struct {
	cfg          BLEConfig
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
		cfg: BLEConfig{
			DeviceName:  deviceName,
			ScanTimeout: 10 * time.Second,
		},
		adapter: bluetooth.DefaultAdapter,
		recvCh:  make(chan []byte, 32),
		closeCh: make(chan struct{}),
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

	scanCtx, scanCancel := context.WithTimeout(ctx, b.cfg.ScanTimeout)
	defer scanCancel()

	go func() {
		_ = b.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			name := result.LocalName()

			if b.cfg.DeviceName != "" {
				if !strings.Contains(strings.ToLower(name), strings.ToLower(b.cfg.DeviceName)) {
					return
				}
			} else {
				hasNUS := false
				for _, uuid := range result.AdvertisementPayload.ServiceUUIDs() {
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
		return fmt.Errorf("bramble/transport/ble: scan timeout (no device found in %v)", b.cfg.ScanTimeout)
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
		uuid := c.UUID()
		if uuid == nusTXUUID {
			tx = c
		} else if uuid == nusRXUUID {
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

// authenticate performs an auth handshake when AuthToken is configured.
func (b *BLE) authenticate(ctx context.Context) error {
	if b.cfg.AuthToken == "" {
		return nil
	}

	if err := b.Send(buildAuthRequest(b.cfg.AuthToken)); err != nil {
		return fmt.Errorf("bramble/transport/ble: auth write: %w", err)
	}

	authCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := b.Receive(authCtx)
	if err != nil {
		return fmt.Errorf("bramble/transport/ble: auth receive: %w", err)
	}
	if err := validateAuthResponse(resp); err != nil {
		return fmt.Errorf("bramble/transport/ble: auth validate: %w", err)
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
func (b *BLE) Send(data []byte) error {
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
	const chunkSize = 240
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
// The token is stored in the nested BLEConfig.AuthConfig.
func (b *BLE) SetAuthToken(token string) {
	b.cfg.AuthToken = token
}

// Info returns a description of the BLE transport.
func (b *BLE) Info() string {
	if b.cfg.DeviceName != "" {
		return fmt.Sprintf("ble:%s", b.cfg.DeviceName)
	}
	return "ble:auto-scan"
}
