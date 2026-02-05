# Network Segment Detection

## Overview

Network segment detection identifies when multiple hosts are connected through the same shared network infrastructure (switches, VLANs). This helps visualize broadcast domains and shared connectivity patterns in complex network topologies.

## Usage

### Enable via CLI Flag

```bash
lldiscovery -show-segments
```

### Enable via Configuration File

```json
{
  "show_segments": true
}
```

### Default Behavior

- **Disabled by default** - opt-in feature to keep visualizations clean
- When disabled, only point-to-point host connections are shown
- When enabled, shared network segments appear as additional nodes

## How It Works

### Detection Algorithm: Connected Components with Clique Verification

The algorithm finds network segments (VLANs/switches) by identifying groups of mutually-connected node:interface pairs:

1. **Build Connectivity Graph**: Create a bidirectional map of all `node:interface` pairs
   - Example: `hostA:em1` ↔ `hostB:br112`, `hostA:em1` ↔ `hostC:eth0`
   - Include both direct and indirect edges (no distinction for VLAN detection)

2. **Find Connected Components**: Use BFS to group reachable node:interface pairs
   - All node:interface pairs that can reach each other form a component

3. **Verify Cliques**: Check that each component is a complete graph (clique)
   - In a true segment, ALL pairs must be mutually connected
   - Linear chains (A→B→C without A→C) are rejected

4. **Filter by Size**: Keep only components with 3+ unique nodes
   - 2-node connections are peer-to-peer links, not shared networks

5. **Label Segments**: 
   - Single interface name: Use that name (e.g., "em1")
   - Multiple names: Combined label (e.g., "br112+em1+eth0" or "mixed(7)")

6. **Collect Edge Info**: Store connection details for visualization

### Key Insight: VLAN = Connected Component + All-to-All Connectivity

A network segment (VLAN/switch) has two properties:
1. **Connectivity**: All node:interface pairs can reach each other (connected component)
2. **Completeness**: Every pair has a direct connection (clique/complete graph)

```
If B:if2 can reach A:if1, then B:if2 and A:if1 are on the same VLAN.
If 3+ node:interface pairs are all mutually connected, they form a segment.
If only 2 nodes connect (C:if4↔D:if3), it's a peer-to-peer link.
```

### Why Connected Components + Clique Verification?

Using **only** connected components would create false positives:
```
Linear: A → B → C → D
  - Forms ONE connected component
  - But A doesn't reach C or D directly
  - NOT a segment (no switch connects them all)
```

Using **only** maximal cliques would create duplicates:
```
Dense graph with 13 nodes all mutually connected
  - Can generate MANY overlapping maximal cliques
  - All represent the same VLAN
  - Need to find unique components first
```

**Solution**: Find connected components (unique VLANs), then verify each is a clique (true segment).

### Properties

- **No false positives**: Linear chains rejected (not cliques)
- **No duplicates**: Each VLAN found once (not all maximal cliques)
- **Direct = Indirect**: Both edge types contribute equally to VLAN detection
- **Overlapping support**: Nodes can have multiple interfaces on different VLANs
- **Peer-to-peer filtering**: 2-node connections excluded (3+ required)

### Threshold

- **Minimum nodes**: 3+ unique nodes (peer-to-peer links with only 2 nodes are excluded)
- **Bidirectional**: All connections must be mutual (both direct and indirect edges count)
- **Clique requirement**: ALL pairs must be mutually connected (not just reachable)
- **Global detection**: Finds all segments across the entire topology
- **No distinction**: Direct and indirect edges contribute equally to VLAN detection

### Examples of Detected Segments

**Scenario 1: Simple triangle (minimal segment)**
```
Host A (eth0) ↔ Host B (eth0)
Host B (eth0) ↔ Host C (eth0)
Host A (eth0) ↔ Host C (eth0)
Result: ONE segment with 3 nodes: A, B, C
Label: "eth0"
```

**Scenario 2: Mixed interface names (same physical switch)**
```
Host A (em1) ↔ Host B (br112)
Host A (em1) ↔ Host C (eth0)
Host B (br112) ↔ Host C (eth0)
Result: ONE segment with 3 nodes: A, B, C (all mutually connected)
Label: "br112+em1+eth0" or "mixed(3)"
```

