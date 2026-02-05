# Example Output

## Running the Daemon

```bash
$ ./lldiscovery -log-level info
time=2026-02-04T22:00:00.000+00:00 level=INFO msg="starting lldiscovery" version=dev send_interval=30s node_timeout=2m0s export_interval=1m0s output_file=./topology.dot telemetry_enabled=false
time=2026-02-04T22:00:00.001+00:00 level=INFO msg="local node added to graph" hostname=server01.example.com interfaces=2
time=2026-02-04T22:00:00.002+00:00 level=INFO msg="starting HTTP server" address=:6469
time=2026-02-04T22:00:00.003+00:00 level=INFO msg="joined multicast group" interface=eth0
time=2026-02-04T22:00:00.003+00:00 level=INFO msg="joined multicast group" interface=eth1
```

## Discovery Packet Example

When a daemon sends a discovery packet, it looks like this:

```json
{
  "hostname": "server01.example.com",
  "machine_id": "a1b2c3d4e5f6789012345678901234567890abcd",
  "timestamp": 1738704000,
  "interface": "eth0",
  "source_ip": "fe80::1%eth0"
}
```

## HTTP API Examples

### Get Current Graph (JSON)

```bash
$ curl -s http://localhost:6469/graph | jq
{
  "a1b2c3d4e5f6789012345678901234567890abcd": {
    "Hostname": "server01.example.com",
    "MachineID": "a1b2c3d4e5f6789012345678901234567890abcd",
    "LastSeen": "2026-02-04T22:00:30Z",
    "Interfaces": {
      "eth0": "fe80::1",
      "eth1": "fe80::2"
    },
    "IsLocal": true
  },
  "e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4": {
    "Hostname": "server02.example.com",
    "MachineID": "e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4",
    "LastSeen": "2026-02-04T22:00:35Z",
    "Interfaces": {
      "eth0": "fe80::10"
    },
    "IsLocal": false
  }
}
```

### Get DOT Format

```bash
$ curl -s http://localhost:6469/graph.dot
graph lldiscovery {
  rankdir=LR;
  node [shape=box, style=rounded];

  "a1b2c3d4" [label="server01.example.com (local)\na1b2c3d4\neth0\neth1", style="rounded,filled", fillcolor="lightblue"];
  "e5f6g7h8" [label="server02.example.com\ne5f6g7h8\neth0"];
  "i9j0k1l2" [label="server03.example.com\ni9j0k1l2\nib0 [mlx5_0]\nN: 0x1234:5678:90ab:cdef\nS: 0xabcd:ef01:2345:6789"];

  "a1b2c3d4" -- "e5f6g7h8" [label="eth0 (fe80::1) <-> eth0 (fe80::10)"];
  "a1b2c3d4" -- "i9j0k1l2" [label="ib0 (fe80::2) <-> ib0 (fe80::20)\nLocal: mlx5_0 N:0x1111:2222:3333:4444 S:0xaaaa:bbbb:cccc:dddd\nRemote: mlx5_1 N:0x5555:6666:7777:8888 S:0xeeee:ffff:0000:1111"];
}
```

**Notes:** 
- The local node is highlighted with a blue background and "(local)" label
- Node labels show only interface names (and RDMA device info if present)
- IP addresses are shown on edge labels, not node labels
- Edges show which interfaces are connected with their addresses
- **RDMA information on edges**: Full details including device names and GUIDs
  - Format: `Local: mlx5_0 N:0x1111... S:0xaaaa...`
  - Shows both Node GUID (N:) and Sys Image GUID (S:) for each side
  - Makes it easy to identify specific hardware on both ends
- **Multiple edges between same hosts**: If hosts have multiple interfaces on the same VLAN, each connection creates a separate edge

### Health Check

```bash
$ curl -s http://localhost:6469/health
{"status":"ok"}
```

## Visualization Example

After running `curl http://localhost:6469/graph.dot | dot -Tpng -o topology.png`:

