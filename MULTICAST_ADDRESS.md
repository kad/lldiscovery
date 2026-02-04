# Multicast Address Choice

## Why not ff02::1?

The initial plan used `ff02::1` (all-nodes multicast), but this has critical issues:

### Problems with ff02::1
1. **Reserved for ICMPv6** - This address is reserved for IPv6 Neighbor Discovery Protocol (NDP)
2. **Kernel automatically joins** - All IPv6 nodes automatically join this group
3. **System-level traffic** - Used for Router Advertisements, Neighbor Solicitations, etc.
4. **Interference risk** - Our application would receive kernel IPv6 traffic not meant for us
5. **RFC 4291 violation** - Using reserved addresses for custom applications violates standards

## Current Solution: ff02::4c4c:6469

We use **`ff02::4c4c:6469`** instead:

- **Hex encoding**: `4c4c:6469` = "LLdi" (Link-Layer Discovery)
- **Link-local scope**: `ff02::` prefix means link-local multicast (same as ff02::1 scope)
- **Unassigned**: Not in IANA's assigned IPv6 multicast addresses
- **Application-specific**: Only nodes running lldiscovery daemon join this group
- **No kernel interference**: Operating system won't handle packets to this address

### Address Breakdown
```
ff02::4c4c:6469
│││└─────────────── Address (4c4c:6469 = "LLdi" in ASCII)
││└──────────────── Scope: Link-local (2)
│└───────────────── Flags: 0 (well-known)
└────────────────── Multicast prefix (ff)
```

### Scope Meanings
- `ff01::` - Interface-local (same interface only)
- `ff02::` - **Link-local (same L2 segment)** ← We use this
- `ff05::` - Site-local (same site/campus)
- `ff0e::` - Global scope

Link-local scope is perfect for VLAN discovery because:
- Packets stay within the L2 broadcast domain
- Won't cross routers (no routing needed)
- Discovers hosts on the same VLAN/network segment
- Exactly what we need for link-layer topology mapping

## Alternative Addresses

If `ff02::4c4c:6469` conflicts with your environment, you can use:

```json
{
  "multicast_address": "ff02::9999"
}
```

Or pick any unassigned address in the `ff02::` range that doesn't conflict with:
- `ff02::1` - All nodes
- `ff02::2` - All routers  
- `ff02::1:2` - All DHCP agents
- `ff02::1:ff00:0/104` - Solicited-node addresses

## Verification

Check what multicast groups your interface has joined:

```bash
# Linux
ip -6 maddr show dev eth0

# You should see ff02::4c4c:6469 after starting lldiscovery
```

Test multicast reachability:
```bash
# Send ping to our custom group
ping6 ff02::4c4c:6469%eth0

# Should only get responses from hosts running lldiscovery
```

## References

- RFC 4291 - IPv6 Addressing Architecture
- RFC 2375 - IPv6 Multicast Address Assignments  
- IANA IPv6 Multicast Address Space Registry
