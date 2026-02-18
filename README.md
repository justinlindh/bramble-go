# bramble-go

Go SDK for [Bramble](https://github.com/justinlindh/bramble) mesh nodes.

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

    bramble "github.com/justinlindh/bramble-go"
    "github.com/justinlindh/bramble-go/transport"
)

func main() {
    // Connect via USB serial
    t := transport.NewSerial("/dev/ttyUSB0")
    client := bramble.NewClient(t)

    ctx := context.Background()
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Get node status
    status, err := client.Status(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Node: %s (%d peers)\n", status.Address, status.Peers)

    // Send a message
    result, err := client.Send(ctx, "6EEA8967", "hello from Go!")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Sent: %s\n", result.MessageID)
}
```

## Transports

| Transport | Status | Usage |
|-----------|--------|-------|
| Serial (UART) | ✅ Ready | `transport.NewSerial("/dev/ttyUSB0")` |
| WebSocket | ✅ Ready | `transport.NewWebSocket("ws://192.168.4.1/ws")` |
| BLE | 🔲 Stub | `transport.NewBLE()` (returns not-implemented) |

## API

All methods accept a `context.Context` for timeout/cancellation.

### Query Methods
- `Status(ctx)` — node status, uptime, peer count
- `Identity(ctx)` — address + public key hash
- `Version(ctx)` — firmware/protocol version
- `Neighbors(ctx)` — neighbor table with RSSI/SNR
- `Routes(ctx)` — routing table
- `Airtime(ctx)` — airtime budget stats
- `Ping(ctx)` — connectivity check
- `Messages(ctx)` — message history
- `PeerLocations(ctx)` — peer location data
- `Config(ctx)` — node configuration

### Action Methods
- `Send(ctx, dest, text)` — send DM
- `Broadcast(ctx, text)` — public channel message
- `SendProbe(ctx)` — network reachability probe
- `SetRadio(ctx, config)` — update radio settings
- `SetNodeName(ctx, name)` — rename node
- `AddChannel(ctx, name, psk)` / `RemoveChannel(ctx, id)` / `SetDefaultChannel(ctx, id)`
- `SetMailbox(ctx, enabled)` — store-and-forward toggle
- `SetLocationConfig(ctx, config)` — GPS settings
- `SetLocationContact(ctx, addr, tier)` / `RemoveLocationContact(ctx, addr)`
- `ShareLocationOnce(ctx, addr)` — one-shot location share
- `Reboot(ctx)` — restart node

### Notifications

```go
go func() {
    for notif := range client.Notifications() {
        switch notif.Method {
        case "bramble.onMessage":
            // handle incoming message
        case "bramble.onNeighborChange":
            // neighbor table updated
        }
    }
}()
```

## License

TBD — see [VERSIONING.md](https://github.com/justinlindh/bramble/src/branch/master/VERSIONING.md)
