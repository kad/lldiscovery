# Unused Functions Analysis

## Summary

Found **1 exported function** in `internal/discovery/interfaces.go` that is **only used in tests**, not in production code:

1. `ParseSourceAddr()` - Line 87

**Removed**: `GetMulticastAddr()` was removed (2026-02-05) as it provided no value and was only used in tests.

**Note**: `GetRDMADevices()` was previously unused but is now used by the `-list-rdma` CLI command (added 2026-02-05).

## Details

### 1. ParseSourceAddr(addr string) string

**Location**: `internal/discovery/interfaces.go:87`

```go
func ParseSourceAddr(addr string) string {
	if idx := strings.Index(addr, "%"); idx != -1 {
		addr = addr[:idx]
	}
	if strings.HasPrefix(addr, "[") {
		addr = strings.TrimPrefix(addr, "[")
		if idx := strings.Index(addr, "]"); idx != -1 {
			addr = addr[:idx]
		}
	}
	return addr
}
```

**Usage**:
- ❌ Not used in production code
- ✅ Used in tests: `internal/discovery/interfaces_test.go` (7 test cases in `TestParseSourceAddr`)

**Purpose**: Strips zone suffix and brackets from IPv6 addresses.

**Why Unused**: In `internal/discovery/receiver.go:154-156`, the code does similar parsing inline:
```go
sourceIP := remoteAddr.IP.String()
if idx := strings.Index(sourceIP, "%"); idx != -1 {
	sourceIP = sourceIP[:idx]
}
```

**Recommendation**:
- **Keep or refactor** - The inline code in receiver.go could call this function for consistency
- Alternative: Remove if we prefer the inline approach

---

### 2. GetRDMADevices() (map[string][]string, error) ✅ NOW USED

**Location**: `internal/discovery/interfaces.go:190`

```go
func GetRDMADevices() (map[string][]string, error) {
	rdmaDevices := make(map[string][]string)
	// ... implementation scans /sys/class/infiniband/ ...
	return rdmaDevices, nil
}
```

**Usage**:
- ✅ Used in production: `cmd/lldiscovery/main.go` (listRDMADevices function)
- ✅ Used in tests: `internal/discovery/interfaces_test.go` (4 times in `TestGetRDMADevices`)
- ✅ **CLI command added**: `lldiscovery -list-rdma` (2026-02-05)

**Purpose**: Returns a map of RDMA device names to their parent network interface names.

**Why Previously Unused**: The production code uses `getRDMADeviceMapping()` (line 106) which is similar but returns `map[string]string` (one device per interface). `GetRDMADevices()` returns `map[string][]string` (multiple parent interfaces per RDMA device).

**Current Status**: ✅ **Now actively used** for RDMA diagnostics via CLI

**Code comparison**:
- `getRDMADeviceMapping()` - interface → RDMA device (used internally by GetActiveInterfaces)
- `GetRDMADevices()` - RDMA device → []interfaces (used by `-list-rdma` command)

---

## Analysis

### Why These Functions Exist

These functions were likely created as part of the API design but ended up not being needed in the main code path:

1. **GetMulticastAddr** - Helper for address formatting, but address format is simple enough to do inline
2. **ParseSourceAddr** - Address parsing utility, but the parsing is simple and was done inline in receiver
3. **GetRDMADevices** - Inspection/diagnostic function for RDMA setup

### Recommendations

**Option 1: Keep All (Recommended)**
- ✅ They're well-tested (100% coverage via unit tests)
- ✅ They're useful utilities that could be used in future features
- ✅ They make good examples/documentation of functionality
- ✅ Small code size (low maintenance burden)
- ✅ Could be useful for future CLI commands or debugging tools

**Option 2: Refactor to Use Them**
- Use `ParseSourceAddr()` in `receiver.go:154` instead of inline parsing
- Use `GetMulticastAddr()` when constructing multicast addresses
- Add CLI command to use `GetRDMADevices()` for diagnostics

**Option 3: Remove Them**
- ⚠️ Would need to remove corresponding tests
- ⚠️ Loses utility functions that might be useful later
- ⚠️ Minimal benefit (small code size savings)

**Verdict

**Recommendation: Keep remaining function**

Rationale:
1. It's already tested and working
2. Small code footprint (~15 lines)
3. Could be useful for future features (e.g., CLI diagnostics, monitoring)
4. Having it in tests serves as documentation of functionality
5. No significant downside to keeping it

✅ **Update (2026-02-05)**: 
- `GetRDMADevices()` is now actively used by the `-list-rdma` CLI command
- `GetMulticastAddr()` was removed as it provided no value (simple format string)

If you want to be strict about "no unused code", then:
- Refactor `receiver.go` to use `ParseSourceAddr()`
- ✅ ~~Add a `lldiscovery list-rdma` CLI command to use `GetRDMADevices()`~~ **DONE**
- Keep `GetMulticastAddr()` as it's a simple utility

---

## All Functions Status

For reference, here's the status of all exported functions in the discovery package:

| Function | Used in Production | Used in Tests | Status |
|----------|-------------------|---------------|--------|
| `GetActiveInterfaces()` | ✅ Yes (main.go:179) | ✅ Yes | Active |
| `GetMulticastAddr()` | ❌ Removed (2026-02-05) | - | **Removed** |
| `ParseSourceAddr()` | ❌ No | ✅ Yes | **Unused** |
| `GetRDMADevices()` | ✅ Yes (main.go:listRDMADevices) | ✅ Yes | **Active** ⭐ |
| `NewReceiver()` | ✅ Yes (main.go:216) | ✅ Yes | Active |
| `NewSender()` | ✅ Yes (main.go:256) | ✅ Yes | Active |
| `NewPacket()` | ✅ Yes (sender.go:104) | ✅ Yes | Active |
| `UnmarshalPacket()` | ✅ Yes (receiver.go:135) | ✅ Yes | Active |

---

## Next Steps

If you decide to keep these functions (recommended), no action needed - they're already tested and documented.

If you want to use it in production code:
1. Refactor `receiver.go:154-156` to use `ParseSourceAddr()` for consistency

If you want to remove it:
1. Delete `ParseSourceAddr()` from `internal/discovery/interfaces.go`
2. Delete corresponding tests from `internal/discovery/interfaces_test.go`
3. Update test coverage documentation

**Decision (2026-02-05)**: Removed `GetMulticastAddr()` as it was too trivial (simple format string with no logic). Keeping `ParseSourceAddr()` as it has actual parsing logic that could be reused.
