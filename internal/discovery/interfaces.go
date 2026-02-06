package discovery

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdlayher/wifi"
	"github.com/vishvananda/netlink"
)

type InterfaceInfo struct {
	Name           string
	LinkLocal      string
	GlobalPrefixes []string // Global unicast network prefixes (e.g., "2001:db8:1::/64")
	IsRDMA         bool
	RDMADevice     string
	NodeGUID       string
	SysImageGUID   string
	Speed          int // Link speed in Mbps
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

		var linkLocal string
		var globalPrefixes []string

		// Collect link-local (IPv6) and global unicast addresses (IPv4 + IPv6)
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP

			// Handle IPv4 addresses
			if ip4 := ip.To4(); ip4 != nil {
				// Skip loopback (127.0.0.0/8)
				if ip4[0] == 127 {
					continue
				}
				// Skip link-local (169.254.0.0/16)
				if ip4[0] == 169 && ip4[1] == 254 {
					continue
				}
				// Collect IPv4 global prefix
				prefix := fmt.Sprintf("%s/%d", ipNet.IP.Mask(ipNet.Mask).String(), getPrefixLength(ipNet.Mask))
				if !contains(globalPrefixes, prefix) {
					globalPrefixes = append(globalPrefixes, prefix)
				}
				continue
			}

			// Handle IPv6 addresses
			if ip.IsLinkLocalUnicast() {
				// Found IPv6 link-local address (required for discovery)
				if linkLocal == "" {
					linkLocal = formatIPv6WithZone(ip, iface.Name)
				}
			} else if ip.IsGlobalUnicast() {
				// Found IPv6 global unicast address - extract network prefix
				prefix := fmt.Sprintf("%s/%d", ipNet.IP.Mask(ipNet.Mask).String(), getPrefixLength(ipNet.Mask))
				// Avoid duplicates
				if !contains(globalPrefixes, prefix) {
					globalPrefixes = append(globalPrefixes, prefix)
				}
			}
		}

		// Only add interface if it has a link-local address
		if linkLocal != "" {
			info := InterfaceInfo{
				Name:           iface.Name,
				LinkLocal:      linkLocal,
				GlobalPrefixes: globalPrefixes,
			}

			// Check if this interface has an RDMA device
			if rdmaDevice, ok := rdmaMap[iface.Name]; ok {
				info.IsRDMA = true
				info.RDMADevice = rdmaDevice
				info.NodeGUID = getRDMANodeGUID(rdmaDevice)
				info.SysImageGUID = getRDMASysImageGUID(rdmaDevice)
			}

			// Get link speed
			info.Speed = getLinkSpeed(iface.Name)

			result = append(result, info)
		}
	}

	return result, nil
}

func formatIPv6WithZone(ip net.IP, zone string) string {
	return fmt.Sprintf("%s%%%s", ip.String(), zone)
}

// getPrefixLength returns the prefix length from an IPv6 mask
func getPrefixLength(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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

// getLinkSpeed returns the link speed in Mbps by reading from /sys/class/net/<iface>/speed
// For WiFi interfaces, it attempts to use iw to get the actual link rate
// Returns 0 if speed cannot be determined
func getLinkSpeed(ifaceName string) int {
	// First try sysfs
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", ifaceName)
	data, err := os.ReadFile(speedPath)
	if err == nil {
		speedStr := strings.TrimSpace(string(data))
		var speed int
		_, err = fmt.Sscanf(speedStr, "%d", &speed)
		if err == nil && speed > 0 {
			// Valid speed from sysfs
			return speed
		}
	}
	
	// If sysfs didn't work (common for WiFi), try WiFi-specific detection
	// Check if this looks like a WiFi interface
	if strings.Contains(ifaceName, "wl") || strings.Contains(ifaceName, "wifi") {
		if speed := getWiFiSpeed(ifaceName); speed > 0 {
			return speed
		}
	}
	
	// No speed could be determined
	return 0
}

// getWiFiSpeed attempts to get the actual WiFi link speed
// First tries native nl80211 via github.com/mdlayher/wifi library
// Falls back to iw tool if native method fails
// Returns 0 if the speed cannot be determined
func getWiFiSpeed(ifaceName string) int {
	// Method 1: Try native nl80211 library (preferred)
	if speed := getWiFiSpeedNative(ifaceName); speed > 0 {
		return speed
	}
	
	// Method 2: Fall back to iw command-line tool
	return getWiFiSpeedIw(ifaceName)
}

// getWiFiSpeedNative uses nl80211 via github.com/mdlayher/wifi to get WiFi speed
func getWiFiSpeedNative(ifaceName string) int {
	client, err := wifi.New()
	if err != nil {
		// nl80211 not available or no permissions
		return 0
	}
	defer client.Close()
	
	// Get all WiFi interfaces
	interfaces, err := client.Interfaces()
	if err != nil {
		return 0
	}
	
	// Find our interface
	var targetIface *wifi.Interface
	for _, iface := range interfaces {
		if iface.Name == ifaceName {
			targetIface = iface
			break
		}
	}
	
	if targetIface == nil {
		return 0
	}
	
	// Get station info (connected AP)
	stations, err := client.StationInfo(targetIface)
	if err != nil || len(stations) == 0 {
		return 0
	}
	
	// Use the first station (typically the connected AP)
	station := stations[0]
	
	// Return the higher of TX/RX bitrate in Mbps
	// Bitrate is in bits per second, convert to Mbps
	rxMbps := int(station.ReceiveBitrate / 1000000)
	txMbps := int(station.TransmitBitrate / 1000000)
	
	if txMbps > rxMbps {
		return txMbps
	}
	return rxMbps
}

// getWiFiSpeedIw uses the iw command-line tool to get WiFi speed
func getWiFiSpeedIw(ifaceName string) int {
	// Try common locations for iw tool
	iwPaths := []string{
		"/usr/sbin/iw",
		"/sbin/iw",
		"/usr/bin/iw",
		"/bin/iw",
	}
	
	var iwPath string
	for _, path := range iwPaths {
		if _, err := os.Stat(path); err == nil {
			iwPath = path
			break
		}
	}
	
	if iwPath == "" {
		// Try PATH
		if path, err := exec.LookPath("iw"); err == nil {
			iwPath = path
		} else {
			return 0
		}
	}
	
	// Run: iw dev <iface> link
	cmd := exec.Command(iwPath, "dev", ifaceName, "link")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	
	// Parse output for tx/rx bitrate
	// Format examples:
	//   "tx bitrate: 1200.9 MBit/s 80MHz HE-MCS 11 HE-NSS 2 HE-GI 0 HE-DCM 0"
	//   "rx bitrate: 960.7 MBit/s 80MHz HE-MCS 9 HE-NSS 2 HE-GI 0 HE-DCM 0"
	
	var txSpeed, rxSpeed float64
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "tx bitrate:") {
			fields := strings.Fields(line)
			// fields: ["tx", "bitrate:", "1200.9", "MBit/s", ...]
			if len(fields) >= 3 {
				speedStr := fields[2]
				speed, err := strconv.ParseFloat(speedStr, 64)
				if err == nil {
					txSpeed = speed
				}
			}
		}
		
		if strings.HasPrefix(line, "rx bitrate:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				speedStr := fields[2]
				speed, err := strconv.ParseFloat(speedStr, 64)
				if err == nil {
					rxSpeed = speed
				}
			}
		}
	}
	
	// Return the higher of TX/RX speeds (as link speed)
	if txSpeed > rxSpeed {
		return int(txSpeed)
	}
	return int(rxSpeed)
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
