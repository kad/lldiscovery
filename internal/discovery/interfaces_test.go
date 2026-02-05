package discovery

import (
	"net"
	"testing"
)

func TestFormatIPv6WithZone(t *testing.T) {
	tests := []struct {
		name     string
		ip       net.IP
		zone     string
		expected string
	}{
		{
			name:     "basic link-local",
			ip:       net.ParseIP("fe80::1"),
			zone:     "eth0",
			expected: "fe80::1%eth0",
		},
		{
			name:     "with interface index",
			ip:       net.ParseIP("fe80::dead:beef"),
			zone:     "enp0s31f6",
			expected: "fe80::dead:beef%enp0s31f6",
		},
		{
			name:     "compact notation",
			ip:       net.ParseIP("fe80::1:2:3:4"),
			zone:     "wlan0",
			expected: "fe80::1:2:3:4%wlan0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatIPv6WithZone(tt.ip, tt.zone)
			if result != tt.expected {
				t.Errorf("formatIPv6WithZone(%v, %v) = %v, want %v", tt.ip, tt.zone, result, tt.expected)
			}
		})
	}
}

func TestGetMulticastAddr(t *testing.T) {
	tests := []struct {
		port     int
		expected string
	}{
		{9999, "[ff02::1]:9999"},
		{1234, "[ff02::1]:1234"},
		{8080, "[ff02::1]:8080"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := GetMulticastAddr(tt.port)
			if result != tt.expected {
				t.Errorf("GetMulticastAddr(%d) = %v, want %v", tt.port, result, tt.expected)
			}
		})
	}
}

func TestParseSourceAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with zone",
			input:    "fe80::1%eth0",
			expected: "fe80::1",
		},
		{
			name:     "without zone",
			input:    "fe80::2",
			expected: "fe80::2",
		},
		{
			name:     "bracketed with zone",
			input:    "[fe80::3%eth1]",
			expected: "fe80::3",
		},
		{
			name:     "bracketed without zone",
			input:    "[fe80::4]",
			expected: "fe80::4",
		},
		{
			name:     "bracketed with port",
			input:    "[fe80::5]:9999",
			expected: "fe80::5",
		},
		{
			name:     "bracketed with zone and port",
			input:    "[fe80::6%eth0]:9999",
			expected: "fe80::6",
		},
		{
			name:     "full address",
			input:    "fe80:0000:0000:0000:0000:0000:0000:0001",
			expected: "fe80:0000:0000:0000:0000:0000:0000:0001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSourceAddr(tt.input)
			if result != tt.expected {
				t.Errorf("ParseSourceAddr(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetRDMADeviceForInterface_NotExists(t *testing.T) {
	// Test with a non-existent interface
	result := getRDMADeviceForInterface("nonexistent-interface-xyz")
	if result != "" {
		t.Errorf("expected empty string for non-existent interface, got %v", result)
	}
}

func TestGetRDMANodeGUID_NotExists(t *testing.T) {
	// Test with a non-existent RDMA device
	result := getRDMANodeGUID("nonexistent-rdma-xyz")
	if result != "" {
		t.Errorf("expected empty string for non-existent RDMA device, got %v", result)
	}
}

func TestGetRDMASysImageGUID_NotExists(t *testing.T) {
	// Test with a non-existent RDMA device
	result := getRDMASysImageGUID("nonexistent-rdma-xyz")
	if result != "" {
		t.Errorf("expected empty string for non-existent RDMA device, got %v", result)
	}
}

func TestGetRDMADeviceMapping(t *testing.T) {
	// This test will succeed whether or not there are actual RDMA devices
	result := getRDMADeviceMapping()
	if result == nil {
		t.Error("getRDMADeviceMapping() returned nil")
	}
	// Result may be empty if no RDMA devices present, which is OK
}

func TestGetRDMADevices(t *testing.T) {
	devices, err := GetRDMADevices()
	if err != nil {
		t.Fatalf("GetRDMADevices() error: %v", err)
	}
	if devices == nil {
		t.Error("GetRDMADevices() returned nil map")
	}
	// Result may be empty if no RDMA devices present, which is OK
	
	// If RDMA devices exist, verify structure
	for rdmaDevice, parentIfaces := range devices {
		if rdmaDevice == "" {
			t.Error("empty RDMA device name in result")
		}
		if len(parentIfaces) == 0 {
			t.Errorf("RDMA device %s has no parent interfaces", rdmaDevice)
		}
		for _, iface := range parentIfaces {
			if iface == "" {
				t.Errorf("empty parent interface for RDMA device %s", rdmaDevice)
			}
		}
	}
}

func TestGetActiveInterfaces(t *testing.T) {
	interfaces, err := GetActiveInterfaces()
	if err != nil {
		t.Fatalf("GetActiveInterfaces() error: %v", err)
	}

	// We should have at least loopback, but loopback is filtered out
	// So the list might be empty on systems with no network interfaces
	// Just verify the structure is correct

	for _, iface := range interfaces {
		if iface.Name == "" {
			t.Error("interface with empty name")
		}
		if iface.LinkLocal == "" {
			t.Error("interface with empty link-local address")
		}

		// Verify link-local format
		if iface.LinkLocal[:4] != "fe80" {
			t.Errorf("interface %s: link-local doesn't start with fe80: %v", iface.Name, iface.LinkLocal)
		}

		// Verify zone is present
		if len(iface.LinkLocal) <= 5 || iface.LinkLocal[len(iface.LinkLocal)-len(iface.Name)-1] != '%' {
			t.Errorf("interface %s: link-local missing zone: %v", iface.Name, iface.LinkLocal)
		}

		// If RDMA, verify fields
		if iface.IsRDMA {
			if iface.RDMADevice == "" {
				t.Errorf("interface %s marked as RDMA but RDMADevice is empty", iface.Name)
			}
			// NodeGUID and SysImageGUID might be empty if sysfs read fails
		}
	}
}

func TestInterfaceInfo_Structure(t *testing.T) {
	// Test that InterfaceInfo struct is properly defined
	info := InterfaceInfo{
		Name:         "eth0",
		LinkLocal:    "fe80::1%eth0",
		IsRDMA:       true,
		RDMADevice:   "mlx5_0",
		NodeGUID:     "0x1111",
		SysImageGUID: "0x2222",
	}

	if info.Name != "eth0" {
		t.Error("Name field not set correctly")
	}
	if info.LinkLocal != "fe80::1%eth0" {
		t.Error("LinkLocal field not set correctly")
	}
	if !info.IsRDMA {
		t.Error("IsRDMA field not set correctly")
	}
	if info.RDMADevice != "mlx5_0" {
		t.Error("RDMADevice field not set correctly")
	}
	if info.NodeGUID != "0x1111" {
		t.Error("NodeGUID field not set correctly")
	}
	if info.SysImageGUID != "0x2222" {
		t.Error("SysImageGUID field not set correctly")
	}
}
