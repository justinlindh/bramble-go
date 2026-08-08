# bramble-go

Go SDK for [Bramble](https://github.com/justinlindh/bramble) LoRa mesh nodes. It speaks JSON-RPC 2.0 over Serial, WebSocket, and BLE (Nordic UART Service), and provides typed query/action APIs plus event callbacks.

## Table of Contents

- [Overview](#overview)
- [Install](#install)
- [Quick Start](#quick-start)
- [Transports](#transports)
  - [WebSocket Auto-Reconnect](#websocket-auto-reconnect)
  - [BLE Notes](#ble-notes)
- [API Overview](#api-overview)
- [Protocol Compatibility](#protocol-compatibility)
- [CI Parity (Local Quality Contract)](#ci-parity-local-quality-contract)
- [Releases (semantic-release)](#releases-semantic-release)
- [License](#license)

## Overview

`bramble-go` is the Go client SDK for Bramble nodes. It handles transport framing, JSON-RPC request/response flow, and event notifications so applications can focus on mesh behavior rather than wire details.

For complete method tables, callback signatures, key type examples, and usage snippets, see [docs/API.md](docs/API.md).

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
| Serial (UART) | Ready | `transport.NewSerial("/dev/ttyUSB0")` |
| WebSocket | Ready | `transport.NewWebSocket("ws://192.168.4.1/ws")` |
| BLE (NUS) | Ready | `transport.NewBLE("Bramble")` |

### WebSocket Auto-Reconnect

The WebSocket transport reconnects automatically on unexpected disconnects using exponential backoff (1s up to 30s).

```go
ws := transport.NewWebSocket("ws://192.168.4.1/ws")
ws.OnDisconnect = func() { log.Println("disconnected") }
ws.OnReconnect = func() { log.Println("reconnected") }
```

During reconnect, `Send()` returns `transport.ErrReconnecting`.

### BLE Notes

BLE support uses Nordic UART Service with newline-delimited JSON-RPC payloads. Platform support and permissions depend on your host BLE stack.

Pass a device name to target a specific node, or an empty string to connect to the first matching Bramble NUS device:

```go
ble := transport.NewBLE("Bramble", transport.WithBLEScanTimeout(15*time.Second))
```

All three constructors share the same functional options: `WithAuthToken` (every transport), `WithBaudRate` (serial), and `WithBLEScanTimeout` (BLE).

The firmware's NUS characteristics require an encrypted link, so the host must already be paired and bonded with the device before calling `Connect`; this SDK does not pair automatically. A stale or missing bond makes writes fail silently (no error, no response).

Pairing itself is handled by the host OS, and which code it asks for depends on the node: a node with a display shows a random 6-digit code per attempt, while a node without one uses a static passkey (or no code at all until one is set). `BleSecurity(ctx)` reports that posture and `SetBlePasskey(ctx, passkey)` configures it.

See [docs/API.md](docs/API.md#ble-transport-details) for the pairing recipe and full BLE transport notes.

## API Overview

All client methods accept `context.Context` for timeout and cancellation.

- **Query methods**: Read node state and telemetry (`Status`, `Neighbors`, `Routes`, `Config`, delivery/traffic reads, etc.).
- **Action methods**: Send messages, probe, modify configuration, run maintenance operations (`Send*`, channel/radio config, `Reboot`, `OTAUpdate`, etc.).
- **Notification callbacks**: Subscribe to async events such as incoming messages, acks, probes, traffic events, Wi-Fi/GPS/location updates.
- **Action message helpers**: `WrapAction`, `IsAction`, and `ActionText` support IRC-style `/me` messages.
- **Test doubles**: the `transport/transporttest` package provides an in-memory `transport.Transport` (`transporttest.NewMock`) for consumer tests.

See the full API reference in [docs/API.md](docs/API.md).

## Protocol Compatibility

This SDK negotiates protocol versions on connect. See [VERSIONING.md](https://github.com/justinlindh/bramble/blob/main/VERSIONING.md) for the compatibility matrix.

## CI Parity (Local Quality Contract)

Use this exact command set locally to match CI quality gates:

```bash
go test ./...
go vet ./...
golangci-lint run
go build ./...
scripts/check-no-internal-refs.sh
```

### golangci-lint install/version contract

`golangci-lint` is expected at `$(go env GOPATH)/bin/golangci-lint` and pinned to `v2.11.4` for CI parity (the repo `.golangci.yml` uses the v2 config schema).

Install locally:

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b "$(go env GOPATH)/bin" v2.11.4
```

If `$(go env GOPATH)/bin` is not on your `PATH`, run lint with:

```bash
"$(go env GOPATH)/bin/golangci-lint" run
```

## Releases (semantic-release)

Releases are automated from `main` using semantic-release and Conventional Commits.

There is no in-repo `CHANGELOG.md`; per-release notes are generated by
semantic-release and published on the
[GitHub releases page](https://github.com/justinlindh/bramble-go/releases).

### Release workflow prerequisites

- Runner: release job targets `runs-on: ubuntu-latest` (GitHub-hosted runner).
- Default trigger: pushes to `main`.
- Manual trigger: `workflow_dispatch` with optional `dry_run=true` to validate release behavior without publishing tags/releases.

### Required GitHub Actions permissions

- `GITHUB_TOKEN`: provided automatically by GitHub Actions. The release job requests `contents: write` so semantic-release can create tags and GitHub Releases. No additional secret is required.

### Commit format

- `feat:` → minor release (`vX.Y.0`)
- `fix:` → patch release (`vX.Y.Z`)
- `feat!:` or `BREAKING CHANGE:` footer → major release (`vX.0.0`)

Examples:

- `feat(client): add message retry policy`
- `fix(ws): handle reconnect race`
- `feat(api)!: rename Status field`

## License

MIT — see [LICENSE](LICENSE)