**Scenario 3: Multiple VLANs (overlapping nodes)**
```
VLAN 1 (eth0): A, B, C all mutually connected on eth0
VLAN 2 (eth1): A, D, E all mutually connected on eth1
Result: Two separate segments
  - Segment 1: A:eth0, B:eth0, C:eth0
  - Segment 2: A:eth1, D:eth1, E:eth1
Note: A participates in both VLANs (different interfaces)
```

**Scenario 4: Not a segment (linear chain)**
```
Host A ↔ Host B
Host B ↔ Host C
Host A ↔ Host C (MISSING)
Result: NO segment detected (not a clique)
Component found but rejected (not all-to-all connected)
```

**Scenario 5: Peer-to-peer link (only 2 nodes)**
```
Host C (if4) ↔ Host D (if3)
Host D (if3) ↔ Host C (if4)
Result: NO segment detected (only 2 nodes, below threshold)
This is a direct link, not a shared network
```

**Scenario 6: Large heterogeneous VLAN**
```
13 hosts all mutually connected through a 10G switch
- Some use em1, em2, em4
- Some use eth0, eth3
- Some use p2p1, p3p1
- Some use br112 (bridges)
Result: ONE segment with 13 nodes
Label: "mixed(8)" (8 different interface names)
```

### Edge Information Preserved

Each connection from segment to member node displays:
- **Interface name**: Remote interface (may vary per host)
- **IP address**: Remote IPv6 link-local address  
- **Speed**: Link speed if available
- **RDMA device**: Device name if RDMA-capable
- **Visual distinction**: Blue dotted lines for RDMA-to-RDMA connections

## Visualization

### Segment Node Styling

- **Shape**: Ellipse (vs. box for hosts)
- **Color**: Light yellow fill (`#ffffcc`)
- **Border**: Black, thin line
- **Label Format**:
  - Line 1: `segment: <interface>` (e.g., "segment: eth0")
  - Line 2: `<count> nodes` (e.g., "4 nodes")
  - Line 3: `[RDMA]` (if all connections have RDMA)

### Connection Styling

- **Default**: Gray dotted lines with edge labels
- **RDMA-to-RDMA**: Blue dotted lines, thicker (penwidth=2.0)
- **Edge labels**: Show interface, IP, speed, RDMA device

### Edge Hiding Logic

**Edges are hidden** when:
- They connect local node to a segment member, AND
- The edge's local interface matches the segment's interface

**Edges are shown** when:
- The edge uses a different interface than the segment
- The edge connects nodes not in the segment
- The edge connects between segment members (peer-to-peer)

**Example**:
```
Segment on eth0 includes: A, B, C, D
- A:eth0 → B:eth0 : HIDDEN (through segment)
- A:eth0 → B:eth1 : HIDDEN (A's side uses segment interface)
- A:eth1 → B:eth0 : SHOWN (A's side uses different interface)
- B:eth0 → C:eth0 : SHOWN (peer-to-peer within segment)
```

### Graph Structure

```
Before (-show-segments OFF):
    host-a:eth0 ---- host-b:eth0 (fe80::2, 10G, mlx5_0)
    host-a:eth0 ---- host-c:eth0 (fe80::3, 10G, mlx5_1)  
    host-a:eth0 ---- host-d:eth0 (fe80::4, 10G, mlx5_2)
    host-a:eth1 ---- host-b:eth1 (fe80::12, 10G)

After (-show-segments ON):
         [segment: eth0]
         [4 nodes] [RDMA]
        /       |        \
    (details) (details) (details)
     host-b   host-c   host-d

    host-a:eth1 ---- host-b:eth1 (still shown - different interface)
```

**What's hidden**: Edges from local node to segment members where the edge's local interface matches the segment interface.

**What's shown**: 
- Edges using different interfaces (A:eth1→B even if segment on A:eth0)
- All edges between segment members (peer-to-peer connections)
- All edges to/from hosts not in segments
- Full edge information on segment connections

## Example Scenarios

### Scenario 1: Simple Switch

