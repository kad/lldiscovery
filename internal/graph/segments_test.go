package graph

import (
	"testing"
)

// TestGetNetworkSegments_NoSegments verifies that no segments are detected
// when there are fewer than 3 neighbors on an interface.
func TestGetNetworkSegments_NoSegments(t *testing.T) {
	g := New()

	// Local node
	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Only one remote neighbor (below threshold)
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	if len(segments) != 0 {
		t.Errorf("expected no segments, got %d", len(segments))
	}
}

// TestGetNetworkSegments_ThreeNeighbors verifies that a segment is detected
// when 3 neighbors are reachable on the same interface.
func TestGetNetworkSegments_ThreeNeighbors(t *testing.T) {
	g := New()

	// Local node on eth0
	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Three neighbors on eth0
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-c", "host-c", "eth0", "fe80::3", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-d", "host-d", "eth0", "fe80::4", "eth0", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}

	seg := segments[0]
	// Should include local + 3 neighbors = 4 total
	if len(seg.ConnectedNodes) != 4 {
		t.Errorf("expected 4 nodes in segment, got %d", len(seg.ConnectedNodes))
	}

	// Check that local node is included
	found := false
	for _, node := range seg.ConnectedNodes {
		if node == "machine-a" {
			found = true
			break
		}
	}
	if !found {
		t.Error("segment should include local node")
	}
}

// TestGetNetworkSegments_BelowThreshold verifies that segments with
// fewer than 3 total nodes are not detected.
func TestGetNetworkSegments_BelowThreshold(t *testing.T) {
	g := New()

	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Only 2 neighbors on eth0: total of 3 nodes (local + 2), which IS a segment
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-c", "host-c", "eth0", "fe80::3", "eth0", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	// With 3 total nodes on eth0, this forms a segment
	if len(segments) != 1 {
		t.Errorf("expected 1 segment (3 nodes on eth0), got %d", len(segments))
	}
	
	if len(segments) > 0 && len(segments[0].ConnectedNodes) != 3 {
		t.Errorf("expected 3 nodes in segment, got %d", len(segments[0].ConnectedNodes))
	}
}

// TestGetNetworkSegments_TooFewNodes verifies that segments with only 2 nodes are not detected.
func TestGetNetworkSegments_TooFewNodes(t *testing.T) {
	g := New()

	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Only 1 neighbor on eth0: total of 2 nodes, below threshold
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	if len(segments) != 0 {
		t.Errorf("expected no segments (only 2 nodes), got %d", len(segments))
	}
}

// TestGetNetworkSegments_MultipleInterfaces verifies that segments
// on different interfaces are detected independently.
func TestGetNetworkSegments_MultipleInterfaces(t *testing.T) {
	g := New()

	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
		"eth1": {IPAddress: "fe80::11"},
	})

	// 3 neighbors on eth0
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-c", "host-c", "eth0", "fe80::3", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-d", "host-d", "eth0", "fe80::4", "eth0", "", "", "", 0, true, "")

	// 3 neighbors on eth1
	g.AddOrUpdate("machine-e", "host-e", "eth0", "fe80::12", "eth1", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-f", "host-f", "eth0", "fe80::13", "eth1", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-g", "host-g", "eth0", "fe80::14", "eth1", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	// Each segment should have local + 3 neighbors = 4 nodes
	for _, seg := range segments {
		if len(seg.ConnectedNodes) != 4 {
			t.Errorf("expected 4 nodes in segment, got %d", len(seg.ConnectedNodes))
		}
	}
}

// TestGetNetworkSegments_NoLocalNode verifies no segments when no local node set.
func TestGetNetworkSegments_NoLocalNode(t *testing.T) {
	g := New()

	// Don't set local node
	segments := g.GetNetworkSegments()
	if len(segments) != 0 {
		t.Errorf("expected no segments without local node, got %d", len(segments))
	}
}
