package bramble

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClient_ExportTopology(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"twin_schema":1,` +
		`"node":{"address":"1191C6E0","name":"ridge-1","firmware_version":"0.2.0-dev",` +
		`"protocol_version":"0.5.0","hardware":"heltec_v3","uptime_s":3600},` +
		`"radio":{"frequency_mhz":906.875,"sf":9,"bw_hz":125000,"coding_rate":1,` +
		`"tx_power_dbm":17,"region":"US915","regulatory":"FCC","max_duty_cycle_pct":100,` +
		`"duty_cycle_enforced":false},` +
		`"neighbors":[{"address":"DEADBEEF","name":"ridge-2","rssi":-92,"snr":6.5,` +
		`"last_seen_ms":4200,"delivery_rate":250,"airtime_remaining":88}],` +
		`"routes":[{"dest":"CAFEBABE","next_hop":"DEADBEEF","hop_count":2,"metric":140,` +
		`"state":"active","last_used_ms":1500,"use_count":9}]}}`)

	export, err := c.ExportTopology(ctx)
	if err != nil {
		t.Fatalf("ExportTopology: %v", err)
	}
	if export.TwinSchema != 1 {
		t.Errorf("twin_schema: got %d, want 1", export.TwinSchema)
	}
	if export.Node.Address != "1191C6E0" || export.Node.Name != "ridge-1" {
		t.Errorf("node identity decoded as %+v", export.Node)
	}
	if export.Node.Hardware != "heltec_v3" || export.Node.UptimeS != 3600 {
		t.Errorf("node block decoded as %+v", export.Node)
	}

	// The PHY numbers are what price a frame's time-on-air, so they are the
	// fields a reconstructed scenario is wrong without.
	if export.Radio.FrequencyMhz != 906.875 || export.Radio.SF != 9 || export.Radio.BwHz != 125000 {
		t.Errorf("radio PHY decoded as %+v", export.Radio)
	}
	if export.Radio.CodingRate != 1 || export.Radio.TxPowerDbm != 17 {
		t.Errorf("radio PHY decoded as %+v", export.Radio)
	}
	if export.Radio.Region != "US915" || export.Radio.Regulatory != "FCC" {
		t.Errorf("frequency plan decoded as %+v", export.Radio)
	}
	if export.Radio.MaxDutyCyclePct != 100 || export.Radio.DutyCycleEnforced {
		t.Errorf("duty cycle decoded as pct=%d enforced=%t",
			export.Radio.MaxDutyCyclePct, export.Radio.DutyCycleEnforced)
	}

	if len(export.Neighbors) != 1 {
		t.Fatalf("got %d neighbors, want 1", len(export.Neighbors))
	}
	n := export.Neighbors[0]
	if n.Address != "DEADBEEF" || n.RSSI != -92 || n.SNR != 6.5 || n.LastSeenAgoMs != 4200 {
		t.Errorf("neighbor decoded as %+v", n)
	}
	if len(export.Routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(export.Routes))
	}
	r := export.Routes[0]
	if r.Dest != "CAFEBABE" || r.NextHop != "DEADBEEF" || r.HopCount != 2 || r.State != "active" {
		t.Errorf("route decoded as %+v", r)
	}

	sent := mock.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0], `"method":"bramble.exportTopology"`) {
		t.Fatalf("unexpected request payload: %v", sent)
	}
}

// TestClient_ExportTopology_IsolatedNode covers a node that hears nobody: the
// arrays come back empty rather than absent, and an export of a node with no
// links is still a valid document.
func TestClient_ExportTopology_IsolatedNode(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"twin_schema":1,` +
		`"node":{"address":"1191C6E0","firmware_version":"0.2.0-dev",` +
		`"protocol_version":"0.5.0","hardware":"heltec_v3","uptime_s":12},` +
		`"radio":{"frequency_mhz":868.1,"sf":7,"bw_hz":250000,"coding_rate":2,` +
		`"tx_power_dbm":14,"region":"EU868","regulatory":"ETSI","max_duty_cycle_pct":1,` +
		`"duty_cycle_enforced":true},"neighbors":[],"routes":[]}}`)

	export, err := c.ExportTopology(ctx)
	if err != nil {
		t.Fatalf("ExportTopology: %v", err)
	}
	if len(export.Neighbors) != 0 || len(export.Routes) != 0 {
		t.Errorf("expected an isolated node, got %d neighbors and %d routes",
			len(export.Neighbors), len(export.Routes))
	}
	// An unset node name is omitted on the wire, not sent empty.
	if export.Node.Name != "" {
		t.Errorf("name: got %q, want empty", export.Node.Name)
	}
	// A duty-cycled plan is the case a capacity estimate must not silently
	// treat as unlimited.
	if export.Radio.MaxDutyCyclePct != 1 || !export.Radio.DutyCycleEnforced {
		t.Errorf("duty cycle decoded as pct=%d enforced=%t",
			export.Radio.MaxDutyCyclePct, export.Radio.DutyCycleEnforced)
	}
}

func TestClient_ExportTopology_ServerError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-1005,"message":"Unauthorized"}}`)
	if _, err := c.ExportTopology(ctx); err == nil {
		t.Fatal("expected an error for an unauthorized response")
	} else if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected the error to mention Unauthorized, got %v", err)
	}
}

func TestClient_ExportTopology_DecodeFailure(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"twin_schema":1,"neighbors":"none"}}`)
	_, err := c.ExportTopology(ctx)
	if err == nil {
		t.Fatal("expected a decode error for a neighbors string where an array belongs")
	}
	if !strings.Contains(err.Error(), "decode TopologyExport") {
		t.Fatalf("expected the error to name the type, got %v", err)
	}
}