```
┌─────────────────────────────┐
│ server01.example.com (local)│  <- Blue background
│ a1b2c3d4                    │     (this is where daemon runs)
│ eth0                        │
│ eth1                        │
└─────────────────────────────┘
         │                 │
    eth0 (fe80::1)   eth1 (fe80::2)
         │                 │
         ↓                 ↓
┌─────────────────────────────┐   ┌─────────────────────────────┐
│ server02.example.com        │   │ server03.example.com        │
│ e5f6g7h8                    │   │ i9j0k1l2                    │
│ eth0                        │   │ ib0 [mlx5_0]                │
└─────────────────────────────┘   └─────────────────────────────┘
    eth0 (fe80::10)               ib0 (fe80::20) [mlx5_0]
```

**Interpretation:**
- server01 (local node, blue) and server02 see each other on eth0 → same VLAN
- server01 (local) and server03 see each other on ib0 (InfiniBand) → same VLAN
- server02 and server03 don't see each other → different VLANs/isolated
- **Edge labels show IP addresses** (e.g., "eth0 (fe80::1) <-> eth0 (fe80::10)")
- **Edge labels show full RDMA info** including device names and GUIDs for both sides
- **Node labels only show interface names**, not addresses
- **Interfaces without connections** are not shown in the graph

### Multi-homed Hosts Example

When hosts have multiple interfaces on the same VLAN:

```
server01 ─ib0 (fe80::1)──────ib0 (fe80::100)─ server02
  │        Local: mlx5_0 N:0x1111... S:0xaaaa...
  │        Remote: mlx5_2 N:0x9999... S:0x2222...
  │
  └───────ib1 (fe80::2)──────ib1 (fe80::101)─┘
           Local: mlx5_1 N:0x5555... S:0xeeee...
           Remote: mlx5_3 N:0xdddd... S:0x6666...
```

This creates **two separate edges** between the same pair of hosts, each showing complete RDMA information including Node GUIDs and Sys Image GUIDs for hardware identification.

## Systemd Service Logs

```bash
$ sudo journalctl -u lldiscovery -f
Feb 04 22:00:00 server01 lldiscovery[1234]: time=2026-02-04T22:00:00.000+00:00 level=INFO msg="starting lldiscovery" version=1.0.0
Feb 04 22:00:00 server01 lldiscovery[1234]: time=2026-02-04T22:00:00.001+00:00 level=INFO msg="starting HTTP server" address=:6469
Feb 04 22:00:30 server01 lldiscovery[1234]: time=2026-02-04T22:00:30.123+00:00 level=INFO msg="exported graph" nodes=3 file=/var/lib/lldiscovery/topology.dot
Feb 04 22:02:15 server01 lldiscovery[1234]: time=2026-02-04T22:02:15.456+00:00 level=INFO msg="removed expired nodes" count=1
```

## Debug Mode Example

```bash
$ ./lldiscovery -log-level debug
time=2026-02-04T22:00:00.000+00:00 level=INFO msg="starting lldiscovery" version=dev
time=2026-02-04T22:00:00.001+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156
time=2026-02-04T22:00:00.002+00:00 level=DEBUG msg="sent discovery packet" interface=eth1 source=fe80::2%eth1 size=156
time=2026-02-04T22:00:05.123+00:00 level=DEBUG msg="received discovery packet" hostname=server02.example.com machine_id=e5f6g7h8 source=fe80::10 sender_interface=eth0 received_on=eth0
time=2026-02-04T22:00:05.234+00:00 level=DEBUG msg="received discovery packet" hostname=server03.example.com machine_id=i9j0k1l2 source=fe80::20 sender_interface=eth1 received_on=eth1
time=2026-02-04T22:00:30.000+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156
```

**Debug log fields for received packets:**
- `hostname`: Remote host's hostname
- `machine_id`: First 8 characters of remote machine ID
- `source`: Source IPv6 link-local address
- `sender_interface`: Which interface the sender used to transmit
- `received_on`: Which local interface received the packet (useful for VLAN analysis)
