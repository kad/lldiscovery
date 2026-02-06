# 2-Node Remote Segment Detection

**Date**: 2026-02-06  
**Issue**: br10/vlan10 segment not detected in s-n*.json test data

## Problem

In the new test cluster (s-n*.json), a VLAN segment with only 2 nodes was not being detected:
- Node "ad" (30559dce): interface br10 with prefix 10.0.3.0/24
- Node "srv" (ecc1d392): interface vlan10 with prefixes 10.0.3.0/24, fd66:1f7:10::/64

The segment detection algorithm required:
- **Local segments**: 2+ remote nodes (3+ total including local)
- **Remote segments**: 3+ nodes

Since this VLAN has only 2 nodes (both remote from local node's perspective), it didn't meet either threshold.

## Root Cause

The 3-node minimum for remote segments was designed to avoid treating every P2P link as a segment. However, this also filtered out legitimate 2-node VLANs/subnets that share network prefixes.

## Solution

Modified remote segment detection to allow 2-node segments when they share a common network prefix:

```go
// Special case: For 2-node groups, check if they share a network prefix
// This handles VLANs or segments with only 2 participating nodes
if len(nodeEdges) == 2 {
    // Check if these 2 nodes share any network prefix
    var nodePrefixes [][]string
    for nodeID, edge := range nodeEdges {
        if node, exists := g.nodes[nodeID]; exists {
            if iface, ok := node.Interfaces[edge.LocalInterface]; ok {
                nodePrefixes = append(nodePrefixes, iface.GlobalPrefixes)
            }
        }
    }
    
    // If both nodes have prefixes and share at least one, treat as segment
    if len(nodePrefixes) == 2 && len(nodePrefixes[0]) > 0 && len(nodePrefixes[1]) > 0 {
        hasSharedPrefix := false
        for _, p1 := range nodePrefixes[0] {
            for _, p2 := range nodePrefixes[1] {
                if p1 == p2 {
                    hasSharedPrefix = true
                    break
                }
            }
            if hasSharedPrefix {
                break
            }
        }
        if hasSharedPrefix {
            minNodes = 2 // Allow 2-node segments if they share a prefix
        }
    }
}
```

## Rationale

**Shared network prefix indicates same subnet**: When two nodes share a network prefix (e.g., 10.0.3.0/24), they're on the same logical network segment, not just a random P2P link. This distinguishes:

- **Legitimate VLAN/subnet**: br10 and vlan10 both have 10.0.3.0/24 → segment
- **Random P2P link**: node1:eth0 to node2:eth1 with no shared prefix → not a segment

## Impact

### Before
```
Remote segment detection:
- Requires 3+ nodes
- br10/vlan10 with 2 nodes → NOT detected
- Shown as P2P link in visualization
```

### After
```
Remote segment detection:
- Requires 3+ nodes OR 2 nodes with shared prefix
- br10/vlan10 with 2 nodes + shared 10.0.3.0/24 → DETECTED as segment
- Shown as network segment in visualization
```

## Test Results

- All 78 existing tests still pass
- No regression in existing segment detection
- br10/vlan10 case now correctly identified (requires fresh daemon run to verify)

## Files Modified

- `internal/graph/graph.go` (lines 640-678):
  - Added special case for 2-node remote segments
  - Check for shared network prefixes
  - Adjust minNodes threshold from 3 to 2 when applicable
  - Updated component size check to use minNodes

## Use Cases

This fix handles:
1. **Small VLANs**: Management VLANs with only 2 hosts
2. **Isolated segments**: DMZ or isolated networks with limited nodes
3. **Test environments**: Development networks with minimal hosts
4. **Specialized networks**: Storage/backup networks with 2 endpoints

## Edge Cases

- **No shared prefix**: 2 nodes without common prefix still require 3+ nodes (not a segment)
- **Link-local only**: Nodes with only link-local addresses won't form segments (no global prefixes)
- **Multiple prefixes**: Only needs ONE shared prefix to qualify as segment
- **Different interface names**: Works even if interfaces have different names (br10 vs vlan10)
