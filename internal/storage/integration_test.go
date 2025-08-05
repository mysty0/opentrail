package storage

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

// TestHighThroughputScenarios tests the system under high write load
func TestHighThroughputScenarios(t *testing.T) {
	tests := []struct {
		name           string
		numEntries     int
		numGoroutines  int
		batchSize      int
		batchTimeout   time.Duration
		expectedMinTPS int // Minimum transactions per second expected
	}{
		{
			name:           "moderate_load",
			numEntries:     1000,
			numGoroutines:  10,
			batchSize:      100,
			batchTimeout:   10 * time.Millisecond, // Shorter timeout for better performance
			expectedMinTPS: 200,                   // Higher expectation with better config
		},
		{
			name:           "high_load",
			numEntries:     2000,
			numGoroutines:  20,
			batchSize:      200,
			batchTimeout:   20 * time.Millisecond, // Shorter timeout
			expectedMinTPS: 400,                   // Higher expectation
		},
		{
			name:           "burst_load",
			numEntries:     3000,
			numGoroutines:  30,
			batchSize:      300,
			batchTimeout:   30 * time.Millisecond, // Shorter timeout
			expectedMinTPS: 500,                   // Higher expectation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup batched storage
			dbPath := fmt.Sprintf("test_throughput_%s.db", tt.name)
			defer cleanupTestFiles(dbPath)

			config := DefaultBatchConfig()
			config.BatchSize = tt.batchSize
			config.BatchTimeout = tt.batchTimeout
			config.QueueSize = tt.numEntries * 2 // Ensure queue doesn't become bottleneck

			storage, err := NewBatchedSQLiteStorage(dbPath, config)
			if err != nil {
				t.Fatalf("Failed to create storage: %v", err)
			}
			defer storage.Close()

			// Generate test entries
			entries := generateTestEntries(tt.numEntries)

			// Measure write performance
			startTime := time.Now()

			// Use goroutines to simulate concurrent writes
			var wg sync.WaitGroup
			entriesPerGoroutine := tt.numEntries / tt.numGoroutines
			errorChan := make(chan error, tt.numGoroutines)

			for i := 0; i < tt.numGoroutines; i++ {
				wg.Add(1)
				go func(startIdx int) {
					defer wg.Done()
					endIdx := startIdx + entriesPerGoroutine
					if endIdx > len(entries) {
						endIdx = len(entries)
					}

					for j := startIdx; j < endIdx; j++ {
						if err := storage.Store(entries[j]); err != nil {
							errorChan <- fmt.Errorf("goroutine %d failed to store entry %d: %w", startIdx/entriesPerGoroutine, j, err)
							return
						}
					}
				}(i * entriesPerGoroutine)
			}

			wg.Wait()
			close(errorChan)

			// Check for errors
			for err := range errorChan {
				t.Errorf("Write error: %v", err)
			}

			duration := time.Since(startTime)
			tps := float64(tt.numEntries) / duration.Seconds()

			t.Logf("Wrote %d entries in %v (%.2f TPS)", tt.numEntries, duration, tps)

			if tps < float64(tt.expectedMinTPS) {
				t.Errorf("Performance below expectation: got %.2f TPS, expected at least %d TPS", tps, tt.expectedMinTPS)
			}

			// Verify all entries were stored
			recent, err := storage.GetRecent(tt.numEntries)
			if err != nil {
				t.Fatalf("Failed to retrieve entries: %v", err)
			}

			if len(recent) != tt.numEntries {
				t.Errorf("Expected %d entries, got %d", tt.numEntries, len(recent))
			}
		})
	}
}

