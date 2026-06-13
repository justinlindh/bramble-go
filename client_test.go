package bramble

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/justinlindh/bramble-go/transport/transporttest"
)

// setupRawClient creates a Client with mock transport without running Connect.
func setupRawClient(t *testing.T) (*Client, *transporttest.Mock) {
	t.Helper()
	mock := transporttest.NewMock()
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
	mock := transporttest.NewMock()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"firmware_version":"0.1.0","protocol_version":"0.1.0","hardware":"heltec_v3"}}`)
	c := NewClient(mock)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = c.Close() }()
}

func TestClient_Connect_Incompatible(t *testing.T) {
	mock := transporttest.NewMock()
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
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"address":"1191C6E0","firmware_version":"0.2.0-dev","protocol_version":"0.2.0","hardware":"heltec_v3","radio_ok":true,"peers":2,"beacon_tx":10,"beacon_rx":20,"packets_tx":10,"packets_rx":20,"uptime_s":3600}}`)

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

func TestClient_WifiStatus(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"mode":"station","ssid":"meshnet","ip":"192.0.2.0","rssi":-57,"mac":"AA:BB:CC:DD:EE:FF","clients":0}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := c.WifiStatus(ctx)
	if err != nil {
		t.Fatalf("WifiStatus error: %v", err)
	}
	if status.Mode != "station" {
		t.Errorf("mode: got %q, want station", status.Mode)
	}
	if status.SSID != "meshnet" {
		t.Errorf("ssid: got %q, want meshnet", status.SSID)
	}
	if status.IP != "192.0.2.0" {
		t.Errorf("ip: got %q, want 192.0.2.0", status.IP)
	}
	if status.RSSI != -57 {
		t.Errorf("rssi: got %d, want -57", status.RSSI)
	}
	if status.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("mac: got %q, want AA:BB:CC:DD:EE:FF", status.MAC)
	}
	if status.Clients != 0 {
		t.Errorf("clients: got %d, want 0", status.Clients)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.getWifiStatus"`) {
		t.Fatalf("expected bramble.getWifiStatus request, got: %s", sent[0])
	}
}

func TestClient_Diagnostics(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"uptime_s":1234,"free_heap":45678,"heap":{"internal_free":1000,"internal_min_ever_free":900,"internal_largest_free_block":700,"dma_free":600,"dma_largest_free_block":500,"psram_free":400,"psram_min_ever_free":300},"task_stack_hwm":[{"task":"main","hwm_words":128,"hwm_bytes":512}]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	d, err := c.Diagnostics(ctx, true)
	if err != nil {
		t.Fatalf("Diagnostics error: %v", err)
	}
	if d.UptimeS != 1234 {
		t.Fatalf("uptime_s: got %v, want 1234", d.UptimeS)
	}
	if d.Heap.InternalFree != 1000 {
		t.Fatalf("heap.internal_free: got %v, want 1000", d.Heap.InternalFree)
	}
	if len(d.TaskStackHWM) != 1 || d.TaskStackHWM[0].Task != "main" {
		t.Fatalf("task_stack_hwm decode failed: %+v", d.TaskStackHWM)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.getDiagnostics"`) {
		t.Fatalf("expected bramble.getDiagnostics request, got: %s", sent[0])
	}
	if !strings.Contains(sent[0], `"include_heap_dump":true`) {
		t.Fatalf("expected include_heap_dump=true in request, got: %s", sent[0])
	}
}

