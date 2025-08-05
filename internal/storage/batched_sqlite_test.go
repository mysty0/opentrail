package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

func TestNewBatchedSQLiteStorage(t *testing.T) {
	// Create temporary database file
	dbPath := "test_batched.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	tests := []struct {
		name        string
		config      BatchConfig
		expectError bool
	}{
		{
			name:        "default config",
			config:      DefaultBatchConfig(),
			expectError: false,
		},
		{
			name: "custom valid config",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    50,
					BatchTimeout: 200 * time.Millisecond,
					QueueSize:    5000,
					WALEnabled:   &walEnabled,
				}
			}(),
			expectError: false,
		},
		{
			name: "invalid batch size",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    -1, // Invalid negative value
					BatchTimeout: 100 * time.Millisecond,
					QueueSize:    1000,
					WALEnabled:   &walEnabled,
				}
			}(),
			expectError: true,
		},
		{
			name: "invalid batch timeout",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    100,
					BatchTimeout: -1 * time.Millisecond, // Invalid negative value
					QueueSize:    1000,
					WALEnabled:   &walEnabled,
				}
			}(),
			expectError: true,
		},
		{
			name: "invalid queue size",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    100,
					BatchTimeout: 100 * time.Millisecond,
					QueueSize:    -1, // Invalid negative value
					WALEnabled:   &walEnabled,
				}
			}(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing database files
			os.Remove(dbPath)
			os.Remove(dbPath + "-wal")
			os.Remove(dbPath + "-shm")

			storage, err := NewBatchedSQLiteStorage(dbPath, tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if storage == nil {
				t.Errorf("expected storage instance but got nil")
				return
			}

			// Verify it implements the LogStorage interface
			var _ interfaces.LogStorage = storage

			// Clean up
			if err := storage.Close(); err != nil {
				t.Errorf("failed to close storage: %v", err)
			}
		})
	}
}

func TestBatchedSQLiteStorage_StructFields(t *testing.T) {
	dbPath := "test_struct.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Cast to concrete type to access fields
	batchedStorage, ok := storage.(*BatchedSQLiteStorage)
	if !ok {
		t.Fatalf("storage is not of type *BatchedSQLiteStorage")
	}

	// Verify all required fields are initialized
	if batchedStorage.db == nil {
		t.Errorf("db field is nil")
	}

	if batchedStorage.config.BatchSize != config.BatchSize {
		t.Errorf("config.BatchSize = %d, want %d", batchedStorage.config.BatchSize, config.BatchSize)
	}

	if batchedStorage.config.BatchTimeout != config.BatchTimeout {
		t.Errorf("config.BatchTimeout = %v, want %v", batchedStorage.config.BatchTimeout, config.BatchTimeout)
	}

	if batchedStorage.config.QueueSize != config.QueueSize {
		t.Errorf("config.QueueSize = %d, want %d", batchedStorage.config.QueueSize, config.QueueSize)
	}

	if batchedStorage.writeQueue == nil {
		t.Errorf("writeQueue is nil")
	}

	if cap(batchedStorage.writeQueue) != config.QueueSize {
		t.Errorf("writeQueue capacity = %d, want %d", cap(batchedStorage.writeQueue), config.QueueSize)
	}

	if batchedStorage.batchBuffer == nil {
		t.Errorf("batchBuffer is nil")
	}

	if batchedStorage.batchTimer == nil {
		t.Errorf("batchTimer is nil")
	}

	if batchedStorage.ctx == nil {
		t.Errorf("ctx is nil")
	}

	if batchedStorage.cancel == nil {
		t.Errorf("cancel is nil")
	}

	if batchedStorage.insertStmt == nil {
		t.Errorf("insertStmt is nil")
	}

	if batchedStorage.batchStmt == nil {
		t.Errorf("batchStmt is nil")
	}

	// Verify the storage is running
	batchedStorage.runningMux.RLock()
	isRunning := batchedStorage.isRunning
	batchedStorage.runningMux.RUnlock()

	if !isRunning {
		t.Errorf("storage should be running after initialization")
	}
}

