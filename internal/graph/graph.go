package graph

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type NeighborData struct {
	MachineID          string
	Hostname           string
	LocalInterface     string
	LocalAddress       string
	LocalPrefixes      []string // Global unicast network prefixes
	LocalRDMADevice    string
	LocalNodeGUID      string
	LocalSysImageGUID  string
	LocalSpeed         int
	RemoteInterface    string
	RemoteAddress      string
	RemotePrefixes     []string // Global unicast network prefixes
	RemoteRDMADevice   string
	RemoteNodeGUID     string
	RemoteSysImageGUID string
	RemoteSpeed        int
}

type InterfaceDetails struct {
	IPAddress      string
	GlobalPrefixes []string // Global unicast network prefixes
	RDMADevice     string
	NodeGUID       string
	SysImageGUID   string
	Speed          int // Link speed in Mbps
}

type Node struct {
	Hostname   string
	MachineID  string
	LastSeen   time.Time
	Interfaces map[string]InterfaceDetails
	IsLocal    bool
}

type Edge struct {
	LocalInterface     string
	LocalAddress       string
	LocalPrefixes      []string // Global unicast network prefixes
	LocalRDMADevice    string
	LocalNodeGUID      string
	LocalSysImageGUID  string
	LocalSpeed         int // Link speed in Mbps
	RemoteInterface    string
	RemoteAddress      string
	RemotePrefixes     []string // Global unicast network prefixes
	RemoteRDMADevice   string
	RemoteNodeGUID     string
	RemoteSysImageGUID string
	RemoteSpeed        int // Link speed in Mbps
	Direct             bool
	LearnedFrom        string
}

type Graph struct {
	mu        sync.RWMutex
	nodes     map[string]*Node
	localNode *Node
	edges     map[string]map[string][]*Edge // [localMachineID][remoteMachineID] -> []Edge (multiple edges)
	changed   bool
}

func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
		edges: make(map[string]map[string][]*Edge),
	}
}

func (g *Graph) SetLocalNode(machineID, hostname string, interfaces map[string]InterfaceDetails) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.localNode = &Node{
		Hostname:   hostname,
		MachineID:  machineID,
		LastSeen:   time.Now(),
		Interfaces: interfaces,
		IsLocal:    true,
	}
	g.changed = true
}

func (g *Graph) AddOrUpdate(machineID, hostname, remoteIface, sourceIP, receivingIface, rdmaDevice, nodeGUID, sysImageGUID string, remoteSpeed int, remotePrefixes []string, direct bool, learnedFrom string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[machineID]
	if !exists {
		node = &Node{
			Hostname:   hostname,
			MachineID:  machineID,
			Interfaces: make(map[string]InterfaceDetails),
			IsLocal:    false,
		}
		g.nodes[machineID] = node
		g.changed = true
	}

	if node.Hostname != hostname {
		node.Hostname = hostname
		g.changed = true
	}

	node.LastSeen = time.Now()

	// Update interface details
	details := InterfaceDetails{
		IPAddress:      sourceIP,
		GlobalPrefixes: remotePrefixes,
		RDMADevice:     rdmaDevice,
		NodeGUID:       nodeGUID,
		SysImageGUID:   sysImageGUID,
		Speed:          remoteSpeed,
	}

	if existing, ok := node.Interfaces[remoteIface]; !ok || existing.IPAddress != details.IPAddress ||
		existing.RDMADevice != details.RDMADevice || existing.Speed != details.Speed {
		node.Interfaces[remoteIface] = details
		g.changed = true
	}

	// Track edge (connection between interfaces)
	if g.localNode != nil {
		// For indirect edges, receivingIface may be empty
		if _, ok := g.edges[g.localNode.MachineID]; !ok {
			g.edges[g.localNode.MachineID] = make(map[string][]*Edge)
		}

		// Get local interface details (only for direct edges)
		localDetails := InterfaceDetails{}
		if receivingIface != "" {
			if ld, localExists := g.localNode.Interfaces[receivingIface]; localExists {
				localDetails = ld
			}
		}

		edge := &Edge{
			LocalInterface:     receivingIface,
			LocalAddress:       localDetails.IPAddress,
			LocalPrefixes:      localDetails.GlobalPrefixes,
			LocalRDMADevice:    localDetails.RDMADevice,
			LocalNodeGUID:      localDetails.NodeGUID,
			LocalSysImageGUID:  localDetails.SysImageGUID,
			LocalSpeed:         localDetails.Speed,
			RemoteInterface:    remoteIface,
			RemoteAddress:      sourceIP,
			RemotePrefixes:     remotePrefixes,
			RemoteRDMADevice:   rdmaDevice,
			RemoteNodeGUID:     nodeGUID,
			RemoteSysImageGUID: sysImageGUID,
			RemoteSpeed:        remoteSpeed,
			Direct:             direct,
			LearnedFrom:        learnedFrom,
		}

		// Check if this exact edge already exists
		edges := g.edges[g.localNode.MachineID][machineID]
		found := false
		for i, existingEdge := range edges {
			// Match on interfaces (both may be empty for indirect edges with no local iface info)
			if existingEdge.LocalInterface == edge.LocalInterface &&
				existingEdge.RemoteInterface == edge.RemoteInterface {
				// Upgrade indirect edge to direct if direct packet arrives
				if !existingEdge.Direct && direct {
					edges[i] = edge
					g.changed = true
				} else if existingEdge.Direct == direct {
					// Update existing edge of same type
					*existingEdge = *edge
				}
				found = true
				break
			}
		}

		if !found {
			// Add new edge
			g.edges[g.localNode.MachineID][machineID] = append(edges, edge)
			g.changed = true
		}
	}
}

