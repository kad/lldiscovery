package export

import (
	"strings"
	"testing"

	"kad.name/lldiscovery/internal/graph"
)

func TestExportNwdiag(t *testing.T) {
	g := graph.New()

	// Create local node
	localInterfaces := map[string]graph.InterfaceDetails{
		"eth0": {
			IPAddress:      "fe80::1%eth0",
			GlobalPrefixes: []string{"192.168.1.0/24"},
			Speed:          1000,
		},
	}
	g.SetLocalNode("local-id", "local-host", localInterfaces)

	// Add enough remote nodes to create a segment (need 3+ nodes)
	g.AddOrUpdate("node1-id", "node1", "eth0", "fe80::100", "eth0", "", "", "", 1000, []string{"192.168.1.0/24"}, true, "")
	g.AddOrUpdate("node2-id", "node2", "eth0", "fe80::200", "eth0", "", "", "", 1000, []string{"192.168.1.0/24"}, true, "")
	g.AddOrUpdate("node3-id", "node3", "eth0", "fe80::300", "eth0", "", "", "", 1000, []string{"192.168.1.0/24"}, true, "")

	nodes := g.GetNodes()
	edges := g.GetEdges()
	segments := g.GetNetworkSegments()

	if len(segments) == 0 {
		t.Fatal("Expected at least one segment to be created")
	}

	nwdiag := ExportNwdiag(nodes, edges, segments)

	// Verify output structure
	if !strings.Contains(nwdiag, "@startuml") {
		t.Error("Expected @startuml tag")
	}
	if !strings.Contains(nwdiag, "nwdiag {") {
		t.Error("Expected nwdiag opening")
	}
	if !strings.Contains(nwdiag, "@enduml") {
		t.Error("Expected @enduml tag")
	}

	// Verify network sections are created
	if !strings.Contains(nwdiag, "network") {
		t.Error("Expected at least one network section")
	}

	// Verify nodes are included
	if !strings.Contains(nwdiag, "local_host") {
		t.Error("Expected local_host in output")
	}
	if !strings.Contains(nwdiag, "node1") {
		t.Error("Expected node1 in output")
	}

	// Verify IP addresses are included
	if !strings.Contains(nwdiag, "fe80::") {
		t.Error("Expected IPv6 addresses in output")
	}

	// Verify network prefix or address section
	if !strings.Contains(nwdiag, "address") && !strings.Contains(nwdiag, "192_168_1_0_24") {
		t.Error("Expected network address or identifier in output")
	}

	// Verify description exists (should be hostname)
	if !strings.Contains(nwdiag, "description") {
		t.Error("Expected description field in output")
	}
}

func TestSanitizeHostname(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-host", "my_host"},
		{"my.host.com", "my_host_com"},
		{"192.168.1.1", "node_192_168_1_1"},
		{"simple", "simple"},
		{"host_name", "host_name"},
	}

	for _, tt := range tests {
		result := sanitizeHostname(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeHostname(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExportNwdiagEmptyGraph(t *testing.T) {
	nodes := make(map[string]*graph.Node)
	edges := make(map[string]map[string][]*graph.Edge)
	segments := []graph.NetworkSegment{}

	nwdiag := ExportNwdiag(nodes, edges, segments)

	// Should still produce valid nwdiag structure
	if !strings.Contains(nwdiag, "@startuml") {
		t.Error("Expected @startuml even for empty graph")
	}
	if !strings.Contains(nwdiag, "@enduml") {
		t.Error("Expected @enduml even for empty graph")
	}
}
