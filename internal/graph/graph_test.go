package graph

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Fatal("New() returned nil")
	}
	if g.nodes == nil {
		t.Error("nodes map not initialized")
	}
	if g.edges == nil {
		t.Error("edges map not initialized")
	}
	if g.changed {
		t.Error("new graph should not be marked as changed")
	}
}

func TestSetLocalNode(t *testing.T) {
	g := New()
	interfaces := map[string]InterfaceDetails{
		"eth0": {
			IPAddress:    "fe80::1",
			RDMADevice:   "mlx5_0",
			NodeGUID:     "0x1111",
			SysImageGUID: "0x2222",
		},
	}

	g.SetLocalNode("machine-123", "testhost", interfaces)

	if g.localNode == nil {
		t.Fatal("local node not set")
	}
	if g.localNode.MachineID != "machine-123" {
		t.Errorf("wrong machine ID: got %v, want %v", g.localNode.MachineID, "machine-123")
	}
	if g.localNode.Hostname != "testhost" {
		t.Errorf("wrong hostname: got %v, want %v", g.localNode.Hostname, "testhost")
	}
	if !g.localNode.IsLocal {
		t.Error("local node not marked as local")
	}
	if len(g.localNode.Interfaces) != 1 {
		t.Errorf("wrong interface count: got %d, want 1", len(g.localNode.Interfaces))
	}
	if !g.changed {
		t.Error("graph should be marked as changed")
	}
}

func TestAddOrUpdate_NewNode(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})
	g.ClearChanges()

	g.AddOrUpdate("remote-456", "remotehost", "eth1", "fe80::2", "eth0", "mlx5_1", "0x3333", "0x4444", true, "")

	node := g.nodes["remote-456"]
	if node == nil {
		t.Fatal("node not added")
	}
	if node.Hostname != "remotehost" {
		t.Errorf("wrong hostname: got %v, want %v", node.Hostname, "remotehost")
	}
	if node.IsLocal {
		t.Error("remote node marked as local")
	}
	if len(node.Interfaces) != 1 {
		t.Errorf("wrong interface count: got %d, want 1", len(node.Interfaces))
	}

	details := node.Interfaces["eth1"]
	if details.IPAddress != "fe80::2" {
		t.Errorf("wrong IP: got %v, want %v", details.IPAddress, "fe80::2")
	}
	if details.RDMADevice != "mlx5_1" {
		t.Errorf("wrong RDMA device: got %v, want %v", details.RDMADevice, "mlx5_1")
	}
	if details.NodeGUID != "0x3333" {
		t.Errorf("wrong node GUID: got %v, want %v", details.NodeGUID, "0x3333")
	}

	if !g.changed {
		t.Error("graph should be marked as changed")
	}
}

func TestAddOrUpdate_UpdateExisting(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	g.AddOrUpdate("remote-456", "oldhost", "eth1", "fe80::2", "eth0", "", "", "", true, "")
	g.ClearChanges()

	// Update hostname
	g.AddOrUpdate("remote-456", "newhost", "eth1", "fe80::2", "eth0", "", "", "", true, "")

	node := g.nodes["remote-456"]
	if node.Hostname != "newhost" {
		t.Errorf("hostname not updated: got %v, want %v", node.Hostname, "newhost")
	}
	if !g.changed {
		t.Error("graph should be marked as changed after hostname update")
	}
}

func TestAddOrUpdate_DirectEdge(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1", RDMADevice: "mlx5_0"},
	})
	g.ClearChanges()

	g.AddOrUpdate("remote-456", "remotehost", "eth1", "fe80::2", "eth0", "mlx5_1", "0x3333", "0x4444", true, "")

	edges := g.edges["local-123"]["remote-456"]
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	edge := edges[0]
	if edge.LocalInterface != "eth0" {
		t.Errorf("wrong local interface: got %v, want eth0", edge.LocalInterface)
	}
	if edge.RemoteInterface != "eth1" {
		t.Errorf("wrong remote interface: got %v, want eth1", edge.RemoteInterface)
	}
	if !edge.Direct {
		t.Error("edge should be marked as direct")
	}
	if edge.RemoteRDMADevice != "mlx5_1" {
		t.Errorf("wrong remote RDMA: got %v, want mlx5_1", edge.RemoteRDMADevice)
	}
}

func TestAddOrUpdate_UpgradeIndirectToDirect(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Add indirect edge
	g.AddOrUpdate("remote-456", "remotehost", "eth1", "fe80::2", "eth0", "", "", "", false, "intermediate")
	
	edges := g.edges["local-123"]["remote-456"]
	if edges[0].Direct {
		t.Error("edge should be indirect initially")
	}

	g.ClearChanges()

	// Upgrade to direct
	g.AddOrUpdate("remote-456", "remotehost", "eth1", "fe80::2", "eth0", "", "", "", true, "")

	edges = g.edges["local-123"]["remote-456"]
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if !edges[0].Direct {
		t.Error("edge should be upgraded to direct")
	}
	if !g.changed {
		t.Error("graph should be marked as changed")
	}
}

