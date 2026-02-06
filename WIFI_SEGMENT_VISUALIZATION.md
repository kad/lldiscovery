# WiFi Segment Visualization Fix

**Date**: 2026-02-06  
**Issue**: WiFi interfaces on nodes that are part of a network segment were not shown as connected to the segment in DOT visualizations.

## Problem

In test cluster data (s-p*.json), several nodes have BOTH wired AND WiFi interfaces on the same network segment:

**Example from s-p1.json segment_0:**
- **imini**: enp1s0f0 (wired) + wlp2s0 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.
- **fork**: eno1 (wired) + wlp0s20f3 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.
- **akanevsk-desk**: enp0s31f6 (wired) + wlp0s20f3 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.
- **srv**: eno1 (wired) + wlp58s0 (WiFi) - both have 10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, etc.

**Current behavior in s-p1.dot:**
```
segment_0 -- node__wired_interface  ✓ (shown)
segment_0 -- node__wifi_interface   ✗ (NOT shown)
```

Result: Only 5 connections (one wired per node), missing 4 WiFi connections.

**Expected behavior:**
```
segment_0 -- node__wired_interface  ✓
segment_0 -- node__wifi_interface   ✓
```

Result: 9 connections (all interfaces on the segment).

## Root Cause

In `internal/export/dot.go`, the segment-to-node connection logic (lines 296-340) only connected ONE interface per node:

```go
for _, nodeID := range segment.ConnectedNodes {
    edge, hasEdge := segment.EdgeInfo[nodeID]
    
    if hasEdge {
        // Only connects to edge.RemoteInterface (one interface)
        ifaceNodeID := fmt.Sprintf("%s__%s", nodeID, edge.RemoteInterface)
        // Draw edge...
    }
}
```

This approach:
1. Gets ONE interface from `segment.EdgeInfo[nodeID]`
2. Connects segment only to that interface
3. Ignores secondary interfaces (WiFi) even if they have matching prefixes

## Solution

Modified the code to connect segments to ALL interfaces on each node that have matching network prefixes:

### Algorithm

```go
for each node in segment:
    1. Collect all interfaces:
       a. Start with interface from EdgeInfo (if available)
       b. Find other interfaces with matching prefixes:
          - Check node.Interfaces[ifaceName].GlobalPrefixes
          - Match against segment.NetworkPrefixes
          - Add if any prefix matches
       
    2. For each matching interface:
       a. Create edge: segment → node__interface
       b. Use EdgeInfo for labels/styling if available for this interface
       c. Otherwise use node's interface details (speed, address)
       d. Default WiFi interfaces to 100 Mbps if speed is 0
```

### Key Changes

**Before:**
- One interface per node from EdgeInfo
- WiFi interfaces ignored

**After:**
- ALL interfaces with matching prefixes
- WiFi interfaces shown with appropriate styling
- Handles nodes with 2+ interfaces on same segment

## Implementation Details

**File**: `internal/export/dot.go` (lines 296-418)

### Interface Collection

```go
// Collect all interfaces this node uses on this segment
var nodeInterfaces []string

// First, get from EdgeInfo if available
if edge, hasEdge := segment.EdgeInfo[nodeID]; hasEdge && edge.RemoteInterface != "" {
    nodeInterfaces = append(nodeInterfaces, edge.RemoteInterface)
}

// Then, find any other interfaces with matching prefixes
if node, exists := nodes[nodeID]; exists {
    for ifaceName, ifaceDetails := range node.Interfaces {
        // Skip if already added from EdgeInfo
        // ...
        
        // Check if this interface has any of the segment's prefixes
        hasMatchingPrefix := false
        for _, ifacePrefix := range ifaceDetails.GlobalPrefixes {
            for _, segPrefix := range segment.NetworkPrefixes {
                if ifacePrefix == segPrefix {
                    hasMatchingPrefix = true
                    break
                }
            }
        }
        
        if hasMatchingPrefix {
            nodeInterfaces = append(nodeInterfaces, ifaceName)
        }
    }
}
```

### Edge Generation

For each interface, the code:
1. Tries to use EdgeInfo for detailed labels (address, speed, RDMA device)
2. Falls back to node's interface details if EdgeInfo unavailable
3. Defaults WiFi (interface name contains "wl") to 100 Mbps if speed is 0
4. Uses speed-based line thickness (penwidth)
5. Colors RDMA connections blue, others gray

## Testing

**Test Data**: s-p1.json

**Before Fix:**
- 5 segment connections (one per node)
- Missing: wlp2s0, wlp0s20f3 (x2), wlp58s0

**After Fix:**
- 9 segment connections (all interfaces)
- Includes: All wired + all WiFi with matching prefixes

**Verification:**
```bash
# Expected connections:
segment_0 -- ad__eno1
segment_0 -- imini__enp1s0f0
segment_0 -- imini__wlp2s0          # NEW: WiFi
segment_0 -- fork__eno1
segment_0 -- fork__wlp0s20f3        # NEW: WiFi
segment_0 -- akanevsk-desk__enp0s31f6
segment_0 -- akanevsk-desk__wlp0s20f3  # NEW: WiFi
segment_0 -- srv__eno1
segment_0 -- srv__wlp58s0           # NEW: WiFi
```

## Results

✅ **All tests passing** (78 tests)  
✅ **Build successful**  
✅ **Multi-homed nodes properly visualized** - both wired and WiFi connections shown  
✅ **WiFi interfaces connected to segments** - no longer appear as orphaned nodes  
✅ **Cleaner topology** - all segment membership relationships visible  

## Related Changes

This fix complements the previous multi-interface edge hiding fix (SECONDARY_SEGMENT_CONNECTIONS.md):
- **Edge hiding** (previous): Hides intra-segment edges for ALL interfaces
- **Segment visualization** (this fix): Shows ALL interfaces connected to segment node

Together, these changes provide complete multi-homing support in segment visualizations.

## Files Modified

- `internal/export/dot.go` - Segment-to-node connection logic (lines 296-418)

## Changelog Entry

```markdown
### Fixed
- **WiFi segment visualization**: Segments now connect to ALL node interfaces with matching prefixes, not just the primary wired interface. Nodes with both wired and WiFi on same segment now show both connections in DOT output.
```
