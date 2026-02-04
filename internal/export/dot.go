package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kad.name/lldiscovery/internal/graph"
)

func GenerateDOT(nodes map[string]*graph.Node, edges map[string]map[string][]*graph.Edge) string {
	var sb strings.Builder

	sb.WriteString("graph lldiscovery {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=rounded];\n")
	sb.WriteString("  // RDMA-to-RDMA connections shown in BLUE with thick lines\n")
	sb.WriteString("  // Dashed lines indicate indirect connections\n\n")

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

	// Add all edges (including multiples between same pair of nodes)
	sb.WriteString("\n")
	edgesAdded := make(map[string]bool) // Track to avoid showing both directions of same edge
	for srcMachineID, dests := range edges {
		for dstMachineID, edgeList := range dests {
			for _, edge := range edgeList {
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
				
				// Build edge attributes - highlight RDMA-to-RDMA connections and indirect edges
				var edgeAttrs string
				styleExtra := ""
				if !edge.Direct {
					styleExtra = ", style=\"dashed\""
				}
				
				if bothRDMA {
					// Both sides have RDMA - thick, colored edge
					edgeAttrs = fmt.Sprintf(" [label=\"%s\", color=\"blue\", penwidth=2.0%s]", edgeLabel, styleExtra)
				} else if hasLocalRDMA || hasRemoteRDMA {
					// Only one side has RDMA - normal edge
					if styleExtra != "" {
						edgeAttrs = fmt.Sprintf(" [label=\"%s\"%s]", edgeLabel, styleExtra)
					} else {
						edgeAttrs = fmt.Sprintf(" [label=\"%s\"]", edgeLabel)
					}
				} else {
					// No RDMA - normal edge
					if styleExtra != "" {
						edgeAttrs = fmt.Sprintf(" [label=\"%s\"%s]", edgeLabel, styleExtra)
					} else {
						edgeAttrs = fmt.Sprintf(" [label=\"%s\"]", edgeLabel)
					}
				}
				
				sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\"%s;\n",
					srcMachineID, dstMachineID, edgeAttrs))
			}
		}
	}

	sb.WriteString("}\n")

	return sb.String()
}

func WriteDOTFile(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename file: %w", err)
	}

	return nil
}
