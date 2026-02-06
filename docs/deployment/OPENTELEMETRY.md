# OpenTelemetry Integration

## Overview

lldiscovery includes built-in OpenTelemetry support for observability through distributed tracing, metrics, and logs. Telemetry data is exported via OTLP (OpenTelemetry Protocol) over HTTP or gRPC to compatible backends like Jaeger, Prometheus, Grafana Tempo, or OpenTelemetry Collector.

## Configuration

### Endpoint URL Format

The `endpoint` field accepts URL-formatted endpoints for automatic protocol detection:

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "grpc://localhost:4317",
    "insecure": true,
    "enable_traces": true,
    "enable_metrics": true,
    "enable_logs": false
  }
}
```

**Supported formats:**

1. **gRPC (default port 4317):**
   ```
   "endpoint": "grpc://localhost:4317"      // explicit port
   "endpoint": "grpc://localhost"           // default port 4317
   "endpoint": "grpc://otel.example.com"    // default port 4317
   ```

2. **HTTP (default port 4318):**
   ```
   "endpoint": "http://localhost:4318"      // explicit port
   "endpoint": "http://localhost"           // default port 4318
   "endpoint": "https://otel.example.com"   // default port 4318 (TLS)
   ```

3. **Legacy format (assumes gRPC):**
   ```
   "endpoint": "localhost:4317"             // assumed grpc://
   "endpoint": "localhost"                  // assumed grpc://localhost:4317
   ```

The protocol is automatically detected from the URL scheme and default ports are applied if not specified.

### Configuration Options

- **`enabled`** (bool): Enable/disable all telemetry
  - Default: `false`
- **`endpoint`** (string): OpenTelemetry collector endpoint URL
  - Format: `protocol://host:port`
  - Supported protocols: `grpc://` (default port 4317), `http://`, `https://` (default port 4318)
  - Legacy format: `host:port` (assumes gRPC)
  - Default: `"grpc://localhost:4317"`
- **`insecure`** (bool): Use insecure connection (no TLS)
  - Default: `true`
- **`enable_traces`** (bool): Enable distributed tracing
  - Default: `true`
- **`enable_metrics`** (bool): Enable metrics export
  - Default: `true`
- **`enable_logs`** (bool): Enable log export
  - Default: `false`

## Traces

Distributed tracing tracks discovery packet flow through the system:

### Spans

- **send_discovery**: Overall discovery broadcast operation
  - Attributes: `interface_count`
  - Child spans: One `send_on_interface` per interface

- **send_on_interface**: Sending packet on a specific interface
  - Attributes: `interface`, `source_ip`, `packet_size`
  - Events: Errors for failures

- **handle_packet**: Processing received discovery packet
  - Attributes: `hostname`, `machine_id`, `interface`
  - Events: `ignored_own_packet` for self-packets

### Example Trace Flow

```
send_discovery (30ms)
├─ send_on_interface[eth0] (5ms)
├─ send_on_interface[eth1] (4ms)
└─ send_on_interface[wlan0] (6ms)

handle_packet (2ms)
└─ [from host2, eth0]
```

## Metrics

Application-specific metrics exported every 10 seconds:

### Available Metrics

| Metric Name | Type | Description | Labels |
|------------|------|-------------|--------|
| `lldiscovery.packets.sent` | Counter | Discovery packets sent | `interface` |
| `lldiscovery.packets.received` | Counter | Discovery packets received | `hostname`, `interface` |
| `lldiscovery.nodes.discovered` | UpDownCounter | Current number of discovered nodes | - |
| `lldiscovery.interfaces.active` | UpDownCounter | Active network interfaces | - |
| `lldiscovery.graph.exports` | Counter | Graph exports performed | - |
| `lldiscovery.nodes.expired` | Counter | Nodes expired due to timeout | - |
| `lldiscovery.errors.discovery` | Counter | Discovery errors encountered | `error`, `interface` |
| `lldiscovery.errors.multicast_join` | Counter | Multicast join failures | `interface` |

### Metric Labels

All metrics include automatic resource attributes:
- `service.name`: "lldiscovery"
- `service.version`: Version from build
- `host.name`: Hostname
- `os.type`: Operating system
- `process.pid`: Process ID

## Logs

When `enable_logs: true`, application logs are exported to OTLP in addition to stdout.

Note: Log export is optional and can add overhead. Recommended for troubleshooting only.

## Setting Up OpenTelemetry Collector

### Quick Start with Docker

```bash
# Create collector config
cat > otel-collector-config.yaml << EOF
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  logging:
    loglevel: debug
  prometheus:
    endpoint: "0.0.0.0:8889"
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [logging, jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [logging, prometheus]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [logging]
EOF

# Run collector
docker run -d \
  --name otel-collector \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 8889:8889 \
  -v $(pwd)/otel-collector-config.yaml:/etc/otel-collector-config.yaml \
  otel/opentelemetry-collector:latest \
  --config=/etc/otel-collector-config.yaml
```

### With Jaeger for Traces

```bash
# Run Jaeger
docker run -d \
  --name jaeger \
  -p 16686:16686 \
  -p 14250:14250 \
  jaegertracing/all-in-one:latest

# Update lldiscovery config
{
  "telemetry": {
    "enabled": true,
    "endpoint": "localhost:4317",
    "protocol": "grpc",
    "insecure": true,
    "enable_traces": true
  }
}

# View traces at http://localhost:16686
```

### With Prometheus for Metrics

