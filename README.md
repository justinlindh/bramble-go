# bramble-go

Go SDK for [Bramble](https://github.com/justinlindh/bramble) LoRa mesh nodes. Communicates via JSON-RPC 2.0 over Serial, WebSocket, or BLE (Nordic UART Service).

## Install

```bash
go get github.com/justinlindh/bramble-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    bramble "github.com/justinlindh/bramble-go"
    "github.com/justinlindh/bramble-go/transport"
)

func main() {
    t := transport.NewWebSocket("ws://192.168.4.1/ws")
    client := bramble.NewClient(t)

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    status, err := client.Status(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Node %s — %d peers, uptime %s\n",
        status.Address, status.Peers,
        time.Duration(status.UptimeSec)*time.Second)

    neighbors, err := client.Neighbors(ctx)
    if err != nil {
        log.Fatal(err)
    }
    for _, n := range neighbors {
        fmt.Printf("  %s  RSSI=%d  SNR=%.1f\n", n.Address, n.RSSI, n.SNR)
    }
}
```

See [`examples/`](examples/) for more.

## Transports

| Transport | Status | Constructor |
|-----------|--------|-------------|
| Serial (UART) | ✅ Ready | `transport.NewSerial("/dev/ttyUSB0")` |
| WebSocket | ✅ Ready | `transport.NewWebSocket("ws://192.168.4.1/ws")` |
| BLE (NUS) | ✅ Ready | `transport.NewBLE(transport.BLEConfig{...})` |

### WebSocket Auto-Reconnect

The WebSocket transport automatically reconnects on unexpected disconnects using exponential backoff (1s → 2s → 4s → … → 30s max). Set callbacks to be notified:

```go
ws := transport.NewWebSocket("ws://192.168.4.1/ws")
ws.OnDisconnect = func() { log.Println("disconnected") }
ws.OnReconnect = func() { log.Println("reconnected!") }
```

During reconnection, `Send()` returns `transport.ErrReconnecting`.

### BLE Transport (Nordic UART Service)

BLE support is implemented via NUS (Nordic UART Service), using newline-delimited JSON-RPC over BLE notifications/writes.

Constructor and config:

```go
ble := transport.NewBLE(transport.BLEConfig{
    DeviceName:  "Bramble",        // optional; empty means auto-scan for first matching NUS device
    ScanTimeout: 15 * time.Second,  // optional; default is 10s
})
client := bramble.NewClient(ble)
```

Practical notes:

- **Platform support depends on your host BLE stack** (`tinygo.org/x/bluetooth` backend).
- On Linux, ensure Bluetooth is enabled and your user has permissions to access BLE (commonly via BlueZ/dbus setup).
- If `DeviceName` is set, scan matching is case-insensitive substring matching.
- If `DeviceName` is empty, transport scans for the first device advertising the Bramble NUS service.
- Pairing/bonding behavior is OS-level; complete pairing first if your platform requires it.
- BLE throughput/latency is lower than Wi-Fi serial/WebSocket; keep RPC deadlines realistic.

Minimal BLE connect example:

```go
ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()

ble := transport.NewBLE(transport.BLEConfig{DeviceName: "Bramble"})
client := bramble.NewClient(ble)

if err := client.Connect(ctx); err != nil {
    log.Fatalf("BLE connect failed: %v", err)
}
defer client.Close()
```

## API Reference

All methods accept a `context.Context` for timeout/cancellation.

### Query Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `Status(ctx)` | `*StatusResponse` | Address, firmware, peers, counters, uptime |
| `GetWifiStatus(ctx)` | `*WifiStatus` | Wi-Fi mode/link/AP client status |
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
| `GetTrafficDebug(ctx)` | `*GetTrafficDebugResponse` | Current traffic debug config and ring-buffer state |
| `GetTrafficEvents(ctx, params)` | `*GetTrafficEventsResponse` | Pull traffic debug events from ring buffer |

#### Query Usage Examples

```go
wifi, _ := client.GetWifiStatus(ctx)
fmt.Printf("wifi mode=%s ssid=%s ip=%s\n", wifi.Mode, wifi.SSID, wifi.IP)

replay, _ := client.DeliveryEvents(ctx, 0, 100)
fmt.Printf("delivery events replayed=%d\n", len(replay.Events))
```

### Action Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `Send(ctx, dest, text)` | `*SendResult` | Send unicast message |
| `SendCritical(ctx, dest, text)` | `*SendResult` | Send critical-priority unicast message |
| `SendBroadcast(ctx, text)` | `*SendResult` | Broadcast on the public channel |
| `SendBroadcastCritical(ctx, text)` | `*SendResult` | Critical-priority broadcast on the public channel |
| `Broadcast(ctx, text)` | `*SendResult` | Deprecated alias for `SendBroadcast` |
| `BroadcastOnChannel(ctx, channel, text)` | `*SendResult` | Broadcast on a specific channel index |
| `BroadcastOnChannelCritical(ctx, channel, text)` | `*SendResult` | Critical-priority broadcast on a specific channel |
| `SendProbe(ctx)` | `*SendProbeResult` | Network reachability probe |
| `SetRadio(ctx, config)` | `error` | Update radio parameters |
| `SetNodeName(ctx, name)` | `error` | Set node display name (max 32 chars) |
| `AddChannel(ctx, name, psk)` | `*AddChannelResult` | Add a channel |
| `RemoveChannel(ctx, index)` | `error` | Remove a channel by index |
| `SetDefaultChannel(ctx, index)` | `error` | Set default outgoing channel |
| `SetMailbox(ctx, enabled)` | `error` | Toggle store-and-forward |
| `SetLocationConfig(ctx, config)` | `error` | Update GPS settings |
| `SetLocationContact(ctx, addr, tier)` | `error` | Add/update location contact |
| `RemoveLocationContact(ctx, addr)` | `error` | Stop sharing location |
| `ShareLocationOnce(ctx, addr)` | `error` | One-shot location share |
| `Reboot(ctx)` | `error` | Reboot node |
| `OTAUpdate(ctx, url)` | `*OTAUpdateResponse` | Trigger OTA update from firmware URL |
| `SetTrafficDebug(ctx, params)` | `*SetTrafficDebugResponse` | Configure traffic debug telemetry capture |

#### Action/Debug Usage Examples

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
state, _ := client.GetTrafficDebug(ctx)
fmt.Println("traffic debug enabled:", state.Enabled)
events, _ := client.GetTrafficEvents(ctx, bramble.GetTrafficEventsParams{})
fmt.Println("events:", events.Returned)
```

### Notification Callbacks

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
| `OnGpsEvent(fn)` | `func(GpsEvent)` | GPS state/position notifications |
| `OnLocationEvent(fn)` | `func(LocationEvent)` | Location sharing event notifications |

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
```

### Key Types

```go
type StatusResponse struct {
    Address, FirmwareVersion, ProtocolVersion, Hardware string
    RadioOk bool
    Peers, BeaconTx, BeaconRx, PacketsTx, PacketsRx, UptimeSec int
}

type Neighbor struct {
    Address string; RSSI int; SNR float64; LastSeenAgoMs int64
}

type SendResult struct {
    MessageID      string
    PacketID       string
    BroadcastID    string
    Status         string
    Fragmented     bool
    FragmentsTotal int
    MaxBytes       int
    ActualBytes    int
    Broadcast      bool
    Channel        int
}

type ConfigResponse struct {
    NodeName, Address string; Radio ConfigRadio; Channels []Channel
}
```

### Action Messages (`/me`)

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

## Protocol Compatibility

This SDK negotiates protocol versions on connect. See [VERSIONING.md](https://github.com/justinlindh/bramble/src/branch/main/VERSIONING.md) for the compatibility matrix.

## Releases (semantic-release)

Releases are automated from `main` using semantic-release and Conventional Commits.

### Required Gitea Actions secrets

- `GITEA_TOKEN` (**required**): Personal Access Token with repository write/release permissions.
- `GITEA_URL` (optional): Base URL of the Gitea instance. Defaults to `https://github.com`.

### Commit format

Use Conventional Commits so semantic-release can determine the next version:

- `feat:` → minor release (`vX.Y.0`)
- `fix:` → patch release (`vX.Y.Z`)
- `feat!:` or footer `BREAKING CHANGE:` → major release (`vX.0.0`)

Examples:

- `feat(client): add message retry policy`
- `fix(ws): handle reconnect race`
- `feat(api)!: rename Status field`

## License

MIT — see [LICENSE](LICENSE)
