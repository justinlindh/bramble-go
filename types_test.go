package bramble

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStatusResponseUnmarshalWireNames(t *testing.T) {
	in := []byte(`{"address":"0x1234","firmware_version":"1.2.3","protocol_version":"2","hardware":"esp32","radio_ok":true,"peers":4,"beacon_tx":1,"beacon_rx":2,"packets_tx":3,"packets_rx":4,"uptime_s":99}`)
	var s StatusResponse
	if err := json.Unmarshal(in, &s); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if s.FirmwareVersion != "1.2.3" || s.ProtocolVersion != "2" || !s.RadioOk || s.UptimeSec != 99 {
		t.Fatalf("unexpected decoded struct: %+v", s)
	}
}

func TestMessageMarshalOmitsEmptyOptionalFields(t *testing.T) {
	m := Message{From: "a", To: "b", Text: "hi", Timestamp: 123}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(out)
	if contains(got, "tier") || contains(got, "msg_id") {
		t.Fatalf("optional fields should be omitted, got %s", got)
	}
}

func TestRadioConfigMarshalPointers(t *testing.T) {
	sf := 9
	freq := 915.5
	r := RadioConfig{SF: &sf, FreqMhz: &freq}
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(out)
	if !contains(got, `"sf":9`) || !contains(got, `"freq_mhz":915.5`) {
		t.Fatalf("missing set fields: %s", got)
	}
	if contains(got, "tx_power_dbm") || contains(got, "bw_khz") || contains(got, "cr") {
		t.Fatalf("nil pointer fields should be omitted: %s", got)
	}
}

func TestLocationPeerUnmarshalOptionalPosition(t *testing.T) {
	withPos := []byte(`{"addr":"ABCDEF01","name":"n","tier":"normal","online":true,"last_updated_ms":5,"position":{"lat":1,"lon":2,"alt":3,"accuracy":4,"timestamp_ms":6}}`)
	var lp LocationPeer
	if err := json.Unmarshal(withPos, &lp); err != nil {
		t.Fatalf("unmarshal with position failed: %v", err)
	}
	if lp.Position == nil || lp.Position.Lat != 1 || lp.Position.TimestampMs != 6 {
		t.Fatalf("unexpected position decode: %+v", lp.Position)
	}
	if lp.Addr != "ABCDEF01" {
		t.Fatalf("expected addr string hex, got %q", lp.Addr)
	}

	withoutPos := []byte(`{"addr":"ABCDEF01","name":"n","tier":"normal","online":false,"last_updated_ms":5}`)
	var lpNoPos LocationPeer
	if err := json.Unmarshal(withoutPos, &lpNoPos); err != nil {
		t.Fatalf("unmarshal without position failed: %v", err)
	}
	if lpNoPos.Position != nil {
		t.Fatalf("expected nil position when omitted, got %+v", lpNoPos.Position)
	}
}

