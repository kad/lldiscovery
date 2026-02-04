package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kad.name/lldiscovery/internal/graph"
)

func GenerateDOT(nodes map[string]*graph.Node) string {
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
		for iface, ip := range node.Interfaces {
			ifaceList = append(ifaceList, fmt.Sprintf("%s: %s", iface, ip))
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
