# lldiscovery

Network discovery daemon for VLAN-segmented environments using IPv6 link-local multicast.

## Overview

`lldiscovery` automatically discovers neighboring hosts on each network interface using IPv6 link-local multicast. It builds a dynamic topology graph showing which hosts are reachable on which network segments, useful for understanding VLAN configurations and network segmentation.

## Features

- **Automatic interface discovery**: Detects all active non-loopback interfaces using netlink
- **IPv6 link-local multicast**: Uses `ff02::4c4c:6469` for discovery without routing
- **Dynamic graph with edges**: Shows connections between nodes with interface labels and IP addresses
- **Multi-edge support**: Handles multiple interfaces between same hosts (redundant/bonded connections)
- **Smart interface display**: Only shows interfaces with active connections in the graph
- **Transitive discovery**: Optional sharing of neighbor information to see complete topology including indirect connections
- **Local node tracking**: Includes the local host in the topology graph
- **RDMA/InfiniBand support**: Detects RDMA devices and includes node_guid, sys_image_guid, and device names
  - Hardware RDMA: InfiniBand adapters (Mellanox/NVIDIA ConnectX, Intel TrueScale, etc.)
  - Software RDMA: RoCE (RDMA over Converged Ethernet) and Soft-RoCE (RXE)
  - Visual distinction: RDMA-to-RDMA connections shown in blue with thick lines
- **Multiple export formats**: DOT file for Graphviz + JSON over HTTP API
- **VLAN-aware**: Discovers hosts per-interface, showing segmentation
- **Configurable**: Timing, ports, and paths via config file or defaults
- **OpenTelemetry support**: Optional traces, metrics, and logs export

## Installation

### From Package (Recommended)

**Debian/Ubuntu (.deb):**
```bash
wget https://github.com/akanevsk/lldiscovery/releases/latest/download/lldiscovery_VERSION_linux_amd64.deb
sudo dpkg -i lldiscovery_VERSION_linux_amd64.deb
```

**RHEL/CentOS/Fedora (.rpm):**
```bash
wget https://github.com/akanevsk/lldiscovery/releases/latest/download/lldiscovery_VERSION_linux_amd64.rpm
sudo rpm -i lldiscovery_VERSION_linux_amd64.rpm
```

Package installation automatically:
- Creates `lldiscovery` system user
- Installs systemd service file
- Creates `/etc/lldiscovery/config.json` (config|noreplace)
- Creates `/var/lib/lldiscovery` data directory
- Sets proper ownership and permissions

After package installation:
```bash
# Edit configuration if needed
sudo nano /etc/lldiscovery/config.json

# Enable and start service
sudo systemctl enable lldiscovery
sudo systemctl start lldiscovery

# Check status
sudo systemctl status lldiscovery

# View logs
journalctl -u lldiscovery -f
```

### From Binary

