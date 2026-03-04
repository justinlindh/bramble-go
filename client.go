package bramble

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/justinlindh/bramble-go/transport"
)

// Client is the high-level Bramble mesh node client.
// Create one with NewClient, then call Connect before any other methods.
type Client struct {
	proto                 *Protocol
	t                     transport.Transport
	mu                    sync.Mutex
	onMessageFn           func(Message)
	onAckFn               func(Ack)
	onNeighborFn          func()
	onTrafficEventFn      func(TrafficEvent)
	onBroadcastDeliveryFn func(BroadcastDelivery)
	onWifiEventFn         func(WifiEvent)
	onGpsEventFn          func(GpsEvent)
	onLocationEventFn     func(LocationEvent)
	onProbeResultFn       func(ProbeResult)
	onProbeCompleteFn     func(ProbeComplete)
}

// NewClient creates a new Client using the given transport.
func NewClient(t transport.Transport) *Client {
	return &Client{t: t}
}

// Connect establishes the transport connection, starts the protocol reader,
// and verifies that the node's protocol version is compatible with this SDK.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.t.Connect(ctx); err != nil {
		return fmt.Errorf("bramble: connect: %w", err)
	}
	c.proto = NewProtocol(c.t)
	c.proto.Start()
	go c.notifyLoop()

	ver, err := c.Version(ctx)
	if err != nil {
		return fmt.Errorf("bramble: version check: %w", err)
	}
	if !IsCompatible(ver.ProtocolVersion) {
		return fmt.Errorf("bramble: incompatible protocol version %q (SDK supports %s-%s)",
			ver.ProtocolVersion, MinProtocolVersion, MaxProtocolVersion)
	}
	return nil
}

// Close stops the protocol reader and closes the underlying transport.
func (c *Client) Close() error {
	if c.proto != nil {
		c.proto.Stop()
	}
	return c.t.Close()
}

// notifyLoop distributes incoming notifications to registered callbacks.
func (c *Client) notifyLoop() {
	for n := range c.proto.Notifications() {
		c.mu.Lock()
		onMsg := c.onMessageFn
		onAck := c.onAckFn
		onNeighbor := c.onNeighborFn
		onTraffic := c.onTrafficEventFn
		onBroadcastDelivery := c.onBroadcastDeliveryFn
		onWifi := c.onWifiEventFn
		onGps := c.onGpsEventFn
		onLocation := c.onLocationEventFn
		onProbeResult := c.onProbeResultFn
		onProbeComplete := c.onProbeCompleteFn
		c.mu.Unlock()

		switch n.Method {
		case "bramble.onMessage":
			if onMsg != nil {
				var m Message
				if json.Unmarshal(n.Params, &m) == nil {
					onMsg(m)
				}
			}
		case "bramble.onAck":
			if onAck != nil {
				var a Ack
				if json.Unmarshal(n.Params, &a) == nil {
					onAck(a)
				}
			}
		case "bramble.onNeighborChange":
			if onNeighbor != nil {
				onNeighbor()
			}
		case "bramble.onTrafficEvent":
			if onTraffic != nil {
				var evt TrafficEvent
				if json.Unmarshal(n.Params, &evt) == nil {
					onTraffic(evt)
				}
			}
		case "bramble.onBroadcastDelivery":
			if onBroadcastDelivery != nil {
				var evt BroadcastDelivery
				if json.Unmarshal(n.Params, &evt) == nil {
					onBroadcastDelivery(evt)
				}
			}
		case "bramble.onWifiEvent":
			if onWifi != nil {
				var evt WifiEvent
				if json.Unmarshal(n.Params, &evt) == nil {
					onWifi(evt)
				}
			}
		case "bramble.onGpsEvent":
			if onGps != nil {
				var evt GpsEvent
				if json.Unmarshal(n.Params, &evt) == nil {
					onGps(evt)
				}
			}
		case "bramble.onLocationEvent":
			if onLocation != nil {
				var evt LocationEvent
				if json.Unmarshal(n.Params, &evt) == nil {
					onLocation(evt)
				}
			}
		case "bramble.onProbeResult":
			if onProbeResult != nil {
				var evt ProbeResult
				if json.Unmarshal(n.Params, &evt) == nil {
					onProbeResult(evt)
				}
			}
		case "bramble.onProbeComplete":
			if onProbeComplete != nil {
				var evt ProbeComplete
				if json.Unmarshal(n.Params, &evt) == nil {
					onProbeComplete(evt)
				}
			}
		}
	}
}

