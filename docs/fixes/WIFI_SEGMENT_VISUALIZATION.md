# WiFi Segment Visualization Fix

**Date**: 2026-02-06  
**Issues Fixed**: 
1. WiFi interfaces on nodes that are part of a network segment were not shown as connected to the segment in DOT visualizations
2. Intra-segment edges between WiFi and wired interfaces were not properly hidden
3. Edge hiding only worked for primary interface from EdgeInfo, missing secondary interfaces

## Problems

### Problem 1: Missing WiFi Connections to Segments

In test cluster data, several nodes have BOTH wired AND WiFi interfaces on the same network segment:

**Example from segment_0:**
- **imini**: enp1s0f0 (wired) + wlp2s0 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.
- **fork**: eno1 (wired) + wlp0s20f3 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.
- **myhost**: enp0s31f6 (wired) + wlp0s20f3 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.
- **srv**: eno1 (wired) + wlp58s0 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.

**Previous behavior:**
```
segment_0 -- node__wired_interface  ✓ (shown)
segment_0 -- node__wifi_interface   ✗ (NOT shown)
```

Result: Only 5 connections (one wired per node), missing 4 WiFi connections.

### Problem 2: WiFi Edges Not Hidden Within Segments

Even after fixing the segment visualization to show WiFi connections, the individual edges between WiFi interfaces (or WiFi-to-wired) were still being shown:

**Example from 11.dot:**
```dot
segment_0 -- srv__wlp58s0  ✓ (correctly shown)
segment_0 -- fork__eno1    ✓ (correctly shown)
segment_0 -- fork__wlp0s20f3 ✓ (correctly shown)

srv__wlp58s0 -- fork__eno1       ✗ (should be hidden - intra-segment)
srv__wlp58s0 -- fork__wlp0s20f3  ✗ (should be hidden - intra-segment)
```

Both nodes (srv and fork) are in segment_0, so ALL edges between their interfaces should be hidden.

## Root Causes

### Root Cause 1: Single Interface per Node in Segment Connections

The segment-to-node connection logic only connected ONE interface per node from EdgeInfo, ignoring secondary WiFi interfaces even when they had matching prefixes.

### Root Cause 2: Asymmetric Edge Marking

The edge hiding logic marked segment edges asymmetrically:

```go
segmentEdgeMap[key1][interfaceA] = true  // Only interfaceA
segmentEdgeMap[key2][interfaceB] = true  // Only interfaceB
```

When checking if an edge should be hidden, only `edge.LocalInterface` was checked. For an edge `srv:wlp58s0 → fork:eno1`, we need BOTH interfaces marked on the `srv:fork` key to properly hide it.

### Root Cause 3: Incomplete Interface Collection for Edge Hiding

The edge hiding logic only checked for additional interfaces on nodes WITHOUT EdgeInfo entries:

```go
if len(nodeInterfaces[nodeID]) == 0 {
    // Check node's interfaces against segment prefixes
}
```

This meant nodes with EdgeInfo (most nodes) only had their ONE interface from EdgeInfo added to the hiding map. Their secondary interfaces (WiFi) were never discovered, so edges involving those interfaces weren't marked for hiding.

## Solutions

### Solution 1: Connect All Interfaces to Segments

Modified code to connect segments to ALL interfaces with matching network prefixes:

1. Collect interfaces from EdgeInfo
2. Find additional interfaces by matching GlobalPrefixes against segment NetworkPrefixes
3. Create segment→interface edge for each matching interface
4. Default WiFi speed to 100 Mbps when Speed=0

### Solution 2: Symmetric Edge Marking

Fixed edge marking to mark BOTH interfaces on BOTH directions:

```go
// Mark BOTH interfaces on BOTH directions
segmentEdgeMap[key1][interfaceA] = true
segmentEdgeMap[key1][interfaceB] = true  // NEW
segmentEdgeMap[key2][interfaceA] = true  // NEW
segmentEdgeMap[key2][interfaceB] = true
```

This ensures any edge between segment members is properly detected and hidden, regardless of which node is source/destination.

### Solution 3: Complete Interface Collection

Fixed the interface collection to check ALL nodes for additional interfaces, not just nodes without EdgeInfo:

**Before:**
```go
for _, nodeID := range segment.ConnectedNodes {
    if len(nodeInterfaces[nodeID]) == 0 {
        // Only check if no EdgeInfo
        for ifaceName, ifaceDetails := range node.Interfaces {
            // Check prefixes...
        }
    }
}
```

**After:**
```go
for _, nodeID := range segment.ConnectedNodes {
    // ALWAYS check for additional interfaces
    for ifaceName, ifaceDetails := range node.Interfaces {
        // Skip if already in nodeInterfaces from EdgeInfo
        if alreadyAdded(ifaceName) {
            continue
        }
        // Check prefixes and add if matching...
    }
}
```

Now all nodes have ALL their segment-participating interfaces discovered and marked for hiding.

## Results

✅ **All tests passing** (78 tests)  
✅ **Multi-homed nodes properly visualized** - both wired and WiFi shown  
✅ **WiFi interfaces connected to segments** - complete topology  
✅ **Intra-segment edges hidden** - WiFi-to-wired and WiFi-to-WiFi properly hidden  
✅ **Clean topology** - segment membership visible without clutter  

**Test Data: 11.dot**

**Before:**
- 5 segment connections (wired only)
- Visible intra-segment edges: srv__wlp58s0 -- fork__eno1, etc.

**After:**
- 10 segment connections (all interfaces: 5 wired + 5 WiFi)
- Intra-segment edges properly hidden

## Files Modified

- `internal/export/dot.go`
  - Lines 296-418: Segment-to-node connections (show all interfaces)
  - Lines 143-162: Edge hiding (symmetric marking)
