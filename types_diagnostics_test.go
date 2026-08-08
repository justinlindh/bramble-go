package bramble

import (
	"encoding/json"
	"reflect"
	"testing"
)

// ptr returns a pointer to v, for building the optional fields of
// DiagnosticsResponse and DiagnosticsRadioHealth in test fixtures.
func ptr[T any](v T) *T { return &v }

// fullDiagnosticsPayload is a bramble.getDiagnostics response carrying every
// field the RPC contract defines, including the optional objects. Addresses
// and identifiers are documentation placeholders.
const fullDiagnosticsPayload = `{
  "uptime_s": 4211,
  "free_heap": 182360,
  "heap": {
    "internal_free": 150112,
    "internal_min_ever_free": 128944,
    "internal_largest_free_block": 65524,
    "dma_free": 149088,
    "dma_largest_free_block": 65524,
    "psram_free": 2048576,
    "psram_min_ever_free": 2031104
  },
  "task_stack_hwm": [
    {"task": "main", "hwm_words": 1024, "hwm_bytes": 4096},
    {"task": "radio", "hwm_words": 768, "hwm_bytes": 3072}
  ],
  "backpressure": {
    "flood_relay_drops": 7,
    "probe_ingress": {
      "accepted": 42,
      "dropped_reply": 3,
      "dropped_forward": 11
    }
  },
  "radio_health": {
    "supported": true,
    "tx_power_dbm": 22,
    "device_errors": 0,
    "device_errors_str": "none",
    "pa_ramp_error": false,
    "status": 44,
    "chip_mode": "STBY_RC",
    "cmd_status": "data-available",
    "ocp": 56,
    "ocp_expected": 56,
    "ocp_ok": true,
    "pa_duty_cycle": 4,
    "pa_hp_max": 7,
    "pa_rated_dbm": 22
  },
  "gps_rx_bytes": 98304,
  "gps_rx_lines": 1521,
  "gps_chip": "$PAIR021,AG3335M_V1.0",
  "gps_rx_overruns": 0,
  "gps_rx_errors": 0,
  "gps_rx_disabled": 0,
  "gps_rx_rearm_fail": 0
}`

func fullDiagnosticsWant() DiagnosticsResponse {
	return DiagnosticsResponse{
		UptimeS:  4211,
		FreeHeap: 182360,
		Heap: DiagnosticsHeap{
			InternalFree:             150112,
			InternalMinEverFree:      128944,
			InternalLargestFreeBlock: 65524,
			DMAFree:                  149088,
			DMALargestFreeBlock:      65524,
			PSRAMFree:                2048576,
			PSRAMMinEverFree:         2031104,
		},
		TaskStackHWM: []TaskStackHWM{
			{Task: "main", HWMWords: 1024, HWMBytes: 4096},
			{Task: "radio", HWMWords: 768, HWMBytes: 3072},
		},
		Backpressure: &DiagnosticsBackpressure{
			FloodRelayDrops: 7,
			ProbeIngress: DiagnosticsProbeIngress{
				Accepted:       42,
				DroppedReply:   3,
				DroppedForward: 11,
			},
		},
		RadioHealth: &DiagnosticsRadioHealth{
			Supported:       true,
			TxPowerDBm:      22,
			DeviceErrors:    ptr(0),
			DeviceErrorsStr: ptr("none"),
			PARampError:     ptr(false),
			Status:          ptr(44),
			ChipMode:        ptr("STBY_RC"),
			CmdStatus:       ptr("data-available"),
			OCP:             ptr(56),
			OCPExpected:     ptr(56),
			OCPOK:           ptr(true),
			PADutyCycle:     ptr(4),
			PAHPMax:         ptr(7),
			PARatedDBm:      ptr(22),
		},
		GPSRxBytes:     ptr(98304.0),
		GPSRxLines:     ptr(1521.0),
		GPSChip:        ptr("$PAIR021,AG3335M_V1.0"),
		GPSRxOverruns:  ptr(0.0),
		GPSRxErrors:    ptr(0.0),
		GPSRxDisabled:  ptr(0.0),
		GPSRxRearmFail: ptr(0.0),
	}
}