// ── Query Methods ─────────────────────────────────────────────────────────────

// Status returns current node status (uptime, counters, position, etc.).
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.getStatus", nil)
	if err != nil {
		return nil, err
	}
	var resp StatusResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode StatusResponse: %w", err)
	}
	return &resp, nil
}

// GetWifiStatus returns current WiFi mode, link, and AP client information.
func (c *Client) GetWifiStatus(ctx context.Context) (*WifiStatus, error) {
	raw, err := c.proto.Call(ctx, "bramble.getWifiStatus", nil)
	if err != nil {
		return nil, fmt.Errorf("bramble: get wifi status: %w", err)
	}
	var resp WifiStatus
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode WifiStatus: %w", err)
	}
	return &resp, nil
}

// GetDiagnostics returns runtime heap diagnostics and task stack high-water marks.
func (c *Client) GetDiagnostics(ctx context.Context, includeHeapDump bool) (*DiagnosticsResponse, error) {
	params := map[string]any{}
	if includeHeapDump {
		params["include_heap_dump"] = true
	}

	raw, err := c.proto.Call(ctx, "bramble.getDiagnostics", params)
	if err != nil {
		return nil, fmt.Errorf("bramble: get diagnostics: %w", err)
	}
	var resp DiagnosticsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode DiagnosticsResponse: %w", err)
	}
	return &resp, nil
}

// Identity returns the node's address and public key hash.
func (c *Client) Identity(ctx context.Context) (*IdentityResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.getIdentity", nil)
	if err != nil {
		return nil, err
	}
	var resp IdentityResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode IdentityResponse: %w", err)
	}
	return &resp, nil
}

// Version returns firmware version, protocol version, and hardware identifier.
func (c *Client) Version(ctx context.Context) (*VersionResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.getVersion", nil)
	if err != nil {
		return nil, err
	}
	var resp VersionResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode VersionResponse: %w", err)
	}
	return &resp, nil
}

// DeliveryEvents returns persisted delivery telemetry events since the given sequence.
func (c *Client) DeliveryEvents(ctx context.Context, sinceEventSeq uint32, limit uint32) (*DeliveryReplayResponse, error) {
	params := map[string]any{"sinceEventSeq": sinceEventSeq}
	if limit > 0 {
		params["limit"] = limit
	}
	raw, err := c.proto.Call(ctx, "bramble.getDeliveryEvents", params)
	if err != nil {
		return nil, err
	}
	var resp DeliveryReplayResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode DeliveryReplayResponse: %w", err)
	}
	return &resp, nil
}

// Neighbors returns the list of direct radio neighbors.
func (c *Client) Neighbors(ctx context.Context) ([]Neighbor, error) {
	raw, err := c.proto.Call(ctx, "bramble.getNeighbors", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Neighbors []Neighbor `json:"neighbors"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode NeighborsResponse: %w", err)
	}
	return resp.Neighbors, nil
}

// Routes returns the current routing table.
func (c *Client) Routes(ctx context.Context) ([]Route, error) {
	raw, err := c.proto.Call(ctx, "bramble.getRoutes", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Routes []Route `json:"routes"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode RoutesResponse: %w", err)
	}
	return resp.Routes, nil
}

