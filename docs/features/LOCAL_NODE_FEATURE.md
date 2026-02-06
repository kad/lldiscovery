# Local Node Feature

## Overview

The daemon now includes its own host information in the topology graph, making it easy to see the complete picture including the observing node itself.

## What's Included

### Local Node Information
- **Hostname**: The hostname of the host running the daemon
- **Machine ID**: Unique identifier from `/etc/machine-id`
- **Interfaces**: All active network interfaces with their IPv6 link-local addresses
- **IsLocal Flag**: Set to `true` to distinguish from discovered remote nodes

### Visual Highlighting

In DOT/Graphviz output:
- **Blue background** (`fillcolor="lightblue"`)
- **"(local)" label** appended to the hostname
- Makes it immediately obvious which node is the observer

## Examples

### JSON API Output

```bash
$ curl -s http://localhost:6469/graph | jq
{
  "dd0621083f6949caafc297ee5711fbdf": {
    "Hostname": "myhost",
    "MachineID": "dd0621083f6949caafc297ee5711fbdf",
    "LastSeen": "2026-02-04T23:23:06.123534219+02:00",
    "Interfaces": {
      "docker0": "fe80::90f5:e2ff:fe9c:9e15%docker0",
      "enp0s31f6": "fe80::f2e5:c0ad:dee1:44e2%enp0s31f6",
      "wlp0s20f3": "fe80::baaf:b632:3404:f9de%wlp0s20f3"
    },
    "IsLocal": true  ← Indicates this is the local node
  },
  "4c8709eab8394194bfb7ac8b768cd314": {
    "Hostname": "imini.v0.example.com",
    "MachineID": "4c8709eab8394194bfb7ac8b768cd314",
    "LastSeen": "2026-02-04T23:23:05.456789012+02:00",
    "Interfaces": {
      "enp1s0f0": "fe80::aa20:66ff:fe34:984f"
    },
    "IsLocal": false  ← Remote discovered node
  }
}
```

### DOT Output

```dot
graph lldiscovery {
  rankdir=LR;
  node [shape=box, style=rounded];

  "dd062108" [label="myhost (local)\ndd062108\neth0: fe80::1", 
              style="rounded,filled", fillcolor="lightblue"];  ← Blue background
  
  "4c8709ea" [label="imini.v0.example.com\n4c8709ea\neth0: fe80::2"];  ← Standard node
}
```

## Startup Log

The daemon logs when the local node is added:

```
time=2026-02-04T23:21:46.921+02:00 level=INFO msg="starting lldiscovery" version=dev
time=2026-02-04T23:21:46.921+02:00 level=INFO msg="local node added to graph" hostname=myhost interfaces=5
```

## Implementation Details

### Graph Structure
- Local node is stored separately in `graph.localNode` field
- Included in `GetNodes()` output alongside discovered nodes
- Never expires (no TTL check)
- Updated only at startup

### Code Changes
1. **internal/graph/graph.go**: Added `IsLocal` field to Node struct, `SetLocalNode()` method
2. **internal/export/dot.go**: Check `node.IsLocal` to apply special styling
3. **cmd/lldiscovery/main.go**: Call `SetLocalNode()` after interface discovery at startup
4. **Documentation**: Updated README, EXAMPLE_OUTPUT, CHANGELOG

## Benefits

1. **Complete View**: See your position in the topology
2. **Multi-Host Clarity**: When running on multiple hosts, immediately know which view you're looking at
3. **Troubleshooting**: Verify your interfaces are being detected correctly
4. **Consistency**: All nodes (local + remote) in one unified graph

## Testing

```bash
# Build
make build

# Run
./lldiscovery -log-level info

# Check JSON API
curl -s http://localhost:6469/graph | jq '.[] | select(.IsLocal == true)'

# Check DOT output
curl http://localhost:6469/graph.dot

# Visualize
curl http://localhost:6469/graph.dot | dot -Tpng -o topology.png
```

The local node will have a light blue background in the rendered image.