func TestClient_Diagnostics_DefaultParamsEmptyObject(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"uptime_s":1,"free_heap":2,"heap":{"internal_free":3,"internal_min_ever_free":4,"internal_largest_free_block":5,"dma_free":6,"dma_largest_free_block":7,"psram_free":8,"psram_min_ever_free":9},"task_stack_hwm":[]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Diagnostics(ctx, false)
	if err != nil {
		t.Fatalf("Diagnostics error: %v", err)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if strings.Contains(sent[0], `"include_heap_dump"`) {
		t.Fatalf("did not expect include_heap_dump in request when false: %s", sent[0])
	}
}

func TestClient_Neighbors(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

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
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"pong":true,"address":"4A555354","protocol_version":"0.2.0"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestClient_Send(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

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

func TestClient_BroadcastOnChannel(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"packet_id":"A1B2C3D4","status":"sent"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := c.BroadcastOnChannel(ctx, 2, "hello ch2")
	if err != nil {
		t.Fatalf("BroadcastOnChannel error: %v", err)
	}
	if result.Status != "sent" {
		t.Errorf("status: got %q, want sent", result.Status)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.sendMessage"`) {
		t.Fatalf("expected bramble.sendMessage request, got: %s", sent[0])
	}
	if !strings.Contains(sent[0], `"dest":"FFFFFFFE"`) || !strings.Contains(sent[0], `"channel":2`) {
		t.Fatalf("expected channel broadcast params in request, got: %s", sent[0])
	}
}

func TestClient_SendCritical(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"packet_id":"A1B2C3D4","status":"sent"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.SendCritical(ctx, 0x12345678, "urgent")
	if err != nil {
		t.Fatalf("SendCritical error: %v", err)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"critical":true`) {
		t.Fatalf("expected critical=true in request, got: %s", sent[0])
	}
}

func TestClient_SendBroadcastCritical(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"broadcast_id":"A1B2C3D4","status":"sent"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.SendBroadcastCritical(ctx, "urgent all")
	if err != nil {
		t.Fatalf("SendBroadcastCritical error: %v", err)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"critical":true`) {
		t.Fatalf("expected critical=true in request, got: %s", sent[0])
	}
}

func TestClient_OnMessage(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

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
	defer func() { _ = c.Close() }()

	received := make(chan Ack, 1)
	c.OnAck(func(a Ack) { received <- a })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onAck","params":{"packet_id":"0000002A","status":"delivered"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case a := <-received:
		if a.PacketID != "0000002A" {
			t.Errorf("packet_id: got %q, want 0000002A", a.PacketID)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for OnAck callback")
	}
}

func TestClient_OnNeighborChange(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

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

func TestClient_OnWifiEvent(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan WifiEvent, 1)
	c.OnWifiEvent(func(e WifiEvent) { received <- e })
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onWifiEvent","params":{"event":"connected","mode":"sta","connected":true,"ip":"192.0.2.0"}}`)

	select {
	case evt := <-received:
		if evt.Event != "connected" || evt.IP != "192.0.2.0" {
			t.Fatalf("unexpected wifi event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnWifiEvent callback")
	}
}

func TestClient_OnGPSEvent(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan GPSEvent, 1)
	c.OnGPSEvent(func(e GPSEvent) { received <- e })
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onGpsEvent","params":{"event":"fix_acquired","valid":true,"lat":37.1,"lon":-122.2}}`)

	select {
	case evt := <-received:
		if evt.Event != "fix_acquired" || !evt.Valid {
			t.Fatalf("unexpected gps event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnGPSEvent callback")
	}
}

func TestClient_OnLocationEvent(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan LocationEvent, 1)
	c.OnLocationEvent(func(e LocationEvent) { received <- e })
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onLocationEvent","params":{"event":"received","peer":"AABBCCDD","tier":1,"timestamp_ms":1730000000}}`)

	select {
	case evt := <-received:
		if evt.Event != "received" || evt.Peer != "AABBCCDD" {
			t.Fatalf("unexpected location event: %+v", evt)
		}
		// Firmware sends tier as integer 1 — SDK should decode to "coarse".
		if evt.Tier != LocationTierCoarse {
			t.Fatalf("expected tier=%q, got %q", LocationTierCoarse, evt.Tier)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnLocationEvent callback")
	}
}

func TestClient_Config(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"node_name":"mynode","address":"1191C6E0","radio":{"frequency_mhz":915,"sf":9,"bw_hz":125000,"tx_power_dbm":17,"profile":"long_range"},"channels":[{"id":0,"name":"public","has_psk":false,"epoch":0,"is_default":true},{"id":1,"name":"team","has_psk":true,"epoch":7,"is_default":false}],"location":{"enabled":true,"default_tier":"coarse","interval_s":300,"source":"gps","contact_rules":[{"address":"AABBCCDD","enabled":true,"tier":"full","interval_s":60}],"channel_targets":[{"channel":0,"enabled":true,"tier":"coarse","interval_s":120}]}}}`)

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
	if len(cfg.Channels) != 2 {
		t.Fatalf("channels len: got %d, want 2", len(cfg.Channels))
	}
	if !cfg.Channels[1].HasPsk {
		t.Fatalf("expected channel[1].HasPsk=true")
	}
	if cfg.Channels[1].Epoch != 7 {
		t.Fatalf("channel[1].Epoch: got %d, want 7", cfg.Channels[1].Epoch)
	}
	if cfg.Location.DefaultTier == nil || *cfg.Location.DefaultTier != "coarse" {
		t.Fatalf("expected location.default_tier=coarse, got %+v", cfg.Location.DefaultTier)
	}
	if len(cfg.Location.ContactRules) != 1 || cfg.Location.ContactRules[0].Address != "AABBCCDD" {
		t.Fatalf("unexpected location.contact_rules decode: %+v", cfg.Location.ContactRules)
	}
}

func TestClient_SetRadio(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sf := 10
	if err := c.SetRadio(ctx, RadioConfig{SF: &sf}); err != nil {
		t.Fatalf("SetRadio error: %v", err)
	}
}

func TestClient_OTAUpdate(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"note":"ota accepted","partition":"app0"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := c.OTAUpdate(ctx, "http://192.0.2.0:8088/bramble.bin")
	if err != nil {
		t.Fatalf("OTAUpdate error: %v", err)
	}
	if !resp.OK {
		t.Fatalf("ok: got false, want true")
	}
	if resp.Note != "ota accepted" {
		t.Fatalf("note: got %q, want ota accepted", resp.Note)
	}
	if resp.Partition != "app0" {
		t.Fatalf("partition: got %q, want app0", resp.Partition)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.otaUpdate"`) {
		t.Fatalf("expected bramble.otaUpdate request, got: %s", sent[0])
	}
	if !strings.Contains(sent[0], `"url":"http://192.0.2.0:8088/bramble.bin"`) {
		t.Fatalf("expected URL param in request, got: %s", sent[0])
	}
}

func TestClient_PeerLocations_CanonicalOnly(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"peerLocations":[{"addr":"AABBCCDD","name":"peer1","tier":"coarse","position":null,"online":true,"last_updated_ms":1234}],"peers":[{"addr":"DEADBEEF","name":"legacy","tier":"coarse","position":null,"online":false,"last_updated_ms":5678}]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	peers, err := c.PeerLocations(ctx)
	if err != nil {
		t.Fatalf("PeerLocations error: %v", err)
	}
	if len(peers) != 1 || peers[0].Addr != "AABBCCDD" {
		t.Fatalf("expected canonical peerLocations only, got %+v", peers)
	}
}

func TestClient_SetLocationConfig_UsesCanonicalDefaultTierField(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)

	enabled := true
	defaultTier := "full"
	intervalS := 120
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.SetLocationConfig(ctx, LocationConfig{Enabled: &enabled, DefaultTier: &defaultTier, IntervalS: &intervalS})
	if err != nil {
		t.Fatalf("SetLocationConfig error: %v", err)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"default_tier":"full"`) || !strings.Contains(sent[0], `"interval_s":120`) {
		t.Fatalf("expected canonical location fields in request, got: %s", sent[0])
	}
	if strings.Contains(sent[0], `"tier":"full"`) {
		t.Fatalf("did not expect tier compatibility alias in request: %s", sent[0])
	}
}

func TestClient_RPCError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.Status(ctx); err == nil {
		t.Fatal("expected error for RPC error response")
	}
}

func TestClient_AuthToken(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"token":"pairing-token"}}`)
	token, err := c.AuthToken(ctx)
	if err != nil {
		t.Fatalf("AuthToken error: %v", err)
	}
	if token != "pairing-token" {
		t.Fatalf("token: got %q, want pairing-token", token)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.getAuthToken"`) {
		t.Fatalf("expected bramble.getAuthToken request, got: %s", sent[0])
	}
}

