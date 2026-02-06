# Segment Edge Hiding Fix

## Problem

In 19.json/19.dot, there were "spare links" (direct edges) between nodes that are both members of the same network segment. For example:

```dot
"5458415b50ff40c5a014fc490c2f05ef__em1" -- "b0c507744d734b708a2ffacf642ad1e2__br112"
```

Both of these nodes are members of `segment_0` (10.102.73.0/24 with 16 nodes). The edge is redundant because the connection is already represented by both nodes being in the same segment.

**Expected behavior**: When both nodes are in the same segment, the direct edge between them should be hidden to reduce visual clutter.

## Root Cause

The `segmentEdgeMap` in `internal/export/dot.go` was not correctly determining which interface each node uses to connect to a segment.

The previous logic at lines 83-88:

```go
// Try to get specific interface from edge info
if edgeInfo, ok := segment.EdgeInfo[nodeA]; ok {
    interfaceA = edgeInfo.RemoteInterface
}
if edgeInfo, ok := segment.EdgeInfo[nodeB]; ok {
    interfaceB = edgeInfo.RemoteInterface
}
```

This had several issues:

1. **Null RemoteInterface**: For the local/reference node in a segment, `EdgeInfo[nodeID].RemoteInterface` is often null/empty
2. **Fallback to segment.Interface**: When RemoteInterface was empty, it fell back to `segment.Interface`, causing incorrect interface matching
3. **Interface mismatch**: Edges like `node1:em1 -- node2:br112` weren't being matched because the code thought both nodes used `br112`

### EdgeInfo Structure

The `EdgeInfo` map in a segment contains edges from a "reference node" (usually the local node discovering the segment) to all other segment members:

```json
"EdgeInfo": {
  "nodeA": {
    "LocalInterface": "em1",    // Reference node's interface
    "RemoteInterface": "br112"  // nodeA's interface
  },
  "nodeB": {
    "LocalInterface": "em1",    // Reference node's interface
    "RemoteInterface": "em1"    // nodeB's interface
  },
  "localNode": {
    "LocalInterface": "",       // Empty (self)
    "RemoteInterface": ""       // Empty (self)
  }
}
```

## Solution

Implemented a two-phase algorithm to correctly identify which interface each node uses:

### Phase 1: Collect Interface Information

```go
nodeInterfaces := make(map[string]string) // nodeID -> interface name

// Collect from EdgeInfo
for nodeID, edgeInfo := range segment.EdgeInfo {
    if edgeInfo.RemoteInterface != "" {
        nodeInterfaces[nodeID] = edgeInfo.RemoteInterface
    }
}
```

### Phase 2: Handle Reference Node

The reference node (local node) doesn't have a `RemoteInterface` in its EdgeInfo entry. Find it using the `LocalInterface` that appears in other entries:

```go
// Find the reference interface
var referenceInterface string
for _, edgeInfo := range segment.EdgeInfo {
    if edgeInfo.LocalInterface != "" {
        referenceInterface = edgeInfo.LocalInterface
        break
    }
}

// Assign reference interface to nodes without interface info
if referenceInterface != "" {
    for _, nodeID := range segment.ConnectedNodes {
        if _, exists := nodeInterfaces[nodeID]; !exists {
            nodeInterfaces[nodeID] = referenceInterface
        }
    }
}
```

### Phase 3: Mark Edges for Hiding

```go
for i, nodeA := range segment.ConnectedNodes {
    for j, nodeB := range segment.ConnectedNodes {
        if i >= j {
            continue // Skip self and duplicates
        }

        interfaceA := nodeInterfaces[nodeA]
        interfaceB := nodeInterfaces[nodeB]

        // Mark this edge pair (both directions) for hiding
        segmentEdgeMap[nodeA+":"+nodeB][interfaceA] = true
        segmentEdgeMap[nodeB+":"+nodeA][interfaceB] = true
    }
}
```

## Implementation

**File**: `internal/export/dot.go`  
**Lines**: 66-137 (rewrote segment edge map construction)

### Changes

1. **Added `nodeInterfaces` map** (line 72): Tracks which interface each node uses in the segment
2. **Extract from EdgeInfo** (lines 75-86): Collect RemoteInterface for each node
3. **Identify reference interface** (lines 88-93): Find LocalInterface used by reference node
4. **Fill in missing interfaces** (lines 95-107): Assign reference interface to nodes without explicit interface info
5. **Build segmentEdgeMap** (lines 109-137): Mark edges for hiding using correct interface names

## Testing

### Test Case: 19.json

**Before fix**:
- 12 edges between `em1` and `br112` interface nodes were visible
- These were redundant (all nodes in same segment_0)

**After fix**:
- 0 edges between `em1` and `br112` interface nodes (correctly hidden)
- segment_0 still properly connected to all member interface nodes
- Visualization much cleaner

### Validation

```bash
# Check that spare edge is hidden
grep "5458415b50ff40c5a014fc490c2f05ef__em1.*b0c507744d734b708a2ffacf642ad1e2__br112" test.dot
# Returns: nothing (edge hidden)

# Verify segment connections still exist
grep "segment_0.*--" test.dot | wc -l
# Returns: 16 (one connection per segment member)
```

## Impact

- **Cleaner visualizations**: Redundant edges within segments are now hidden
- **Correct behavior**: Only inter-segment edges and peer-to-peer connections shown
- **Better scalability**: Large segments (16+ nodes) no longer create dense meshes
- **No regressions**: All 78 tests pass

## Example

### Before (19.dot - old)
```dot
// segment_0 has 16 members
"segment_0" [label="..."];
"segment_0" -- "nodeA__em1" [...]
"segment_0" -- "nodeB__br112" [...]

// REDUNDANT edges (should be hidden):
"nodeA__em1" -- "nodeB__br112" [label="fe80::..."]
"nodeA__em1" -- "nodeC__br112" [label="fe80::..."]
// ... 10 more redundant edges
```

### After (19.dot - new)
```dot
// segment_0 has 16 members
"segment_0" [label="..."];
"segment_0" -- "nodeA__em1" [...]
"segment_0" -- "nodeB__br112" [...]

// Redundant edges hidden - cleaner graph!
```

## Related Files

- `internal/export/dot.go` - DOT export with segment edge hiding
- `internal/graph/graph.go` - NetworkSegment structure with EdgeInfo
- All previous segment-related documentation still applies

## Summary

Fixed smart segment edge hiding to properly identify which interface each node uses when connecting to a segment. This eliminates redundant edge visualization when both nodes are segment members, resulting in cleaner and more scalable topology graphs.

**Status**: ✅ Complete and tested (2026-02-05)
