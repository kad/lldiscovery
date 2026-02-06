package export

import (
	"fmt"
	"sort"
	"strings"

	"kad.name/lldiscovery/internal/graph"
)

// ExportNwdiag generates a PlantUML nwdiag format representation of the network topology
func ExportNwdiag(nodes map[string]*graph.Node, edges map[string]map[string][]*graph.Edge, segments []graph.NetworkSegment) string {
	var sb strings.Builder

	sb.WriteString("@startuml\n")
	sb.WriteString("nwdiag {\n")

	// Track which node pairs on which interfaces have been added to segments
	// Key: "nodeA:nodeB:interfaceA:interfaceB"
	processedEdges := make(map[string]bool)

	// Export each network segment
	for _, segment := range segments {
		// Determine network name from first prefix (or interface if no prefix)
		networkName := segment.Interface
		if len(segment.NetworkPrefixes) > 0 {
			// Use first prefix for network name
			networkName = strings.ReplaceAll(segment.NetworkPrefixes[0], "/", "_")
			networkName = strings.ReplaceAll(networkName, ":", "_")
			networkName = strings.ReplaceAll(networkName, ".", "_")
		}

		// Determine network speed and color
		// Collect all speeds from segment members
		speeds := make(map[int]int) // speed -> count
		hasRDMA := false
		for _, nodeID := range segment.ConnectedNodes {
			if edge, ok := segment.EdgeInfo[nodeID]; ok {
				// For segments, RemoteSpeed is the speed of the remote node's interface on this segment
				speed := getEffectiveSpeed(edge.RemoteSpeed, edge.RemoteInterface)
				if speed > 0 {
					speeds[speed]++
				}
				if edge.RemoteRDMADevice != "" {
					hasRDMA = true
				}
			}
		}

		// Use the most common speed (mode) as the segment speed
		// This handles cases where bridge interfaces report higher speeds than physical interfaces
		var segmentSpeed int
		var maxCount int
		for speed, count := range speeds {
			if count > maxCount {
				segmentSpeed = speed
				maxCount = count
			}
		}

		networkColor := getNetworkColor(segmentSpeed, hasRDMA)

		sb.WriteString(fmt.Sprintf("  network %s {\n", networkName))

		// Add network address with all prefixes and speed
		if len(segment.NetworkPrefixes) > 0 {
			prefixStr := strings.Join(segment.NetworkPrefixes, ", ")
			if segmentSpeed > 0 {
				sb.WriteString(fmt.Sprintf("    address = \"%s (%d Mbps)\"\n", prefixStr, segmentSpeed))
			} else {
				sb.WriteString(fmt.Sprintf("    address = \"%s\"\n", prefixStr))
			}
		} else if segmentSpeed > 0 {
			sb.WriteString(fmt.Sprintf("    address = \"%d Mbps\"\n", segmentSpeed))
		}

		// Add color
		if networkColor != "" {
			sb.WriteString(fmt.Sprintf("    color = \"%s\"\n", networkColor))
		}

		// Add nodes in this segment
		for _, nodeID := range segment.ConnectedNodes {
			node, exists := nodes[nodeID]
			if !exists {
				continue
			}

			// Get edge info to find the interface and IP
			edge, hasEdge := segment.EdgeInfo[nodeID]
			if !hasEdge {
				// Node without edge info, just add hostname with description
				hostname := sanitizeHostname(node.Hostname)
				sb.WriteString(fmt.Sprintf("    %s [description = \"%s\"", hostname, node.Hostname))
				if node.IsLocal {
					sb.WriteString(", color = \"#90EE90\"")
				}
				sb.WriteString("];\n")
				continue
			}

			hostname := sanitizeHostname(node.Hostname)

			// Determine which interface and address to show
			var ifaceName string
			var ipAddress string
			var speed int
			var rdmaDevice string

			if edge.LocalInterface != "" {
				// Local node - use local interface
				ifaceName = edge.LocalInterface
				ipAddress = edge.LocalAddress
				speed = edge.LocalSpeed
				rdmaDevice = edge.LocalRDMADevice
			} else {
				// Remote node - use remote interface
				ifaceName = edge.RemoteInterface
				ipAddress = edge.RemoteAddress
				speed = edge.RemoteSpeed
				rdmaDevice = edge.RemoteRDMADevice
			}

			// Clean up IPv6 zone identifier
			ipAddress = strings.Split(ipAddress, "%")[0]

			// Build address string with interface and speed
			var addrStr string
			if ipAddress != "" {
				addrStr = ipAddress
			}
			if ifaceName != "" {
				if addrStr != "" {
					addrStr += " (" + ifaceName
				} else {
					addrStr = ifaceName
				}
				if speed > 0 {
					addrStr += fmt.Sprintf(", %d Mbps", speed)
				}
				if rdmaDevice != "" {
					addrStr += fmt.Sprintf(", %s", rdmaDevice)
				}
				if strings.Contains(addrStr, "(") {
					addrStr += ")"
				}
			}

			// Build node entry with description = hostname
			sb.WriteString(fmt.Sprintf("    %s", hostname))
			if addrStr != "" {
				sb.WriteString(fmt.Sprintf(" [address = \"%s\", description = \"%s\"", addrStr, node.Hostname))
			} else {
				sb.WriteString(fmt.Sprintf(" [description = \"%s\"", node.Hostname))
			}

			// Mark as local node if applicable
			if node.IsLocal {
				sb.WriteString(", color = \"#90EE90\"")
			}

			sb.WriteString("];\n")
		}

		sb.WriteString("  }\n")

		// Mark edges in this segment as processed
		// We need to mark ACTUAL edges between segment members, not all possible pairs
		// The EdgeInfo contains edges from the local node to other segment members
		// We also need to check the global edges map for edges between non-local segment members

		// First, identify the local node (the one NOT in EdgeInfo but in ConnectedNodes)
		localNodeID := ""
		for _, nodeID := range segment.ConnectedNodes {
			found := false
			for edgeNodeID := range segment.EdgeInfo {
				if edgeNodeID == nodeID {
					found = true
					break
				}
			}
			if !found {
				localNodeID = nodeID
				break
			}
		}

		// Mark edges from segment EdgeInfo (local node to remote nodes)
		for remoteNodeID, edge := range segment.EdgeInfo {
			if localNodeID != "" && edge.LocalInterface != "" && edge.RemoteInterface != "" {
				key := makeEdgeKeyWithInterfaces(localNodeID, remoteNodeID, edge.LocalInterface, edge.RemoteInterface)
				processedEdges[key] = true
			}
		}

		// Also mark edges between segment members that share the segment's network prefixes
		// Only edges that are part of THIS segment should be marked
		segmentNodes := make(map[string]bool)
		for _, nodeID := range segment.ConnectedNodes {
			segmentNodes[nodeID] = true
		}

		// Create a set of segment prefixes for quick lookup
		segmentPrefixes := make(map[string]bool)
		for _, prefix := range segment.NetworkPrefixes {
			segmentPrefixes[prefix] = true
		}

		for srcNodeID := range segmentNodes {
			if destMap, ok := edges[srcNodeID]; ok {
				for dstNodeID, edgeList := range destMap {
					if segmentNodes[dstNodeID] {
						// Both nodes are in the same segment
						// But only mark edges that share the segment's network prefixes
						for _, edge := range edgeList {
							// Check if this edge shares any prefix with the segment
							edgeInSegment := false
							for _, localPrefix := range edge.LocalPrefixes {
								if segmentPrefixes[localPrefix] {
									edgeInSegment = true
									break
								}
							}
							if !edgeInSegment {
								for _, remotePrefix := range edge.RemotePrefixes {
									if segmentPrefixes[remotePrefix] {
										edgeInSegment = true
										break
									}
								}
							}

							// Only mark as processed if edge is part of this segment
							if edgeInSegment {
								key := makeEdgeKeyWithInterfaces(srcNodeID, dstNodeID, edge.LocalInterface, edge.RemoteInterface)
								processedEdges[key] = true
							}
						}
					}
				}
			}
		}
	}

	// Add point-to-point links as peer networks
	nodesInSegments := make(map[string]bool)
	for _, segment := range segments {
		for _, nodeID := range segment.ConnectedNodes {
			nodesInSegments[nodeID] = true
		}
	}

	peerNetworkIdx := 1

	// Sort source node IDs for deterministic processing
	var srcNodeIDs []string
	for srcNodeID := range edges {
		srcNodeIDs = append(srcNodeIDs, srcNodeID)
	}
	sort.Strings(srcNodeIDs)

	for _, srcNodeID := range srcNodeIDs {
		dests := edges[srcNodeID]

		// Sort destination node IDs
		var dstNodeIDs []string
		for dstNodeID := range dests {
			dstNodeIDs = append(dstNodeIDs, dstNodeID)
		}
		sort.Strings(dstNodeIDs)

		for _, dstNodeID := range dstNodeIDs {
			edgeList := dests[dstNodeID]
			if len(edgeList) == 0 {
				continue
			}

			// Process each edge separately (nodes can have multiple edges on different interfaces)
			for _, edge := range edgeList {
				// Check if this specific edge was already processed as part of a segment
				edgeKey := makeEdgeKeyWithInterfaces(srcNodeID, dstNodeID, edge.LocalInterface, edge.RemoteInterface)
				if processedEdges[edgeKey] {
					continue
				}
				processedEdges[edgeKey] = true

				srcNode := nodes[srcNodeID]
				dstNode := nodes[dstNodeID]
				if srcNode == nil || dstNode == nil {
					continue
				}

				// Determine if this is RDMA
				hasRDMA := edge.LocalRDMADevice != "" || edge.RemoteRDMADevice != ""

				// Determine speed
				maxSpeed := edge.LocalSpeed
				if edge.RemoteSpeed > maxSpeed {
					maxSpeed = edge.RemoteSpeed
				}

				networkColor := getNetworkColor(maxSpeed, hasRDMA)

				peerNetworkName := fmt.Sprintf("p2p_%d", peerNetworkIdx)
				peerNetworkIdx++

				sb.WriteString(fmt.Sprintf("  network %s {\n", peerNetworkName))

				// Collect all unique prefixes from this P2P link
				prefixSet := make(map[string]bool)
				for _, prefix := range edge.LocalPrefixes {
					prefixSet[prefix] = true
				}
				for _, prefix := range edge.RemotePrefixes {
					prefixSet[prefix] = true
				}

				// Sort prefixes for deterministic output
				var prefixes []string
				for prefix := range prefixSet {
					prefixes = append(prefixes, prefix)
				}
				sort.Strings(prefixes)

				// Add address with prefixes and speed for peer network
				if len(prefixes) > 0 {
					// Show prefixes + speed
					sb.WriteString(fmt.Sprintf("    address = \"%s", strings.Join(prefixes, ", ")))
					if maxSpeed > 0 {
						sb.WriteString(fmt.Sprintf(" (%d Mbps", maxSpeed))
						if hasRDMA {
							sb.WriteString(", RDMA")
						}
						sb.WriteString(")")
					}
					sb.WriteString("\"\n")
				} else if maxSpeed > 0 {
					// Fallback to just speed if no prefixes available
					sb.WriteString(fmt.Sprintf("    address = \"P2P (%d Mbps", maxSpeed))
					if hasRDMA {
						sb.WriteString(", RDMA")
					}
					sb.WriteString(")\"\n")
				}

				// Add color
				if networkColor != "" {
					sb.WriteString(fmt.Sprintf("    color = \"%s\"\n", networkColor))
				}

				// Add source node
				srcHostname := sanitizeHostname(srcNode.Hostname)
				srcAddrStr := strings.Split(edge.LocalAddress, "%")[0]
				if edge.LocalInterface != "" {
					srcAddrStr += " (" + edge.LocalInterface
					if edge.LocalRDMADevice != "" {
						srcAddrStr += fmt.Sprintf(", %s", edge.LocalRDMADevice)
					}
					srcAddrStr += ")"
				}
				sb.WriteString(fmt.Sprintf("    %s [address = \"%s\", description = \"%s\"",
					srcHostname, srcAddrStr, srcNode.Hostname))
				if srcNode.IsLocal {
					sb.WriteString(", color = \"#90EE90\"")
				}
				sb.WriteString("];\n")

				// Add destination node
				dstHostname := sanitizeHostname(dstNode.Hostname)
				dstAddrStr := strings.Split(edge.RemoteAddress, "%")[0]
				if edge.RemoteInterface != "" {
					dstAddrStr += " (" + edge.RemoteInterface
					if edge.RemoteRDMADevice != "" {
						dstAddrStr += fmt.Sprintf(", %s", edge.RemoteRDMADevice)
					}
					dstAddrStr += ")"
				}
				sb.WriteString(fmt.Sprintf("    %s [address = \"%s\", description = \"%s\"",
					dstHostname, dstAddrStr, dstNode.Hostname))
				if dstNode.IsLocal {
					sb.WriteString(", color = \"#90EE90\"")
				}
				sb.WriteString("];\n")

				sb.WriteString("  }\n")
			}
		}
	}

	sb.WriteString("}\n")
	sb.WriteString("@enduml\n")

	return sb.String()
}