// Airtime returns per-tier airtime budget usage.
func (c *Client) Airtime(ctx context.Context) (*AirtimeStats, error) {
	raw, err := c.proto.Call(ctx, "bramble.getAirtime", nil)
	if err != nil {
		return nil, err
	}

	// Newer wire format: {"tiers":[...]}
	var resp AirtimeStats
	if err := json.Unmarshal(raw, &resp); err == nil && len(resp.Tiers) > 0 {
		return &resp, nil
	}

	// Current firmware format: flat fields.
	var flat struct {
		CriticalRemainingMs  int   `json:"critical_remaining_ms"`
		NormalRemainingMs    int   `json:"normal_remaining_ms"`
		BroadcastRemainingMs int   `json:"broadcast_remaining_ms"`
		CriticalMaxMs        int   `json:"critical_max_ms"`
		NormalMaxMs          int   `json:"normal_max_ms"`
		BroadcastMaxMs       int   `json:"broadcast_max_ms"`
		NextRefillMs         int64 `json:"next_refill_ms"`
	}
	if err := json.Unmarshal(raw, &flat); err != nil {
		return nil, fmt.Errorf("bramble: decode AirtimeStats: %w", err)
	}
	resp.Tiers = []AirtimeTier{
		{Name: "critical", RemainingMs: flat.CriticalRemainingMs, MaxMs: flat.CriticalMaxMs, RefillAtMs: flat.NextRefillMs},
		{Name: "normal", RemainingMs: flat.NormalRemainingMs, MaxMs: flat.NormalMaxMs, RefillAtMs: flat.NextRefillMs},
		{Name: "broadcast", RemainingMs: flat.BroadcastRemainingMs, MaxMs: flat.BroadcastMaxMs, RefillAtMs: flat.NextRefillMs},
	}
	return &resp, nil
}

// Ping performs a health check. Returns nil on success.
func (c *Client) Ping(ctx context.Context) error {
	raw, err := c.proto.Call(ctx, "bramble.ping", nil)
	if err != nil {
		return err
	}
	var resp PingResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("bramble: decode PingResponse: %w", err)
	}
	if !resp.Pong {
		return fmt.Errorf("bramble: ping returned pong=false")
	}
	return nil
}

// Messages returns messages stored in the node's message store.
func (c *Client) Messages(ctx context.Context) ([]Message, error) {
	raw, err := c.proto.Call(ctx, "bramble.getMessages", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Messages []Message `json:"messages"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode MessagesResponse: %w", err)
	}
	return resp.Messages, nil
}

// PeerLocations returns location information for all known peers.
func (c *Client) PeerLocations(ctx context.Context) ([]LocationPeer, error) {
	raw, err := c.proto.Call(ctx, "bramble.getPeerLocations", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		PeerLocations []LocationPeer `json:"peerLocations"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode PeerLocationsResponse: %w", err)
	}
	return resp.PeerLocations, nil
}

// ── Action / Config Methods ───────────────────────────────────────────────────

// Send sends a unicast text message to dest (uint32 address).
// Returns a SendResult containing the message_id for delivery tracking via OnAck.
func (c *Client) Send(ctx context.Context, dest uint32, text string) (*SendResult, error) {
	params := map[string]any{"dest": fmt.Sprintf("%08X", dest), "text": text}
	return c.sendMessageWithParams(ctx, params)
}

// SendCritical sends a unicast message marked as critical priority.
func (c *Client) SendCritical(ctx context.Context, dest uint32, text string) (*SendResult, error) {
	params := map[string]any{"dest": fmt.Sprintf("%08X", dest), "text": text, "critical": true}
	return c.sendMessageWithParams(ctx, params)
}

// SendBroadcast sends a text message to all peers on the public channel.
func (c *Client) SendBroadcast(ctx context.Context, text string) (*SendResult, error) {
	params := map[string]any{"text": text}
	return c.sendBroadcastWithParams(ctx, params)
}

// SendBroadcastCritical sends a broadcast marked as critical priority.
func (c *Client) SendBroadcastCritical(ctx context.Context, text string) (*SendResult, error) {
	params := map[string]any{"text": text, "critical": true}
	return c.sendBroadcastWithParams(ctx, params)
}

// Broadcast sends a text message to all peers on the public channel.
// Deprecated: prefer SendBroadcast.
func (c *Client) Broadcast(ctx context.Context, text string) (*SendResult, error) {
	return c.SendBroadcast(ctx, text)
}

