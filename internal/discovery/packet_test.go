package discovery

import (
	"encoding/json"
	"testing"
)

func TestPacket_Marshal(t *testing.T) {
	packet := Packet{
		Hostname:      "test-host",
		MachineID:     "test-machine-id",
		Timestamp:     1234567890,
		Interface:     "eth0",
		SourceIP:      "fe80::1",
		RDMADevice:    "mlx5_0",
		NodeGUID:      "0x1111:2222:3333:4444",
		SysImageGUID:  "0xaaaa:bbbb:cccc:dddd",
	}
	
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("Failed to marshal packet: %v", err)
	}
	
	// Verify it contains expected fields
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	if result["hostname"] != packet.Hostname {
		t.Errorf("Expected hostname %s, got %v", packet.Hostname, result["hostname"])
	}
	
	if result["machine_id"] != packet.MachineID {
		t.Errorf("Expected machine_id %s, got %v", packet.MachineID, result["machine_id"])
	}
	
	if result["rdma_device"] != packet.RDMADevice {
		t.Errorf("Expected rdma_device %s, got %v", packet.RDMADevice, result["rdma_device"])
	}
}

func TestPacket_Unmarshal(t *testing.T) {
	jsonData := `{
		"hostname": "test-host",
		"machine_id": "test-machine-id",
		"timestamp": 1234567890,
		"interface": "eth0",
		"source_ip": "fe80::1",
		"rdma_device": "mlx5_0",
		"node_guid": "0x1111:2222:3333:4444",
		"sys_image_guid": "0xaaaa:bbbb:cccc:dddd"
	}`
	
	var packet Packet
	if err := json.Unmarshal([]byte(jsonData), &packet); err != nil {
		t.Fatalf("Failed to unmarshal packet: %v", err)
	}
	
	if packet.Hostname != "test-host" {
		t.Errorf("Expected hostname test-host, got %s", packet.Hostname)
	}
	
	if packet.MachineID != "test-machine-id" {
		t.Errorf("Expected machine_id test-machine-id, got %s", packet.MachineID)
	}
	
	if packet.Timestamp != 1234567890 {
		t.Errorf("Expected timestamp 1234567890, got %d", packet.Timestamp)
	}
	
	if packet.RDMADevice != "mlx5_0" {
		t.Errorf("Expected rdma_device mlx5_0, got %s", packet.RDMADevice)
	}
	
	if packet.NodeGUID != "0x1111:2222:3333:4444" {
		t.Errorf("Expected node_guid 0x1111:2222:3333:4444, got %s", packet.NodeGUID)
	}
}

func TestPacket_WithNeighbors(t *testing.T) {
	packet := Packet{
		Hostname:  "test-host",
		MachineID: "test-machine-id",
		Timestamp: 1234567890,
		Interface: "eth0",
		SourceIP:  "fe80::1",
		Neighbors: []NeighborInfo{
			{
				MachineID:          "neighbor-id-1",
				Hostname:           "neighbor-1",
				LocalInterface:     "eth0",
				LocalAddress:       "fe80::1",
				RemoteInterface:    "eth1",
				RemoteAddress:      "fe80::2",
				RemoteRDMADevice:   "mlx5_1",
				RemoteNodeGUID:     "0x5555:6666:7777:8888",
				RemoteSysImageGUID: "0xeeee:ffff:0000:1111",
			},
			{
				MachineID:       "neighbor-id-2",
				Hostname:        "neighbor-2",
				LocalInterface:  "eth1",
				LocalAddress:    "fe80::3",
				RemoteInterface: "eth0",
				RemoteAddress:   "fe80::4",
			},
		},
	}
	
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("Failed to marshal packet with neighbors: %v", err)
	}
	
	// Unmarshal and verify
	var decoded Packet
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal packet with neighbors: %v", err)
	}
	
	if len(decoded.Neighbors) != 2 {
		t.Fatalf("Expected 2 neighbors, got %d", len(decoded.Neighbors))
	}
	
	// Verify first neighbor
	n1 := decoded.Neighbors[0]
	if n1.MachineID != "neighbor-id-1" {
		t.Errorf("Expected neighbor machine_id neighbor-id-1, got %s", n1.MachineID)
	}
	
	if n1.RemoteRDMADevice != "mlx5_1" {
		t.Errorf("Expected neighbor RDMA device mlx5_1, got %s", n1.RemoteRDMADevice)
	}
	
	if n1.LocalInterface != "eth0" {
		t.Errorf("Expected local interface eth0, got %s", n1.LocalInterface)
	}
	
	if n1.RemoteInterface != "eth1" {
		t.Errorf("Expected remote interface eth1, got %s", n1.RemoteInterface)
	}
	
	// Verify second neighbor (no RDMA)
	n2 := decoded.Neighbors[1]
	if n2.RemoteRDMADevice != "" {
		t.Errorf("Expected no RDMA device for neighbor 2, got %s", n2.RemoteRDMADevice)
	}
}

