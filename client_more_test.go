package bramble

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClient_QueryMethodsCoverage(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"address":"AABBCCDD","pubkey_hash":"pkh123"}}`)
	id, err := c.Identity(ctx)
	if err != nil || id.Address != "AABBCCDD" || id.PubkeyHash != "pkh123" {
		t.Fatalf("Identity failed: resp=%+v err=%v", id, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"routes":[{"dest":"11111111","next_hop":"22222222","hop_count":2,"metric":5,"state":"active","last_used_ms":99}]}}`)
	routes, err := c.Routes(ctx)
	if err != nil || len(routes) != 1 || routes[0].Dest != "11111111" {
		t.Fatalf("Routes failed: routes=%+v err=%v", routes, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":3,"result":{"messages":[{"from":"A","to":"B","text":"hello","timestamp_s":123}]}}`)
	msgs, err := c.Messages(ctx)
	if err != nil || len(msgs) != 1 || msgs[0].Text != "hello" {
		t.Fatalf("Messages failed: msgs=%+v err=%v", msgs, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":4,"result":{"peerLocations":[{"addr":"CAFEBABE","name":"peer","tier":"normal","online":true,"last_updated_ms":42}]}}`)
	peers, err := c.PeerLocations(ctx)
	if err != nil || len(peers) != 1 || peers[0].Addr != "CAFEBABE" {
		t.Fatalf("PeerLocations failed: peers=%+v err=%v", peers, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":5,"result":{"critical_remaining_ms":10,"normal_remaining_ms":20,"broadcast_remaining_ms":30,"critical_max_ms":100,"normal_max_ms":200,"broadcast_max_ms":300,"next_refill_ms":999}}`)
	air, err := c.Airtime(ctx)
	if err != nil || len(air.Tiers) != 3 || air.Tiers[0].Name != "critical" || air.Tiers[2].RemainingMs != 30 {
		t.Fatalf("Airtime failed: air=%+v err=%v", air, err)
	}
}

func TestClient_DeliveryEvents_WithAndWithoutLimit(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"events":[{"event_seq":7,"event_type":"delivered","packet_id":"AA","timestamp_ms":111}],"latest_event_seq":8}}`)
	resp, err := c.DeliveryEvents(ctx, 5, 25)
	if err != nil || len(resp.Events) != 1 || resp.LatestEventSeq != 8 {
		t.Fatalf("DeliveryEvents(limit) failed: resp=%+v err=%v", resp, err)
	}

	sent := mock.Sent()
	if len(sent) == 0 || !strings.Contains(sent[0], `"sinceEventSeq":5`) || !strings.Contains(sent[0], `"limit":25`) {
		t.Fatalf("expected sinceEventSeq and limit in request, got %v", sent)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"events":[],"latest_event_seq":9}}`)
	_, err = c.DeliveryEvents(ctx, 8, 0)
	if err != nil {
		t.Fatalf("DeliveryEvents(no limit) error: %v", err)
	}

	sent = mock.Sent()
	if len(sent) < 2 {
		t.Fatalf("expected 2 requests, got %d", len(sent))
	}
	if !strings.Contains(sent[1], `"sinceEventSeq":8`) || strings.Contains(sent[1], `"limit"`) {
		t.Fatalf("expected sinceEventSeq only, got %s", sent[1])
	}
}

func TestClient_ActionAndConfigMethodsCoverage(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"probe_id":"2A","ack_window":3,"ok":true}}`)
	probe, err := c.SendProbe(ctx)
	if err != nil || probe.ProbeID != 0x2A || probe.AckWindow != 3 {
		t.Fatalf("SendProbe failed: probe=%+v err=%v", probe, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"message_id":"B1","status":"queued"}}`)
	bcast, err := c.SendBroadcast(ctx, "hi all")
	if err != nil || bcast.BroadcastID != "B1" {
		t.Fatalf("SendBroadcast fallback failed: bcast=%+v err=%v", bcast, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":3,"result":{"ok":true}}`)
	if err := c.SetNodeName(ctx, "node1"); err != nil {
		t.Fatalf("SetNodeName: %v", err)
	}
	mock.QueueResponse(`{"jsonrpc":"2.0","id":4,"result":{"index":3}}`)
	add, err := c.AddChannel(ctx, "team", "psk")
	if err != nil || add.Index != 3 {
		t.Fatalf("AddChannel: resp=%+v err=%v", add, err)
	}
	mock.QueueResponse(`{"jsonrpc":"2.0","id":5,"result":{"ok":true}}`)
	if err := c.RemoveChannel(ctx, 3); err != nil {
		t.Fatalf("RemoveChannel: %v", err)
	}
	mock.QueueResponse(`{"jsonrpc":"2.0","id":6,"result":{"ok":true}}`)
	if err := c.SetDefaultChannel(ctx, 1); err != nil {
		t.Fatalf("SetDefaultChannel: %v", err)
	}
	mock.QueueResponse(`{"jsonrpc":"2.0","id":7,"result":{"ok":true}}`)
	if err := c.SetMailbox(ctx, true); err != nil {
		t.Fatalf("SetMailbox: %v", err)
	}
	mock.QueueResponse(`{"jsonrpc":"2.0","id":8,"result":{"ok":true}}`)
	if err := c.SetLocationContact(ctx, 0xAABBCCDD, "normal"); err != nil { t.Fatalf("SetLocationContact: %v", err) }
	enabled := false
	interval := 900
	mock.QueueResponse(`{"jsonrpc":"2.0","id":9,"result":{"ok":true}}`)
	if err := c.SetLocationContact(ctx, 0xAABBCCDD, "normal", LocationContactRule{Enabled: &enabled, IntervalS: &interval}); err != nil {
		t.Fatalf("SetLocationContact with rule override: %v", err)
	}
	mock.QueueResponse(`{"jsonrpc":"2.0","id":10,"result":{"ok":true}}`)
	if err := c.RemoveLocationContact(ctx, 0xAABBCCDD); err != nil { t.Fatalf("RemoveLocationContact: %v", err) }
	mock.QueueResponse(`{"jsonrpc":"2.0","id":11,"result":{"ok":true}}`)
	if err := c.ShareLocationOnce(ctx, 0xAABBCCDD); err != nil { t.Fatalf("ShareLocationOnce: %v", err) }
	mock.QueueResponse(`{"jsonrpc":"2.0","id":12,"result":{"ok":true}}`)
	if err := c.Reboot(ctx); err != nil { t.Fatalf("Reboot: %v", err) }

	sent := strings.Join(mock.Sent(), "\n")
	for _, want := range []string{
		`"method":"bramble.setNodeName"`,
		`"method":"bramble.addChannel"`,
		`"method":"bramble.removeChannel"`,
		`"method":"bramble.setDefaultChannel"`,
		`"method":"bramble.setMailbox"`,
		`"method":"bramble.setLocationContact"`,
		`"address":"AABBCCDD"`,
		`"enabled":false`,
		`"interval_s":900`,
		`"method":"bramble.removeLocationContact"`,
		`"method":"bramble.shareLocationOnce"`,
		`"method":"bramble.reboot"`,
	} {
		if !strings.Contains(sent, want) {
			t.Fatalf("expected sent requests to include %s\nall sent:\n%s", want, sent)
		}
	}
}

func TestClient_MissingRPCWrappersCoverage(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"voltage_mv":4021,"percentage":83}}`)
	battery, err := c.GetBattery(ctx)
	if err != nil || battery.VoltageMV != 4021 || battery.Percentage != 83 {
		t.Fatalf("GetBattery failed: resp=%+v err=%v", battery, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"lat":37.1,"lon":-122.2,"alt":15,"speed_kmh":1.2,"heading_deg":270,"accuracy_m":6.5,"timestamp":1730000000,"valid":true}}`)
	gps, err := c.GetGpsPosition(ctx)
	if err != nil || !gps.Valid || gps.Lat != 37.1 || gps.AccuracyM != 6.5 {
		t.Fatalf("GetGpsPosition failed: resp=%+v err=%v", gps, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":3,"result":{"config":{"enabled":true,"mode":"adaptive","baseIntervalMs":10000,"minIntervalMs":3000,"maxIntervalMs":30000,"denseThreshold":8,"churnThreshold":3,"churnWindowMs":60000},"status":{"activeMode":"adaptive","currentIntervalMs":12000,"neighborCount":4,"churnEvents":1,"lastTransitionMs":1730000100,"inBackoff":false}}}`)
	policy, err := c.GetBeaconPolicy(ctx)
	if err != nil || policy.Config.Mode != "adaptive" || policy.Status.CurrentIntervalMs != 12000 {
		t.Fatalf("GetBeaconPolicy failed: resp=%+v err=%v", policy, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":4,"result":{"ok":true}}`)
	if err := c.SetBeaconPolicy(ctx, SetBeaconPolicyParams{Mode: "fixed"}); err != nil {
		t.Fatalf("SetBeaconPolicy failed: %v", err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":5,"result":{"level":180}}`)
	backlight, err := c.SetBacklight(ctx, 180)
	if err != nil || backlight.Level != 180 {
		t.Fatalf("SetBacklight failed: resp=%+v err=%v", backlight, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":6,"result":{"ok":true,"wake_after_s":30,"note":"Entering deep sleep with timer wake"}}`)
	sleep, err := c.Sleep(ctx, 30)
	if err != nil || !sleep.OK || sleep.WakeAfterS != 30 {
		t.Fatalf("Sleep failed: resp=%+v err=%v", sleep, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":7,"result":{"ok":true}}`)
	if err := c.PlayTone(ctx, "message_rx"); err != nil {
		t.Fatalf("PlayTone failed: %v", err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":8,"result":{"ok":true}}`)
	if err := c.SetVolume(ctx, 75); err != nil {
		t.Fatalf("SetVolume failed: %v", err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":9,"result":{"ok":true}}`)
	if err := c.SetMuted(ctx, true); err != nil {
		t.Fatalf("SetMuted failed: %v", err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":10,"result":{"available":true,"volume":75,"muted":true,"playing":false}}`)
	audio, err := c.GetAudioStatus(ctx)
	if err != nil || !audio.Available || audio.Volume != 75 || !audio.Muted {
		t.Fatalf("GetAudioStatus failed: resp=%+v err=%v", audio, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":11,"result":{"sd_present":true,"mount_point":"/sdcard"}}`)
	storage, err := c.GetStorageInfo(ctx)
	if err != nil || !storage.SDPresent || storage.MountPoint != "/sdcard" {
		t.Fatalf("GetStorageInfo failed: resp=%+v err=%v", storage, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":12,"result":{"ok":true,"broadcast_telemetry_mode":"path_sampled"}}`)
	telemetry, err := c.SetBroadcastTelemetryMode(ctx, "path_sampled")
	if err != nil || !telemetry.OK || telemetry.BroadcastTelemetryMode != "path_sampled" {
		t.Fatalf("SetBroadcastTelemetryMode failed: resp=%+v err=%v", telemetry, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":13,"result":{"ok":true}}`)
	if err := c.SetAuthToken(ctx, "abc123"); err != nil {
		t.Fatalf("SetAuthToken failed: %v", err)
	}

	sent := strings.Join(mock.Sent(), "\n")
	for _, want := range []string{
		`"method":"bramble.getBattery"`,
		`"method":"bramble.getGpsPosition"`,
		`"method":"bramble.getBeaconPolicy"`,
		`"method":"bramble.setBeaconPolicy"`,
		`"mode":"fixed"`,
		`"method":"bramble.setBacklight"`,
		`"level":180`,
		`"method":"bramble.sleep"`,
		`"wake_after_s":30`,
		`"method":"bramble.playTone"`,
		`"tone":"message_rx"`,
		`"method":"bramble.setVolume"`,
		`"volume":75`,
		`"method":"bramble.setMuted"`,
		`"muted":true`,
		`"method":"bramble.getAudioStatus"`,
		`"method":"bramble.getStorageInfo"`,
		`"method":"bramble.setBroadcastTelemetryMode"`,
		`"mode":"path_sampled"`,
		`"method":"bramble.setAuthToken"`,
		`"token":"abc123"`,
	} {
		if !strings.Contains(sent, want) {
			t.Fatalf("expected sent requests to include %s\nall sent:\n%s", want, sent)
		}
	}
}

func TestClient_Close_CallsTransportClose(t *testing.T) {
	c, mock := setupRawClient(t)
	if !mock.IsConnected() {
		t.Fatal("expected connected mock")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if mock.IsConnected() {
		t.Fatal("expected mock disconnected after Close")
	}
}

func TestClient_TrafficDebugMethodsAndCallbacks(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	enabled := true
	sample := 50
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"enabled":true,"include_tx":true,"include_rx":false,"sample_rate":50}}`)
	setResp, err := c.SetTrafficDebug(ctx, SetTrafficDebugParams{Enabled: &enabled, SampleRate: &sample})
	if err != nil || !setResp.OK || setResp.SampleRate != 50 {
		t.Fatalf("SetTrafficDebug failed: resp=%+v err=%v", setResp, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":2,"result":{"enabled":true,"include_tx":true,"include_rx":false,"sample_rate":50,"buffer_capacity":256,"buffer_count":10,"dropped_count":2}}`)
	getResp, err := c.GetTrafficDebug(ctx)
	if err != nil || getResp.BufferCapacity != 256 || getResp.DroppedCount != 2 {
		t.Fatalf("GetTrafficDebug failed: resp=%+v err=%v", getResp, err)
	}

	mock.QueueResponse(`{"jsonrpc":"2.0","id":3,"result":{"events":[{"seq":1,"timestamp_ms":123,"pkt_type":1,"category":"chat","airtime_tier":"normal","packet_len":16,"rssi":-80,"is_tx":false}],"returned":1,"total_available":1}}`)
	eventsResp, err := c.GetTrafficEvents(ctx, GetTrafficEventsParams{})
	if err != nil || eventsResp.Returned != 1 || len(eventsResp.Events) != 1 {
		t.Fatalf("GetTrafficEvents failed: resp=%+v err=%v", eventsResp, err)
	}

	probeCh := make(chan ProbeResult, 1)
	completeCh := make(chan ProbeComplete, 1)
	trafficCh := make(chan TrafficEvent, 1)
	c.OnProbeResult(func(p ProbeResult) { probeCh <- p })
	c.OnProbeComplete(func(pc ProbeComplete) { completeCh <- pc })
	c.OnTrafficEvent(func(te TrafficEvent) { trafficCh <- te })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onProbeResult","params":{"address":"AABBCCDD","hops":1,"rssi":-70,"snr":7.5,"latency_ms":40,"probe_round":2,"probe_id":"AA"}}`)
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onProbeComplete","params":{"probe_id":170}}`)
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onTrafficEvent","params":{"seq":9,"timestamp_ms":999,"pkt_type":2,"category":"routing","airtime_tier":"critical","packet_len":12,"rssi":-90,"is_tx":false}}`)

	select {
	case p := <-probeCh:
		if p.Address != "AABBCCDD" || p.ProbeID != "AA" {
			t.Fatalf("bad probe result: %+v", p)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting probe result")
	}
	select {
	case p := <-completeCh:
		if p.ProbeID != 170 {
			t.Fatalf("bad probe complete: %+v", p)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting probe complete")
	}
	select {
	case te := <-trafficCh:
		if te.Seq != 9 || te.Category != "routing" {
			t.Fatalf("bad traffic event: %+v", te)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting traffic event")
	}
}

func TestClient_CheckOKFalsePath(t *testing.T) {
	c, mock := setupRawClient(t)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":false}}`)
	if err := c.SetMailbox(ctx, false); err == nil {
		t.Fatal("expected SetMailbox ok=false error")
	}
}
