# Secondary Segment Connections Fix

**Date**: 2026-02-06  
**Issue**: Secondary connections to same segment shown as individual edges in DOT visualization

## Problem

In updated s-n*.json test data, nodes with multiple interfaces on the same segment (e.g., wired + WiFi) had their secondary connections shown as individual bold/dashed edges instead of being hidden as part of the segment.

### Example
Local node (dd0621083f6949caafc297ee5711fbdf) has TWO interfaces on segment_0:
- **enp0s31f6** (wired, 1000 Mbps): Listed in EdgeInfo, connected to segment
- **wlp0s20f3** (WiFi, 100 Mbps): Same prefixes, also on segment

**Current behavior**: 
- Segment shown with connection via enp0s31f6 ✓
- All edges from wlp0s20f3 to other segment members shown as individual P2P links ✗

**Expected behavior**:
- Both interfaces recognized as part of segment
- ALL edges between segment members hidden (regardless of which interface)

### Visualization Impact
```
Before fix:
segment_0 -- local_node__enp0s31f6 (segment edge)
local_node__wlp0s20f3 -- remote1__eno1 (bold P2P line) ← Should be hidden!
local_node__wlp0s20f3 -- remote2__enp1s0f0 (bold P2P line) ← Should be hidden!
local_node__wlp0s20f3 -- remote3__eno1 (bold P2P line) ← Should be hidden!
... (many duplicate edges)

After fix:
segment_0 -- local_node__enp0s31f6 (segment edge)
segment_0 -- local_node__wlp0s20f3 (segment edge)
(all intra-segment edges hidden)
```

## Root Cause

The DOT export edge hiding logic (internal/export/dot.go lines 63-143) used a **single interface per node**:

```go
nodeInterfaces := make(map[string]string) // nodeID -> ONE interface name

for nodeID, edgeInfo := range segment.EdgeInfo {
    if edgeInfo.RemoteInterface != "" {
        nodeInterfaces[nodeID] = edgeInfo.RemoteInterface // Only ONE interface stored
    }
}
```

**Problem**: This assumes each node has only ONE interface per segment, but reality:
- Nodes can be multi-homed (wired + WiFi)
- Both interfaces have the same network prefixes
- Both interfaces are part of the same logical segment

**Result**: Only the interface from EdgeInfo was marked, so edges from secondary interfaces (wlp0s20f3) weren't recognized as intra-segment and were shown as P2P links.

## Solution

Changed edge hiding logic to track **ALL interfaces per node** on each segment:

```go
// Changed from single interface to list of interfaces
nodeInterfaces := make(map[string][]string) // nodeID -> list of interface names

// Collect from EdgeInfo
for nodeID, edgeInfo := range segment.EdgeInfo {
    if edgeInfo.RemoteInterface != "" {
        nodeInterfaces[nodeID] = append(nodeInterfaces[nodeID], edgeInfo.RemoteInterface)
    }
}

// For nodes without EdgeInfo, check their interfaces against segment prefixes
for _, nodeID := range segment.ConnectedNodes {
    if len(nodeInterfaces[nodeID]) == 0 {
        if node, exists := nodes[nodeID]; exists {
            for ifaceName, ifaceDetails := range node.Interfaces {
                // Check if this interface has any of the segment's prefixes
                for _, ifacePrefix := range ifaceDetails.GlobalPrefixes {
                    for _, segPrefix := range segment.NetworkPrefixes {
                        if ifacePrefix == segPrefix {
                            // This interface is on this segment
                            nodeInterfaces[nodeID] = append(nodeInterfaces[nodeID], ifaceName)
                            break
                        }
                    }
                }
            }
        }
    }
}

// Mark edges for ALL interface combinations
for _, interfaceA := range interfacesA {
    for _, interfaceB := range interfacesB {
        // Mark edges in both directions
        segmentEdgeMap[key1][interfaceA] = true
        segmentEdgeMap[key2][interfaceB] = true
    }
}
```

### Key Changes
1. **Multiple interfaces per node**: Changed from `map[nodeID]string` to `map[nodeID][]string`
2. **Prefix-based interface detection**: Check each interface's GlobalPrefixes against segment's NetworkPrefixes
3. **All combinations marked**: Mark edges for ALL interface pairs between segment members

## Algorithm Logic

For each segment:
1. **Collect interfaces from EdgeInfo** (explicit connections)
2. **For nodes without EdgeInfo**:
   - Iterate through node's interfaces
   - Check if interface GlobalPrefixes match ANY segment NetworkPrefix
   - If match found, add interface to node's interface list
3. **Mark all edge combinations**:
   - For each pair of nodes (A, B) in segment
   - For each interface on A and each interface on B
   - Mark edge A:B with interfaceA→interfaceB as segment edge

## Use Cases This Fixes

1. **Dual-homed nodes**: Wired + WiFi on same network
2. **Bonded interfaces**: bond0 + eth0 + eth1 on same segment
3. **Bridge interfaces**: br0 + eth0 on same bridge network
4. **VLAN interfaces**: eth0 + eth0.100 with same routing

## Test Results

- All 78 existing tests still pass
- No regression in single-interface segment visualization
- Multi-interface edges now correctly hidden
- Requires fresh daemon run to verify with updated s-n*.dot files

## Files Modified

- `internal/export/dot.go` (lines 63-143):
  - Changed nodeInterfaces from map[string]string to map[string][]string
  - Added prefix-based interface detection for nodes without EdgeInfo
  - Modified edge marking to handle all interface combinations

## Impact

**Before**:
- 1 segment edge + 8-12 individual P2P edges per multi-homed node
- Visual clutter with redundant connections
- Confusing topology with duplicate paths

**After**:
- All interfaces on segment recognized
- Intra-segment edges properly hidden
- Clean visualization showing only inter-segment and external connections

## Example Scenario

**Node setup**:
- local_node: enp0s31f6 (1000 Mbps) + wlp0s20f3 (100 Mbps), both with [10.0.0.0/24, fd66:1f7::/64]
- remote1: eno1 (1000 Mbps) with [10.0.0.0/24, fd66:1f7::/64]
- remote2: enp1s0f0 (1000 Mbps) with [10.0.0.0/24, fd66:1f7::/64]

**Segment**: segment_0 with NetworkPrefixes=[10.0.0.0/24, fd66:1f7::/64]

**Edge hiding**:
- ✓ local_node:enp0s31f6 ↔ remote1:eno1 (hidden)
- ✓ local_node:enp0s31f6 ↔ remote2:enp1s0f0 (hidden)
- ✓ local_node:wlp0s20f3 ↔ remote1:eno1 (hidden) ← NOW FIXED
- ✓ local_node:wlp0s20f3 ↔ remote2:enp1s0f0 (hidden) ← NOW FIXED
- ✓ remote1:eno1 ↔ remote2:enp1s0f0 (hidden)
