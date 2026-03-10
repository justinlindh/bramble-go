# Examples

Usage examples for the `bramble-go` SDK.

## Contents

| Directory | Description |
|-----------|-------------|
| [`basic/`](basic/) | Connect to a node, read status, and send a broadcast message |
| [`monitor/`](monitor/) | Subscribe to real-time node events (messages, beacons, location) |

## Running

Each example is a standalone Go program:

```bash
cd examples/basic
go run main.go
```

By default, examples auto-detect a serial-connected node. Set `BRAMBLE_TRANSPORT` to use WebSocket or BLE:

```bash
BRAMBLE_TRANSPORT=ws://192.168.4.1/ws go run main.go
```

See the [SDK documentation](../docs/API.md) for the full client API reference.