func TestAddOrUpdateIndirectEdge(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{})

	// Add intermediate node
	g.AddOrUpdate("intermediate-789", "intermediate", "eth0", "fe80::3", "eth0", "", "", "", true, "")

	g.ClearChanges()

	// Add indirect edge through intermediate
	g.AddOrUpdateIndirectEdge(
		"remote-456", "remotehost",
		"eth1", "fe80::2",
		"mlx5_1", "0x3333", "0x4444",
		"eth0", "fe80::3",
		"", "", "",
		"intermediate-789",
	)

	// Check neighbor node created
	node := g.nodes["remote-456"]
	if node == nil {
		t.Fatal("neighbor node not created")
	}
	if node.Hostname != "remotehost" {
		t.Errorf("wrong hostname: got %v, want remotehost", node.Hostname)
	}

	// Check interface details
	details := node.Interfaces["eth1"]
	if details.IPAddress != "fe80::2" {
		t.Errorf("wrong IP: got %v, want fe80::2", details.IPAddress)
	}
	if details.RDMADevice != "mlx5_1" {
		t.Errorf("wrong RDMA: got %v, want mlx5_1", details.RDMADevice)
	}

	// Check edge from intermediate to neighbor
	edges := g.edges["intermediate-789"]["remote-456"]
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	edge := edges[0]
	if edge.Direct {
		t.Error("indirect edge marked as direct")
	}
	if edge.LearnedFrom != "intermediate-789" {
		t.Errorf("wrong learned from: got %v, want intermediate-789", edge.LearnedFrom)
	}
	if edge.RemoteRDMADevice != "mlx5_1" {
		t.Errorf("wrong RDMA: got %v, want mlx5_1", edge.RemoteRDMADevice)
	}

	if !g.changed {
		t.Error("graph should be marked as changed")
	}
}

func TestRemoveExpired(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Add two nodes
	g.AddOrUpdate("remote-456", "remote1", "eth1", "fe80::2", "eth0", "", "", "", true, "")
	g.AddOrUpdate("remote-789", "remote2", "eth2", "fe80::3", "eth0", "", "", "", true, "")

	// Age out first node
	g.nodes["remote-456"].LastSeen = time.Now().Add(-2 * time.Hour)

	removed := g.RemoveExpired(1 * time.Hour)

	if removed != 1 {
		t.Errorf("wrong removed count: got %d, want 1", removed)
	}

	if _, exists := g.nodes["remote-456"]; exists {
		t.Error("expired node not removed")
	}
	if _, exists := g.nodes["remote-789"]; !exists {
		t.Error("non-expired node removed")
	}

	// Check edges cleaned up
	if edges, exists := g.edges["local-123"]["remote-456"]; exists && len(edges) > 0 {
		t.Error("edges to expired node not removed")
	}

	if !g.changed {
		t.Error("graph should be marked as changed")
	}
}

func TestRemoveExpired_CascadingEdges(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Add intermediate node with indirect edge
	g.AddOrUpdate("intermediate-789", "intermediate", "eth0", "fe80::3", "eth0", "", "", "", true, "")
	g.AddOrUpdate("remote-456", "remote", "eth1", "fe80::2", "", "", "", "", false, "intermediate-789")

	// Age out intermediate node
	g.nodes["intermediate-789"].LastSeen = time.Now().Add(-2 * time.Hour)

	removed := g.RemoveExpired(1 * time.Hour)

	if removed != 1 {
		t.Errorf("wrong removed count: got %d, want 1", removed)
	}

	// Check indirect edges learned from expired node are removed
	if edges, exists := g.edges["local-123"]["remote-456"]; exists {
		for _, edge := range edges {
			if edge.LearnedFrom == "intermediate-789" {
				t.Error("indirect edge learned from expired node not removed")
			}
		}
	}
}

func TestGetNodes(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})
	g.AddOrUpdate("remote-456", "remote", "eth1", "fe80::2", "eth0", "", "", "", true, "")

	nodes := g.GetNodes()

	if len(nodes) != 2 {
		t.Errorf("wrong node count: got %d, want 2", len(nodes))
	}

	local := nodes["local-123"]
	if local == nil {
		t.Fatal("local node not in result")
	}
	if !local.IsLocal {
		t.Error("local node not marked as local")
	}

	remote := nodes["remote-456"]
	if remote == nil {
		t.Fatal("remote node not in result")
	}
	if remote.IsLocal {
		t.Error("remote node marked as local")
	}
}

