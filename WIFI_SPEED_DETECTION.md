# WiFi Speed Detection via iw Tool

**Date**: 2026-02-06  
**Feature**: Actual WiFi link speed detection using the `iw` tool

## Problem

WiFi interfaces do not expose link speed through sysfs (`/sys/class/net/*/speed`), which typically returns -1, 0, or an error. The previous workaround was to assume 100 Mbps for all WiFi interfaces, which is inaccurate for modern WiFi standards:

- **WiFi 4 (802.11n)**: Up to 600 Mbps
- **WiFi 5 (802.11ac)**: Up to 3.5 Gbps  
- **WiFi 6 (802.11ax)**: Up to 9.6 Gbps

**Example from test system:**
- Actual WiFi speed: **1200 Mbps** (WiFi 6, 80MHz, HE-MCS 11, 2 spatial streams)
- Previous assumption: **100 Mbps** (12x lower!)

## Solution

Implemented automatic WiFi speed detection using the `iw` tool via nl80211 interface.

### Detection Method

1. **Try sysfs first** (`/sys/class/net/<iface>/speed`)
   - Works for wired interfaces (Ethernet)
   - Returns valid speed or fails for WiFi

2. **For WiFi interfaces, use iw tool**:
   - Check if interface name contains "wl" or "wifi"
   - Execute: `iw dev <interface> link`
   - Parse TX and RX bitrate from output
   - Return the higher of TX/RX as link speed

### iw Output Format

```
Connected to 78:45:58:d6:13:07 (on wlp0s20f3)
	SSID: Koti_867F
	freq: 5320.0
	RX: 2463140778 bytes (34970123 packets)
	TX: 711305370 bytes (2785015 packets)
	signal: -56 dBm
	rx bitrate: 960.7 MBit/s 80MHz HE-MCS 9 HE-NSS 2 HE-GI 0 HE-DCM 0
	tx bitrate: 1200.9 MBit/s 80MHz HE-MCS 11 HE-NSS 2 HE-GI 0 HE-DCM 0
```

**Extracted**: TX bitrate 1200.9 Mbps → **1200 Mbps**

### Tool Availability

The code searches for `iw` in common locations:
- `/usr/sbin/iw` (most Linux distributions)
- `/sbin/iw`
- `/usr/bin/iw`
- `/bin/iw`
- `$PATH` (via `exec.LookPath`)

If `iw` is not found, falls back to 0 (speed unknown), which triggers the 100 Mbps default in export code.

## Implementation

### File: `internal/discovery/interfaces.go`

**Added function `getWiFiSpeed(ifaceName string) int`:**
- Locates `iw` tool
- Executes `iw dev <iface> link`
- Parses `tx bitrate:` and `rx bitrate:` lines
- Returns max(TX, RX) speed in Mbps

**Modified function `getLinkSpeed(ifaceName string) int`:**
```go
func getLinkSpeed(ifaceName string) int {
    // Try sysfs first (works for wired)
    if speed := readSysfsSpeed(ifaceName); speed > 0 {
        return speed
    }
    
    // For WiFi interfaces, try iw tool
    if strings.Contains(ifaceName, "wl") || strings.Contains(ifaceName, "wifi") {
        if speed := getWiFiSpeed(ifaceName); speed > 0 {
            return speed
        }
    }
    
    return 0 // Unknown
}
```

## Testing

**Test system:**
- Interface: `wlp0s20f3`
- WiFi standard: 802.11ax (WiFi 6)
- Frequency: 5.32 GHz
- Channel width: 80 MHz
- MCS: HE-MCS 11 (TX), HE-MCS 9 (RX)
- Spatial streams: 2

**Results:**
```json
{
  "Name": "wlp0s20f3",
  "Speed": 1200,    // Correctly detected!
  "GlobalPrefixes": ["10.0.0.0/24", ...]
}
```

**Compared to wired:**
```json
{
  "Name": "enp0s31f6",
  "Speed": 1000,    // Gigabit Ethernet
  "IsRDMA": true
}
```

## Benefits

✅ **Accurate speeds**: Real WiFi link speeds instead of hardcoded 100 Mbps  
✅ **Automatic detection**: Works if `iw` tool is installed  
✅ **Graceful fallback**: Returns 0 if iw unavailable (triggers 100 Mbps default)  
✅ **Modern WiFi support**: Handles WiFi 6/6E speeds (1200+ Mbps)  
✅ **Better visualizations**: Topology diagrams show actual WiFi performance  

## Visualization Impact

### Before (assuming 100 Mbps)

```dot
segment_0 -- node__wlp0s20f3 [label="fe80::...\n100 Mbps", penwidth=1.5];
```

WiFi shown as slower than Gigabit Ethernet, even when faster.

### After (actual 1200 Mbps)

```dot
segment_0 -- node__wlp0s20f3 [label="fe80::...\n1200 Mbps", penwidth=2.0];
```

WiFi correctly shown as high-speed link, thicker line than 1 Gbps Ethernet.

## Requirements

**Optional**: `iw` tool (wireless-tools or iw package)
- Debian/Ubuntu: `apt install iw`
- RHEL/CentOS: `yum install iw`  
- Arch: `pacman -S iw`

**Fallback**: If not installed, defaults to 100 Mbps (previous behavior)

## Performance

- **Cost**: One `iw` process execution per WiFi interface during discovery startup
- **Frequency**: Only called once when interface is detected
- **Impact**: Negligible (~10ms per interface)

## Limitations

1. **Requires iw tool**: Must be installed on the system
2. **Needs active connection**: Interface must be associated with an AP
3. **Root not required**: `iw` can be run as regular user for link info
4. **Linux-specific**: Uses Linux wireless extensions (nl80211)

## Alternative Approaches Considered

1. **Pure netlink/nl80211**: Complex, requires nl80211 Go bindings
2. **iwconfig**: Deprecated, less accurate
3. **Parse /proc/net/wireless**: Doesn't include speed
4. **Hardcode by standard**: Can't detect actual negotiated rate

**Chosen approach** (`iw` tool): Simple, accurate, well-supported.

## Files Modified

- `internal/discovery/interfaces.go`:
  - Added `getWiFiSpeed()` function
  - Modified `getLinkSpeed()` to use WiFi detection
  - Added imports: `os/exec`, `strconv`

## Changelog Entry

```markdown
### Added
- **WiFi Speed Detection**: Implemented actual WiFi link speed detection using the `iw` tool. Instead of assuming 100 Mbps for all WiFi interfaces, the daemon now queries the actual TX/RX bitrate via nl80211. Modern WiFi 6 connections (1200+ Mbps) are now correctly detected and visualized. Falls back to 100 Mbps default if `iw` tool is not available. No root privileges required.
```
