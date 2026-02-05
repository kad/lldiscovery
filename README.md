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

# List RDMA devices (diagnostic command)
./lldiscovery -list-rdma

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
  "http_address": ":6469",
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
| HTTP Address | `http_address` | `-http-address` | :6469 | HTTP API bind address |
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
    "endpoint": "grpc://localhost:4317",
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
  -telemetry-endpoint grpc://localhost:4317 \
  -telemetry-traces \
  -telemetry-metrics
```

**Endpoint formats:**
- `grpc://localhost:4317` (gRPC with explicit port)
- `grpc://localhost` (gRPC with default port 4317)
- `http://localhost:4318` (HTTP with explicit port)
- `http://localhost` (HTTP with default port 4318)
- `https://otel.example.com` (HTTPS with default port 4318)
- `localhost:4317` (legacy format, assumes gRPC)

See `OPENTELEMETRY.md` for complete documentation.

### HTTP API

The daemon exposes an HTTP API for querying the current graph:

```bash
# Get complete topology as JSON (nodes, edges, and segments if enabled)
curl http://localhost:6469/graph

# Get graph as DOT format
curl http://localhost:6469/graph.dot

# Health check
curl http://localhost:6469/health
```

**JSON Response Format:**

The `/graph` endpoint returns a JSON object with:
- `nodes`: Map of machine IDs to node information (hostname, interfaces, RDMA devices, etc.)
- `edges`: Map of edges between nodes showing direct and indirect connections
- `segments`: Network segments/VLANs detected (only when `--show-segments` is enabled)

Example response structure:
```json
{
  "nodes": {
    "machine-id-1": {
      "Hostname": "host1",
      "MachineID": "machine-id-1",
      "LastSeen": "2026-02-05T20:00:00Z",
      "Interfaces": {
        "eth0": {
          "IPAddress": "fe80::1",
          "RDMADevice": "",
          "Speed": 1000
        }
      },
      "IsLocal": false
    }
  },
  "edges": {
    "machine-id-1": {
      "machine-id-2": [
        {
          "LocalInterface": "eth0",
          "LocalAddress": "fe80::1",
          "RemoteInterface": "eth0",
          "RemoteAddress": "fe80::2",
          "LocalSpeed": 1000,
          "RemoteSpeed": 1000,
          "Direct": true,
          "LearnedFrom": ""
        }
      ]
    }
  },
  "segments": [
    {
      "Interface": "eth0",
      "ConnectedNodes": ["machine-id-1", "machine-id-2", "machine-id-3"],
      "EdgeInfo": { ... }
    }
  ]
}
```

**Edge Types:**
- `Direct: true` - Direct discovery between nodes (solid lines in DOT)
- `Direct: false` - Indirectly learned connections (dashed lines in DOT)
- `LearnedFrom` - Machine ID that shared this neighbor information (for indirect edges)

### Visualization

Generate a PNG or SVG image from the DOT file:

```bash
# Install graphviz
sudo apt-get install graphviz

# Generate visualization
dot -Tpng /var/lib/lldiscovery/topology.dot -o topology.png

# Or SVG (recommended for large topologies)
dot -Tsvg /var/lib/lldiscovery/topology.dot -o topology.svg

# Watch for changes and auto-regenerate
watch -n 5 'dot -Tpng /var/lib/lldiscovery/topology.dot -o topology.png'
```

**Subgraph Structure Benefits:**
- **Scalability**: Large topologies (100+ hosts) remain readable
- **Interface Clarity**: Easy to see which interfaces are connected
- **Multi-Interface Support**: Machines with multiple connections clearly show all paths
- **RDMA Visibility**: RDMA interfaces highlighted with light blue fill
- **Network Segmentation**: VLANs and segments visible through interface grouping

**Example Topology:**
```
┌─────────────────────────────────┐
│ server-01 (local)               │ ← Blue cluster border
│ ┌─────────┐ ┌─────────┐        │
│ │ eth0    │ │ ib0     │        │ ← Interface nodes
│ │ fe80::1 │ │ fe80::3 │        │
│ │ 10G     │ │ 100G    │        │
│ └────┬────┘ └────┬────┘        │
└──────┼──────────┼──────────────┘
       │          │ Blue thick line (RDMA)
       │          ↓
       │    ┌─────────────────┐
       │    │ server-02       │
       │    │ ┌─────────┐    │
       │    │ │ ib0     │    │
       │    │ │ fe80::5 │    │
       │    │ │ 100G    │    │
       │    │ └─────────┘    │
       │    └─────────────────┘
       ↓
  ┌─────────────────┐
  │ server-03       │
  │ ┌─────────┐    │
  │ │ eth0    │    │
  │ │ fe80::6 │    │
  │ │ 10G     │    │
  │ └─────────┘    │
  └─────────────────┘
```

