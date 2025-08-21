package btree

import (
	"bytes"
	"errors"
	"sync"
)

const (
	// MaxItems is the maximum number of items in a node (soft cap; actual fit is enforced by size estimation)
	MaxItems = 255

	// MinItems is the minimum number of items in a node
	MinItems = MaxItems / 2
)

var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrKeyTooLarge   = errors.New("key too large")
	ErrValueTooLarge = errors.New("value too large")
)

// BTree represents a B-tree
type BTree struct {
	mu      sync.RWMutex
	storage *Storage
}

// NewBTree creates a new B-tree
func NewBTree(storagePath string) (*BTree, error) {
	storage, err := OpenStorage(storagePath)
	if err != nil {
		return nil, err
	}

	return &BTree{
		storage: storage,
	}, nil
}

// Reload refreshes in-memory metadata to reflect external changes.
func (t *BTree) Reload() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.storage.ReloadHeader()
}

// Close closes the B-tree
func (t *BTree) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.storage.Close()
}

// Get gets a value from the B-tree
func (t *BTree) Get(key []byte) ([]byte, error) {
	if len(key) > MaxKeySize {
		return nil, ErrKeyTooLarge
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Get the root node
	root, err := t.storage.GetRootNode()
	if err != nil {
		return nil, err
	}

	// Search for the key
	return t.search(root, key)
}

// search searches for a key in the B-tree
func (t *BTree) search(node *Node, key []byte) ([]byte, error) {
	if node.nodeType == LeafNode {
		// Search in leaf node
		for _, item := range node.items {
			if bytes.Equal(item.Key, key) {
				return item.Value, nil
			}
		}
		return nil, ErrKeyNotFound
	}

	// Search in internal node
	childPos := node.FindChildPos(key)
	childID := node.children[childPos]
	child, err := t.storage.GetNode(childID)
	if err != nil {
		return nil, err
	}

	return t.search(child, key)
}

// Put puts a key-value pair in the B-tree
func (t *BTree) Put(key []byte, value []byte) error {
	if len(key) > MaxKeySize {
		return ErrKeyTooLarge
	}
	if len(value) > MaxValueSize {
		return ErrValueTooLarge
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Begin transaction
	if err := t.storage.BeginTransaction(); err != nil {
		return err
	}

	// Get the root node
	root, err := t.storage.GetRootNode()
	if err != nil {
		t.storage.abortTransaction()
		return err
	}

	// Insert the key-value pair
	newRoot, split, err := t.insert(root, key, value)
	if err != nil {
		t.storage.abortTransaction()
		return err
	}

	// Handle root split
	if split {
		// Create a new root
		newRootID := t.storage.nodePool.Allocate()
		rootNode := NewInternalNode(newRootID)

		// Add the old root as a child
		if err := rootNode.AddChild(0, root.id); err != nil {
			t.storage.abortTransaction()
			return err
		}

		// Add the new node (returned from insert) as a child
		if err := rootNode.AddChild(1, newRoot.id); err != nil {
			t.storage.abortTransaction()
			return err
		}

		// Add the split key from the new node
		rootNode.AddItem(Item{Key: newRoot.items[0].Key, Value: nil})

		// Update children's parent pointers
		if err := t.setParent(root.id, rootNode.id); err != nil {
			t.storage.abortTransaction()
			return err
		}
		if err := t.setParent(newRoot.id, rootNode.id); err != nil {
			t.storage.abortTransaction()
			return err
		}

		// Set the new root
		if err := t.storage.SetRootNode(rootNode); err != nil {
			t.storage.abortTransaction()
			return err
		}
	} else if newRoot != nil && newRoot.id != root.id {
		// Set the new root (no split but path-copied root)
		if err := t.storage.SetRootNode(newRoot); err != nil {
			t.storage.abortTransaction()
			return err
		}
	}

	// Commit transaction
	return t.storage.CommitTransaction()
}

// estimateNodeSize computes the size if node had its current content;
// if withItem!=nil, includes that item; if withNewChild>=0, includes one new child pointer.
func estimateNodeSize(node *Node, withItem *Item, withNewChild int) int {
	size := NodeHeaderSize
	// items
	for _, it := range node.items {
		size += 2 + len(it.Key) + 4 + len(it.Value)
	}
	if withItem != nil {
		size += 2 + len(withItem.Key) + 4 + len(withItem.Value)
	}
	// children ids for internal nodes
	if node.nodeType == InternalNode {
		childCount := len(node.children)
		if withNewChild >= 0 {
			childCount++
		}
		size += 8 * childCount
	}
	return size
}

// insert inserts a key-value pair in a node
func (t *BTree) insert(node *Node, key []byte, value []byte) (*Node, bool, error) {
	if node.nodeType == LeafNode {
		// Check if the key already exists
		pos := node.FindKey(key)
		if pos >= 0 {
			// Update the value
			node.items[pos].Value = value
			return node, false, t.storage.PutNode(node)
		}

		// Create a copy of the node (copy-on-write)
		nodeCopy, err := t.storage.CloneNode(node)
		if err != nil {
			return nil, false, err
		}

		// Ensure adding the item will fit the page; if not, split first
		candidate := Item{Key: key, Value: value}
		if estimateNodeSize(nodeCopy, &candidate, -1) > NodeSize || len(nodeCopy.items)+1 > MaxItems {
			// Split first, then insert into the appropriate half by recursing
			newSibling, _, err := t.splitLeaf(nodeCopy)
			if err != nil {
				return nil, false, err
			}
			// Decide target: compare to split boundary (first key of sibling)
			if bytes.Compare(key, newSibling.items[0].Key) < 0 {
				// insert into left (nodeCopy)
				nodeCopy.AddItem(candidate)
				if err := t.storage.PutNode(nodeCopy); err != nil {
					return nil, false, err
				}
				return newSibling, true, nil
			}
			// insert into right (newSibling)
			newSibling.AddItem(candidate)
			if err := t.storage.PutNode(newSibling); err != nil {
				return nil, false, err
			}
			return newSibling, true, nil
		}

		// Add the item
		nodeCopy.AddItem(candidate)

		// Check if the node needs to be split by count (secondary guard)
		if len(nodeCopy.items) > MaxItems || estimateNodeSize(nodeCopy, nil, -1) > NodeSize {
			return t.splitLeaf(nodeCopy)
		}

		return nodeCopy, false, nil
	}

	// Internal node
	childPos := node.FindChildPos(key)
	childID := node.children[childPos]
	child, err := t.storage.GetNode(childID)
	if err != nil {
		return nil, false, err
	}

	// Recursively insert in the child
	newChild, split, err := t.insert(child, key, value)
	if err != nil {
		return nil, false, err
	}

	if !split {
		// No split occurred, update the child pointer if needed
		if newChild != nil && newChild.id != child.id {
			// Create a copy of the node (copy-on-write)
			nodeCopy, err := t.storage.CloneNode(node)
			if err != nil {
				return nil, false, err
			}

			// Update the child pointer
			nodeCopy.children[childPos] = newChild.id

			// Maintain child's parent pointer
			if err := t.setParent(newChild.id, nodeCopy.id); err != nil {
				return nil, false, err
			}

			return nodeCopy, false, nil
		}

		return node, false, nil
	}

	// Split occurred in child, create a copy of this node (copy-on-write)
	nodeCopy, err := t.storage.CloneNode(node)
	if err != nil {
		return nil, false, err
	}

	// Add the new child and split key
	splitKey := newChild.items[0].Key
	// Ensure capacity for key and new child pointer
	if estimateNodeSize(nodeCopy, &Item{Key: splitKey, Value: nil}, childPos+1) > NodeSize || len(nodeCopy.items)+1 > MaxItems {
		// Split this internal node before inserting
		promoted, _, err := t.splitInternal(nodeCopy)
		if err != nil {
			return nil, false, err
		}
		// After splitting, the caller handles parent propagation
		return promoted, true, nil
	}

	nodeCopy.AddItem(Item{Key: splitKey, Value: nil})
	if err := nodeCopy.AddChild(childPos+1, newChild.id); err != nil {
		return nil, false, err
	}

	// Maintain new child's parent pointer
	if err := t.setParent(newChild.id, nodeCopy.id); err != nil {
		return nil, false, err
	}

	// Check if the node needs to be split
	if len(nodeCopy.items) > MaxItems || estimateNodeSize(nodeCopy, nil, -1) > NodeSize {
		return t.splitInternal(nodeCopy)
	}

	return nodeCopy, false, nil
}

// setParent updates a child's parent pointer and persists it in the current tx
func (t *BTree) setParent(childID NodeID, parentID NodeID) error {
	child, err := t.storage.GetNode(childID)
	if err != nil {
		return err
	}
	childCopy, err := t.storage.CloneNode(child)
	if err != nil {
		return err
	}
	childCopy.SetParent(parentID)
	return t.storage.PutNode(childCopy)
}

// splitLeaf splits a leaf node
func (t *BTree) splitLeaf(node *Node) (*Node, bool, error) {
	// Create a new node
	newNodeID := t.storage.nodePool.Allocate()
	newNode := NewLeafNode(newNodeID)

	// Move half of the items to the new node
	mid := len(node.items) / 2
	newNode.items = append(newNode.items, node.items[mid:]...)
	node.items = node.items[:mid]
	node.count = uint16(len(node.items))
	newNode.count = uint16(len(newNode.items))

	// Set parents (new node inherits node.parent)
	newNode.parent = node.parent

	// Save the nodes
	if err := t.storage.PutNode(node); err != nil {
		return nil, false, err
	}
	if err := t.storage.PutNode(newNode); err != nil {
		return nil, false, err
	}

	return newNode, true, nil
}

// splitInternal splits an internal node
func (t *BTree) splitInternal(node *Node) (*Node, bool, error) {
	// Create a new node
	newNodeID := t.storage.nodePool.Allocate()
	newNode := NewInternalNode(newNodeID)

	// Move half of the items to the new node
	mid := len(node.items) / 2
	newNode.items = append(newNode.items, node.items[mid+1:]...)
	splitItem := node.items[mid]
	node.items = node.items[:mid]
	node.count = uint16(len(node.items))
	newNode.count = uint16(len(newNode.items))

	// Move half of the children to the new node
	newNode.children = append(newNode.children, node.children[mid+1:]...)
	node.children = node.children[:mid+1]

	// Update parent pointers for children moved to newNode
	for _, childID := range newNode.children {
		if err := t.setParent(childID, newNode.id); err != nil {
			return nil, false, err
		}
	}

	// Save the nodes
	if err := t.storage.PutNode(node); err != nil {
		return nil, false, err
	}
	if err := t.storage.PutNode(newNode); err != nil {
		return nil, false, err
	}

	// Return the split item
	newNode.items = append([]Item{splitItem}, newNode.items...)
	newNode.count = uint16(len(newNode.items))

	return newNode, true, nil
}

// Delete deletes a key from the B-tree
func (t *BTree) Delete(key []byte) error {
	if len(key) > MaxKeySize {
		return ErrKeyTooLarge
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Begin transaction
	if err := t.storage.BeginTransaction(); err != nil {
		return err
	}

	// Get the root node
	root, err := t.storage.GetRootNode()
	if err != nil {
		t.storage.abortTransaction()
		return err
	}

	// Delete the key
	newRoot, err := t.delete(root, key)
	if err != nil {
		t.storage.abortTransaction()
		return err
	}

	// Update the root if needed
	if newRoot != nil && newRoot.id != root.id {
		if err := t.storage.SetRootNode(newRoot); err != nil {
			t.storage.abortTransaction()
			return err
		}
	}

	// Commit transaction
	return t.storage.CommitTransaction()
}

// delete deletes a key from a node
func (t *BTree) delete(node *Node, key []byte) (*Node, error) {
	if node.nodeType == LeafNode {
		// Find the key
		pos := node.FindKey(key)
		if pos < 0 {
			return nil, ErrKeyNotFound
		}

		// Create a copy of the node (copy-on-write)
		nodeCopy, err := t.storage.CloneNode(node)
		if err != nil {
			return nil, err
		}

		// Remove the item
		if err := nodeCopy.RemoveItem(pos); err != nil {
			return nil, err
		}

		// Check if the node is underflowing
		if nodeCopy.count < MinItems && nodeCopy.parent != 0 {
			// Get the parent
			parent, err := t.storage.GetNode(nodeCopy.parent)
			if err != nil {
				return nil, err
			}

			// Rebalance
			return t.rebalanceLeaf(nodeCopy, parent)
		}

		return nodeCopy, nil
	}

	// Internal node
	childPos := node.FindChildPos(key)
	childID := node.children[childPos]
	child, err := t.storage.GetNode(childID)
	if err != nil {
		return nil, err
	}

	// Recursively delete from the child
	newChild, err := t.delete(child, key)
	if err != nil {
		return nil, err
	}

	// Create a copy of the node (copy-on-write)
	nodeCopy, err := t.storage.CloneNode(node)
	if err != nil {
		return nil, err
	}

	// Update the child pointer
	nodeCopy.children[childPos] = newChild.id

	// Check if the node is underflowing
	if newChild.count < MinItems && newChild.parent != 0 {
		// Rebalance
		return t.rebalanceInternal(newChild, nodeCopy)
	}

	return nodeCopy, nil
}

// rebalanceLeaf rebalances a leaf node
func (t *BTree) rebalanceLeaf(node *Node, parent *Node) (*Node, error) {
	// Find the position of the node in the parent
	pos := -1
	for i, childID := range parent.children {
		if childID == node.id {
			pos = i
			break
		}
	}
	if pos < 0 {
		return nil, errors.New("node not found in parent")
	}

	// Try to borrow from the left sibling
	if pos > 0 {
		leftSiblingID := parent.children[pos-1]
		leftSibling, err := t.storage.GetNode(leftSiblingID)
		if err != nil {
			return nil, err
		}

		if leftSibling.count > MinItems {
			// Create copies (copy-on-write)
			nodeCopy, err := t.storage.CloneNode(node)
			if err != nil {
				return nil, err
			}
			leftSiblingCopy, err := t.storage.CloneNode(leftSibling)
			if err != nil {
				return nil, err
			}
			parentCopy, err := t.storage.CloneNode(parent)
			if err != nil {
				return nil, err
			}

			// Borrow the rightmost item from the left sibling
			item := leftSiblingCopy.items[leftSiblingCopy.count-1]
			nodeCopy.AddItem(item)
			if err := leftSiblingCopy.RemoveItem(int(leftSiblingCopy.count) - 1); err != nil {
				return nil, err
			}

			// Update the parent's key
			parentCopy.items[pos-1].Key = nodeCopy.items[0].Key

			// Save the nodes
			if err := t.storage.PutNode(nodeCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(leftSiblingCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(parentCopy); err != nil {
				return nil, err
			}

			return parentCopy, nil
		}
	}

	// Try to borrow from the right sibling
	if pos < len(parent.children)-1 {
		rightSiblingID := parent.children[pos+1]
		rightSibling, err := t.storage.GetNode(rightSiblingID)
		if err != nil {
			return nil, err
		}

		if rightSibling.count > MinItems {
			// Create copies (copy-on-write)
			nodeCopy, err := t.storage.CloneNode(node)
			if err != nil {
				return nil, err
			}
			rightSiblingCopy, err := t.storage.CloneNode(rightSibling)
			if err != nil {
				return nil, err
			}
			parentCopy, err := t.storage.CloneNode(parent)
			if err != nil {
				return nil, err
			}

			// Borrow the leftmost item from the right sibling
			item := rightSiblingCopy.items[0]
			nodeCopy.AddItem(item)
			if err := rightSiblingCopy.RemoveItem(0); err != nil {
				return nil, err
			}

			// Update the parent's key
			parentCopy.items[pos].Key = rightSiblingCopy.items[0].Key

			// Save the nodes
			if err := t.storage.PutNode(nodeCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(rightSiblingCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(parentCopy); err != nil {
				return nil, err
			}

			return parentCopy, nil
		}
	}

	// Merge with a sibling
	if pos > 0 {
		// Merge with left sibling
		leftSiblingID := parent.children[pos-1]
		leftSibling, err := t.storage.GetNode(leftSiblingID)
		if err != nil {
			return nil, err
		}

		// Create a copy of the left sibling (copy-on-write)
		leftSiblingCopy, err := t.storage.CloneNode(leftSibling)
		if err != nil {
			return nil, err
		}

		// Merge the node into the left sibling
		leftSiblingCopy.items = append(leftSiblingCopy.items, node.items...)
		leftSiblingCopy.count = uint16(len(leftSiblingCopy.items))

		// Create a copy of the parent (copy-on-write)
		parentCopy, err := t.storage.CloneNode(parent)
		if err != nil {
			return nil, err
		}

		// Remove the node from the parent
		if err := parentCopy.RemoveItem(pos - 1); err != nil {
			return nil, err
		}
		if err := parentCopy.RemoveChild(pos); err != nil {
			return nil, err
		}

		// Save the nodes
		if err := t.storage.PutNode(leftSiblingCopy); err != nil {
			return nil, err
		}
		if err := t.storage.PutNode(parentCopy); err != nil {
			return nil, err
		}

		// Delete the node
		if err := t.storage.DeleteNode(node.id); err != nil {
			return nil, err
		}

		return parentCopy, nil
	} else {
		// Merge with right sibling
		rightSiblingID := parent.children[pos+1]
		rightSibling, err := t.storage.GetNode(rightSiblingID)
		if err != nil {
			return nil, err
		}

		// Create a copy of the node (copy-on-write)
		nodeCopy, err := t.storage.CloneNode(node)
		if err != nil {
			return nil, err
		}

		// Merge the right sibling into the node
		nodeCopy.items = append(nodeCopy.items, rightSibling.items...)
		nodeCopy.count = uint16(len(nodeCopy.items))

		// Create a copy of the parent (copy-on-write)
		parentCopy, err := t.storage.CloneNode(parent)
		if err != nil {
			return nil, err
		}

		// Remove the right sibling from the parent
		if err := parentCopy.RemoveItem(pos); err != nil {
			return nil, err
		}
		if err := parentCopy.RemoveChild(pos + 1); err != nil {
			return nil, err
		}

		// Save the nodes
		if err := t.storage.PutNode(nodeCopy); err != nil {
			return nil, err
		}
		if err := t.storage.PutNode(parentCopy); err != nil {
			return nil, err
		}

		// Delete the right sibling
		if err := t.storage.DeleteNode(rightSibling.id); err != nil {
			return nil, err
		}

		return parentCopy, nil
	}
}

// rebalanceInternal rebalances an internal node
func (t *BTree) rebalanceInternal(node *Node, parent *Node) (*Node, error) {
	// Find the position of the node in the parent
	pos := -1
	for i, childID := range parent.children {
		if childID == node.id {
			pos = i
			break
		}
	}
	if pos < 0 {
		return nil, errors.New("node not found in parent")
	}

	// Try to borrow from the left sibling
	if pos > 0 {
		leftSiblingID := parent.children[pos-1]
		leftSibling, err := t.storage.GetNode(leftSiblingID)
		if err != nil {
			return nil, err
		}

		if leftSibling.count > MinItems {
			// Create copies (copy-on-write)
			nodeCopy, err := t.storage.CloneNode(node)
			if err != nil {
				return nil, err
			}
			leftSiblingCopy, err := t.storage.CloneNode(leftSibling)
			if err != nil {
				return nil, err
			}
			parentCopy, err := t.storage.CloneNode(parent)
			if err != nil {
				return nil, err
			}

			// Move the parent's key down to the node
			nodeCopy.items = append([]Item{{Key: parentCopy.items[pos-1].Key, Value: nil}}, nodeCopy.items...)
			nodeCopy.count = uint16(len(nodeCopy.items))

			// Move the left sibling's rightmost key up to the parent
			parentCopy.items[pos-1].Key = leftSiblingCopy.items[leftSiblingCopy.count-1].Key
			if err := leftSiblingCopy.RemoveItem(int(leftSiblingCopy.count) - 1); err != nil {
				return nil, err
			}

			// Move the left sibling's rightmost child to the node
			childID := leftSiblingCopy.children[len(leftSiblingCopy.children)-1]
			nodeCopy.children = append([]NodeID{childID}, nodeCopy.children...)
			leftSiblingCopy.children = leftSiblingCopy.children[:len(leftSiblingCopy.children)-1]

			// Update the child's parent
			if err := t.setParent(childID, nodeCopy.id); err != nil {
				return nil, err
			}

			// Save the nodes
			if err := t.storage.PutNode(nodeCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(leftSiblingCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(parentCopy); err != nil {
				return nil, err
			}

			return parentCopy, nil
		}
	}

	// Try to borrow from the right sibling
	if pos < len(parent.children)-1 {
		rightSiblingID := parent.children[pos+1]
		rightSibling, err := t.storage.GetNode(rightSiblingID)
		if err != nil {
			return nil, err
		}

		if rightSibling.count > MinItems {
			// Create copies (copy-on-write)
			nodeCopy, err := t.storage.CloneNode(node)
			if err != nil {
				return nil, err
			}
			rightSiblingCopy, err := t.storage.CloneNode(rightSibling)
			if err != nil {
				return nil, err
			}
			parentCopy, err := t.storage.CloneNode(parent)
			if err != nil {
				return nil, err
			}

			// Move the parent's key down to the node
			nodeCopy.items = append(nodeCopy.items, Item{Key: parentCopy.items[pos].Key, Value: nil})
			nodeCopy.count = uint16(len(nodeCopy.items))

			// Move the right sibling's leftmost key up to the parent
			parentCopy.items[pos].Key = rightSiblingCopy.items[0].Key
			if err := rightSiblingCopy.RemoveItem(0); err != nil {
				return nil, err
			}

			// Move the right sibling's leftmost child to the node
			childID := rightSiblingCopy.children[0]
			nodeCopy.children = append(nodeCopy.children, childID)
			rightSiblingCopy.children = rightSiblingCopy.children[1:]

			// Update the child's parent
			if err := t.setParent(childID, nodeCopy.id); err != nil {
				return nil, err
			}

			// Save the nodes
			if err := t.storage.PutNode(nodeCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(rightSiblingCopy); err != nil {
				return nil, err
			}
			if err := t.storage.PutNode(parentCopy); err != nil {
				return nil, err
			}

			return parentCopy, nil
		}
	}

	// Merge with a sibling
	if pos > 0 {
		// Merge with left sibling
		leftSiblingID := parent.children[pos-1]
		leftSibling, err := t.storage.GetNode(leftSiblingID)
		if err != nil {
			return nil, err
		}

		// Create copies (copy-on-write)
		leftSiblingCopy, err := t.storage.CloneNode(leftSibling)
		if err != nil {
			return nil, err
		}
		parentCopy, err := t.storage.CloneNode(parent)
		if err != nil {
			return nil, err
		}

		// Move the parent's key down to the left sibling
		leftSiblingCopy.items = append(leftSiblingCopy.items, Item{Key: parentCopy.items[pos-1].Key, Value: nil})

		// Merge the node's items into the left sibling
		leftSiblingCopy.items = append(leftSiblingCopy.items, node.items...)
		leftSiblingCopy.count = uint16(len(leftSiblingCopy.items))

		// Merge the node's children into the left sibling
		leftSiblingCopy.children = append(leftSiblingCopy.children, node.children...)

		// Update the children's parent
		for _, childID := range node.children {
			if err := t.setParent(childID, leftSiblingCopy.id); err != nil {
				return nil, err
			}
		}

		// Remove the node from the parent
		if err := parentCopy.RemoveItem(pos - 1); err != nil {
			return nil, err
		}
		if err := parentCopy.RemoveChild(pos); err != nil {
			return nil, err
		}

		// Save the nodes
		if err := t.storage.PutNode(leftSiblingCopy); err != nil {
			return nil, err
		}
		if err := t.storage.PutNode(parentCopy); err != nil {
			return nil, err
		}

		// Delete the node
		if err := t.storage.DeleteNode(node.id); err != nil {
			return nil, err
		}

		// Check if the parent is the root and has no items
		if parentCopy.id == t.storage.rootNodeID && parentCopy.count == 0 {
			// Make the left sibling the new root
			if err := t.storage.SetRootNode(leftSiblingCopy); err != nil {
				return nil, err
			}
			// Delete the parent
			if err := t.storage.DeleteNode(parentCopy.id); err != nil {
				return nil, err
			}
			return leftSiblingCopy, nil
		}

		return parentCopy, nil
	} else {
		// Merge with right sibling
		rightSiblingID := parent.children[pos+1]
		rightSibling, err := t.storage.GetNode(rightSiblingID)
		if err != nil {
			return nil, err
		}

		// Create copies (copy-on-write)
		nodeCopy, err := t.storage.CloneNode(node)
		if err != nil {
			return nil, err
		}
		parentCopy, err := t.storage.CloneNode(parent)
		if err != nil {
			return nil, err
		}

		// Move the parent's key down to the node
		nodeCopy.items = append(nodeCopy.items, Item{Key: parentCopy.items[pos].Key, Value: nil})

		// Merge the right sibling's items into the node
		nodeCopy.items = append(nodeCopy.items, rightSibling.items...)
		nodeCopy.count = uint16(len(nodeCopy.items))

		// Merge the right sibling's children into the node
		nodeCopy.children = append(nodeCopy.children, rightSibling.children...)

		// Update the children's parent
		for _, childID := range rightSibling.children {
			if err := t.setParent(childID, nodeCopy.id); err != nil {
				return nil, err
			}
		}

		// Remove the right sibling from the parent
		if err := parentCopy.RemoveItem(pos); err != nil {
			return nil, err
		}
		if err := parentCopy.RemoveChild(pos + 1); err != nil {
			return nil, err
		}

		// Save the nodes
		if err := t.storage.PutNode(nodeCopy); err != nil {
			return nil, err
		}
		if err := t.storage.PutNode(parentCopy); err != nil {
			return nil, err
		}

		// Delete the right sibling
		if err := t.storage.DeleteNode(rightSibling.id); err != nil {
			return nil, err
		}

		// Check if the parent is the root and has no items
		if parentCopy.id == t.storage.rootNodeID && parentCopy.count == 0 {
			// Make the node the new root
			if err := t.storage.SetRootNode(nodeCopy); err != nil {
				return nil, err
			}
			// Delete the parent
			if err := t.storage.DeleteNode(parentCopy.id); err != nil {
				return nil, err
			}
			return nodeCopy, nil
		}

		return parentCopy, nil
	}
}

// Sync syncs the B-tree to disk
func (t *BTree) Sync() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.storage.Sync()
}