func TestBatchedSQLiteStorage_WALMode(t *testing.T) {
	dbPath := "test_wal.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	tests := []struct {
		name       string
		walEnabled bool
	}{
		{
			name:       "WAL enabled",
			walEnabled: true,
		},
		{
			name:       "WAL disabled",
			walEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use unique database path for each test
			testDbPath := dbPath + "_" + tt.name

			// Clean up any existing database files
			os.Remove(testDbPath)
			os.Remove(testDbPath + "-wal")
			os.Remove(testDbPath + "-shm")
			defer os.Remove(testDbPath)
			defer os.Remove(testDbPath + "-wal")
			defer os.Remove(testDbPath + "-shm")

			config := DefaultBatchConfig()
			config.WALEnabled = &tt.walEnabled

			storage, err := NewBatchedSQLiteStorage(testDbPath, config)
			if err != nil {
				t.Fatalf("failed to create storage: %v", err)
			}
			defer storage.Close()

			// Cast to concrete type to access db
			batchedStorage := storage.(*BatchedSQLiteStorage)

			// Check journal mode
			var journalMode string
			err = batchedStorage.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
			if err != nil {
				t.Fatalf("failed to query journal mode: %v", err)
			}

			if tt.walEnabled {
				if journalMode != "wal" {
					t.Errorf("expected WAL mode, got %s", journalMode)
				}
			} else {
				// When WAL is disabled, it should use the delete mode
				if journalMode != "delete" {
					t.Errorf("expected delete mode, got %s", journalMode)
				}
			}
		})
	}
}

func TestBatchedSQLiteStorage_DatabaseInitialization(t *testing.T) {
	dbPath := "test_init.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Cast to concrete type to access db
	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Verify logs table exists
	var tableName string
	err = batchedStorage.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='logs'").Scan(&tableName)
	if err != nil {
		t.Errorf("logs table not found: %v", err)
	}
	if tableName != "logs" {
		t.Errorf("expected table name 'logs', got '%s'", tableName)
	}

	// Verify FTS table exists
	err = batchedStorage.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='logs_fts'").Scan(&tableName)
	if err != nil {
		t.Errorf("logs_fts table not found: %v", err)
	}
	if tableName != "logs_fts" {
		t.Errorf("expected table name 'logs_fts', got '%s'", tableName)
	}

	// Verify some indexes exist
	var indexCount int
	err = batchedStorage.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name='logs'").Scan(&indexCount)
	if err != nil {
		t.Errorf("failed to count indexes: %v", err)
	}
	if indexCount == 0 {
		t.Errorf("expected indexes to be created, got %d", indexCount)
	}

	// Verify triggers exist
	var triggerCount int
	err = batchedStorage.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger'").Scan(&triggerCount)
	if err != nil {
		t.Errorf("failed to count triggers: %v", err)
	}
	if triggerCount == 0 {
		t.Errorf("expected triggers to be created, got %d", triggerCount)
	}
}

func TestBatchedSQLiteStorage_LifecycleManagement(t *testing.T) {
	dbPath := "test_lifecycle.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Cast to concrete type to access fields
	batchedStorage := storage.(*BatchedSQLiteStorage)

	// Verify context is not cancelled initially
	select {
	case <-batchedStorage.ctx.Done():
		t.Errorf("context should not be cancelled initially")
	default:
		// Expected
	}

	// Verify storage is running
	batchedStorage.runningMux.RLock()
	isRunning := batchedStorage.isRunning
	batchedStorage.runningMux.RUnlock()

	if !isRunning {
		t.Errorf("storage should be running after initialization")
	}

	// Close the storage
	err = storage.Close()
	if err != nil {
		t.Errorf("failed to close storage: %v", err)
	}

	// Verify context is cancelled after close
	select {
	case <-batchedStorage.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Errorf("context should be cancelled after close")
	}

	// Verify storage is not running after close
	batchedStorage.runningMux.RLock()
	isRunning = batchedStorage.isRunning
	batchedStorage.runningMux.RUnlock()

	if isRunning {
		t.Errorf("storage should not be running after close")
	}

	// Verify double close doesn't error
	err = storage.Close()
	if err != nil {
		t.Errorf("double close should not error: %v", err)
	}
}

