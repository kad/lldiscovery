# Network Prefix-Based Segment Naming

**Date**: 2026-02-05  
**Updated**: 2026-02-05 (Added IPv4 support)

## Overview

The daemon now collects network prefixes (both IPv4 and IPv6) from all interfaces and uses them to provide intelligent, consistent naming for network segments in topology visualizations.

## Problem Statement

### Before: Interface-Based Naming

Previously, network segments were labeled with the interface name from the local host's perspective:

```
segment: eth0
4 nodes
```

**Issues:**
1. **Inconsistent across hosts**: Same VLAN might be `eth0` on host A, `em1` on host B, and `p2p1` on host C
2. **No network context**: Doesn't show which subnet/VLAN the segment represents
3. **Configuration ambiguity**: Hard to correlate with network documentation
4. **Debugging difficulty**: Must cross-reference interface names with network diagrams

### After: Prefix-Based Naming

Now segments display the network prefix when global addresses are configured:

**IPv4 example:**
```
192.168.1.0/24
4 nodes
(eth0)
```

**IPv6 example:**
```
2001:db8:100::/64
4 nodes
(eth0)
```

**Dual-stack example (most common prefix shown):**
```
192.168.1.0/24
4 nodes
(eth0)
```

**Benefits:**
1. **Consistent everywhere**: All hosts label the same VLAN identically
2. **Self-documenting**: Network prefix immediately shows the subnet
3. **Configuration clarity**: Easy to match with network design docs
4. **Debugging ease**: Spot misconfigured networks instantly
5. **Dual-stack support**: Works with IPv4-only, IPv6-only, or dual-stack networks

## Implementation

### Data Collection

The daemon collects both IPv4 and IPv6 global addresses during interface discovery:

```go
// In GetActiveInterfaces()
for _, addr := range addrs {
    ip := ipNet.IP
    
    // Handle IPv4 addresses
    if ip4 := ip.To4(); ip4 != nil {
        // Skip loopback (127.0.0.0/8)
        if ip4[0] == 127 {
            continue
        }
        // Skip link-local (169.254.0.0/16)
        if ip4[0] == 169 && ip4[1] == 254 {
            continue
        }
        // Collect IPv4 prefix
        prefix := fmt.Sprintf("%s/%d", 
            ipNet.IP.Mask(ipNet.Mask).String(), 
            getPrefixLength(ipNet.Mask))
        globalPrefixes = append(globalPrefixes, prefix)
        continue
    }
    
    // Handle IPv6 addresses
    if ip.IsGlobalUnicast() {
        // Extract IPv6 network prefix
        prefix := fmt.Sprintf("%s/%d", 
            ipNet.IP.Mask(ipNet.Mask).String(), 
            getPrefixLength(ipNet.Mask))
        globalPrefixes = append(globalPrefixes, prefix)
    }
}
```

**Collected information:**
- **IPv4**: All non-loopback, non-link-local addresses
  - Typical: 192.168.x.0/24, 10.x.x.0/16, 172.16-31.x.0/16
  - Excludes: 127.0.0.0/8 (loopback), 169.254.0.0/16 (link-local)
- **IPv6**: Global unicast addresses only (not link-local)
  - Typical: 2001:db8::/32, fc00::/7 (ULA), other global addresses
  - Excludes: fe80::/10 (link-local)
- Network prefix with CIDR notation (e.g., `/24`, `/64`)
- Multiple prefixes per interface supported (dual-stack)
- Deduplicated per interface
    ip := ipNet.IP
    if ip.IsGlobalUnicast() {
        // Extract network prefix (e.g., "2001:db8:100::/64")
        prefix := fmt.Sprintf("%s/%d", 
            ipNet.IP.Mask(ipNet.Mask).String(), 
            getPrefixLength(ipNet.Mask))
        globalPrefixes = append(globalPrefixes, prefix)
    }
}
```

**Collected information:**
- IPv6 global unicast addresses only (not link-local)
- Network prefix with CIDR notation (e.g., `/64`, `/48`)
- Multiple prefixes per interface supported
- Deduplicated per interface

### Data Propagation

Prefixes flow through the entire discovery protocol:

1. **InterfaceInfo** → Stored when interface detected
2. **Packet** → Sent in discovery multicast
3. **NeighborInfo** → Shared when transitive discovery enabled
4. **InterfaceDetails** → Stored in graph per node/interface
5. **NetworkSegment** → Associated with segment during detection

**Protocol additions:**
```json
{
  "interface": "eth0",
  "source_ip": "fe80::1%eth0",
  "global_prefixes": ["2001:db8:100::/64", "2001:db8:200::/64"],
  "neighbors": [
    {
      "local_prefixes": ["2001:db8:100::/64"],
      "remote_prefixes": ["2001:db8:100::/64"]
    }
  ]
}
```

### Segment Detection

During network segment detection, the daemon:

1. **Collects prefixes** from all segment members
2. **Counts occurrences** of each unique prefix
3. **Selects most common** as the segment's network prefix
4. **Falls back** to interface name if no prefixes found

**Algorithm (`getMostCommonPrefix()`):**

```go
func (g *Graph) getMostCommonPrefix(nodeIDs []string, edgeInfo map[string]*Edge) string {
    prefixCount := make(map[string]int)
    
    // Collect prefixes from each node's interface
    for _, nodeID := range nodeIDs {
        prefixes := getNodeInterfacePrefixes(nodeID, edgeInfo)
        for _, prefix := range prefixes {
            prefixCount[prefix]++
        }
    }
    
    // Find most common prefix
    maxCount := 0
    mostCommon := ""
    for prefix, count := range prefixCount {
        if count > maxCount {
            maxCount = count
            mostCommon = prefix
        }
    }
    
    return mostCommon
}
```

**Why most common?**
- Handles misconfigurations gracefully (one node has wrong subnet)
- Works when not all nodes have global addresses
- Produces consistent results across all hosts
- In dual-stack environments, selects whichever prefix (IPv4 or IPv6) is most common

### Visualization

DOT export shows prefix-based segment labels:

```graphviz
// IPv4 prefix
"segment_0" [label="192.168.1.0/24\n4 nodes\n(eth0)", 
    shape=ellipse, style=filled, fillcolor="#ffffcc"];

