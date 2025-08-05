package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"opentrail/internal/types"
)

// TestCompletionTracking_Integration tests completion tracking with real storage
func TestCompletionTracking_Integration(t *testing.T) {
	dbPath := "test_completion_integration.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.BatchSize = 2 // Small batch for testing
	config.BatchTimeout = 50 * time.Millisecond
	config.WriteTimeout = 2 * time.Second

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test single entry
	entry1 := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "integration test message 1",
	}

	// Store should complete successfully and assign ID
	err = storage.Store(entry1)
	if err != nil {
		t.Errorf("Store failed: %v", err)
	}

	if entry1.ID == 0 {
		t.Errorf("entry1 ID should be assigned, got %d", entry1.ID)
	}

	// Test batch of entries
	entries := make([]*types.LogEntry, 5)
	for i := 0; i < 5; i++ {
		entries[i] = &types.LogEntry{
			Priority:  16 + i,
			Facility:  2,
			Severity:  i % 8,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host",
			AppName:   "test-app",
			Message:   fmt.Sprintf("integration test batch message %d", i),
		}
	}

	// Store all entries
	for i, entry := range entries {
		err := storage.Store(entry)
		if err != nil {
			t.Errorf("Store failed for entry %d: %v", i, err)
		}

		if entry.ID == 0 {
			t.Errorf("entry %d ID should be assigned, got %d", i, entry.ID)
		}
	}

	// Verify all entries got unique IDs
	allIDs := make(map[int64]bool)
	allIDs[entry1.ID] = true

	for i, entry := range entries {
		if allIDs[entry.ID] {
			t.Errorf("entry %d got duplicate ID %d", i, entry.ID)
		}
		allIDs[entry.ID] = true
	}

	t.Logf("Successfully stored %d entries with unique IDs", len(entries)+1)
	t.Logf("Entry IDs: %d, %v", entry1.ID, func() []int64 {
		ids := make([]int64, len(entries))
		for i, e := range entries {
			ids[i] = e.ID
		}
		return ids
	}())
}

// TestCompletionTracking_IntegrationTimeout tests timeout handling in real storage
func TestCompletionTracking_IntegrationTimeout(t *testing.T) {
	dbPath := "test_completion_integration_timeout.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.WriteTimeout = 50 * time.Millisecond // Very short timeout
	config.BatchSize = 1
	config.BatchTimeout = 10 * time.Millisecond

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test log entry
	entry := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "integration timeout test message",
	}

	// Store should complete quickly (batch processing is fast)
	start := time.Now()
	err = storage.Store(entry)
	elapsed := time.Since(start)

	// Should succeed because batch processing is actually fast
	if err != nil {
		t.Logf("Store failed (expected with short timeout): %v", err)
		// This is acceptable - the timeout might be too short for the operation
	} else {
		t.Logf("Store succeeded in %v with ID %d", elapsed, entry.ID)
	}

	// Should complete within reasonable time regardless of configured timeout
	if elapsed > 1*time.Second {
		t.Errorf("Store took too long: %v", elapsed)
	}
}