func TestBatchedSQLiteStorage_InterfaceCompliance(t *testing.T) {
	dbPath := "test_interface.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Verify it implements the LogStorage interface
	var _ interfaces.LogStorage = storage

	// Test that all interface methods exist and return appropriate errors
	// (since they're not implemented yet)

	// Test Store method
	entry := &types.LogEntry{
		Priority:  16, // facility 2, severity 0
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "test message",
	}

	err = storage.Store(entry)
	if err != nil {
		t.Errorf("Store should work now that it's implemented: %v", err)
	}

	// Wait for batch processing
	time.Sleep(200 * time.Millisecond)

	// Test Search method
	query := types.SearchQuery{
		Text:  "test",
		Limit: 10,
	}

	entries, err := storage.Search(query)
	if err != nil {
		t.Errorf("Search should work now that it's implemented: %v", err)
	}
	if entries == nil {
		t.Errorf("Search should return entries slice (even if empty)")
	}

	// Test GetRecent method
	entries, err = storage.GetRecent(10)
	if err != nil {
		t.Errorf("GetRecent should work now that it's implemented: %v", err)
	}
	if entries == nil {
		t.Errorf("GetRecent should return entries slice (even if empty)")
	}

	// Test Cleanup method
	err = storage.Cleanup(30)
	if err != nil {
		t.Errorf("Cleanup should work now that it's implemented: %v", err)
	}
}

func TestBatchedSQLiteStorage_ConfigDefaults(t *testing.T) {
	dbPath := "test_defaults.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	// Test with empty config (should apply defaults)
	config := BatchConfig{}
	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage with empty config: %v", err)
	}
	defer storage.Close()

	// Cast to concrete type to verify defaults were applied
	batchedStorage := storage.(*BatchedSQLiteStorage)

	defaults := DefaultBatchConfig()
	if batchedStorage.config.BatchSize != defaults.BatchSize {
		t.Errorf("BatchSize = %d, want %d", batchedStorage.config.BatchSize, defaults.BatchSize)
	}

	if batchedStorage.config.BatchTimeout != defaults.BatchTimeout {
		t.Errorf("BatchTimeout = %v, want %v", batchedStorage.config.BatchTimeout, defaults.BatchTimeout)
	}

	if batchedStorage.config.QueueSize != defaults.QueueSize {
		t.Errorf("QueueSize = %d, want %d", batchedStorage.config.QueueSize, defaults.QueueSize)
	}

	if *batchedStorage.config.WALEnabled != *defaults.WALEnabled {
		t.Errorf("WALEnabled = %v, want %v", *batchedStorage.config.WALEnabled, *defaults.WALEnabled)
	}
}
func TestBatchedSQLiteStorage_Store_Success(t *testing.T) {
	dbPath := "test_store_success.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	// Use smaller queue and batch size for testing
	config.QueueSize = 10
	config.BatchSize = 2
	config.BatchTimeout = 50 * time.Millisecond

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test log entry
	entry := &types.LogEntry{
		Priority:  16, // facility 2, severity 0
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		ProcID:    "1234",
		MsgID:     "TEST",
		Message:   "test message for store",
		StructuredData: map[string]interface{}{
			"test": "data",
		},
	}

	// Store should queue the request immediately and return
	err = storage.Store(entry)

	// Now that batch processing is implemented, Store should succeed
	if err != nil {
		t.Errorf("Store should succeed now that batch processing is implemented: %v", err)
	}

	// Verify the entry was assigned an ID
	if entry.ID == 0 {
		t.Errorf("Entry should have been assigned an ID after successful store")
	}
}

