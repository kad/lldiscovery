# Debug Logging

## Overview

When running with `-log-level debug`, the daemon provides detailed information about every discovery packet sent and received. This is invaluable for troubleshooting network connectivity, understanding VLAN segmentation, and diagnosing discovery issues.

## Sent Packets

Every time a discovery packet is sent, the daemon logs:

```
level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156
```

**Fields:**
- `interface`: Local interface used to send the packet
- `source`: IPv6 link-local address with zone ID
- `size`: Packet size in bytes

## Received Packets

Every time a discovery packet is received from another host, the daemon logs:

```
level=DEBUG msg="received discovery packet" hostname=server02 machine_id=e5f6g7h8 source=fe80::10 sender_interface=eth0 received_on=eth1
```

**Fields:**
- `hostname`: The remote host's hostname
- `machine_id`: First 8 characters of the remote host's machine ID (from `/etc/machine-id`)
- `source`: Source IPv6 link-local address (without zone ID for readability)
- `sender_interface`: Which interface the **sender** used to transmit the packet
- `received_on`: Which **local** interface received the packet

## Use Cases

### 1. Verifying VLAN Connectivity

If you expect two hosts to see each other but they don't appear in the graph, check debug logs:

```bash
./lldiscovery -log-level debug 2>&1 | grep "received discovery packet"
```

If you see no logs from the expected host, the VLANs may not be connected.

### 2. Understanding Interface Mappings

The `sender_interface` and `received_on` fields help you understand which interfaces are on the same network segment:

```
received discovery packet ... sender_interface=eth0 received_on=eth0
received discovery packet ... sender_interface=eth1 received_on=eth1
```

This indicates `eth0` on both hosts are on the same VLAN, and `eth1` on both hosts are on the same VLAN.

### 3. Detecting Asymmetric Routing

If you receive packets on a different interface than expected:

```
received discovery packet ... sender_interface=eth0 received_on=eth1
```

This might indicate unexpected routing or network configuration.

### 4. Monitoring Packet Frequency

Watch for regular packet arrivals (default every 30 seconds):

```bash
./lldiscovery -log-level debug 2>&1 | grep "server02" | awk '{print $1}'
```

If packets stop arriving, the remote host may have stopped or lost network connectivity.

## Implementation Details

### Control Messages

The daemon enables IPv6 control messages on the receiving socket:

```go
p.SetControlMessage(ipv6.FlagInterface, true)
```

This allows the kernel to provide metadata about received packets, including the receiving interface index. The daemon converts the interface index to a human-readable interface name.

### Performance Impact

Debug logging has minimal performance impact:
- Logs are only written when packets are actually sent/received
- Default send interval is 30 seconds, so logs are infrequent
- No additional network traffic is generated

## Example Session

```bash
$ ./lldiscovery -log-level debug

time=2026-02-04T22:00:00.000+00:00 level=INFO msg="starting lldiscovery"
time=2026-02-04T22:00:00.005+00:00 level=INFO msg="joined multicast group" interface=eth0
time=2026-02-04T22:00:00.006+00:00 level=INFO msg="joined multicast group" interface=eth1

# Sent packets on both interfaces
time=2026-02-04T22:00:00.010+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156
time=2026-02-04T22:00:00.011+00:00 level=DEBUG msg="sent discovery packet" interface=eth1 source=fe80::2%eth1 size=156

# Received packet from server02 on eth0
time=2026-02-04T22:00:05.123+00:00 level=DEBUG msg="received discovery packet" hostname=server02 machine_id=e5f6g7h8 source=fe80::10 sender_interface=eth0 received_on=eth0

# Received packet from server03 on eth1
time=2026-02-04T22:00:07.456+00:00 level=DEBUG msg="received discovery packet" hostname=server03 machine_id=i9j0k1l2 source=fe80::20 sender_interface=eth1 received_on=eth1

# Next send cycle (30 seconds later)
time=2026-02-04T22:00:30.010+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156
time=2026-02-04T22:00:30.011+00:00 level=DEBUG msg="sent discovery packet" interface=eth1 source=fe80::2%eth1 size=156
```

## Tips

1. **Grep for specific hosts**: `./lldiscovery -log-level debug 2>&1 | grep "hostname=server02"`
2. **Watch live**: `./lldiscovery -log-level debug 2>&1 | grep --line-buffered "received discovery packet"`
3. **Count packets**: `./lldiscovery -log-level debug 2>&1 | grep -c "received discovery packet"`
4. **Filter by interface**: `./lldiscovery -log-level debug 2>&1 | grep "received_on=eth0"`