func TestLocationConfigMarshalCanonicalFieldNames(t *testing.T) {
	enabled := true
	defaultTier := "critical"
	intervalS := 90
	source := "gps"
	cfg := LocationConfig{
		Enabled:     &enabled,
		DefaultTier: &defaultTier,
		IntervalS:   &intervalS,
		Source:      &source,
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(out)
	if !contains(got, `"default_tier":"critical"`) || !contains(got, `"interval_s":90`) || !contains(got, `"source":"gps"`) {
		t.Fatalf("missing canonical location fields: %s", got)
	}
	if contains(got, `"tier"`) {
		t.Fatalf("should not emit compatibility tier alias: %s", got)
	}
}

func TestSendProbeResultUnmarshalFirmwareFormat(t *testing.T) {
	// Firmware sends: {"ok":true,"probe_id":"0000002A","ack_window":5,"rounds_total":3}
	fw := []byte(`{"ok":true,"probe_id":"0000002A","ack_window":5}`)
	var r SendProbeResult
	if err := json.Unmarshal(fw, &r); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if r.ProbeIDHex != "0000002A" {
		t.Fatalf("expected ProbeIDHex=0000002A, got %q", r.ProbeIDHex)
	}
	if r.AckWindow != 5 {
		t.Fatalf("expected AckWindow=5, got %d", r.AckWindow)
	}
	if !r.OK {
		t.Fatalf("expected OK=true")
	}
}

func TestNeighborUnmarshalDeliveryRateAndAirtime(t *testing.T) {
	// Firmware sends delivery_rate and airtime_remaining as snake_case.
	in := []byte(`{"address":"AABBCCDD","rssi":-80,"snr":7.5,"last_seen_ms":1000,"delivery_rate":204,"airtime_remaining":75}`)
	var n Neighbor
	if err := json.Unmarshal(in, &n); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if n.DeliveryRate != 204 {
		t.Fatalf("expected DeliveryRate=204, got %d", n.DeliveryRate)
	}
	if n.AirtimeRemaining != 75 {
		t.Fatalf("expected AirtimeRemaining=75, got %d", n.AirtimeRemaining)
	}
	if n.LastSeenAgoMs != 1000 {
		t.Fatalf("expected LastSeenAgoMs=1000, got %d", n.LastSeenAgoMs)
	}
}

func TestMessageUnmarshalTimestampS(t *testing.T) {
	// Firmware sends timestamp_s (seconds), not timestamp.
	in := []byte(`{"from":"AABBCCDD","to":"EEFF0011","text":"hello","timestamp_s":1700000000}`)
	var m Message
	if err := json.Unmarshal(in, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m.Timestamp != 1700000000 {
		t.Fatalf("expected Timestamp=1700000000, got %d", m.Timestamp)
	}
}

func TestProbeResultUnmarshalFirmwareFormat(t *testing.T) {
	// Firmware bramble.onProbeResult notification format.
	in := []byte(`{"address":"AABBCCDD","hops":2,"rssi":-65,"snr":9.5,"latency_ms":350,"probe_round":1,"probe_id":"0000002A"}`)
	var pr ProbeResult
	if err := json.Unmarshal(in, &pr); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if pr.Address != "AABBCCDD" {
		t.Fatalf("expected Address=AABBCCDD, got %q", pr.Address)
	}
	if pr.Hops != 2 {
		t.Fatalf("expected Hops=2, got %d", pr.Hops)
	}
	if pr.LatencyMs != 350 {
		t.Fatalf("expected LatencyMs=350, got %d", pr.LatencyMs)
	}
	if pr.ProbeID != "0000002A" {
		t.Fatalf("expected ProbeID=0000002A, got %q", pr.ProbeID)
	}
}

func TestAckUnmarshalFirmwareFormat(t *testing.T) {
	// Firmware bramble.onAck notification format (delivery ack).
	in := []byte(`{"from":"AABBCCDD","packet_id":"0000002A","status":"delivered","rssi_at_dest":-70,"relay_path":[{"addr":"AABBCCDD","rssi":-70}]}`)
	var a Ack
	if err := json.Unmarshal(in, &a); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if a.PacketID != "0000002A" {
		t.Fatalf("expected PacketID=0000002A, got %q", a.PacketID)
	}
	if a.From != "AABBCCDD" {
		t.Fatalf("expected From=AABBCCDD, got %q", a.From)
	}
	if a.Status != "delivered" {
		t.Fatalf("expected Status=delivered, got %q", a.Status)
	}
	if a.RSSIAtDest != -70 {
		t.Fatalf("expected RSSIAtDest=-70, got %d", a.RSSIAtDest)
	}
	if len(a.RelayPath) != 1 {
		t.Fatalf("expected 1 relay hop, got %d", len(a.RelayPath))
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestMessageIsAction(t *testing.T) {
	m := Message{Text: "\x01ACTION waves hello\x01"}
	if !m.IsAction() {
		t.Fatal("expected IsAction")
	}
	if m.ActionText() != "waves hello" {
		t.Fatalf("got %q", m.ActionText())
	}

	m2 := Message{Text: "normal message"}
	if m2.IsAction() {
		t.Fatal("expected not action")
	}
}

func TestWrapAction(t *testing.T) {
	got := WrapAction("waves hello")
	want := "\x01ACTION waves hello\x01"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
