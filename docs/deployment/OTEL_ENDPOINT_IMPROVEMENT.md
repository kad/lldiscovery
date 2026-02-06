# OpenTelemetry Endpoint URL Improvement - Summary

## Overview

Enhanced OpenTelemetry endpoint configuration to use URL-based format with automatic protocol detection and default ports.

## What Changed

### Before
```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "localhost:4317",
    "protocol": "grpc",
    "insecure": true
  }
}
```

Separate fields for endpoint and protocol, no default ports.

### After
```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "grpc://localhost:4317",
    "insecure": true
  }
}
```

Single URL-based endpoint with automatic protocol detection and default ports.

## Features

### URL Format Support

1. **gRPC (default port 4317):**
   - `grpc://localhost:4317` - explicit port
   - `grpc://localhost` - uses default port 4317
   - `grpc://otel.example.com` - uses default port 4317

2. **HTTP (default port 4318):**
   - `http://localhost:4318` - explicit port
   - `http://localhost` - uses default port 4318
   - `https://otel.example.com` - uses default port 4318 (TLS)

3. **Legacy format (backward compatible):**
   - `localhost:4317` - assumes gRPC
   - `localhost` - assumes gRPC with default port 4317

### Automatic Protocol Detection

The protocol is automatically extracted from the URL scheme:
- `grpc://` → protocol = "grpc"
- `http://` or `https://` → protocol = "http"
- No scheme → assumes "grpc://"

### Default Ports

Industry-standard OTLP ports applied automatically:
- gRPC: 4317 (if not specified)
- HTTP/HTTPS: 4318 (if not specified)

## Implementation

### New Function: ParseEndpoint()

```go
func (t *TelemetryConfig) ParseEndpoint() error {
    // Parses URL-formatted endpoint
    // Extracts protocol from scheme
    // Applies default ports
    // Updates Endpoint and Protocol fields
}
```

Located in `internal/config/config.go`

### Changes Made

1. **internal/config/config.go**
   - Added `ParseEndpoint()` method
   - Imports `net/url`, `fmt`, `strings`
   - Called during config loading and after CLI flag parsing

2. **cmd/lldiscovery/main.go**
   - Removed `-telemetry-protocol` CLI flag
   - Updated `-telemetry-endpoint` description
   - Call `ParseEndpoint()` after flag processing

3. **config.example.json**
   - Updated endpoint to `"grpc://localhost:4317"`
   - Removed `"protocol"` field

4. **Documentation**
   - **OPENTELEMETRY.md**: Complete rewrite of configuration section with examples
   - **README.md**: Updated endpoint examples
   - **CHANGELOG.md**: Added entry for this change

5. **Tests**
   - **internal/config/config_test.go**: Comprehensive test suite with 12 test cases
   - Tests cover all URL formats, protocols, default ports, and edge cases

## Testing

### Unit Tests

All 12 test cases pass:
```bash
go test ./internal/config -v
=== RUN   TestTelemetryConfig_ParseEndpoint
    --- PASS: TestTelemetryConfig_ParseEndpoint/grpc_with_port
    --- PASS: TestTelemetryConfig_ParseEndpoint/grpc_without_port
    --- PASS: TestTelemetryConfig_ParseEndpoint/http_with_port
    --- PASS: TestTelemetryConfig_ParseEndpoint/http_without_port
    --- PASS: TestTelemetryConfig_ParseEndpoint/https_with_port
    --- PASS: TestTelemetryConfig_ParseEndpoint/https_without_port
    --- PASS: TestTelemetryConfig_ParseEndpoint/bare_host:port_(assumes_grpc)
    --- PASS: TestTelemetryConfig_ParseEndpoint/bare_host_(assumes_grpc,_adds_port)
    --- PASS: TestTelemetryConfig_ParseEndpoint/custom_port_with_grpc
    --- PASS: TestTelemetryConfig_ParseEndpoint/custom_port_with_http
    --- PASS: TestTelemetryConfig_ParseEndpoint/unsupported_protocol
    --- PASS: TestTelemetryConfig_ParseEndpoint/empty_endpoint
PASS
```

### Integration Tests

```bash
./test.sh
==> All tests passed! ✓
```

### Manual Endpoint Testing

Tested all supported formats:
- ✅ `grpc://localhost:4317`
- ✅ `grpc://localhost` → becomes `localhost:4317`
- ✅ `http://localhost:4318`
- ✅ `http://localhost` → becomes `localhost:4318`
- ✅ `https://otel.example.com:4318`
- ✅ `localhost:4317` → assumes gRPC
- ✅ `localhost` → becomes `localhost:4317` with gRPC
- ✅ CLI flag: `-telemetry-endpoint grpc://localhost:9999`

## Benefits

1. **Simplicity**: Single field instead of separate endpoint + protocol
2. **Industry Standard**: Uses standard OTLP port numbers (4317 gRPC, 4318 HTTP)
3. **Intuitive**: URL format is familiar and self-documenting
4. **Backward Compatible**: Legacy `host:port` format still works
5. **Less Error-Prone**: Protocol is derived from URL, no mismatch possible
6. **Cleaner Config**: One less field to configure

## Migration Guide

### Old Configuration
```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "localhost:4317",
    "protocol": "grpc"
  }
}
```

### New Configuration (Recommended)
```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "grpc://localhost:4317"
  }
}
```

### New Configuration (With Defaults)
```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "grpc://localhost"
  }
}
```

**Note**: The old format (`"endpoint": "localhost:4317"` without protocol) still works and assumes gRPC.

## Examples from Documentation

### gRPC (Default)
```bash
lldiscovery -telemetry-enabled -telemetry-endpoint grpc://localhost:4317
```

### HTTP
```bash
lldiscovery -telemetry-enabled -telemetry-endpoint http://localhost:4318
```

### HTTPS with Custom Port
```bash
lldiscovery -telemetry-enabled -telemetry-endpoint https://otel.example.com:9999
```

### Legacy Format
```bash
lldiscovery -telemetry-enabled -telemetry-endpoint localhost:4317
```

## Files Changed

- `internal/config/config.go` - Added ParseEndpoint() method
- `internal/config/config_test.go` - New comprehensive test suite
- `cmd/lldiscovery/main.go` - Removed -telemetry-protocol flag
- `config.example.json` - Updated endpoint format
- `OPENTELEMETRY.md` - Rewrote configuration section
- `README.md` - Updated examples
- `CHANGELOG.md` - Added entry

## Error Handling

Unsupported protocols produce clear errors:
```
invalid telemetry endpoint: unsupported protocol scheme: ftp (use grpc://, http://, or https://)
```

Empty endpoints are handled gracefully (no parsing attempted).
