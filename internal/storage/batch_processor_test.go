package storage

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"opentrail/internal/types"
)

func TestBatchProcessor_BatchSizeTrigger(t *testing.T) {
	dbPath := "test_batch_size_trigger.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 3                  // Small batch size for testing
	config.BatchTimeout = 1 * time.Second // Long timeout so size triggers first
	config.QueueSize = 10

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Create test entries
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "message 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "message 2"},
		{Priority: 18, Facility: 2, Severity: 2, Version: 1, Timestamp: time.Now(), Message: "message 3"},
	}

	// Store entries - this should trigger batch processing when the 3rd entry is added
	var wg sync.WaitGroup
	results := make([]error, len(entries))

	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg.Done()
			results[idx] = storage.Store(e)
		}(i, entry)
	}

	// Wait for all stores to complete
	wg.Wait()

	// All should have received results (now that database operations are implemented)
	for i, err := range results {
		if err != nil {
			t.Errorf("entry %d should succeed now that database operations are implemented: %v", i, err)
		}
	}

	// Verify batch buffer is empty after processing
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should be empty after processing, size: %d", batchedStorage.batchBuffer.size())
	}
}

func TestBatchProcessor_TimeoutTrigger(t *testing.T) {
	dbPath := "test_timeout_trigger.db"
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

	// Create test entries (fewer than batch size)
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "timeout message 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "timeout message 2"},
	}

	// Store entries
	var wg sync.WaitGroup
	results := make([]error, len(entries))

	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg.Done()
			results[idx] = storage.Store(e)
		}(i, entry)
	}

	// Wait for all stores to complete (should happen due to timeout)
	wg.Wait()

	// All should have received results due to timeout trigger
	for i, err := range results {
		if err != nil {
			t.Errorf("entry %d should succeed now that database operations are implemented: %v", i, err)
		}
	}

	// Verify batch buffer is empty after timeout processing
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should be empty after timeout processing, size: %d", batchedStorage.batchBuffer.size())
	}
}

func TestBatchProcessor_MixedTriggers(t *testing.T) {
	dbPath := "test_mixed_triggers.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 3
	config.BatchTimeout = 100 * time.Millisecond
	config.QueueSize = 20

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// First batch: trigger by size (3 entries)
	entries1 := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "batch1 message 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "batch1 message 2"},
		{Priority: 18, Facility: 2, Severity: 2, Version: 1, Timestamp: time.Now(), Message: "batch1 message 3"},
	}

	var wg1 sync.WaitGroup
	results1 := make([]error, len(entries1))

	for i, entry := range entries1 {
		wg1.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg1.Done()
			results1[idx] = storage.Store(e)
		}(i, entry)
	}

	wg1.Wait()

	// Verify first batch was processed
	for i, err := range results1 {
		if err != nil {
			t.Errorf("batch1 entry %d should succeed now that database operations are implemented: %v", i, err)
		}
	}

	// Second batch: trigger by timeout (2 entries, less than batch size)
	entries2 := []*types.LogEntry{
		{Priority: 19, Facility: 2, Severity: 3, Version: 1, Timestamp: time.Now(), Message: "batch2 message 1"},
		{Priority: 20, Facility: 2, Severity: 4, Version: 1, Timestamp: time.Now(), Message: "batch2 message 2"},
	}

	var wg2 sync.WaitGroup
	results2 := make([]error, len(entries2))

	for i, entry := range entries2 {
		wg2.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg2.Done()
			results2[idx] = storage.Store(e)
		}(i, entry)
	}

	wg2.Wait()

	// Verify second batch was processed by timeout
	for i, err := range results2 {
		if err != nil {
			t.Errorf("batch2 entry %d should succeed now that database operations are implemented: %v", i, err)
		}
	}
}

func TestBatchProcessor_ContextCancellation(t *testing.T) {
	dbPath := "test_context_cancellation.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 5
	config.BatchTimeout = 1 * time.Second // Long timeout
	config.QueueSize = 10

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create test entries
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "cancel message 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "cancel message 2"},
	}

	// Start storing entries
	var wg sync.WaitGroup
	results := make([]error, len(entries))

	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg.Done()
			results[idx] = storage.Store(e)
		}(i, entry)
	}

	// Close storage to trigger context cancellation
	go func() {
		time.Sleep(10 * time.Millisecond)
		storage.Close()
	}()

	wg.Wait()

	// All entries should receive cancellation errors
	for i, err := range results {
		if err == nil {
			t.Errorf("expected cancellation error for entry %d", i)
		} else if !contains(err.Error(), "cancelled") && !contains(err.Error(), "not running") {
			t.Errorf("entry %d should get cancellation or not running error, got: %v", i, err)
		}
	}
}

