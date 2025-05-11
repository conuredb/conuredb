package btree

import (
	"sync"
)

// NodePool manages a pool of reusable nodes
type NodePool struct {
	mu          sync.Mutex
	freeNodeIDs []NodeID
	nextNodeID  NodeID
}

// NewNodePool creates a new node pool
func NewNodePool() *NodePool {
	return &NodePool{
		freeNodeIDs: make([]NodeID, 0),
		nextNodeID:  1, // Start from 1, 0 is reserved for null/nil
	}
}

// Allocate allocates a new node ID
func (p *NodePool) Allocate() NodeID {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Reuse a free node ID if available
	if len(p.freeNodeIDs) > 0 {
		nodeID := p.freeNodeIDs[len(p.freeNodeIDs)-1]
		p.freeNodeIDs = p.freeNodeIDs[:len(p.freeNodeIDs)-1]
		return nodeID
	}

	// Allocate a new node ID
	nodeID := p.nextNodeID
	p.nextNodeID++
	return nodeID
}

// Free returns a node ID to the pool for reuse
func (p *NodePool) Free(nodeID NodeID) {
	if nodeID == 0 {
		return // Don't free the null/nil node
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.freeNodeIDs = append(p.freeNodeIDs, nodeID)
}

// Reset resets the node pool
func (p *NodePool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.freeNodeIDs = make([]NodeID, 0)
	p.nextNodeID = 1
}

// Stats returns statistics about the node pool
func (p *NodePool) Stats() (nextNodeID NodeID, freeNodeCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.nextNodeID, len(p.freeNodeIDs)
}
