# PlantUML nwdiag Export Enhancements

## Date
2026-02-05 23:30 (Updated: 2026-02-05 23:45)

## Updates (2026-02-05 23:45 → 23:55)

### Issues Fixed (Session 2)
1. **Excessive P2P networks**: 37 → 5 networks (fixed in 3 iterations)
   - Iteration 1: Canonical edge keys (37 → 34)
   - Iteration 2: Fallback to segment interface (34 → 20)
   - Iteration 3: Correct local node interface detection (20 → 5)

**Root Cause**: Local node interface determination was using segment.Interface as fallback, but segment.Interface represents the most common interface among segment members, not necessarily the local node's interface.

**Solution**: Extract local node interface from EdgeInfo.LocalInterface (present in all EdgeInfo entries) and use it for nodes without explicit EdgeInfo entries.

### Issues Fixed (Session 1)
1. **Segment speed calculation**: Networks showed incorrect speed (10 Gbps instead of 1 Gbps)
   - Root cause: Using maximum speed across all interfaces including bridges with inflated speeds
   - Fix: Use most common speed (mode) instead of maximum
   - Result: Segment 10.102.73.0/24 now correctly shows 1000 Mbps (12 nodes with 1G vs 3 with 10G)

2. **RDMA links missing**: Point-to-point RDMA links were not appearing in diagrams
   - Root cause: Edges between nodes in same segment were marked as processed regardless of interface
   - Fix: Track processed edges per specific interface pair, not just node pair
   - Result: RDMA links (p2p1, p3p1, p8p1) now appear as peer networks with sky blue color

### Technical Changes

#### Local Node Interface Detection (Final Fix)
**Problem**: Segment.Interface doesn't represent the local node's interface, causing local node edges not to be marked as processed.

**Before**: 
```go
// Use segment.Interface as fallback for all nodes
for _, nodeID := range segment.ConnectedNodes {
    if nodeInterfaces[nodeID] == "" {
        nodeInterfaces[nodeID] = segment.Interface
    }
}
```

**After**:
```go
// Extract local node interface from any EdgeInfo entry
var localNodeInterface string
for _, edge := range segment.EdgeInfo {
    if edge.LocalInterface != "" {
        localNodeInterface = edge.LocalInterface
        break
    }
}

// Use local node interface for nodes without EdgeInfo
for _, nodeID := range segment.ConnectedNodes {
    if nodeInterfaces[nodeID] == "" {
        if localNodeInterface != "" {
            nodeInterfaces[nodeID] = localNodeInterface
        } else {
            nodeInterfaces[nodeID] = segment.Interface
        }
    }
}
```