func TestClient_OnDecodeError_MalformedNotification(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	type decodeErr struct {
		method  string
		err     error
		payload []byte
	}
	errCh := make(chan decodeErr, 10)
	c.OnDecodeError(func(method string, err error, payload []byte) {
		errCh <- decodeErr{method: method, err: err, payload: payload}
	})

	// Also register an OnMessage callback to confirm the connection stays alive
	// after the malformed notification is processed.
	goodMsg := make(chan Message, 1)
	c.OnMessage(func(m Message) { goodMsg <- m })

	// Inject a malformed bramble.onMessage notification (params is not an object).
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onMessage","params":"not-valid-json-for-message"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case de := <-errCh:
		if de.method != "bramble.onMessage" {
			t.Errorf("method: got %q, want bramble.onMessage", de.method)
		}
		if de.err == nil {
			t.Error("expected a non-nil decode error")
		}
		if len(de.payload) == 0 {
			t.Error("expected non-empty payload in decode error callback")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for OnDecodeError callback")
	}

	// Connection must still be alive: a valid notification must arrive and fire its callback.
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onMessage","params":{"from":"AABBCCDD","to":"EEFF0011","text":"still alive","timestamp":1}}`)
	select {
	case m := <-goodMsg:
		if m.Text != "still alive" {
			t.Errorf("text: got %q, want still alive", m.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for valid OnMessage callback after decode error")
	}
}

func TestClient_OnDecodeError_PayloadTruncated(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	errCh := make(chan []byte, 1)
	c.OnDecodeError(func(_ string, _ error, payload []byte) {
		errCh <- payload
	})
	c.OnMessage(func(Message) {}) // register callback so decode is attempted

	// Build a payload larger than the 512-byte limit.
	longJunk := make([]byte, 600)
	for i := range longJunk {
		longJunk[i] = 'x'
	}
	// Wrap it as a JSON string (valid JSON, but wrong type for Message params).
	bigPayload := `"` + string(longJunk) + `"`
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onMessage","params":` + bigPayload + `}`)

	select {
	case payload := <-errCh:
		if len(payload) > 512 {
			t.Errorf("payload snippet should be ≤512 bytes, got %d", len(payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnDecodeError callback")
	}
}
