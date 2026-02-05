package discovery

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/vishvananda/netlink"
)

type InterfaceInfo struct {
	Name          string
	LinkLocal     string
	IsRDMA        bool
	RDMADevice    string
	NodeGUID      string
	SysImageGUID  string
}

func GetActiveInterfaces() ([]InterfaceInfo, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	// Get RDMA device to netdev mapping
	rdmaMap := getRDMADeviceMapping()

	var result []InterfaceInfo

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			if ip.To4() != nil {
				continue
			}

			if !ip.IsLinkLocalUnicast() {
				continue
			}

			info := InterfaceInfo{
				Name:      iface.Name,
				LinkLocal: formatIPv6WithZone(ip, iface.Name),
			}

			// Check if this interface has an RDMA device
			if rdmaDevice, ok := rdmaMap[iface.Name]; ok {
				info.IsRDMA = true
				info.RDMADevice = rdmaDevice
				info.NodeGUID = getRDMANodeGUID(rdmaDevice)
				info.SysImageGUID = getRDMASysImageGUID(rdmaDevice)
			}

			result = append(result, info)
			break
		}
	}

	return result, nil
}

func formatIPv6WithZone(ip net.IP, zone string) string {
	return fmt.Sprintf("%s%%%s", ip.String(), zone)
}

func ParseSourceAddr(addr string) string {
	if idx := strings.Index(addr, "%"); idx != -1 {
		addr = addr[:idx]
	}
	if strings.HasPrefix(addr, "[") {
		addr = strings.TrimPrefix(addr, "[")
		if idx := strings.Index(addr, "]"); idx != -1 {
			addr = addr[:idx]
		}
	}
	return addr
}

// getRDMADeviceMapping returns a map of network interface name to RDMA device name
// Uses netlink to get the parent interface for RDMA devices
func getRDMADeviceMapping() map[string]string {
	result := make(map[string]string)

	// Use netlink to list all links
	links, err := netlink.LinkList()
	if err != nil {
		return result
	}

	// Check each link for RDMA capabilities
	for _, link := range links {
		attrs := link.Attrs()
		if attrs == nil {
			continue
		}

		// Check if this interface has an associated RDMA device
		rdmaDevice := getRDMADeviceForInterface(attrs.Name)
		if rdmaDevice != "" {
			result[attrs.Name] = rdmaDevice
		}
	}

	return result
}

// getRDMADeviceForInterface finds the RDMA device associated with a network interface
func getRDMADeviceForInterface(ifaceName string) string {
	// First try hardware RDMA: /sys/class/net/<ifaceName>/device/infiniband/
	devicePath := fmt.Sprintf("/sys/class/net/%s/device/infiniband", ifaceName)
	entries, err := os.ReadDir(devicePath)
	if err == nil && len(entries) > 0 {
		// Return the first RDMA device found
		return entries[0].Name()
	}

	// For software RDMA (RXE), check if any RDMA device has this interface as parent
	ibPath := "/sys/class/infiniband"
	ibEntries, err := os.ReadDir(ibPath)
	if err != nil {
		return ""
	}

	for _, entry := range ibEntries {
		// Follow symlinks - RXE devices are symlinks in /sys/class/infiniband/
		entryPath := filepath.Join(ibPath, entry.Name())
		info, err := os.Stat(entryPath)
		if err != nil || !info.IsDir() {
			continue
		}
		
		// Check parent file for RXE devices
		parentFile := filepath.Join(ibPath, entry.Name(), "parent")
		parentData, err := os.ReadFile(parentFile)
		if err == nil {
			parent := strings.TrimSpace(string(parentData))
			if parent == ifaceName {
				return entry.Name()
			}
		}
	}

	return ""
}

// getRDMANodeGUID reads the node GUID for an RDMA device
func getRDMANodeGUID(rdmaDevice string) string {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/infiniband/%s/node_guid", rdmaDevice))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// getRDMASysImageGUID reads the system image GUID for an RDMA device
func getRDMASysImageGUID(rdmaDevice string) string {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/infiniband/%s/sys_image_guid", rdmaDevice))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// GetRDMADevices returns a list of all RDMA devices and their parent network interfaces
func GetRDMADevices() (map[string][]string, error) {
	rdmaDevices := make(map[string][]string)

	// List all RDMA devices in /sys/class/infiniband/
	ibPath := "/sys/class/infiniband"
	entries, err := os.ReadDir(ibPath)
	if err != nil {
		// No RDMA devices present
		return rdmaDevices, nil
	}

	for _, entry := range entries {
		// Follow symlinks - RXE devices are symlinks in /sys/class/infiniband/
		entryPath := filepath.Join(ibPath, entry.Name())
		info, err := os.Stat(entryPath)
		if err != nil || !info.IsDir() {
			continue
		}

		rdmaDevice := entry.Name()
		
		// Try hardware RDMA: /sys/class/infiniband/<device>/device/net/
		netPath := filepath.Join(ibPath, rdmaDevice, "device", "net")
		netEntries, err := os.ReadDir(netPath)
		
		var parentIfaces []string
		
		if err == nil && len(netEntries) > 0 {
			// Hardware RDMA device
			for _, netEntry := range netEntries {
				parentIfaces = append(parentIfaces, netEntry.Name())
			}
		} else {
			// Try software RDMA (RXE): check parent file
			parentFile := filepath.Join(ibPath, rdmaDevice, "parent")
			parentData, err := os.ReadFile(parentFile)
			if err == nil {
				parent := strings.TrimSpace(string(parentData))
				if parent != "" {
					parentIfaces = append(parentIfaces, parent)
				}
			}
		}

		if len(parentIfaces) > 0 {
			rdmaDevices[rdmaDevice] = parentIfaces
		}
	}

	return rdmaDevices, nil
}