// TestDiagnosticsResponseDecodesFullPayload proves the typed struct carries
// every contract field, so re-serializing it (which is what the CLI's --json
// output does) does not silently drop the diagnostics the firmware sent.
func TestDiagnosticsResponseDecodesFullPayload(t *testing.T) {
	var got DiagnosticsResponse
	if err := json.Unmarshal([]byte(fullDiagnosticsPayload), &got); err != nil {
		t.Fatalf("unmarshal full payload: %v", err)
	}

	want := fullDiagnosticsWant()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded struct mismatch:\n got %#v\nwant %#v", got, want)
	}
}

// TestDiagnosticsResponseRoundTripPreservesKeys re-marshals a decoded full
// payload and re-decodes it, which is the exact path a JSON-printing consumer
// takes. Every key present on the wire must survive it.
func TestDiagnosticsResponseRoundTripPreservesKeys(t *testing.T) {
	var decoded DiagnosticsResponse
	if err := json.Unmarshal([]byte(fullDiagnosticsPayload), &decoded); err != nil {
		t.Fatalf("unmarshal full payload: %v", err)
	}

	out, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var original, reencoded map[string]any
	if err := json.Unmarshal([]byte(fullDiagnosticsPayload), &original); err != nil {
		t.Fatalf("unmarshal original into map: %v", err)
	}
	if err := json.Unmarshal(out, &reencoded); err != nil {
		t.Fatalf("unmarshal re-encoded into map: %v", err)
	}
	if !reflect.DeepEqual(original, reencoded) {
		t.Fatalf("round trip changed the payload:\n got %v\nwant %v", reencoded, original)
	}

	var again DiagnosticsResponse
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("unmarshal re-encoded payload: %v", err)
	}
	if !reflect.DeepEqual(again, decoded) {
		t.Fatalf("round trip changed the struct:\n got %#v\nwant %#v", again, decoded)
	}
}

