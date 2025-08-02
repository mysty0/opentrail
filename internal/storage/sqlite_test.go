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
		Priority:   134, // facility 16, severity 6
		Facility:   16,
		Severity:   6,
		Version:    1,
		Timestamp:  time.Now(),
		Hostname:   "test-host",
		AppName:    "test-app",
		ProcID:     "123",
		MsgID:      "test",
		Message:    "Test log message",
		CreatedAt:  time.Now(),
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
			Priority:  134, // facility 16, severity 6
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now().Add(-2 * time.Hour),
			Hostname:  "test-host-1",
			AppName:   "test-app",
			ProcID:    "1",
			MsgID:     "test",
			Message:   "First message",
			CreatedAt: time.Now(),
		},
		{
			Priority:  132, // facility 16, severity 4
			Facility:  16,
			Severity:  4,
			Version:   1,
			Timestamp: time.Now().Add(-1 * time.Hour),
			Hostname:  "test-host-2",
			AppName:   "test-app",
			ProcID:    "2",
			MsgID:     "test",
			Message:   "Second message",
			CreatedAt: time.Now(),
		},
		{
			Priority:  131, // facility 16, severity 3
			Facility:  16,
			Severity:  3,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host-3",
			AppName:   "test-app",
			ProcID:    "3",
			MsgID:     "test",
			Message:   "Third message",
			CreatedAt: time.Now(),
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
			Priority:  134, // facility 16, severity 6
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "db-host",
			AppName:   "db-app",
			ProcID:    "1",
			MsgID:     "conn",
			Message:   "Database connection established",
			CreatedAt: time.Now(),
		},
		{
			Priority:  131, // facility 16, severity 3
			Facility:  16,
			Severity:  3,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "db-host",
			AppName:   "db-app",
			ProcID:    "2",
			MsgID:     "error",
			Message:   "Failed to connect to database",
			CreatedAt: time.Now(),
		},
		{
			Priority:  134, // facility 16, severity 6
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "auth-host",
			AppName:   "auth-app",
			ProcID:    "3",
			MsgID:     "auth",
			Message:   "User authentication successful",
			CreatedAt: time.Now(),
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

	// Store test entries with different severities
	entries := []*types.LogEntry{
		{
			Priority:  134, // facility 16, severity 6 (info)
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "1",
			MsgID:     "info",
			Message:   "Info message",
			CreatedAt: time.Now(),
		},
		{
			Priority:  131, // facility 16, severity 3 (error)
			Facility:  16,
			Severity:  3,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "2",
			MsgID:     "error",
			Message:   "Error message",
			CreatedAt: time.Now(),
		},
		{
			Priority:  132, // facility 16, severity 4 (warning)
			Facility:  16,
			Severity:  4,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "3",
			MsgID:     "warn",
			Message:   "Warning message",
			CreatedAt: time.Now(),
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test severity filter
	severity := 3 // error severity
	query := types.SearchQuery{
		Severity: &severity,
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for severity 3, got %d", len(results))
	}

	if results[0].Severity != 3 {
		t.Errorf("Expected severity 3, got %d", results[0].Severity)
	}
}

func TestSQLiteStorage_Search_HostnameFilter(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store test entries with different hostnames
	entries := []*types.LogEntry{
		{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "web-01",
			AppName:   "nginx",
			ProcID:    "123",
			MsgID:     "access",
			Message:   "User action",
			CreatedAt: time.Now(),
		},
		{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "web-02",
			AppName:   "nginx",
			ProcID:    "456",
			MsgID:     "access",
			Message:   "Another user action",
			CreatedAt: time.Now(),
		},
		{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "web-01",
			AppName:   "nginx",
			ProcID:    "789",
			MsgID:     "access",
			Message:   "Same host different action",
			CreatedAt: time.Now(),
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test hostname filter
	query := types.SearchQuery{
		Hostname: "web-01",
	}

	results, err := storage.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for hostname 'web-01', got %d", len(results))
	}

	for _, result := range results {
		if result.Hostname != "web-01" {
			t.Errorf("Expected hostname 'web-01', got %s", result.Hostname)
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
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: now.Add(-2 * time.Hour),
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "1",
			MsgID:     "test",
			Message:   "Old message",
			CreatedAt: time.Now(),
		},
		{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: now.Add(-30 * time.Minute),
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "2",
			MsgID:     "test",
			Message:   "Recent message",
			CreatedAt: time.Now(),
		},
		{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: now,
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "3",
			MsgID:     "test",
			Message:   "Current message",
			CreatedAt: time.Now(),
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
			Priority:  131, // facility 16, severity 3 (error)
			Facility:  16,
			Severity:  3,
			Version:   1,
			Timestamp: now,
			Hostname:  "db-host",
			AppName:   "db-app",
			ProcID:    "123",
			MsgID:     "error",
			Message:   "Database connection failed",
			CreatedAt: time.Now(),
		},
		{
			Priority:  134, // facility 16, severity 6 (info)
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: now,
			Hostname:  "db-host",
			AppName:   "db-app",
			ProcID:    "123",
			MsgID:     "info",
			Message:   "Database connection established",
			CreatedAt: time.Now(),
		},
		{
			Priority:  131, // facility 16, severity 3 (error)
			Facility:  16,
			Severity:  3,
			Version:   1,
			Timestamp: now,
			Hostname:  "db-host",
			AppName:   "db-app",
			ProcID:    "456",
			MsgID:     "error",
			Message:   "Database timeout error",
			CreatedAt: time.Now(),
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test combined filters: text search + severity + proc_id
	severity := 3
	query := types.SearchQuery{
		Text:     "database",
		Severity: &severity,
		ProcID:   "123",
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
		if result.Severity != 3 || result.ProcID != "123" || result.Message != "Database connection failed" {
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
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    fmt.Sprintf("%d", i),
			MsgID:     "test",
			Message:   fmt.Sprintf("Message %d", i),
			CreatedAt: time.Now(),
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

func TestSQLiteStorage_Search_RFC5424Filters(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store test entries with different RFC5424 field values
	entries := []*types.LogEntry{
		{
			Priority:       134, // facility 16, severity 6
			Facility:       16,
			Severity:       6,
			Version:        1,
			Timestamp:      time.Now(),
			Hostname:       "web01",
			AppName:        "nginx",
			ProcID:         "1234",
			MsgID:          "access",
			StructuredData: map[string]interface{}{"user": "john", "action": "login"},
			Message:        "User login successful",
			CreatedAt:      time.Now(),
		},
		{
			Priority:       67, // facility 8, severity 3
			Facility:       8,
			Severity:       3,
			Version:        1,
			Timestamp:      time.Now(),
			Hostname:       "db01",
			AppName:        "postgres",
			ProcID:         "5678",
			MsgID:          "error",
			StructuredData: map[string]interface{}{"error_code": 500, "table": "users"},
			Message:        "Database connection failed",
			CreatedAt:      time.Now(),
		},
		{
			Priority:       132, // facility 16, severity 4
			Facility:       16,
			Severity:       4,
			Version:        1,
			Timestamp:      time.Now(),
			Hostname:       "web01",
			AppName:        "nginx",
			ProcID:         "1234",
			MsgID:          "warning",
			StructuredData: map[string]interface{}{"response_time": 2000},
			Message:        "Slow response detected",
			CreatedAt:      time.Now(),
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	testCases := []struct {
		name     string
		query    types.SearchQuery
		expected int
		validate func(*testing.T, []*types.LogEntry)
	}{
		{
			name: "Filter by facility",
			query: types.SearchQuery{
				Facility: func() *int { i := 16; return &i }(),
			},
			expected: 2,
			validate: func(t *testing.T, results []*types.LogEntry) {
				for _, result := range results {
					if result.Facility != 16 {
						t.Errorf("Expected facility 16, got %d", result.Facility)
					}
				}
			},
		},
		{
			name: "Filter by severity",
			query: types.SearchQuery{
				Severity: func() *int { i := 3; return &i }(),
			},
			expected: 1,
			validate: func(t *testing.T, results []*types.LogEntry) {
				if len(results) > 0 && results[0].Severity != 3 {
					t.Errorf("Expected severity 3, got %d", results[0].Severity)
				}
			},
		},
		{
			name: "Filter by min severity (4 and below)",
			query: types.SearchQuery{
				MinSeverity: func() *int { i := 4; return &i }(),
			},
			expected: 2,
			validate: func(t *testing.T, results []*types.LogEntry) {
				for _, result := range results {
					if result.Severity > 4 {
						t.Errorf("Expected severity <= 4, got %d", result.Severity)
					}
				}
			},
		},
		{
			name: "Filter by hostname",
			query: types.SearchQuery{
				Hostname: "web01",
			},
			expected: 2,
			validate: func(t *testing.T, results []*types.LogEntry) {
				for _, result := range results {
					if result.Hostname != "web01" {
						t.Errorf("Expected hostname 'web01', got '%s'", result.Hostname)
					}
				}
			},
		},
		{
			name: "Filter by app name",
			query: types.SearchQuery{
				AppName: "nginx",
			},
			expected: 2,
			validate: func(t *testing.T, results []*types.LogEntry) {
				for _, result := range results {
					if result.AppName != "nginx" {
						t.Errorf("Expected app_name 'nginx', got '%s'", result.AppName)
					}
				}
			},
		},
		{
			name: "Filter by proc ID",
			query: types.SearchQuery{
				ProcID: "1234",
			},
			expected: 2,
			validate: func(t *testing.T, results []*types.LogEntry) {
				for _, result := range results {
					if result.ProcID != "1234" {
						t.Errorf("Expected proc_id '1234', got '%s'", result.ProcID)
					}
				}
			},
		},
		{
			name: "Filter by msg ID",
			query: types.SearchQuery{
				MsgID: "error",
			},
			expected: 1,
			validate: func(t *testing.T, results []*types.LogEntry) {
				if len(results) > 0 && results[0].MsgID != "error" {
					t.Errorf("Expected msg_id 'error', got '%s'", results[0].MsgID)
				}
			},
		},
		{
			name: "Filter by structured data query",
			query: types.SearchQuery{
				StructuredDataQuery: "john",
			},
			expected: 1,
			validate: func(t *testing.T, results []*types.LogEntry) {
				if len(results) > 0 {
					if user, ok := results[0].StructuredData["user"]; !ok || user != "john" {
						t.Errorf("Expected structured data to contain user 'john'")
					}
				}
			},
		},
		{
			name: "Combined RFC5424 filters",
			query: types.SearchQuery{
				Facility: func() *int { i := 16; return &i }(),
				Hostname: "web01",
				AppName:  "nginx",
			},
			expected: 2,
			validate: func(t *testing.T, results []*types.LogEntry) {
				for _, result := range results {
					if result.Facility != 16 || result.Hostname != "web01" || result.AppName != "nginx" {
						t.Errorf("Combined filter validation failed for result: %+v", result)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := storage.Search(tc.query)
			if err != nil {
				t.Fatalf("Failed to search logs: %v", err)
			}

			if len(results) != tc.expected {
				t.Errorf("Expected %d results, got %d", tc.expected, len(results))
			}

			if tc.validate != nil {
				tc.validate(t, results)
			}
		})
	}
}

func TestSQLiteStorage_Cleanup(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	now := time.Now()
	
	// Store entries with different ages
	entries := []*types.LogEntry{
		{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: now.AddDate(0, 0, -10), // 10 days old
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "1",
			MsgID:     "test",
			Message:   "Old message",
			CreatedAt: time.Now(),
		},
		{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: now.AddDate(0, 0, -1), // 1 day old
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    "2",
			MsgID:     "test",
			Message:   "Recent message",
			CreatedAt: time.Now(),
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