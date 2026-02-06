# lldiscovery v0.0.1 - First Release 🎉

**Release Date**: 2026-02-06

We're excited to announce the first stable release of **lldiscovery** - a production-ready network discovery daemon for VLAN-segmented environments!

## What is lldiscovery?

`lldiscovery` automatically discovers neighboring hosts across multiple network interfaces using IPv6 link-local multicast. It builds a dynamic topology graph showing which hosts are reachable on which network segments, making it invaluable for understanding VLAN configurations, network segmentation, and infrastructure connectivity.

### Key Use Cases

- 🔍 **Network Discovery**: Automatically map all reachable hosts across VLANs
- 🗺️ **Topology Visualization**: Generate network diagrams showing actual connectivity
- 🏢 **Data Center Management**: Track which hosts are on which segments
- 🔬 **RDMA/InfiniBand Networks**: Visualize high-performance computing interconnects
- 📊 **Network Auditing**: Verify VLAN isolation and segmentation policies
- 🐛 **Troubleshooting**: Understand complex multi-VLAN environments

## Major Features

### 🌐 Intelligent Network Discovery

- **Automatic Interface Detection**: Discovers all active non-loopback interfaces using netlink
- **IPv6 Link-Local Multicast**: Uses custom multicast group (`ff02::4c4c:6469`) for discovery without routing
- **Multi-Interface Support**: Handles hosts with multiple network connections (redundant links, bonding)
- **Transitive Discovery**: Optional neighbor sharing to see complete topology including indirect connections
- **Network Prefix Detection**: Collects global unicast prefixes (IPv4 and IPv6) for intelligent segment naming
- **VLAN-Aware**: Discovers hosts per-interface, revealing network segmentation

### 🎯 Network Segment Detection

Automatically identifies shared network segments (switches/VLANs):

- **Connected Components Algorithm**: Uses BFS to find groups of 3+ mutually reachable hosts
- **Multi-Prefix Support**: Segments can have multiple IPv4/IPv6 address spaces
- **Remote VLAN Detection**: Discovers VLANs even if local node isn't a member
- **2-Node Segments**: Detects small VLANs with only 2 participants via prefix matching
- **Smart Edge Hiding**: Only hides intra-segment edges, preserving inter-segment and P2P links
- **Multi-Homing Support**: Correctly handles nodes with multiple interfaces on same segment

### ⚡ RDMA/InfiniBand Support

First-class support for high-performance interconnects:

- **Hardware RDMA**: InfiniBand adapters (Mellanox/NVIDIA ConnectX, Intel TrueScale)
- **Software RDMA**: RoCE (RDMA over Converged Ethernet) and Soft-RoCE (RXE)
- **Complete Metadata**: node_guid, sys_image_guid, device names included
- **Visual Distinction**: RDMA-to-RDMA connections highlighted in blue with thick lines
- **Diagnostic Command**: `-list-rdma` flag to display RDMA device information

### 📊 Multiple Export Formats

**1. DOT/Graphviz Export** (`.dot` files)
- Subgraph visualization with machines as clusters containing interface nodes
- RDMA interfaces highlighted with light blue fill
- Speed-based line thickness (100 Mbps - 100+ Gbps)
- Segment visualization with yellow ellipse nodes
- Local node marked with blue border
- Direct links: bold solid lines
- Indirect links: dashed lines
- RDMA-to-RDMA: blue dotted lines

**2. PlantUML nwdiag Export** (`.puml` files)
- Network-centric horizontal layout
- Speed-based network coloring (Gold/Orange/Green/Blue/Gray)
- RDMA networks in sky blue
- Local node in green
- Complete node information: IP, interface, speed, RDMA device
- P2P networks show actual network prefixes
- Perfect for documentation and presentations

**3. JSON API Export**
- Real-time topology over HTTP
- Complete graph data: nodes, edges, segments
- Edge information includes prefixes, speeds, RDMA metadata
- Suitable for integration with monitoring systems

