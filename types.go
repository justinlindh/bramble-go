// Package bramble provides a Go client SDK for Bramble mesh nodes.
// It communicates using JSON-RPC 2.0 over serial (UART), WebSocket, or BLE.
package bramble

// Position represents a GPS fix with accuracy and motion data.
type Position struct {
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Alt         float64 `json:"alt"`
	Accuracy    float64 `json:"accuracy"`
	Speed       float64 `json:"speed,omitempty"`
	Heading     float64 `json:"heading,omitempty"`
	TimestampMs int64   `json:"timestampMs"`
}

// StatusResponse is returned by bramble.getStatus.
// Field names match the firmware JSON wire format.
type StatusResponse struct {
	Address                   string `json:"address"`
	FirmwareVersion           string `json:"firmware_version"`
	ProtocolVersion           string `json:"protocol_version"`
	Hardware                  string `json:"hardware"`
	RadioOk                   bool   `json:"radio_ok"`
	Peers                     int    `json:"peers"`
	BeaconTx                  int    `json:"beacon_tx"`
	BeaconRx                  int    `json:"beacon_rx"`
	PacketsTx                 int    `json:"packets_tx"`
	PacketsRx                 int    `json:"packets_rx"`
	UptimeSec                 int    `json:"uptime_s"`
	SupportsDeliveryEventSync bool   `json:"supports_delivery_event_sync,omitempty"`
}

// IdentityResponse is returned by bramble.getIdentity.
type IdentityResponse struct {
	Address    string `json:"address"`
	PubkeyHash string `json:"pubkey_hash"`
}

// VersionResponse is returned by bramble.getVersion.
type VersionResponse struct {
	FirmwareVersion           string `json:"firmware_version"`
	ProtocolVersion           string `json:"protocol_version"`
	Hardware                  string `json:"hardware"`
	SupportsDeliveryEventSync bool   `json:"supports_delivery_event_sync,omitempty"`
}

// DeliveryReplayEvent is returned by bramble.getDeliveryEvents.
type DeliveryReplayEvent struct {
	EventSeq    uint32                 `json:"event_seq"`
	EventID     string                 `json:"event_id,omitempty"`
	EventType   string                 `json:"event_type"`
	PacketID    string                 `json:"packet_id,omitempty"`
	BroadcastID string                 `json:"broadcast_id,omitempty"`
	TimestampMs int64                  `json:"timestamp_ms"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

// DeliveryReplayResponse is returned by bramble.getDeliveryEvents.
type DeliveryReplayResponse struct {
	Events         []DeliveryReplayEvent `json:"events"`
	LatestEventSeq uint32                `json:"latest_event_seq"`
}

// PingResponse is returned by bramble.ping.
type PingResponse struct {
	Pong            bool   `json:"pong"`
	Address         string `json:"address"`
	ProtocolVersion string `json:"protocol_version"`
}

// Neighbor represents a directly heard radio neighbor.
// Firmware sends "address" as hex string, not numeric.
type Neighbor struct {
	Address string  `json:"address"`
	RSSI    int     `json:"rssi"`
	SNR     float64 `json:"snr"`
	// LastSeenAgoMs is milliseconds since this neighbor was last heard
	// (relative duration, not an absolute timestamp).
	LastSeenAgoMs int64 `json:"last_seen_ms"`
}

// Route is a routing table entry.
type Route struct {
	Dest       string `json:"dest"`
	NextHop    string `json:"next_hop"`
	HopCount   int    `json:"hop_count"`
	Metric     int    `json:"metric"`
	State      string `json:"state"`
	LastUsedMs int64  `json:"last_used_ms"`
	UseCount   int    `json:"use_count,omitempty"`
}

// Channel is a configured channel.
type Channel struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	HasPsk    bool   `json:"hasPsk,omitempty"`
	Epoch     int    `json:"epoch,omitempty"`
	IsDefault bool   `json:"is_default"`
}

// Message is a stored or incoming message.
type Message struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text"`
	Tier      string `json:"tier,omitempty"`
	Timestamp int64  `json:"timestamp"`
	MsgID     string `json:"msgId,omitempty"`
}

// AirtimeTier holds the airtime budget for a single priority tier.
type AirtimeTier struct {
	Name        string `json:"name"`
	RemainingMs int    `json:"remainingMs"`
	MaxMs       int    `json:"maxMs"`
	UsedPct     int    `json:"usedPct"`
	RefillAtMs  int64  `json:"refillAtMs"`
}

// AirtimeStats holds all airtime tier data returned by bramble.getAirtime.
type AirtimeStats struct {
	Tiers []AirtimeTier `json:"tiers"`
}

// LocationContact is a location sharing contact configuration.
type LocationContact struct {
	Addr             string `json:"addr"`
	Tier             string `json:"tier"`
	IntervalSec      int    `json:"intervalSec"`
	DistanceTriggerM int    `json:"distanceTriggerM"`
}

