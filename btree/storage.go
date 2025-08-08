package btree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	// Magic number for file format identification
	MagicNumber uint32 = 0x434F4E55 // "CONU" in ASCII

	// Version of the file format
	Version uint32 = 1

	// HeaderSize defines the size of the file header region in bytes.
	// We reserve a full page to simplify offset math and avoid variable-length headers.
	HeaderSize = NodeSize
)

var (
	ErrInvalidMagicNumber = errors.New("invalid magic number")
	ErrInvalidVersion     = errors.New("invalid version")
	ErrNodeNotFound       = errors.New("node not found")
)

// Storage manages the on-disk storage of nodes
type Storage struct {
	mu           sync.RWMutex
	file         *os.File
	nodeCache    map[NodeID]*Node
	rootNodeID   NodeID
	nodePool     *NodePool
	dirtyNodes   map[NodeID]struct{}
	transaction  bool
	originalRoot NodeID
}

// OpenStorage opens a storage file
func OpenStorage(path string) (*Storage, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	storage := &Storage{
		file:       file,
		nodeCache:  make(map[NodeID]*Node),
		nodePool:   NewNodePool(),
		dirtyNodes: make(map[NodeID]struct{}),
	}

	// Check if the file is empty
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if info.Size() == 0 {
		// Initialize a new file
		if err := storage.initializeNewFile(); err != nil {
			file.Close()
			return nil, err
		}
	} else {
		// Read the header
		if err := storage.readHeader(); err != nil {
			file.Close()
			return nil, err
		}
	}

	return storage, nil
}

// Close closes the storage
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.transaction {
		s.abortTransaction()
	}

	return s.file.Close()
}

// initializeNewFile initializes a new file with header and root node
func (s *Storage) initializeNewFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write an empty header first so subsequent writes land after a full page
	if err := s.writeHeader(); err != nil {
		return err
	}

	// Create root node
	rootNodeID := s.nodePool.Allocate()
	rootNode := NewLeafNode(rootNodeID)
	s.rootNodeID = rootNodeID
	s.nodeCache[rootNodeID] = rootNode

	// Write root node
	if err := s.writeNode(rootNode); err != nil {
		return err
	}

	// Update header with root node ID
	return s.writeHeader()
}

// readHeader reads the file header
func (s *Storage) readHeader() error {
	// Read exactly one header page
	head := make([]byte, HeaderSize)
	n, err := s.file.ReadAt(head, 0)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	if n < 28 { // minimally need fixed fields
		return fmt.Errorf("header too small: %d bytes", n)
	}

	r := bytes.NewReader(head)

	// Read magic number
	var magic uint32
	if err := binary.Read(r, binary.LittleEndian, &magic); err != nil {
		return err
	}
	if magic != MagicNumber {
		return ErrInvalidMagicNumber
	}

	// Read version
	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return err
	}
	if version != Version {
		return ErrInvalidVersion
	}

	// Read root node ID
	if err := binary.Read(r, binary.LittleEndian, &s.rootNodeID); err != nil {
		return err
	}

	// Read next node ID
	var nextNodeID NodeID
	if err := binary.Read(r, binary.LittleEndian, &nextNodeID); err != nil {
		return err
	}
	s.nodePool = NewNodePool()
	s.nodePool.nextNodeID = nextNodeID

	// Read free node count (bounded by what can fit in the header)
	var freeNodeCount uint32
	if err := binary.Read(r, binary.LittleEndian, &freeNodeCount); err != nil {
		return err
	}

	// Compute how many NodeIDs fit after fixed fields
	const fixedFields = 4 + 4 + 8 + 8 + 4 // magic + version + root + next + count
	maxFree := uint32((HeaderSize - fixedFields) / 8)
	if freeNodeCount > maxFree {
		freeNodeCount = maxFree
	}

	// Read free node IDs
	s.nodePool.freeNodeIDs = make([]NodeID, freeNodeCount)
	for i := uint32(0); i < freeNodeCount; i++ {
		var nodeID NodeID
		if err := binary.Read(r, binary.LittleEndian, &nodeID); err != nil {
			return err
		}
		s.nodePool.freeNodeIDs[i] = nodeID
	}

	return nil
}

// writeHeader writes the file header
func (s *Storage) writeHeader() error {
	// Build a fixed-size header page
	buf := bytes.NewBuffer(make([]byte, 0, HeaderSize))

	// Write magic number
	if err := binary.Write(buf, binary.LittleEndian, MagicNumber); err != nil {
		return err
	}

	// Write version
	if err := binary.Write(buf, binary.LittleEndian, Version); err != nil {
		return err
	}

	// Write root node ID
	if err := binary.Write(buf, binary.LittleEndian, s.rootNodeID); err != nil {
		return err
	}

	// Write next node ID
	nextNodeID, _ := s.nodePool.Stats()
	if err := binary.Write(buf, binary.LittleEndian, nextNodeID); err != nil {
		return err
	}

	// Determine how many free node IDs we can persist in the header page
	const fixedFields = 4 + 4 + 8 + 8 + 4
	maxFree := (HeaderSize - fixedFields) / 8
	freeNodeCount := len(s.nodePool.freeNodeIDs)
	if freeNodeCount > maxFree {
		freeNodeCount = maxFree
	}

	// Write free node count
	if err := binary.Write(buf, binary.LittleEndian, uint32(freeNodeCount)); err != nil {
		return err
	}

	// Write free node IDs (bounded)
	for i := 0; i < freeNodeCount; i++ {
		if err := binary.Write(buf, binary.LittleEndian, s.nodePool.freeNodeIDs[i]); err != nil {
			return err
		}
	}

	// Pad the rest of the header page
	if buf.Len() > HeaderSize {
		return fmt.Errorf("header size %d exceeds reserved header page %d", buf.Len(), HeaderSize)
	}
	padding := make([]byte, HeaderSize-buf.Len())
	if _, err := buf.Write(padding); err != nil {
		return err
	}

	// Write header at the beginning of the file
	data := buf.Bytes()
	n, err := s.file.WriteAt(data, 0)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("short write for header: wrote %d of %d", n, len(data))
	}

	return nil
}

