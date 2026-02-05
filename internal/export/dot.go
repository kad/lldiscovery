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
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=rounded];\n")
	sb.WriteString("  // RDMA-to-RDMA connections shown in BLUE with thick lines\n")
	sb.WriteString("  // Dashed lines indicate indirect connections\n")
	if len(segments) > 0 {
		sb.WriteString("  // Network segments: yellow ellipses representing mutually connected node groups\n")
		sb.WriteString("  // Individual links within segments are hidden\n")
	}
	sb.WriteString("\n")

	// Build map of edges that are part of segments (to hide them)
	// We hide edges from local node to segment members when the edge's local interface
	// matches the segment's interface (meaning the connectivity is through that segment)
	segmentEdgeMap := make(map[string]map[string]bool) // [nodeA:nodeB][interface] -> true
	localNodeID := ""
	segmentInterfaces := make(map[string]map[string]string) // [nodeA][nodeB] -> segment interface
	
	if len(segments) > 0 {
		// Find local node ID
		for nodeID, node := range nodes {
			if node.IsLocal {
				localNodeID = nodeID
				break
			}
		}
		
		if localNodeID != "" {
			for _, segment := range segments {
				// Mark edges from local node to segment members on this specific interface
				for _, nodeID := range segment.ConnectedNodes {
					if nodeID != localNodeID {
						// Store which interface this segment uses
						if segmentInterfaces[localNodeID] == nil {
							segmentInterfaces[localNodeID] = make(map[string]string)
						}
						segmentInterfaces[localNodeID][nodeID] = segment.Interface
						
						key1 := localNodeID + ":" + nodeID
						key2 := nodeID + ":" + localNodeID
						
						if segmentEdgeMap[key1] == nil {
							segmentEdgeMap[key1] = make(map[string]bool)
						}
						if segmentEdgeMap[key2] == nil {
							segmentEdgeMap[key2] = make(map[string]bool)
						}
						
						// Mark this edge on this interface as part of segment
						segmentEdgeMap[key1][segment.Interface] = true
						segmentEdgeMap[key2][segment.Interface] = true
					}
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

	// Generate nodes - only show connected interfaces and RDMA info
	for machineID, node := range nodes {
		shortID := machineID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		var ifaceList []string
		for iface, details := range node.Interfaces {
			// Only show interfaces that have connections
			if !connectedInterfaces[machineID][iface] {
				continue
			}

			ifaceStr := iface
			// Add RDMA device name if present
			if details.RDMADevice != "" {
				ifaceStr += fmt.Sprintf(" [%s]", details.RDMADevice)
			}
			// Add RDMA GUIDs if present (compact format)
			if details.NodeGUID != "" || details.SysImageGUID != "" {
				if details.NodeGUID != "" {
					ifaceStr += fmt.Sprintf("\\nN: %s", details.NodeGUID)
				}
				if details.SysImageGUID != "" {
					ifaceStr += fmt.Sprintf("\\nS: %s", details.SysImageGUID)
				}
			}
			ifaceList = append(ifaceList, ifaceStr)
		}

		var label string
		if len(ifaceList) > 0 {
			ifaceStr := strings.Join(ifaceList, "\\n")
			label = fmt.Sprintf("%s\\n%s\\n%s",
				node.Hostname,
				shortID,
				ifaceStr)
		} else {
			// No connected interfaces - just show hostname and ID
			label = fmt.Sprintf("%s\\n%s",
				node.Hostname,
				shortID)
		}

		// Highlight local node with different style
		if node.IsLocal {
			sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s (local)\", style=\"rounded,filled\", fillcolor=\"lightblue\"];\n",
				machineID, label))
		} else {
			sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\"];\n",
				machineID, label))
		}
	}

	// Add network segment nodes if provided
	if len(segments) > 0 {
		sb.WriteString("\n  // Network Segments\n")
		for i, segment := range segments {
			segmentNodeID := fmt.Sprintf("segment_%d", i)
			
			// Build segment label with interface and node count
			segmentLabel := fmt.Sprintf("segment: %s\\n%d nodes", segment.Interface, len(segment.ConnectedNodes))
			
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
			
			// Create segment node (ellipse, yellow)
			sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\", shape=ellipse, style=filled, fillcolor=\"#ffffcc\"];\n",
				segmentNodeID, segmentLabel))
			
			// Connect segment to each member node with edge details
			for _, nodeID := range segment.ConnectedNodes {
				// Get edge info for this node if available
				edge, hasEdge := segment.EdgeInfo[nodeID]
				
				if hasEdge {
					// Build edge label with interface and address info
					edgeLabel := fmt.Sprintf("%s\\n%s", edge.RemoteInterface, edge.RemoteAddress)
					
					// Add speed if available
					if edge.RemoteSpeed > 0 {
						edgeLabel += fmt.Sprintf("\\n%d Mbps", edge.RemoteSpeed)
					}
					
					// Add RDMA info if present
					if edge.RemoteRDMADevice != "" {
						edgeLabel += fmt.Sprintf("\\n[%s]", edge.RemoteRDMADevice)
					}
					
					// Determine if this edge should be highlighted
					styleAttr := "style=dotted, color=gray"
					if edge.RemoteRDMADevice != "" && edge.LocalRDMADevice != "" {
						styleAttr = "style=dotted, color=blue, penwidth=2.0"
					}
					
					sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\" [label=\"%s\", %s];\n",
						segmentNodeID, nodeID, edgeLabel, styleAttr))
				} else {
					// No edge info (shouldn't happen, but handle gracefully)
					sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\" [style=dotted, color=gray];\n",
						segmentNodeID, nodeID))
				}
			}
		}
	}

	// Add edges (excluding those in segments on matching interfaces)
	sb.WriteString("\n")
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
				// Create a canonical edge key for deduplication (sorted + interface pair)
				edgeKey := fmt.Sprintf("%s:%s--%s:%s", srcMachineID, edge.LocalInterface, dstMachineID, edge.RemoteInterface)
				reverseKey := fmt.Sprintf("%s:%s--%s:%s", dstMachineID, edge.RemoteInterface, srcMachineID, edge.LocalInterface)

				if edgesAdded[edgeKey] || edgesAdded[reverseKey] {
					continue
				}
				edgesAdded[edgeKey] = true

				// Build edge label with addresses
				edgeLabel := fmt.Sprintf("%s (%s) <-> %s (%s)",
					edge.LocalInterface, edge.LocalAddress,
					edge.RemoteInterface, edge.RemoteAddress)

				// Add speed information if available
				if edge.LocalSpeed > 0 || edge.RemoteSpeed > 0 {
					speedLine := "Speed:"
					if edge.LocalSpeed > 0 {
						speedLine += fmt.Sprintf(" %d Mbps", edge.LocalSpeed)
					}
					if edge.RemoteSpeed > 0 && edge.RemoteSpeed != edge.LocalSpeed {
						speedLine += fmt.Sprintf(" <-> %d Mbps", edge.RemoteSpeed)
					} else if edge.LocalSpeed == 0 && edge.RemoteSpeed > 0 {
						speedLine += fmt.Sprintf(" %d Mbps", edge.RemoteSpeed)
					}
					edgeLabel += "\\n" + speedLine
				}

				// Check RDMA status on both sides
				hasLocalRDMA := edge.LocalRDMADevice != ""
				hasRemoteRDMA := edge.RemoteRDMADevice != ""
				bothRDMA := hasLocalRDMA && hasRemoteRDMA

				// Add RDMA info to edge label if present on either side
				var rdmaLines []string

				// Build local RDMA info line
				if hasLocalRDMA {
					localRDMA := fmt.Sprintf("Local: %s", edge.LocalRDMADevice)
					if edge.LocalNodeGUID != "" {
						localRDMA += fmt.Sprintf(" N:%s", edge.LocalNodeGUID)
					}
					if edge.LocalSysImageGUID != "" {
						localRDMA += fmt.Sprintf(" S:%s", edge.LocalSysImageGUID)
					}
					rdmaLines = append(rdmaLines, localRDMA)
				}

				// Build remote RDMA info line
				if hasRemoteRDMA {
					remoteRDMA := fmt.Sprintf("Remote: %s", edge.RemoteRDMADevice)
					if edge.RemoteNodeGUID != "" {
						remoteRDMA += fmt.Sprintf(" N:%s", edge.RemoteNodeGUID)
					}
					if edge.RemoteSysImageGUID != "" {
						remoteRDMA += fmt.Sprintf(" S:%s", edge.RemoteSysImageGUID)
					}
					rdmaLines = append(rdmaLines, remoteRDMA)
				}

				// Add RDMA info to label
				if len(rdmaLines) > 0 {
					for _, line := range rdmaLines {
						edgeLabel += "\\n" + line
					}
				}

				// Add RDMA-to-RDMA indicator
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
				styleExtra := ""
				if !edge.Direct {
					styleExtra = ", style=\"dashed\""
				}

				if bothRDMA {
					// Both sides have RDMA - colored edge with speed-based thickness
					edgeAttrs = fmt.Sprintf(" [label=\"%s\", color=\"blue\", penwidth=%.1f%s]", edgeLabel, penwidth, styleExtra)
				} else if hasLocalRDMA || hasRemoteRDMA {
					// Only one side has RDMA - normal edge with speed-based thickness
					edgeAttrs = fmt.Sprintf(" [label=\"%s\", penwidth=%.1f%s]", edgeLabel, penwidth, styleExtra)
				} else {
					// No RDMA - normal edge with speed-based thickness
					edgeAttrs = fmt.Sprintf(" [label=\"%s\", penwidth=%.1f%s]", edgeLabel, penwidth, styleExtra)
				}

				sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\"%s;\n",
					srcMachineID, dstMachineID, edgeAttrs))
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