```bash
# Run Prometheus
docker run -d \
  --name prometheus \
  -p 9090:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus:latest

# prometheus.yml
cat > prometheus.yml << EOF
scrape_configs:
  - job_name: 'otel-collector'
    scrape_interval: 10s
    static_configs:
      - targets: ['host.docker.internal:8889']
EOF

# View metrics at http://localhost:9090
```

### Complete Stack (Grafana + Tempo + Prometheus)

```bash
# docker-compose.yml
version: '3.8'
services:
  tempo:
    image: grafana/tempo:latest
    command: [ "-config.file=/etc/tempo.yaml" ]
    ports:
      - "3200:3200"
      - "4317:4317"
    volumes:
      - ./tempo.yaml:/etc/tempo.yaml

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_AUTH_ANONYMOUS_ENABLED=true
      - GF_AUTH_ANONYMOUS_ORG_ROLE=Admin

docker-compose up -d

# Access Grafana at http://localhost:3000
# Add data sources for Tempo and Prometheus
```

## Example Queries

### Prometheus Queries

```promql
# Packets sent per second per interface
rate(lldiscovery_packets_sent[5m])

# Current number of discovered nodes
lldiscovery_nodes_discovered

# Error rate
rate(lldiscovery_errors_discovery[5m])

# Packet loss (sent vs received across cluster)
sum(rate(lldiscovery_packets_sent[5m])) - sum(rate(lldiscovery_packets_received[5m]))
```

### Grafana Dashboard

Import this dashboard for quick visualization:

```json
{
  "panels": [
    {
      "title": "Discovery Packets",
      "targets": [
        {"expr": "rate(lldiscovery_packets_sent[5m])", "legendFormat": "Sent"},
        {"expr": "rate(lldiscovery_packets_received[5m])", "legendFormat": "Received"}
      ]
    },
    {
      "title": "Discovered Nodes",
      "targets": [
        {"expr": "lldiscovery_nodes_discovered"}
      ]
    },
    {
      "title": "Error Rate",
      "targets": [
        {"expr": "rate(lldiscovery_errors_discovery[5m])"}
      ]
    }
  ]
}
```

## Protocol Support

### gRPC (Recommended)

```json
{
  "telemetry": {
    "protocol": "grpc",
    "endpoint": "localhost:4317"
  }
}
```

- More efficient for high-volume data
- Better for production environments
- Default OTLP port: 4317

### HTTP

```json
{
  "telemetry": {
    "protocol": "http",
    "endpoint": "localhost:4318"
  }
}
```

- Easier to debug with proxies
- Better firewall compatibility
- Default OTLP port: 4318

## TLS Configuration

For production with TLS:

```json
{
  "telemetry": {
    "endpoint": "otel-collector.example.com:4317",
    "insecure": false
  }
}
```

Ensure the collector has valid TLS certificates. The daemon uses system certificate pool for verification.

## Performance Impact

Telemetry adds minimal overhead:

- **Traces**: ~1-2ms per operation
- **Metrics**: Collected in memory, exported every 10s
- **Logs**: Batched export, ~100μs per log entry

Disable logs export in production for best performance:

```json
{
  "telemetry": {
    "enable_logs": false
  }
}
```

## Troubleshooting

### Connection Refused

```
failed to setup telemetry: failed to setup trace provider: context deadline exceeded
```

**Solution**: Ensure OTLP collector is running and reachable:

```bash
curl -v http://localhost:4318/v1/traces  # HTTP
telnet localhost 4317  # gRPC
```

### High Memory Usage

Telemetry batches data before export. Adjust batch size if needed (requires code change):

```go
sdktrace.WithBatcher(exporter,
    sdktrace.WithMaxQueueSize(512),
    sdktrace.WithBatchTimeout(5*time.Second),
)
```

### Disable Telemetry

Set `enabled: false` in config, or remove telemetry section entirely.

## Examples

### gRPC Endpoint (Default)

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "grpc://localhost:4317",
    "insecure": true,
    "enable_traces": true,
    "enable_metrics": true
  }
}
```

### HTTP Endpoint

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "http://localhost:4318",
    "insecure": true,
    "enable_traces": true,
    "enable_metrics": true
  }
}
```

### HTTPS Endpoint (TLS)

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "https://otel.example.com:4318",
    "insecure": false,
    "enable_traces": true,
    "enable_metrics": true
  }
}
```

### CLI Flags

Override endpoint via command line:

```bash
# gRPC endpoint
lldiscovery -telemetry-enabled -telemetry-endpoint grpc://localhost:4317

# HTTP endpoint
lldiscovery -telemetry-enabled -telemetry-endpoint http://localhost:4318

# HTTPS with TLS
lldiscovery -telemetry-enabled -telemetry-endpoint https://otel.example.com:4318

# Legacy format (assumes gRPC)
lldiscovery -telemetry-enabled -telemetry-endpoint localhost:4317
```

### Minimal Setup (gRPC)

```json
{
  "telemetry": {
    "enabled": true
  }
}
```

Uses defaults: `localhost:4317`, gRPC, traces + metrics only.

### Production Setup (HTTP with TLS)

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "otel.company.com:4318",
    "protocol": "http",
    "insecure": false,
    "enable_traces": true,
    "enable_metrics": true,
    "enable_logs": false
  }
}
```

### Local Development (All Features)

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "localhost:4317",
    "protocol": "grpc",
    "insecure": true,
    "enable_traces": true,
    "enable_metrics": true,
    "enable_logs": true
  }
}
```

## Further Reading

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [OTLP Specification](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [Grafana Cloud](https://grafana.com/products/cloud/)
- [Jaeger](https://www.jaegertracing.io/)
