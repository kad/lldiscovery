# Segment Detection Fix: One Interface Per Segment

## Date: 2026-02-05

## Problem

In 7.dot and 7.json, the same interface from a host appeared in multiple segments:
```
"segment_0" -- "018b39dad610450eabc1d562e4efdc8c__em1"
"segment_1" -- "018b39dad610450eabc1d562e4efdc8c__em1"
```

**Analysis showed:**
- Both segments had the same 13 nodes
- Both segments had the same interfaces (em1, br112, em4, eth0, p3p1)
- The segments were duplicates or incorrectly merged

## Root Cause

The BFS-based algorithm was trying to find "global" VLANs by treating all edges transitively. When nodes had multiple interfaces on different VLANs, and interface names were asymmetric (e.g., local uses em4, remote uses em1), the BFS would merge everything into one giant component.

**Example of the problem:**
```
LocalNode:em4 connects to RemoteA:em1
RemoteA:em1 connects to RemoteB:em1
RemoteB:em1 connects to LocalNode:em1

=> BFS sees: LocalNode:em4 → LocalNode:em1 (transitively)
=> Everything merges into one component
```

## Solution

Changed from a "global VLAN detection" approach to a **"local interface grouping" approach**:

**Old Algorithm (BFS on all edges):**
1. Build connectivity graph of ALL node:interface pairs
2. Find connected components using BFS
3. Filter to 3+ nodes
4. Problem: Merges VLANs when interface names differ

**New Algorithm (Group by local interface):**
1. Look at edges FROM the local node only
2. Group remote nodes by which LOCAL interface reaches them
3. Each local interface with 2+ remote nodes = one segment
4. Interface on host can only belong to ONE segment

## Implementation

```go
// Old: Complex BFS on global connectivity graph
connectivity := make(map[string]map[string]bool)
for all edges {
    connectivity[src:iface][dst:iface] = true
}
components := BFS(connectivity)

// New: Simple grouping by local interface
interfaceGroups := make(map[string]map[string]*Edge)
for remoteID, edges := range localNode.edges {
    for edge := range edges {
        interfaceGroups[edge.LocalInterface][remoteID] = edge
    }
}
```

## Key Insight

**User's requirement:** "Multiple interfaces from the same host can go into same segment, but not one interface from the host to multiple segments."

This means:
- ✅ Host A can have em1 in VLAN1 and br112 in VLAN2 (different interfaces, different segments)
- ❌ Host A cannot have em1 in both VLAN1 and VLAN2 (same interface, multiple segments)

**Our fix:** Detect segments from the LOCAL node's perspective. Each local interface defines a segment by which remote nodes it can reach.

## Benefits

1. **Simpler algorithm**: ~30 lines vs ~150 lines
2. **Correct semantics**: One interface = one segment
3. **No duplicates**: Each local interface appears in exactly one segment
4. **Fast**: O(edges) instead of O(nodes * edges) for BFS
5. **Clear meaning**: "Segment X = nodes reachable via local interface Y"

## Testing

All 6 segment tests pass:
- ✅ No segments when < 2 remote nodes
- ✅ Detects segment with 3 neighbors
- ✅ Multiple interfaces create multiple segments
- ✅ Filters peer-to-peer links (only 1 remote node)
- ✅ Works with direct and indirect edges
- ✅ Returns empty when no local node

## Example

**Before (7.json):**
```
Segment 0: mixed(6) - 13 nodes
  018b:em1, 2524:em1, 2aaa:em1, ... (all interfaces mixed)
  
Segment 1: mixed(4) - 13 nodes  
  018b:em1, 2524:em1, 2aaa:em1, ... (duplicate!)
```

**After (expected):**
```
Segment 0: em4 - N nodes
  Nodes reachable from local em4 interface
  
Segment 1: p3p1 - M nodes
  Nodes reachable from local p3p1 interface
  
...
```

Each local interface gets its own segment listing which remote nodes are reachable.

## Files Modified

- `internal/graph/graph.go` (lines 488-557)
  - Removed: BFS connectivity graph building (~100 lines)
  - Removed: Connected components search
  - Removed: Interface name mixing/labeling logic
  - Added: Simple grouping by local interface (~30 lines)

## Migration Notes

**Behavior Change:**
- Old: Tried to detect "global" VLANs across all nodes
- New: Detects segments from local node's perspective only

**Impact:**
- Non-local nodes don't get their own segment analysis
- This is correct per original requirements (local daemon view)
- Each daemon sees its own segments independently

## Status

✅ All tests passing (75 tests)
✅ Build successful
✅ Ready for testing with real topology