// IPv6 prefix
"segment_1" [label="2001:db8:100::/64\n4 nodes\n(eth0)", 
    shape=ellipse, style=filled, fillcolor="#ffffcc"];

// Without global addresses (fallback)
"segment_2" [label="segment: eth1\n3 nodes", 
    shape=ellipse, style=filled, fillcolor="#ffffcc"];

// With RDMA
"segment_3" [label="10.1.0.0/16\n5 nodes\n(ib0)\n[RDMA]",
    shape=ellipse, style=filled, fillcolor="#ffffcc"];
```

**Label format:**
- **Line 1**: Network prefix (IPv4 or IPv6) or "segment: interface" if no prefix
- **Line 2**: Node count
- **Line 3**: Interface name in parentheses (only if prefix shown)
- **Line 4**: `[RDMA]` indicator if applicable

## Configuration

No configuration required - feature works automatically when:

1. **Global addresses configured** on interfaces (IPv4 and/or IPv6)
2. **Proper subnet masks** set (e.g., `/24` for IPv4, `/64` for IPv6)
3. **Same prefix used** across all hosts on the VLAN

## Use Cases

### Use Case 1: Traditional IPv4 Data Center

**Network design:**
- VLAN 10: `10.0.10.0/24` (management)
- VLAN 20: `10.0.20.0/24` (storage)
- VLAN 30: `10.0.30.0/24` (compute)

**Visualization:**
```
    ┌─────────────────────────┐
    │  10.0.10.0/24           │
    │  15 nodes               │
    │  (em1)                  │
    └────────┬────────────────┘
             │
     ┌───────┼───────┐
     │       │       │
  [Host1] [Host2] [Host3] ...
     │       │       │
     └───────┼───────┘
             │
    ┌────────┴────────────────┐
    │  10.0.20.0/24           │
    │  15 nodes               │
    │  (em2)                  │
    └─────────────────────────┘
```

**Benefits:**
- Works with existing IPv4-only infrastructure
- No IPv6 required for intelligent naming
- Clear subnet identification

### Use Case 2: Dual-Stack Enterprise Network

**Network design (both IPv4 and IPv6):**
- VLAN 100: `192.168.100.0/24` + `2001:db8:100::/64` (management)
- VLAN 200: `192.168.200.0/24` + `2001:db8:200::/64` (storage)
- VLAN 300: `192.168.300.0/24` + `2001:db8:300::/64` (compute)

**Visualization:**
```
    ┌─────────────────────────┐
    │  192.168.100.0/24       │
    │  15 nodes               │
    │  (em1)                  │
    └────────┬────────────────┘
             │
      [Dual-stack hosts]
             │
    ┌────────┴────────────────┐
    │  192.168.200.0/24       │
    │  15 nodes               │
    │  (em2)                  │
    │  [RDMA]                 │
    └─────────────────────────┘
```

**Note:** Most common prefix shown (typically IPv4 in dual-stack environments)

### Use Case 3: IPv6-Only Data Center

**Network design:**
- VLAN 100: `2001:db8:100::/64` (management)
- VLAN 200: `2001:db8:200::/64` (storage)
- VLAN 300: `2001:db8:300::/64` (compute)

**Visualization:**
```
    ┌─────────────────────────┐
    │  2001:db8:100::/64      │
    │  15 nodes               │
    │  (em1)                  │
    └────────┬────────────────┘
             │
     ┌───────┼───────┐
     │       │       │
  [Host1] [Host2] [Host3] ...
     │       │       │
     └───────┼───────┘
             │
    ┌────────┴────────────────┐
    │  2001:db8:200::/64      │
    │  15 nodes               │
    │  (em2)                  │
    │  [RDMA]                 │
    └─────────────────────────┘