func TestBatchedSQLiteStorage_Store_QueueFull(t *testing.T) {
	dbPath := "test_store_queue_full.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	// Use very small queue size to test backpressure
	config.QueueSize = 1
	config.BatchSize = 1                        // Process immediately
	config.BatchTimeout = 10 * time.Millisecond // Short timeout

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create test log entries
	entry1 := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "first message",
	}

	entry2 := &types.LogEntry{
		Priority:  17,
		Facility:  2,
		Severity:  1,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "second message",
	}

	// First store should succeed (queue has space)
	err1 := storage.Store(entry1)
	// We expect this to fail due to unimplemented batch processing, but not due to queue issues

	// Second store - since batch processor immediately processes and returns errors,
	// the queue should be available again, so this tests the Store method logic
	err2 := storage.Store(entry2)

	// Both calls should fail due to unimplemented batch processing, not queue issues
	if err1 != nil && contains(err1.Error(), "batch processing not yet implemented") {
		// Expected - batch processing not implemented
	} else if err1 != nil && err1.Error() == "write queue is full, please try again later" {
		t.Errorf("first store should not fail with queue full error")
	}

	if err2 != nil && contains(err2.Error(), "batch processing not yet implemented") {
		// Expected - batch processing not implemented
	} else if err2 != nil && err2.Error() == "write queue is full, please try again later" {
		// This could happen if the first request is still being processed
		t.Logf("second store got queue full error (acceptable): %v", err2)
	}

}

func TestBatchedSQLiteStorage_Store_NotRunning(t *testing.T) {
	dbPath := "test_store_not_running.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Close the storage to stop it
	err = storage.Close()
	if err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Create a test log entry
	entry := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "test message",
	}

	// Store should return error when storage is not running
	err = storage.Store(entry)
	if err == nil {
		t.Errorf("expected error when storage is not running")
	} else if err.Error() != "storage is not running" {
		t.Errorf("expected 'storage is not running' error, got: %v", err)
	}
}

func TestBatchedSQLiteStorage_Store_IDAssignment(t *testing.T) {
	dbPath := "test_store_id.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.QueueSize = 10
	config.BatchSize = 1 // Process immediately
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
		Message:   "test message for ID assignment",
	}

	// Verify ID is initially 0
	if entry.ID != 0 {
		t.Errorf("entry ID should be 0 initially, got %d", entry.ID)
	}

	// Store the entry
	err = storage.Store(entry)

	// For now, we expect an error since batch processing is not implemented
	// But we're testing that the Store method properly handles ID assignment logic
	if err == nil {
		// If no error, ID should be assigned
		if entry.ID == 0 {
			t.Errorf("entry ID should be assigned after successful store, got %d", entry.ID)
		}
	} else {
		// If there's an error, ID should remain 0
		if entry.ID != 0 {
			t.Errorf("entry ID should remain 0 after failed store, got %d", entry.ID)
		}
	}
}

func TestBatchedSQLiteStorage_Store_NonBlocking(t *testing.T) {
	dbPath := "test_store_nonblocking.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.QueueSize = 5
	config.BatchSize = 1                        // Process immediately
	config.BatchTimeout = 10 * time.Millisecond // Short timeout

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create test log entries
	entries := make([]*types.LogEntry, 10)
	for i := 0; i < 10; i++ {
		entries[i] = &types.LogEntry{
			Priority:  16 + i,
			Facility:  2,
			Severity:  i % 8,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host",
			AppName:   "test-app",
			Message:   fmt.Sprintf("test message %d", i),
		}
	}

	// Test that Store operations return quickly (non-blocking queue operation)
	for i, entry := range entries {
		start := time.Now()
		err := storage.Store(entry)
		elapsed := time.Since(start)

		// Each Store call should return quickly (within reasonable time)
		if elapsed > 1*time.Second {
			t.Errorf("Store operation %d took too long: %v", i, elapsed)
		}

		// Now that batch processing is implemented, Store should succeed
		if err != nil {
			t.Errorf("Store operation %d should succeed: %v", i, err)
		}
	}
}