// AddOrUpdateIndirectEdge adds an edge from a neighbor report with complete information about both sides
func (g *Graph) AddOrUpdateIndirectEdge(
	neighborMachineID, neighborHostname,
	neighborIface, neighborAddress,
	neighborRDMA, neighborNodeGUID, neighborSysImageGUID string,
	neighborSpeed int,
	neighborPrefixes []string,
	intermediateIface, intermediateAddress,
	intermediateRDMA, intermediateNodeGUID, intermediateSysImageGUID string,
	intermediateSpeed int,
	intermediatePrefixes []string,
	learnedFrom string) {

	g.mu.Lock()
	defer g.mu.Unlock()

	// Ensure neighbor node exists
	node, exists := g.nodes[neighborMachineID]
	if !exists {
		node = &Node{
			Hostname:   neighborHostname,
			MachineID:  neighborMachineID,
			Interfaces: make(map[string]InterfaceDetails),
			IsLocal:    false,
		}
		g.nodes[neighborMachineID] = node
		g.changed = true
	}

	node.LastSeen = time.Now()

	// Update neighbor's interface details
	neighborDetails := InterfaceDetails{
		IPAddress:      neighborAddress,
		GlobalPrefixes: neighborPrefixes,
		RDMADevice:     neighborRDMA,
		NodeGUID:       neighborNodeGUID,
		SysImageGUID:   neighborSysImageGUID,
		Speed:          neighborSpeed,
	}
	if existing, ok := node.Interfaces[neighborIface]; !ok || existing.IPAddress != neighborDetails.IPAddress ||
		existing.RDMADevice != neighborDetails.RDMADevice || existing.Speed != neighborDetails.Speed {
		node.Interfaces[neighborIface] = neighborDetails
		g.changed = true
	}

	// Also ensure the intermediate node exists and update its interface
	intermediateNode, intermediateExists := g.nodes[learnedFrom]
	if intermediateExists && intermediateIface != "" {
		intermediateDetails := InterfaceDetails{
			IPAddress:      intermediateAddress,
			GlobalPrefixes: intermediatePrefixes,
			RDMADevice:     intermediateRDMA,
			NodeGUID:       intermediateNodeGUID,
			SysImageGUID:   intermediateSysImageGUID,
			Speed:          intermediateSpeed,
		}
		if existing, ok := intermediateNode.Interfaces[intermediateIface]; !ok || existing.IPAddress != intermediateDetails.IPAddress ||
			existing.RDMADevice != intermediateDetails.RDMADevice || existing.Speed != intermediateDetails.Speed {
			intermediateNode.Interfaces[intermediateIface] = intermediateDetails
			g.changed = true
		}
	}

	// Create edge showing the connection between intermediate and neighbor
	// This edge is from intermediate node's perspective, so we store it there
	if intermediateExists {
		if _, ok := g.edges[learnedFrom]; !ok {
			g.edges[learnedFrom] = make(map[string][]*Edge)
		}

		edge := &Edge{
			LocalInterface:     intermediateIface,
			LocalAddress:       intermediateAddress,
			LocalPrefixes:      intermediatePrefixes,
			LocalRDMADevice:    intermediateRDMA,
			LocalNodeGUID:      intermediateNodeGUID,
			LocalSysImageGUID:  intermediateSysImageGUID,
			LocalSpeed:         intermediateSpeed,
			RemoteInterface:    neighborIface,
			RemoteAddress:      neighborAddress,
			RemotePrefixes:     neighborPrefixes,
			RemoteRDMADevice:   neighborRDMA,
			RemoteNodeGUID:     neighborNodeGUID,
			RemoteSysImageGUID: neighborSysImageGUID,
			RemoteSpeed:        neighborSpeed,
			Direct:             false,
			LearnedFrom:        learnedFrom,
		}

		// Check if this edge already exists
		edges := g.edges[learnedFrom][neighborMachineID]
		found := false
		for i, existingEdge := range edges {
			if existingEdge.LocalInterface == edge.LocalInterface &&
				existingEdge.RemoteInterface == edge.RemoteInterface {
				// Update existing indirect edge
				if !existingEdge.Direct {
					edges[i] = edge
				}
				found = true
				break
			}
		}

		if !found {
			g.edges[learnedFrom][neighborMachineID] = append(edges, edge)
			g.changed = true
		}
	}
}