### 🚀 WiFi Network Support

Modern WiFi handling with accurate speed detection:

- **Native nl80211 Support**: Direct kernel communication via `github.com/mdlayher/wifi`
- **Actual Speed Detection**: Queries real TX/RX bitrates from nl80211
- **Fallback to iw Tool**: Uses external `iw` command if native method fails
- **WiFi 6 Support**: Correctly detects 1200+ Mbps for 802.11ax connections
- **No Root Required**: Works with regular user privileges

### 🎨 Deterministic Output

All export formats produce consistent, reproducible output:

- **Sorted Export**: Node IDs, interface names, edge pairs sorted alphabetically
- **Version Control Friendly**: Identical input produces identical output
- **Easy Comparison**: Simplifies diff operations and debugging
- **No Performance Impact**: Efficient sorting for typical topologies

## Technical Highlights

### Architecture

- **Language**: Pure Go (no CGO dependencies)
- **Lines of Code**: ~6,400 lines across core packages
- **Test Coverage**: 78 unit tests + 5 integration tests (all passing)
  - Graph operations: 92.8% coverage
  - Configuration: 89.4% coverage
  - Discovery: 38.0% coverage
- **Commits**: 62 commits of iterative development and refinement

### Performance

- **Lightweight**: Minimal resource usage (single-threaded core with goroutines for I/O)
- **Scalable**: Tested with 100+ host topologies
- **Concurrent**: Thread-safe graph access with read-write locks
- **Efficient**: Change detection prevents unnecessary exports

### Dependencies

Minimal external dependencies:
- `github.com/vishvananda/netlink` - Linux network configuration
- `github.com/mdlayher/wifi` - Native nl80211 WiFi support
- `golang.org/x/net/ipv6` - IPv6 multicast operations
- Standard library for everything else

### Configuration

Flexible configuration via JSON file or CLI flags:

```json
{
  "send_interval": "30s",
  "node_timeout": "120s",
  "export_interval": "60s",
  "multicast_address": "ff02::4c4c:6469",
  "multicast_port": 9999,
  "http_address": ":6469",
  "output_file": "/var/lib/lldiscovery/topology.dot",
  "show_segments": true,
  "include_neighbors": true,
  "telemetry_enabled": false
}
```

CLI flags override config file values for maximum flexibility.

## Installation

### Package Installation (Recommended)

**Debian/Ubuntu:**
```bash
wget https://github.com/kad/lldiscovery/releases/download/v0.0.1/lldiscovery_0.0.1_linux_amd64.deb
sudo dpkg -i lldiscovery_0.0.1_linux_amd64.deb
```

**RHEL/CentOS/Fedora:**
```bash
wget https://github.com/kad/lldiscovery/releases/download/v0.0.1/lldiscovery_0.0.1_linux_amd64.rpm
sudo rpm -i lldiscovery_0.0.1_linux_amd64.rpm
```

Package installation automatically:
- Creates `lldiscovery` system user
- Installs systemd service
- Creates `/etc/lldiscovery/config.json`
- Creates `/var/lib/lldiscovery` data directory
- Sets proper ownership and permissions

**Start the service:**
```bash
sudo systemctl enable lldiscovery
sudo systemctl start lldiscovery
sudo systemctl status lldiscovery
```

### Binary Installation

Download pre-built binaries for Linux x86_64:

```bash
wget https://github.com/kad/lldiscovery/releases/download/v0.0.1/lldiscovery_0.0.1_linux_amd64.tar.gz
tar xzf lldiscovery_0.0.1_linux_amd64.tar.gz
sudo cp lldiscovery /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/kad/lldiscovery.git
cd lldiscovery
make build
sudo make install
```

## Quick Start

### Basic Usage

**Run as daemon with default settings:**
```bash
lldiscovery
```

**Enable transitive discovery:**
```bash
lldiscovery -include-neighbors
```

**Enable network segment detection:**
```bash
lldiscovery -show-segments
```