// LocationPeer holds location data for a peer node.
type LocationPeer struct {
	Addr          string    `json:"addr"`
	Name          string    `json:"name"`
	Tier          string    `json:"tier"`
	Position      *Position `json:"position,omitempty"`
	GridSquare    string    `json:"gridSquare,omitempty"`
	Online        bool      `json:"online"`
	LastUpdatedMs int64     `json:"lastUpdatedMs"`
}

// RelayHop is a single hop in a relay path.
type RelayHop struct {
	Addr string `json:"addr"`
	RSSI int    `json:"rssi"`
}

// Ack is the delivery acknowledgment payload for bramble.onAck.
type Ack struct {
	PacketID  int        `json:"packetId"`
	Status    string     `json:"status"`
	RelayPath []RelayHop `json:"relayPath,omitempty"`
}

// BroadcastDelivery is the telemetry payload for bramble.onBroadcastDelivery.
type BroadcastDelivery struct {
	BroadcastID string `json:"broadcast_id"`
	Recipient   string `json:"recipient"`
	Status      string `json:"status"`
	TimestampMs int64  `json:"timestamp_ms,omitempty"`
}

// WifiEvent is delivered via bramble.onWifiEvent notifications.
type WifiEvent struct {
	Event     string `json:"event"`
	Mode      string `json:"mode"`
	Connected bool   `json:"connected"`
	SSID      string `json:"ssid,omitempty"`
	IP        string `json:"ip,omitempty"`
	RSSI      int    `json:"rssi,omitempty"`
}

// GpsEvent is delivered via bramble.onGpsEvent notifications.
type GpsEvent struct {
	Event string  `json:"event"`
	Valid bool    `json:"valid,omitempty"`
	Lat   float64 `json:"lat,omitempty"`
	Lon   float64 `json:"lon,omitempty"`
	AltM  int     `json:"alt_m,omitempty"`
	Sats  int     `json:"sats,omitempty"`
}

// LocationEvent is delivered via bramble.onLocationEvent notifications.
type LocationEvent struct {
	Event       string `json:"event"`
	Peer        string `json:"peer,omitempty"`
	Tier        int    `json:"tier"`
	TimestampMs uint32 `json:"timestamp_ms"`
	RSSI        int    `json:"rssi,omitempty"`
	SNR         int    `json:"snr,omitempty"`
	Count       int    `json:"count,omitempty"`
}

// ProbeResult is delivered via bramble.onProbeResult / probe.ack notifications.
type ProbeResult struct {
	ResponderAddr string   `json:"responderAddr"`
	HopCount      int      `json:"hopCount"`
	RSSI          int      `json:"rssi"`
	SNR           float64  `json:"snr"`
	PathLen       int      `json:"pathLen"`
	RelayPath     []string `json:"relayPath,omitempty"`
	ReceivedAt    int64    `json:"receivedAt"`
}

// ProbeComplete is delivered when a probe window closes.
type ProbeComplete struct {
	ProbeID int `json:"probeId"`
}

// LocationUpdate is delivered via location.update notifications.
type LocationUpdate struct {
	Addr          string    `json:"addr"`
	Name          string    `json:"name"`
	Tier          string    `json:"tier"`
	Position      *Position `json:"position,omitempty"`
	Online        bool      `json:"online"`
	LastUpdatedMs int64     `json:"lastUpdatedMs"`
}

// SendResult is returned by bramble.sendMessage / bramble.sendBroadcast.
type SendResult struct {
	MessageID      string `json:"message_id,omitempty"`
	PacketID       string `json:"packetId,omitempty"`
	BroadcastID    string `json:"broadcast_id,omitempty"`
	Status         string `json:"status"`
	Fragmented     bool   `json:"fragmented,omitempty"`
	FragmentsTotal int    `json:"fragments_total,omitempty"`
	MaxBytes       int    `json:"max_bytes,omitempty"`
	ActualBytes    int    `json:"actual_bytes,omitempty"`
	Broadcast      bool   `json:"broadcast,omitempty"`
	Channel        int    `json:"channel,omitempty"`
}

// SendProbeResult is returned by bramble.sendProbe.
type SendProbeResult struct {
	ProbeID    int    `json:"probeId,omitempty"`
	ProbeIDHex string `json:"probe_id,omitempty"`
	AckWindow  int    `json:"ackWindow,omitempty"`
	OK         bool   `json:"ok,omitempty"`
}

// AddChannelResult is returned by bramble.addChannel.
type AddChannelResult struct {
	Index int `json:"index"`
}

// OTAUpdateParams contains parameters for bramble.otaUpdate.
type OTAUpdateParams struct {
	URL string `json:"url"`
}

// OTAUpdateResponse is returned by bramble.otaUpdate.
type OTAUpdateResponse struct {
	OK        bool   `json:"ok"`
	Note      string `json:"note,omitempty"`
	Partition string `json:"partition,omitempty"`
}

