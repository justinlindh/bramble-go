package bramble

import (
	"context"
	"testing"
	"time"

	"github.com/justinlindh/bramble-go/transport"
)

// setupRawClient creates a Client with mock transport without running Connect.
func setupRawClient(t *testing.T) (*Client, *transport.MockTransport) {
	t.Helper()
	mock := transport.NewMock()
	ctx := context.Background()
	if err := mock.Connect(ctx); err != nil {
		t.Fatalf("mock.Connect: %v", err)
	}
	c := &Client{t: mock}
	c.proto = NewProtocol(mock)
	c.proto.Start()
	go c.notifyLoop()
	return c, mock
}

func TestClient_Connect(t *testing.T) {
	mock := transport.NewMock()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"firmware_version":"0.1.0","protocol_version":"0.1.0","hardware":"heltec_v3"}}`)
	c := NewClient(mock)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()
}

func TestClient_Connect_Incompatible(t *testing.T) {
	mock := transport.NewMock()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"firmware_version":"99.0.0","protocol_version":"99.0.0","hardware":"esp32"}}`)
	c := NewClient(mock)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err == nil {
		t.Fatal("expected error for incompatible protocol version")
	}
}

func TestClient_Status(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"address":"1191C6E0","firmware_version":"0.1.0-dev","protocol_version":"0.1.0","hardware":"heltec_v3","radio_ok":true,"peers":2,"beacon_tx":10,"beacon_rx":20,"packets_tx":10,"packets_rx":20,"uptime_s":3600}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if status.UptimeSec != 3600 {
		t.Errorf("uptime_s: got %d, want 3600", status.UptimeSec)
	}
	if status.Peers != 2 {
		t.Errorf("peers: got %d, want 2", status.Peers)
	}
	if status.Address != "1191C6E0" {
		t.Errorf("address: got %q, want 1191C6E0", status.Address)
	}
}

func TestClient_Neighbors(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"neighbors":[{"address":"12345678","rssi":-75,"snr":8.5,"last_seen_ms":1000},{"address":"DEADBEEF","rssi":-90,"snr":4.2,"last_seen_ms":2000}]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	neighbors, err := c.Neighbors(ctx)
	if err != nil {
		t.Fatalf("Neighbors error: %v", err)
	}
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors, got %d", len(neighbors))
	}
	if neighbors[0].Address != "12345678" {
		t.Errorf("address: got %q, want 12345678", neighbors[0].Address)
	}
	if neighbors[0].RSSI != -75 {
		t.Errorf("rssi: got %d, want -75", neighbors[0].RSSI)
	}
}

func TestClient_Ping(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"pong":true,"address":"4A555354","protocol_version":"0.1.0"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestClient_Send(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"message_id":"TODO","status":"sent"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := c.Send(ctx, 0x12345678, "hello mesh")
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if result.Status != "sent" {
		t.Errorf("status: got %q, want sent", result.Status)
	}
}

func TestClient_Broadcast(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"message_id":"TODO","status":"sent"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := c.Broadcast(ctx, "hello everyone")
	if err != nil {
		t.Fatalf("Broadcast error: %v", err)
	}
	if result.Status != "sent" {
		t.Errorf("status: got %q, want sent", result.Status)
	}
}

func TestClient_OnMessage(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	received := make(chan Message, 1)
	c.OnMessage(func(m Message) { received <- m })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onMessage","params":{"from":"00000001","to":"00000002","text":"hey","timestamp":9999}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case m := <-received:
		if m.Text != "hey" {
			t.Errorf("text: got %q, want hey", m.Text)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for OnMessage callback")
	}
}

func TestClient_OnAck(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	received := make(chan Ack, 1)
	c.OnAck(func(a Ack) { received <- a })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onAck","params":{"packetId":42,"status":"delivered"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case a := <-received:
		if a.PacketID != 42 {
			t.Errorf("packetId: got %d, want 42", a.PacketID)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for OnAck callback")
	}
}

func TestClient_OnNeighborChange(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	fired := make(chan struct{}, 1)
	c.OnNeighborChange(func() { fired <- struct{}{} })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onNeighborChange","params":{}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case <-fired:
	case <-ctx.Done():
		t.Fatal("timed out waiting for OnNeighborChange callback")
	}
}

func TestClient_Config(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"node_name":"mynode","address":"1191C6E0","radio":{"frequency_mhz":915,"sf":9,"bw_hz":125000,"tx_power_dbm":17,"profile":"long_range"},"channels":[{"id":0,"name":"public","is_default":true}]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg, err := c.Config(ctx)
	if err != nil {
		t.Fatalf("Config error: %v", err)
	}
	if cfg.NodeName != "mynode" {
		t.Errorf("node_name: got %q, want mynode", cfg.NodeName)
	}
	if cfg.Radio.FrequencyMhz != 915 {
		t.Errorf("frequency_mhz: got %d, want 915", cfg.Radio.FrequencyMhz)
	}
}

func TestClient_SetRadio(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sf := 10
	if err := c.SetRadio(ctx, RadioConfig{SF: &sf}); err != nil {
		t.Fatalf("SetRadio error: %v", err)
	}
}

func TestClient_RPCError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.Status(ctx); err == nil {
		t.Fatal("expected error for RPC error response")
	}
}
