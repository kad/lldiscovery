package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
			// Build a map of which interfaces each node uses in this segment
			// A node can have MULTIPLE interfaces on the same segment (e.g., wired + WiFi)
			nodeInterfaces := make(map[string][]string) // nodeID -> list of interface names

			// Collect interface information from EdgeInfo
			// Sort node IDs for deterministic processing
			var edgeInfoNodeIDs []string
			for nodeID := range segment.EdgeInfo {
				edgeInfoNodeIDs = append(edgeInfoNodeIDs, nodeID)
			}
			sort.Strings(edgeInfoNodeIDs)

			for _, nodeID := range edgeInfoNodeIDs {
				edgeInfo := segment.EdgeInfo[nodeID]
				if edgeInfo.RemoteInterface != "" {
					// This is the interface the remote node uses
					nodeInterfaces[nodeID] = append(nodeInterfaces[nodeID], edgeInfo.RemoteInterface)
				}
			}

			// Try to identify the reference/local node by finding LocalInterface
			var referenceInterfaces []string
			for _, nodeID := range edgeInfoNodeIDs {
				edgeInfo := segment.EdgeInfo[nodeID]
				if edgeInfo.LocalInterface != "" {
					// Collect all unique local interfaces
					found := false
					for _, iface := range referenceInterfaces {
						if iface == edgeInfo.LocalInterface {
							found = true
							break
						}
					}
					if !found {
						referenceInterfaces = append(referenceInterfaces, edgeInfo.LocalInterface)
					}
				}
			}

			// For nodes without EdgeInfo, check if they have interfaces with segment's prefixes
			// Also check ALL nodes for additional interfaces beyond what EdgeInfo provides
			for _, nodeID := range segment.ConnectedNodes {
				// No interface info from EdgeInfo, OR need to find additional interfaces
				// Check node's interfaces against segment prefixes
				if node, exists := nodes[nodeID]; exists {
					// Sort interface names for deterministic processing
					var ifaceNames []string
					for ifaceName := range node.Interfaces {
						ifaceNames = append(ifaceNames, ifaceName)
					}
					sort.Strings(ifaceNames)

					for _, ifaceName := range ifaceNames {
						// Skip if already added from EdgeInfo
						alreadyAdded := false
						for _, existing := range nodeInterfaces[nodeID] {
							if existing == ifaceName {
								alreadyAdded = true
								break
							}
						}
						if alreadyAdded {
							continue
						}

						ifaceDetails := node.Interfaces[ifaceName]
						// Check if this interface has any of the segment's prefixes
						for _, ifacePrefix := range ifaceDetails.GlobalPrefixes {
							for _, segPrefix := range segment.NetworkPrefixes {
								if ifacePrefix == segPrefix {
									// This interface is on this segment
									nodeInterfaces[nodeID] = append(nodeInterfaces[nodeID], ifaceName)
									break
								}
							}
						}
					}
				}

				// If still no interfaces found, use reference interfaces as fallback
				if len(nodeInterfaces[nodeID]) == 0 && len(referenceInterfaces) > 0 {
					nodeInterfaces[nodeID] = referenceInterfaces
				}

				// Final fallback: use segment.Interface
				if len(nodeInterfaces[nodeID]) == 0 {
					nodeInterfaces[nodeID] = []string{segment.Interface}
				}
			}

			// For each pair of nodes in this segment, mark ALL interface combinations
			for i, nodeA := range segment.ConnectedNodes {
				for j, nodeB := range segment.ConnectedNodes {
					if i >= j {
						continue // Skip self and duplicates
					}

					// Get all interfaces these nodes use
					interfacesA := nodeInterfaces[nodeA]
					interfacesB := nodeInterfaces[nodeB]

					// Mark edges for ALL interface combinations
					for _, interfaceA := range interfacesA {
						for _, interfaceB := range interfacesB {
							// Mark edges in both directions
							key1 := nodeA + ":" + nodeB
							key2 := nodeB + ":" + nodeA

							if segmentEdgeMap[key1] == nil {
								segmentEdgeMap[key1] = make(map[string]bool)
							}
							if segmentEdgeMap[key2] == nil {
								segmentEdgeMap[key2] = make(map[string]bool)
							}

							// Mark BOTH interfaces on BOTH directions
							// This ensures we catch edges regardless of which node is src/dst
							segmentEdgeMap[key1][interfaceA] = true
							segmentEdgeMap[key1][interfaceB] = true
							segmentEdgeMap[key2][interfaceA] = true
							segmentEdgeMap[key2][interfaceB] = true
						}
					}
				}
			}
		}
	}

	// First pass: collect which interfaces have connections
	connectedInterfaces := make(map[string]map[string]bool) // [machineID][interface] -> true

	// Sort source machine IDs for deterministic processing
	var srcMachineIDs []string
	for srcMachineID := range edges {
		srcMachineIDs = append(srcMachineIDs, srcMachineID)
	}
	sort.Strings(srcMachineIDs)

	for _, srcMachineID := range srcMachineIDs {
		dests := edges[srcMachineID]
		if connectedInterfaces[srcMachineID] == nil {
			connectedInterfaces[srcMachineID] = make(map[string]bool)
		}

		// Sort destination machine IDs
		var dstMachineIDs []string
		for dstMachineID := range dests {
			dstMachineIDs = append(dstMachineIDs, dstMachineID)
		}
		sort.Strings(dstMachineIDs)

		for _, dstMachineID := range dstMachineIDs {
			edgeList := dests[dstMachineID]
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
	// Sort machine IDs for deterministic output
	var machineIDs []string
	for machineID := range nodes {
		machineIDs = append(machineIDs, machineID)
	}
	sort.Strings(machineIDs)

	for _, machineID := range machineIDs {
		node := nodes[machineID]
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
		// Sort interface names for deterministic output
		var ifaceNames []string
		for iface := range node.Interfaces {
			// Only include interfaces that have connections
			if connectedInterfaces[machineID][iface] {
				ifaceNames = append(ifaceNames, iface)
			}
		}
		sort.Strings(ifaceNames)

		hasInterfaces := len(ifaceNames) > 0
		for _, iface := range ifaceNames {
			details := node.Interfaces[iface]

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

			// Build segment label - show all network prefixes
			var segmentLabel string
			if len(segment.NetworkPrefixes) > 0 {
				// Use all network prefixes as primary label
				// If more than 3 prefixes, show first 3 and "..."
				prefixLabel := ""
				if len(segment.NetworkPrefixes) <= 3 {
					prefixLabel = strings.Join(segment.NetworkPrefixes, "\\n")
				} else {
					prefixLabel = strings.Join(segment.NetworkPrefixes[:3], "\\n") + "\\n..."
				}
				segmentLabel = fmt.Sprintf("%s\\n%d nodes", prefixLabel, len(segment.ConnectedNodes))
				// Add interface name as secondary info
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

			// Connect segment to each member node's interface(s)
			// A node can have multiple interfaces on the same segment (e.g., wired + WiFi)
			for _, nodeID := range segment.ConnectedNodes {
				// Collect all interfaces this node uses on this segment
				var nodeInterfaces []string

				// First, get from EdgeInfo if available
				if edge, hasEdge := segment.EdgeInfo[nodeID]; hasEdge && edge.RemoteInterface != "" {
					nodeInterfaces = append(nodeInterfaces, edge.RemoteInterface)
				}

				// Then, find any other interfaces with matching prefixes
				if node, exists := nodes[nodeID]; exists {
					// Sort interface names for deterministic processing
					var ifaceNames []string
					for ifaceName := range node.Interfaces {
						ifaceNames = append(ifaceNames, ifaceName)
					}
					sort.Strings(ifaceNames)

					for _, ifaceName := range ifaceNames {
						ifaceDetails := node.Interfaces[ifaceName]
						// Skip if already added from EdgeInfo
						alreadyAdded := false
						for _, added := range nodeInterfaces {
							if added == ifaceName {
								alreadyAdded = true
								break
							}
						}
						if alreadyAdded {
							continue
						}

						// Check if this interface has any of the segment's prefixes
						hasMatchingPrefix := false
						for _, ifacePrefix := range ifaceDetails.GlobalPrefixes {
							for _, segPrefix := range segment.NetworkPrefixes {
								if ifacePrefix == segPrefix {
									hasMatchingPrefix = true
									break
								}
							}
							if hasMatchingPrefix {
								break
							}
						}

						if hasMatchingPrefix {
							nodeInterfaces = append(nodeInterfaces, ifaceName)
						}
					}
				}

				// If no interfaces found, fall back to any connected interface
				if len(nodeInterfaces) == 0 {
					for iface := range connectedInterfaces[nodeID] {
						nodeInterfaces = append(nodeInterfaces, iface)
						break // Just use first one as fallback
					}
				}

				// Sort interfaces for deterministic output
				sort.Strings(nodeInterfaces)

				// Connect segment to each interface this node uses
				for _, ifaceName := range nodeInterfaces {
					ifaceNodeID := fmt.Sprintf("%s__%s", nodeID, ifaceName)

					// Get edge info if available for this specific interface
					var edgeLabel string
					var penwidth float64 = 2.0
					var styleAttr string = "style=solid, color=gray"

					if edge, hasEdge := segment.EdgeInfo[nodeID]; hasEdge && edge.RemoteInterface == ifaceName {
						// Build edge label with address info
						edgeLabel = edge.RemoteAddress

						// Add speed if available
						if edge.RemoteSpeed > 0 {
							edgeLabel += fmt.Sprintf("\\n%d Mbps", edge.RemoteSpeed)
						} else {
							// Check if this is a WiFi interface and show default speed
							if strings.Contains(ifaceName, "wl") {
								edgeLabel += "\\n100 Mbps"
							}
						}

						// Add RDMA info if present
						if edge.RemoteRDMADevice != "" {
							edgeLabel += fmt.Sprintf("\\n[%s]", edge.RemoteRDMADevice)
						}

						// Calculate line thickness based on speed
						penwidth = calculatePenwidth(edge.RemoteSpeed)

						// RDMA segments get blue color
						if edge.RemoteRDMADevice != "" && edge.LocalRDMADevice != "" {
							styleAttr = fmt.Sprintf("style=solid, penwidth=%.1f, color=blue", penwidth)
						} else {
							styleAttr = fmt.Sprintf("style=solid, penwidth=%.1f, color=gray", penwidth)
						}
					} else {
						// No edge info for this interface, just connect with basic style
						// Check node's interface details for speed/address
						if node, exists := nodes[nodeID]; exists {
							if ifaceDetails, ok := node.Interfaces[ifaceName]; ok {
								edgeLabel = ifaceDetails.IPAddress
								speed := ifaceDetails.Speed
								if speed == 0 && strings.Contains(ifaceName, "wl") {
									speed = 100 // Default WiFi
								}
								if speed > 0 {
									edgeLabel += fmt.Sprintf("\\n%d Mbps", speed)
								}
								penwidth = calculatePenwidth(speed)
								styleAttr = fmt.Sprintf("style=solid, penwidth=%.1f, color=gray", penwidth)
							}
						}
					}

					if edgeLabel != "" {
						sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\" [label=\"%s\", %s];\n",
							segmentNodeID, ifaceNodeID, edgeLabel, styleAttr))
					} else {
						sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\" [%s];\n",
							segmentNodeID, ifaceNodeID, styleAttr))
					}
				}
			}
		}
	}

	// Add edges between interface nodes (excluding those in segments on matching interfaces)
	sb.WriteString("\n  // Connections between interfaces\n")
	edgesAdded := make(map[string]bool) // Track to avoid showing both directions of same edge

	// Sort source machine IDs for deterministic edge order
	srcMachineIDs = nil // reuse variable
	for srcMachineID := range edges {
		srcMachineIDs = append(srcMachineIDs, srcMachineID)
	}
	sort.Strings(srcMachineIDs)

	for _, srcMachineID := range srcMachineIDs {
		dests := edges[srcMachineID]

		// Sort destination machine IDs
		var dstMachineIDs []string
		for dstMachineID := range dests {
			dstMachineIDs = append(dstMachineIDs, dstMachineID)
		}
		sort.Strings(dstMachineIDs)

		for _, dstMachineID := range dstMachineIDs {
			edgeList := dests[dstMachineID]

			// Sort edges by local interface name for deterministic output
			sort.Slice(edgeList, func(i, j int) bool {
				if edgeList[i].LocalInterface != edgeList[j].LocalInterface {
					return edgeList[i].LocalInterface < edgeList[j].LocalInterface
				}
				// If local interfaces are the same, sort by remote interface
				return edgeList[i].RemoteInterface < edgeList[j].RemoteInterface
			})

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