// BroadcastOnChannel sends a mesh-wide message on the specified channel index.
// This uses bramble.sendMessage with dest=0xFFFFFFFE (channel broadcast alias).
func (c *Client) BroadcastOnChannel(ctx context.Context, channel int, text string) (*SendResult, error) {
	params := map[string]any{
		"dest":    "FFFFFFFE",
		"text":    text,
		"channel": channel,
	}
	return c.sendMessageWithParams(ctx, params)
}

// BroadcastOnChannelCritical sends a channel-wide message marked as critical priority.
func (c *Client) BroadcastOnChannelCritical(ctx context.Context, channel int, text string) (*SendResult, error) {
	params := map[string]any{
		"dest":     "FFFFFFFE",
		"text":     text,
		"channel":  channel,
		"critical": true,
	}
	return c.sendMessageWithParams(ctx, params)
}

func (c *Client) sendMessageWithParams(ctx context.Context, params map[string]any) (*SendResult, error) {
	raw, err := c.proto.Call(ctx, "bramble.sendMessage", params)
	if err != nil {
		return nil, err
	}
	var resp SendResult
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode SendResult: %w", err)
	}
	if resp.MessageID == "" && resp.PacketID != "" {
		resp.MessageID = resp.PacketID
	}
	return &resp, nil
}

func (c *Client) sendBroadcastWithParams(ctx context.Context, params map[string]any) (*SendResult, error) {
	raw, err := c.proto.Call(ctx, "bramble.sendBroadcast", params)
	if err != nil {
		return nil, err
	}
	var resp SendResult
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode SendResult: %w", err)
	}
	if resp.BroadcastID == "" && resp.MessageID != "" {
		resp.BroadcastID = resp.MessageID
	}
	return &resp, nil
}

// SendProbe broadcasts a probe packet. Results arrive as notifications.
func (c *Client) SendProbe(ctx context.Context) (*SendProbeResult, error) {
	raw, err := c.proto.Call(ctx, "bramble.sendProbe", nil)
	if err != nil {
		return nil, err
	}
	var resp SendProbeResult
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode SendProbeResult: %w", err)
	}
	if resp.ProbeID == 0 && resp.ProbeIDHex != "" {
		var parsed int
		if _, perr := fmt.Sscanf(resp.ProbeIDHex, "%x", &parsed); perr == nil {
			resp.ProbeID = parsed
		}
	}
	return &resp, nil
}

// SetRadio updates radio parameters. Only non-nil fields in config are sent.
func (c *Client) SetRadio(ctx context.Context, config RadioConfig) error {
	params := map[string]any{}
	if config.TxPowerDbm != nil {
		params["tx_power_dbm"] = *config.TxPowerDbm
	}
	if config.SF != nil {
		params["sf"] = *config.SF
	}
	if config.BwKhz != nil {
		params["bw_hz"] = *config.BwKhz * 1000
	}
	if config.CR != nil {
		params["coding_rate"] = *config.CR
	}
	if config.FreqMhz != nil {
		params["frequency_mhz"] = *config.FreqMhz
	}

	raw, err := c.proto.Call(ctx, "bramble.setRadio", params)
	if err != nil {
		return err
	}
	return checkOK(raw, "setRadio")
}

// SetNodeName sets the node display name (max 8 characters).
func (c *Client) SetNodeName(ctx context.Context, name string) error {
	raw, err := c.proto.Call(ctx, "bramble.setNodeName", map[string]string{"name": name})
	if err != nil {
		return err
	}
	return checkOK(raw, "setNodeName")
}

// AddChannel adds a new channel with the given name and PSK.
// Returns the index of the new channel.
func (c *Client) AddChannel(ctx context.Context, name, psk string) (*AddChannelResult, error) {
	raw, err := c.proto.Call(ctx, "bramble.addChannel", map[string]string{"name": name, "psk": psk})
	if err != nil {
		return nil, err
	}
	var resp AddChannelResult
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode AddChannelResult: %w", err)
	}
	return &resp, nil
}

