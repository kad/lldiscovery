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
	LocalRDMADevice    string
	LocalNodeGUID      string
	LocalSysImageGUID  string
	LocalSpeed         int
	RemoteInterface    string
	RemoteAddress      string
	RemoteRDMADevice   string
	RemoteNodeGUID     string
	RemoteSysImageGUID string
	RemoteSpeed        int
}

type InterfaceDetails struct {
	IPAddress    string
	RDMADevice   string
	NodeGUID     string
	SysImageGUID string
	Speed        int // Link speed in Mbps
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
	LocalRDMADevice    string
	LocalNodeGUID      string
	LocalSysImageGUID  string
	LocalSpeed         int // Link speed in Mbps
	RemoteInterface    string
	RemoteAddress      string
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

func (g *Graph) AddOrUpdate(machineID, hostname, remoteIface, sourceIP, receivingIface, rdmaDevice, nodeGUID, sysImageGUID string, remoteSpeed int, direct bool, learnedFrom string) {
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
		IPAddress:    sourceIP,
		RDMADevice:   rdmaDevice,
		NodeGUID:     nodeGUID,
		SysImageGUID: sysImageGUID,
		Speed:        remoteSpeed,
	}

	if existing, ok := node.Interfaces[remoteIface]; !ok || existing != details {
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
			LocalRDMADevice:    localDetails.RDMADevice,
			LocalNodeGUID:      localDetails.NodeGUID,
			LocalSysImageGUID:  localDetails.SysImageGUID,
			LocalSpeed:         localDetails.Speed,
			RemoteInterface:    remoteIface,
			RemoteAddress:      sourceIP,
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
	intermediateIface, intermediateAddress,
	intermediateRDMA, intermediateNodeGUID, intermediateSysImageGUID string,
	intermediateSpeed int,
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
		IPAddress:    neighborAddress,
		RDMADevice:   neighborRDMA,
		NodeGUID:     neighborNodeGUID,
		SysImageGUID: neighborSysImageGUID,
		Speed:        neighborSpeed,
	}
	if existing, ok := node.Interfaces[neighborIface]; !ok || existing != neighborDetails {
		node.Interfaces[neighborIface] = neighborDetails
		g.changed = true
	}

	// Also ensure the intermediate node exists and update its interface
	intermediateNode, intermediateExists := g.nodes[learnedFrom]
	if intermediateExists && intermediateIface != "" {
		intermediateDetails := InterfaceDetails{
			IPAddress:    intermediateAddress,
			RDMADevice:   intermediateRDMA,
			NodeGUID:     intermediateNodeGUID,
			SysImageGUID: intermediateSysImageGUID,
			Speed:        intermediateSpeed,
		}
		if existing, ok := intermediateNode.Interfaces[intermediateIface]; !ok || existing != intermediateDetails {
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
			LocalRDMADevice:    intermediateRDMA,
			LocalNodeGUID:      intermediateNodeGUID,
			LocalSysImageGUID:  intermediateSysImageGUID,
			LocalSpeed:         intermediateSpeed,
			RemoteInterface:    neighborIface,
			RemoteAddress:      neighborAddress,
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
					LocalRDMADevice:    edge.LocalRDMADevice,
					LocalNodeGUID:      edge.LocalNodeGUID,
					LocalSysImageGUID:  edge.LocalSysImageGUID,
					LocalSpeed:         edge.LocalSpeed,
					RemoteInterface:    edge.RemoteInterface,
					RemoteAddress:      edge.RemoteAddress,
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

					result = append(result, NeighborData{
						MachineID:          dstID,
						Hostname:           node.Hostname,
						LocalInterface:     edge.LocalInterface,
						LocalAddress:       edge.LocalAddress,
						LocalRDMADevice:    edge.LocalRDMADevice,
						LocalNodeGUID:      edge.LocalNodeGUID,
						LocalSysImageGUID:  edge.LocalSysImageGUID,
						LocalSpeed:         edge.LocalSpeed,
						RemoteInterface:    edge.RemoteInterface,
						RemoteAddress:      edge.RemoteAddress,
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
	ID             string           // Unique ID for this segment
	Interface      string           // Local interface name (e.g., "eth0")
	ConnectedNodes []string         // Machine IDs of nodes in this segment
	EdgeInfo       map[string]*Edge // Map of nodeID -> edge info for connections to segment
}

// GetNetworkSegments finds groups of nodes connected to shared network segments
// A segment is detected when the local node can reach 3+ other nodes on the same interface
func (g *Graph) GetNetworkSegments() []NetworkSegment {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var segments []NetworkSegment

	// Only meaningful if we have a local node
	if g.localNode == nil {
		return segments
	}

	localID := g.localNode.MachineID

	// Build a connectivity graph: which node:interface pairs can reach which other node:interface pairs
	connectivity := make(map[string]map[string]bool) // [nodeID:iface][otherNodeID:otherIface] -> true

	// Scan all edges to build bidirectional connectivity map
	for srcID, dests := range g.edges {
		for dstID, edgeList := range dests {
			for _, edge := range edgeList {
				srcKey := fmt.Sprintf("%s:%s", srcID, edge.LocalInterface)
				dstKey := fmt.Sprintf("%s:%s", dstID, edge.RemoteInterface)

				if connectivity[srcKey] == nil {
					connectivity[srcKey] = make(map[string]bool)
				}
				connectivity[srcKey][dstKey] = true

				// Add reverse connectivity (bidirectional)
				if connectivity[dstKey] == nil {
					connectivity[dstKey] = make(map[string]bool)
				}
				connectivity[dstKey][srcKey] = true
			}
		}
	}

	// Find all maximal cliques using Bron-Kerbosch algorithm
	allNodeInterfaces := make([]string, 0, len(connectivity))
	for ni := range connectivity {
		allNodeInterfaces = append(allNodeInterfaces, ni)
	}

	var cliques [][]string
	bronKerbosch(
		[]string{},              // R: current clique being built
		allNodeInterfaces,       // P: candidates to add
		[]string{},              // X: already processed
		connectivity,
		&cliques,
	)

	// Convert cliques to segments
	segmentID := 0
	for _, clique := range cliques {
		// Extract unique node IDs from clique
		nodeSet := make(map[string]bool)
		interfaceSet := make(map[string]bool)
		nodeInterfaceMap := make(map[string]string) // nodeID -> interface used in this clique

		for _, nodeIface := range clique {
			parts := strings.SplitN(nodeIface, ":", 2)
			if len(parts) == 2 {
				nodeSet[parts[0]] = true
				interfaceSet[parts[1]] = true
				nodeInterfaceMap[parts[0]] = parts[1]
			}
		}

		// Need at least 3 unique nodes for a segment
		if len(nodeSet) < 3 {
			continue
		}

		// Create sorted list of node IDs
		nodeIDs := make([]string, 0, len(nodeSet))
		for nodeID := range nodeSet {
			nodeIDs = append(nodeIDs, nodeID)
		}
		sort.Strings(nodeIDs)

		// Determine interface name for the segment
		var segmentInterface string
		if len(interfaceSet) == 1 {
			for iface := range interfaceSet {
				segmentInterface = iface
			}
		} else {
			// Multiple interface names - create a representative name
			interfaces := make([]string, 0, len(interfaceSet))
			for iface := range interfaceSet {
				interfaces = append(interfaces, iface)
			}
			sort.Strings(interfaces)
			if len(interfaces) <= 3 {
				segmentInterface = strings.Join(interfaces, "+")
			} else {
				segmentInterface = fmt.Sprintf("mixed(%d)", len(interfaces))
			}
		}

		// Collect edge info from local node to segment members
		edgeInfo := make(map[string]*Edge)
		if localEdges, ok := g.edges[localID]; ok {
			for _, nodeID := range nodeIDs {
				if nodeID == localID {
					continue
				}
				if edges, ok := localEdges[nodeID]; ok {
					// Use the first edge (prefer direct over indirect)
					for _, edge := range edges {
						if existing, ok := edgeInfo[nodeID]; !ok || (!existing.Direct && edge.Direct) {
							edgeInfo[nodeID] = edge
						}
					}
				}
			}
		}

		segments = append(segments, NetworkSegment{
			ID:             fmt.Sprintf("segment_%d", segmentID),
			Interface:      segmentInterface,
			ConnectedNodes: nodeIDs,
			EdgeInfo:       edgeInfo,
		})
		segmentID++
	}

	return segments
}

// bronKerbosch implements the Bron-Kerbosch algorithm to find all maximal cliques
// with pivot optimization for better performance
func bronKerbosch(R, P, X []string, connectivity map[string]map[string]bool, cliques *[][]string) {
	if len(P) == 0 && len(X) == 0 {
		// Found a maximal clique
		if len(R) >= 3 { // Only keep cliques with 3+ node:interface pairs
			// Extract unique nodes to verify we have 3+ unique machines
			uniqueNodes := make(map[string]bool)
			for _, nodeIface := range R {
				parts := strings.SplitN(nodeIface, ":", 2)
				if len(parts) == 2 {
					uniqueNodes[parts[0]] = true
				}
			}
			if len(uniqueNodes) >= 3 {
				clique := make([]string, len(R))
				copy(clique, R)
				*cliques = append(*cliques, clique)
			}
		}
		return
	}

	// Create a copy of P since we'll be modifying it
	PCopy := make([]string, len(P))
	copy(PCopy, P)

	// Choose pivot (vertex with most connections in P ∪ X) for optimization
	pivot := choosePivot(PCopy, X, connectivity)
	pivotNeighbors := make(map[string]bool)
	if pivot != "" && connectivity[pivot] != nil {
		for n := range connectivity[pivot] {
			pivotNeighbors[n] = true
		}
	}

	// Iterate over P \ N(pivot)
	for _, v := range PCopy {
		// Skip neighbors of pivot (optimization)
		if pivotNeighbors[v] {
			continue
		}

		// Build new sets for recursive call
		newR := append([]string{}, R...)
		newR = append(newR, v)

		// newP = P ∩ N(v)
		newP := intersect(P, getNeighborsList(v, connectivity))

		// newX = X ∩ N(v)
		newX := intersect(X, getNeighborsList(v, connectivity))

		bronKerbosch(newR, newP, newX, connectivity, cliques)

		// Move v from P to X
		P = remove(P, v)
		X = append(X, v)
	}
}

// choosePivot selects a vertex with the most connections in P ∪ X
func choosePivot(P, X []string, connectivity map[string]map[string]bool) string {
	var pivot string
	maxConnections := -1

	candidates := append([]string{}, P...)
	candidates = append(candidates, X...)

	for _, v := range candidates {
		if connectivity[v] == nil {
			continue
		}
		connections := 0
		for _, u := range P {
			if connectivity[v][u] {
				connections++
			}
		}
		if connections > maxConnections {
			maxConnections = connections
			pivot = v
		}
	}

	return pivot
}

// getNeighborsList returns all vertices connected to v
func getNeighborsList(v string, connectivity map[string]map[string]bool) []string {
	var result []string
	if neighMap, ok := connectivity[v]; ok {
		for neighbor := range neighMap {
			result = append(result, neighbor)
		}
	}
	return result
}

// intersect returns the intersection of two slices
func intersect(a, b []string) []string {
	set := make(map[string]bool)
	for _, v := range b {
		set[v] = true
	}

	var result []string
	for _, v := range a {
		if set[v] {
			result = append(result, v)
		}
	}
	return result
}

// remove removes an element from a slice
func remove(slice []string, elem string) []string {
	var result []string
	for _, v := range slice {
		if v != elem {
			result = append(result, v)
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
