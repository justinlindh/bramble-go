# bramble-go API Reference

Full method and callback reference for `bramble-go`.

## Table of Contents

- [Method Conventions](#method-conventions)
- [Query Methods](#query-methods)
  - [Query Usage Examples](#query-usage-examples)
- [Action Methods](#action-methods)
  - [Action and Debug Usage Examples](#action-and-debug-usage-examples)
- [Notification Callbacks](#notification-callbacks)
- [Key Types](#key-types)
- [Action Messages (`/me`)](#action-messages-me)
- [BLE Transport Details](#ble-transport-details)

## Method Conventions

All methods accept a `context.Context` for timeout/cancellation.

## Query Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `Status(ctx)` | `*StatusResponse` | Address, firmware, peers, counters, uptime |
| `WifiStatus(ctx)` | `*WifiStatus` | Wi-Fi mode/link/AP client status |
| `Diagnostics(ctx, includeHeapDump)` | `*DiagnosticsResponse` | Runtime heap and task stack diagnostics |
| `Identity(ctx)` | `*IdentityResponse` | Address + public key hash |
| `Version(ctx)` | `*VersionResponse` | Firmware/protocol version, hardware |
| `DeliveryEvents(ctx, sinceEventSeq, limit)` | `*DeliveryReplayResponse` | Replay persisted delivery telemetry events |
| `Neighbors(ctx)` | `[]Neighbor` | Direct radio neighbors (RSSI, SNR, last heard) |
| `Routes(ctx)` | `[]Route` | Routing table entries |
| `DmSessions(ctx)` | `*DmSessionsResponse` | DM session table: which peers a directed send can reach |
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
| `Screenshot(ctx)` | `*Screenshot` | Capture the display and reassemble the paged framebuffer |

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

// Which peers can this node actually send a DM (or a per-contact location
// share) to right now? A configured peer with no active session is silently
// unreachable for directed traffic.
dm, _ := client.DmSessions(ctx)
for _, s := range dm.Sessions {
    fmt.Printf("%s active=%v verified=%v\n", s.Address, s.Active(), s.Verified)
}

// Capture the display. Screenshot issues the capture and pages the whole
// framebuffer back; Pixels is the raw frame, not an encoded image.
shot, _ := client.Screenshot(ctx)
fmt.Printf("%dx%d %s, %d bytes\n", shot.Width, shot.Height, shot.Format, len(shot.Pixels))
```

### Screenshot Pixel Format

`Screenshot.Pixels` is the framebuffer exactly as the device holds it. For
`Format` `"rgb565"` that is two bytes per pixel, row major, and **little
endian**: the low byte of each pixel comes first.

```go
// Correct: little endian.
v := uint16(shot.Pixels[i]) | uint16(shot.Pixels[i+1])<<8
r := uint8((v>>11)&0x1F) << 3
g := uint8((v>>5)&0x3F) << 2
b := uint8(v&0x1F) << 3
```

Reading those two bytes big endian produces an image with recognisable layout
and wrong colours (a pink or red cast), which looks like a display or capture
fault and is not one.

### Raw RPC

Every typed method is a thin wrapper over `Call`, which is exported so a caller
is never blocked on this SDK catching up with the firmware:

```go
raw, err := client.Call(ctx, "bramble.someNewMethod", map[string]any{"arg": 1})
```

Prefer the typed methods where they exist; they pin the response shape.

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

// Audio controls
_ = client.PlayTone(ctx, "startup")
_ = client.SetVolume(ctx, 75)
_ = client.SetMuted(ctx, false)
```

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
})

client.OnDecodeError(func(method string, err error, payload []byte) {
    log.Printf("decode error on %s: %v (raw: %s)\n", method, err, payload)
})
```

## Key Types

The canonical type definitions live in [`types.go`](../types.go) and are kept in sync with firmware wire fields.

Notably, `StatusResponse` and `ConfigResponse` include additional fields beyond older snippets (for example `SupportsDeliveryEventSync` and `Location`) — refer to source for the current schema.

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
- Pairing/bonding behavior is OS-level; complete pairing first if your platform requires it.
- BLE throughput/latency is lower than serial/WebSocket; set realistic RPC deadlines.

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
