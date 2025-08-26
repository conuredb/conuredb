package db

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/conuredb/conuredb/btree"
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

// Reload refreshes in-memory metadata to reflect external changes.
func (db *DB) Reload() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.isClosed {
		return errors.New("database closed")
	}
	return db.tree.Reload()
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

// SnapshotTo streams a durable snapshot of the database file to w.
// This acquires the DB lock for the duration for simplicity and consistency.
func (db *DB) SnapshotTo(w io.Writer) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.isClosed {
		return errors.New("database closed")
	}

	// Ensure latest state is on disk
	if err := db.tree.Sync(); err != nil {
		return err
	}

	f, err := os.Open(db.path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close database file during snapshot: %v\n", closeErr)
		}
	}()

	_, err = io.Copy(w, f)
	return err
}

// RestoreFrom replaces the on-disk database with the provided snapshot stream.
// This closes and reopens the underlying B-Tree atomically via rename.
func (db *DB) RestoreFrom(r io.Reader) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.isClosed {
		return errors.New("database closed")
	}

	// Close the current tree to release file handles
	if err := db.tree.Close(); err != nil {
		return err
	}

	dir := filepath.Dir(db.path)
	tmpPath := filepath.Join(dir, ".conure.restore.tmp")
	// Write snapshot to a temp file
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmpFile, r); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close temp file after copy error: %v\n", closeErr)
		}
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close temp file after sync error: %v\n", closeErr)
		}
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Atomically replace the db file
	if err := os.Rename(tmpPath, db.path); err != nil {
		return err
	}

	// Reopen the tree
	tree, err := btree.NewBTree(db.path)
	if err != nil {
		return err
	}
	db.tree = tree

	return nil
}
