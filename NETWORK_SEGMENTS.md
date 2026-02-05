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

### Detection Algorithm

1. **Group by Interface**: For each host, group all neighbors by local interface
2. **Apply Threshold**: If 3+ neighbors exist on the same interface, create a segment
3. **Verify Completeness**: Check if all neighbors can see each other (complete island)
4. **Generate Visualization**: Create segment nodes in DOT graph

### Threshold

- **Minimum neighbors**: 3+ (prevents creating segments for simple P2P connections)
- **Includes both**: Direct edges (discovered locally) and indirect edges (learned from neighbors)

### Island Verification

A segment is marked as a **complete island** when:
- All neighbors can see each other (full mesh connectivity)
- Indicates a true shared broadcast domain where all hosts are mutually reachable

## Visualization

### Segment Node Styling

- **Shape**: Ellipse (vs. box for hosts)
- **Color**: Light yellow fill (`#ffffcc`)
- **Border**: Black, thin line
- **Label Format**:
  - `segment: <interface>` - incomplete island
  - `segment: <interface> *` - complete island (marked with asterisk)

### Graph Structure

```
Before (-show-segments OFF):
    host-a ---- host-b
    host-a ---- host-c  
    host-a ---- host-d

After (-show-segments ON):
         segment_host-a_eth0
        /       |        \
    host-b   host-c   host-d
       |
    host-a
```

The segment node represents the shared network (switch/VLAN) connecting all hosts on that interface.

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
