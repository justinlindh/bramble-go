# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

## [Unreleased]

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