// RemoveChannel removes a channel by index.
func (c *Client) RemoveChannel(ctx context.Context, index int) error {
	raw, err := c.proto.Call(ctx, "bramble.removeChannel", map[string]int{"index": index})
	if err != nil {
		return err
	}
	return checkOK(raw, "removeChannel")
}

// SetDefaultChannel sets which channel index is used for outgoing messages.
func (c *Client) SetDefaultChannel(ctx context.Context, index int) error {
	raw, err := c.proto.Call(ctx, "bramble.setDefaultChannel", map[string]int{"index": index})
	if err != nil {
		return err
	}
	return checkOK(raw, "setDefaultChannel")
}

// SetMailbox enables or disables store-and-forward mailbox mode.
func (c *Client) SetMailbox(ctx context.Context, enabled bool) error {
	raw, err := c.proto.Call(ctx, "bramble.setMailbox", map[string]bool{"enabled": enabled})
	if err != nil {
		return err
	}
	return checkOK(raw, "setMailbox")
}

// SetLocationConfig updates location sharing configuration.
func (c *Client) SetLocationConfig(ctx context.Context, config LocationConfig) error {
	raw, err := c.proto.Call(ctx, "bramble.setLocationConfig", config)
	if err != nil {
		return err
	}
	return checkOK(raw, "setLocationConfig")
}

// SetLocationContact adds or updates a location sharing contact.
func (c *Client) SetLocationContact(ctx context.Context, addr uint32, tier string) error {
	raw, err := c.proto.Call(ctx, "bramble.setLocationContact", map[string]any{
		"address": fmt.Sprintf("%08X", addr),
		"tier":    tier,
	})
	if err != nil {
		return err
	}
	return checkOK(raw, "setLocationContact")
}

// RemoveLocationContact stops sharing location with the specified peer.
func (c *Client) RemoveLocationContact(ctx context.Context, addr uint32) error {
	raw, err := c.proto.Call(ctx, "bramble.removeLocationContact", map[string]any{"address": fmt.Sprintf("%08X", addr)})
	if err != nil {
		return err
	}
	return checkOK(raw, "removeLocationContact")
}

// ShareLocationOnce sends a one-time location update to the specified peer.
func (c *Client) ShareLocationOnce(ctx context.Context, addr uint32) error {
	raw, err := c.proto.Call(ctx, "bramble.shareLocationOnce", map[string]any{"address": fmt.Sprintf("%08X", addr)})
	if err != nil {
		return err
	}
	return checkOK(raw, "shareLocationOnce")
}

// Reboot triggers a software reboot of the node.
func (c *Client) Reboot(ctx context.Context) error {
	raw, err := c.proto.Call(ctx, "bramble.reboot", nil)
	if err != nil {
		return err
	}
	return checkOK(raw, "reboot")
}

// OTAUpdate triggers an OTA firmware update from the provided URL.
func (c *Client) OTAUpdate(ctx context.Context, url string) (*OTAUpdateResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.otaUpdate", OTAUpdateParams{URL: url})
	if err != nil {
		return nil, err
	}
	var resp OTAUpdateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode OTAUpdateResponse: %w", err)
	}
	return &resp, nil
}

// Config returns the full node configuration.
func (c *Client) Config(ctx context.Context) (*ConfigResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.getConfig", nil)
	if err != nil {
		return nil, err
	}
	var resp ConfigResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode ConfigResponse: %w", err)
	}
	return &resp, nil
}

// ── Traffic Debug ─────────────────────────────────────────────────────────────

// SetTrafficDebug configures traffic debug telemetry recording.
// Settings are persisted to NVS and survive reboots.
func (c *Client) SetTrafficDebug(ctx context.Context, params SetTrafficDebugParams) (*SetTrafficDebugResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.setTrafficDebug", params)
	if err != nil {
		return nil, err
	}
	var resp SetTrafficDebugResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode SetTrafficDebugResponse: %w", err)
	}
	return &resp, nil
}

