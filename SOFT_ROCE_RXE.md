# Soft-RoCE (RXE) Support

## Overview

`lldiscovery` fully supports Soft-RoCE (RXE) - software-based RDMA over standard Ethernet interfaces. RXE allows you to use RDMA protocols on any Ethernet adapter without requiring specialized hardware.

## What is Soft-RoCE (RXE)?

RXE (RDMA over Converged Ethernet, software implementation) is a Linux kernel module that implements the RoCE v2 protocol in software. It allows standard Ethernet NICs to act as RDMA-capable devices.

**Key Differences from Hardware RDMA:**
- **Software-based**: RDMA protocol implemented in kernel software, not hardware
- **No special NIC required**: Works with any Ethernet adapter
- **Virtual devices**: RXE creates virtual RDMA devices associated with Ethernet interfaces
- **Lower performance**: Software overhead vs. hardware offload, but still faster than TCP/IP for many workloads

## Detection

`lldiscovery` automatically detects RXE interfaces by:

1. **Scanning `/sys/class/infiniband/`** for all RDMA devices
2. **Checking for hardware** via `/sys/class/infiniband/<device>/device/net/`
3. **Checking for software (RXE)** via `/sys/class/infiniband/<device>/parent` file
4. **Reading GUIDs** from `node_guid` and `sys_image_guid` files

### sysfs Structure for RXE

```
/sys/class/infiniband/rxe0/
├── parent              # Points to parent Ethernet interface (e.g., "enp0s31f6")
├── node_guid           # Node GUID (e.g., "7686:e2ff:fe32:6988")
├── sys_image_guid      # System Image GUID
├── ports/
│   └── 1/              # Port 1 (RXE uses single port)
└── ...
```

**vs. Hardware RDMA:**
```
/sys/class/infiniband/mlx5_0/
├── device/             # Symlink to PCI device
│   └── net/            # Parent network interfaces
│       └── ib0
├── node_guid
├── sys_image_guid
└── ...
```

## Setup Example

### Creating an RXE Interface

```bash
# Load the RXE kernel module
sudo modprobe rdma_rxe

# Create RXE device on eth0
sudo rdma link add rxe0 type rxe netdev eth0

# Verify
rdma link show
# Output: link rxe0/1 state ACTIVE physical_state LINK_UP netdev eth0

# Check in lldiscovery
./lldiscovery -log-level debug
# Should show RDMA info for eth0
```

### Removing an RXE Interface

```bash
sudo rdma link delete rxe0
```

## Discovery Behavior

When `lldiscovery` detects an RXE interface:

1. **Interface Detection**: The parent Ethernet interface (e.g., `enp0s31f6`) is detected as RDMA-capable
2. **Packet Transmission**: Discovery packets include RXE device name and GUIDs:
   ```json
   {
     "hostname": "node01",
     "interface": "enp0s31f6",
     "rdma_device": "rxe0",
     "node_guid": "7686:e2ff:fe32:6988",
     "sys_image_guid": "7686:e2ff:fe32:6988"
   }
   ```
3. **Graph Visualization**: Ethernet interface labeled with RXE device info
4. **HTTP API**: Complete RXE information available in JSON

## Example Output

### Debug Log
```
level=DEBUG msg="sent discovery packet" 
  interface=enp0s31f6 
  source=fe80::f2e5:c0ad:dee1:44e2%enp0s31f6 
  size=267 
  content="{...,\"rdma_device\":\"rxe0\",\"node_guid\":\"7686:e2ff:fe32:6988\",...}"
```

### HTTP API Response
```bash
$ curl http://localhost:8080/graph | jq '.[] | .Interfaces.enp0s31f6'
{
  "IPAddress": "fe80::f2e5:c0ad:dee1:44e2%enp0s31f6",
  "RDMADevice": "rxe0",
  "NodeGUID": "7686:e2ff:fe32:6988",
  "SysImageGUID": "7686:e2ff:fe32:6988"
}
```

### DOT Graph
```dot
"node01" [label="node01\nlocal-no\nenp0s31f6 [rxe0]\nN: 7686:e2ff:fe32:6988\nS: 7686:e2ff:fe32:6988", ...];
```

The node label shows:
- Interface name: `enp0s31f6`
- RXE device: `[rxe0]`
- Node GUID: `N: 7686:e2ff:fe32:6988`
- Sys Image GUID: `S: 7686:e2ff:fe32:6988`

## Use Cases

### 1. Testing RDMA Applications

Develop and test RDMA applications on standard hardware:
```bash
# Set up RXE on your development machine
sudo modprobe rdma_rxe
sudo rdma link add rxe_test type rxe netdev eth0

# Run lldiscovery to verify RDMA topology
./lldiscovery -log-level debug
```

### 2. HPC Cluster with Mixed Hardware

Use RXE on nodes without InfiniBand adapters:
- Compute nodes with InfiniBand: `mlx5_0`, `mlx5_1`
- Storage nodes with RXE: `rxe0`, `rxe1`
- All show up in the same topology graph

### 3. RDMA over WAN

Test RDMA protocols over routed networks (though performance will be limited by software processing).

## Performance Considerations

**RXE Performance:**
- **CPU overhead**: RDMA processing done in kernel, uses CPU cycles
- **Bandwidth**: Limited by Ethernet bandwidth (1G, 10G, 25G, etc.)
- **Latency**: Higher than hardware RDMA, but lower than TCP/IP

**Hardware RDMA Performance:**
- **Hardware offload**: RDMA processing in NIC hardware
- **Bandwidth**: Up to 200 Gbps (InfiniBand HDR)
- **Latency**: Sub-microsecond with hardware offload

## Troubleshooting

### RXE Not Detected

Check if RXE module is loaded:
```bash
lsmod | grep rdma_rxe
```

Verify RXE device exists:
```bash
rdma link show
ls /sys/class/infiniband/
```

Check parent file:
```bash
cat /sys/class/infiniband/rxe0/parent
# Should show parent interface name
```

### Packet Size

RXE packets are larger due to RDMA information:
- Without RDMA: ~156 bytes
- With RXE: ~267 bytes

This is normal and expected.

## Technical Details

### Implementation

The detection code handles both hardware and software RDMA:

```go
// Try hardware RDMA first
devicePath := fmt.Sprintf("/sys/class/net/%s/device/infiniband", ifaceName)
if exists(devicePath) {
    return getHardwareRDMA(devicePath)
}

// Try software RDMA (RXE)
for each rdmaDevice in /sys/class/infiniband/ {
    parentFile := fmt.Sprintf("/sys/class/infiniband/%s/parent", rdmaDevice)
    if readFile(parentFile) == ifaceName {
        return rdmaDevice
    }
}
```

### Symlink Handling

RXE devices appear as symlinks in `/sys/class/infiniband/`:
```bash
$ ls -l /sys/class/infiniband/
lrwxrwxrwx 1 root root 0 /sys/class/infiniband/rxe0 -> ../../devices/virtual/infiniband/rxe0
```

The code uses `os.Stat()` to follow symlinks instead of `entry.IsDir()`.

## References

- [Linux RXE Documentation](https://github.com/linux-rdma/rdma-core/blob/master/Documentation/rxe.md)
- [RDMA Core User Space](https://github.com/linux-rdma/rdma-core)
- [Soft-RoCE Wiki](https://github.com/SoftRoCE/rxe-dev/wiki)
