# Changelog

## [Unreleased]

### Added
- **Packet Content in Debug Logs**: Complete JSON packet content shown in debug logs
  - Sent packets include full JSON in `content` field
  - Received packets include full JSON in `content` field
  - Makes it easy to verify RDMA information transmission
  - Useful for debugging packet parsing issues
  - Can pipe to `jq` for pretty-printing: `grep content | jq`
- **RDMA GUIDs on Edge Labels**: Complete RDMA identification information on edges
  - Edge labels include Node GUID and Sys Image GUID for both local and remote sides
  - Format: `Local: mlx5_0 N:0x1111:2222:3333:4444 S:0xaaaa:bbbb:cccc:dddd`
  - Enables precise hardware identification directly from the topology graph
  - Critical for InfiniBand fabric management and troubleshooting
- **RDMA Information on Edges**: Full RDMA details preserved for both connection endpoints
  - Edge labels show RDMA devices for both local and remote sides when present
  - Format: `[Local: mlx5_0, Remote: mlx5_1]` when both sides have RDMA
  - All RDMA GUIDs (node_guid, sys_image_guid) preserved in edge data structure
  - Useful for InfiniBand/RoCE topology visualization
- **Multi-Edge Support**: Handles multiple connections between same pair of hosts
  - When hosts have multiple interfaces on the same VLAN, each creates a separate edge
  - Useful for visualizing redundant/bonded connections
  - All edges are shown in the graph, not just one per host pair
- **Address Labels on Edges**: IP addresses moved from node labels to edge labels
  - Edge labels now show: `eth0 (fe80::1) <-> eth1 (fe80::2)`
  - Makes it easier to see which specific addresses are connected
  - Node labels are cleaner and show only interface names (and RDMA info)
- **Smart Interface Display**: Only show interfaces that have discovered connections
  - Interfaces without neighbors are hidden from node labels
  - Reduces clutter in large deployments
  - Makes it immediately obvious which interfaces are active
- **Graph Edges**: Topology graph now shows connections between nodes
  - Edges display interface names on both ends (e.g., `eth0 <-> eth1`)
  - Helps visualize which interfaces are on the same network segment
  - Edges automatically created when packets are received
  - DOT output includes edge labels for easy identification
- **RDMA/InfiniBand Support**: Robust detection using netlink library
  - Uses `github.com/vishvananda/netlink` for interface discovery
  - Detects all RDMA devices (InfiniBand, RoCE, iWARP) and their parent network interfaces
  - Maps network interfaces to RDMA devices via sysfs (`/sys/class/infiniband/<device>/device/net/`)
  - Includes RDMA device name (e.g., `mlx5_0`, `mlx4_0`) in output
  - Reads and includes `node_guid` and `sys_image_guid` for each RDMA device
  - GUIDs and RDMA device names displayed in both DOT and JSON outputs
  - More reliable than type-based detection, works with all RDMA protocols
- **Enhanced Debug Logging**: Detailed packet reception information
  - Added `sender_interface` field showing which interface the sender used
  - Added `received_on` field showing which local interface received the packet
  - Enabled IPv6 control messages to capture receiving interface information
  - Useful for troubleshooting VLAN connectivity and understanding traffic flow
- **Local Node in Graph**: Daemon now includes itself in the topology graph
  - Local node is highlighted with blue background and "(local)" label in DOT output
  - JSON API includes `IsLocal: true` field for the local node
  - Makes it easy to identify the observing host in multi-host topologies
  - Shows local interfaces and addresses alongside discovered remote nodes
- **Smart Output File Selection**: Automatically falls back to `./topology.dot` if `/var/lib/lldiscovery/` is not writable
  - No more permission errors when running as non-root user
  - Startup log shows which output file path is being used
  - Can still override with explicit `output_file` configuration
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
