# Deterministic Graph Export

**Date**: 2026-02-06  
**Change**: Made all graph export formats (DOT, nwdiag, JSON) produce deterministic, sorted output.

## Problem

Graph exports iterated over Go maps without sorting, resulting in **non-deterministic output**:
- Node order varied between runs
- Interface order changed randomly
- Edge order was unpredictable
- Segment member order inconsistent

This made it difficult to:
- Compare outputs between runs
- Track changes in version control
- Generate reproducible diagrams
- Debug topology issues

**Example**: Same topology data could produce different DOT files on each export, with nodes and edges in different orders.

## Solution

Added sorting to all map iterations in export code:

### Changes in `internal/export/dot.go`

1. **Node ordering**: Sort machine IDs before iterating
2. **Interface ordering**: Sort interface names within each node
3. **Edge ordering**: Sort source and destination machine IDs
4. **Edge list ordering**: Within each node pair, sort edges by local interface name, then remote interface name
5. **Segment EdgeInfo**: Sort node IDs when processing segment edges
6. **Interface collection**: Sort interface names when checking prefixes

This ensures that when a node pair has multiple edges (e.g., wired + WiFi, or multiple RDMA devices), the edges always appear in the same order.

### Changes in `internal/export/nwdiag.go`

1. **Edge ordering**: Sort source and destination node IDs
2. **Segment EdgeInfo**: Sort node IDs when populating interfaces

### Implementation Pattern

**Before:**
```go
for machineID, node := range nodes {
    // Process node - ORDER UNPREDICTABLE
}
```

**After:**
```go
// Sort machine IDs for deterministic output
var machineIDs []string
for machineID := range nodes {
    machineIDs = append(machineIDs, machineID)
}
sort.Strings(machineIDs)

for _, machineID := range machineIDs {
    node := nodes[machineID]
    // Process node - ORDER PREDICTABLE
}
```

**Edge list sorting:**
```go
// Sort edges within each node pair
sort.Slice(edgeList, func(i, j int) bool {
    if edgeList[i].LocalInterface != edgeList[j].LocalInterface {
        return edgeList[i].LocalInterface < edgeList[j].LocalInterface
    }
    return edgeList[i].RemoteInterface < edgeList[j].RemoteInterface
})
```

## Benefits

✅ **Deterministic output**: Same input always produces same output  
✅ **Reproducible**: Multiple runs generate identical files  
✅ **Version control friendly**: Diffs show actual topology changes, not ordering changes  
✅ **Easier debugging**: Consistent output makes comparison easier  
✅ **Better testing**: Tests can rely on consistent output order  
✅ **Documentation**: Examples in docs remain valid across runs  

## Examples

### DOT Export

**Nodes sorted by machine ID:**
```dot
subgraph cluster_30559dce... {
  label="ad\n30559dce";
  ...
}
subgraph cluster_4c8709ea... {
  label="imini\n4c8709ea";
  ...
}
subgraph cluster_a68f2602... {
  label="fork\na68f2602";
  ...
}
```

**Interfaces sorted alphabetically:**
```dot
"dd062108__docker0" [label="docker0\n..."];
"dd062108__enp0s31f6" [label="enp0s31f6\n..."];
"dd062108__veth8dc9d53" [label="veth8dc9d53\n..."];
"dd062108__wlp0s20f3" [label="wlp0s20f3\n..."];
```

**Edges sorted by node pairs AND interfaces:**
```dot
// Multiple edges between same nodes always in same order
"srv__eno1" -- "fork__eno1" [...]        // wired to wired (eno1 < wlp58s0)
"srv__wlp58s0" -- "fork__eno1" [...]     // WiFi to wired
"srv__wlp58s0" -- "fork__wlp0s20f3" [...] // WiFi to WiFi
```

### nwdiag Export

**Networks and nodes in consistent order:**
```plantuml
network 10_0_0_0_24 {
    ad [address="fe80::...", description="ad"];
    akanevsk_desk [address="fe80::...", description="akanevsk-desk"];
    fork_kad_name [address="fe80::...", description="fork.kad.name"];
    imini_v0_kad_name [address="fe80::...", description="imini.v0.kad.name"];
    srv [address="fe80::...", description="srv"];
}
```

## Performance Impact

**Negligible**: Sorting adds minimal overhead:
- Typical topology: 5-50 nodes, 10-100 interfaces
- Sorting cost: O(n log n) where n is small
- One-time cost during export
- No impact on runtime discovery or graph building

## Testing

All 78 existing tests pass with sorted output. The changes are transparent to test logic since we're only changing the order of iteration, not the data itself.

## Files Modified

- `internal/export/dot.go`: Added `sort` import, sorted all map iterations
- `internal/export/nwdiag.go`: Added `sort` import, sorted all map iterations

## Related Issues

This change complements previous fixes:
- Multi-prefix segment support
- WiFi segment visualization
- Intra-segment edge hiding

All of these features now produce consistent, comparable output across runs.
