# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

## [Unreleased]

### Added

- feat(transport): refactor NewBLE to accept functional Option pattern (7727862)
- feat(client): add GetAuthToken() for serial pairing (c87e892)
- transport: add SetAuthToken to Transport interface (cf16d63)
- Add Go SDK wrappers for missing firmware RPC methods — GetBattery, GetGpsPosition, GetBeaconPolicy, SetBeaconPolicy, GetAudioStatus, GetStorageInfo, SetBroadcastTelemetryMode, SetBacklight, Sleep, PlayTone, SetVolume, SetMuted, OnDecodeError (0745504)
- feat(client): expose full setLocationContact fields (a6254bf)
- feat(transport): share auth option across serial and websocket (e840898)
- test(transport): add BLE auth handshake and Connect coverage (b812954)
- test(transport): cover auth token setters across transports (5b3e4c3)

### Fixed

- fix(transport): add BLE GetAuthToken for transport symmetry (f3f1f9d)
- fix(location): align sdk location tier examples and fixtures (932e9e6)
- fix(transport): add BLE disconnect and reconnect hooks (5526497)
- fix(transport): propagate caller context through Send (5fc68b8)
- fix(client): format Broadcast deprecation as godoc (3a53db8)
- fix(sdk): address audit findings C31-C34, C37 (274ad0d)
- fix(notify): log JSON decode errors in notifyLoop with payload context (57ac99b)
- fix(types): change SendResult.Channel from int to *int (a9bf28f)
- fix(client): use snake_case since_event_seq param in DeliveryEvents (d6e17ef)
- fix(types): remove unused LocationContact and prevent LocationEvent timestamp overflow (2e0fa73)
- Fix IsCompatible semver comparison for protocol versions (5b96306)

### Changed

- refactor(transport): make auth token fields unexported across all transports (943740c)
- build: bump Go minimum to 1.26.1 (6a2c871)
- docs: fix SetNodeName max chars (8→32), add examples README (be22584)

## [2026-03-01]

_Changes captured from `git log --oneline --since=2026-02-01`._

### Added

- (client): add get wifi status sdk method (b2113ec)
- (client): add critical send helpers for message and broadcast (535dd62)
- add Name field to Neighbor struct (7f1a90b)
- add GetConfig, OnProbeResult, OnProbeComplete methods (4a77873)
- (sdk): add delivery replay API types and protocol 0.5.0 support (8d2e189)
- add bramble.otaUpdate client API (414bfa8)
- (sdk): align location models/decoding with OpenAPI (cf4394c)
- (go-sdk): support broadcast delivery telemetry events (79e3d18)
- (sdk): add channel-scoped broadcast API (6947834)
- (protocol): expose channel hasPsk/epoch in SDK types (54c6f87)
- (sdk): support protocol version 0.2.0 (b71b0aa)
- add traffic debug SDK support (6d4b874)
- add missing fields from firmware updates (b59b27e)
- (transport): serial auto-reconnect with exponential backoff (cfede56)
- implement BLE transport using NimBLE NUS (240c916)
- bramble-go SDK v0.1.0 — transport interface, serial/ws/ble, JSON-RPC protocol, full client (69a7dba)

### Fixed

- normalize JSON tags to snake_case and add missing fields (ff547a4)
- align JSON field names with firmware wire format (0b58414)
- align SDK types with actual firmware wire format (69323fa)

### Documentation

- update VERSIONING.md links to bramble main branch (0e5a4c4)
- add /me action message helpers to README (4a5d0e1)
- fix node name max length (8 → 32) in README (58615c3)
- (go-sdk): refresh BLE and transport usage docs (b21185d)