Download the latest binary from [releases](https://github.com/akanevsk/lldiscovery/releases):

```bash
wget https://github.com/akanevsk/lldiscovery/releases/latest/download/lldiscovery_VERSION_linux_amd64.tar.gz
tar xzf lldiscovery_VERSION_linux_amd64.tar.gz
sudo cp lldiscovery /usr/local/bin/
sudo chmod +x /usr/local/bin/lldiscovery
```

Then follow manual setup steps below.

### Build from Source

```bash
go build -o lldiscovery ./cmd/lldiscovery
sudo cp lldiscovery /usr/local/bin/
```

### Manual Setup (for binary or source builds)

If installing from binary or building from source, follow these steps:

```bash
# Create user and directories
sudo useradd -r -s /bin/false lldiscovery
sudo mkdir -p /var/lib/lldiscovery /etc/lldiscovery
sudo chown lldiscovery:lldiscovery /var/lib/lldiscovery

# Copy configuration
sudo cp config.example.json /etc/lldiscovery/config.json
sudo chown root:lldiscovery /etc/lldiscovery/config.json
sudo chmod 640 /etc/lldiscovery/config.json

# Install systemd service
sudo cp lldiscovery.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable lldiscovery
sudo systemctl start lldiscovery
```

## Usage

### Run manually

```bash
# With default configuration (outputs to ./topology.dot if not running as root)
./lldiscovery

# With custom config file
./lldiscovery -config /etc/lldiscovery/config.json

# With debug logging
./lldiscovery -log-level debug

# Override specific settings via CLI flags
./lldiscovery -send-interval 10s -http-address :9999

# Enable transitive discovery
./lldiscovery -include-neighbors

# Combine config file with flag overrides (flags take precedence)
./lldiscovery -config config.json -log-level debug -send-interval 15s

# Show version
./lldiscovery -version

# Show all available flags
./lldiscovery --help
```

**Note:** The daemon automatically selects an output file location:
- If running with permissions to write to `/var/lib/lldiscovery/`, uses `/var/lib/lldiscovery/topology.dot`
- Otherwise, falls back to `./topology.dot` in the current directory
- You can override this with `-output-file` flag or `output_file` in configuration

### Configuration

Configuration can be provided via:
1. **Config file** (JSON format) - loaded with `-config` flag
2. **CLI flags** - override config file values
3. **Defaults** - used when neither file nor flag is specified

#### Config File Example

```json
{
  "send_interval": "30s",
  "node_timeout": "120s",
  "export_interval": "60s",
  "multicast_address": "ff02::1",
  "multicast_port": 9999,
  "output_file": "/var/lib/lldiscovery/topology.dot",
  "http_address": ":8080",
  "log_level": "info"
}
```

**Parameters:**

All parameters can be set via config file or CLI flags. CLI flags take precedence over config file values.

| Parameter | Config File | CLI Flag | Default | Description |
|-----------|-------------|----------|---------|-------------|
| Send Interval | `send_interval` | `-send-interval` | 30s | How often to send discovery packets |
| Node Timeout | `node_timeout` | `-node-timeout` | 120s | Remove nodes after no packets |
| Export Interval | `export_interval` | `-export-interval` | 60s | How often to export changes |
| Multicast Address | `multicast_address` | `-multicast-address` | ff02::4c4c:6469 | IPv6 multicast group |
| Multicast Port | `multicast_port` | `-multicast-port` | 9999 | UDP port for discovery |
| Output File | `output_file` | `-output-file` | (auto) | Path to DOT file output |
| HTTP Address | `http_address` | `-http-address` | :8080 | HTTP API bind address |
| Log Level | `log_level` | `-log-level` | info | Logging level (debug/info/warn/error) |
| Include Neighbors | `include_neighbors` | `-include-neighbors` | false | Enable transitive discovery |

**CLI Flag Examples:**
```bash
# Override send interval
./lldiscovery -send-interval 10s

# Change HTTP port
./lldiscovery -http-address :9999

# Enable transitive discovery with custom timeout
./lldiscovery -include-neighbors -node-timeout 60s

# Multiple overrides
./lldiscovery -config config.json -log-level debug -send-interval 15s -output-file /tmp/topology.dot
```

**Note on multicast_address:** The default `ff02::4c4c:6469` is a custom application-specific address.
Do NOT use `ff02::1` (all-nodes) as it's reserved for ICMPv6 and will cause interference with kernel networking.
See `MULTICAST_ADDRESS.md` for details.

### OpenTelemetry

Enable observability with distributed tracing and metrics via config file:

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "localhost:4317",
    "protocol": "grpc",
    "insecure": true,
    "enable_traces": true,
    "enable_metrics": true,
    "enable_logs": false
  }
}
```

Or via CLI flags:
```bash
./lldiscovery \
  -telemetry-enabled \
  -telemetry-endpoint localhost:4317 \
  -telemetry-protocol grpc \
  -telemetry-traces \
  -telemetry-metrics
```

See `OPENTELEMETRY.md` for complete documentation.

### OpenTelemetry

Enable observability with distributed tracing and metrics:

```json
{
  "telemetry": {
    "enabled": true,
    "endpoint": "localhost:4317",
    "protocol": "grpc",
    "insecure": true,
    "enable_traces": true,
    "enable_metrics": true,
    "enable_logs": false
  }
}
```

See `OPENTELEMETRY.md` for complete documentation.

### HTTP API

The daemon exposes an HTTP API for querying the current graph:

```bash
# Get graph as JSON
curl http://localhost:8080/graph

# Get graph as DOT format
curl http://localhost:8080/graph.dot

# Health check
curl http://localhost:8080/health
```

### Visualization

Generate a PNG image from the DOT file:

```bash
# Install graphviz
sudo apt-get install graphviz

# Generate visualization
dot -Tpng /var/lib/lldiscovery/topology.dot -o topology.png

# Or SVG
dot -Tsvg /var/lib/lldiscovery/topology.dot -o topology.svg

