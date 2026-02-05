# RDMA Diagnostic CLI Command - Implementation Summary

## Overview

Added `-list-rdma` CLI flag to display RDMA device information, utilizing the previously unused `GetRDMADevices()` function.

## What Was Added

### 1. CLI Flag

**File**: `cmd/lldiscovery/main.go`

Added new flag:
```go
listRDMA = flag.Bool("list-rdma", false, "list RDMA devices and their parent interfaces, then exit")
```

### 2. Command Handler

**File**: `cmd/lldiscovery/main.go`

Added `listRDMADevices()` function that:
- Calls `discovery.GetRDMADevices()` to enumerate RDMA devices
- Reads sysfs for additional info (GUIDs, node type)
- Formats output with device details and parent interfaces
- Shows IPv6 link-local addresses for parent interfaces
- Provides helpful message when no RDMA devices found

### 3. Helper Functions

**File**: `cmd/lldiscovery/main.go`

Added utility functions:
- `readSysfs(path string)` - Reads sysfs files
- `countUniqueInterfaces(devices)` - Counts unique parent interfaces

## Usage

```bash
$ ./lldiscovery -list-rdma
Found 1 RDMA device(s):

📡 rxe0
   Node GUID:      7686:e2ff:fe32:6988
   Sys Image GUID: 7686:e2ff:fe32:6988
   Node Type:      1 (CA)
   Parent interfaces:
      - enp0s31f6
        IPv6 link-local: fe80::f2e5:c0ad:dee1:44e2%enp0s31f6

Total: 1 RDMA device(s) on 1 network interface(s)
```

**When no devices found:**
```bash
$ ./lldiscovery -list-rdma
No RDMA devices found

Note: RDMA devices can be:
  - Hardware: InfiniBand, RoCE adapters
  - Software: RXE (RDMA over Converged Ethernet)

To create a software RXE device:
  sudo rdma link add rxe0 type rxe netdev eth0
```

## Information Displayed

For each RDMA device:
- **Device name** (e.g., rxe0, mlx5_0)
- **Node GUID**: Unique identifier for the RDMA port
- **Sys Image GUID**: System-wide GUID (same for all ports on multi-port adapters)
- **Node Type**: 
  - 1 = CA (Channel Adapter / Host adapter)
  - 2 = Switch
  - 3 = Router
- **Parent interfaces**: Network interfaces the RDMA device is bound to
- **IPv6 link-local addresses**: Used for discovery

## Technical Details

### Node Type Parsing

The sysfs `node_type` file can contain:
- Just the number: `1`
- Number with description: `1: CA`

The code handles both formats:
```go
parts := strings.Split(nodeType, ":")
typeNum := strings.TrimSpace(parts[0])
if len(parts) > 1 {
    typeDesc = " (" + strings.TrimSpace(parts[1]) + ")"
} else {
    // Add description based on number
    switch typeNum {
    case "1": typeDesc = " (CA - Channel Adapter)"
    // ...
    }
}
```

### Integration with GetActiveInterfaces

The command queries active interfaces to show IPv6 link-local addresses:
```go
if ifaceObj, err := discovery.GetActiveInterfaces(); err == nil {
    for _, info := range ifaceObj {
        if info.Name == iface {
            fmt.Printf("        IPv6 link-local: %s\n", info.LinkLocal)
            break
        }
    }
}
```

## Function Status Update

**Before**: `GetRDMADevices()` was unused in production code (only in tests)

**Now**: ✅ Actively used by `-list-rdma` CLI command

This demonstrates the value of keeping well-tested utility functions - they become useful when new features are added.

## Documentation Updated

1. **README.md** - Added usage example and RDMA diagnostics section
2. **CHANGELOG.md** - Added CLI diagnostic command to unreleased changes
3. **UNUSED_FUNCTIONS.md** - Updated to show `GetRDMADevices()` is now used
4. **Help output** - Automatically includes `-list-rdma` flag description

## Testing

```bash
# Build
go build -o lldiscovery cmd/lldiscovery/main.go

# Test command
./lldiscovery -list-rdma

# Verify help
./lldiscovery -help | grep list-rdma

# All tests still pass
go test ./... -cover
✅ 60 tests passing
```

## Benefits

1. **Debugging**: Quick way to verify RDMA configuration
2. **Documentation**: Shows what RDMA devices lldiscovery will use
3. **Diagnostics**: Helps troubleshoot RDMA setup issues
4. **User-friendly**: Clear output with helpful hints
5. **Validates unused function**: Demonstrates utility of `GetRDMADevices()`

## Future Enhancements

Potential additions:
- Show RDMA device state (active/down)
- Display port capabilities (link speed, MTU)
- List connected peers (if available)
- Export as JSON for scripting

## Files Modified

- `cmd/lldiscovery/main.go` - Added flag, command handler, helpers (~90 lines)
- `README.md` - Usage examples and RDMA diagnostics section
- `CHANGELOG.md` - Added to unreleased changes
- `UNUSED_FUNCTIONS.md` - Updated function status

## Lines of Code

- New code: ~90 lines (listRDMADevices + helpers)
- Documentation: ~50 lines
- **Total impact**: ~140 lines

---

**Result**: Successfully added useful diagnostic command that utilizes previously unused function, validating the decision to keep utility functions.