// GetTrafficDebug returns the current traffic debug configuration and buffer state.
func (c *Client) GetTrafficDebug(ctx context.Context) (*GetTrafficDebugResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.getTrafficDebug", nil)
	if err != nil {
		return nil, err
	}
	var resp GetTrafficDebugResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode GetTrafficDebugResponse: %w", err)
	}
	return &resp, nil
}

// GetTrafficEvents retrieves traffic events from the ring buffer.
// Use params.SinceSeq to request only new events (incremental pull).
func (c *Client) GetTrafficEvents(ctx context.Context, params GetTrafficEventsParams) (*GetTrafficEventsResponse, error) {
	raw, err := c.proto.Call(ctx, "bramble.getTrafficEvents", params)
	if err != nil {
		return nil, err
	}
	var resp GetTrafficEventsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("bramble: decode GetTrafficEventsResponse: %w", err)
	}
	return &resp, nil
}

// ── Notification Callbacks ────────────────────────────────────────────────────

// OnMessage registers a callback invoked when a bramble.onMessage notification arrives.
func (c *Client) OnMessage(fn func(Message)) {
	c.mu.Lock()
	c.onMessageFn = fn
	c.mu.Unlock()
}

// OnAck registers a callback invoked when a bramble.onAck notification arrives.
func (c *Client) OnAck(fn func(Ack)) {
	c.mu.Lock()
	c.onAckFn = fn
	c.mu.Unlock()
}

// OnNeighborChange registers a callback invoked when a bramble.onNeighborChange arrives.
func (c *Client) OnNeighborChange(fn func()) {
	c.mu.Lock()
	c.onNeighborFn = fn
	c.mu.Unlock()
}

// OnProbeResult registers a callback invoked when a bramble.onProbeResult notification arrives.
func (c *Client) OnProbeResult(fn func(ProbeResult)) {
	c.mu.Lock()
	c.onProbeResultFn = fn
	c.mu.Unlock()
}

// OnProbeComplete registers a callback invoked when a bramble.onProbeComplete notification arrives.
func (c *Client) OnProbeComplete(fn func(ProbeComplete)) {
	c.mu.Lock()
	c.onProbeCompleteFn = fn
	c.mu.Unlock()
}

// OnTrafficEvent registers a callback invoked when a bramble.onTrafficEvent notification arrives.
// Used for real-time traffic debug event monitoring.
func (c *Client) OnTrafficEvent(fn func(TrafficEvent)) {
	c.mu.Lock()
	c.onTrafficEventFn = fn
	c.mu.Unlock()
}

// OnBroadcastDelivery registers a callback invoked when a bramble.onBroadcastDelivery notification arrives.
func (c *Client) OnBroadcastDelivery(fn func(BroadcastDelivery)) {
	c.mu.Lock()
	c.onBroadcastDeliveryFn = fn
	c.mu.Unlock()
}

// OnWifiEvent registers a callback invoked when a bramble.onWifiEvent notification arrives.
func (c *Client) OnWifiEvent(fn func(WifiEvent)) {
	c.mu.Lock()
	c.onWifiEventFn = fn
	c.mu.Unlock()
}

// OnGpsEvent registers a callback invoked when a bramble.onGpsEvent notification arrives.
func (c *Client) OnGpsEvent(fn func(GpsEvent)) {
	c.mu.Lock()
	c.onGpsEventFn = fn
	c.mu.Unlock()
}

// OnLocationEvent registers a callback invoked when a bramble.onLocationEvent notification arrives.
func (c *Client) OnLocationEvent(fn func(LocationEvent)) {
	c.mu.Lock()
	c.onLocationEventFn = fn
	c.mu.Unlock()
}

// ── helpers ───────────────────────────────────────────────────────────────────

// checkOK unmarshals an OkResponse and verifies the ok field is true.
func checkOK(raw json.RawMessage, method string) error {
	var ok OkResponse
	if err := json.Unmarshal(raw, &ok); err != nil {
		return fmt.Errorf("bramble: decode OkResponse for %s: %w", method, err)
	}
	if !ok.OK {
		return fmt.Errorf("bramble: %s returned ok=false", method)
	}
	return nil
}