// TestDiagnosticsResponseOptionalFieldsAbsent covers older or differently
// provisioned firmware, which omits the optional objects entirely. Decoding
// must succeed and every absent field must stay nil, so a consumer can tell
// "the node did not report this" from "the node reported zero".
func TestDiagnosticsResponseOptionalFieldsAbsent(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		check   func(t *testing.T, d DiagnosticsResponse)
	}{
		{
			name:    "required fields only",
			payload: `{"uptime_s":10,"free_heap":1000,"heap":{"internal_free":900,"internal_min_ever_free":800,"internal_largest_free_block":700,"dma_free":600,"dma_largest_free_block":500,"psram_free":0,"psram_min_ever_free":0},"task_stack_hwm":[]}`,
			check: func(t *testing.T, d DiagnosticsResponse) {
				t.Helper()
				if d.UptimeS != 10 || d.FreeHeap != 1000 {
					t.Fatalf("required fields lost: %+v", d)
				}
				if d.Backpressure != nil {
					t.Fatalf("Backpressure must be nil when absent, got %+v", *d.Backpressure)
				}
				if d.RadioHealth != nil {
					t.Fatalf("RadioHealth must be nil when absent, got %+v", *d.RadioHealth)
				}
				for name, got := range map[string]*float64{
					"GPSRxBytes":     d.GPSRxBytes,
					"GPSRxLines":     d.GPSRxLines,
					"GPSRxOverruns":  d.GPSRxOverruns,
					"GPSRxErrors":    d.GPSRxErrors,
					"GPSRxDisabled":  d.GPSRxDisabled,
					"GPSRxRearmFail": d.GPSRxRearmFail,
				} {
					if got != nil {
						t.Fatalf("%s must be nil when absent, got %v", name, *got)
					}
				}
				if d.GPSChip != nil {
					t.Fatalf("GPSChip must be nil when absent, got %q", *d.GPSChip)
				}
			},
		},
		{
			name:    "gps counters present and zero",
			payload: `{"uptime_s":10,"free_heap":1000,"heap":{},"task_stack_hwm":[],"gps_rx_bytes":0,"gps_rx_lines":0,"gps_chip":""}`,
			check: func(t *testing.T, d DiagnosticsResponse) {
				t.Helper()
				// A present zero is the dead-UART signal and must not be
				// indistinguishable from the absent case above.
				if d.GPSRxBytes == nil || *d.GPSRxBytes != 0 {
					t.Fatalf("GPSRxBytes must decode as a present zero, got %v", d.GPSRxBytes)
				}
				if d.GPSRxLines == nil || *d.GPSRxLines != 0 {
					t.Fatalf("GPSRxLines must decode as a present zero, got %v", d.GPSRxLines)
				}
				// An empty gps_chip means the board has GPS but has not seen a
				// banner, which is not the same as having no GPS at all.
				if d.GPSChip == nil || *d.GPSChip != "" {
					t.Fatalf("GPSChip must decode as a present empty string, got %v", d.GPSChip)
				}
			},
		},
		{
			name:    "radio health unsupported",
			payload: `{"uptime_s":10,"free_heap":1000,"heap":{},"task_stack_hwm":[],"radio_health":{"supported":false,"tx_power_dbm":17}}`,
			check: func(t *testing.T, d DiagnosticsResponse) {
				t.Helper()
				if d.RadioHealth == nil {
					t.Fatal("RadioHealth must be present")
				}
				if d.RadioHealth.Supported {
					t.Fatal("Supported must decode as false")
				}
				if d.RadioHealth.TxPowerDBm != 17 {
					t.Fatalf("TxPowerDBm = %d, want 17", d.RadioHealth.TxPowerDBm)
				}
				// Nothing else is populated when the driver has no SX1262 to
				// interrogate. OCPOK in particular must stay nil rather than
				// decoding to false, which would read as a PA config fault.
				if d.RadioHealth.OCPOK != nil {
					t.Fatalf("OCPOK must be nil when unsupported, got %v", *d.RadioHealth.OCPOK)
				}
				if d.RadioHealth.PARampError != nil {
					t.Fatalf("PARampError must be nil when unsupported, got %v", *d.RadioHealth.PARampError)
				}
				if d.RadioHealth.DeviceErrors != nil {
					t.Fatalf("DeviceErrors must be nil when unsupported, got %v", *d.RadioHealth.DeviceErrors)
				}
				if d.RadioHealth.ChipMode != nil {
					t.Fatalf("ChipMode must be nil when unsupported, got %q", *d.RadioHealth.ChipMode)
				}
			},
		},
		{
			name:    "radio health supported with faults",
			payload: `{"uptime_s":10,"free_heap":1000,"heap":{},"task_stack_hwm":[],"radio_health":{"supported":true,"tx_power_dbm":22,"device_errors":1,"device_errors_str":"PA_RAMP","pa_ramp_error":true,"ocp":0,"ocp_expected":56,"ocp_ok":false}}`,
			check: func(t *testing.T, d DiagnosticsResponse) {
				t.Helper()
				rh := d.RadioHealth
				if rh == nil {
					t.Fatal("RadioHealth must be present")
				}
				if rh.PARampError == nil || !*rh.PARampError {
					t.Fatalf("PARampError must decode as a present true, got %v", rh.PARampError)
				}
				// A present false is a fault report, distinct from nil.
				if rh.OCPOK == nil || *rh.OCPOK {
					t.Fatalf("OCPOK must decode as a present false, got %v", rh.OCPOK)
				}
				if rh.OCP == nil || *rh.OCP != 0 {
					t.Fatalf("OCP must decode as a present zero, got %v", rh.OCP)
				}
				if rh.DeviceErrorsStr == nil || *rh.DeviceErrorsStr != "PA_RAMP" {
					t.Fatalf("DeviceErrorsStr = %v, want PA_RAMP", rh.DeviceErrorsStr)
				}
			},
		},
		{
			name:    "backpressure present with zero counters",
			payload: `{"uptime_s":10,"free_heap":1000,"heap":{},"task_stack_hwm":[],"backpressure":{"flood_relay_drops":0,"probe_ingress":{"accepted":0,"dropped_reply":0,"dropped_forward":0}}}`,
			check: func(t *testing.T, d DiagnosticsResponse) {
				t.Helper()
				if d.Backpressure == nil {
					t.Fatal("Backpressure must be present even when every counter is zero")
				}
				if d.Backpressure.FloodRelayDrops != 0 {
					t.Fatalf("FloodRelayDrops = %v, want 0", d.Backpressure.FloodRelayDrops)
				}
				if d.Backpressure.ProbeIngress.DroppedForward != 0 {
					t.Fatalf("DroppedForward = %v, want 0", d.Backpressure.ProbeIngress.DroppedForward)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d DiagnosticsResponse
			if err := json.Unmarshal([]byte(tt.payload), &d); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			tt.check(t, d)
		})
	}
}