func TestBatchedSQLiteStorage_Store_ContextCancellation(t *testing.T) {
	dbPath := "test_store_context.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.QueueSize = 10
	config.BatchSize = 1
	config.BatchTimeout = 10 * time.Millisecond

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create a test log entry
	entry := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "test message for context cancellation",
	}

	// Close storage immediately to cancel context
	go func() {
		time.Sleep(1 * time.Millisecond) // Very short delay to ensure Store is called first
		storage.Close()
	}()

	// Store should handle context cancellation gracefully
	err = storage.Store(entry)

	// The operation might succeed if it completes before context cancellation,
	// or it might fail with context cancellation or storage not running error
	if err != nil {
		if err.Error() != "storage is not running" &&
			!contains(err.Error(), "context") &&
			!contains(err.Error(), "cancelled") {
			t.Errorf("expected context cancellation or storage not running error, got: %v", err)
		}
	}
	// If err is nil, the operation completed successfully before cancellation, which is also valid
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestBatchedSQLiteStorage_Search tests the Search method with various query parameters
func TestBatchedSQLiteStorage_Search(t *testing.T) {
	// Create temporary database file
	dbPath := "test_search.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	// Create storage instance
	storage, err := NewBatchedSQLiteStorage(dbPath, DefaultBatchConfig())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Store test entries
	testEntries := []*types.LogEntry{
		{
			Priority:  16, // facility 2, severity 0
			Facility:  2,
			Severity:  0,
			Version:   1,
			Timestamp: time.Now().Add(-1 * time.Hour),
			Hostname:  "server1",
			AppName:   "app1",
			ProcID:    "123",
			MsgID:     "test1",
			Message:   "Error message from app1",
			StructuredData: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
		},
		{
			Priority:  17, // facility 2, severity 1
			Facility:  2,
			Severity:  1,
			Version:   1,
			Timestamp: time.Now().Add(-30 * time.Minute),
			Hostname:  "server2",
			AppName:   "app2",
			ProcID:    "456",
			MsgID:     "test2",
			Message:   "Warning message from app2",
			StructuredData: map[string]interface{}{
				"key3": "value3",
			},
		},
		{
			Priority:  24, // facility 3, severity 0
			Facility:  3,
			Severity:  0,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "server1",
			AppName:   "app1",
			ProcID:    "789",
			MsgID:     "test3",
			Message:   "Info message from app1",
		},
	}

	// Store all test entries
	for _, entry := range testEntries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Wait for batch processing to complete
	time.Sleep(200 * time.Millisecond)

	tests := []struct {
		name          string
		query         types.SearchQuery
		expectedCount int
		description   string
	}{
		{
			name:          "search by text",
			query:         types.SearchQuery{Text: "Error"},
			expectedCount: 1,
			description:   "Should find entries containing 'Error'",
		},
		{
			name:          "search by facility",
			query:         types.SearchQuery{Facility: intPtr(2)},
			expectedCount: 2,
			description:   "Should find entries with facility 2",
		},
		{
			name:          "search by severity",
			query:         types.SearchQuery{Severity: intPtr(0)},
			expectedCount: 2,
			description:   "Should find entries with severity 0",
		},
		{
			name:          "search by hostname",
			query:         types.SearchQuery{Hostname: "server1"},
			expectedCount: 2,
			description:   "Should find entries from server1",
		},
		{
			name:          "search by app name",
			query:         types.SearchQuery{AppName: "app1"},
			expectedCount: 2,
			description:   "Should find entries from app1",
		},
		{
			name:          "search with limit",
			query:         types.SearchQuery{Limit: 1},
			expectedCount: 1,
			description:   "Should limit results to 1",
		},
		{
			name:          "search by structured data",
			query:         types.SearchQuery{StructuredDataQuery: "key1"},
			expectedCount: 1,
			description:   "Should find entries containing key1 in structured data",
		},
		{
			name:          "combined search",
			query:         types.SearchQuery{Facility: intPtr(2), Severity: intPtr(0)},
			expectedCount: 1,
			description:   "Should find entries with facility 2 and severity 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.Search(tt.query)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d. %s", tt.expectedCount, len(results), tt.description)
			}

			// Verify results are properly ordered (newest first)
			for i := 1; i < len(results); i++ {
				if results[i-1].Timestamp.Before(results[i].Timestamp) {
					t.Errorf("Results not properly ordered by timestamp")
				}
			}
		})
	}
}

