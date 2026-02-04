# RDMA Edge Labels - Complete Example

This document shows the complete format of edge labels with RDMA information.

## Edge Label Format

### Standard Ethernet Connection
```
eth0 (fe80::1) <-> eth0 (fe80::10)
```
Simple format showing interface names and IPv6 link-local addresses.

### InfiniBand/RDMA Connection (Full Detail)
```
ib0 (fe80::1) <-> ib0 (fe80::100)
Local: mlx5_0 N:0x1111:2222:3333:4444 S:0xaaaa:bbbb:cccc:dddd
Remote: mlx5_2 N:0x9999:aaaa:bbbb:cccc S:0x2222:3333:4444:5555
```

Where:
- **Interface names and addresses**: `ib0 (fe80::1) <-> ib0 (fe80::100)`
- **Local RDMA device**: `mlx5_0` (Mellanox ConnectX-5 or newer)
- **Local Node GUID**: `N:0x1111:2222:3333:4444` - Unique identifier for the HCA port
- **Local Sys Image GUID**: `S:0xaaaa:bbbb:cccc:dddd` - System-level identifier for the HCA
- **Remote side**: Same format for remote endpoint

### Mixed Connection (One side RDMA)
```
ib0 (fe80::1) <-> eth0 (fe80::200)
Local: mlx5_0 N:0x1111:2222:3333:4444 S:0xaaaa:bbbb:cccc:dddd
```
Only local side has RDMA - remote side is standard Ethernet.

## Complete Graph Example

```dot
graph lldiscovery {
  rankdir=LR;
  node [shape=box, style=rounded];

  "local-host" [label="compute-01 (local)\nlocal-ho\nib0 [mlx5_0]\nN: 0x1111:2222:3333:4444\nS: 0xaaaa:bbbb:cccc:dddd\nib1 [mlx5_1]\nN: 0x5555:6666:7777:8888\nS: 0xeeee:ffff:0000:1111", style="rounded,filled", fillcolor="lightblue"];
  
  "remote-ib-host" [label="compute-02\nremote-i\nib0 [mlx5_2]\nN: 0x9999:aaaa:bbbb:cccc\nS: 0x2222:3333:4444:5555\nib1 [mlx5_3]\nN: 0xdddd:eeee:ffff:0000\nS: 0x6666:7777:8888:9999"];
  
  "standard-host" [label="storage-01\nstandard\neth0"];

  "local-host" -- "remote-ib-host" [label="ib0 (fe80::1) <-> ib0 (fe80::100)\nLocal: mlx5_0 N:0x1111:2222:3333:4444 S:0xaaaa:bbbb:cccc:dddd\nRemote: mlx5_2 N:0x9999:aaaa:bbbb:cccc S:0x2222:3333:4444:5555"];
  
  "local-host" -- "remote-ib-host" [label="ib1 (fe80::2) <-> ib1 (fe80::101)\nLocal: mlx5_1 N:0x5555:6666:7777:8888 S:0xeeee:ffff:0000:1111\nRemote: mlx5_3 N:0xdddd:eeee:ffff:0000 S:0x6666:7777:8888:9999"];
  
  "local-host" -- "standard-host" [label="eth0 (fe80::10) <-> eth0 (fe80::200)"];
}
```

## Benefits

1. **Complete Hardware Identification**: Both Node GUID and Sys Image GUID visible
2. **Multi-path Visualization**: Each IB port pair shown separately
3. **Troubleshooting**: Easy to identify specific HCA ports
4. **Fabric Management**: Clear view of all RDMA connections
5. **Mixed Environments**: Clearly shows RDMA vs standard Ethernet connections

## Use Cases

### InfiniBand Fabric Mapping
Track all IB connections between compute nodes with complete HCA identification.

### RoCE Network Verification
Verify RDMA over Converged Ethernet deployments with GUID information.

### Multi-rail InfiniBand
Visualize dual-rail or multi-rail IB configurations with separate edges per rail.

### Hardware Inventory
Identify all Mellanox/NVIDIA HCAs in the cluster with their GUIDs.