// GetNode gets a node from storage
func (s *Storage) GetNode(nodeID NodeID) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if the node is in cache
	if node, ok := s.nodeCache[nodeID]; ok {
		return node, nil
	}

	// Read the node from disk
	node, err := s.readNode(nodeID)
	if err != nil {
		return nil, err
	}

	// Add the node to cache
	s.nodeCache[nodeID] = node

	return node, nil
}

// readNode reads a node from disk
func (s *Storage) readNode(nodeID NodeID) (*Node, error) {
	// Calculate the offset (header occupies one full page)
	offset := int64(HeaderSize) + int64(nodeID-1)*int64(NodeSize)

	// Read the node data
	data := make([]byte, NodeSize)
	n, err := s.file.ReadAt(data, offset)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if n != NodeSize {
		return nil, fmt.Errorf("short read for node %d: read %d of %d", nodeID, n, NodeSize)
	}

	// Deserialize the node
	node, err := DeserializeNode(data)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// writeNode writes a node to disk
func (s *Storage) writeNode(node *Node) error {
	// Calculate the offset (header occupies one full page)
	offset := int64(HeaderSize) + int64(node.id-1)*int64(NodeSize)

	// Serialize the node
	data, err := node.Serialize()
	if err != nil {
		return err
	}

	// Write the node data
	n, err := s.file.WriteAt(data, offset)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("short write for node %d: wrote %d of %d", node.id, n, len(data))
	}

	return nil
}

// GetRootNode gets the root node
func (s *Storage) GetRootNode() (*Node, error) {
	return s.GetNode(s.rootNodeID)
}

// SetRootNode sets the root node
func (s *Storage) SetRootNode(node *Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rootNodeID = node.id
	s.nodeCache[node.id] = node
	s.dirtyNodes[node.id] = struct{}{}

	// During a transaction we defer header persistence until commit
	if s.transaction {
		return nil
	}

	return s.writeHeader()
}

// BeginTransaction begins a transaction
func (s *Storage) BeginTransaction() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.transaction {
		return errors.New("transaction already in progress")
	}

	s.transaction = true
	s.originalRoot = s.rootNodeID
	s.dirtyNodes = make(map[NodeID]struct{})

	return nil
}

// CommitTransaction commits a transaction
func (s *Storage) CommitTransaction() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.transaction {
		return errors.New("no transaction in progress")
	}

	// Write all dirty nodes
	for nodeID := range s.dirtyNodes {
		node, ok := s.nodeCache[nodeID]
		if !ok {
			return fmt.Errorf("dirty node %d not found in cache", nodeID)
		}

		if err := s.writeNode(node); err != nil {
			return err
		}
	}

	// Update header
	if err := s.writeHeader(); err != nil {
		return err
	}

	// Ensure durability by syncing to disk
	if err := s.file.Sync(); err != nil {
		return err
	}

	// Reset transaction state
	s.transaction = false
	s.dirtyNodes = make(map[NodeID]struct{})

	return nil
}

// abortTransaction aborts a transaction
func (s *Storage) abortTransaction() {
	if !s.transaction {
		return
	}

	// Restore original root
	s.rootNodeID = s.originalRoot

	// Reset transaction state
	s.transaction = false
	s.dirtyNodes = make(map[NodeID]struct{})
}

// PutNode puts a node in storage with copy-on-write
func (s *Storage) PutNode(node *Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.transaction {
		// Mark the node as dirty
		s.dirtyNodes[node.id] = struct{}{}
		// Update the cache
		s.nodeCache[node.id] = node
		return nil
	}

	// Write the node immediately if not in a transaction
	if err := s.writeNode(node); err != nil {
		return err
	}

	// Update the cache
	s.nodeCache[node.id] = node

	return nil
}

// CloneNode creates a copy of a node with a new ID (copy-on-write)
func (s *Storage) CloneNode(node *Node) (*Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Allocate a new node ID
	newNodeID := s.nodePool.Allocate()

	// Create a new node of the same type
	var newNode *Node
	if node.nodeType == LeafNode {
		newNode = NewLeafNode(newNodeID)
	} else {
		newNode = NewInternalNode(newNodeID)
	}

	// Copy properties
	newNode.count = node.count
	newNode.parent = node.parent
	newNode.items = make([]Item, len(node.items))
	copy(newNode.items, node.items)

	if node.nodeType == InternalNode {
		newNode.children = make([]NodeID, len(node.children))
		copy(newNode.children, node.children)
	}

	// Add to cache
	s.nodeCache[newNodeID] = newNode

	if s.transaction {
		// Mark the node as dirty
		s.dirtyNodes[newNodeID] = struct{}{}
	} else {
		// Write the node immediately if not in a transaction
		if err := s.writeNode(newNode); err != nil {
			return nil, err
		}
	}

	return newNode, nil
}

// DeleteNode marks a node for deletion
func (s *Storage) DeleteNode(nodeID NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from cache
	delete(s.nodeCache, nodeID)

	// Add to free list
	s.nodePool.Free(nodeID)

	return nil
}

// Sync syncs the storage to disk
func (s *Storage) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.file.Sync()
}