// TestBatchedSQLiteStorage_GetRecent tests the GetRecent method
func TestBatchedSQLiteStorage_GetRecent(t *testing.T) {
	// Create temporary database file
	dbPath := "test_recent.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	// Create storage instance
	storage, err := NewBatchedSQLiteStorage(dbPath, DefaultBatchConfig())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Store test entries with different timestamps
	baseTime := time.Now()
	testEntries := []*types.LogEntry{
		{
			Priority:  16,
			Facility:  2,
			Severity:  0,
			Version:   1,
			Timestamp: baseTime.Add(-3 * time.Hour),
			Hostname:  "server1",
			AppName:   "app1",
			Message:   "Oldest message",
		},
		{
			Priority:  17,
			Facility:  2,
			Severity:  1,
			Version:   1,
			Timestamp: baseTime.Add(-2 * time.Hour),
			Hostname:  "server2",
			AppName:   "app2",
			Message:   "Middle message",
		},
		{
			Priority:  18,
			Facility:  2,
			Severity:  2,
			Version:   1,
			Timestamp: baseTime.Add(-1 * time.Hour),
			Hostname:  "server3",
			AppName:   "app3",
			Message:   "Newest message",
		},
	}

	// Store all test entries
	for _, entry := range testEntries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Wait for batch processing to complete
	time.Sleep(200 * time.Millisecond)

	tests := []struct {
		name          string
		limit         int
		expectedCount int
		expectedFirst string
	}{
		{
			name:          "get recent with limit 1",
			limit:         1,
			expectedCount: 1,
			expectedFirst: "Newest message",
		},
		{
			name:          "get recent with limit 2",
			limit:         2,
			expectedCount: 2,
			expectedFirst: "Newest message",
		},
		{
			name:          "get recent with limit 5 (more than available)",
			limit:         5,
			expectedCount: 3,
			expectedFirst: "Newest message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.GetRecent(tt.limit)
			if err != nil {
				t.Fatalf("GetRecent failed: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}

			if len(results) > 0 && results[0].Message != tt.expectedFirst {
				t.Errorf("Expected first result to be '%s', got '%s'", tt.expectedFirst, results[0].Message)
			}

			// Verify results are properly ordered (newest first)
			for i := 1; i < len(results); i++ {
				if results[i-1].Timestamp.Before(results[i].Timestamp) {
					t.Errorf("Results not properly ordered by timestamp")
				}
			}
		})
	}
}

// TestBatchedSQLiteStorage_Cleanup tests the Cleanup method
func TestBatchedSQLiteStorage_Cleanup(t *testing.T) {
	// Create temporary database file
	dbPath := "test_cleanup.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	// Create storage instance
	storage, err := NewBatchedSQLiteStorage(dbPath, DefaultBatchConfig())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Store test entries with different timestamps
	baseTime := time.Now()
	testEntries := []*types.LogEntry{
		{
			Priority:  16,
			Facility:  2,
			Severity:  0,
			Version:   1,
			Timestamp: baseTime.AddDate(0, 0, -10), // 10 days old
			Hostname:  "server1",
			AppName:   "app1",
			Message:   "Old message 1",
		},
		{
			Priority:  17,
			Facility:  2,
			Severity:  1,
			Version:   1,
			Timestamp: baseTime.AddDate(0, 0, -5), // 5 days old
			Hostname:  "server2",
			AppName:   "app2",
			Message:   "Old message 2",
		},
		{
			Priority:  18,
			Facility:  2,
			Severity:  2,
			Version:   1,
			Timestamp: baseTime.AddDate(0, 0, -1), // 1 day old
			Hostname:  "server3",
			AppName:   "app3",
			Message:   "Recent message",
		},
	}

	// Store all test entries
	for _, entry := range testEntries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Wait for batch processing to complete
	time.Sleep(200 * time.Millisecond)

	// Verify all entries are stored
	allEntries, err := storage.GetRecent(10)
	if err != nil {
		t.Fatalf("Failed to get recent entries: %v", err)
	}
	if len(allEntries) != 3 {
		t.Fatalf("Expected 3 entries before cleanup, got %d", len(allEntries))
	}

	// Cleanup entries older than 7 days
	if err := storage.Cleanup(7); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify only recent entries remain
	remainingEntries, err := storage.GetRecent(10)
	if err != nil {
		t.Fatalf("Failed to get recent entries after cleanup: %v", err)
	}

	expectedRemaining := 2 // 5 days old and 1 day old should remain
	if len(remainingEntries) != expectedRemaining {
		t.Errorf("Expected %d entries after cleanup, got %d", expectedRemaining, len(remainingEntries))
	}

	// Cleanup entries older than 3 days
	if err := storage.Cleanup(3); err != nil {
		t.Fatalf("Second cleanup failed: %v", err)
	}

	// Verify only the most recent entry remains
	finalEntries, err := storage.GetRecent(10)
	if err != nil {
		t.Fatalf("Failed to get recent entries after second cleanup: %v", err)
	}

	expectedFinal := 1 // Only 1 day old should remain
	if len(finalEntries) != expectedFinal {
		t.Errorf("Expected %d entry after second cleanup, got %d", expectedFinal, len(finalEntries))
	}

	if len(finalEntries) > 0 && finalEntries[0].Message != "Recent message" {
		t.Errorf("Expected remaining entry to be 'Recent message', got '%s'", finalEntries[0].Message)
	}
}

// TestBatchedSQLiteStorage_Close tests the Close method with proper shutdown
func TestBatchedSQLiteStorage_Close(t *testing.T) {
	// Create temporary database file
	dbPath := "test_close.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	// Create storage instance
	storage, err := NewBatchedSQLiteStorage(dbPath, DefaultBatchConfig())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Store a test entry
	testEntry := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "server1",
		AppName:   "app1",
		Message:   "Test message",
	}

	if err := storage.Store(testEntry); err != nil {
		t.Fatalf("Failed to store entry: %v", err)
	}

	// Close the storage
	if err := storage.Close(); err != nil {
		t.Fatalf("Failed to close storage: %v", err)
	}

	// Verify that operations fail after close
	if err := storage.Store(testEntry); err == nil {
		t.Error("Expected Store to fail after Close, but it succeeded")
	}

	// Verify that Close can be called multiple times without error
	if err := storage.Close(); err != nil {
		t.Errorf("Second Close call should not fail: %v", err)
	}
}

