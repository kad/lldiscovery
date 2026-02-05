# Deployment Guide: Network Prefix Feature

**Version**: Complete with local node prefix fix  
**Date**: 2026-02-05  
**Status**: ✅ Ready for Production

## Overview

This guide covers deploying the complete network prefix feature, including:
- IPv4 and IPv6 prefix collection
- Prefix-based segment naming
- Edge prefix fields in JSON export
- Critical bug fix for local node prefix storage

## Prerequisites

- Go 1.x or later (for building)
- Root or sudo access on all nodes (for systemd restart)
- SSH access to all cluster nodes
- systemd-based service management

## Quick Deploy

```bash
# 1. Build
cd /home/akanevsk/go/lldiscovery
make build

# 2. Deploy (choose your method below)

# 3. Restart all nodes
systemctl restart lldiscovery

# 4. Verify (wait 60s, then check)
curl http://localhost:6469/graph | jq '.nodes | .[] | select(.IsLocal == true) | .Interfaces'
```

## Step-by-Step Deployment

### 1. Build Binary

```bash
cd /home/akanevsk/go/lldiscovery

# Option A: Using Makefile
make build

# Option B: Direct go build
go build -o lldiscovery ./cmd/lldiscovery

# Verify build
./lldiscovery -version
```

**Output should show**: `lldiscovery dev`

### 2. Deploy to Cluster Nodes

#### Option A: Manual SCP (Small Clusters)

```bash
# List of nodes
NODES="cse-r740-1 cse-r740-2 cse-r740-3 r640-1 r640-2"

# Copy to each node
for node in $NODES; do
    echo "Deploying to $node..."
    scp lldiscovery root@$node:/usr/local/bin/lldiscovery
    ssh root@$node "chmod +x /usr/local/bin/lldiscovery"
done
```

#### Option B: Ansible (Recommended for Large Clusters)

```yaml
# deploy-lldiscovery.yml
---
- name: Deploy lldiscovery daemon
  hosts: all
  become: yes
  tasks:
    - name: Copy lldiscovery binary
      copy:
        src: ./lldiscovery
        dest: /usr/local/bin/lldiscovery
        mode: '0755'
        owner: root
        group: root

    - name: Restart lldiscovery service
      systemd:
        name: lldiscovery
        state: restarted
        daemon_reload: yes

    - name: Wait for service to be active
      systemd:
        name: lldiscovery
        state: started
      register: result
      until: result.status.ActiveState == "active"
      retries: 5
      delay: 2
```

Run with:
```bash
ansible-playbook -i inventory.ini deploy-lldiscovery.yml
```

#### Option C: Parallel SSH (pssh)

```bash
# Install if needed: sudo apt install pssh

# Create hosts file
cat > hosts.txt <<EOF
cse-r740-1
cse-r740-2
cse-r740-3
r640-1
r640-2
EOF

# Deploy
parallel-scp -h hosts.txt -O StrictHostKeyChecking=no lldiscovery /usr/local/bin/lldiscovery

# Set permissions
parallel-ssh -h hosts.txt -O StrictHostKeyChecking=no "chmod +x /usr/local/bin/lldiscovery"
```

### 3. Restart Services

#### Option A: Manual Restart
```bash
# On each node
ssh root@cse-r740-1 "systemctl restart lldiscovery"
ssh root@cse-r740-2 "systemctl restart lldiscovery"
# ... etc
```

#### Option B: Parallel Restart
```bash
# Using parallel-ssh
parallel-ssh -h hosts.txt "systemctl restart lldiscovery"

# Or using ansible
ansible all -i inventory.ini -b -m systemd -a "name=lldiscovery state=restarted"
```

#### Option C: Rolling Restart (Zero Downtime)
```bash
# Restart one node at a time with delay
for node in $NODES; do
    echo "Restarting $node..."
    ssh root@$node "systemctl restart lldiscovery"
    echo "Waiting 10s for discovery..."
    sleep 10
done
```

### 4. Verification

#### A. Check Service Status

```bash
# On local node
systemctl status lldiscovery

# On all nodes
parallel-ssh -h hosts.txt "systemctl is-active lldiscovery"
```

Expected: All should return `active`

#### B. Verify Local Node Prefixes

```bash
# Check local node interfaces
curl -s http://localhost:6469/graph | jq '.nodes | .[] | select(.IsLocal == true) | {
  hostname: .Hostname,
  interfaces: [.Interfaces | to_entries[] | {
    name: .key,
    ip: .value.IPAddress,
    prefixes: .value.GlobalPrefixes
  }]
}'
```

**Expected output example:**
```json
{
  "hostname": "cse-r740-1",
  "interfaces": [
    {
      "name": "em1",
      "ip": "fe80::e643:4bff:fe23:22fc%em1",
      "prefixes": ["10.102.73.0/24"]
    },
    {
      "name": "em4",
      "ip": "fe80::e643:4bff:fe23:22ff%em4",
      "prefixes": ["192.168.180.0/24"]
    }
  ]
}
```

#### C. Verify Remote Node Prefixes

