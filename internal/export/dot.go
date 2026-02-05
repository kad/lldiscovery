package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kad.name/lldiscovery/internal/graph"
)

// calculatePenwidth returns the line thickness based on link speed in Mbps
// Returns sensible defaults for common speeds
func calculatePenwidth(speedMbps int) float64 {
	if speedMbps == 0 {
		// Unknown speed - use default thickness
		return 1.0
	}

	// Map speed ranges to line thickness
	switch {
	case speedMbps >= 100000: // 100+ Gbps
		return 5.0
	case speedMbps >= 40000: // 40-100 Gbps
		return 4.0
	case speedMbps >= 10000: // 10-40 Gbps
		return 3.0
	case speedMbps >= 1000: // 1-10 Gbps
		return 2.0
	case speedMbps >= 100: // 100 Mbps - 1 Gbps
		return 1.5
	default: // < 100 Mbps
		return 1.0
	}
}

func GenerateDOT(nodes map[string]*graph.Node, edges map[string]map[string][]*graph.Edge) string {
	return GenerateDOTWithSegments(nodes, edges, nil)
}

func GenerateDOTWithSegments(nodes map[string]*graph.Node, edges map[string]map[string][]*graph.Edge, segments []graph.NetworkSegment) string {
	var sb strings.Builder

	sb.WriteString("graph lldiscovery {\n")
	sb.WriteString("  // Layout hints for better visualization\n")
	if len(segments) > 0 {
		sb.WriteString("  layout=fdp;\n")
	} else {
		sb.WriteString("  rankdir=LR;\n") // Left-to-right for non-segment graphs
	}
	sb.WriteString("  node [shape=box, style=rounded];\n")
	sb.WriteString("  // Each machine is a subgraph (cluster) with interface nodes\n")
	sb.WriteString("  // Direct links: BOLD lines\n")
	sb.WriteString("  // Indirect links: dashed lines\n")
	sb.WriteString("  // RDMA-to-RDMA connections: BLUE with thick lines\n")
	if len(segments) > 0 {
		sb.WriteString("  // Network segments: yellow ellipses in center, machines around periphery\n")
		sb.WriteString("  // Segment connections: solid lines, thickness based on speed\n")
		sb.WriteString("  // Individual links within segments: hidden\n")
	}
	sb.WriteString("\n")

	// Build map of edges that are part of segments (to hide them)
	// Build a map of segment edges to avoid showing them as individual edges
	// Mark ALL edges between any segment members (both direct and indirect)
	segmentEdgeMap := make(map[string]map[string]bool) // [nodeA:nodeB][interface] -> true

	if len(segments) > 0 {
		// Mark ALL edges between segment members (not just from local node)
		for _, segment := range segments {
			// Build a map of which interface each node uses in this segment
			nodeInterfaces := make(map[string]string) // nodeID -> interface name
			
			// Collect interface information from EdgeInfo
			for nodeID, edgeInfo := range segment.EdgeInfo {
				if edgeInfo.RemoteInterface != "" {
					// This is the interface the remote node uses
					nodeInterfaces[nodeID] = edgeInfo.RemoteInterface
				}
				// Also track the local node's interface (if present in LocalInterface)
				if edgeInfo.LocalInterface != "" {
					// Find which node is the local/reference node
					// It's the one that appears in most EdgeInfo entries with same LocalInterface
					// For now, just collect it - we'll identify the reference node below
				}
			}
			
			// Try to identify the reference/local node by finding LocalInterface
			var referenceInterface string
			for _, edgeInfo := range segment.EdgeInfo {
				if edgeInfo.LocalInterface != "" {
					referenceInterface = edgeInfo.LocalInterface
					break
				}
			}
			
			// If we found a reference interface, find which node(s) don't have EdgeInfo entry
			// or have null RemoteInterface - those likely use the reference interface
			if referenceInterface != "" {
				for _, nodeID := range segment.ConnectedNodes {
					if _, exists := nodeInterfaces[nodeID]; !exists {
						// No interface info for this node, it might be the reference node
						nodeInterfaces[nodeID] = referenceInterface
					}
				}
			}
			
			// If still missing interfaces, use segment.Interface as fallback
			for _, nodeID := range segment.ConnectedNodes {
				if _, exists := nodeInterfaces[nodeID]; !exists {
					nodeInterfaces[nodeID] = segment.Interface
				}
			}
			
			// For each pair of nodes in this segment
			for i, nodeA := range segment.ConnectedNodes {
				for j, nodeB := range segment.ConnectedNodes {
					if i >= j {
						continue // Skip self and duplicates
					}

					// Get the interfaces these nodes use
					interfaceA := nodeInterfaces[nodeA]
					interfaceB := nodeInterfaces[nodeB]

					// Mark edges in both directions
					key1 := nodeA + ":" + nodeB
					key2 := nodeB + ":" + nodeA

					if segmentEdgeMap[key1] == nil {
						segmentEdgeMap[key1] = make(map[string]bool)
					}
					if segmentEdgeMap[key2] == nil {
						segmentEdgeMap[key2] = make(map[string]bool)
					}

					// Mark this edge on these interfaces as part of segment
					segmentEdgeMap[key1][interfaceA] = true
					segmentEdgeMap[key2][interfaceB] = true
				}
			}
		}
	}

	// First pass: collect which interfaces have connections
	connectedInterfaces := make(map[string]map[string]bool) // [machineID][interface] -> true
	for srcMachineID, dests := range edges {
		if connectedInterfaces[srcMachineID] == nil {
			connectedInterfaces[srcMachineID] = make(map[string]bool)
		}
		for dstMachineID, edgeList := range dests {
			if connectedInterfaces[dstMachineID] == nil {
				connectedInterfaces[dstMachineID] = make(map[string]bool)
			}
			for _, edge := range edgeList {
				connectedInterfaces[srcMachineID][edge.LocalInterface] = true
				connectedInterfaces[dstMachineID][edge.RemoteInterface] = true
			}
		}
	}

	// Generate machine subgraphs with interface nodes
	for machineID, node := range nodes {
		shortID := machineID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		// Create subgraph (cluster) for this machine
		sb.WriteString(fmt.Sprintf("  subgraph cluster_%s {\n", machineID))
		sb.WriteString("    style=rounded;\n")

		// Different colors for local vs remote machines
		if node.IsLocal {
			sb.WriteString("    color=blue;\n")
			sb.WriteString("    label=\"" + node.Hostname + " (local)\\n" + shortID + "\";\n")
		} else {
			sb.WriteString("    color=black;\n")
			sb.WriteString("    label=\"" + node.Hostname + "\\n" + shortID + "\";\n")
		}

		// Create interface nodes inside the subgraph
		hasInterfaces := false
		for iface, details := range node.Interfaces {
			// Only show interfaces that have connections
			if !connectedInterfaces[machineID][iface] {
				continue
			}
			hasInterfaces = true

			// Build interface node ID
			ifaceNodeID := fmt.Sprintf("%s__%s", machineID, iface)

			// Build interface label with IP and RDMA info
			ifaceLabel := iface
			if details.IPAddress != "" {
				ifaceLabel += fmt.Sprintf("\\n%s", details.IPAddress)
			}
			if details.Speed > 0 {
				ifaceLabel += fmt.Sprintf("\\n%d Mbps", details.Speed)
			}
			if details.RDMADevice != "" {
				ifaceLabel += fmt.Sprintf("\\n[%s]", details.RDMADevice)
				// Add RDMA GUIDs if present
				if details.NodeGUID != "" {
					ifaceLabel += fmt.Sprintf("\\nN: %s", details.NodeGUID)
				}
				if details.SysImageGUID != "" {
					ifaceLabel += fmt.Sprintf("\\nS: %s", details.SysImageGUID)
				}
			}

			// Interface node styling
			nodeStyle := "shape=box, style=\"rounded\""
			if details.RDMADevice != "" {
				nodeStyle = "shape=box, style=\"rounded,filled\", fillcolor=\"#e6f3ff\""
			}

			sb.WriteString(fmt.Sprintf("    \"%s\" [label=\"%s\", %s];\n",
				ifaceNodeID, ifaceLabel, nodeStyle))
		}

		// If no interfaces, create a placeholder node
		if !hasInterfaces {
			placeholderID := fmt.Sprintf("%s__placeholder", machineID)
			sb.WriteString(fmt.Sprintf("    \"%s\" [label=\"(no connections)\", shape=plaintext, fontcolor=gray];\n",
				placeholderID))
		}

		sb.WriteString("  }\n\n")
	}

	// Add network segment nodes if provided
	if len(segments) > 0 {
		sb.WriteString("\n  // Network Segments (positioned in center)\n")
		for i, segment := range segments {
			segmentNodeID := fmt.Sprintf("segment_%d", i)

			// Build segment label - prefer network prefix over interface name
			var segmentLabel string
			if segment.NetworkPrefix != "" {
				// Use network prefix as primary label
				segmentLabel = fmt.Sprintf("%s\\n%d nodes", segment.NetworkPrefix, len(segment.ConnectedNodes))
				// Add interface name as secondary info if different from prefix
				segmentLabel += fmt.Sprintf("\\n(%s)", segment.Interface)
			} else {
				// Fall back to interface name
				segmentLabel = fmt.Sprintf("segment: %s\\n%d nodes", segment.Interface, len(segment.ConnectedNodes))
			}

			// Check if all edges have RDMA
			allHaveRDMA := true
			for _, edge := range segment.EdgeInfo {
				if edge.LocalRDMADevice == "" && edge.RemoteRDMADevice == "" {
					allHaveRDMA = false
					break
				}
			}
			if allHaveRDMA && len(segment.EdgeInfo) > 0 {
				segmentLabel += "\\n[RDMA]"
			}

			// Create segment node (ellipse, yellow, with position hint for center)
			sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\", shape=ellipse, style=filled, fillcolor=\"#ffffcc\", pos=\"0,0!\", pin=true];\n",
				segmentNodeID, segmentLabel))

			// Connect segment to each member node's interface
			for _, nodeID := range segment.ConnectedNodes {
				// Get edge info for this node if available
				edge, hasEdge := segment.EdgeInfo[nodeID]

				if hasEdge {
					// Build interface node ID for the connection
					ifaceNodeID := fmt.Sprintf("%s__%s", nodeID, edge.RemoteInterface)

					// Build edge label with address info
					edgeLabel := edge.RemoteAddress

					// Add speed if available
					if edge.RemoteSpeed > 0 {
						edgeLabel += fmt.Sprintf("\\n%d Mbps", edge.RemoteSpeed)
					}

					// Add RDMA info if present
					if edge.RemoteRDMADevice != "" {
						edgeLabel += fmt.Sprintf("\\n[%s]", edge.RemoteRDMADevice)
					}

					// Calculate line thickness based on speed
					penwidth := calculatePenwidth(edge.RemoteSpeed)

					// Solid lines with speed-based thickness for segment connections
					styleAttr := fmt.Sprintf("style=solid, penwidth=%.1f, color=gray", penwidth)
					if edge.RemoteRDMADevice != "" && edge.LocalRDMADevice != "" {
						// RDMA segments get blue color
						styleAttr = fmt.Sprintf("style=solid, penwidth=%.1f, color=blue", penwidth)
					}

					sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\" [label=\"%s\", %s];\n",
						segmentNodeID, ifaceNodeID, edgeLabel, styleAttr))
				} else {
					// No edge info - try to find any interface on this node
					// (shouldn't happen with proper segment detection, but handle gracefully)
					for iface := range connectedInterfaces[nodeID] {
						ifaceNodeID := fmt.Sprintf("%s__%s", nodeID, iface)
						sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\" [style=solid, color=gray];\n",
							segmentNodeID, ifaceNodeID))
						break // Just connect to first interface
					}
				}
			}
		}
	}

	// Add edges between interface nodes (excluding those in segments on matching interfaces)
	sb.WriteString("\n  // Connections between interfaces\n")
	edgesAdded := make(map[string]bool) // Track to avoid showing both directions of same edge
	for srcMachineID, dests := range edges {
		for dstMachineID, edgeList := range dests {
			for _, edge := range edgeList {
				// Check if this edge is part of a segment (if segments are enabled)
				if len(segments) > 0 {
					edgeKey := srcMachineID + ":" + dstMachineID
					// Check if this specific edge (on this interface) is part of a segment
					if interfaceMap, exists := segmentEdgeMap[edgeKey]; exists {
						if interfaceMap[edge.LocalInterface] {
							continue // Skip this edge - it's represented by the segment
						}
					}
				}

				// Build interface node IDs
				srcIfaceNodeID := fmt.Sprintf("%s__%s", srcMachineID, edge.LocalInterface)
				dstIfaceNodeID := fmt.Sprintf("%s__%s", dstMachineID, edge.RemoteInterface)

				// Create a canonical edge key for deduplication
				edgeKey := fmt.Sprintf("%s--%s", srcIfaceNodeID, dstIfaceNodeID)
				reverseKey := fmt.Sprintf("%s--%s", dstIfaceNodeID, srcIfaceNodeID)

				if edgesAdded[edgeKey] || edgesAdded[reverseKey] {
					continue
				}
				edgesAdded[edgeKey] = true

				// Build simplified edge label (addresses only, since interface info is in nodes)
				edgeLabel := fmt.Sprintf("%s <-> %s", edge.LocalAddress, edge.RemoteAddress)

				// Add speed information if available
				if edge.LocalSpeed > 0 || edge.RemoteSpeed > 0 {
					speedLine := ""
					if edge.LocalSpeed > 0 {
						speedLine += fmt.Sprintf("%d", edge.LocalSpeed)
					}
					if edge.RemoteSpeed > 0 && edge.RemoteSpeed != edge.LocalSpeed {
						speedLine += fmt.Sprintf(" <-> %d Mbps", edge.RemoteSpeed)
					} else if edge.LocalSpeed > 0 {
						speedLine += " Mbps"
					} else if edge.RemoteSpeed > 0 {
						speedLine += fmt.Sprintf("%d Mbps", edge.RemoteSpeed)
					}
					edgeLabel += "\\n" + speedLine
				}

				// Check RDMA status on both sides
				hasLocalRDMA := edge.LocalRDMADevice != ""
				hasRemoteRDMA := edge.RemoteRDMADevice != ""
				bothRDMA := hasLocalRDMA && hasRemoteRDMA

				// Add RDMA-to-RDMA indicator (device names are in interface nodes)
				if bothRDMA {
					edgeLabel += "\\n[RDMA-to-RDMA]"
				}

				// Calculate line thickness based on speed
				maxSpeed := edge.LocalSpeed
				if edge.RemoteSpeed > maxSpeed {
					maxSpeed = edge.RemoteSpeed
				}

				penwidth := calculatePenwidth(maxSpeed)

				// Build edge attributes - highlight RDMA-to-RDMA connections and indirect edges
				var edgeAttrs string
				var styleExtra string

				if edge.Direct {
					// Direct links: bold (solid with thicker line)
					styleExtra = ", style=\"bold\""
				} else {
					// Indirect links: dashed
					styleExtra = ", style=\"dashed\""
				}

				if bothRDMA {
					// Both sides have RDMA - colored edge with speed-based thickness
					edgeAttrs = fmt.Sprintf(" [label=\"%s\", color=\"blue\", penwidth=%.1f%s]", edgeLabel, penwidth, styleExtra)
				} else {
					// Normal edge with speed-based thickness
					edgeAttrs = fmt.Sprintf(" [label=\"%s\", penwidth=%.1f%s]", edgeLabel, penwidth, styleExtra)
				}

				sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\"%s;\n",
					srcIfaceNodeID, dstIfaceNodeID, edgeAttrs))
			}
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// WriteDOTFile writes DOT content to a file
func WriteDOTFile(filename, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
