// Package bramble provides a Go client SDK for Bramble mesh nodes.
package bramble

// StatusResponse is returned by bramble.getStatus.
type StatusResponse struct {
	Address         string `json:"address"`
	FirmwareVersion string `json:"firmware_version"`
	ProtocolVersion string `json:"protocol_version"`
	Hardware        string `json:"hardware"`
	RadioOK         bool   `json:"radio_ok"`
	Peers           int    `json:"peers"`
	BeaconTX        int    `json:"beacon_tx"`
	BeaconRX        int    `json:"beacon_rx"`
	PacketsTX       int    `json:"packets_tx"`
	PacketsRX       int    `json:"packets_rx"`
	UptimeS         int    `json:"uptime_s"`
}

// IdentityResponse is returned by bramble.getIdentity.
type IdentityResponse struct {
	Address    string `json:"address"`
	PubkeyHash string `json:"pubkey_hash"`
}

// VersionResponse is returned by bramble.getVersion.
type VersionResponse struct {
	FirmwareVersion string `json:"firmware_version"`
	ProtocolVersion string `json:"protocol_version"`
	Hardware        string `json:"hardware"`
}

// PingResponse is returned by bramble.ping.
type PingResponse struct {
	Pong            bool   `json:"pong"`
	Address         string `json:"address"`
	ProtocolVersion string `json:"protocol_version"`
}

// Neighbor represents a mesh neighbor.
type Neighbor struct {
	Address    string `json:"address"`
	RSSI       int    `json:"rssi"`
	SNR        int    `json:"snr"`
	LastSeenMs int    `json:"last_seen_ms"`
}

// Route represents a routing table entry.
type Route struct {
	Dest      string `json:"dest"`
	NextHop   string `json:"next_hop"`
	HopCount  int    `json:"hop_count"`
	ExpiresMs int    `json:"expires_ms"`
}

// Channel represents a mesh channel.
type Channel struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

// Message represents a mesh message.
type Message struct {
	ID        string   `json:"id"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Text      string   `json:"text"`
	Timestamp int64    `json:"timestamp"`
	ChannelID int      `json:"channel_id"`
	Delivered bool     `json:"delivered"`
	RelayPath []string `json:"relay_path,omitempty"`
}

// AirtimeStats is returned by bramble.getAirtime.
type AirtimeStats struct {
	CriticalRemainingMs  int `json:"critical_remaining_ms"`
	NormalRemainingMs    int `json:"normal_remaining_ms"`
	BroadcastRemainingMs int `json:"broadcast_remaining_ms"`
	CriticalMaxMs        int `json:"critical_max_ms"`
	NormalMaxMs          int `json:"normal_max_ms"`
	BroadcastMaxMs       int `json:"broadcast_max_ms"`
}

// LocationPeer represents a peer with location data.
type LocationPeer struct {
	Address   string  `json:"address"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Accuracy  float64 `json:"accuracy"`
	Tier      string  `json:"tier"`
	Timestamp int64   `json:"timestamp"`
}

// ProbeResult represents a network reachability probe response.
type ProbeResult struct {
	Address string `json:"address"`
	Hops    int    `json:"hops"`
	RSSI    int    `json:"rssi"`
	RTTMs   int    `json:"rtt_ms"`
}

// SendResult is returned by bramble.sendMessage.
type SendResult struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// RadioConfig for bramble.setRadio.
type RadioConfig struct {
	Frequency *float64 `json:"frequency,omitempty"`
	SF        *int     `json:"sf,omitempty"`
	BW        *int     `json:"bw,omitempty"`
	TXPower   *int     `json:"tx_power,omitempty"`
}

// LocationConfig for bramble.setLocationConfig.
type LocationConfig struct {
	Enabled   bool `json:"enabled"`
	IntervalS *int `json:"interval_s,omitempty"`
}

// ConfigResponse is returned by bramble.getConfig.
type ConfigResponse struct {
	NodeName string    `json:"node_name"`
	Channels []Channel `json:"channels"`
	Radio    struct {
		Frequency float64 `json:"frequency"`
		SF        int     `json:"sf"`
		BW        int     `json:"bw"`
		TXPower   int     `json:"tx_power"`
	} `json:"radio"`
	Mailbox bool `json:"mailbox"`
}

// Ack represents a delivery receipt notification.
type Ack struct {
	MessageID string   `json:"message_id"`
	From      string   `json:"from"`
	RelayPath []string `json:"relay_path,omitempty"`
}
