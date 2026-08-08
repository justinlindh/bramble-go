// Package bramble provides a Go client SDK for Bramble mesh nodes.
// It communicates using JSON-RPC 2.0 over serial (UART), WebSocket, or BLE.
package bramble

import (
	"encoding/json"
	"strings"
)

// Position represents a GPS fix with accuracy and motion data.
type Position struct {
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Alt         float64 `json:"alt"`
	Accuracy    float64 `json:"accuracy"`
	Speed       float64 `json:"speed,omitempty"`
	Heading     float64 `json:"heading,omitempty"`
	TimestampMs int64   `json:"timestamp_ms"`
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

// WifiStatus is returned by bramble.getWifiStatus.
// Field names match firmware JSON wire format.
type WifiStatus struct {
	Mode    string `json:"mode"`
	SSID    string `json:"ssid"`
	IP      string `json:"ip"`
	RSSI    int    `json:"rssi"`
	MAC     string `json:"mac"`
	Clients int    `json:"clients"`
}

// SetWifiConfigResponse is returned by bramble.setWifiConfig.
type SetWifiConfigResponse struct {
	OK      bool   `json:"ok"`
	Applied string `json:"applied"` // "live" or "reboot_required"
}

// BatteryStatus is returned by bramble.getBattery.
type BatteryStatus struct {
	VoltageMV  int `json:"voltage_mv"`
	Percentage int `json:"percentage"`
}

// GPSPosition is returned by bramble.getGpsPosition.
type GPSPosition struct {
	Lat        float64 `json:"lat,omitempty"`
	Lon        float64 `json:"lon,omitempty"`
	Alt        float64 `json:"alt,omitempty"`
	SpeedKmh   float64 `json:"speed_kmh,omitempty"`
	HeadingDeg float64 `json:"heading_deg,omitempty"`
	AccuracyM  float64 `json:"accuracy_m,omitempty"`
	// Timestamp is the GPS fix time as Unix epoch seconds.
	// Note: unlike other SDK fields which use _ms suffixes for milliseconds,
	// the firmware returns this as whole seconds with the wire key "timestamp".
	Timestamp int64 `json:"timestamp,omitempty"`
	Valid     bool  `json:"valid"`
}

// BeaconPolicyConfig is the config section returned by bramble.getBeaconPolicy.
type BeaconPolicyConfig struct {
	Enabled        bool   `json:"enabled"`
	Mode           string `json:"mode"`
	BaseIntervalMs int    `json:"baseIntervalMs"`
	MinIntervalMs  int    `json:"minIntervalMs"`
	MaxIntervalMs  int    `json:"maxIntervalMs"`
	DenseThreshold int    `json:"denseThreshold"`
	ChurnThreshold int    `json:"churnThreshold"`
	ChurnWindowMs  int    `json:"churnWindowMs"`
}

// BeaconPolicyStatus is the status section returned by bramble.getBeaconPolicy.
type BeaconPolicyStatus struct {
	ActiveMode        string `json:"activeMode"`
	CurrentIntervalMs int    `json:"currentIntervalMs"`
	NeighborCount     int    `json:"neighborCount"`
	ChurnEvents       int    `json:"churnEvents"`
	LastTransitionMs  int64  `json:"lastTransitionMs"`
	InBackoff         bool   `json:"inBackoff"`
}

// BeaconPolicyResponse is returned by bramble.getBeaconPolicy.
type BeaconPolicyResponse struct {
	Config BeaconPolicyConfig `json:"config"`
	Status BeaconPolicyStatus `json:"status"`
}

// SetBeaconPolicyParams contains params for bramble.setBeaconPolicy.
type SetBeaconPolicyParams struct {
	Enabled        *bool  `json:"enabled,omitempty"`
	Mode           string `json:"mode,omitempty"`
	BaseIntervalMs *int   `json:"baseIntervalMs,omitempty"`
	MinIntervalMs  *int   `json:"minIntervalMs,omitempty"`
	MaxIntervalMs  *int   `json:"maxIntervalMs,omitempty"`
	DenseThreshold *int   `json:"denseThreshold,omitempty"`
	ChurnThreshold *int   `json:"churnThreshold,omitempty"`
	ChurnWindowMs  *int   `json:"churnWindowMs,omitempty"`
}

// BacklightResponse is returned by bramble.setBacklight.
type BacklightResponse struct {
	Level int `json:"level"`
}

// SleepResponse is returned by bramble.sleep.
type SleepResponse struct {
	OK         bool   `json:"ok"`
	WakeAfterS int    `json:"wake_after_s,omitempty"`
	Note       string `json:"note,omitempty"`
}

// AudioStatus is returned by bramble.getAudioStatus.
type AudioStatus struct {
	Available bool `json:"available"`
	Volume    int  `json:"volume"`
	Muted     bool `json:"muted"`
	Playing   bool `json:"playing"`
}

// StorageInfo is returned by bramble.getStorageInfo.
type StorageInfo struct {
	SDPresent  bool   `json:"sd_present"`
	MountPoint string `json:"mount_point,omitempty"`
}

// SetBroadcastTelemetryModeResponse is returned by bramble.setBroadcastTelemetryMode.
type SetBroadcastTelemetryModeResponse struct {
	OK                     bool   `json:"ok"`
	BroadcastTelemetryMode string `json:"broadcast_telemetry_mode"`
}

// DiagnosticsHeap describes per-region heap metrics returned by bramble.getDiagnostics.
type DiagnosticsHeap struct {
	InternalFree             float64 `json:"internal_free"`
	InternalMinEverFree      float64 `json:"internal_min_ever_free"`
	InternalLargestFreeBlock float64 `json:"internal_largest_free_block"`
	DMAFree                  float64 `json:"dma_free"`
	DMALargestFreeBlock      float64 `json:"dma_largest_free_block"`
	PSRAMFree                float64 `json:"psram_free"`
	PSRAMMinEverFree         float64 `json:"psram_min_ever_free"`
}

// TaskStackHWM is one task stack high-water-mark entry from bramble.getDiagnostics.
type TaskStackHWM struct {
	Task     string  `json:"task"`
	HWMWords float64 `json:"hwm_words"`
	HWMBytes float64 `json:"hwm_bytes"`
}

// DiagnosticsResponse is returned by bramble.getDiagnostics.
type DiagnosticsResponse struct {
	UptimeS      float64         `json:"uptime_s"`
	FreeHeap     float64         `json:"free_heap"`
	Heap         DiagnosticsHeap `json:"heap"`
	TaskStackHWM []TaskStackHWM  `json:"task_stack_hwm"`
}

// IdentityResponse is returned by bramble.getIdentity.
type IdentityResponse struct {
	Address    string `json:"address"`
	PubkeyHash string `json:"pubkey_hash"`
	// Ed25519Pub is the node's full Ed25519 identity public key (64 hex
	// chars). Added with the trust-anchor feature; empty on older firmware.
	Ed25519Pub string `json:"ed25519_pub,omitempty"`
}

// NetworkKeyStatusResponse is returned by bramble.getNetworkKeyStatus.
// Provisioned false means the node holds no network key and is INERT: it is not
// meshing and will not participate in the authenticated control plane.
// Fingerprint is SHA256(key)[0:4] as 8 lowercase hex, and reads as the all-zero
// sentinel "00000000" while unprovisioned. The key itself is never returned.
type NetworkKeyStatusResponse struct {
	Provisioned bool   `json:"provisioned"`
	Fingerprint string `json:"fingerprint"`
}

// GenerateNetworkKeyResponse is returned by bramble.generateNetworkKey. Key is
// the freshly minted 32-byte network key as 64 lowercase hex, returned exactly
// once: the node is already provisioned with it and will never read it back.
// Treat it as the secret it is, record it out of band, and never log it.
type GenerateNetworkKeyResponse struct {
	Key         string `json:"key"`
	Fingerprint string `json:"fingerprint"`
}

// AnchorStatusResponse is returned by bramble.getAnchorStatus. Anchored
// reports whether a fleet anchor public key is provisioned on the node;
// Endorsed reports whether the node holds a cert that verifies against the
// currently provisioned anchor and this node's own key.
type AnchorStatusResponse struct {
	Anchored          bool   `json:"anchored"`
	AnchorFingerprint string `json:"anchor_fingerprint,omitempty"`
	Endorsed          bool   `json:"endorsed"`
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
	EventSeq    uint32         `json:"event_seq"`
	EventID     string         `json:"event_id,omitempty"`
	EventType   string         `json:"event_type"`
	PacketID    string         `json:"packet_id,omitempty"`
	BroadcastID string         `json:"broadcast_id,omitempty"`
	TimestampMs int64          `json:"timestamp_ms"`
	Payload     map[string]any `json:"payload,omitempty"`
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
	Name    string  `json:"name,omitempty"`
	RSSI    int     `json:"rssi"`
	SNR     float64 `json:"snr"`
	// LastSeenAgoMs is milliseconds since this neighbor was last heard
	// (relative duration, not an absolute timestamp).
	LastSeenAgoMs int64 `json:"last_seen_ms"`
	// DeliveryRate is 0-255 where 255 = 100% packet delivery rate.
	DeliveryRate int `json:"delivery_rate"`
	// AirtimeRemaining is 0-100% airtime budget remaining for this neighbor.
	AirtimeRemaining int `json:"airtime_remaining"`
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
	HasPsk    bool   `json:"has_psk,omitempty"`
	Epoch     int    `json:"epoch,omitempty"`
	IsDefault bool   `json:"is_default"`
}

// Message is a stored or incoming message.
type Message struct {
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"text"`
	Tier string `json:"tier,omitempty"`
	// Timestamp is seconds since epoch (firmware key: timestamp_s).
	Timestamp int64  `json:"timestamp_s"`
	MsgID     string `json:"msg_id,omitempty"`
	Status    string `json:"status,omitempty"`
	// Broadcast reports whether the message was addressed to every node rather
	// than to a single peer or a channel. It is the authoritative routing
	// signal on the live bramble.onMessage notification, which does NOT carry a
	// "to" field, so callers must classify DM vs broadcast on this flag (and
	// Channel), not on To.
	Broadcast bool `json:"broadcast,omitempty"`
	// Channel is the mesh channel index for a channel message. The firmware
	// reports -1 (or 0) for DMs and plain broadcasts; a value greater than 0
	// identifies a channel message.
	Channel int `json:"channel,omitempty"`
}

// AirtimeTier holds the airtime budget for a single priority tier.
type AirtimeTier struct {
	Name        string `json:"name"`
	RemainingMs int    `json:"remaining_ms"`
	MaxMs       int    `json:"max_ms"`
	UsedPct     int    `json:"used_pct"`
	RefillAtMs  int64  `json:"refill_at_ms"`
}

// AirtimeStats holds all airtime tier data returned by bramble.getAirtime.
type AirtimeStats struct {
	Tiers []AirtimeTier `json:"tiers"`
}

// LocationPeer holds location data for a peer node.
type LocationPeer struct {
	Addr          string    `json:"addr"`
	Name          string    `json:"name"`
	Tier          string    `json:"tier"`
	Position      *Position `json:"position,omitempty"`
	GridSquare    string    `json:"grid_square,omitempty"`
	Online        bool      `json:"online"`
	LastUpdatedMs int64     `json:"last_updated_ms"`
}

// RelayHop is a single hop in a relay path.
type RelayHop struct {
	Addr string `json:"addr"`
	RSSI int    `json:"rssi"`
}

// Ack is the delivery acknowledgment payload for bramble.onAck.
// Field names match the firmware wire format.
type Ack struct {
	// From is the source node address as a hex string (present on delivery acks).
	From string `json:"from,omitempty"`
	// PacketID is the acknowledged packet identifier as a hex string (firmware key: packet_id).
	PacketID   string     `json:"packet_id"`
	Status     string     `json:"status"`
	RSSIAtDest int        `json:"rssi_at_dest,omitempty"`
	RelayPath  []RelayHop `json:"relay_path,omitempty"`
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

// GPSEvent is delivered via bramble.onGpsEvent notifications.
type GPSEvent struct {
	Event string  `json:"event"`
	Valid bool    `json:"valid,omitempty"`
	Lat   float64 `json:"lat,omitempty"`
	Lon   float64 `json:"lon,omitempty"`
	AltM  int     `json:"alt_m,omitempty"`
	Sats  int     `json:"sats,omitempty"`
}

// LocationTier represents a location privacy tier.
type LocationTier string

const (
	// LocationTierFull shares full position (lat/lon/alt/speed/heading).
	LocationTierFull LocationTier = "full"
	// LocationTierCoarse shares approximate position (~1km grid).
	LocationTierCoarse LocationTier = "coarse"
	// LocationTierPresence shares online/offline only.
	LocationTierPresence LocationTier = "presence"
)

// locationTierFromInt converts a firmware uint8 tier enum to a LocationTier string.
// The firmware onLocationEvent notification sends tier as a raw integer while
// all other RPC methods send it as a string — this bridges the inconsistency.
func locationTierFromInt(v int) LocationTier {
	switch v {
	case 0:
		return LocationTierFull
	case 1:
		return LocationTierCoarse
	case 2:
		return LocationTierPresence
	default:
		return LocationTierCoarse
	}
}

// UnmarshalJSON handles both string ("full") and integer (0) representations
// of the tier field. The firmware onLocationEvent notification sends an integer
// while all other RPC methods send a string.
func (t *LocationTier) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if json.Unmarshal(data, &s) == nil {
		*t = LocationTier(s)
		return nil
	}
	// Fall back to integer.
	var v int
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = locationTierFromInt(v)
	return nil
}

// LocationEvent is delivered via bramble.onLocationEvent notifications.
type LocationEvent struct {
	Event       string       `json:"event"`
	Peer        string       `json:"peer,omitempty"`
	Tier        LocationTier `json:"tier"`
	TimestampMs int64        `json:"timestamp_ms"`
	RSSI        int          `json:"rssi,omitempty"`
	SNR         int          `json:"snr,omitempty"`
	Count       int          `json:"count,omitempty"`
}

// ProbeResult is delivered via bramble.onProbeResult notifications.
// Field names match the firmware wire format (snake_case).
type ProbeResult struct {
	// Address is the responder node address as a hex string (firmware key: address).
	Address    string  `json:"address"`
	Hops       int     `json:"hops"`
	RSSI       int     `json:"rssi"`
	SNR        float64 `json:"snr"`
	LatencyMs  int64   `json:"latency_ms"`
	ProbeRound int     `json:"probe_round"`
	// ProbeID is the probe identifier as a hex string (firmware key: probe_id).
	ProbeID string `json:"probe_id,omitempty"`
}

// ProbeResponder is a single responder entry in a ProbeComplete notification.
type ProbeResponder struct {
	Address    string  `json:"address"`
	Hops       int     `json:"hops"`
	RSSI       int     `json:"rssi"`
	SNR        float64 `json:"snr"`
	LatencyMs  int64   `json:"latency_ms"`
	SeenRounds int     `json:"seen_rounds"`
}

// ProbeComplete is delivered when a probe window closes.
type ProbeComplete struct {
	// ProbeID is a hex string (e.g. "A1B2C3D4").
	ProbeID     string           `json:"probe_id"`
	UniqueCount int              `json:"unique_count"`
	DurationMs  int64            `json:"duration_ms"`
	RoundsTotal int              `json:"rounds_total"`
	Responders  []ProbeResponder `json:"responders,omitempty"`
}

// PeerLocationEvent is delivered via bramble.onPeerLocation notifications.
// The firmware sends this with null params whenever a peer location update is
// received and cached. Callers should call PeerLocations to fetch the updated
// location data. The event itself carries no payload.
type PeerLocationEvent struct{}

// IdentityChangeEvent is delivered via bramble.onIdentityChange notifications.
// The firmware sends this when an address collision is detected and a new
// identity (address) has been generated. Callers should update any cached
// local address after receiving this event.
type IdentityChangeEvent struct {
	// NewAddress is the node's newly assigned address as a hex string (e.g. "1A2B3C4D").
	NewAddress string `json:"new_address"`
	// Reason describes why the identity changed (e.g. "address_collision").
	Reason string `json:"reason"`
}

// LocationUpdate is delivered via location.update notifications.
type LocationUpdate struct {
	Addr          string    `json:"addr"`
	Name          string    `json:"name"`
	Tier          string    `json:"tier"`
	Position      *Position `json:"position,omitempty"`
	Online        bool      `json:"online"`
	LastUpdatedMs int64     `json:"last_updated_ms"`
}

// SendResult is returned by bramble.sendMessage / bramble.sendBroadcast.
type SendResult struct {
	MessageID      string `json:"message_id,omitempty"`
	PacketID       string `json:"packet_id,omitempty"`
	BroadcastID    string `json:"broadcast_id,omitempty"`
	Status         string `json:"status"`
	Fragmented     bool   `json:"fragmented,omitempty"`
	FragmentsTotal int    `json:"fragments_total,omitempty"`
	MaxBytes       int    `json:"max_bytes,omitempty"`
	ActualBytes    int    `json:"actual_bytes,omitempty"`
	Broadcast      bool   `json:"broadcast,omitempty"`
	// Channel is a pointer so callers can distinguish "no channel info" (nil)
	// from "channel 0" (*Channel == 0). The zero value of int was ambiguous.
	Channel *int `json:"channel,omitempty"`
}

// SendProbeResult is returned by bramble.sendProbe.
// Firmware sends probe_id (string), ack_window (int), ok (bool).
type SendProbeResult struct {
	// ProbeIDHex is the probe ID as a hex string (firmware key: probe_id).
	ProbeIDHex string `json:"probe_id,omitempty"`
	// AckWindow is the acknowledgment window in seconds (firmware key: ack_window).
	AckWindow int  `json:"ack_window,omitempty"`
	OK        bool `json:"ok,omitempty"`
	// ProbeID is a convenience field parsed from ProbeIDHex by the client; not a JSON field.
	ProbeID int `json:"-"`
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
//
// Field names and units match the firmware wire format:
//   - BwHz: bandwidth in Hz (e.g. 125000), matching ConfigRadio.BwHz
//   - FrequencyMhz: frequency in MHz (e.g. 915.0), matching ConfigRadio.FrequencyMhz
type RadioConfig struct {
	TxPowerDbm   *int     `json:"tx_power_dbm,omitempty"`
	SF           *int     `json:"sf,omitempty"`
	BwHz         *int     `json:"bw_hz,omitempty"`
	CR           *int     `json:"cr,omitempty"`
	FrequencyMhz *float64 `json:"frequency_mhz,omitempty"`
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

// TrafficDebugResponse is returned by bramble.getTrafficDebug.
type TrafficDebugResponse struct {
	Enabled        bool `json:"enabled"`
	IncludeTx      bool `json:"include_tx"`
	IncludeRx      bool `json:"include_rx"`
	SampleRate     int  `json:"sample_rate"`
	BufferCapacity int  `json:"buffer_capacity"`
	BufferCount    int  `json:"buffer_count"`
	DroppedCount   int  `json:"dropped_count"`
}

// TrafficEventsParams contains parameters for bramble.getTrafficEvents.
type TrafficEventsParams struct {
	SinceSeq *uint32 `json:"since_seq,omitempty"`
	Limit    *int    `json:"limit,omitempty"` // 1-512, default 100
}

// TrafficEventsResponse is returned by bramble.getTrafficEvents.
type TrafficEventsResponse struct {
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

// ActionPrefix and ActionSuffix are the CTCP ACTION delimiters used for /me messages.
const ActionPrefix = "\x01ACTION "
const ActionSuffix = "\x01"

// IsAction returns true if this message is a /me action.
func (m Message) IsAction() bool {
	return strings.HasPrefix(m.Text, ActionPrefix) && strings.HasSuffix(m.Text, ActionSuffix)
}

// ActionText returns the action text without CTCP wrapping, or empty string if not an action.
func (m Message) ActionText() string {
	if !m.IsAction() {
		return ""
	}
	return m.Text[len(ActionPrefix) : len(m.Text)-len(ActionSuffix)]
}

// WrapAction wraps text in CTCP ACTION format for sending.
func WrapAction(text string) string {
	return ActionPrefix + text + ActionSuffix
}
