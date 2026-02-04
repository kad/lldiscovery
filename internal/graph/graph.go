package graph

import (
	"sync"
	"time"
)

type Node struct {
	Hostname   string
	MachineID  string
	LastSeen   time.Time
	Interfaces map[string]string
}

type Graph struct {
	mu      sync.RWMutex
	nodes   map[string]*Node
	changed bool
}

func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
	}
}

func (g *Graph) AddOrUpdate(machineID, hostname, iface, sourceIP string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[machineID]
	if !exists {
		node = &Node{
			Hostname:   hostname,
			MachineID:  machineID,
			Interfaces: make(map[string]string),
		}
		g.nodes[machineID] = node
		g.changed = true
	}

	if node.Hostname != hostname {
		node.Hostname = hostname
		g.changed = true
	}

	node.LastSeen = time.Now()

	if node.Interfaces[iface] != sourceIP {
		node.Interfaces[iface] = sourceIP
		g.changed = true
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

	result := make(map[string]*Node, len(g.nodes))
	for k, v := range g.nodes {
		nodeCopy := &Node{
			Hostname:   v.Hostname,
			MachineID:  v.MachineID,
			LastSeen:   v.LastSeen,
			Interfaces: make(map[string]string),
		}
		for ik, iv := range v.Interfaces {
			nodeCopy.Interfaces[ik] = iv
		}
		result[k] = nodeCopy
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