**Both features enabled:**
```bash
lldiscovery -include-neighbors -show-segments
```

### Access the Topology

**HTTP API endpoints:**
```bash
# JSON format
curl http://localhost:6469/graph

# DOT format
curl http://localhost:6469/graph.dot

# PlantUML nwdiag format
curl http://localhost:6469/graph.nwdiag

# Health check
curl http://localhost:6469/health
```

**File output:**
```bash
# DOT file written to /var/lib/lldiscovery/topology.dot
cat /var/lib/lldiscovery/topology.dot
```

### Visualize the Topology

**Generate PNG from DOT:**
```bash
curl http://localhost:6469/graph.dot | dot -Tpng -o topology.png
```

**Generate SVG from DOT:**
```bash
curl http://localhost:6469/graph.dot | neato -Tsvg -o topology.svg
```

**Generate PNG from PlantUML:**
```bash
curl http://localhost:6469/graph.nwdiag > topology.puml
plantuml topology.puml
```

## OpenTelemetry Integration

Optional observability with OpenTelemetry:

```bash
# With gRPC endpoint
lldiscovery -telemetry-enabled -telemetry-endpoint grpc://otel-collector:4317

# With HTTP endpoint
lldiscovery -telemetry-enabled -telemetry-endpoint http://otel-collector:4318/v1/traces
```

Exports:
- **Traces**: Discovery operations, graph updates, exports
- **Metrics**: Node counts, edge counts, discovery latency
- **Logs**: Structured logs with trace correlation

## Example Topologies

### Small Office Network
```
3 hosts across 2 VLANs:
- Desktop + Laptop on VLAN1 (10.0.1.0/24)
- Server on both VLAN1 and VLAN2 (10.0.2.0/24)
- Printer on VLAN2
```

Result: 2 segments detected, server shows dual-homing

### Data Center Cluster
```
16 compute nodes:
- Management network (1 Gbps, eth0)
- Storage network (10 Gbps, eth1)
- RDMA network (100 Gbps, ib0)
```

Result: 3 segments detected, RDMA connections highlighted in blue

### WiFi + Wired Hybrid
```
Mixed environment:
- Laptops with WiFi (up to 1200 Mbps)
- Desktops with Ethernet (1 Gbps)
- All on same subnet (10.0.0.0/24)
```

Result: 1 segment with mixed interface types, accurate WiFi speeds

## Documentation

Comprehensive documentation included:

- **README.md**: Complete usage guide with examples
- **QUICKSTART.md**: 5-minute getting started guide
- **CHANGELOG.md**: Full feature history and fixes
- **RELEASING.md**: Release process documentation

Technical deep-dives (25+ docs):
- Network segment detection algorithms
- RDMA device handling
- WiFi speed detection
- PlantUML nwdiag export
- Multi-prefix segment support
- Transitive discovery protocol
- OpenTelemetry integration

## What's Next?

This is a stable v0.0.1 release with production-ready features. Future enhancements may include:

- Additional export formats (Mermaid, D2)
- Enhanced monitoring metrics
- REST API for graph manipulation
- Docker container images
- Additional platform support (arm64, macOS)

## Contributing

Contributions welcome! Please see:
- Issue tracker: https://github.com/kad/lldiscovery/issues
- Pull requests: https://github.com/kad/lldiscovery/pulls

## License

MIT License - See LICENSE file for details

## Acknowledgments

Built with:
- Go 1.21+
- vishvananda/netlink for Linux networking
- mdlayher/wifi for native nl80211 support
- OpenTelemetry for observability

Special thanks to the Go community and all contributors who helped shape this project through 62 commits of development!

---

**Download**: https://github.com/kad/lldiscovery/releases/tag/v0.0.1  
**Documentation**: https://github.com/kad/lldiscovery  
**Issues**: https://github.com/kad/lldiscovery/issues

---

*lldiscovery - Network discovery made simple. Know your network.*
