package export

import (
	"fmt"
	"strings"

	"kad.name/lldiscovery/internal/graph"
)

// ExportNwdiag generates a PlantUML nwdiag format representation of the network topology
func ExportNwdiag(nodes map[string]*graph.Node, segments []graph.NetworkSegment) string {
	var sb strings.Builder

	sb.WriteString("@startuml\n")
	sb.WriteString("nwdiag {\n")

	// Export each network segment
	for _, segment := range segments {
		// Determine network name and address
		networkName := segment.Interface
		if segment.NetworkPrefix != "" {
			networkName = strings.ReplaceAll(segment.NetworkPrefix, "/", "_")
			networkName = strings.ReplaceAll(networkName, ":", "_")
			networkName = strings.ReplaceAll(networkName, ".", "_")
		}

		sb.WriteString(fmt.Sprintf("  network %s {\n", networkName))

		// Add network address if available
		if segment.NetworkPrefix != "" {
			sb.WriteString(fmt.Sprintf("    address = \"%s\"\n", segment.NetworkPrefix))
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
				// Node without edge info, just add hostname
				hostname := sanitizeHostname(node.Hostname)
				sb.WriteString(fmt.Sprintf("    %s;\n", hostname))
				continue
			}

			hostname := sanitizeHostname(node.Hostname)
			
			// Determine which interface and address to show
			var ifaceName string
			var ipAddress string
			
			if edge.LocalInterface != "" {
				// Local node - use local interface
				ifaceName = edge.LocalInterface
				ipAddress = edge.LocalAddress
			} else {
				// Remote node - use remote interface
				ifaceName = edge.RemoteInterface
				ipAddress = edge.RemoteAddress
			}

			// Clean up IPv6 zone identifier
			ipAddress = strings.Split(ipAddress, "%")[0]

			// Build node entry
			if ipAddress != "" {
				sb.WriteString(fmt.Sprintf("    %s [address = \"%s\"", hostname, ipAddress))
			} else {
				sb.WriteString(fmt.Sprintf("    %s [", hostname))
			}

			// Add interface description
			if ifaceName != "" {
				sb.WriteString(fmt.Sprintf(", description = \"%s\"", ifaceName))
			}

			sb.WriteString("];\n")
		}

		sb.WriteString("  }\n")
	}

	// Add nodes that are not in any segment (isolated or point-to-point only)
	nodesInSegments := make(map[string]bool)
	for _, segment := range segments {
		for _, nodeID := range segment.ConnectedNodes {
			nodesInSegments[nodeID] = true
		}
	}

	isolatedNodes := make([]string, 0)
	for nodeID, node := range nodes {
		if !nodesInSegments[nodeID] {
			isolatedNodes = append(isolatedNodes, sanitizeHostname(node.Hostname))
		}
	}

	if len(isolatedNodes) > 0 {
		sb.WriteString("  // Isolated nodes\n")
		for _, hostname := range isolatedNodes {
			sb.WriteString(fmt.Sprintf("  %s;\n", hostname))
		}
	}

	sb.WriteString("}\n")
	sb.WriteString("@enduml\n")

	return sb.String()
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
