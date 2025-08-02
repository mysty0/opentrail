package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

// SQLiteStorage implements the LogStorage interface using SQLite with FTS5
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (interfaces.LogStorage, error) {
	db, err := sql.Open("sqlite", dbPath+"?_fk=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &SQLiteStorage{db: db}
	if err := storage.initializeDatabase(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return storage, nil
}

// initializeDatabase creates the necessary tables and indexes
func (s *SQLiteStorage) initializeDatabase() error {
	// Remove old database file on startup (hard reset)
	if _, err := s.db.Exec("DROP TABLE IF EXISTS logs"); err != nil {
		return fmt.Errorf("failed to drop old logs table: %w", err)
	}
	if _, err := s.db.Exec("DROP TABLE IF EXISTS logs_fts"); err != nil {
		return fmt.Errorf("failed to drop old FTS table: %w", err)
	}

	// Create main RFC5424 logs table
	createLogsTable := `
	CREATE TABLE logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		
		-- RFC5424 Header Fields
		priority INTEGER NOT NULL,
		facility INTEGER NOT NULL,
		severity INTEGER NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		timestamp DATETIME NOT NULL,
		hostname TEXT,
		app_name TEXT,
		proc_id TEXT,
		msg_id TEXT,
		
		-- Structured Data and Message
		structured_data TEXT, -- JSON string
		message TEXT NOT NULL,
		
		-- System Fields
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(createLogsTable); err != nil {
		return fmt.Errorf("failed to create logs table: %w", err)
	}

	// Create FTS5 virtual table for full-text search on message
	createFTSTable := `
	CREATE VIRTUAL TABLE logs_fts USING fts5(
		message,
		content='logs',
		content_rowid='id'
	);`

	if _, err := s.db.Exec(createFTSTable); err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}

	// Create indexes for efficient RFC5424 field queries
	indexes := []string{
		"CREATE INDEX idx_logs_timestamp ON logs(timestamp);",
		"CREATE INDEX idx_logs_facility ON logs(facility);",
		"CREATE INDEX idx_logs_severity ON logs(severity);",
		"CREATE INDEX idx_logs_hostname ON logs(hostname);",
		"CREATE INDEX idx_logs_app_name ON logs(app_name);",
		"CREATE INDEX idx_logs_proc_id ON logs(proc_id);",
		"CREATE INDEX idx_logs_msg_id ON logs(msg_id);",
		"CREATE INDEX idx_logs_priority ON logs(priority);",
		"CREATE INDEX idx_logs_created_at ON logs(created_at);",
		// Composite indexes for common query patterns
		"CREATE INDEX idx_logs_facility_severity ON logs(facility, severity);",
		"CREATE INDEX idx_logs_hostname_app_name ON logs(hostname, app_name);",
		"CREATE INDEX idx_logs_timestamp_severity ON logs(timestamp, severity);",
	}

	for _, indexSQL := range indexes {
		if _, err := s.db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Create triggers to keep FTS5 table in sync
	triggers := []string{
		`CREATE TRIGGER logs_ai AFTER INSERT ON logs BEGIN
			INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
		END;`,
		`CREATE TRIGGER logs_ad AFTER DELETE ON logs BEGIN
			INSERT INTO logs_fts(logs_fts, rowid, message) VALUES('delete', old.id, old.message);
		END;`,
		`CREATE TRIGGER logs_au AFTER UPDATE ON logs BEGIN
			INSERT INTO logs_fts(logs_fts, rowid, message) VALUES('delete', old.id, old.message);
			INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
		END;`,
	}

	for _, triggerSQL := range triggers {
		if _, err := s.db.Exec(triggerSQL); err != nil {
			return fmt.Errorf("failed to create trigger: %w", err)
		}
	}

	return nil
}

// Store saves a log entry to the database
func (s *SQLiteStorage) Store(entry *types.LogEntry) error {
	// Convert structured data to JSON string
	var structuredDataJSON string
	if entry.StructuredData != nil {
		jsonBytes, err := json.Marshal(entry.StructuredData)
		if err != nil {
			return fmt.Errorf("failed to marshal structured data: %w", err)
		}
		structuredDataJSON = string(jsonBytes)
	}

	query := `
	INSERT INTO logs (priority, facility, severity, version, timestamp, hostname, app_name, proc_id, msg_id, structured_data, message)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(query, 
		entry.Priority, entry.Facility, entry.Severity, entry.Version,
		entry.Timestamp, entry.Hostname, entry.AppName, entry.ProcID, entry.MsgID,
		structuredDataJSON, entry.Message)
	if err != nil {
		return fmt.Errorf("failed to store log entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get inserted ID: %w", err)
	}

	entry.ID = id
	return nil
}

// Search retrieves log entries based on the provided query
func (s *SQLiteStorage) Search(query types.SearchQuery) ([]*types.LogEntry, error) {
	var conditions []string
	var args []interface{}

	baseQuery := `SELECT id, priority, facility, severity, version, timestamp, hostname, app_name, proc_id, msg_id, structured_data, message, created_at FROM logs`

	// Handle full-text search
	if query.Text != "" {
		baseQuery = `
		SELECT l.id, l.priority, l.facility, l.severity, l.version, l.timestamp, l.hostname, l.app_name, l.proc_id, l.msg_id, l.structured_data, l.message, l.created_at
		FROM logs l 
		JOIN logs_fts fts ON l.id = fts.rowid 
		WHERE logs_fts MATCH ?`
		args = append(args, query.Text)
	}

	// Add RFC5424 filters
	if query.Facility != nil {
		conditions = append(conditions, "facility = ?")
		args = append(args, *query.Facility)
	}

	if query.Severity != nil {
		conditions = append(conditions, "severity = ?")
		args = append(args, *query.Severity)
	}

	if query.MinSeverity != nil {
		conditions = append(conditions, "severity <= ?")
		args = append(args, *query.MinSeverity)
	}

	if query.Hostname != "" {
		conditions = append(conditions, "hostname = ?")
		args = append(args, query.Hostname)
	}

	if query.AppName != "" {
		conditions = append(conditions, "app_name = ?")
		args = append(args, query.AppName)
	}

	if query.ProcID != "" {
		conditions = append(conditions, "proc_id = ?")
		args = append(args, query.ProcID)
	}

	if query.MsgID != "" {
		conditions = append(conditions, "msg_id = ?")
		args = append(args, query.MsgID)
	}

	if query.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, query.StartTime)
	}

	if query.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, query.EndTime)
	}

	// Handle structured data query (basic JSON search)
	if query.StructuredDataQuery != "" {
		conditions = append(conditions, "structured_data LIKE ?")
		args = append(args, "%"+query.StructuredDataQuery+"%")
	}

	// Combine conditions
	if len(conditions) > 0 {
		if query.Text != "" {
			baseQuery += " AND " + strings.Join(conditions, " AND ")
		} else {
			baseQuery += " WHERE " + strings.Join(conditions, " AND ")
		}
	}

	// Add ordering and limits
	baseQuery += " ORDER BY timestamp DESC"

	if query.Limit > 0 {
		baseQuery += " LIMIT ?"
		args = append(args, query.Limit)
	}

	if query.Offset > 0 {
		baseQuery += " OFFSET ?"
		args = append(args, query.Offset)
	}

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var entries []*types.LogEntry
	for rows.Next() {
		entry := &types.LogEntry{}
		var structuredDataJSON sql.NullString
		
		err := rows.Scan(&entry.ID, &entry.Priority, &entry.Facility, &entry.Severity, &entry.Version,
			&entry.Timestamp, &entry.Hostname, &entry.AppName, &entry.ProcID, &entry.MsgID,
			&structuredDataJSON, &entry.Message, &entry.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}

		// Parse structured data JSON
		if structuredDataJSON.Valid && structuredDataJSON.String != "" {
			var structuredData map[string]interface{}
			if err := json.Unmarshal([]byte(structuredDataJSON.String), &structuredData); err == nil {
				entry.StructuredData = structuredData
			}
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return entries, nil
}

// GetRecent retrieves the most recent log entries up to the specified limit
func (s *SQLiteStorage) GetRecent(limit int) ([]*types.LogEntry, error) {
	query := `
	SELECT id, priority, facility, severity, version, timestamp, hostname, app_name, proc_id, msg_id, structured_data, message, created_at
	FROM logs 
	ORDER BY timestamp DESC 
	LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent logs: %w", err)
	}
	defer rows.Close()

	var entries []*types.LogEntry
	for rows.Next() {
		entry := &types.LogEntry{}
		var structuredDataJSON sql.NullString
		
		err := rows.Scan(&entry.ID, &entry.Priority, &entry.Facility, &entry.Severity, &entry.Version,
			&entry.Timestamp, &entry.Hostname, &entry.AppName, &entry.ProcID, &entry.MsgID,
			&structuredDataJSON, &entry.Message, &entry.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}

		// Parse structured data JSON
		if structuredDataJSON.Valid && structuredDataJSON.String != "" {
			var structuredData map[string]interface{}
			if err := json.Unmarshal([]byte(structuredDataJSON.String), &structuredData); err == nil {
				entry.StructuredData = structuredData
			}
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return entries, nil
}

// Cleanup removes log entries older than the specified retention period
func (s *SQLiteStorage) Cleanup(retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	
	query := "DELETE FROM logs WHERE timestamp < ?"
	result, err := s.db.Exec(query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get cleanup result: %w", err)
	}

	// Run VACUUM to reclaim space after cleanup
	if rowsAffected > 0 {
		if _, err := s.db.Exec("VACUUM"); err != nil {
			return fmt.Errorf("failed to vacuum database: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}