func (g *Graph) RemoveExpired(timeout time.Duration) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	removed := 0
	expiredMachineIDs := []string{}

	for machineID, node := range g.nodes {
		if now.Sub(node.LastSeen) > timeout {
			delete(g.nodes, machineID)
			expiredMachineIDs = append(expiredMachineIDs, machineID)
			removed++
			g.changed = true
		}
	}

	// Cascading deletion: remove edges learned from expired nodes
	if len(expiredMachineIDs) > 0 {
		for srcID, dstMap := range g.edges {
			for dstID, edges := range dstMap {
				// Remove edges to/from expired nodes
				shouldDeleteAll := false
				for _, expiredID := range expiredMachineIDs {
					if srcID == expiredID || dstID == expiredID {
						shouldDeleteAll = true
						break
					}
				}

				if shouldDeleteAll {
					delete(dstMap, dstID)
					if len(dstMap) == 0 {
						delete(g.edges, srcID)
					}
					g.changed = true
					continue
				}

				// Filter out indirect edges learned from expired nodes
				filteredEdges := make([]*Edge, 0, len(edges))
				for _, edge := range edges {
					isLearnedFromExpired := false
					for _, expiredID := range expiredMachineIDs {
						if edge.LearnedFrom == expiredID {
							isLearnedFromExpired = true
							break
						}
					}
					if !isLearnedFromExpired {
						filteredEdges = append(filteredEdges, edge)
					} else {
						g.changed = true
					}
				}

				if len(filteredEdges) == 0 {
					delete(dstMap, dstID)
					if len(dstMap) == 0 {
						delete(g.edges, srcID)
					}
				} else if len(filteredEdges) != len(edges) {
					g.edges[srcID][dstID] = filteredEdges
				}
			}
		}
	}

	return removed
}

func (g *Graph) GetNodes() map[string]*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]*Node)

	// Include local node if set
	if g.localNode != nil {
		nodeCopy := &Node{
			Hostname:   g.localNode.Hostname,
			MachineID:  g.localNode.MachineID,
			LastSeen:   g.localNode.LastSeen,
			Interfaces: make(map[string]InterfaceDetails),
			IsLocal:    true,
		}
		for ik, iv := range g.localNode.Interfaces {
			nodeCopy.Interfaces[ik] = iv
		}
		result[g.localNode.MachineID] = nodeCopy
	}

	// Include discovered nodes
	for k, v := range g.nodes {
		nodeCopy := &Node{
			Hostname:   v.Hostname,
			MachineID:  v.MachineID,
			LastSeen:   v.LastSeen,
			Interfaces: make(map[string]InterfaceDetails),
			IsLocal:    false,
		}
		for ik, iv := range v.Interfaces {
			nodeCopy.Interfaces[ik] = iv
		}
		result[k] = nodeCopy
	}

	return result
}

func (g *Graph) GetEdges() map[string]map[string][]*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]map[string][]*Edge)
	for src, dests := range g.edges {
		result[src] = make(map[string][]*Edge)
		for dst, edges := range dests {
			edgeCopies := make([]*Edge, len(edges))
			for i, edge := range edges {
				edgeCopies[i] = &Edge{
					LocalInterface:     edge.LocalInterface,
					LocalAddress:       edge.LocalAddress,
					LocalPrefixes:      edge.LocalPrefixes,
					LocalRDMADevice:    edge.LocalRDMADevice,
					LocalNodeGUID:      edge.LocalNodeGUID,
					LocalSysImageGUID:  edge.LocalSysImageGUID,
					LocalSpeed:         edge.LocalSpeed,
					RemoteInterface:    edge.RemoteInterface,
					RemoteAddress:      edge.RemoteAddress,
					RemotePrefixes:     edge.RemotePrefixes,
					RemoteRDMADevice:   edge.RemoteRDMADevice,
					RemoteNodeGUID:     edge.RemoteNodeGUID,
					RemoteSysImageGUID: edge.RemoteSysImageGUID,
					RemoteSpeed:        edge.RemoteSpeed,
					Direct:             edge.Direct,
					LearnedFrom:        edge.LearnedFrom,
				}
			}
			result[src][dst] = edgeCopies
		}
	}

	return result
}