### RDMA Diagnostics

List detected RDMA devices with their configuration:

```bash
$ ./lldiscovery -list-rdma
Found 1 RDMA device(s):

📡 rxe0
   Node GUID:      7686:e2ff:fe32:6988
   Sys Image GUID: 7686:e2ff:fe32:6988
   Node Type:      1 (CA)
   Parent interfaces:
      - enp0s31f6
        IPv6 link-local: fe80::f2e5:c0ad:dee1:44e2%enp0s31f6

Total: 1 RDMA device(s) on 1 network interface(s)
```

This command shows:
- **Node GUID**: Unique identifier for the RDMA device
- **Sys Image GUID**: System image GUID (same for all ports on multi-port adapters)
- **Node Type**: 1=CA (Channel Adapter), 2=Switch, 3=Router
- **Parent interfaces**: Network interfaces associated with the RDMA device
- **IPv6 link-local**: Addresses used for discovery

**Setting up software RDMA (RXE):**
```bash
# Install RDMA tools
sudo apt-get install rdma-core

# Create RXE device on an Ethernet interface
sudo rdma link add rxe0 type rxe netdev eth0

# Verify
./lldiscovery -list-rdma
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
- Each machine is shown as a **subgraph (cluster)** containing its interfaces
- The local node's cluster is highlighted with a blue border and "(local)" label
- Each interface is a separate node within the cluster showing:
  - Interface name (e.g., `eth0`, `ib0`)
  - IPv6 link-local address
  - Link speed in Mbps
  - RDMA device name (e.g., `[mlx5_0]`) with node_guid and sys_image_guid if present
- RDMA interfaces are highlighted with light blue fill
- Edges connect interface nodes between machines
- RDMA-to-RDMA connections shown in blue with thick lines

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

## Network Segment Detection

When multiple hosts are connected through the same network segment (switch/VLAN), the daemon can detect and visualize these shared connectivity patterns. Enable this feature with:

```bash
lldiscovery -show-segments
```

Or in configuration:
```json
{
  "show_segments": true
}
```

### How It Works

- **Detection**: When a host can reach 3+ other hosts on the same local interface, they're identified as sharing a network segment (switch/VLAN)
- **Visualization**: Segments appear as yellow ellipse nodes with interface name and node count
- **Edge Information**: Connections from segment to member nodes show:
  - Remote interface name and IP address
  - Link speed (if available)
  - RDMA device information (if available)
  - Blue dotted lines for RDMA-to-RDMA connections
- **Smart Edge Hiding**: 
  - Hides edges from local node to segment members **only when the edge uses the segment's interface**
  - Example: If segment is on eth0, edge A:eth0→B:eth1 is hidden (goes through segment)
  - Example: If segment is on eth0, edge A:eth1→B:eth2 is still shown (different interface)
- **Preserved Edges**: 
  - All edges on different interfaces than the segment
  - All edges between segment members
  - All edges between non-segment hosts
- **Default**: Disabled (opt-in feature)

### Example

For a topology where:
- Host-a has both eth0 and eth1
- Host-a reaches host-b, host-c, host-d via eth0 (forms segment)
- Host-a also has direct connection to host-b via eth1

Without `-show-segments`:
```
host-a:eth0 ---- host-b:eth0
host-a:eth0 ---- host-c:eth0
host-a:eth0 ---- host-d:eth0
host-a:eth1 ---- host-b:eth1
```

With `-show-segments`:
```
     [segment: eth0]
     [4 nodes]
    /    |    \
   /     |     \
  b:eth0 c:eth0 d:eth0
 (info) (info) (info)

host-a:eth1 ---- host-b:eth1  (still shown - different interface)
```

The segment hides only the eth0 connections (which go through the segment). The eth1 connection is preserved because it uses a different interface.

