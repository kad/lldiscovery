# Debug Logging

## Overview

When running with `-log-level debug`, the daemon provides detailed information about every discovery packet sent and received. This is invaluable for troubleshooting network connectivity, understanding VLAN segmentation, and diagnosing discovery issues.

## Sent Packets

Every time a discovery packet is sent, the daemon logs the complete packet content:

```
level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156 content="{\"hostname\":\"server01\",\"machine_id\":\"abc123...\",\"timestamp\":1770243798,\"interface\":\"eth0\",\"source_ip\":\"fe80::1%eth0\"}"
```

**Fields:**
- `interface`: Local interface used to send the packet
- `source`: IPv6 link-local address with zone ID
- `size`: Packet size in bytes
- `content`: **Complete JSON packet content** (new in this version)

### Sent Packet with RDMA

When sending from an InfiniBand or RoCE interface:

```
level=DEBUG msg="sent discovery packet" interface=ib0 source=fe80::100%ib0 size=241 content="{\"hostname\":\"compute-01\",\"machine_id\":\"abc123...\",\"timestamp\":1770243798,\"interface\":\"ib0\",\"source_ip\":\"fe80::100%ib0\",\"rdma_device\":\"mlx5_0\",\"node_guid\":\"0x1111:2222:3333:4444\",\"sys_image_guid\":\"0xaaaa:bbbb:cccc:dddd\"}"
```

Note the larger packet size (241 bytes vs 156) and inclusion of RDMA fields.

## Received Packets

Every time a discovery packet is received from another host, the daemon logs the packet details including content:

```
level=DEBUG msg="received discovery packet" hostname=server02 machine_id=e5f6g7h8 source=fe80::10 sender_interface=eth0 received_on=eth1 content="{\"hostname\":\"server02\",\"machine_id\":\"e5f6g7h8...\",\"timestamp\":1770243800,\"interface\":\"eth0\",\"source_ip\":\"fe80::10\"}"
```

**Fields:**
- `hostname`: The remote host's hostname
- `machine_id`: First 8 characters of the remote host's machine ID (from `/etc/machine-id`)
- `source`: Source IPv6 link-local address (without zone ID for readability)
- `sender_interface`: Which interface the **sender** used to transmit the packet
- `received_on`: Which **local** interface received the packet
- `content`: **Complete JSON packet content** received from remote host (new in this version)

### Received Packet with RDMA

When receiving from a host with RDMA interfaces:

```
level=DEBUG msg="received discovery packet" hostname=compute-02 machine_id=def456ab source=fe80::200 sender_interface=ib0 received_on=ib0 content="{\"hostname\":\"compute-02\",\"machine_id\":\"def456...\",\"timestamp\":1770243805,\"interface\":\"ib0\",\"source_ip\":\"fe80::200\",\"rdma_device\":\"mlx5_1\",\"node_guid\":\"0x5555:6666:7777:8888\",\"sys_image_guid\":\"0xeeee:ffff:0000:1111\"}"
```

The `content` field shows the complete RDMA information sent by the remote host.

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

### 5. Verifying RDMA Information

Check that RDMA devices and GUIDs are being transmitted correctly:

```bash
./lldiscovery -log-level debug 2>&1 | grep rdma_device
```

You can parse the JSON content to verify specific fields:

```bash
./lldiscovery -log-level debug 2>&1 | grep "sent discovery packet" | grep -o 'content="[^"]*"' | sed 's/content="//;s/"$//' | jq '.rdma_device'
```

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
time=2026-02-04T22:00:00.006+00:00 level=INFO msg="joined multicast group" interface=ib0

# Sent packets on both interfaces (with full content)
time=2026-02-04T22:00:00.010+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156 content="{\"hostname\":\"server01\",\"machine_id\":\"abc123...\",\"timestamp\":1770243600,\"interface\":\"eth0\",\"source_ip\":\"fe80::1%eth0\"}"

time=2026-02-04T22:00:00.011+00:00 level=DEBUG msg="sent discovery packet" interface=ib0 source=fe80::100%ib0 size=241 content="{\"hostname\":\"server01\",\"machine_id\":\"abc123...\",\"timestamp\":1770243600,\"interface\":\"ib0\",\"source_ip\":\"fe80::100%ib0\",\"rdma_device\":\"mlx5_0\",\"node_guid\":\"0x1111:2222:3333:4444\",\"sys_image_guid\":\"0xaaaa:bbbb:cccc:dddd\"}"

# Received packet from server02 on eth0 (standard Ethernet)
time=2026-02-04T22:00:05.123+00:00 level=DEBUG msg="received discovery packet" hostname=server02 machine_id=e5f6g7h8 source=fe80::10 sender_interface=eth0 received_on=eth0 content="{\"hostname\":\"server02\",\"machine_id\":\"e5f6g7h8...\",\"timestamp\":1770243605,\"interface\":\"eth0\",\"source_ip\":\"fe80::10\"}"

# Received packet from compute-node on ib0 (InfiniBand with RDMA)
time=2026-02-04T22:00:07.456+00:00 level=DEBUG msg="received discovery packet" hostname=compute-node machine_id=i9j0k1l2 source=fe80::200 sender_interface=ib0 received_on=ib0 content="{\"hostname\":\"compute-node\",\"machine_id\":\"i9j0k1l2...\",\"timestamp\":1770243607,\"interface\":\"ib0\",\"source_ip\":\"fe80::200\",\"rdma_device\":\"mlx5_1\",\"node_guid\":\"0x5555:6666:7777:8888\",\"sys_image_guid\":\"0xeeee:ffff:0000:1111\"}"

# Next send cycle (30 seconds later)
time=2026-02-04T22:00:30.010+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156 content=...
```

## Tips

1. **Grep for specific hosts**: `./lldiscovery -log-level debug 2>&1 | grep "hostname=server02"`
2. **Watch live**: `./lldiscovery -log-level debug 2>&1 | grep --line-buffered "received discovery packet"`
3. **Count packets**: `./lldiscovery -log-level debug 2>&1 | grep -c "received discovery packet"`
4. **Filter by interface**: `./lldiscovery -log-level debug 2>&1 | grep "received_on=eth0"`
5. **Extract packet JSON**: `./lldiscovery -log-level debug 2>&1 | grep "sent discovery packet" | grep -o 'content="[^"]*"' | sed 's/content="//;s/"$//'`
6. **Pretty-print packets**: `./lldiscovery -log-level debug 2>&1 | grep content | grep -o 'content="[^"]*"' | sed 's/content="//;s/"$//' | jq '.'`
7. **Monitor RDMA traffic only**: `./lldiscovery -log-level debug 2>&1 | grep rdma_device`
