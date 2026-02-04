package graph

import (
	"sync"
	"time"
)

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
	LocalInterface  string
	RemoteInterface string
}

type Graph struct {
	mu         sync.RWMutex
	nodes      map[string]*Node
	localNode  *Node
	edges      map[string]map[string]*Edge // [localMachineID][remoteMachineID] -> Edge
	changed    bool
}

func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
		edges: make(map[string]map[string]*Edge),
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

func (g *Graph) AddOrUpdate(machineID, hostname, remoteIface, sourceIP, receivingIface, rdmaDevice, nodeGUID, sysImageGUID string) {
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
	if g.localNode != nil && receivingIface != "" {
		if _, ok := g.edges[g.localNode.MachineID]; !ok {
			g.edges[g.localNode.MachineID] = make(map[string]*Edge)
		}
		
		edge := &Edge{
			LocalInterface:  receivingIface,
			RemoteInterface: remoteIface,
		}
		
		if existing, ok := g.edges[g.localNode.MachineID][machineID]; !ok || *existing != *edge {
			g.edges[g.localNode.MachineID][machineID] = edge
			g.changed = true
		}
	}
}

func (g *Graph) RemoveExpired(timeout time.Duration) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	removed := 0

	for machineID, node := range g.nodes {
		if now.Sub(node.LastSeen) > timeout {
			delete(g.nodes, machineID)
			removed++
			g.changed = true
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

func (g *Graph) GetEdges() map[string]map[string]*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]map[string]*Edge)
	for src, dests := range g.edges {
		result[src] = make(map[string]*Edge)
		for dst, edge := range dests {
			edgeCopy := &Edge{
				LocalInterface:  edge.LocalInterface,
				RemoteInterface: edge.RemoteInterface,
			}
			result[src][dst] = edgeCopy
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