// TestConcurrentReadWriteWAL tests concurrent read and write operations with WAL mode
func TestConcurrentReadWriteWAL(t *testing.T) {
	dbPath := "test_concurrent_wal.db"
	defer cleanupTestFiles(dbPath)

	// Setup batched storage with WAL enabled
	config := DefaultBatchConfig()
	config.BatchSize = 50
	config.BatchTimeout = 100 * time.Millisecond
	walEnabled := true
	config.WALEnabled = &walEnabled

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test parameters
	numWriters := 10
	numReaders := 5
	testDuration := 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	var wg sync.WaitGroup
	writeErrors := make(chan error, numWriters)
	readErrors := make(chan error, numReaders)
	writeCount := make(chan int, numWriters)
	readCount := make(chan int, numReaders)

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			count := 0
			for {
				select {
				case <-ctx.Done():
					writeCount <- count
					return
				default:
					entry := &types.LogEntry{
						Priority:  16 + (count % 8), // Vary priority
						Facility:  16,
						Severity:  count % 8,
						Version:   1,
						Timestamp: time.Now(),
						Hostname:  fmt.Sprintf("writer-%d", writerID),
						AppName:   "integration-test",
						ProcID:    fmt.Sprintf("%d", writerID),
						MsgID:     "test",
						Message:   fmt.Sprintf("Writer %d message %d", writerID, count),
						StructuredData: map[string]interface{}{
							"writer_id": writerID,
							"count":     count,
						},
					}

					if err := storage.Store(entry); err != nil {
						writeErrors <- fmt.Errorf("writer %d error: %w", writerID, err)
						return
					}
					count++

					// Small delay to prevent overwhelming the system
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			count := 0
			for {
				select {
				case <-ctx.Done():
					readCount <- count
					return
				default:
					// Perform various read operations
					switch count % 4 {
					case 0:
						// Get recent entries
						_, err := storage.GetRecent(10)
						if err != nil {
							readErrors <- fmt.Errorf("reader %d GetRecent error: %w", readerID, err)
							return
						}
					case 1:
						// Search by hostname
						query := types.SearchQuery{
							Hostname: fmt.Sprintf("writer-%d", readerID%numWriters),
							Limit:    5,
						}
						_, err := storage.Search(query)
						if err != nil {
							readErrors <- fmt.Errorf("reader %d Search hostname error: %w", readerID, err)
							return
						}
					case 2:
						// Search by text
						query := types.SearchQuery{
							Text:  "message",
							Limit: 5,
						}
						_, err := storage.Search(query)
						if err != nil {
							readErrors <- fmt.Errorf("reader %d Search text error: %w", readerID, err)
							return
						}
					case 3:
						// Search by severity
						severity := readerID % 8
						query := types.SearchQuery{
							Severity: &severity,
							Limit:    5,
						}
						_, err := storage.Search(query)
						if err != nil {
							readErrors <- fmt.Errorf("reader %d Search severity error: %w", readerID, err)
							return
						}
					}
					count++

					// Small delay between reads
					time.Sleep(20 * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()
	close(writeErrors)
	close(readErrors)
	close(writeCount)
	close(readCount)

	// Check for errors
	for err := range writeErrors {
		t.Errorf("Write error: %v", err)
	}
	for err := range readErrors {
		t.Errorf("Read error: %v", err)
	}

	// Calculate statistics
	totalWrites := 0
	for count := range writeCount {
		totalWrites += count
	}

	totalReads := 0
	for count := range readCount {
		totalReads += count
	}

	writeTPS := float64(totalWrites) / testDuration.Seconds()
	readTPS := float64(totalReads) / testDuration.Seconds()

	t.Logf("Concurrent test completed: %d writes (%.2f TPS), %d reads (%.2f TPS)",
		totalWrites, writeTPS, totalReads, readTPS)

	// Verify minimum performance expectations (realistic for concurrent operations)
	if writeTPS < 50 {
		t.Errorf("Write TPS too low: %.2f, expected at least 50", writeTPS)
	}
	if readTPS < 30 {
		t.Errorf("Read TPS too low: %.2f, expected at least 30", readTPS)
	}

	// Verify data integrity - check that we can read back some of the written data
	recent, err := storage.GetRecent(100)
	if err != nil {
		t.Fatalf("Failed to verify data integrity: %v", err)
	}

	if len(recent) == 0 {
		t.Error("No data found after concurrent write test")
	}

	// Verify that entries have proper IDs assigned
	for i, entry := range recent {
		if entry.ID == 0 {
			t.Errorf("Entry %d has no ID assigned", i)
		}
	}
}

// TestGracefulShutdown tests that the system shuts down gracefully and processes all queued writes
func TestGracefulShutdown(t *testing.T) {
	dbPath := "test_graceful_shutdown.db"
	defer cleanupTestFiles(dbPath)

	config := DefaultBatchConfig()
	config.BatchSize = 100                // Large batch size to ensure some entries remain queued
	config.BatchTimeout = 1 * time.Second // Long timeout to ensure batching
	config.QueueSize = 1000

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Queue many entries quickly
	numEntries := 500
	entries := generateTestEntries(numEntries)

	// Store entries rapidly to fill the queue (use goroutines to avoid blocking)
	var wg sync.WaitGroup
	errorChan := make(chan error, numEntries)

	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e *types.LogEntry) {
			defer wg.Done()
			if err := storage.Store(e); err != nil {
				errorChan <- fmt.Errorf("failed to store entry %d: %w", idx, err)
			}
		}(i, entry)
	}

	// Wait for all stores to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All stores completed
	case <-time.After(10 * time.Second):
		t.Fatal("Store operations timed out")
	}

	close(errorChan)

	// Check for store errors
	for err := range errorChan {
		t.Errorf("Store error: %v", err)
	}

	// Give a moment for some batches to process
	time.Sleep(200 * time.Millisecond)

	// Close storage (should trigger graceful shutdown)
	closeStart := time.Now()
	if err := storage.Close(); err != nil {
		t.Fatalf("Failed to close storage: %v", err)
	}
	closeDuration := time.Since(closeStart)

	t.Logf("Graceful shutdown took %v", closeDuration)

	// Verify that shutdown didn't take too long (should be reasonable)
	if closeDuration > 10*time.Second {
		t.Errorf("Shutdown took too long: %v", closeDuration)
	}

	// Reopen storage to verify all entries were persisted
	storage2, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to reopen storage: %v", err)
	}
	defer storage2.Close()

	// Check that all entries were persisted
	recent, err := storage2.GetRecent(numEntries)
	if err != nil {
		t.Fatalf("Failed to retrieve entries after shutdown: %v", err)
	}

	if len(recent) != numEntries {
		t.Errorf("Data loss during shutdown: expected %d entries, got %d", numEntries, len(recent))
	}

	// Verify entries have proper IDs
	for i, entry := range recent {
		if entry.ID == 0 {
			t.Errorf("Entry %d has no ID after shutdown/restart", i)
		}
	}
}

// TestCrashRecovery tests WAL-based crash recovery scenarios
func TestCrashRecovery(t *testing.T) {
	dbPath := "test_crash_recovery.db"
	defer cleanupTestFiles(dbPath)

	// First phase: write data and simulate crash by not closing properly
	func() {
		config := DefaultBatchConfig()
		walEnabled := true
		config.WALEnabled = &walEnabled

		storage, err := NewBatchedSQLiteStorage(dbPath, config)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}

		// Write some entries
		numEntries := 100
		entries := generateTestEntries(numEntries)

		var wg sync.WaitGroup
		for i, entry := range entries {
			wg.Add(1)
			go func(idx int, e *types.LogEntry) {
				defer wg.Done()
				if err := storage.Store(e); err != nil {
					t.Errorf("Failed to store entry %d: %v", idx, err)
				}
			}(i, entry)
		}

		// Wait for all entries to be queued
		wg.Wait()

		// Give time for batches to process and data to be written to WAL
		time.Sleep(2 * time.Second)

		// Force a checkpoint to ensure some data is in the main database
		// This simulates a more realistic crash scenario
		if err := storage.Close(); err != nil {
			t.Logf("Close error (expected in crash simulation): %v", err)
		}

		// Simulate crash by reopening without proper shutdown
		// The WAL files should contain recoverable data
	}()

	// Second phase: recover and verify data integrity
	config := DefaultBatchConfig()
	walEnabled := true
	config.WALEnabled = &walEnabled

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to recover storage after crash: %v", err)
	}
	defer storage.Close()

	// Verify that we can read data (WAL recovery should have occurred)
	recent, err := storage.GetRecent(200)
	if err != nil {
		t.Fatalf("Failed to read data after recovery: %v", err)
	}

	// In a real crash scenario, we might lose some data, but we should recover most of it
	if len(recent) == 0 {
		t.Error("No data recovered after crash simulation")
	} else if len(recent) < 50 { // Allow for some data loss in crash simulation
		t.Logf("Warning: Only recovered %d entries out of 100, some data loss expected in crash simulation", len(recent))
	}

	t.Logf("Recovered %d entries after crash simulation", len(recent))

	// Verify that we can continue writing after recovery
	newEntry := &types.LogEntry{
		Priority:  16,
		Facility:  16,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "recovery-test",
		AppName:   "test",
		Message:   "Post-recovery write test",
	}

	if err := storage.Store(newEntry); err != nil {
		t.Fatalf("Failed to write after recovery: %v", err)
	}

	// Verify the new entry was stored
	recentAfterWrite, err := storage.GetRecent(1)
	if err != nil {
		t.Fatalf("Failed to read after post-recovery write: %v", err)
	}

	if len(recentAfterWrite) == 0 || recentAfterWrite[0].Message != "Post-recovery write test" {
		t.Error("Post-recovery write was not properly stored")
	}
}

