package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kad.name/lldiscovery/internal/graph"
)

func GenerateDOT(nodes map[string]*graph.Node, edges map[string]map[string]*graph.Edge) string {
	var sb strings.Builder

	sb.WriteString("graph lldiscovery {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=rounded];\n\n")

	for machineID, node := range nodes {
		shortID := machineID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		var ifaceList []string
		for iface, details := range node.Interfaces {
			ifaceStr := fmt.Sprintf("%s: %s", iface, details.IPAddress)
			// Add RDMA device name if present
			if details.RDMADevice != "" {
				ifaceStr += fmt.Sprintf(" [%s]", details.RDMADevice)
			}
			// Add RDMA GUIDs if present
			if details.NodeGUID != "" {
				ifaceStr += fmt.Sprintf("\\nNode GUID: %s", details.NodeGUID)
			}
			if details.SysImageGUID != "" {
				ifaceStr += fmt.Sprintf("\\nSys GUID: %s", details.SysImageGUID)
			}
			ifaceList = append(ifaceList, ifaceStr)
		}
		ifaceStr := strings.Join(ifaceList, "\\n")

		label := fmt.Sprintf("%s\\n%s\\n%s",
			node.Hostname,
			shortID,
			ifaceStr)

		// Highlight local node with different style
		if node.IsLocal {
			sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s (local)\", style=\"rounded,filled\", fillcolor=\"lightblue\"];\n",
				machineID, label))
		} else {
			sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\"];\n",
				machineID, label))
		}
	}

	// Add edges
	sb.WriteString("\n")
	edgesAdded := make(map[string]bool) // Track to avoid duplicate edges
	for srcMachineID, dests := range edges {
		for dstMachineID, edge := range dests {
			// Create a canonical edge key (sorted)
			edgeKey := srcMachineID + "--" + dstMachineID
			reverseKey := dstMachineID + "--" + srcMachineID
			
			if edgesAdded[edgeKey] || edgesAdded[reverseKey] {
				continue
			}
			edgesAdded[edgeKey] = true

			edgeLabel := fmt.Sprintf("%s <-> %s", edge.LocalInterface, edge.RemoteInterface)
			sb.WriteString(fmt.Sprintf("  \"%s\" -- \"%s\" [label=\"%s\"];\n",
				srcMachineID, dstMachineID, edgeLabel))
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