// OkResponse is a generic success response.
type OkResponse struct {
	OK bool `json:"ok"`
}

// ConfigResponse is returned by bramble.getConfig.
// Matches firmware wire format.
type ConfigResponse struct {
	NodeName string         `json:"node_name"`
	Address  string         `json:"address"`
	Radio    ConfigRadio    `json:"radio"`
	Channels []Channel      `json:"channels"`
	Location LocationConfig `json:"location"`
}

// ConfigRadio is the radio section of ConfigResponse.
type ConfigRadio struct {
	FrequencyMhz int    `json:"frequency_mhz"`
	SF           int    `json:"sf"`
	BwHz         int    `json:"bw_hz"`
	TxPowerDbm   int    `json:"tx_power_dbm"`
	Profile      string `json:"profile"`
}

// RadioConfig contains the radio parameters for bramble.setRadio.
// All fields are optional pointers; nil means leave unchanged.
type RadioConfig struct {
	TxPowerDbm *int     `json:"txPowerDbm,omitempty"`
	SF         *int     `json:"sf,omitempty"`
	BwKhz      *int     `json:"bwKhz,omitempty"`
	CR         *int     `json:"cr,omitempty"`
	FreqMhz    *float64 `json:"freqMhz,omitempty"`
}

// LocationContactRule is a per-peer location sharing policy rule.
type LocationContactRule struct {
	Address   string `json:"address"`
	Enabled   *bool  `json:"enabled,omitempty"`
	Tier      string `json:"tier,omitempty"`
	IntervalS *int   `json:"interval_s,omitempty"`
}

// LocationChannelTarget is a per-channel location sharing policy rule.
type LocationChannelTarget struct {
	Channel   int    `json:"channel"`
	Enabled   *bool  `json:"enabled,omitempty"`
	Tier      string `json:"tier,omitempty"`
	IntervalS *int   `json:"interval_s,omitempty"`
}

// LocationConfig contains the location configuration for bramble.getConfig and bramble.setLocationConfig.
type LocationConfig struct {
	Enabled        *bool                   `json:"enabled,omitempty"`
	DefaultTier    *string                 `json:"default_tier,omitempty"`
	IntervalS      *int                    `json:"interval_s,omitempty"`
	Source         *string                 `json:"source,omitempty"`
	ContactRules   []LocationContactRule   `json:"contact_rules,omitempty"`
	ChannelTargets []LocationChannelTarget `json:"channel_targets,omitempty"`
}

// ── Traffic Debug Types ──────────────────────────────────────────────────────

// SetTrafficDebugParams contains parameters for bramble.setTrafficDebug.
type SetTrafficDebugParams struct {
	Enabled    *bool `json:"enabled,omitempty"`
	IncludeTx  *bool `json:"include_tx,omitempty"`
	IncludeRx  *bool `json:"include_rx,omitempty"`
	SampleRate *int  `json:"sample_rate,omitempty"` // 0-100
}

// SetTrafficDebugResponse is returned by bramble.setTrafficDebug.
type SetTrafficDebugResponse struct {
	OK         bool `json:"ok"`
	Enabled    bool `json:"enabled"`
	IncludeTx  bool `json:"include_tx"`
	IncludeRx  bool `json:"include_rx"`
	SampleRate int  `json:"sample_rate"`
}

// GetTrafficDebugResponse is returned by bramble.getTrafficDebug.
type GetTrafficDebugResponse struct {
	Enabled        bool `json:"enabled"`
	IncludeTx      bool `json:"include_tx"`
	IncludeRx      bool `json:"include_rx"`
	SampleRate     int  `json:"sample_rate"`
	BufferCapacity int  `json:"buffer_capacity"`
	BufferCount    int  `json:"buffer_count"`
	DroppedCount   int  `json:"dropped_count"`
}

// GetTrafficEventsParams contains parameters for bramble.getTrafficEvents.
type GetTrafficEventsParams struct {
	SinceSeq *uint32 `json:"since_seq,omitempty"`
	Limit    *int    `json:"limit,omitempty"` // 1-512, default 100
}

// GetTrafficEventsResponse is returned by bramble.getTrafficEvents.
type GetTrafficEventsResponse struct {
	Events         []TrafficEvent `json:"events"`
	Returned       int            `json:"returned"`
	TotalAvailable int            `json:"total_available"`
}

// TrafficEvent represents a single TX or RX packet event for telemetry.
type TrafficEvent struct {
	Seq         uint32 `json:"seq"`
	TimestampMs uint32 `json:"timestamp_ms"`
	PktType     int    `json:"pkt_type"`
	Category    string `json:"category"`     // beacon, timesync, routing, ack, chat, maintenance, other, unknown
	AirtimeTier string `json:"airtime_tier"` // none, normal, critical, broadcast, unknown
	PacketLen   int    `json:"packet_len"`
	RSSI        int    `json:"rssi"` // 0 for TX events
	IsTx        bool   `json:"is_tx"`
}