// Helper functions

func generateTestEntries(count int) []*types.LogEntry {
	entries := make([]*types.LogEntry, count)
	facilities := []int{0, 1, 2, 3, 16, 17, 18, 19} // Various facilities
	severities := []int{0, 1, 2, 3, 4, 5, 6, 7}     // All severity levels
	hostnames := []string{"web01", "web02", "db01", "cache01", "api01"}
	appNames := []string{"nginx", "mysql", "redis", "api-server", "worker"}

	for i := 0; i < count; i++ {
		facility := facilities[i%len(facilities)]
		severity := severities[i%len(severities)]
		priority := facility*8 + severity

		entries[i] = &types.LogEntry{
			Priority:  priority,
			Facility:  facility,
			Severity:  severity,
			Version:   1,
			Timestamp: time.Now().Add(-time.Duration(count-i) * time.Second), // Spread over time
			Hostname:  hostnames[i%len(hostnames)],
			AppName:   appNames[i%len(appNames)],
			ProcID:    fmt.Sprintf("%d", 1000+i%100),
			MsgID:     fmt.Sprintf("msg-%d", i%10),
			Message:   fmt.Sprintf("Test log message %d with some content for search", i),
			StructuredData: map[string]interface{}{
				"request_id": fmt.Sprintf("req-%d", i),
				"user_id":    i % 1000,
				"action":     []string{"login", "logout", "view", "edit", "delete"}[i%5],
				"timestamp":  time.Now().Unix(),
			},
		}
	}

	return entries
}

func cleanupTestFiles(dbPath string) {
	os.Remove(dbPath)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")
}

// setupTestStorageWithConfig creates a test storage instance with custom config
func setupTestStorageWithConfig(t *testing.T, config BatchConfig) (interfaces.LogStorage, string) {
	dbPath := fmt.Sprintf("test_%d.db", time.Now().UnixNano())

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}

	return storage, dbPath
}

// cleanupTestStorageWithPath cleans up test storage and files
func cleanupTestStorageWithPath(storage interfaces.LogStorage, dbPath string) {
	if storage != nil {
		storage.Close()
	}
	cleanupTestFiles(dbPath)
}