func TestGetEdges(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1", RDMADevice: "mlx5_0"},
	})
	g.AddOrUpdate("remote-456", "remote", "eth1", "fe80::2", "eth0", "mlx5_1", "0x3333", "0x4444", true, "")

	edges := g.GetEdges()

	localEdges, exists := edges["local-123"]
	if !exists {
		t.Fatal("no edges from local node")
	}

	remoteEdges, exists := localEdges["remote-456"]
	if !exists {
		t.Fatal("no edges to remote node")
	}

	if len(remoteEdges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(remoteEdges))
	}

	edge := remoteEdges[0]
	if edge.LocalInterface != "eth0" {
		t.Errorf("wrong local interface: got %v, want eth0", edge.LocalInterface)
	}
	if edge.RemoteRDMADevice != "mlx5_1" {
		t.Errorf("wrong RDMA: got %v, want mlx5_1", edge.RemoteRDMADevice)
	}
}

func TestGetDirectNeighbors(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1", RDMADevice: "mlx5_0", NodeGUID: "0x1111"},
	})

	// Add direct neighbor
	g.AddOrUpdate("remote-direct", "direct", "eth1", "fe80::2", "eth0", "mlx5_1", "0x3333", "0x4444", true, "")
	
	// Add indirect neighbor
	g.AddOrUpdate("remote-indirect", "indirect", "eth2", "fe80::3", "", "", "", "", false, "intermediate")

	neighbors := g.GetDirectNeighbors()

	if len(neighbors) != 1 {
		t.Fatalf("expected 1 direct neighbor, got %d", len(neighbors))
	}

	neighbor := neighbors[0]
	if neighbor.MachineID != "remote-direct" {
		t.Errorf("wrong machine ID: got %v, want remote-direct", neighbor.MachineID)
	}
	if neighbor.Hostname != "direct" {
		t.Errorf("wrong hostname: got %v, want direct", neighbor.Hostname)
	}
	if neighbor.LocalInterface != "eth0" {
		t.Errorf("wrong local interface: got %v, want eth0", neighbor.LocalInterface)
	}
	if neighbor.RemoteInterface != "eth1" {
		t.Errorf("wrong remote interface: got %v, want eth1", neighbor.RemoteInterface)
	}
	if neighbor.RemoteRDMADevice != "mlx5_1" {
		t.Errorf("wrong RDMA: got %v, want mlx5_1", neighbor.RemoteRDMADevice)
	}
}

func TestGetDirectNeighbors_NoLocalNode(t *testing.T) {
	g := New()
	neighbors := g.GetDirectNeighbors()

	if len(neighbors) != 0 {
		t.Errorf("expected no neighbors without local node, got %d", len(neighbors))
	}
}

func TestHasChanges(t *testing.T) {
	g := New()

	if g.HasChanges() {
		t.Error("new graph should not have changes")
	}

	g.SetLocalNode("local-123", "localhost", nil)

	if !g.HasChanges() {
		t.Error("graph should have changes after SetLocalNode")
	}
}

func TestClearChanges(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", nil)

	g.ClearChanges()

	if g.HasChanges() {
		t.Error("changes not cleared")
	}
}

func TestGetLocalMachineID(t *testing.T) {
	g := New()

	if id := g.GetLocalMachineID(); id != "" {
		t.Errorf("expected empty machine ID, got %v", id)
	}

	g.SetLocalNode("local-123", "localhost", nil)

	if id := g.GetLocalMachineID(); id != "local-123" {
		t.Errorf("wrong machine ID: got %v, want local-123", id)
	}
}

func TestMultipleEdges(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
		"eth1": {IPAddress: "fe80::2"},
	})

	// Add edges on different interfaces to same remote node
	g.AddOrUpdate("remote-456", "remote", "eth10", "fe80::10", "eth0", "", "", "", true, "")
	g.AddOrUpdate("remote-456", "remote", "eth11", "fe80::11", "eth1", "", "", "", true, "")

	edges := g.edges["local-123"]["remote-456"]
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	// Verify both edges exist with different interfaces
	foundEth0 := false
	foundEth1 := false
	for _, edge := range edges {
		if edge.LocalInterface == "eth0" && edge.RemoteInterface == "eth10" {
			foundEth0 = true
		}
		if edge.LocalInterface == "eth1" && edge.RemoteInterface == "eth11" {
			foundEth1 = true
		}
	}

	if !foundEth0 {
		t.Error("eth0 edge not found")
	}
	if !foundEth1 {
		t.Error("eth1 edge not found")
	}
}

func TestConcurrentAccess(t *testing.T) {
	g := New()
	g.SetLocalNode("local-123", "localhost", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			g.AddOrUpdate("remote-456", "remote", "eth1", "fe80::2", "eth0", "", "", "", true, "")
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = g.GetNodes()
			_ = g.GetEdges()
			_ = g.GetDirectNeighbors()
		}
		done <- true
	}()

	<-done
	<-done
}