// getEffectiveSpeed returns the effective speed for an interface
// WiFi interfaces often report 0, so we default them to 100 Mbps
func getEffectiveSpeed(speed int, interfaceName string) int {
	if speed == 0 {
		// WiFi interfaces (wlan, wlp, wl*) typically report 0
		if strings.Contains(interfaceName, "wl") {
			return 100 // Default WiFi to 100 Mbps
		}
		return 100 // Default any unknown speed to 100 Mbps
	}
	return speed
}

// makeEdgeKey creates a canonical key for an edge (order-independent)
func makeEdgeKey(nodeA, nodeB string) string {
	if nodeA < nodeB {
		return nodeA + ":" + nodeB
	}
	return nodeB + ":" + nodeA
}

// makeEdgeKeyWithInterfaces creates a canonical key for an edge with interfaces
func makeEdgeKeyWithInterfaces(nodeA, nodeB, ifaceA, ifaceB string) string {
	if nodeA < nodeB {
		return fmt.Sprintf("%s:%s:%s:%s", nodeA, nodeB, ifaceA, ifaceB)
	}
	return fmt.Sprintf("%s:%s:%s:%s", nodeB, nodeA, ifaceB, ifaceA)
}

// getNetworkColor returns a color based on speed and RDMA presence
func getNetworkColor(speedMbps int, hasRDMA bool) string {
	if hasRDMA {
		return "#87CEEB" // Sky blue for RDMA
	}

	if speedMbps >= 100000 {
		return "#FFD700" // Gold for 100+ Gbps
	} else if speedMbps >= 40000 {
		return "#FFA500" // Orange for 40+ Gbps
	} else if speedMbps >= 10000 {
		return "#90EE90" // Light green for 10+ Gbps
	} else if speedMbps >= 1000 {
		return "#ADD8E6" // Light blue for 1+ Gbps
	} else if speedMbps > 0 {
		return "#D3D3D3" // Light gray for < 1 Gbps
	}

	return "" // Default color
}

// sanitizeHostname converts hostname to valid nwdiag identifier
func sanitizeHostname(hostname string) string {
	// Replace characters that aren't valid in identifiers
	sanitized := strings.ReplaceAll(hostname, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, ".", "_")

	// Ensure it starts with a letter or underscore
	if len(sanitized) > 0 && (sanitized[0] >= '0' && sanitized[0] <= '9') {
		sanitized = "node_" + sanitized
	}

	return sanitized
}
