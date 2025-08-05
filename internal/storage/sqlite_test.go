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
		Priority:  134, // facility 16, severity 6
		Facility:  16,
		Severity:  6,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		ProcID:    "123",
		MsgID:     "test",
		Message:   "Test log message",
		CreatedAt: time.Now(),
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

// WAL Mode Configuration Tests

func TestSQLiteStorage_WALModeConfiguration(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Test that WAL mode is enabled
	var journalMode string
	err := storage.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("Failed to query journal mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("Expected journal mode 'wal', got '%s'", journalMode)
	}
}

func TestSQLiteStorage_WALSynchronousMode(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Test that synchronous mode is set to NORMAL
	var synchronousMode int
	err := storage.db.QueryRow("PRAGMA synchronous").Scan(&synchronousMode)
	if err != nil {
		t.Fatalf("Failed to query synchronous mode: %v", err)
	}

	// NORMAL mode should be 1
	if synchronousMode != 1 {
		t.Errorf("Expected synchronous mode 1 (NORMAL), got %d", synchronousMode)
	}
}

func TestSQLiteStorage_WALAutoCheckpoint(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Test that WAL auto-checkpoint is configured
	var autoCheckpoint int
	err := storage.db.QueryRow("PRAGMA wal_autocheckpoint").Scan(&autoCheckpoint)
	if err != nil {
		t.Fatalf("Failed to query WAL auto-checkpoint: %v", err)
	}

	if autoCheckpoint != 1000 {
		t.Errorf("Expected WAL auto-checkpoint 1000, got %d", autoCheckpoint)
	}
}

func TestSQLiteStorage_BusyTimeout(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Test that busy timeout is configured
	var busyTimeout int
	err := storage.db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout)
	if err != nil {
		t.Fatalf("Failed to query busy timeout: %v", err)
	}

	if busyTimeout != 5000 {
		t.Errorf("Expected busy timeout 5000ms, got %d", busyTimeout)
	}
}

func TestSQLiteStorage_ForeignKeys(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Test that foreign keys are enabled
	var foreignKeys int
	err := storage.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
	if err != nil {
		t.Fatalf("Failed to query foreign keys: %v", err)
	}

	if foreignKeys != 1 {
		t.Errorf("Expected foreign keys enabled (1), got %d", foreignKeys)
	}
}

func TestSQLiteStorage_ConcurrentReadWrite(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Test concurrent read and write operations (WAL mode benefit)
	done := make(chan bool, 2)
	errors := make(chan error, 2)

	// Start a writer goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			entry := &types.LogEntry{
				Priority:  134,
				Facility:  16,
				Severity:  6,
				Version:   1,
				Timestamp: time.Now(),
				Hostname:  "test-host",
				AppName:   "test-app",
				ProcID:    fmt.Sprintf("writer-%d", i),
				MsgID:     "test",
				Message:   fmt.Sprintf("Writer message %d", i),
				CreatedAt: time.Now(),
			}
			if err := storage.Store(entry); err != nil {
				errors <- fmt.Errorf("writer error: %w", err)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Start a reader goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			_, err := storage.GetRecent(5)
			if err != nil {
				errors <- fmt.Errorf("reader error: %w", err)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Wait for both goroutines to complete
	completedCount := 0
	for completedCount < 2 {
		select {
		case <-done:
			completedCount++
		case err := <-errors:
			t.Fatalf("Concurrent operation failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent operations timed out")
		}
	}
}

func TestSQLiteStorage_CheckpointWAL(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store some entries to create WAL data
	for i := 0; i < 5; i++ {
		entry := &types.LogEntry{
			Priority:  134,
			Facility:  16,
			Severity:  6,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host",
			AppName:   "test-app",
			ProcID:    fmt.Sprintf("%d", i),
			MsgID:     "test",
			Message:   fmt.Sprintf("Test message %d", i),
			CreatedAt: time.Now(),
		}
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Failed to store entry: %v", err)
		}
	}

	// Test manual checkpoint
	err := storage.checkpointWAL()
	if err != nil {
		t.Errorf("Failed to checkpoint WAL: %v", err)
	}

	// Verify data is still accessible after checkpoint
	recent, err := storage.GetRecent(5)
	if err != nil {
		t.Fatalf("Failed to get recent logs after checkpoint: %v", err)
	}

	if len(recent) != 5 {
		t.Errorf("Expected 5 entries after checkpoint, got %d", len(recent))
	}
}

func TestSQLiteStorage_WALErrorDetection(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	testCases := []struct {
		name     string
		error    error
		expected bool
	}{
		{
			name:     "WAL error",
			error:    fmt.Errorf("wal file corrupted"),
			expected: true,
		},
		{
			name:     "Checkpoint error",
			error:    fmt.Errorf("checkpoint failed"),
			expected: true,
		},
		{
			name:     "Database locked error",
			error:    fmt.Errorf("database is locked"),
			expected: true,
		},
		{
			name:     "Disk I/O error",
			error:    fmt.Errorf("disk i/o error occurred"),
			expected: true,
		},
		{
			name:     "Malformed database error",
			error:    fmt.Errorf("database disk image is malformed"),
			expected: true,
		},
		{
			name:     "Regular SQL error",
			error:    fmt.Errorf("syntax error"),
			expected: false,
		},
		{
			name:     "Nil error",
			error:    nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := storage.isWALError(tc.error)
			if result != tc.expected {
				t.Errorf("Expected isWALError to return %v for error '%v', got %v", tc.expected, tc.error, result)
			}
		})
	}
}

func TestSQLiteStorage_WALRecovery(t *testing.T) {
	storage := setupTestStorage(t)
	defer cleanupTestStorage(storage)

	// Store some data first
	entry := &types.LogEntry{
		Priority:  134,
		Facility:  16,
		Severity:  6,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		ProcID:    "1",
		MsgID:     "test",
		Message:   "Test message before recovery",
		CreatedAt: time.Now(),
	}
	if err := storage.Store(entry); err != nil {
		t.Fatalf("Failed to store initial entry: %v", err)
	}

	// Test WAL recovery (this should not fail on a healthy database)
	err := storage.recoverFromWALCorruption()
	if err != nil {
		t.Errorf("WAL recovery failed on healthy database: %v", err)
	}

	// Verify data is still accessible after recovery
	recent, err := storage.GetRecent(1)
	if err != nil {
		t.Fatalf("Failed to get recent logs after recovery: %v", err)
	}

	if len(recent) != 1 {
		t.Errorf("Expected 1 entry after recovery, got %d", len(recent))
	}

	if len(recent) > 0 && recent[0].Message != "Test message before recovery" {
		t.Errorf("Data integrity check failed after recovery")
	}
}

func TestSQLiteStorage_CloseWithCheckpoint(t *testing.T) {
	storage := setupTestStorage(t)

	// Store some data to create WAL content
	entry := &types.LogEntry{
		Priority:  134,
		Facility:  16,
		Severity:  6,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		ProcID:    "1",
		MsgID:     "test",
		Message:   "Test message for close",
		CreatedAt: time.Now(),
	}
	if err := storage.Store(entry); err != nil {
		t.Fatalf("Failed to store entry: %v", err)
	}

	// Test that Close() performs checkpoint and closes cleanly
	err := storage.Close()
	if err != nil {
		t.Errorf("Failed to close storage with checkpoint: %v", err)
	}

	// Verify database is closed by trying to query (should fail)
	_, err = storage.GetRecent(1)
	if err == nil {
		t.Error("Expected error when querying closed database")
	}
}