func (g *Graph) GetDirectNeighbors() []NeighborData {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := []NeighborData{}

	if g.localNode == nil {
		return result
	}

	// Get all direct edges from local node
	if localEdges, ok := g.edges[g.localNode.MachineID]; ok {
		for dstID, edges := range localEdges {
			for _, edge := range edges {
				if edge.Direct {
					// Get remote node info
					node, exists := g.nodes[dstID]
					if !exists {
						continue
					}

					// Get prefix information from stored interface details
					var localPrefixes, remotePrefixes []string
					if localDetails, ok := g.localNode.Interfaces[edge.LocalInterface]; ok {
						localPrefixes = localDetails.GlobalPrefixes
					}
					if remoteDetails, ok := node.Interfaces[edge.RemoteInterface]; ok {
						remotePrefixes = remoteDetails.GlobalPrefixes
					}

					result = append(result, NeighborData{
						MachineID:          dstID,
						Hostname:           node.Hostname,
						LocalInterface:     edge.LocalInterface,
						LocalAddress:       edge.LocalAddress,
						LocalPrefixes:      localPrefixes,
						LocalRDMADevice:    edge.LocalRDMADevice,
						LocalNodeGUID:      edge.LocalNodeGUID,
						LocalSysImageGUID:  edge.LocalSysImageGUID,
						LocalSpeed:         edge.LocalSpeed,
						RemoteInterface:    edge.RemoteInterface,
						RemoteAddress:      edge.RemoteAddress,
						RemotePrefixes:     remotePrefixes,
						RemoteRDMADevice:   edge.RemoteRDMADevice,
						RemoteNodeGUID:     edge.RemoteNodeGUID,
						RemoteSysImageGUID: edge.RemoteSysImageGUID,
						RemoteSpeed:        edge.RemoteSpeed,
					})
				}
			}
		}
	}

	return result
}

// NetworkSegment represents a group of nodes reachable on a shared network (switch/VLAN)
type NetworkSegment struct {
	ID              string           // Unique ID for this segment
	Interface       string           // Primary interface name (e.g., "eth0")
	NetworkPrefix   string           // Primary network prefix (first in NetworkPrefixes), for backward compatibility
	NetworkPrefixes []string         // All network prefixes on this segment (both IPv4 and IPv6)
	ConnectedNodes  []string         // Machine IDs of nodes in this segment
	EdgeInfo        map[string]*Edge // Map of nodeID -> edge info for connections to segment
}

