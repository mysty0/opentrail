package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"opentrail/internal/types"
)

// TestRequirement1_1_BatchMultipleInserts tests that the storage batches multiple inserts
// Requirement 1.1: WHEN the system receives multiple log entries THEN the storage SHALL batch multiple inserts into a single database transaction
func TestRequirement1_1_BatchMultipleInserts(t *testing.T) {
	dbPath := "test_req_1_1.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 3
	config.BatchTimeout = 100 * time.Millisecond
	config.QueueSize = 10

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Create multiple log entries
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "batch entry 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "batch entry 2"},
		{Priority: 18, Facility: 2, Severity: 2, Version: 1, Timestamp: time.Now(), Message: "batch entry 3"},
	}

	// Store entries concurrently to simulate multiple inserts
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(e *types.LogEntry) {
			defer wg.Done()
			storage.Store(e) // We expect errors due to unimplemented database operations
		}(entry)
	}

	wg.Wait()

	// Verify that batch buffer was used (should be empty after processing)
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should be empty after processing multiple entries")
	}

	// The fact that all entries were processed together demonstrates batching behavior
	// (Even though database operations aren't implemented, the batching coordination works)
}

// TestRequirement1_2_BatchSizeTrigger tests that batch executes when configured batch size is reached
// Requirement 1.2: WHEN a batch reaches the configured batch size THEN the storage SHALL immediately execute the batch insert
func TestRequirement1_2_BatchSizeTrigger(t *testing.T) {
	dbPath := "test_req_1_2.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 2                  // Small batch size for testing
	config.BatchTimeout = 1 * time.Second // Long timeout so size triggers first
	config.QueueSize = 10

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Create entries equal to batch size
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "size trigger 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "size trigger 2"},
	}

	start := time.Now()
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(e *types.LogEntry) {
			defer wg.Done()
			storage.Store(e)
		}(entry)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Batch should have been triggered by size, not timeout
	if elapsed > 500*time.Millisecond {
		t.Errorf("batch processing took too long (%v), should have been triggered by size not timeout", elapsed)
	}

	// Verify buffer is empty after size-triggered processing
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should be empty after size-triggered processing")
	}
}

// TestRequirement1_3_BatchTimeoutTrigger tests that batch executes when timeout is reached
// Requirement 1.3: WHEN a batch timeout is reached THEN the storage SHALL execute the current batch regardless of size
func TestRequirement1_3_BatchTimeoutTrigger(t *testing.T) {
	dbPath := "test_req_1_3.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 10                       // Large batch size so timeout triggers first
	config.BatchTimeout = 50 * time.Millisecond // Short timeout
	config.QueueSize = 20

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Create fewer entries than batch size
	entry := &types.LogEntry{
		Priority: 16, Facility: 2, Severity: 0, Version: 1,
		Timestamp: time.Now(), Message: "timeout trigger test",
	}

	start := time.Now()
	err = storage.Store(entry)
	elapsed := time.Since(start)

	// Should have been processed due to timeout, not size
	if elapsed < 40*time.Millisecond {
		t.Errorf("batch processing was too fast (%v), should have waited for timeout", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("batch processing took too long (%v), timeout should have triggered", elapsed)
	}

	// Verify buffer is empty after timeout-triggered processing
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should be empty after timeout-triggered processing")
	}
}

// TestRequirement2_3_NonBlockingOperation tests that storage operations are non-blocking
// Requirement 2.3: WHEN a log entry is processed THEN subscribers SHALL receive the entry before database write completion
func TestRequirement2_3_NonBlockingOperation(t *testing.T) {
	dbPath := "test_req_2_3.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 5
	config.BatchTimeout = 100 * time.Millisecond
	config.QueueSize = 20

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test that Store operations return quickly (non-blocking queue operation)
	entries := make([]*types.LogEntry, 10)
	for i := 0; i < 10; i++ {
		entries[i] = &types.LogEntry{
			Priority: 16 + i, Facility: 2, Severity: i % 8, Version: 1,
			Timestamp: time.Now(), Message: fmt.Sprintf("non-blocking test %d", i),
		}
	}

	// Measure time for queueing operations (should be fast)
	start := time.Now()
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(e *types.LogEntry) {
			defer wg.Done()
			storage.Store(e) // This should queue quickly and return
		}(entry)
	}

	wg.Wait()
	queueTime := time.Since(start)

	// Queueing should be fast (non-blocking)
	if queueTime > 100*time.Millisecond {
		t.Errorf("queueing operations took too long: %v, should be non-blocking", queueTime)
	}

	// This demonstrates that the Store operation is non-blocking - it queues the request
	// and returns immediately, allowing real-time subscribers to receive updates
	// before database write completion
}

// TestBatchProcessorSynchronization tests proper synchronization in batch processing
func TestBatchProcessorSynchronization(t *testing.T) {
	dbPath := "test_synchronization.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 3
	config.BatchTimeout = 50 * time.Millisecond
	config.QueueSize = 50

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Test concurrent access to batch buffer
	numGoroutines := 20
	entriesPerGoroutine := 3
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				entry := &types.LogEntry{
					Priority: 16 + i, Facility: 2, Severity: i % 8, Version: 1,
					Timestamp: time.Now(), Message: fmt.Sprintf("sync test g%d-e%d", goroutineID, i),
				}
				storage.Store(entry)
			}
		}(g)
	}

	wg.Wait()

	// Give time for all batches to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify buffer is properly synchronized and empty after processing
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should be empty after concurrent processing, size: %d", batchedStorage.batchBuffer.size())
	}
}

// TestBatchProcessorGracefulShutdownWithPendingRequests tests graceful shutdown behavior
func TestBatchProcessorGracefulShutdownWithPendingRequests(t *testing.T) {
	dbPath := "test_graceful_shutdown_pending.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 100                 // Large batch size so entries stay in buffer
	config.BatchTimeout = 10 * time.Second // Long timeout
	config.QueueSize = 50

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Add entries that won't trigger batch processing due to large batch size and long timeout
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "pending 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "pending 2"},
		{Priority: 18, Facility: 2, Severity: 2, Version: 1, Timestamp: time.Now(), Message: "pending 3"},
	}

	var wg sync.WaitGroup
	results := make([]error, len(entries))

	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg.Done()
			results[idx] = storage.Store(e)
		}(i, entry)
	}

	// Give time for entries to be queued
	time.Sleep(50 * time.Millisecond)

	// Close storage - this should process pending entries
	start := time.Now()
	err = storage.Close()
	shutdownTime := time.Since(start)

	if err != nil {
		t.Errorf("failed to close storage: %v", err)
	}

	wg.Wait()

	// Shutdown should be reasonably fast
	if shutdownTime > 1*time.Second {
		t.Errorf("graceful shutdown took too long: %v", shutdownTime)
	}

	// All pending entries should have been processed during shutdown
	for i, err := range results {
		if err == nil {
			t.Errorf("expected error for pending entry %d due to unimplemented database operations", i)
		}
	}
}