**Rationale**: 
- EdgeInfo.LocalInterface is consistent across all entries (represents local node's interface)
- segment.Interface is the most common interface among members (often a bridge like br112)
- Local node might use a different interface (e.g., em1) than the segment's main interface

**Result**: Local node edges correctly marked as processed, reducing P2P networks from 20 to 5.

#### Segment Speed Algorithm
**Before**: Use maximum RemoteSpeed
```go
if edge.RemoteSpeed > maxSpeed {
    maxSpeed = edge.RemoteSpeed
}
```

**After**: Use most common speed (mode)
```go
speeds := make(map[int]int) // speed -> count
for _, edge := range segment.EdgeInfo {
    if edge.RemoteSpeed > 0 {
        speeds[edge.RemoteSpeed]++
    }
}
// Find mode
for speed, count := range speeds {
    if count > maxCount {
        segmentSpeed = speed
    }
}
```

**Rationale**: Bridge interfaces often report higher speeds than underlying physical interfaces. Using the mode provides more accurate representation of actual segment speed.

#### Edge Processing for P2P Networks
**Before**: Track processed edges by node pair only
```go
edgeKey := makeEdgeKey(nodeA, nodeB)
processedEdges[edgeKey] = true
```

**After**: Track processed edges by node pair AND interface pair
```go
edgeKey := fmt.Sprintf("%s:%s:%s:%s", nodeA, nodeB, ifaceA, ifaceB)
processedEdges[edgeKey] = true
```

**Rationale**: Nodes can have multiple connections on different interfaces:
- em1/br112 → Ethernet segment (10.102.73.0/24)
- p2p1 → RDMA point-to-point link

Both should appear in the diagram.

## Problem
The initial PlantUML nwdiag export implementation (from checkpoint 012) had several limitations:
1. Node descriptions showed interface names (e.g., "em1") instead of hostnames
2. Local node had no attributes (missing description and color highlighting)
3. RDMA links were not visually distinguished
4. Point-to-point links were not represented as peer networks
5. Network speed was not included in network address field
6. Networks had no speed-based coloring

## Solution
Enhanced the `ExportNwdiag()` function in `internal/export/nwdiag.go` to provide comprehensive network visualization with all requested features.

## Changes Implemented

### 1. Hostname in Description
**Before**: `cse_r740_3 [address = "fe80::...", description = "em1"]`  
**After**: `cse_r740_3 [address = "fe80::... (em1, 1000 Mbps)", description = "cse-r740-3"]`

Interface name moved to address field, description now shows actual hostname.

### 2. Local Node Attributes
**Before**: `cse_r740_1;` (no attributes)  
**After**: `cse_r740_1 [description = "cse-r740-1", color = "#90EE90"];`

Nodes without EdgeInfo now get description and IsLocal highlighting.

### 3. RDMA Link Coloring
**Implementation**: Networks with RDMA devices colored sky blue (#87CEEB)

```plantuml
network p2p1 {
  address = "100000 Mbps"
  color = "#87CEEB"
  node1 [address = "fe80::1 (p2p1, 100000 Mbps, irdma0)", ...];
  node2 [address = "fe80::2 (p2p1, 100000 Mbps, irdma0)", ...];
}
```

RDMA device names (e.g., "irdma0") included in address field.

### 4. Point-to-Point Networks
**Implementation**: Direct edges not in segments exported as peer networks

```plantuml
network p2p_1 {
  address = "P2P (100000 Mbps, RDMA)"
  color = "#87CEEB"
  server1 [address = "fe80::1 (p2p1, 100000 Mbps, irdma0)", ...];
  server2 [address = "fe80::2 (p2p1, 100000 Mbps, irdma0)", ...];
}
```

Tracks processed edges to avoid duplicating segment connections.

### 5. Speed in Network Address
**Before**: `address = "192.168.1.0/24"`  
**After**: `address = "192.168.1.0/24 (10000 Mbps)"`

Maximum speed from all segment members included in network address.

### 6. Speed-Based Network Coloring
Color scheme based on link speed:
- **Gold** (#FFD700): 100+ Gbps
- **Orange** (#FFA500): 40-100 Gbps  
- **Light Green** (#90EE90): 10-40 Gbps
- **Light Blue** (#ADD8E6): 1-10 Gbps
- **Light Gray** (#D3D3D3): < 1 Gbps
- **Sky Blue** (#87CEEB): RDMA (overrides speed color)

## Technical Implementation

### Function Signature Change
```go
// Old
func ExportNwdiag(nodes map[string]*graph.Node, segments []graph.NetworkSegment) string

// New  
func ExportNwdiag(nodes map[string]*graph.Node, edges map[string]map[string][]*graph.Edge, segments []graph.NetworkSegment) string
```

Now takes edges parameter to generate point-to-point networks.

### New Helper Functions

1. **`makeEdgeKey(nodeA, nodeB string) string`**
   - Creates canonical order-independent edge key
   - Used for deduplication of bidirectional edges

2. **`getNetworkColor(speedMbps int, hasRDMA bool) string`**
   - Returns hex color based on speed and RDMA presence
   - RDMA takes precedence over speed-based coloring

### Edge Processing Logic

```go
// Track processed edges
processedEdges := make(map[string]bool)

// Process segments first, mark all intra-segment edges
for _, segment := range segments {
    for _, nodeA := range segment.ConnectedNodes {
        for _, nodeB := range segment.ConnectedNodes {
            edgeKey := makeEdgeKey(nodeA, nodeB)
            processedEdges[edgeKey] = true
        }
    }
}

// Then process remaining edges as P2P
for srcNodeID, dests := range edges {
    for dstNodeID, edgeList := range dests {
        edgeKey := makeEdgeKey(srcNodeID, dstNodeID)
        if !processedEdges[edgeKey] {
            // Generate peer network
        }
    }
}
```

### Node Entry Format

Complete node information in address field:
```
address = "<IPv6> (<interface>, <speed> Mbps[, <RDMA device>])"
```

Examples:
- Regular: `"fe80::1 (eth0, 1000 Mbps)"`
- RDMA: `"fe80::1 (p2p1, 100000 Mbps, irdma0)"`
- No speed: `"fe80::1 (eth0)"`

## Files Modified

1. **internal/export/nwdiag.go** (~300 lines)
   - Added edges parameter to ExportNwdiag()
   - Implemented speed calculation for networks
   - Added network coloring logic
   - Added P2P network generation
   - Enhanced address field with speed and RDMA device
   - Fixed local node attribute generation

2. **internal/export/nwdiag_test.go**
   - Updated tests to pass edges parameter
   - Tests still pass with empty edges map

3. **internal/server/server.go**
   - Updated handleGraphNwdiag() to fetch and pass edges

4. **PLANTUML_NWDIAG.md**
   - Updated with all new features
   - Added examples showing colors and P2P networks
   - Documented network color scheme

5. **CHANGELOG.md**
   - Added comprehensive feature list for nwdiag enhancements

## Testing

### Test Data
- **19.json**: 16 nodes, 6 segments, all nodes in segments
- **10.json**: Contains RDMA segment (p2p1) with 4 nodes

### Validation
```bash
# Generate nwdiag from test data
./lldiscovery server &
curl http://localhost:6469/graph.nwdiag > test.puml

# Verify features
grep 'description = "cse-r740-1"' test.puml  # ✅ Hostname in description
grep 'color = "#90EE90"' test.puml           # ✅ Local node colored
grep 'color = "#87CEEB"' test.puml           # ✅ RDMA networks colored
grep 'irdma0' test.puml                      # ✅ RDMA device shown
grep '(10000 Mbps)' test.puml                # ✅ Speed in address
grep 'network p2p_' test.puml                # ✅ P2P networks (if any)
```

### Test Results
- ✅ All 78 unit tests pass
- ✅ Descriptions show hostnames
- ✅ Local node has attributes and color
- ✅ RDMA networks colored sky blue
- ✅ Speed included in network and node addresses
- ✅ Networks colored by speed

## Example Output

### Segment Network (10 Gbps)
```plantuml
network 192_168_180_0_24 {
  address = "192.168.180.0/24 (10000 Mbps)"
  color = "#90EE90"
  cse_r740_1 [description = "cse-r740-1", color = "#90EE90"];
  cse_r740_3 [address = "fe80::e643:4bff:fe23:22ff (em4, 10000 Mbps)", description = "cse-r740-3"];
  r640_2 [address = "fe80::e643:4bff:fe23:22ff (em4, 10000 Mbps)", description = "r640-2"];
}
```

### RDMA Network (100 Gbps)
```plantuml
network p2p1 {
  address = "100000 Mbps"
  color = "#87CEEB"
  r640_2 [address = "fe80::42a6:b7ff:fe7b:a8e8 (p2p1, 100000 Mbps, irdma0)", description = "r640-2"];
  cse_r640_3 [address = "fe80::42a6:b7ff:fe7b:a680 (p2p1, 10000 Mbps, irdma0)", description = "cse-r640-3"];
}
```

## Benefits

1. **Better readability**: Hostnames in descriptions easier to understand than interface names
2. **Visual hierarchy**: Colors help identify local node and network types at a glance
3. **Complete information**: Speed and RDMA details visible without external documentation
4. **Network-centric view**: Shows how networks connect different nodes
5. **Documentation-ready**: Can be directly embedded in technical docs with PlantUML

## Integration with Existing Features

- Works with segment detection (minimum 3 nodes)
- Respects network prefix-based segment naming
- Handles multi-homed hosts (nodes on multiple networks)
- Supports mixed IPv4/IPv6 environments
- Works with RDMA edge detection

## Summary

Successfully enhanced PlantUML nwdiag export with all 6 requested features:
1. ✅ Hostname in description field
2. ✅ Local node attributes (description + color)
3. ✅ RDMA link coloring (sky blue)
4. ✅ Point-to-point peer networks
5. ✅ Speed in network address
6. ✅ Speed-based network coloring

Result: Rich, informative network diagrams with visual differentiation by speed, RDMA capability, and node role.

**Status**: ✅ Complete and tested (2026-02-05 23:30)
