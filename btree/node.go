package btree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// NodeSize is the size of a node in bytes
	NodeSize = 4096

	// MaxKeySize is the maximum size of a key in bytes
	MaxKeySize = 128

	// MaxValueSize is the maximum size of a value in bytes
	MaxValueSize = 1024

	// NodeHeaderSize is the size of the node header in bytes
	NodeHeaderSize = 16
)

// NodeType represents the type of a node
type NodeType uint8

const (
	// LeafNode is a node that contains key-value pairs
	LeafNode NodeType = iota

	// InternalNode is a node that contains keys and child pointers
	InternalNode
)

// NodeID represents the ID of a node
type NodeID uint64

// Node represents a node in the B-tree
type Node struct {
	id       NodeID
	nodeType NodeType
	count    uint16
	parent   NodeID
	items    []Item
	children []NodeID // Only used for internal nodes
}

// Item represents a key-value pair in a node
type Item struct {
	Key   []byte
	Value []byte
}

// NewLeafNode creates a new leaf node
func NewLeafNode(id NodeID) *Node {
	return &Node{
		id:       id,
		nodeType: LeafNode,
		count:    0,
		parent:   0,
		items:    make([]Item, 0),
		children: nil,
	}
}

// NewInternalNode creates a new internal node
func NewInternalNode(id NodeID) *Node {
	return &Node{
		id:       id,
		nodeType: InternalNode,
		count:    0,
		parent:   0,
		items:    make([]Item, 0),
		children: make([]NodeID, 0),
	}
}

// ID returns the ID of the node
func (n *Node) ID() NodeID {
	return n.id
}

// Type returns the type of the node
func (n *Node) Type() NodeType {
	return n.nodeType
}

// Count returns the number of items in the node
func (n *Node) Count() uint16 {
	return n.count
}

// Parent returns the parent of the node
func (n *Node) Parent() NodeID {
	return n.parent
}

// SetParent sets the parent of the node
func (n *Node) SetParent(parent NodeID) {
	n.parent = parent
}

// Items returns the items in the node
func (n *Node) Items() []Item {
	return n.items
}

// Children returns the children of the node
func (n *Node) Children() []NodeID {
	return n.children
}

// AddItem adds an item to the node
func (n *Node) AddItem(item Item) {
	// Find the position to insert the item
	pos := 0
	for pos < len(n.items) && bytes.Compare(n.items[pos].Key, item.Key) < 0 {
		pos++
	}

	// Insert the item
	if pos < len(n.items) {
		n.items = append(n.items[:pos+1], n.items[pos:]...)
		n.items[pos] = item
	} else {
		n.items = append(n.items, item)
	}
	n.count++
}

// AddChild adds a child to the node
func (n *Node) AddChild(pos int, child NodeID) error {
	if n.nodeType != InternalNode {
		return errors.New("cannot add child to leaf node")
	}

	if pos < 0 || pos > len(n.children) {
		return errors.New("invalid position")
	}

	if pos == len(n.children) {
		n.children = append(n.children, child)
	} else {
		n.children = append(n.children[:pos+1], n.children[pos:]...)
		n.children[pos] = child
	}

	return nil
}

// RemoveItem removes an item from the node
func (n *Node) RemoveItem(pos int) error {
	if pos < 0 || pos >= len(n.items) {
		return errors.New("invalid position")
	}

	n.items = append(n.items[:pos], n.items[pos+1:]...)
	n.count--

	return nil
}

// RemoveChild removes a child from the node
func (n *Node) RemoveChild(pos int) error {
	if n.nodeType != InternalNode {
		return errors.New("cannot remove child from leaf node")
	}

	if pos < 0 || pos >= len(n.children) {
		return errors.New("invalid position")
	}

	n.children = append(n.children[:pos], n.children[pos+1:]...)

	return nil
}

// FindKey finds the position of a key in the node
func (n *Node) FindKey(key []byte) int {
	for i, item := range n.items {
		if bytes.Equal(item.Key, key) {
			return i
		}
	}
	return -1
}

// FindChildPos finds the position of the child that should contain the key
func (n *Node) FindChildPos(key []byte) int {
	if n.nodeType != InternalNode {
		return -1
	}

	for i, item := range n.items {
		if bytes.Compare(key, item.Key) < 0 {
			return i
		}
	}

	return len(n.items)
}

// Serialize serializes the node to a byte slice
func (n *Node) Serialize() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, NodeSize))

	// Write header
	if err := binary.Write(buf, binary.LittleEndian, n.id); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, n.nodeType); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, n.count); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, n.parent); err != nil {
		return nil, err
	}

	// Write items
	for _, item := range n.items {
		// Write key length
		keyLen := uint16(len(item.Key))
		if err := binary.Write(buf, binary.LittleEndian, keyLen); err != nil {
			return nil, err
		}

		// Write key
		if _, err := buf.Write(item.Key); err != nil {
			return nil, err
		}

		// Write value length
		valueLen := uint32(len(item.Value))
		if err := binary.Write(buf, binary.LittleEndian, valueLen); err != nil {
			return nil, err
		}

		// Write value
		if _, err := buf.Write(item.Value); err != nil {
			return nil, err
		}
	}

	// Write children for internal nodes
	if n.nodeType == InternalNode {
		for _, child := range n.children {
			if err := binary.Write(buf, binary.LittleEndian, child); err != nil {
				return nil, err
			}
		}
	}

	// Check if we've exceeded NodeSize
	currentSize := buf.Len()
	if currentSize > NodeSize {
		return nil, fmt.Errorf("node size %d exceeds maximum size %d", currentSize, NodeSize)
	}
	
	// Pad to NodeSize
	padding := make([]byte, NodeSize-currentSize)
	if _, err := buf.Write(padding); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Deserialize deserializes a byte slice to a node
func DeserializeNode(data []byte) (*Node, error) {
	if len(data) != NodeSize {
		return nil, errors.New("invalid data size")
	}

	buf := bytes.NewReader(data)
	node := &Node{}

	// Read header
	if err := binary.Read(buf, binary.LittleEndian, &node.id); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &node.nodeType); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &node.count); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &node.parent); err != nil {
		return nil, err
	}

	// Read items
	node.items = make([]Item, node.count)
	for i := uint16(0); i < node.count; i++ {
		// Read key length
		var keyLen uint16
		if err := binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
			return nil, err
		}

		// Read key
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(buf, key); err != nil {
			return nil, err
		}

		// Read value length
		var valueLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &valueLen); err != nil {
			return nil, err
		}

		// Read value
		value := make([]byte, valueLen)
		if _, err := io.ReadFull(buf, value); err != nil {
			return nil, err
		}

		node.items[i] = Item{Key: key, Value: value}
	}

	// Read children for internal nodes
	if node.nodeType == InternalNode {
		node.children = make([]NodeID, node.count+1)
		for i := uint16(0); i <= node.count; i++ {
			if err := binary.Read(buf, binary.LittleEndian, &node.children[i]); err != nil {
				return nil, err
			}
		}
	}

	return node, nil
}
