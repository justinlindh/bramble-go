# bramble-go API Reference

Full method and callback reference for `bramble-go`.

## Table of Contents

- [Method Conventions](#method-conventions)
- [Query Methods](#query-methods)
  - [Query Usage Examples](#query-usage-examples)
- [Action Methods](#action-methods)
  - [Action and Debug Usage Examples](#action-and-debug-usage-examples)
- [Provisioning](#provisioning)
- [Notification Callbacks](#notification-callbacks)
- [Key Types](#key-types)
  - [Optional Fields](#optional-fields)
- [Action Messages (`/me`)](#action-messages-me)
- [BLE Transport Details](#ble-transport-details)

## Method Conventions

All methods accept a `context.Context` for timeout/cancellation.

## Query Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `Status(ctx)` | `*StatusResponse` | Address, firmware, peers, counters, uptime |
| `WifiStatus(ctx)` | `*WifiStatus` | Wi-Fi mode/link/AP client status |
| `Diagnostics(ctx, includeHeapDump)` | `*DiagnosticsResponse` | Runtime heap, task stack, airtime backpressure, radio health, and GNSS feed diagnostics |
| `Identity(ctx)` | `*IdentityResponse` | Address + public key hash |
| `Version(ctx)` | `*VersionResponse` | Firmware/protocol version, hardware |
| `DeliveryEvents(ctx, sinceEventSeq, limit)` | `*DeliveryReplayResponse` | Replay persisted delivery telemetry events |
| `Neighbors(ctx)` | `[]Neighbor` | Direct radio neighbors (RSSI, SNR, last heard) |
| `Routes(ctx)` | `[]Route` | Routing table entries |
| `Airtime(ctx)` | `*AirtimeStats` | Per-tier airtime budget usage |
| `Ping(ctx)` | `error` | Health check (returns nil on success) |
| `Messages(ctx)` | `[]Message` | Stored message history |
| `PeerLocations(ctx)` | `[]LocationPeer` | Peer location data |
| `Config(ctx)` | `*ConfigResponse` | Full node config (name, address, radio, channels) |
| `TrafficDebug(ctx)` | `*TrafficDebugResponse` | Current traffic debug config and ring-buffer state |
| `TrafficEvents(ctx, params)` | `*TrafficEventsResponse` | Pull traffic debug events from ring buffer |
| `Battery(ctx)` | `*BatteryStatus` | Battery voltage (mV) and charge percentage |
| `GPSPosition(ctx)` | `*GPSPosition` | Current GPS fix (lat/lon/alt/speed/heading/accuracy) |
| `BeaconPolicy(ctx)` | `*BeaconPolicyResponse` | Adaptive beacon policy config and runtime status |
| `AudioStatus(ctx)` | `*AudioStatus` | Audio availability, volume, mute, and playback state |
| `StorageInfo(ctx)` | `*StorageInfo` | Board storage status (SD card presence and mount point) |
| `AuthToken(ctx)` | `(string, error)` | Retrieve the device's WebSocket auth token (typically over serial) |
| `NetworkKeyStatus(ctx)` | `*NetworkKeyStatusResponse` | Whether the node holds a network key, and that key's one-way fingerprint |
| `AnchorStatus(ctx)` | `*AnchorStatusResponse` | Whether a fleet trust anchor is provisioned, and whether this node is endorsed |
| `BleSecurity(ctx)` | `*BleSecurity` | BLE pairing mode and whether a static passkey is stored (the passkey itself is never returned) |

### Query Usage Examples

```go
wifi, _ := client.WifiStatus(ctx)
fmt.Printf("wifi mode=%s ssid=%s ip=%s\n", wifi.Mode, wifi.SSID, wifi.IP)

replay, _ := client.DeliveryEvents(ctx, 0, 100)
fmt.Printf("delivery events replayed=%d\n", len(replay.Events))

bat, _ := client.Battery(ctx)
fmt.Printf("battery %dmV (%d%%)\n", bat.VoltageMV, bat.Percentage)

pos, _ := client.GPSPosition(ctx)
if pos.Valid {
    fmt.Printf("GPS lat=%.6f lon=%.6f alt=%.1fm\n", pos.Lat, pos.Lon, pos.Alt)
}

beacon, _ := client.BeaconPolicy(ctx)
fmt.Printf("beacon mode=%s interval=%dms\n", beacon.Status.ActiveMode, beacon.Status.CurrentIntervalMs)

audio, _ := client.AudioStatus(ctx)
fmt.Printf("audio available=%v volume=%d muted=%v\n", audio.Available, audio.Volume, audio.Muted)

storage, _ := client.StorageInfo(ctx)
fmt.Printf("SD present=%v mount=%s\n", storage.SDPresent, storage.MountPoint)

// Retrieve auth token over serial before connecting via WebSocket
token, _ := client.AuthToken(ctx)
fmt.Println("auth token:", token)
```

## Action Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `Send(ctx, dest, text)` | `*SendResult` | Send unicast message |
| `SendCritical(ctx, dest, text)` | `*SendResult` | Send critical-priority unicast message |
| `SendBroadcast(ctx, text)` | `*SendResult` | Broadcast on the public channel |
| `SendBroadcastCritical(ctx, text)` | `*SendResult` | Critical-priority broadcast on the public channel |
| `BroadcastOnChannel(ctx, channel, text)` | `*SendResult` | Broadcast on a specific channel index |
| `BroadcastOnChannelCritical(ctx, channel, text)` | `*SendResult` | Critical-priority broadcast on a specific channel |
| `SendProbe(ctx)` | `*SendProbeResult` | Network reachability probe |
| `SetRadio(ctx, config)` | `error` | Update radio parameters |
| `SetNodeName(ctx, name)` | `error` | Set node display name (max 32 chars) |
| `SetWifiConfig(ctx, ssid, password)` | `*SetWifiConfigResponse` | Provision Wi-Fi station credentials (empty password = open network) |
| `AddChannel(ctx, name, psk)` | `*AddChannelResult` | Add a channel |
| `RemoveChannel(ctx, index)` | `error` | Remove a channel by index |
| `SetDefaultChannel(ctx, index)` | `error` | Set default outgoing channel |
| `SetMailbox(ctx, enabled)` | `error` | Toggle store-and-forward |
| `SetLocationConfig(ctx, config)` | `error` | Update location sharing configuration |
| `SetLocationContact(ctx, addr, tier)` | `error` | Add/update location contact |
| `RemoveLocationContact(ctx, addr)` | `error` | Stop sharing location |
| `ShareLocationOnce(ctx, addr)` | `error` | One-shot location share |
| `Reboot(ctx)` | `error` | Reboot node |
| `OTAUpdate(ctx, url)` | `*OTAUpdateResponse` | Trigger OTA update from firmware URL |
| `SetTrafficDebug(ctx, params)` | `*SetTrafficDebugResponse` | Configure traffic debug telemetry capture |
| `SetBeaconPolicy(ctx, params)` | `error` | Update adaptive beaconing policy settings |
| `SetAuthToken(ctx, token)` | `error` | Set or clear the device's WebSocket auth token |
| `SetNetworkKey(ctx, keyHex)` | `error` | Provision the control-plane network key, joining the node to that network |
| `GenerateNetworkKey(ctx)` | `*GenerateNetworkKeyResponse` | Mint a key on the node, provision it, and return it once (founds a network) |
| `SetAnchor(ctx, anchorPubHex)` | `error` | Provision the fleet trust anchor's public key |
| `SetEndorsement(ctx, notAfterHex, sigHex)` | `error` | Apply an anchor-signed endorsement cert to the node |
| `SetBlePasskey(ctx, passkey)` | `*SetBlePasskeyResponse` | Set the 6-digit static BLE pairing passkey, or clear it with an empty string; wipes all BLE bonds |
| `SetBroadcastTelemetryMode(ctx, mode)` | `*SetBroadcastTelemetryModeResponse` | Update broadcast telemetry mode |
| `SetBacklight(ctx, level)` | `*BacklightResponse` | Set display backlight level (0–255) |
| `Sleep(ctx, wakeAfterS)` | `*SleepResponse` | Enter deep sleep; wake after `wakeAfterS` seconds (0 = no timer) |
| `PlayTone(ctx, tone)` | `error` | Play a predefined audio tone by name |
| `SetVolume(ctx, volume)` | `error` | Set output volume (0–100) |
| `SetMuted(ctx, muted)` | `error` | Mute or unmute audio output |

### Action and Debug Usage Examples

```go
// Critical unicast
_, _ = client.SendCritical(ctx, 0xAABBCCDD, "priority ping")

// Channel-targeted broadcast (channel index 2)
_, _ = client.BroadcastOnChannel(ctx, 2, "ops update")

// OTA update
ota, _ := client.OTAUpdate(ctx, "https://example.com/bramble.bin")
fmt.Println("ota scheduled:", ota.OK)

// Traffic debug pull flow
enabled := true
sample := 100
_, _ = client.SetTrafficDebug(ctx, bramble.SetTrafficDebugParams{Enabled: &enabled, SampleRate: &sample})
state, _ := client.TrafficDebug(ctx)
fmt.Println("traffic debug enabled:", state.Enabled)
events, _ := client.TrafficEvents(ctx, bramble.TrafficEventsParams{})
fmt.Println("events:", events.Returned)

// Beacon policy: read then update
bp, _ := client.BeaconPolicy(ctx)
fmt.Println("beacon active mode:", bp.Status.ActiveMode)

// Wi-Fi provisioning: persist credentials, then reboot to apply
cfg, _ := client.SetWifiConfig(ctx, "my-network", "hunter22")
if cfg.Applied == "reboot_required" {
    _ = client.Reboot(ctx)
}
enabled, base := true, 5000
_ = client.SetBeaconPolicy(ctx, bramble.SetBeaconPolicyParams{Enabled: &enabled, BaseIntervalMs: &base})

// Auth token round-trip (read over serial, apply to WS client)
token, _ := client.AuthToken(ctx)
_ = client.SetAuthToken(ctx, token)

// Broadcast telemetry mode
result, _ := client.SetBroadcastTelemetryMode(ctx, "full")
fmt.Println("telemetry mode:", result.BroadcastTelemetryMode)

// Backlight and sleep
bl, _ := client.SetBacklight(ctx, 128)
fmt.Println("backlight level:", bl.Level)
sr, _ := client.Sleep(ctx, 300) // wake after 5 min
fmt.Println("sleep scheduled:", sr.OK)

// BLE pairing security: read the posture, then set a static passkey on a
// node without a display. A refusal arrives as ok:false in a successful
// call, so check resp.OK rather than only the error.
sec, _ := client.BleSecurity(ctx)
fmt.Printf("ble mode=%s static passkey set=%v\n", sec.Mode, sec.StaticPasskeySet)
if sec.Mode != bramble.BleSecurityModePasskeyDisplay {
    pk, err := client.SetBlePasskey(ctx, "314159") // "" clears it
    if err != nil {
        log.Fatalf("setBlePasskey call failed: %v", err)
    }
    if !pk.OK {
        log.Fatalf("node refused the passkey: %s", pk.Error)
    }
    fmt.Println("ble mode now:", pk.Mode) // all existing bonds were wiped
}

// Audio controls
_ = client.PlayTone(ctx, "startup")
_ = client.SetVolume(ctx, 75)
_ = client.SetMuted(ctx, false)
```

## Provisioning

A node ships with no network key and is **inert**: it neither emits nor accepts authenticated control-plane traffic, so it does not mesh until a key is provisioned. Provisioning is therefore the first thing you do to a new node, before anything else in this API is useful on the radio.

There are two paths, and they differ only in where the key comes from. To **found** a network, call `GenerateNetworkKey` on the first node: it mints an entropy-gated key on the device, provisions itself atomically, and returns the raw key exactly once. To **join** an existing network, pass that same key to `SetNetworkKey` on every other node.

The key is write-only at the device boundary. No API reads a provisioned key back, so the copy `GenerateNetworkKey` returns is the only copy that will ever exist: record it out of band before you rely on it, and never log it. What you can read back is `NetworkKeyStatus`, which reports a one-way fingerprint (`SHA256(key)[0:4]`). Nodes showing the same fingerprint hold the same key, which is how you confirm a fleet converged without moving the secret again.

These helpers need no device and are safe to use offline:

| Helper | Returns | Description |
|--------|---------|-------------|
| `GenerateNetworkKeySeed()` | `([]byte, error)` | Mint a 32-byte key on this host (prefer `GenerateNetworkKey` on the node) |
| `NetworkKeyFingerprint(key)` | `string` | `SHA256(key)[0:4]` as 8 lowercase hex, matching what every node reports |
| `EncodeNetworkKeyShare(key)` | `(string, error)` | Encode a key as the `bramble://net/v1?k=` share string the webapp QR emits |
| `ParseNetworkKeyShare(s)` | `([]byte, error)` | Parse that share string, or a bare 64-hex key |

```go
// Found a network on the first node.
gen, err := client.GenerateNetworkKey(ctx)
if err != nil {
    log.Fatalf("found network: %v", err)
}
// gen.Key is the only copy. Record it out of band right here.
fmt.Println("fingerprint:", gen.Fingerprint)

// Join every other node to it.
if err := other.SetNetworkKey(ctx, gen.Key); err != nil {
    log.Fatalf("join network: %v", err)
}

// Confirm convergence: same fingerprint means same key.
st, _ := other.NetworkKeyStatus(ctx)
if !st.Provisioned || st.Fingerprint != gen.Fingerprint {
    log.Fatalf("node did not converge: provisioned=%v fingerprint=%s", st.Provisioned, st.Fingerprint)
}
```

A network key admits members but does not stop a member from minting extra identities. To close that too, provision a fleet **trust anchor** (`SetAnchor`) and enroll each node with an anchor-signed cert (`SetEndorsement`); `AnchorStatus` reports both the anchor fingerprint and whether this node is endorsed. The anchor's private seed stays in your client and is never sent to a node. `GenerateAnchorSeed`, `AnchorPublicKey`, `AnchorFingerprint`, `SignEndorsementHex`, `EncodeAnchorBackup`, and `ParseAnchorBackup` are the offline half of that ceremony.

## Notification Callbacks

| Method | Callback Signature | Description |
|--------|--------------------|-------------|
| `OnMessage(fn)` | `func(Message)` | Incoming message notifications |
| `OnAck(fn)` | `func(Ack)` | Message ACK/delivery status notifications |
| `OnNeighborChange(fn)` | `func()` | Neighbor table change notification |
| `OnProbeResult(fn)` | `func(ProbeResult)` | Per-peer probe result notifications |
| `OnProbeComplete(fn)` | `func(ProbeComplete)` | Probe completion summary notification |
| `OnTrafficEvent(fn)` | `func(TrafficEvent)` | Real-time traffic debug event notifications |
| `OnBroadcastDelivery(fn)` | `func(BroadcastDelivery)` | Broadcast delivery telemetry notifications |
| `OnWifiEvent(fn)` | `func(WifiEvent)` | Wi-Fi state change notifications |
| `OnGPSEvent(fn)` | `func(GPSEvent)` | GPS state/position notifications |
| `OnLocationEvent(fn)` | `func(LocationEvent)` | Location sharing event notifications |
| `OnPeerLocation(fn)` | `func(PeerLocationEvent)` | Peer location cache update notifications (no payload; call `PeerLocations`) |
| `OnIdentityChange(fn)` | `func(IdentityChangeEvent)` | Node identity regeneration notifications (address collision) |
| `OnDecodeError(fn)` | `func(method string, err error, payload []byte)` | Called when a notification's JSON payload cannot be decoded |

```go
client.OnMessage(func(m bramble.Message) {
    fmt.Printf("From %s: %s\n", m.From, m.Text)
})

client.OnAck(func(a bramble.Ack) {
    fmt.Printf("Packet %s: %s\n", a.PacketID, a.Status)
})

client.OnProbeResult(func(p bramble.ProbeResult) {
    fmt.Printf("Probe %s reached %s in %dms\n", p.ProbeID, p.Address, p.LatencyMs)
})

client.OnTrafficEvent(func(e bramble.TrafficEvent) {
    fmt.Printf("Traffic seq=%d tx=%t len=%d category=%s\n", e.Seq, e.IsTx, e.PacketLen, e.Category)
    if e.SrcAddr != "" {
        fmt.Printf("  from %s at %d dBm\n", e.SrcAddr, e.RSSI)
    }
})

client.OnDecodeError(func(method string, err error, payload []byte) {
    log.Printf("decode error on %s: %v (raw: %s)\n", method, err, payload)
})
```

## Key Types

The canonical type definitions live in [`types.go`](../types.go) and are kept in sync with firmware wire fields.

Notably, `StatusResponse` and `ConfigResponse` include additional fields beyond older snippets (for example `SupportsDeliveryEventSync` and `Location`): refer to source for the current schema.

### Optional Fields

Some wire fields are optional: firmware omits them when the build has no such subsystem, and older firmware omits them entirely. Those fields are pointers, so an absent field stays distinguishable from a genuine zero reading. That distinction is the diagnostic value, so test for `nil` before dereferencing.

`DiagnosticsResponse` carries three optional groups:

- `Backpressure` (`*DiagnosticsBackpressure`): airtime backpressure counters. Non-zero values mean the node shed load rather than transmitting, which separates "we deliberately yielded the channel" from "the radio is broken". Its nested `ProbeIngress` holds the node-global inbound PROBE token-bucket accounting.
- `RadioHealth` (`*DiagnosticsRadioHealth`): what the radio reports about its own transmit path, as generic verdicts (`PAFault`, `PLLFault`, `OscillatorFault`, `CalibrationFault`, `ConfigVerified`) rather than one part's register layout, so they stay meaningful as other radios learn to answer them. `Supported` is false when the driver cannot interrogate its transmit path, and only `TxPowerDBm` is populated then; every other field stays nil. `ConfigVerified` false means config writes are not landing, which caps output well below the commanded level, and a present false there is a fault report, which is exactly why an absent field must not decode to false. `Detail` carries the chip-specific raw values as human-readable text: render it, never parse it, since the format is the driver's to choose and may change with the part.
- The GNSS feed counters (`GPSRxBytes`, `GPSRxLines`, `GPSChip`, `GPSRxOverruns`, `GPSRxErrors`, `GPSRxDisabled`, `GPSRxRearmFail`): present only on boards with GPS capability. A present `GPSRxBytes` of 0 with the driver running means the UART link is dead, whereas a nil `GPSRxBytes` means the board has no GPS to report on.

`TrafficEvent.SrcAddr` is the claimed origin address of an RX frame, as 8 uppercase hex digits. It is empty when the frame's packet type carries no origin address and on every TX event. The wire form can never be empty and an all-zero address is a real value rather than a sentinel, so compare against `""` to detect absence. The address is read from the unauthenticated wire prefix, so it is telemetry, not a verified identity; pairing it with `RSSI` is what makes per-peer signal strength measurable, since neighbour RSSI only refreshes on beacons.

```go
diag, _ := client.Diagnostics(ctx, false)

if diag.RadioHealth != nil && diag.RadioHealth.Supported {
    if diag.RadioHealth.ConfigVerified != nil && !*diag.RadioHealth.ConfigVerified {
        fmt.Println("configuration writes are not landing on the chip")
    }
    if diag.RadioHealth.PAFault != nil && *diag.RadioHealth.PAFault {
        fmt.Println("the PA did not ramp, so nothing usable went on air")
    }
    if diag.RadioHealth.Detail != nil {
        fmt.Println(*diag.RadioHealth.Detail) // display only, do not parse
    }
}

if diag.GPSRxBytes != nil && *diag.GPSRxBytes == 0 {
    fmt.Println("GNSS driver is running but no bytes have arrived")
}

if diag.Backpressure != nil && diag.Backpressure.FloodRelayDrops > 0 {
    fmt.Printf("shed %v flood relays\n", diag.Backpressure.FloodRelayDrops)
}
```

## Action Messages (`/me`)

The SDK supports IRC-style action messages using the CTCP ACTION convention:

```go
// Sending an action message
text := bramble.WrapAction("waves hello")
// text == "\x01ACTION waves hello\x01"
result, _ := client.SendBroadcast(ctx, text)

// Detecting action messages
msg := bramble.Message{Text: "\x01ACTION waves hello\x01"}
msg.IsAction()   // true
msg.ActionText() // "waves hello"
```

## BLE Transport Details

BLE support is implemented via NUS (Nordic UART Service), using newline-delimited JSON-RPC over BLE notifications and writes.

Constructor and options:

```go
// Device name is positional; empty string means auto-scan for the first NUS device.
ble := transport.NewBLE("Bramble",
    transport.WithBLEScanTimeout(15*time.Second), // optional; default is 10s
    transport.WithAuthToken("secret"),            // optional; only if firmware requires auth
)
client := bramble.NewClient(ble)
```

Practical notes:

- Platform support depends on your host BLE stack (`tinygo.org/x/bluetooth` backend).
- On Linux, ensure Bluetooth is enabled and your user has permission to access BLE (typically BlueZ/dbus configuration).
- If a device name is given, scan matching uses case-insensitive substring matching.
- If the device name is empty, the transport scans for the first device advertising the Bramble NUS service.
- The code the host asks for depends on the node's pairing mode, which `BleSecurity(ctx)` reports. A node with a display shows a random 6-digit code on its own screen for each attempt. A node without one uses the static passkey set through `SetBlePasskey(ctx, passkey)`, or pairs with no code at all (`just-works`) while none is set. Setting, changing, or clearing the passkey wipes the node's bonds, so every paired host pairs again.
- BLE throughput/latency is lower than serial/WebSocket; set realistic RPC deadlines.

### Pairing is required

The firmware declares its NUS TX/RX characteristics as encryption-required (`components/ble/ble_server.c`), so BlueZ refuses to write to or notify on them over an unencrypted link. The host must be paired and bonded with the target device before `Connect` is called; this SDK does not attempt to pair automatically (tinygo's BlueZ pairing support is not reliable enough to drive from Go).

Pair once per host/device pair with `bluetoothctl`:

```
bluetoothctl
  agent NoInputNoOutput
  default-agent
  scan on
  pair <MAC>
```

**Symptom of a missing or stale bond**: `Connect` succeeds (scanning and GATT discovery do not require encryption), but every `Send` silently vanishes: no error is returned and the device never responds. This happens because writes use write-without-response, and BlueZ drops the write instead of erroring when it declines to use an unencrypted link. If a device's bond is destroyed after the fact (firmware NVS erase, `bluetoothctl remove`, factory reset), you will see exactly this symptom until you re-pair.

Minimal BLE connect example:

```go
ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()

ble := transport.NewBLE("Bramble")
client := bramble.NewClient(ble)

if err := client.Connect(ctx); err != nil {
    log.Fatalf("BLE connect failed: %v", err)
}
defer client.Close()
```
