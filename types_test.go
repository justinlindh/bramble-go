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
	if contains(got, "tier") || contains(got, "msgId") {
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
	if !contains(got, `"sf":9`) || !contains(got, `"freqMhz":915.5`) {
		t.Fatalf("missing set fields: %s", got)
	}
	if contains(got, "txPowerDbm") || contains(got, "bwKhz") || contains(got, "cr") {
		t.Fatalf("nil pointer fields should be omitted: %s", got)
	}
}

func TestLocationPeerUnmarshalOptionalPosition(t *testing.T) {
	withPos := []byte(`{"addr":"0x1","name":"n","tier":"normal","online":true,"lastUpdatedMs":5,"position":{"lat":1,"lon":2,"alt":3,"accuracy":4,"timestampMs":6}}`)
	var lp LocationPeer
	if err := json.Unmarshal(withPos, &lp); err != nil {
		t.Fatalf("unmarshal with position failed: %v", err)
	}
	if lp.Position == nil || lp.Position.Lat != 1 || lp.Position.TimestampMs != 6 {
		t.Fatalf("unexpected position decode: %+v", lp.Position)
	}

	withoutPos := []byte(`{"addr":"0x1","name":"n","tier":"normal","online":false,"lastUpdatedMs":5}`)
	var lpNoPos LocationPeer
	if err := json.Unmarshal(withoutPos, &lpNoPos); err != nil {
		t.Fatalf("unmarshal without position failed: %v", err)
	}
	if lpNoPos.Position != nil {
		t.Fatalf("expected nil position when omitted, got %+v", lpNoPos.Position)
	}
}

func TestSendProbeResultSupportsLegacyAndCurrentFields(t *testing.T) {
	cur := []byte(`{"probeId":42,"ackWindow":15,"ok":true}`)
	var r SendProbeResult
	if err := json.Unmarshal(cur, &r); err != nil {
		t.Fatalf("unmarshal current failed: %v", err)
	}
	if r.ProbeID != 42 || r.AckWindow != 15 || !r.OK {
		t.Fatalf("unexpected current decode: %+v", r)
	}

	legacy := []byte(`{"probe_id":"0x2a"}`)
	if err := json.Unmarshal(legacy, &r); err != nil {
		t.Fatalf("unmarshal legacy failed: %v", err)
	}
	if r.ProbeIDHex != "0x2a" {
		t.Fatalf("unexpected legacy decode: %+v", r)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
