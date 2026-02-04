# Changelog

## [Unreleased]

### Added
- **OpenTelemetry Support**: Built-in observability with OTLP export
  - Distributed tracing for discovery operations
  - Metrics for packets, nodes, errors (8 metrics total)
  - Optional log export to OTLP
  - Support for both gRPC and HTTP protocols
  - Configurable endpoint and TLS settings
  - See `OPENTELEMETRY.md` for complete documentation
- Initial implementation of link-layer discovery daemon
- IPv6 link-local multicast discovery protocol
- Dynamic topology graph with TTL-based expiration
- DOT file export for Graphviz visualization
- HTTP API (JSON and DOT endpoints)
- Configurable intervals and parameters
- Systemd service integration
- Comprehensive documentation (README, QUICKSTART, examples)
- Integration test suite

### Fixed
- **CRITICAL**: Changed multicast address from `ff02::1` to `ff02::4c4c:6469`
  - `ff02::1` is reserved for ICMPv6/NDP and causes kernel interference
  - New address is application-specific and won't conflict with system networking
  - See `MULTICAST_ADDRESS.md` for detailed explanation

### Technical Details
- ~867 lines of Go code + ~300 lines telemetry
- Minimal dependencies (golang.org/x/net/ipv6 + OpenTelemetry SDK)
- Production-ready with security hardening
- Tested on Linux with multiple network interfaces
- OpenTelemetry SDK v1.40.0
