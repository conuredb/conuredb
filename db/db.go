package db

import (
	"errors"
	"sync"

	"github.com/conure-db/conure-db/btree"
)

// DB represents a key-value database
type DB struct {
	mu       sync.RWMutex
	tree     *btree.BTree
	path     string
	isClosed bool
}

// Open opens a database
func Open(path string) (*DB, error) {
	tree, err := btree.NewBTree(path)
	if err != nil {
		return nil, err
	}

	return &DB{
		tree: tree,
		path: path,
	}, nil
}

// Close closes the database
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.isClosed {
		return errors.New("database already closed")
	}

	db.isClosed = true
	return db.tree.Close()
}

// Get gets a value from the database
func (db *DB) Get(key []byte) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.isClosed {
		return nil, errors.New("database closed")
	}

	return db.tree.Get(key)
}

// Put puts a key-value pair in the database
func (db *DB) Put(key, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.isClosed {
		return errors.New("database closed")
	}

	return db.tree.Put(key, value)
}

// Delete deletes a key from the database
func (db *DB) Delete(key []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.isClosed {
		return errors.New("database closed")
	}

	return db.tree.Delete(key)
}

// Sync syncs the database to disk
func (db *DB) Sync() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.isClosed {
		return errors.New("database closed")
	}

	return db.tree.Sync()
}