```

**Benefits:**
- Modern IPv6-first infrastructure
- Globally unique addressing
- No NAT complexities

### Use Case 4: Cloud Overlay Networks

**AWS/Azure environment** with multiple subnets:
- Subnet A: `fd00:ec2:100::/48`
- Subnet B: `fd00:ec2:200::/48`
- Subnet C: `fd00:ec2:300::/48`

**Visualization shows:**
- Cloud network segmentation
- Which instances are in which subnet
- Overlay network connectivity

### Use Case 3: Kubernetes Pod Networks

**Pod networks** with IPv6:
- Node 1 pods: `2001:db8:a001::/64`
- Node 2 pods: `2001:db8:a002::/64`
- Node 3 pods: `2001:db8:a003::/64`

**Visualization reveals:**
- Pod network allocation per node
- Cross-node pod communication paths
- Network policy enforcement boundaries

### Use Case 4: Lab Environment

**Development lab** with test networks:
- Production mirror: `2001:db8:prod::/48`
- Staging: `2001:db8:stage::/48`
- Development: `2001:db8:dev::/48`

**Benefits:**
- Clear separation of environments in topology
- Verify isolation between prod/stage/dev
- Document network architecture automatically

## Troubleshooting

### Segment shows interface name instead of prefix

**Cause**: No global unicast addresses configured on interfaces

**Solution**:
```bash
# Verify global addresses
ip -6 addr show dev eth0 | grep "scope global"

# If missing, configure (example)
sudo ip -6 addr add 2001:db8:100::10/64 dev eth0

# Or via NetworkManager/netplan/systemd-networkd
```

### Different hosts show different prefixes

**Cause**: Misconfigured network - hosts have different subnets on same VLAN

**Symptom**:
```
# Host A sees:
2001:db8:100::/64
4 nodes
(eth0)

# Host B sees:
2001:db8:200::/64
4 nodes
(eth0)
```

**Diagnosis**:
- Check routing/switching configuration
- Verify VLAN assignments
- Inspect DHCP/SLAAC configuration

**Solution**: Standardize network prefix across all hosts in the segment

### Prefix includes host bits

**Cause**: Subnet mask incorrectly configured

**Example**: `2001:db8:100::10/64` should show as `2001:db8:100::/64`

**How it works**: Daemon applies mask before display:
```go
prefix := fmt.Sprintf("%s/%d", 
    ipNet.IP.Mask(ipNet.Mask).String(),  // Masks out host bits
    getPrefixLength(ipNet.Mask))
```

### Multiple prefixes on same interface

**Cause**: Multiple subnets configured (valid configuration)

**Behavior**: Daemon selects most common prefix among segment members

**Example**:
- Interface has: `2001:db8:100::/64` and `2001:db8:200::/64`
- 5 nodes have `2001:db8:100::/64`
- 2 nodes have `2001:db8:200::/64`
- Segment labeled: `2001:db8:100::/64` (most common)

## Technical Details

### Data Structures

```go
// InterfaceInfo - collected during interface discovery
type InterfaceInfo struct {
    Name           string
    LinkLocal      string
    GlobalPrefixes []string  // NEW: network prefixes
    IsRDMA         bool
    Speed          int
}

// NetworkSegment - used in graph
type NetworkSegment struct {
    ID             string
    Interface      string
    NetworkPrefix  string    // NEW: chosen prefix or empty
    ConnectedNodes []string
    EdgeInfo       map[string]*Edge
}
```

### Protocol Compatibility

**Backward compatible**: Old daemons ignore unknown JSON fields

**Forward compatible**: New daemons handle missing prefix fields (nil/empty)

**Mixed environments**: Works correctly when some hosts lack global addresses

### Performance

**Impact**: Minimal
- Collection: O(interfaces × addresses) during startup
- Propagation: ~50-100 bytes per interface in packets
- Selection: O(nodes × prefixes) during segment detection (typically < 1ms)

**Memory**: ~100 bytes per interface for prefix storage

## Future Enhancements

Possible improvements:

1. **IPv4 support**: Collect IPv4 subnets as fallback
2. **Prefix filtering**: Ignore ULA/private prefixes via config
3. **Custom naming**: User-defined labels per prefix
4. **Hierarchy**: Show prefix relationships (/48 → /64 → /80)
5. **Change detection**: Alert when network prefixes change

## Related Documentation

- `README.md` - General daemon documentation
- `NETWORK_SEGMENTS.md` - Segment detection algorithm
- `VISUALIZATION_IMPROVEMENTS.md` - DOT export enhancements
- `TRANSITIVE_DISCOVERY.md` - Neighbor sharing protocol

## References

- RFC 4291 - IPv6 Addressing Architecture
- RFC 4193 - Unique Local IPv6 Unicast Addresses
- RFC 5952 - IPv6 Address Text Representation
