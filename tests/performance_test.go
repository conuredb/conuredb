package tests

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/conure-db/conure-db/db"
)

const (
	perfTestDBPath = "performance.db"
)

// TestPerformancePut measures the performance of a single Put operation
func TestPerformancePut(t *testing.T) {
	// Remove any existing test database
	os.Remove(perfTestDBPath)

	// Create a new database
	database, err := db.Open(perfTestDBPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		database.Close()
		os.Remove(perfTestDBPath)
	}()

	// Measure a single Put operation
	key := []byte("key")
	value := []byte("value")

	start := time.Now()
	if err := database.Put(key, value); err != nil {
		t.Fatalf("Failed to put: %v", err)
	}
	duration := time.Since(start)

	t.Logf("Put operation took: %v", duration)
}

// TestPerformanceGet measures the performance of a single Get operation
func TestPerformanceGet(t *testing.T) {
	// Remove any existing test database
	os.Remove(perfTestDBPath)

	// Create a new database
	database, err := db.Open(perfTestDBPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		database.Close()
		os.Remove(perfTestDBPath)
	}()

	// Insert a key-value pair
	key := []byte("key")
	value := []byte("value")
	if err := database.Put(key, value); err != nil {
		t.Fatalf("Failed to put: %v", err)
	}

	// Measure a single Get operation
	start := time.Now()
	_, err = database.Get(key)
	if err != nil {
		t.Fatalf("Failed to get: %v", err)
	}
	duration := time.Since(start)

	t.Logf("Get operation took: %v", duration)
}

// TestPerformanceDelete measures the performance of a single Delete operation
func TestPerformanceDelete(t *testing.T) {
	// Remove any existing test database
	os.Remove(perfTestDBPath)

	// Create a new database
	database, err := db.Open(perfTestDBPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		database.Close()
		os.Remove(perfTestDBPath)
	}()

	// Insert a key-value pair
	key := []byte("key")
	value := []byte("value")
	if err := database.Put(key, value); err != nil {
		t.Fatalf("Failed to put: %v", err)
	}

	// Measure a single Delete operation
	start := time.Now()
	if err := database.Delete(key); err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}
	duration := time.Since(start)

	t.Logf("Delete operation took: %v", duration)
}

// TestPerformanceSync measures the performance of a single Sync operation
func TestPerformanceSync(t *testing.T) {
	// Remove any existing test database
	os.Remove(perfTestDBPath)

	// Create a new database
	database, err := db.Open(perfTestDBPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		database.Close()
		os.Remove(perfTestDBPath)
	}()

	// Insert a key-value pair
	key := []byte("key")
	value := []byte("value")
	if err := database.Put(key, value); err != nil {
		t.Fatalf("Failed to put: %v", err)
	}

	// Measure a single Sync operation
	start := time.Now()
	if err := database.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}
	duration := time.Since(start)

	t.Logf("Sync operation took: %v", duration)
}

// TestPerformanceMultipleOperations measures the performance of multiple operations
func TestPerformanceMultipleOperations(t *testing.T) {
	// Remove any existing test database
	os.Remove(perfTestDBPath)

	// Create a new database
	database, err := db.Open(perfTestDBPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		database.Close()
		os.Remove(perfTestDBPath)
	}()

	// Perform a sequence of operations
	numOperations := 5
	start := time.Now()

	for i := 0; i < numOperations; i++ {
		// Put
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		if err := database.Put(key, value); err != nil {
			t.Fatalf("Failed to put: %v", err)
		}

		// Sync after each operation
		if err := database.Sync(); err != nil {
			t.Fatalf("Failed to sync: %v", err)
		}
	}

	duration := time.Since(start)
	t.Logf("%d Put+Sync operations took: %v (avg: %v per operation)",
		numOperations, duration, duration/time.Duration(numOperations))

	// Get operations
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		_, err := database.Get(key)
		if err != nil {
			t.Fatalf("Failed to get: %v", err)
		}
	}
	duration = time.Since(start)
	t.Logf("%d Get operations took: %v (avg: %v per operation)",
		numOperations, duration, duration/time.Duration(numOperations))

	// Delete operations
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		if err := database.Delete(key); err != nil {
			t.Fatalf("Failed to delete: %v", err)
		}

		// Sync after each operation
		if err := database.Sync(); err != nil {
			t.Fatalf("Failed to sync: %v", err)
		}
	}
	duration = time.Since(start)
	t.Logf("%d Delete+Sync operations took: %v (avg: %v per operation)",
		numOperations, duration, duration/time.Duration(numOperations))
}
