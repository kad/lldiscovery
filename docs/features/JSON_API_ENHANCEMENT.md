# JSON HTTP API Enhancement

## Summary

Extended the `/graph` HTTP endpoint to export complete topology information including nodes, edges (both direct and indirect), and optionally network segments.

## Changes Made

### 1. Updated HTTP Handler (`internal/server/server.go`)

**Before:**
```go
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
    nodes := s.graph.GetNodes()
    json.NewEncoder(w).Encode(nodes)
}
```

**After:**
```go
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
    nodes := s.graph.GetNodes()
    edges := s.graph.GetEdges()
    
    response := map[string]interface{}{
        "nodes": nodes,
        "edges": edges,
    }
    
    if s.showSegments {
        segments := s.graph.GetNetworkSegments()
        response["segments"] = segments
    }
    
    json.NewEncoder(w).Encode(response)
}
```

### 2. Added Comprehensive Tests (`internal/server/server_test.go`)

Created 5 test cases covering:
- ✅ Basic JSON response structure validation
- ✅ Nodes and edges presence in response
- ✅ Segments included when `showSegments=true`
- ✅ Segments excluded when `showSegments=false`
- ✅ DOT export functionality
- ✅ Health endpoint
- ✅ Method validation (405 for non-GET requests)

## API Response Format

### Structure

```json
{
  "nodes": {
    "machine-id": {
      "Hostname": "string",
      "MachineID": "string",
      "LastSeen": "timestamp",
      "Interfaces": {
        "interface-name": {
          "IPAddress": "string",
          "RDMADevice": "string",
          "NodeGUID": "string",
          "SysImageGUID": "string",
          "Speed": 0
        }
      },
      "IsLocal": false
    }
  },
  "edges": {
    "source-machine-id": {
      "dest-machine-id": [
        {
          "LocalInterface": "string",
          "LocalAddress": "string",
          "LocalRDMADevice": "string",
          "LocalNodeGUID": "string",
          "LocalSysImageGUID": "string",
          "LocalSpeed": 0,
          "RemoteInterface": "string",
          "RemoteAddress": "string",
          "RemoteRDMADevice": "string",
          "RemoteNodeGUID": "string",
          "RemoteSysImageGUID": "string",
          "RemoteSpeed": 0,
          "Direct": true,
          "LearnedFrom": "string"
        }
      ]
    }
  },
  "segments": [
    {
      "Interface": "string",
      "ConnectedNodes": ["machine-id-1", "machine-id-2", ...],
      "EdgeInfo": {
        "machine-id": {
          "LocalInterface": "string",
          "LocalAddress": "string",
          ...
        }
      }
    }
  ]
}
```

## Key Features

### 1. Complete Topology Export
- **Nodes**: All discovered hosts with full interface details
- **Edges**: All connections (direct and indirect) with complete metadata
- **Segments**: Network VLAN detection when enabled with `--show-segments`

### 2. Direct vs Indirect Edges
- `Direct: true` - Edges discovered through direct multicast packets
- `Direct: false` - Edges learned transitively from neighbor information
- `LearnedFrom` - Identifies which node shared the indirect connection

### 3. RDMA Information
Each interface and edge includes:
- `RDMADevice` - Device name (e.g., "mlx5_0", "irdma0")
- `NodeGUID` - InfiniBand node GUID
- `SysImageGUID` - InfiniBand system image GUID
- Visual distinction in DOT export (blue edges)

### 4. Network Segments
When `--show-segments` is enabled:
- Detects VLANs/shared network segments (3+ nodes)
- Lists all participating nodes
- Provides edge information for each node-to-segment connection

## Use Cases

### 1. External Monitoring Tools
Tools can consume the JSON API to:
- Build custom visualizations
- Integrate with monitoring systems
- Analyze network topology programmatically

### 2. Debugging Network Issues
Complete edge information helps identify:
- Missing connections (compare direct vs expected)
- Indirect-only paths (no direct connectivity)
- RDMA connectivity issues
- VLAN misconfigurations

### 3. Automated Testing
Test infrastructure can:
- Verify expected topology
- Detect unexpected network isolation
- Validate segment detection
- Check RDMA fabric connectivity

## Example Usage

### Get Complete Topology
```bash
curl http://localhost:6469/graph | jq .
```

### Extract Only Nodes
```bash
curl http://localhost:6469/graph | jq '.nodes'
```

### Count Direct vs Indirect Edges
```bash
curl http://localhost:6469/graph | jq '
  .edges
  | to_entries[]
  | .value
  | to_entries[]
  | .value[]
  | .Direct
' | grep -c true

# Indirect edges
curl http://localhost:6469/graph | jq '
  .edges
  | to_entries[]
  | .value
  | to_entries[]
  | .value[]
  | select(.Direct == false)
' | jq -s 'length'
```

### List All Segments
```bash
curl http://localhost:6469/graph | jq '.segments[] | {
  interface: .Interface,
  nodes: .ConnectedNodes | length
}'
```

### Find RDMA-to-RDMA Connections
```bash
curl http://localhost:6469/graph | jq '
  .edges
  | to_entries[]
  | .value
  | to_entries[]
  | .value[]
  | select(.LocalRDMADevice != "" and .RemoteRDMADevice != "")
  | {
      from: .LocalInterface,
      to: .RemoteInterface,
      local_rdma: .LocalRDMADevice,
      remote_rdma: .RemoteRDMADevice
    }
'
```

## Testing

All server tests pass:
```
=== RUN   TestHandleGraph
--- PASS: TestHandleGraph (0.00s)
=== RUN   TestHandleGraphWithSegments
--- PASS: TestHandleGraphWithSegments (0.00s)
=== RUN   TestHandleGraphDOT
--- PASS: TestHandleGraphDOT (0.00s)
=== RUN   TestHandleHealth
--- PASS: TestHandleHealth (0.00s)
=== RUN   TestHandleGraphMethodNotAllowed
--- PASS: TestHandleGraphMethodNotAllowed (0.00s)
PASS
ok  	github.com/kad/lldiscovery/internal/server
```

## Backward Compatibility

⚠️ **Breaking Change**: The `/graph` endpoint response format has changed from:
```json
{
  "machine-id": { ... }
}
```

To:
```json
{
  "nodes": { "machine-id": { ... } },
  "edges": { ... }
}
```

**Migration Path:**
- Old clients expecting just nodes should update to access `response.nodes`
- New clients get full topology information
- No changes to `/graph.dot` and `/health` endpoints

## Benefits

1. **Complete Information**: Single API call gets entire topology
2. **No Data Loss**: Edges were previously unavailable via HTTP API
3. **Debugging**: Indirect edges help understand transitive discovery
4. **Integration**: Standard JSON format easy to consume
5. **Consistency**: Same data structure used internally by DOT exporter
6. **Flexibility**: Segments optional based on configuration

## Files Modified

- `internal/server/server.go` (26 lines) - Enhanced `/graph` handler
- `internal/server/server_test.go` (238 lines) - New test file
- `README.md` - Updated API documentation with examples

## Related Features

- Transitive Discovery: `--include-neighbors` enables indirect edge learning
- Network Segments: `--show-segments` enables VLAN detection
- DOT Export: Visual representation uses same data structure
- RDMA Support: Full RDMA metadata included in edges