// TestDiagnosticsResponseOmitsAbsentOptionalFields checks the marshal side: a
// response that never carried the optional fields must not gain them back as
// zero values when it is re-serialized.
func TestDiagnosticsResponseOmitsAbsentOptionalFields(t *testing.T) {
	out, err := json.Marshal(DiagnosticsResponse{UptimeS: 1})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(out, &keys); err != nil {
		t.Fatalf("unmarshal marshalled output: %v", err)
	}

	for _, key := range []string{
		"backpressure", "radio_health", "gps_rx_bytes", "gps_rx_lines",
		"gps_chip", "gps_rx_overruns", "gps_rx_errors", "gps_rx_disabled",
		"gps_rx_rearm_fail",
	} {
		if _, ok := keys[key]; ok {
			t.Fatalf("absent optional field %q must be omitted, got %s", key, out)
		}
	}
}

// TestTrafficEventSrcAddr covers the origin address, which the firmware sends
// only for RX frames whose packet type actually carries one. A consumer
// plotting per-peer signal strength has to be able to tell "no origin" from a
// real address, and "00000000" is a real address.
func TestTrafficEventSrcAddr(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantSrcAddr string
		wantIsTx    bool
	}{
		{
			name:        "rx event with origin address",
			payload:     `{"seq":11,"timestamp_ms":9000,"pkt_type":3,"category":"chat","airtime_tier":"normal","packet_len":48,"rssi":-72,"is_tx":false,"src_addr":"A1B2C3D4"}`,
			wantSrcAddr: "A1B2C3D4",
		},
		{
			name:        "rx event whose packet type carries no origin",
			payload:     `{"seq":12,"timestamp_ms":9100,"pkt_type":9,"category":"maintenance","airtime_tier":"none","packet_len":16,"rssi":-90,"is_tx":false}`,
			wantSrcAddr: "",
		},
		{
			name:        "tx event never carries an origin",
			payload:     `{"seq":13,"timestamp_ms":9200,"pkt_type":3,"category":"chat","airtime_tier":"normal","packet_len":48,"rssi":0,"is_tx":true}`,
			wantSrcAddr: "",
			wantIsTx:    true,
		},
		{
			name:        "all-zero address is a real address, not an absence",
			payload:     `{"seq":14,"timestamp_ms":9300,"pkt_type":3,"category":"chat","airtime_tier":"normal","packet_len":48,"rssi":-65,"is_tx":false,"src_addr":"00000000"}`,
			wantSrcAddr: "00000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e TrafficEvent
			if err := json.Unmarshal([]byte(tt.payload), &e); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if e.SrcAddr != tt.wantSrcAddr {
				t.Fatalf("SrcAddr = %q, want %q", e.SrcAddr, tt.wantSrcAddr)
			}
			if e.IsTx != tt.wantIsTx {
				t.Fatalf("IsTx = %t, want %t", e.IsTx, tt.wantIsTx)
			}

			// The wire form is 8 hex digits and can never be empty, so an
			// empty SrcAddr survives a re-serialize as an absent key rather
			// than as an address-shaped zero value.
			out, err := json.Marshal(e)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var keys map[string]json.RawMessage
			if err := json.Unmarshal(out, &keys); err != nil {
				t.Fatalf("unmarshal marshalled output: %v", err)
			}
			_, present := keys["src_addr"]
			if present != (tt.wantSrcAddr != "") {
				t.Fatalf("src_addr present = %t, want %t; got %s", present, tt.wantSrcAddr != "", out)
			}
		})
	}
}
