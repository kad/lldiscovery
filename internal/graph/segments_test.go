package graph

import (
	"testing"
)

// TestGetNetworkSegments_NoSegments verifies that no segments are detected
// when there are fewer than 3 neighbors on any interface.
func TestGetNetworkSegments_NoSegments(t *testing.T) {
	g := New()

	// Local node on eth0
	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Only one remote neighbor on eth0 (below threshold)
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	if len(segments) != 0 {
		t.Errorf("expected no segments, got %d", len(segments))
	}
}

// TestGetNetworkSegments_ThreeNodes verifies that a segment is detected
// when 3+ neighbors are connected on the same interface.
func TestGetNetworkSegments_ThreeNodes(t *testing.T) {
	g := New()

	// Local node on eth0
	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Three neighbors on eth0 (triggers segment)
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-c", "host-c", "eth0", "fe80::3", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-d", "host-d", "eth0", "fe80::4", "eth0", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}

	seg := segments[0]
	if seg.Interface != "eth0" {
		t.Errorf("expected interface eth0, got %s", seg.Interface)
	}
	// ConnectedNodes contains only the neighbors (not the owner node)
	if len(seg.ConnectedNodes) != 3 {
		t.Errorf("expected 3 neighbors in segment, got %d", len(seg.ConnectedNodes))
	}
	if seg.IsComplete {
		t.Error("expected IsComplete=false (neighbors don't all see each other yet)")
	}
	if seg.OwnerNodeID != "machine-a" {
		t.Errorf("expected owner machine-a, got %s", seg.OwnerNodeID)
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

	// Check that we have one segment per interface
	interfaces := make(map[string]bool)
	for _, seg := range segments {
		interfaces[seg.Interface] = true
		if len(seg.ConnectedNodes) != 3 { // 3 neighbors per interface
			t.Errorf("expected 3 neighbors in segment %s, got %d", seg.Interface, len(seg.ConnectedNodes))
		}
	}

	if !interfaces["eth0"] || !interfaces["eth1"] {
		t.Error("expected segments on both eth0 and eth1")
	}
}

// TestGetNetworkSegments_BelowThreshold verifies that segments with
// fewer than 3 neighbors are not detected.
func TestGetNetworkSegments_BelowThreshold(t *testing.T) {
	g := New()

	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	// Only 2 neighbors on eth0 (below threshold of 3)
	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-c", "host-c", "eth0", "fe80::3", "eth0", "", "", "", 0, true, "")

	segments := g.GetNetworkSegments()
	if len(segments) != 0 {
		t.Errorf("expected no segments (below threshold), got %d", len(segments))
	}
}

// TestIsCompleteIsland_EmptySet verifies that an empty neighbor set
// is considered complete (trivially true).
func TestIsCompleteIsland_EmptySet(t *testing.T) {
	g := New()
	result := g.isCompleteIsland("machine-a", []string{})
	if !result {
		t.Error("expected true for empty neighbor set (trivially complete)")
	}
}

// TestIsCompleteIsland_IncompleteTriangle verifies that an
// incomplete triangle is not considered a complete island.
func TestIsCompleteIsland_IncompleteTriangle(t *testing.T) {
	g := New()

	g.SetLocalNode("machine-a", "host-a", map[string]InterfaceDetails{
		"eth0": {IPAddress: "fe80::1"},
	})

	g.AddOrUpdate("machine-b", "host-b", "eth0", "fe80::2", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-c", "host-c", "eth0", "fe80::3", "eth0", "", "", "", 0, true, "")
	g.AddOrUpdate("machine-d", "host-d", "eth0", "fe80::4", "eth0", "", "", "", 0, true, "")

	// Only add B-C edge, missing B-D and C-D
	g.AddOrUpdateIndirectEdge("machine-b", "host-b", "eth0", "fe80::2", "", "", "", 0,
		"eth0", "fe80::3", "", "", "", 0, "machine-c")

	result := g.isCompleteIsland("machine-a", []string{"machine-b", "machine-c", "machine-d"})
	if result {
		t.Error("expected false for incomplete triangle (missing B-D and C-D)")
	}
}
