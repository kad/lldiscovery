# Changelog

## [Unreleased]

### Added

- **Subgraph Visualization**: Improved DOT output for large topologies
  - Each machine rendered as a subgraph (cluster) with separate interface nodes
  - Machine cluster shows hostname and machine ID in border label
  - Each interface is a distinct node showing: name, IP, speed, RDMA info
  - RDMA interfaces highlighted with light blue fill (#e6f3ff)
  - Edges connect interface nodes directly (not machine nodes)
  - Edge labels simplified (addresses + speed) since interface info is in nodes
  - Local node cluster marked with blue border
  - Dramatically improves readability for 100+ host topologies
  - Easier to trace which interfaces connect where
  - Better visual grouping of multi-interface hosts
- **Clique-Based Network Segment Detection**: Detects actual physical network segments using maximal cliques
  - **Algorithm**: Bron-Kerbosch with pivoting to find all maximal cliques
  - **Key insight**: A segment is a CLIQUE (complete subgraph) where ALL nodes mutually connect
  - **Example**: If A↔B, B↔C, A↔C all exist, they form a segment (triangle clique)
  - **Not just connected**: Linear chain A→B→C is NOT a segment (A doesn't reach C directly)
  - **Mixed interface support**: Segment can include em1, br112, eth0, p2p1 simultaneously
  - **Overlapping segments**: Nodes can participate in multiple segments using different interfaces
  - **Intelligent labels**: Single name or "if1+if2+if3" or "mixed(N)" for heterogeneous segments
  - **Transitive discovery**: Works with indirect edges learned from neighbor sharing
  - **Fewer false positives**: Only detects actual network segments (switches/VLANs)
  - **Examples**:
    * 8 hosts all mutually connected on same switch → ONE segment
    * Node A on eth0 segment + eth1 segment → TWO separate cliques detected
    * Linear topology A-B-C without full mesh → NO segment (not a clique)
    * Previously: Connected components gave false segments → Now: Only true cliques
  - Handles heterogeneous networks with diverse interface naming conventions
- **Network Segment Detection**: Identify and visualize shared network segments (switches/VLANs)
  - Detects when a host can reach 3+ neighbors on the same interface
  - Segments show interface name, node count, and RDMA capability
  - Preserves edge information: interface names, IP addresses, speeds, RDMA devices
  - **Smart edge hiding**: Only hides edges that match the segment's interface
    - Edge A:eth0→B:eth1 hidden if segment on eth0 (goes through segment)
    - Edge A:eth1→B:eth2 shown if segment on eth0 (different interface)
  - Keeps all edges on different interfaces and between non-segment hosts
  - Visualizes segments as yellow ellipse nodes with detailed connection labels
  - RDMA-to-RDMA connections shown with blue dotted lines
  - Opt-in feature via `-show-segments` flag or `show_segments: true` in config
  - 5 unit tests for segment detection logic
  - Helps identify broadcast domains and shared connectivity patterns
- **GoReleaser Support**: Automated releases with deb/rpm packages
  - Linux x86_64 packages (.deb for Debian/Ubuntu, .rpm for RHEL/CentOS/Fedora)
  - Automated GitHub releases with GitHub Actions
  - Package installation includes user creation, systemd service, and proper permissions
  - Binary archives with documentation and configuration files
- **Unit Tests**: Comprehensive test suite for core packages
  - Configuration: 20 test cases with 89.4% coverage
  - Graph operations: 23 test cases with 92.8% coverage (node/edge management, expiration, concurrency, segments)
  - Discovery packets: 6 test cases for JSON serialization and neighbor information
  - Discovery helpers: 10 test cases for interface detection, IPv6 parsing, RDMA device mapping
  - Total: 59 unit tests + 5 integration tests = 64 tests, all passing
  - Excellent coverage of critical components: graph (92.8%), config (89.4%), discovery (38.0%)
  - Thread-safety validated with concurrent access tests
  - See UNIT_TESTS.md for details
- **CLI Diagnostic Command**: `-list-rdma` flag to display RDMA device information
  - Shows node GUIDs, sys image GUIDs, node types
  - Lists parent network interfaces with IPv6 addresses
  - Useful for debugging RDMA/InfiniBand configurations
  - Supports both hardware RDMA and software RXE devices

### Changed

- **Enhanced OpenTelemetry endpoint configuration**:
  - URL-based endpoint format with automatic protocol detection
  - Supports `grpc://host:port` (default port 4317) and `http://host:port` (default port 4318)
  - Legacy `host:port` format still supported (assumes gRPC)
  - Removed separate `-telemetry-protocol` flag (now derived from endpoint URL)
  - Examples updated to show both gRPC and HTTP endpoints
- Enhanced version output to include commit hash and build date

### Previous Features
  - Timing: `-send-interval`, `-node-timeout`, `-export-interval`
  - Network: `-multicast-address`, `-multicast-port`
  - Output: `-output-file`, `-http-address`
  - Features: `-include-neighbors`
  - Telemetry: `-telemetry-enabled`, `-telemetry-endpoint`, `-telemetry-protocol`, etc.
  - Logging: `-log-level`
  - CLI flags override config file values (precedence: CLI > config file > defaults)
  - Use `--help` to see all available flags
- **Transitive Topology Discovery**: Build complete network topology including indirect connections
  - Disabled by default (`include_neighbors: false`) for backward compatibility
  - When enabled, each daemon shares its direct neighbor list with **complete edge information**
  - Neighbor packets include both sides of each edge: local interface/address/RDMA and remote interface/address/RDMA
  - Creates indirect edges (dashed lines) with complete information, no empty fields
  - Cascading expiration: indirect edges removed when source node expires
  - Edge upgrades: indirect edges automatically upgraded to direct when direct connection established
  - Topology depth limited to 1-hop (only direct neighbors shared) to prevent packet size explosion
  - Backward compatible: old daemons ignore neighbors field, new daemons work without it
  - See [TRANSITIVE_DISCOVERY.md](TRANSITIVE_DISCOVERY.md) for complete guide

### Changed

- Enhanced version output to include commit hash and build date

### Previous Features
- **RDMA-to-RDMA Edge Marking**: Visual distinction for full RDMA connections
  - Edges with RDMA on both sides marked with `[RDMA-to-RDMA]` label
  - RDMA-to-RDMA edges displayed in **blue** with **thick lines** (penwidth=2.0)
  - Mixed edges (RDMA on one side only) shown with normal styling
  - Non-RDMA edges shown with normal styling
  - Makes it easy to identify high-performance RDMA fabric paths
- **Soft-RoCE (RXE) Support**: Full detection of software RDMA devices
  - Detects RXE interfaces created with `rxe` kernel module
  - Reads parent interface from `/sys/class/infiniband/<device>/parent` file
  - Works alongside hardware RDMA detection (InfiniBand, Mellanox adapters)
  - Includes Node GUID and Sys Image GUID for RXE devices
  - RXE devices correctly associated with their parent Ethernet interfaces
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