// GetNetworkSegments finds groups of nodes connected to shared network segments
// GetNetworkSegments finds groups of nodes connected to shared network segments
// Detects both local segments (where local node participates) and remote segments (visible via indirect discovery)
func (g *Graph) GetNetworkSegments() []NetworkSegment {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var segments []NetworkSegment

	// Only meaningful if we have a local node
	if g.localNode == nil {
		return segments
	}

	localID := g.localNode.MachineID
	segmentID := 0

	// Part 1: Detect segments from LOCAL node's perspective
	// Group remote nodes by which LOCAL interface reaches them
	if localEdges, ok := g.edges[localID]; ok {
		interfaceGroups := make(map[string]map[string]*Edge) // [local_iface][remote_node] = edge

		for remoteID, edgeList := range localEdges {
			for _, edge := range edgeList {
				localIface := edge.LocalInterface

				if interfaceGroups[localIface] == nil {
					interfaceGroups[localIface] = make(map[string]*Edge)
				}

				// Prefer direct edges over indirect
				if existing, ok := interfaceGroups[localIface][remoteID]; !ok || (!existing.Direct && edge.Direct) {
					interfaceGroups[localIface][remoteID] = edge
				}
			}
		}

		// Create segments for local interfaces with 2+ remote nodes
		for localIface, remoteNodes := range interfaceGroups {
			if len(remoteNodes) < 2 {
				continue
			}

			// Collect node IDs (local + all remotes)
			nodeIDs := []string{localID}
			edgeInfo := make(map[string]*Edge)

			for remoteID, edge := range remoteNodes {
				nodeIDs = append(nodeIDs, remoteID)
				edgeInfo[remoteID] = edge
			}

			sort.Strings(nodeIDs)

			// Collect all network prefixes
			allPrefixes := g.getAllPrefixes(nodeIDs, edgeInfo)
			primaryPrefix := ""
			if len(allPrefixes) > 0 {
				primaryPrefix = allPrefixes[0]
			}

			segments = append(segments, NetworkSegment{
				ID:              fmt.Sprintf("segment_%d", segmentID),
				Interface:       localIface,
				NetworkPrefix:   primaryPrefix,
				NetworkPrefixes: allPrefixes,
				ConnectedNodes:  nodeIDs,
				EdgeInfo:        edgeInfo,
			})
			segmentID++
		}
	}

	// Part 2: Detect REMOTE segments (VLANs where local node is not a member)
	// Look for groups of nodes connected on same interface name via indirect edges
	// Skip interfaces that local node already has segments on
	localInterfaces := make(map[string]bool)
	for _, seg := range segments {
		localInterfaces[seg.Interface] = true
	}

	remoteInterfaceGroups := make(map[string]map[string]*Edge) // [interface_name][node_id] = edge

	for srcID, dests := range g.edges {
		if srcID == localID {
			continue // Skip local node (already handled above)
		}

		for dstID, edgeList := range dests {
			if dstID == localID {
				continue // Skip edges to local node
			}

			for _, edge := range edgeList {
				// Only consider edges where both sides use the SAME interface name
				// This indicates they're on the same VLAN
				if edge.LocalInterface == edge.RemoteInterface {
					ifaceName := edge.LocalInterface

					// Skip if local node already has a segment on this interface
					if localInterfaces[ifaceName] {
						continue
					}

					if remoteInterfaceGroups[ifaceName] == nil {
						remoteInterfaceGroups[ifaceName] = make(map[string]*Edge)
					}

					// Add both source and destination to this interface group
					if _, exists := remoteInterfaceGroups[ifaceName][srcID]; !exists {
						remoteInterfaceGroups[ifaceName][srcID] = edge
					}
					if _, exists := remoteInterfaceGroups[ifaceName][dstID]; !exists {
						remoteInterfaceGroups[ifaceName][dstID] = edge
					}
				}
			}
		}
	}

	// Create segments for remote VLANs with 3+ nodes
	// BUT: verify nodes are actually connected (not just using same interface name)
	for ifaceName, nodeEdges := range remoteInterfaceGroups {
		if len(nodeEdges) < 3 {
			continue // Need at least 3 nodes for a segment
		}

		// Build connectivity graph for this interface
		// Only include edges on this specific interface
		connectivity := make(map[string]map[string]bool)
		for srcID, dests := range g.edges {
			for dstID, edgeList := range dests {
				for _, edge := range edgeList {
					if edge.LocalInterface == ifaceName && edge.RemoteInterface == ifaceName {
						// Both sides use this interface
						if connectivity[srcID] == nil {
							connectivity[srcID] = make(map[string]bool)
						}
						connectivity[srcID][dstID] = true
					}
				}
			}
		}

		// Find connected components within this interface group using BFS
		visited := make(map[string]bool)
		for startNode := range nodeEdges {
			if visited[startNode] {
				continue
			}

			// BFS to find all nodes in this component
			component := []string{}
			queue := []string{startNode}
			visited[startNode] = true

			for len(queue) > 0 {
				current := queue[0]
				queue = queue[1:]
				component = append(component, current)

				// Add connected neighbors (only those in nodeEdges)
				if neighbors, ok := connectivity[current]; ok {
					for neighbor := range neighbors {
						if _, inGroup := nodeEdges[neighbor]; inGroup && !visited[neighbor] {
							visited[neighbor] = true
							queue = append(queue, neighbor)
						}
					}
				}
			}

			// Create segment if this component has 3+ nodes
			if len(component) < 3 {
				continue
			}

			// Collect edge info for nodes in this component
			componentEdgeInfo := make(map[string]*Edge)
			for _, nodeID := range component {
				if edge, ok := nodeEdges[nodeID]; ok {
					componentEdgeInfo[nodeID] = edge
				}
			}

			sort.Strings(component)

			// Collect all network prefixes for this component
			allPrefixes := g.getAllPrefixes(component, componentEdgeInfo)
			primaryPrefix := ""
			if len(allPrefixes) > 0 {
				primaryPrefix = allPrefixes[0]
			}

			segments = append(segments, NetworkSegment{
				ID:              fmt.Sprintf("segment_%d", segmentID),
				Interface:       ifaceName,
				NetworkPrefix:   primaryPrefix,
				NetworkPrefixes: allPrefixes,
				ConnectedNodes:  component,
				EdgeInfo:        componentEdgeInfo,
			})
			segmentID++
		}
	}

	// First merge segments with same network prefix (existing logic)
	// This handles cases where different interfaces (em1, br112, etc.) are on the same subnet
	segments = mergeSegmentsByPrefix(segments)

	// Then merge segments with same node set but different prefixes
	// This handles cases where nodes have multiple interfaces (wired + WiFi) on same network
	segments = mergeSegmentsByNodeSet(segments, g)

	return segments
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

// getAllPrefixes collects all unique network prefixes from the given nodes
// Returns a sorted list of all IPv4 and IPv6 prefixes found
func (g *Graph) getAllPrefixes(nodeIDs []string, edgeInfo map[string]*Edge) []string {
	prefixSet := make(map[string]bool)

	// Collect prefixes from each node's interface
	for _, nodeID := range nodeIDs {
		var prefixes []string

		if nodeID == g.localNode.MachineID {
			// For local node, get prefixes from the edge's local interface
			for _, edge := range edgeInfo {
				if localDetails, ok := g.localNode.Interfaces[edge.LocalInterface]; ok {
					prefixes = localDetails.GlobalPrefixes
					break // All edges use same local interface
				}
			}
		} else {
			// For remote nodes, get prefixes from the edge's remote interface
			if edge, ok := edgeInfo[nodeID]; ok {
				if remoteNode, exists := g.nodes[nodeID]; exists {
					if remoteDetails, ok := remoteNode.Interfaces[edge.RemoteInterface]; ok {
						prefixes = remoteDetails.GlobalPrefixes
					}
				}
			}
		}

		// Add each prefix to the set
		for _, prefix := range prefixes {
			if prefix != "" {
				prefixSet[prefix] = true
			}
		}
	}

	// Convert to sorted list
	prefixList := make([]string, 0, len(prefixSet))
	for prefix := range prefixSet {
		prefixList = append(prefixList, prefix)
	}
	sort.Strings(prefixList)

	return prefixList
}

// getMostCommonPrefix returns the most frequently occurring network prefix
// from the given set of nodes and their edges. Returns empty string if no prefixes found.
func (g *Graph) getMostCommonPrefix(nodeIDs []string, edgeInfo map[string]*Edge) string {
	prefixCount := make(map[string]int)

	// Collect prefixes from each node's interface
	for _, nodeID := range nodeIDs {
		var prefixes []string

		if nodeID == g.localNode.MachineID {
			// For local node, get prefixes from the edge's local interface
			for _, edge := range edgeInfo {
				if localDetails, ok := g.localNode.Interfaces[edge.LocalInterface]; ok {
					prefixes = localDetails.GlobalPrefixes
					break // All edges use same local interface
				}
			}
		} else {
			// For remote nodes, get prefixes from the edge's remote interface
			if edge, ok := edgeInfo[nodeID]; ok {
				if remoteNode, exists := g.nodes[nodeID]; exists {
					if remoteDetails, ok := remoteNode.Interfaces[edge.RemoteInterface]; ok {
						prefixes = remoteDetails.GlobalPrefixes
					}
				}
			}
		}

		// Count each prefix
		for _, prefix := range prefixes {
			if prefix != "" {
				prefixCount[prefix]++
			}
		}
	}

	// Find most common prefix
	maxCount := 0
	mostCommon := ""
	for prefix, count := range prefixCount {
		if count > maxCount {
			maxCount = count
			mostCommon = prefix
		}
	}

	return mostCommon
}

// mergeSegmentsByPrefix merges segments that share the same network prefix
// This handles cases where different interfaces (e.g., em1, br112) are on the same subnet
func mergeSegmentsByPrefix(segments []NetworkSegment) []NetworkSegment {
	if len(segments) == 0 {
		return segments
	}

	// Group segments by network prefix
	prefixGroups := make(map[string][]int) // prefix -> list of segment indices

	for i, seg := range segments {
		// Only merge segments with non-empty prefixes
		if seg.NetworkPrefix != "" {
			prefixGroups[seg.NetworkPrefix] = append(prefixGroups[seg.NetworkPrefix], i)
		}
	}

	// Track which segments have been merged
	merged := make(map[int]bool)
	var result []NetworkSegment
	nextID := 0

	// Process each prefix group
	for prefix, indices := range prefixGroups {
		if len(indices) == 1 {
			// Only one segment with this prefix, keep as-is
			continue
		}

		// Multiple segments share this prefix - merge them
		mergedNodes := make(map[string]bool)
		mergedEdgeInfo := make(map[string]*Edge)
		interfaceNames := make(map[string]bool)

		for _, idx := range indices {
			seg := segments[idx]
			merged[idx] = true

			// Collect all nodes
			for _, nodeID := range seg.ConnectedNodes {
				mergedNodes[nodeID] = true
			}

			// Collect all edges (prefer edges with more information)
			for nodeID, edge := range seg.EdgeInfo {
				if existing, exists := mergedEdgeInfo[nodeID]; exists {
					// Node already has an edge, keep the better one
					// Prefer: 1) edges with local interface, 2) edges with prefixes
					keepNew := false

					if edge.LocalInterface != "" && existing.LocalInterface == "" {
						keepNew = true // New edge has local interface
					} else if len(edge.LocalPrefixes) > 0 && len(existing.LocalPrefixes) == 0 {
						keepNew = true // New edge has local prefixes
					} else if len(edge.RemotePrefixes) > 0 && len(existing.RemotePrefixes) == 0 {
						keepNew = true // New edge has remote prefixes
					}

					if keepNew {
						mergedEdgeInfo[nodeID] = edge
					}
				} else {
					mergedEdgeInfo[nodeID] = edge
				}
			}

			// Collect interface names
			interfaceNames[seg.Interface] = true
		}

		// Convert merged nodes to sorted list
		var nodeList []string
		for nodeID := range mergedNodes {
			nodeList = append(nodeList, nodeID)
		}
		sort.Strings(nodeList)

		// Create merged segment
		// Use first interface name, but could be comma-separated list
		interfaceList := make([]string, 0, len(interfaceNames))
		for ifName := range interfaceNames {
			interfaceList = append(interfaceList, ifName)
		}
		sort.Strings(interfaceList)
		primaryInterface := interfaceList[0]

		// Collect all unique prefixes from merged segments
		allPrefixes := make(map[string]bool)
		for _, idx := range indices {
			for _, p := range segments[idx].NetworkPrefixes {
				if p != "" {
					allPrefixes[p] = true
				}
			}
		}
		prefixList := make([]string, 0, len(allPrefixes))
		for p := range allPrefixes {
			prefixList = append(prefixList, p)
		}
		sort.Strings(prefixList)

		result = append(result, NetworkSegment{
			ID:              fmt.Sprintf("segment_%d", nextID),
			Interface:       primaryInterface,
			NetworkPrefix:   prefix,
			NetworkPrefixes: prefixList,
			ConnectedNodes:  nodeList,
			EdgeInfo:        mergedEdgeInfo,
		})
		nextID++
	}

	// Add segments that weren't merged (no prefix or unique prefix)
	for i, seg := range segments {
		if !merged[i] {
			result = append(result, NetworkSegment{
				ID:              fmt.Sprintf("segment_%d", nextID),
				Interface:       seg.Interface,
				NetworkPrefix:   seg.NetworkPrefix,
				NetworkPrefixes: seg.NetworkPrefixes,
				ConnectedNodes:  seg.ConnectedNodes,
				EdgeInfo:        seg.EdgeInfo,
			})
			nextID++
		}
	}

	return result
}

// mergeSegmentsByNodeSet merges segments that have the same set of connected nodes
// This handles cases where nodes have multiple interfaces (wired + WiFi) on the same physical network
// Each interface may have different prefixes, but they're all on the same segment
func mergeSegmentsByNodeSet(segments []NetworkSegment, g *Graph) []NetworkSegment {
	if len(segments) == 0 {
		return segments
	}

	// Create a canonical key for a node set (sorted, comma-separated)
	makeNodeSetKey := func(nodes []string) string {
		sorted := make([]string, len(nodes))
		copy(sorted, nodes)
		sort.Strings(sorted)
		return strings.Join(sorted, ",")
	}

	// Group segments by their node set
	nodeSetGroups := make(map[string][]int) // nodeSetKey -> list of segment indices

	for i, seg := range segments {
		key := makeNodeSetKey(seg.ConnectedNodes)
		nodeSetGroups[key] = append(nodeSetGroups[key], i)
	}

	// Track which segments have been merged
	merged := make(map[int]bool)
	var result []NetworkSegment
	nextID := 0

	// Process each node set group
	for _, indices := range nodeSetGroups {
		if len(indices) == 1 {
			// Only one segment with this node set, keep as-is
			continue
		}

		// Multiple segments share this node set - merge them
		mergedPrefixes := make(map[string]bool)
		mergedEdgeInfo := make(map[string]*Edge)
		var primaryInterface string
		var maxSpeed int

		for _, idx := range indices {
			seg := segments[idx]
			merged[idx] = true

			// Collect all prefixes
			for _, prefix := range seg.NetworkPrefixes {
				if prefix != "" {
					mergedPrefixes[prefix] = true
				}
			}

			// Collect all edges (prefer edges with more information)
			for nodeID, edge := range seg.EdgeInfo {
				if existing, exists := mergedEdgeInfo[nodeID]; exists {
					// Node already has an edge, keep the better one
					keepNew := false

					if edge.LocalInterface != "" && existing.LocalInterface == "" {
						keepNew = true
					} else if len(edge.LocalPrefixes) > len(existing.LocalPrefixes) {
						keepNew = true
					} else if len(edge.RemotePrefixes) > len(existing.RemotePrefixes) {
						keepNew = true
					}

					if keepNew {
						mergedEdgeInfo[nodeID] = edge
					}
				} else {
					mergedEdgeInfo[nodeID] = edge
				}
			}

			// Select primary interface (highest effective speed, prefer wired over WiFi)
			ifaceSpeed := getEffectiveSpeed(0, seg.Interface) // Get speed from edges
			// Find the highest speed for this interface from EdgeInfo
			for _, edge := range seg.EdgeInfo {
				localSpeed := getEffectiveSpeed(edge.LocalSpeed, edge.LocalInterface)
				remoteSpeed := getEffectiveSpeed(edge.RemoteSpeed, edge.RemoteInterface)
				if edge.LocalInterface == seg.Interface && localSpeed > ifaceSpeed {
					ifaceSpeed = localSpeed
				}
				if edge.RemoteInterface == seg.Interface && remoteSpeed > ifaceSpeed {
					ifaceSpeed = remoteSpeed
				}
			}

			// Prefer this interface if:
			// 1. It has higher speed, OR
			// 2. Same speed but this one is wired (not wl*)
			if primaryInterface == "" {
				primaryInterface = seg.Interface
				maxSpeed = ifaceSpeed
			} else {
				takeNew := false
				if ifaceSpeed > maxSpeed {
					takeNew = true
				} else if ifaceSpeed == maxSpeed {
					// Same speed, prefer wired over WiFi
					currentIsWifi := strings.Contains(primaryInterface, "wl")
					newIsWifi := strings.Contains(seg.Interface, "wl")
					if currentIsWifi && !newIsWifi {
						takeNew = true
					}
				}
				if takeNew {
					primaryInterface = seg.Interface
					maxSpeed = ifaceSpeed
				}
			}
		}

		// Convert merged prefixes to sorted list
		prefixList := make([]string, 0, len(mergedPrefixes))
		for prefix := range mergedPrefixes {
			prefixList = append(prefixList, prefix)
		}
		sort.Strings(prefixList)

		// Get node list (already sorted since all segments have same nodes)
		nodeList := segments[indices[0]].ConnectedNodes

		// Primary prefix for backward compatibility
		primaryPrefix := ""
		if len(prefixList) > 0 {
			primaryPrefix = prefixList[0]
		}

		result = append(result, NetworkSegment{
			ID:              fmt.Sprintf("segment_%d", nextID),
			Interface:       primaryInterface,
			NetworkPrefix:   primaryPrefix,
			NetworkPrefixes: prefixList,
			ConnectedNodes:  nodeList,
			EdgeInfo:        mergedEdgeInfo,
		})
		nextID++
	}

	// Add segments that weren't merged (unique node sets)
	for i, seg := range segments {
		if !merged[i] {
			result = append(result, NetworkSegment{
				ID:              fmt.Sprintf("segment_%d", nextID),
				Interface:       seg.Interface,
				NetworkPrefix:   seg.NetworkPrefix,
				NetworkPrefixes: seg.NetworkPrefixes,
				ConnectedNodes:  seg.ConnectedNodes,
				EdgeInfo:        seg.EdgeInfo,
			})
			nextID++
		}
	}

	return result
}

func (g *Graph) isCompleteIsland(ownerID string, neighborIDs []string) bool {
	// Check if each neighbor sees all other neighbors
	for _, neighborID := range neighborIDs {
		neighborEdges, exists := g.edges[neighborID]
		if !exists {
			return false
		}

		// Check if this neighbor can reach all other neighbors
		for _, otherNeighborID := range neighborIDs {
			if neighborID == otherNeighborID {
				continue
			}

			// Check if edge exists from neighbor to otherNeighbor
			if _, hasEdge := neighborEdges[otherNeighborID]; !hasEdge {
				return false
			}
		}
	}

	return true
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(strings []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strings {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func (g *Graph) HasChanges() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.changed
}

func (g *Graph) ClearChanges() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.changed = false
}

func (g *Graph) GetLocalMachineID() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.localNode != nil {
		return g.localNode.MachineID
	}
	return ""
}

// removeDuplicates returns a new slice with duplicate strings removed
