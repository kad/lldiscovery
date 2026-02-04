package discovery

import (
	"fmt"
	"net"
	"strings"
)

type InterfaceInfo struct {
	Name      string
	LinkLocal string
}

func GetActiveInterfaces() ([]InterfaceInfo, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

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

			result = append(result, InterfaceInfo{
				Name:      iface.Name,
				LinkLocal: formatIPv6WithZone(ip, iface.Name),
			})
			break
		}
	}

	return result, nil
}

func formatIPv6WithZone(ip net.IP, zone string) string {
	return fmt.Sprintf("%s%%%s", ip.String(), zone)
}

func GetMulticastAddr(port int) string {
	return fmt.Sprintf("[ff02::1]:%d", port)
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
