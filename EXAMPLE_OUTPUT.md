# Example Output

## Running the Daemon

```bash
$ ./lldiscovery -log-level info
time=2026-02-04T22:00:00.000+00:00 level=INFO msg="starting lldiscovery" version=dev send_interval=30s node_timeout=2m0s export_interval=1m0s
time=2026-02-04T22:00:00.001+00:00 level=INFO msg="starting HTTP server" address=:8080
time=2026-02-04T22:00:00.002+00:00 level=INFO msg="joined multicast group" interface=eth0
time=2026-02-04T22:00:00.002+00:00 level=INFO msg="joined multicast group" interface=eth1
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
$ curl -s http://localhost:8080/graph | jq
{
  "a1b2c3d4e5f6789012345678901234567890abcd": {
    "Hostname": "server01.example.com",
    "MachineID": "a1b2c3d4e5f6789012345678901234567890abcd",
    "LastSeen": "2026-02-04T22:00:30Z",
    "Interfaces": {
      "eth0": "fe80::1",
      "eth1": "fe80::2"
    }
  },
  "e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4": {
    "Hostname": "server02.example.com",
    "MachineID": "e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4",
    "LastSeen": "2026-02-04T22:00:35Z",
    "Interfaces": {
      "eth0": "fe80::10"
    }
  }
}
```

### Get DOT Format

```bash
$ curl -s http://localhost:8080/graph.dot
graph lldiscovery {
  rankdir=LR;
  node [shape=box, style=rounded];

  "a1b2c3d4" [label="server01.example.com\na1b2c3d4\neth0: fe80::1\neth1: fe80::2"];
  "e5f6g7h8" [label="server02.example.com\ne5f6g7h8\neth0: fe80::10"];
}
```

### Health Check

```bash
$ curl -s http://localhost:8080/health
{"status":"ok"}
```

## Visualization Example

After running `curl http://localhost:8080/graph.dot | dot -Tpng -o topology.png`:

```
┌─────────────────────────────┐
│ server01.example.com        │
│ a1b2c3d4                    │
│ eth0: fe80::1               │
│ eth1: fe80::2               │
└─────────────────────────────┘

┌─────────────────────────────┐
│ server02.example.com        │
│ e5f6g7h8                    │
│ eth0: fe80::10              │
└─────────────────────────────┘

┌─────────────────────────────┐
│ server03.example.com        │
│ i9j0k1l2                    │
│ eth1: fe80::20              │
└─────────────────────────────┘
```

**Interpretation:**
- server01 and server02 see each other on eth0 → same VLAN
- server01 and server03 see each other on eth1 → same VLAN
- server02 and server03 don't see each other → different VLANs/isolated

## Systemd Service Logs

```bash
$ sudo journalctl -u lldiscovery -f
Feb 04 22:00:00 server01 lldiscovery[1234]: time=2026-02-04T22:00:00.000+00:00 level=INFO msg="starting lldiscovery" version=1.0.0
Feb 04 22:00:00 server01 lldiscovery[1234]: time=2026-02-04T22:00:00.001+00:00 level=INFO msg="starting HTTP server" address=:8080
Feb 04 22:00:30 server01 lldiscovery[1234]: time=2026-02-04T22:00:30.123+00:00 level=INFO msg="exported graph" nodes=3 file=/var/lib/lldiscovery/topology.dot
Feb 04 22:02:15 server01 lldiscovery[1234]: time=2026-02-04T22:02:15.456+00:00 level=INFO msg="removed expired nodes" count=1
```

## Debug Mode Example

```bash
$ ./lldiscovery -log-level debug
time=2026-02-04T22:00:00.000+00:00 level=INFO msg="starting lldiscovery" version=dev
time=2026-02-04T22:00:00.001+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156
time=2026-02-04T22:00:00.002+00:00 level=DEBUG msg="sent discovery packet" interface=eth1 source=fe80::2%eth1 size=156
time=2026-02-04T22:00:05.123+00:00 level=DEBUG msg="received discovery packet" hostname=server02.example.com machine_id=e5f6g7h8 source=fe80::10 interface=eth0
time=2026-02-04T22:00:30.000+00:00 level=DEBUG msg="sent discovery packet" interface=eth0 source=fe80::1%eth0 size=156
```
