package graph

import (
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
	RemoteSpeed        int  // Link speed in Mbps
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
