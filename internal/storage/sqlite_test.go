package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"opentrail/internal/types"
)

func TestNewSQLiteStorage(t *testing.T) {
	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	storage, err := NewSQLiteStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}
	defer storage.(*SQLiteStorage).Close()

	// Verify that the storage implements the interface
	if storage == nil {
		t.Fatal("Storage should not be nil")
	}
}

func TestSQLiteStorage_Store(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	entry := &types.LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		TrackingID: "test-123",
		Message:    "Test log message",
	}

	err := storage.Store(entry)
	if err != nil {
		t.Fatalf("Failed to store log entry: %v", err)
	}

	// Verify that ID was set
	if entry.ID == 0 {
		t.Error("Expected ID to be set after storing")
	}
}

func TestSQLiteStorage_GetRecent(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store test entries
	entries := []*types.LogEntry{
		{
			Timestamp:  time.Now().Add(-2 * time.Hour),
			Level:      "INFO",
			TrackingID: "test-1",
			Message:    "First message",
		},
		{
			Timestamp:  time.Now().Add(-1 * time.Hour),
			Level:      "WARN",
			TrackingID: "test-2",
			Message:    "Second message",
		},
		{
			Timestamp:  time.Now(),
			Level:      "ERROR",
			TrackingID: "test-3",
			Message:    "Third message",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test GetRecent
	recent, err := storage.GetRecent(2)
	if err != nil {
		t.Fatalf("Failed to get recent logs: %v", err)
	}

	if len(recent) != 2 {
		t.Errorf("Expected 2 recent entries, got %d", len(recent))
	}

	// Should be ordered by timestamp DESC (most recent first)
	if recent[0].Message != "Third message" {
		t.Errorf("Expected most recent message first, got: %s", recent[0].Message)
	}
}

func TestSQLiteStorage_Search_FullText(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store test entries
	entries := []*types.LogEntry{
		{
			Timestamp:  time.Now(),
			Level:      "INFO",
			TrackingID: "test-1",
			Message:    "Database connection established",
		},
		{
			Timestamp:  time.Now(),
			Level:      "ERROR",
			TrackingID: "test-2",
			Message:    "Failed to connect to database",
		},
		{
			Timestamp:  time.Now(),
			Level:      "INFO",
			TrackingID: "test-3",
			Message:    "User authentication successful",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test full-text search
	query := types.SearchQuery{
		Text: "database",
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'database' search, got %d", len(results))
	}

	// Verify results contain the expected messages
	foundMessages := make(map[string]bool)
	for _, result := range results {
		foundMessages[result.Message] = true
	}

	expectedMessages := []string{
		"Database connection established",
		"Failed to connect to database",
	}

	for _, expected := range expectedMessages {
		if !foundMessages[expected] {
			t.Errorf("Expected to find message: %s", expected)
		}
	}
}

func TestSQLiteStorage_Search_LevelFilter(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store test entries with different levels
	entries := []*types.LogEntry{
		{
			Timestamp:  time.Now(),
			Level:      "INFO",
			TrackingID: "test-1",
			Message:    "Info message",
		},
		{
			Timestamp:  time.Now(),
			Level:      "ERROR",
			TrackingID: "test-2",
			Message:    "Error message",
		},
		{
			Timestamp:  time.Now(),
			Level:      "WARN",
			TrackingID: "test-3",
			Message:    "Warning message",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test level filter
	query := types.SearchQuery{
		Level: "ERROR",
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for ERROR level, got %d", len(results))
	}

	if results[0].Level != "ERROR" {
		t.Errorf("Expected ERROR level, got %s", results[0].Level)
	}
}

func TestSQLiteStorage_Search_TrackingIDFilter(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store test entries with different tracking IDs
	entries := []*types.LogEntry{
		{
			Timestamp:  time.Now(),
			Level:      "INFO",
			TrackingID: "user-123",
			Message:    "User action",
		},
		{
			Timestamp:  time.Now(),
			Level:      "INFO",
			TrackingID: "user-456",
			Message:    "Another user action",
		},
		{
			Timestamp:  time.Now(),
			Level:      "INFO",
			TrackingID: "user-123",
			Message:    "Same user different action",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test tracking ID filter
	query := types.SearchQuery{
		TrackingID: "user-123",
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for tracking ID 'user-123', got %d", len(results))
	}

	for _, result := range results {
		if result.TrackingID != "user-123" {
			t.Errorf("Expected tracking ID 'user-123', got %s", result.TrackingID)
		}
	}
}

func TestSQLiteStorage_Search_TimeRange(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	now := time.Now()
	
	// Store test entries with different timestamps
	entries := []*types.LogEntry{
		{
			Timestamp:  now.Add(-2 * time.Hour),
			Level:      "INFO",
			TrackingID: "test-1",
			Message:    "Old message",
		},
		{
			Timestamp:  now.Add(-30 * time.Minute),
			Level:      "INFO",
			TrackingID: "test-2",
			Message:    "Recent message",
		},
		{
			Timestamp:  now,
			Level:      "INFO",
			TrackingID: "test-3",
			Message:    "Current message",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test time range filter (last hour)
	startTime := now.Add(-1 * time.Hour)
	query := types.SearchQuery{
		StartTime: &startTime,
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for last hour, got %d", len(results))
	}

	// Verify no old messages
	for _, result := range results {
		if result.Message == "Old message" {
			t.Error("Should not find old message in time range filter")
		}
	}
}

func TestSQLiteStorage_Search_CombinedFilters(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	now := time.Now()
	
	// Store test entries
	entries := []*types.LogEntry{
		{
			Timestamp:  now,
			Level:      "ERROR",
			TrackingID: "user-123",
			Message:    "Database connection failed",
		},
		{
			Timestamp:  now,
			Level:      "INFO",
			TrackingID: "user-123",
			Message:    "Database connection established",
		},
		{
			Timestamp:  now,
			Level:      "ERROR",
			TrackingID: "user-456",
			Message:    "Database timeout error",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test combined filters: text search + level + tracking ID
	query := types.SearchQuery{
		Text:       "database",
		Level:      "ERROR",
		TrackingID: "user-123",
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for combined filters, got %d", len(results))
	}

	if len(results) > 0 {
		result := results[0]
		if result.Level != "ERROR" || result.TrackingID != "user-123" || result.Message != "Database connection failed" {
			t.Errorf("Unexpected result: %+v", result)
		}
	}
}

func TestSQLiteStorage_Search_LimitAndOffset(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store multiple test entries
	for i := 0; i < 10; i++ {
		entry := &types.LogEntry{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
			Level:      "INFO",
			TrackingID: "test",
			Message:    fmt.Sprintf("Message %d", i),
		}
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test limit
	query := types.SearchQuery{
		Limit: 3,
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results with limit, got %d", len(results))
	}

	// Test offset
	query = types.SearchQuery{
		Limit:  3,
		Offset: 2,
	}

	results, err = storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results with limit and offset, got %d", len(results))
	}
}

func TestSQLiteStorage_Cleanup(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	now := time.Now()
	
	// Store entries with different ages
	entries := []*types.LogEntry{
		{
			Timestamp:  now.AddDate(0, 0, -10), // 10 days old
			Level:      "INFO",
			TrackingID: "test-1",
			Message:    "Old message",
		},
		{
			Timestamp:  now.AddDate(0, 0, -1), // 1 day old
			Level:      "INFO",
			TrackingID: "test-2",
			Message:    "Recent message",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Cleanup entries older than 5 days
	err := storage.Cleanup(5)
	if err != nil {
		t.Fatalf("Failed to cleanup logs: %v", err)
	}

	// Verify only recent entries remain
	recent, err := storage.GetRecent(10)
	if err != nil {
		t.Fatalf("Failed to get recent logs: %v", err)
	}

	if len(recent) != 1 {
		t.Errorf("Expected 1 entry after cleanup, got %d", len(recent))
	}

	if len(recent) > 0 && recent[0].Message != "Recent message" {
		t.Errorf("Expected recent message to remain, got: %s", recent[0].Message)
	}
}

// Helper functions for testing

func setupTestStorage(t *testing.T) *SQLiteStorage {
	tmpFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	storage, err := NewSQLiteStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}

	return storage.(*SQLiteStorage)
}

func cleanupTestStorage(storage *SQLiteStorage) {
	if storage != nil {
		storage.Close()
	}
}