func TestBatchProcessor_GracefulShutdown(t *testing.T) {
	dbPath := "test_graceful_shutdown.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 10                 // Large batch size
	config.BatchTimeout = 1 * time.Second // Long timeout
	config.QueueSize = 20

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Add some entries to the buffer
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "shutdown message 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "shutdown message 2"},
		{Priority: 18, Facility: 2, Severity: 2, Version: 1, Timestamp: time.Now(), Message: "shutdown message 3"},
	}

	var wg sync.WaitGroup
	results := make([]error, len(entries))

	// Start storing entries (they won't trigger batch processing due to large batch size and long timeout)
	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg.Done()
			results[idx] = storage.Store(e)
		}(i, entry)
	}

	// Give a moment for entries to be queued
	time.Sleep(10 * time.Millisecond)

	// Close storage - this should process remaining entries
	err = storage.Close()
	if err != nil {
		t.Errorf("failed to close storage: %v", err)
	}

	wg.Wait()

	// All entries should have been processed during shutdown (may get context cancellation errors)
	for i, err := range results {
		if err != nil && !contains(err.Error(), "context canceled") && !contains(err.Error(), "cancelled") {
			t.Errorf("entry %d should either succeed or fail with context cancellation: %v", i, err)
		}
	}

	// Verify batch buffer is empty after shutdown
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should be empty after shutdown, size: %d", batchedStorage.batchBuffer.size())
	}

	// Verify storage is not running
	batchedStorage.runningMux.RLock()
	isRunning := batchedStorage.isRunning
	batchedStorage.runningMux.RUnlock()

	if isRunning {
		t.Errorf("storage should not be running after close")
	}
}

func TestBatchProcessor_ConcurrentAccess(t *testing.T) {
	dbPath := "test_concurrent_access.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 5
	config.BatchTimeout = 50 * time.Millisecond
	config.QueueSize = 100

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create many entries to test concurrent access
	numGoroutines := 10
	entriesPerGoroutine := 5
	totalEntries := numGoroutines * entriesPerGoroutine

	var wg sync.WaitGroup
	results := make([][]error, numGoroutines)

	// Start multiple goroutines storing entries concurrently
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		results[g] = make([]error, entriesPerGoroutine)

		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < entriesPerGoroutine; i++ {
				entry := &types.LogEntry{
					Priority:  16 + i,
					Facility:  2,
					Severity:  i % 8,
					Version:   1,
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("concurrent message g%d-e%d", goroutineID, i),
				}

				results[goroutineID][i] = storage.Store(entry)
			}
		}(g)
	}

	wg.Wait()

	// Count successful operations (all should succeed now that database operations are implemented)
	processedCount := 0
	for g := 0; g < numGoroutines; g++ {
		for i := 0; i < entriesPerGoroutine; i++ {
			if results[g][i] == nil {
				processedCount++
			}
		}
	}

	if processedCount != totalEntries {
		t.Errorf("expected %d processed entries, got %d", totalEntries, processedCount)
	}
}

func TestBatchProcessor_BufferManagement(t *testing.T) {
	dbPath := "test_buffer_management.db"
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

	// Verify buffer starts empty
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("batch buffer should start empty")
	}

	// Add one entry
	entry1 := &types.LogEntry{
		Priority: 16, Facility: 2, Severity: 0, Version: 1,
		Timestamp: time.Now(), Message: "buffer test 1",
	}

	go func() {
		storage.Store(entry1)
	}()

	// Give time for entry to be added to buffer
	time.Sleep(10 * time.Millisecond)

	// Buffer should have 1 entry (before timeout triggers)
	if batchedStorage.batchBuffer.size() != 1 {
		t.Errorf("expected buffer size 1, got %d", batchedStorage.batchBuffer.size())
	}

	// Wait for timeout to trigger processing
	time.Sleep(150 * time.Millisecond)

	// Buffer should be empty after timeout processing
	if !batchedStorage.batchBuffer.isEmpty() {
		t.Errorf("buffer should be empty after timeout, size: %d", batchedStorage.batchBuffer.size())
	}
}

func TestBatchProcessor_TimerManagement(t *testing.T) {
	dbPath := "test_timer_management.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 5 // Won't be reached in this test
	config.BatchTimeout = 50 * time.Millisecond
	config.QueueSize = 10

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Add entry to start timer
	entry := &types.LogEntry{
		Priority: 16, Facility: 2, Severity: 0, Version: 1,
		Timestamp: time.Now(), Message: "timer test",
	}

	start := time.Now()
	err = storage.Store(entry)
	elapsed := time.Since(start)

	// Store should return within reasonable time (includes batch processing time)
	if elapsed > 200*time.Millisecond {
		t.Errorf("Store took too long: %v", elapsed)
	}

	// Operation should succeed now that database operations are implemented
	if err != nil {
		t.Errorf("operation should succeed now that database operations are implemented: %v", err)
	}
}

func TestBatchProcessor_RequestContextHandling(t *testing.T) {
	dbPath := "test_request_context.db"
	defer cleanupDB(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 2
	config.BatchTimeout = 100 * time.Millisecond
	config.QueueSize = 10

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create entries
	entries := []*types.LogEntry{
		{Priority: 16, Facility: 2, Severity: 0, Version: 1, Timestamp: time.Now(), Message: "context test 1"},
		{Priority: 17, Facility: 2, Severity: 1, Version: 1, Timestamp: time.Now(), Message: "context test 2"},
	}

	var wg sync.WaitGroup
	results := make([]error, len(entries))

	// Store entries to trigger batch processing
	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg.Done()
			results[idx] = storage.Store(e)
		}(i, entry)
	}

	wg.Wait()

	// Both should succeed now that database operations are implemented
	for i, err := range results {
		if err != nil {
			t.Errorf("entry %d should succeed now that database operations are implemented: %v", i, err)
		}
	}
}

// Helper function to clean up database files
func cleanupDB(dbPath string) {
	os.Remove(dbPath)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")
}