# Watch for changes and auto-regenerate
watch -n 5 'dot -Tpng /var/lib/lldiscovery/topology.dot -o topology.png'
```

## How It Works

1. **Interface Discovery**: Uses netlink library to detect all active non-loopback interfaces. Discovers RDMA devices and maps them to their parent network interfaces via sysfs.
2. **Local Node**: Adds local host to the graph with hostname, machine-id, interface information, and RDMA device names/GUIDs.
3. **Packet Broadcast**: Every `send_interval`, sends a JSON discovery packet to `ff02::4c4c:6469` (custom multicast group) on each interface. Packets include RDMA device names and GUIDs if applicable.
4. **Packet Reception**: Listens for discovery packets from other hosts on all interfaces, tracking which local interface received each packet
5. **Graph Building**: Adds/updates remote nodes and creates edges showing interface-to-interface connections
6. **Expiration**: Removes remote nodes that haven't sent packets within `node_timeout`
7. **Export**: Periodically checks for changes and exports the graph to DOT file and serves via HTTP API

**Graph Visualization**: 
- The local node is highlighted with a blue background and "(local)" label
- Edges between nodes show the connected interfaces (e.g., `eth0 <-> eth1`)
- RDMA interfaces display their device name (e.g., `[mlx5_0]`), node_guid, and sys_image_guid

### Multicast Address

**Important:** We use `ff02::4c4c:6469` (ASCII: "LLdi" for Link-Layer Discovery), NOT `ff02::1`.

- `ff02::1` is reserved for ICMPv6/NDP and causes interference with kernel networking
- `ff02::4c4c:6469` is application-specific and only joined by lldiscovery daemons
- See `MULTICAST_ADDRESS.md` for detailed explanation

### Discovery Packet Format

Discovery packets include complete RDMA information when available:

```json
{
  "hostname": "host1.example.com",
  "machine_id": "a1b2c3d4e5f6789...",
  "timestamp": 1234567890,
  "interface": "ib0",
  "source_ip": "fe80::1",
  "rdma_device": "mlx5_0",
  "node_guid": "0x1111:2222:3333:4444",
  "sys_image_guid": "0xaaaa:bbbb:cccc:dddd"
}
```

Note: `rdma_device`, `node_guid`, and `sys_image_guid` are omitted for non-RDMA interfaces.

## Network Requirements

- **IPv6**: Interfaces must have IPv6 link-local addresses (auto-configured)
- **Multicast**: Network switches must allow IPv6 multicast (usually enabled by default)
- **Firewall**: UDP port 9999 (or configured port) must be allowed for multicast
- **Permissions**: Requires `CAP_NET_RAW` and `CAP_NET_ADMIN` for multicast operations

### Firewall Configuration

```bash
# Allow multicast discovery (iptables)
sudo ip6tables -A INPUT -p udp --dport 9999 -j ACCEPT

# Allow multicast discovery (firewalld)
sudo firewall-cmd --permanent --add-port=9999/udp
sudo firewall-cmd --reload
```

## Troubleshooting

### No nodes discovered

- Check that other hosts are running the daemon
- Verify IPv6 is enabled: `ip -6 addr`
- Check firewall rules: `ip6tables -L -n`
- Enable debug logging: `-log-level debug`
- Verify multicast is working: `ping6 ff02::1%eth0`

### Permission denied errors

- Ensure daemon has network capabilities
- Run with sudo for testing: `sudo ./lldiscovery`
- Check systemd service has proper capabilities

### Nodes not expiring

- Check `node_timeout` configuration
- Verify system time is synchronized (NTP)
- Check logs for expiration messages

### Graph not updating

- Verify write permissions for `output_file` directory
- Check `export_interval` setting
- Look for errors in logs: `journalctl -u lldiscovery -f`
- If running as non-root, graph exports to `./topology.dot` in current directory
- Check startup log for `output_file` location: `output_file=./topology.dot` or `output_file=/var/lib/lldiscovery/topology.dot`

## Architecture

```
┌─────────────────────────────────────────┐
│           lldiscovery daemon            │
├─────────────────────────────────────────┤
│  Interface Discovery (netlink + RDMA)   │
│    ↓                                    │
│  Multicast Sender (per interface)       │
│  [sends packets with RDMA info]         │
│    ↓                                    │
│  Multicast Receiver (all interfaces)    │
│  [extracts RDMA from packets]           │
│    ↓                                    │
│  Graph Manager (with TTL expiration)    │
│  [stores RDMA in nodes and edges]       │
│    ↓                                    │
│  ┌─────────────┬──────────────────┐    │
│  │  DOT Export │   HTTP API       │    │
│  │  (periodic) │   (on-demand)    │    │
│  │  with RDMA  │   with RDMA      │    │
│  └─────────────┴──────────────────┘    │
└─────────────────────────────────────────┘
```

## Documentation

- **README.md** - This file (overview and quickstart)
- **QUICKSTART.md** - Fast setup guide
- **CHANGELOG.md** - Version history and features
- **EXAMPLE_OUTPUT.md** - Sample outputs and visualization examples
- **OPENTELEMETRY.md** - Observability setup guide
- **MULTICAST_ADDRESS.md** - Why we use ff02::4c4c:6469
- **DEBUG_LOGGING.md** - Debug logging guide with packet content
- **GRAPH_EDGES_IB.md** - Graph edges and InfiniBand support
- **LOCAL_NODE_FEATURE.md** - Local node highlighting
- **TRANSITIVE_DISCOVERY.md** - Transitive topology discovery (indirect connections)
- **RDMA_EDGE_LABELS.md** - RDMA information on edge labels
- **RDMA_EDGE_VISUAL.md** - Visual styling for RDMA-to-RDMA connections
- **RDMA_INFORMATION_FLOW.md** - Complete RDMA data flow verification
- **SOFT_ROCE_RXE.md** - Soft-RoCE (RXE) software RDMA support

## License

MIT

## Contributing

Issues and pull requests welcome!
