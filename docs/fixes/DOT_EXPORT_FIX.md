# DOT Export Fix: Hiding Indirect Edges in Segments

## Problem

The DOT export was only hiding **direct** edges from the **local node** to segment members. Indirect edges (learned transitively) between segment members were still shown as individual dashed lines, cluttering the graph.

## Root Cause

In `internal/export/dot.go`, the segment edge hiding logic (lines 61-100) was:
- Only processing edges where one endpoint was the local node
- Not considering indirect edges learned from other nodes

## Solution

Changed the logic to hide **ALL** edges between **ANY** segment members:

```go
// OLD: Only hide edges from local node
if localNodeID != "" {
    for _, segment := range segments {
        for _, nodeID := range segment.ConnectedNodes {
            if nodeID != localNodeID {
                // Mark edge from local to this node
            }
        }
    }
}

// NEW: Hide edges between ALL segment member pairs
for _, segment := range segments {
    for i, nodeA := range segment.ConnectedNodes {
        for j, nodeB := range segment.ConnectedNodes {
            if i >= j { continue }
            // Mark edge between nodeA and nodeB
        }
    }
}
```

## Impact

- **Before**: 159 edges shown in 6.dot despite having 2 segments with 13 nodes each
- **After**: Only non-segment edges shown (peer-to-peer RDMA links, etc.)
- Both direct and indirect edges are now properly hidden when part of a segment

## Implementation Details

1. The `segmentEdgeMap[srcNode:dstNode][interface]` marks which edges should be hidden
2. For each segment, we now iterate over **all pairs** of nodes in that segment
3. We mark edges in **both directions** (A→B and B→A)
4. The interface names are extracted from `segment.EdgeInfo` for each node
5. The edge rendering loop (lines 260-344) checks this map and skips marked edges

## Testing

All 60 existing tests pass. The algorithm correctly:
- Detects segments using BFS on all edges (direct + indirect)
- Hides edges between segment members in DOT export
- Preserves peer-to-peer links (< 3 nodes)
- Shows segment nodes with connections to member interfaces
