package graph

import (
	"sync"
	"time"
)

type NeighborData struct {
	MachineID    string
	Hostname     string
	Interface    string
	Address      string
	RDMADevice   string
	NodeGUID     string
	SysImageGUID string
}

type InterfaceDetails struct {
	IPAddress    string
	RDMADevice   string
	NodeGUID     string
	SysImageGUID string
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
	RemoteInterface    string
	RemoteAddress      string
	RemoteRDMADevice   string
	RemoteNodeGUID     string
	RemoteSysImageGUID string
	Direct             bool
	LearnedFrom        string
}

type Graph struct {
	mu         sync.RWMutex
	nodes      map[string]*Node
	localNode  *Node
	edges      map[string]map[string][]*Edge // [localMachineID][remoteMachineID] -> []Edge (multiple edges)
	changed    bool
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

func (g *Graph) AddOrUpdate(machineID, hostname, remoteIface, sourceIP, receivingIface, rdmaDevice, nodeGUID, sysImageGUID string, direct bool, learnedFrom string) {
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
			RemoteInterface:    remoteIface,
			RemoteAddress:      sourceIP,
			RemoteRDMADevice:   rdmaDevice,
			RemoteNodeGUID:     nodeGUID,
			RemoteSysImageGUID: sysImageGUID,
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
					RemoteInterface:    edge.RemoteInterface,
					RemoteAddress:      edge.RemoteAddress,
					RemoteRDMADevice:   edge.RemoteRDMADevice,
					RemoteNodeGUID:     edge.RemoteNodeGUID,
					RemoteSysImageGUID: edge.RemoteSysImageGUID,
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
						MachineID:    dstID,
						Hostname:     node.Hostname,
						Interface:    edge.RemoteInterface,
						Address:      edge.RemoteAddress,
						RDMADevice:   edge.RemoteRDMADevice,
						NodeGUID:     edge.RemoteNodeGUID,
						SysImageGUID: edge.RemoteSysImageGUID,
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