func TestPacket_EmptyNeighbors(t *testing.T) {
	packet := Packet{
		Hostname:  "test-host",
		MachineID: "test-machine-id",
		Timestamp: 1234567890,
		Interface: "eth0",
		SourceIP:  "fe80::1",
		Neighbors: []NeighborInfo{},
	}
	
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("Failed to marshal packet: %v", err)
	}
	
	var decoded Packet
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal packet: %v", err)
	}
	
	// Empty neighbors slice gets omitted during marshal (omitempty tag)
	// and comes back as nil during unmarshal
	if len(decoded.Neighbors) != 0 {
		t.Errorf("Expected 0 neighbors, got %d", len(decoded.Neighbors))
	}
}

func TestPacket_OmitEmptyRDMA(t *testing.T) {
	packet := Packet{
		Hostname:  "test-host",
		MachineID: "test-machine-id",
		Timestamp: 1234567890,
		Interface: "eth0",
		SourceIP:  "fe80::1",
		// No RDMA fields set
	}
	
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("Failed to marshal packet: %v", err)
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	// RDMA fields should be omitted when empty
	if _, exists := result["rdma_device"]; exists {
		t.Error("Expected rdma_device to be omitted when empty")
	}
	
	if _, exists := result["node_guid"]; exists {
		t.Error("Expected node_guid to be omitted when empty")
	}
	
	if _, exists := result["sys_image_guid"]; exists {
		t.Error("Expected sys_image_guid to be omitted when empty")
	}
}

func TestNeighborInfo_CompleteEdgeInformation(t *testing.T) {
	neighbor := NeighborInfo{
		MachineID:          "neighbor-id",
		Hostname:           "neighbor-host",
		LocalInterface:     "eth0",
		LocalAddress:       "fe80::1",
		LocalRDMADevice:    "mlx5_0",
		LocalNodeGUID:      "0x1111:2222:3333:4444",
		LocalSysImageGUID:  "0xaaaa:bbbb:cccc:dddd",
		RemoteInterface:    "eth1",
		RemoteAddress:      "fe80::2",
		RemoteRDMADevice:   "mlx5_1",
		RemoteNodeGUID:     "0x5555:6666:7777:8888",
		RemoteSysImageGUID: "0xeeee:ffff:0000:1111",
	}
	
	data, err := json.Marshal(neighbor)
	if err != nil {
		t.Fatalf("Failed to marshal neighbor: %v", err)
	}
	
	var decoded NeighborInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal neighbor: %v", err)
	}
	
	// Verify all fields are preserved
	if decoded.LocalInterface != neighbor.LocalInterface {
		t.Errorf("Expected local interface %s, got %s", neighbor.LocalInterface, decoded.LocalInterface)
	}
	
	if decoded.LocalRDMADevice != neighbor.LocalRDMADevice {
		t.Errorf("Expected local RDMA device %s, got %s", neighbor.LocalRDMADevice, decoded.LocalRDMADevice)
	}
	
	if decoded.RemoteInterface != neighbor.RemoteInterface {
		t.Errorf("Expected remote interface %s, got %s", neighbor.RemoteInterface, decoded.RemoteInterface)
	}
	
	if decoded.RemoteRDMADevice != neighbor.RemoteRDMADevice {
		t.Errorf("Expected remote RDMA device %s, got %s", neighbor.RemoteRDMADevice, decoded.RemoteRDMADevice)
	}
}
