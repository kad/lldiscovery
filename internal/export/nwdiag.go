package export

import (
	"fmt"
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
		// Determine network name and address
		networkName := segment.Interface
		if segment.NetworkPrefix != "" {
			networkName = strings.ReplaceAll(segment.NetworkPrefix, "/", "_")
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
				if edge.RemoteSpeed > 0 {
					speeds[edge.RemoteSpeed]++
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

		// Add network address with speed if available
		if segment.NetworkPrefix != "" {
			if segmentSpeed > 0 {
				sb.WriteString(fmt.Sprintf("    address = \"%s (%d Mbps)\"\n", segment.NetworkPrefix, segmentSpeed))
			} else {
				sb.WriteString(fmt.Sprintf("    address = \"%s\"\n", segment.NetworkPrefix))
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
		// Build a map of which interface each node uses in this segment
		nodeInterfaces := make(map[string]string)

		// First, find the local node's interface from any EdgeInfo entry's LocalInterface
		var localNodeInterface string
		for _, edge := range segment.EdgeInfo {
			if edge.LocalInterface != "" {
				localNodeInterface = edge.LocalInterface
				break
			}
		}

		// Then populate interfaces for all nodes
		for nodeID, edge := range segment.EdgeInfo {
			if edge.RemoteInterface != "" {
				nodeInterfaces[nodeID] = edge.RemoteInterface
			} else if edge.LocalInterface != "" {
				// This is the local node's own entry (empty RemoteInterface)
				nodeInterfaces[nodeID] = edge.LocalInterface
			}
		}

		// For nodes without EdgeInfo entry, try to identify the local node
		// The local node is the one not in EdgeInfo but in ConnectedNodes
		for _, nodeID := range segment.ConnectedNodes {
			if nodeInterfaces[nodeID] == "" {
				// Check if this could be the local node (has edges to other segment members)
				if localNodeInterface != "" {
					nodeInterfaces[nodeID] = localNodeInterface
				} else {
					// Fallback to segment interface
					nodeInterfaces[nodeID] = segment.Interface
				}
			}
		}

		// Mark all pairs in this segment with their interfaces as processed
		for i, nodeA := range segment.ConnectedNodes {
			for j, nodeB := range segment.ConnectedNodes {
				if i >= j {
					continue
				}
				ifaceA := nodeInterfaces[nodeA]
				ifaceB := nodeInterfaces[nodeB]
				if ifaceA != "" && ifaceB != "" {
					// Mark with canonical key (order-independent)
					key := makeEdgeKeyWithInterfaces(nodeA, nodeB, ifaceA, ifaceB)
					processedEdges[key] = true
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
	for srcNodeID, dests := range edges {
		for dstNodeID, edgeList := range dests {
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

				// Add address with speed for peer network
				if maxSpeed > 0 {
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