```bash
# Check remote nodes have prefixes
curl -s http://localhost:6469/graph | jq '[.nodes | .[] | select(.IsLocal == false) | {
  hostname: .Hostname,
  prefix_count: [.Interfaces | .[] | select(.GlobalPrefixes != null)] | length
}] | sort_by(.hostname)'
```

#### D. Verify Edge Prefixes

```bash
# Check edges have LocalPrefixes and RemotePrefixes
curl -s http://localhost:6469/graph | jq '.segments[0].EdgeInfo | to_entries[0] | .value | {
  local_if: .LocalInterface,
  local_prefixes: .LocalPrefixes,
  remote_if: .RemoteInterface,
  remote_prefixes: .RemotePrefixes
}'
```

**Expected**: LocalPrefixes and RemotePrefixes should be arrays (possibly null if interface has no global addresses, but not due to bug)

#### E. Verify Segment Naming

```bash
# Check segments show prefix names
curl -s http://localhost:6469/graph.dot | grep -E "segment_[0-9]+" | head -5
```

**Expected output example:**
```
  "segment_0" [label="192.168.180.0/24\n15 nodes\n(em4)", ...];
  "segment_1" [label="10.102.73.0/24\n16 nodes\n(em1)", ...];
```

#### F. Full Topology Check

```bash
# Generate visualization
curl -s http://localhost:6469/graph.dot > /tmp/topology.dot
dot -Tpng /tmp/topology.dot > /tmp/topology.png

# View segment summary
curl -s http://localhost:6469/graph | jq '.segments[] | {
  id: .ID,
  prefix: .NetworkPrefix,
  interface: .Interface,
  nodes: (.ConnectedNodes | length)
}'
```

### 5. Monitoring

#### Log Inspection

```bash
# Check for errors during startup
journalctl -u lldiscovery -n 50 --no-pager

# Follow logs in real-time
journalctl -u lldiscovery -f

# Check for prefix collection messages
journalctl -u lldiscovery | grep -i "prefix"
```

#### Common Issues

**Issue**: Service fails to start
```bash
# Check binary permissions
ls -l /usr/local/bin/lldiscovery

# Check for port conflicts
ss -tulpn | grep 6469

# Check config file
cat /etc/lldiscovery/config.json
```

**Issue**: Prefixes still null
```bash
# Check interfaces actually have global addresses
ip addr show

# Verify binary version (should be recent build)
/usr/local/bin/lldiscovery -version
ls -lh /usr/local/bin/lldiscovery

# Check if old process is still running
ps aux | grep lldiscovery
```

**Issue**: Service restarted but old data cached
```bash
# Force full restart with cache clear
systemctl stop lldiscovery
rm -f /var/lib/lldiscovery/* 2>/dev/null
systemctl start lldiscovery
```

## Rollback Procedure

If issues occur, rollback to previous version:

```bash
# 1. Stop service
systemctl stop lldiscovery

# 2. Restore old binary (if backed up)
cp /usr/local/bin/lldiscovery.backup /usr/local/bin/lldiscovery

# 3. Start service
systemctl start lldiscovery

# 4. Verify
systemctl status lldiscovery
```

## Feature Verification Checklist

After deployment, verify these features work:

- [ ] Local node shows in graph with `IsLocal: true`
- [ ] Local node interfaces have non-null `GlobalPrefixes` (if global IPs configured)
- [ ] Remote nodes show non-null `GlobalPrefixes` (if global IPs configured)
- [ ] Edges have `LocalPrefixes` and `RemotePrefixes` fields
- [ ] Segments show network prefix labels (e.g., "192.168.1.0/24")
- [ ] Segments fall back to interface names when no prefix available
- [ ] Both IPv4 and IPv6 prefixes are collected
- [ ] DOT export shows prefix-based segment names
- [ ] JSON export includes all prefix information

## Performance Notes

- No significant performance impact
- Discovery packet size increased by ~50-200 bytes per interface (prefix data)
- JSON export size increased by ~10-20% (prefix fields)
- Graph processing time unchanged (prefix selection is O(n))

## Security Considerations

- Network prefixes are now visible in discovery packets (link-local multicast only)
- JSON API exposes network topology including prefixes (bind to localhost or use firewall)
- No authentication on HTTP API (use iptables or nginx proxy for security)

## Next Steps

After successful deployment:

1. **Monitor for 24h** - Watch logs for unexpected behavior
2. **Export baseline topology** - Save current state for comparison
3. **Document network** - Use prefix labels to create network map
4. **Set up alerting** - Monitor for missing nodes or incorrect prefixes
5. **Plan maintenance** - Schedule regular updates for future features

## Support

For issues or questions:
- Check logs: `journalctl -u lldiscovery`
- Review documentation: `README.md`, `NETWORK_PREFIXES.md`
- Test locally: Build and run with test data

## Summary

This deployment adds complete network prefix support to lldiscovery:
- ✅ IPv4 and IPv6 prefix collection
- ✅ Prefix-based segment naming
- ✅ Complete JSON export with prefix data
- ✅ Fixed critical local node storage bug

**Estimated deployment time**: 5-15 minutes (depending on cluster size)

**Recommendation**: Deploy during maintenance window for large clusters (though service interruption is minimal).
