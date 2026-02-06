# Fix: False Positive Remote Segments (Point-to-Point Links)

## Date: 2026-02-05

## Problem

In 10.json/10.dot, segment_6 showed 4 nodes with p2p1 RDMA interfaces as a shared segment:
```
segment_6: p2p1 - 4 nodes [RDMA]
  - 2524fe12 (r640-2)
  - 2aaa7bc5 (cse-r640-3)
  - 4d02e435 (cse-r640-6)
  - f7c633a5 (cse-r640-4)
```

But these were actually **TWO separate point-to-point links**, not a shared segment:
- Link 1: 2524fe12 ↔ f7c633a5 (point-to-point RDMA)
- Link 2: 2aaa7bc5 ↔ 4d02e435 (point-to-point RDMA)

## Root Cause

The remote segment detection algorithm grouped nodes by **interface name only**:
- If nodes use the same interface name (p2p1), group them together
- If 3+ nodes use p2p1 → create segment

This was too naive! It didn't verify that the nodes were actually **connected to each other** through that interface.

**Example of the bug:**
```
Host A: p2p1 connects to Host B (direct link A-B)
Host C: p2p1 connects to Host D (direct link C-D)

Algorithm sees: A, B, C, D all use p2p1
Bug: Groups as one 4-node segment
Reality: Two separate 2-node point-to-point links
```

## Solution

Added **connectivity verification** using BFS within each interface group:

### Before (Broken)
```go
// Group all nodes using same interface name
for each interface {
    if 3+ nodes use this interface {
        create segment with ALL nodes
    }
}
```

### After (Fixed)
```go
// Group by interface name, then verify connectivity
for each interface {
    build connectivity graph for this interface only
    
    // Find connected components via BFS
    for each unvisited node {
        BFS to find all reachable nodes
        
        if component has 3+ nodes {
            create segment for this component
        }
    }
}
```

## Implementation Details

```go
// Build connectivity for this specific interface
connectivity := make(map[string]map[string]bool)
for each edge {
    if edge.LocalInterface == ifaceName && edge.RemoteInterface == ifaceName {
        connectivity[src][dst] = true  // Same interface on both sides
    }
}

// BFS to find connected components
for each starting node {
    component := BFS(starting_node, connectivity)
    
    if len(component) >= 3 {
        create segment
    }
}
```

## Key Differences

### Grouping vs Connectivity

**Grouping (wrong):**
- A:p2p1, B:p2p1, C:p2p1, D:p2p1 → "All use p2p1, make segment"
- Doesn't check if they're connected

**Connectivity (correct):**
- Check: Can A reach B, C, D on p2p1?
- If A-B connected and C-D connected (separately), create TWO components
- Point-to-point links have only 2 nodes → filtered out (< 3 threshold)

## Example: RDMA Point-to-Point Links

**Network:**
```
r640-2 :p2p1 ↔ p2p1: cse-r640-4     (100 Gbps RDMA)
cse-r640-3 :p2p1 ↔ p2p1: cse-r640-6 (10 Gbps RDMA)
```

**Before fix:**
```
segment_6: p2p1 - 4 nodes [RDMA]
  (Incorrectly grouped all 4 hosts)
```

**After fix:**
```
(No segment detected - two 2-node links are below 3-node threshold)
```

This is correct! Point-to-point links are direct connections, not shared network segments.

## Edge Cases Handled

### 1. Multiple Point-to-Point Links
- 3 separate p2p1 links with 6 hosts total
- Before: One 6-node segment (wrong)
- After: No segment (correct - all components have 2 nodes)

### 2. Mixed: Shared + Point-to-Point
- 4 hosts on vlan74 (shared VLAN)
- 2 hosts on p2p1 (point-to-point)
- Before: Would group all 6 if using same name
- After: One 4-node vlan74 segment, p2p1 filtered out

### 3. Legitimate Remote Segments
- 5 hosts all connected on vlan74
- BFS finds one 5-node component
- Creates one segment (correct)

## Testing

All existing tests pass:
- ✅ Local segments work as before
- ✅ Remote segments now verify connectivity
- ✅ Point-to-point links filtered (< 3 nodes per component)
- ✅ Legitimate shared VLANs detected correctly

## Performance

**Before:** O(edges)
**After:** O(edges + nodes * edges) for BFS

The extra BFS work is only done for remote segments (typically few), so impact is minimal.

## Expected Behavior

### 10.json Before
```
segment_5: p2p1 - 4 nodes [RDMA]
  (False positive: two 2-node links grouped)
```

### 10.json After
```
(No p2p1 segment detected)
(Point-to-point RDMA links remain as individual edges in the graph)
```

### Legitimate VLANs (Still Detected)
```
segment_2: vlan74 - 3 nodes
  (All 3 hosts can reach each other on vlan74)
  
segment_3: br177 - 5 nodes
  (All 5 hosts can reach each other on br177)
```

## Files Modified

- `internal/graph/graph.go` (lines 598-660)
  - Added connectivity graph building per interface
  - Added BFS to find connected components
  - Changed from grouping to component detection
  - Filters components with < 3 nodes

## Status

✅ All 75 tests passing
✅ Build successful
✅ Point-to-point links no longer create false segments
✅ Legitimate shared VLANs still detected correctly