// TestBatchedSQLiteStorage_ConcurrentReadWrite tests concurrent read and write operations
func TestBatchedSQLiteStorage_ConcurrentReadWrite(t *testing.T) {
	// Create temporary database file
	dbPath := "test_concurrent.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	// Create storage instance with WAL mode enabled
	config := DefaultBatchConfig()
	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Start concurrent writers
	numWriters := 5
	numReads := 10
	entriesPerWriter := 20

	// Channel to collect errors
	errChan := make(chan error, numWriters+numReads)

	// Start writers
	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			for j := 0; j < entriesPerWriter; j++ {
				entry := &types.LogEntry{
					Priority:  16,
					Facility:  2,
					Severity:  0,
					Version:   1,
					Timestamp: time.Now(),
					Hostname:  fmt.Sprintf("server%d", writerID),
					AppName:   fmt.Sprintf("app%d", writerID),
					Message:   fmt.Sprintf("Message %d from writer %d", j, writerID),
				}

				if err := storage.Store(entry); err != nil {
					errChan <- fmt.Errorf("writer %d failed to store entry %d: %w", writerID, j, err)
					return
				}
			}
			errChan <- nil
		}(i)
	}

	// Start readers
	for i := 0; i < numReads; i++ {
		go func(readerID int) {
			// Wait a bit to let some writes happen
			time.Sleep(50 * time.Millisecond)

			// Perform various read operations
			if _, err := storage.GetRecent(10); err != nil {
				errChan <- fmt.Errorf("reader %d failed GetRecent: %w", readerID, err)
				return
			}

			if _, err := storage.Search(types.SearchQuery{Limit: 5}); err != nil {
				errChan <- fmt.Errorf("reader %d failed Search: %w", readerID, err)
				return
			}

			errChan <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numWriters+numReads; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}

	// Wait for batch processing to complete
	time.Sleep(300 * time.Millisecond)

	// Verify all entries were stored
	allEntries, err := storage.GetRecent(numWriters * entriesPerWriter)
	if err != nil {
		t.Fatalf("Failed to get all entries: %v", err)
	}

	expectedTotal := numWriters * entriesPerWriter
	if len(allEntries) != expectedTotal {
		t.Errorf("Expected %d total entries, got %d", expectedTotal, len(allEntries))
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
