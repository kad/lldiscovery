# Changelog

## [Unreleased]

### Fixed
- **PlantUML nwdiag Speed Calculation**: Fixed segment speed showing maximum instead of most common speed. Bridge interfaces often report higher speeds (10 Gbps) than underlying physical interfaces (1 Gbps), causing segments to display incorrect speeds. Now uses mode (most common speed) for more accurate segment representation.
- **PlantUML nwdiag RDMA Links Missing**: Fixed RDMA point-to-point links not appearing in diagrams. Edges between nodes in same segment were being marked as processed regardless of interface, preventing RDMA links on different interfaces (p2p1, p3p1) from appearing. Now tracks processed edges per specific interface pair, allowing multi-homed nodes to show all their connections.
- **Segment Edge Hiding**: Fixed redundant edge visualization between nodes in the same segment. Previously, direct edges were shown even when both nodes were segment members, creating visual clutter. Now correctly identifies which interface each node uses to connect to segments (handling reference/local nodes with empty RemoteInterface) and hides all intra-segment edges. Results in cleaner topology graphs, especially for large segments (16+ nodes).
- **Segment Merge Edge Selection**: Fixed edge information loss during segment merging. When a node appears in multiple segments with the same network prefix, now intelligently selects the edge with the most complete information (preferring edges with local interface, local prefixes, remote prefixes). Prevents loss of local node connection details and network prefix data during merge.
- **Segment Merging by Network Prefix**: Fixed segment duplication bug where different interfaces (em1, br112, etc.) on the same subnet were incorrectly treated as separate segments. Now merges segments that share the same network prefix, providing accurate topology with one segment per subnet. Handles mixed configurations with physical interfaces, bridges, and VLANs correctly.
- **Critical**: Local node prefix storage bug - prefixes were not being copied from InterfaceInfo to InterfaceDetails during local node initialization, resulting in null prefixes for the local node in JSON exports. Fixed by adding missing `GlobalPrefixes` field assignment in cmd/lldiscovery/main.go.

### Added
- **PlantUML nwdiag Export Enhancements**: Enhanced `/graph.nwdiag` HTTP endpoint with comprehensive visualization features:
  - **Hostname descriptions**: Node descriptions now show actual hostname instead of interface name for better readability
  - **Local node highlighting**: Local node marked with green color for easy identification
  - **RDMA link coloring**: Networks with RDMA devices colored sky blue (#87CEEB) to highlight high-performance interconnects
  - **Point-to-point networks**: P2P links exported as peer networks with speed and RDMA indication
  - **Speed in network address**: Network address includes speed (e.g., "192.168.1.0/24 (10000 Mbps)")
  - **Speed-based network coloring**: Gold (100+ Gbps), Orange (40+ Gbps), Light Green (10+ Gbps), Light Blue (1+ Gbps), Light Gray (< 1 Gbps)
  - **Complete node information**: Shows IP address, interface name, speed, and RDMA device in address field
- **PlantUML nwdiag Export**: Added `/graph.nwdiag` HTTP endpoint for exporting topology in PlantUML nwdiag format. Provides network-centric visualization showing segments horizontally with nodes on multiple networks. Includes IP addresses, interface names, and network prefixes. Ideal for documentation and understanding network segregation.
- **Edge Prefix Fields**: Added `LocalPrefixes` and `RemotePrefixes` fields to Edge struct for complete network prefix information in JSON exports. EdgeInfo in segments now includes prefix data for both local and remote sides of connections.
- **Network Prefix-Based Segment Naming**: Segments now display network prefixes instead of interface names when global addresses are available
  - Supports both IPv4 (e.g., "192.168.1.0/24") and IPv6 (e.g., "2001:db8:100::/64") prefixes
  - Daemon collects global unicast addresses from all interfaces
  - IPv4: Excludes loopback (127.0.0.0/8) and link-local (169.254.0.0/16)
  - IPv6: Uses global unicast addresses only
  - Prefixes propagated through discovery protocol
  - Segment detection chooses most common prefix as name hint
  - Falls back to interface name if no prefixes available
  - Interface name shown in parentheses for reference
  - Works correctly in dual-stack IPv4/IPv6 environments
- Enhanced DOT visualization with improved styling:
  - Direct links now use bold style for clear visual distinction
  - Segment-to-interface connections use solid lines with speed-based thickness
  - Circular graph layout (neato engine) for topologies with segments
  - Segment nodes positioned centrally with machines distributed around periphery
  - Speed-based line thickness for all connections (1.0-5.0 based on 100 Mbps to 100+ Gbps)

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
- **Connected Components Network Segment Detection**: Detects VLANs/network segments by reachability
  - **Algorithm**: BFS to find connected components of node:interface pairs
  - **Key insight**: B:if2 reaches A:if1 → they're on the same VLAN (direct or indirect doesn't matter)
  - **Segment criteria**: 3+ nodes all mutually reachable (connected component)
  - **Peer-to-peer filtering**: 2-node connections excluded (not shared networks)
  - **Trust transitive**: Doesn't verify complete cliques (trusts incomplete information)
  - **Why**: With transitive discovery, we may not see all edges - that's OK!
  - **Mixed interface support**: Segment can include em1, br112, eth0, p2p1 simultaneously
  - **Overlapping VLANs**: Nodes can have different interfaces on different VLANs
  - **Intelligent labels**: Single name or "if1+if2+if3" or "mixed(N)" for heterogeneous segments
  - **Examples**:
    * 13 hosts all reachable → ONE segment (even with incomplete edge visibility)
    * Node A on VLAN1 (eth0) + VLAN2 (eth1) → TWO separate segments
    * 2 nodes C↔D → NO segment (peer-to-peer link, below 3-node threshold)
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
