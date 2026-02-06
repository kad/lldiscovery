# Multi-Prefix Network Segment Detection - Implementation Summary

**Date**: 2026-02-06
**Status**: ✅ Implementation Complete, Awaiting Live Testing

## Problem Analysis

Test cluster data (s-*.json files with 1-second intervals) revealed three critical issues:

### Issue 1: Duplicate Segments from Multi-Interface Nodes
**Example**: Node `dd0621083f6949caafc297ee5711fbdf` has both:
- `enp0s31f6` (wired, 1000 Mbps): 4 prefixes (10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, ...)
- `wlp0s20f3` (WiFi, 0 Mbps): SAME 4 prefixes

**Result**: Algorithm created 3 segments for the same 5 nodes:
- segment_0: fd66:cd1e:5ac2:9f5d::/64 (enp0s31f6)
- segment_1: 10.0.0.0/24 (wlp0s20f3) ← DUPLICATE!
- segment_2: 2001:470:28:3e6::/64 (eno1)

### Issue 2: WiFi Interface Speed = 0
All WiFi interfaces (wlp0s20f3, wlp2s0, wlp58s0) reported Speed=0, needed default of 100 Mbps.

### Issue 3: Single Prefix Per Segment
Each segment showed only ONE prefix (selected via "most common"), but physical networks have MULTIPLE prefixes (IPv4 + multiple IPv6).

## Solution Implemented

### 1. NetworkSegment Structure Change
```go
type NetworkSegment struct {
    ID              string           // Unique ID
    Interface       string           // Primary interface (highest speed)
    NetworkPrefix   string           // First prefix (backward compatibility)
    NetworkPrefixes []string         // ALL prefixes (NEW!)
    ConnectedNodes  []string         // Machine IDs
    EdgeInfo        map[string]*Edge // Edge info
}
```

### 2. New Functions

**getAllPrefixes()** (`internal/graph/graph.go:744`)
- Collects ALL unique IPv4 and IPv6 prefixes from segment nodes
- Returns sorted list of prefixes
- Used during segment creation

**getEffectiveSpeed()** (`internal/graph/graph.go:732`, `internal/export/nwdiag.go:326`)
- Defaults WiFi interfaces (containing "wl") to 100 Mbps when Speed=0
- Applied in nwdiag speed calculations and interface selection

**mergeSegmentsByNodeSet()** (`internal/graph/graph.go:975`)
- Merges segments with IDENTICAL ConnectedNodes sets
- Aggregates all prefixes from merged segments
- Selects primary interface based on:
  1. Highest effective speed
  2. Prefers wired (not "wl*") over WiFi when speeds equal
- Merges EdgeInfo (prefers edges with more information)
- Result: One segment per physical network

### 3. Modified Functions

**GetNetworkSegments()** (`internal/graph/graph.go:521`)
- Now calls `mergeSegmentsByNodeSet()` after `mergeSegmentsByPrefix()`
- Two-stage merging:
  1. Merge segments with same prefix (existing)
  2. Merge segments with same nodes (new)

**mergeSegmentsByPrefix()** (`internal/graph/graph.go:846`)
- Updated to aggregate NetworkPrefixes from merged segments
- Maintains backward compatibility with NetworkPrefix field

### 4. Export Updates

**DOT Export** (`internal/export/dot.go:238`)
- Shows first 3 prefixes, adds "..." if more
- Example: `10.0.0.0/24\nfd66:cd1e:5ac2:9f5d::/64\nfd66:1f7::/64\n5 nodes\n(enp0s31f6)`

**PlantUML nwdiag Export** (`internal/export/nwdiag.go:22`)
- Shows all prefixes comma-separated with speed
- Uses getEffectiveSpeed() for WiFi interfaces
- Example: `address = "10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, ... (1000 Mbps)"`

**JSON Export** (automatic via struct marshaling)
- Includes both NetworkPrefix (first) and NetworkPrefixes (all)
- Backward compatible with old clients using NetworkPrefix

## Code Changes

### Files Modified
1. `internal/graph/graph.go`:
   - Added `strings` import
   - Modified NetworkSegment struct
   - Added getAllPrefixes() (lines 744-796)
   - Added getEffectiveSpeed() (lines 732-742)
   - Added mergeSegmentsByNodeSet() (lines 975-1137)
   - Updated segment creation (2 places)
   - Updated mergeSegmentsByPrefix()

2. `internal/export/dot.go`:
   - Updated segment label generation (lines 238-260)

3. `internal/export/nwdiag.go`:
   - Added getEffectiveSpeed() (lines 326-336)
   - Updated network name/address handling (lines 22-70)
   - Uses effective speed in speed calculation

### Test Results
- ✅ All 78 existing tests pass
- ✅ `go build ./...` successful
- ✅ Binary builds without errors

## Expected Behavior

**Before** (s-3.json example):
```
3 segments detected:
- segment_0: fd66:cd1e:5ac2:9f5d::/64, enp0s31f6, 5 nodes
- segment_1: 10.0.0.0/24, wlp0s20f3, 5 nodes (SAME nodes!)
- segment_2: 2001:470:28:3e6::/64, eno1, 3 nodes
```

**After** (expected with fresh daemon run):
```
2 segments detected:
- segment_0: [10.0.0.0/24, fd66:cd1e:5ac2:9f5d::/64, fd66:1f7::/64, 2001:470:28:3e6::/64], enp0s31f6, 5 nodes
- segment_1: [2001:470:28:3e6::/64, fd66:1f7::/64], eno1, 3 nodes
```

Key differences:
1. First segment merged (was 2-3 separate segments)
2. Shows ALL prefixes per segment
3. Primary interface selected (enp0s31f6 over wlp0s20f3 due to higher speed)
4. WiFi speed 0 treated as 100 Mbps

## Testing Checklist

### Live Testing Required
- [ ] Start daemon on multi-interface hosts (wired + WiFi)
- [ ] Verify segments merge correctly (same nodes → 1 segment)
- [ ] Check WiFi interfaces show 100 Mbps instead of 0
- [ ] Verify all IPv4/IPv6 prefixes appear in segment
- [ ] Check DOT visualization shows multiple prefixes
- [ ] Check nwdiag shows all prefixes in address
- [ ] Verify JSON exports both fields correctly

### Test Scenarios
1. **Multi-interface node**: Host with eth0 + wlan0 on same network
2. **Multi-prefix network**: Network with IPv4 + ULA + GUA
3. **WiFi-only node**: Node with only WiFi interface (Speed=0)
4. **Mixed speeds**: Nodes with 1G wired + WiFi on same segment

## Known Limitations

1. **Existing JSON files**: s-*.json files don't have NetworkPrefixes (generated with old code)
   - Solution: Fresh daemon run will populate correctly
   
2. **Backward compatibility**: Old clients may only read NetworkPrefix field
   - Mitigation: NetworkPrefix kept as first prefix for compatibility

3. **Interface selection**: Heuristic (speed + wired preference) may not always pick "best"
   - Acceptable: Generally prefers faster, more stable interfaces

## Documentation Updates Needed

- [ ] Update NETWORK_SEGMENTS.md with multi-prefix behavior
- [ ] Add example showing merged segments
- [ ] Document WiFi speed defaulting
- [ ] Update API docs for NetworkPrefixes field

## Deployment Notes

- **Backward compatible**: Old code can read new JSON (ignores NetworkPrefixes)
- **Forward compatible**: New code reads old JSON (derives from NetworkPrefix)
- **No migration needed**: Segments recomputed on daemon start
- **Breaking change**: Only for code directly accessing NetworkSegment.NetworkPrefix (should use NetworkPrefixes now)