**Topology**: 4 hosts connected to same switch
```
Host A (eth0) --\
Host B (eth0) ---|--- Switch
Host C (eth0) ---|
Host D (eth0) --/
```

**Detection**: Segment detected on Host A's eth0 with 3 neighbors (B, C, D)
**Visualization**: `segment_host-a_eth0` node connects to B, C, D

### Scenario 2: Multiple Interfaces

**Topology**: Host with 2 interfaces, each with multiple neighbors
```
Host A eth0: connects to B, C, D (via switch 1)
Host A eth1: connects to E, F, G (via switch 2)
```

**Detection**: Two segments detected
- `segment_host-a_eth0` with neighbors B, C, D
- `segment_host-a_eth1` with neighbors E, F, G

### Scenario 3: Incomplete vs Complete Island

**Incomplete**: Host A sees B, C, D on eth0, but B/C/D don't all see each other
- **Label**: `segment: eth0`
- **Meaning**: Possible VLAN isolation or filtering

**Complete**: All hosts can see each other (B sees C and D, C sees D, etc.)
- **Label**: `segment: eth0 *`
- **Meaning**: True shared broadcast domain

## Implementation Details

### Data Structure

```go
type NetworkSegment struct {
    ID             string   // Unique ID: "nodeID:interface"
    Interface      string   // Interface name (e.g., "eth0")
    OwnerNodeID    string   // Node that owns this interface
    ConnectedNodes []string // Machine IDs of nodes on this segment
    IsComplete     bool     // True if complete island
}
```

### API Methods

- `Graph.GetNetworkSegments() []NetworkSegment` - Detect all segments in topology
- `export.GenerateDOTWithSegments(nodes, edges, segments)` - Generate DOT with segments

### DOT Generation

When segments are enabled:
1. Original host nodes and edges are generated normally
2. Segment nodes are added with special styling
3. Edges from segment owner to segment node
4. Edges from segment node to all neighbor nodes

## Testing

### Unit Tests (6 tests)

- `TestGetNetworkSegments_NoSegments` - Below threshold (< 3 neighbors)
- `TestGetNetworkSegments_ThreeNodes` - Exactly 3 neighbors
- `TestGetNetworkSegments_MultipleInterfaces` - Segments on different interfaces
- `TestGetNetworkSegments_BelowThreshold` - 2 neighbors (no segment)
- `TestIsCompleteIsland_EmptySet` - Edge case handling
- `TestIsCompleteIsland_IncompleteTriangle` - Incomplete connectivity

All tests validate:
- Correct segment detection threshold
- Accurate neighbor counting
- Complete island verification
- Multiple interface handling

## Configuration

### Config File

```json
{
  "show_segments": true,
  "send_interval": "30s",
  "node_timeout": "120s",
  "output_file": "topology.dot"
}
```

### CLI Override

```bash
# Enable segments (overrides config file)
lldiscovery -config config.json -show-segments

# Use config file setting
lldiscovery -config config.json
```

## Use Cases

1. **Network Topology Understanding**: Visualize which hosts share network segments
2. **VLAN Identification**: Detect broadcast domains and VLAN boundaries
3. **Switch Discovery**: Identify switch positions in logical topology
4. **Debugging**: Understand why certain hosts can/cannot communicate
5. **Capacity Planning**: Identify congestion points (many hosts on same segment)

## Limitations

1. **Threshold**: Requires 3+ neighbors to detect segment (by design)
2. **Perspective**: Each host's view may differ (asymmetric routing, filtering)
3. **Overhead**: Adds visual complexity to graphs
4. **Static**: No real switch discovery (inferred from connectivity)

## Future Enhancements

Potential improvements:
- Configurable threshold (currently hardcoded at 3)
- Per-segment traffic statistics
- Segment naming/labeling hints
- Integration with LLDP/CDP for actual switch info
- Segment-level RDMA capabilities summary

## See Also

- [README.md](README.md) - Main documentation
- [CHANGELOG.md](CHANGELOG.md) - Version history
- [NETWORK_SEGMENTS_DESIGN.md](NETWORK_SEGMENTS_DESIGN.md) - Design decisions
