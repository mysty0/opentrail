package storage

import (
	"database/sql"
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
	// Create main logs table
	createLogsTable := `
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		level TEXT NOT NULL,
		tracking_id TEXT,
		message TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(createLogsTable); err != nil {
		return fmt.Errorf("failed to create logs table: %w", err)
	}

	// Create FTS5 virtual table for full-text search
	createFTSTable := `
	CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts USING fts5(
		message,
		content='logs',
		content_rowid='id'
	);`

	if _, err := s.db.Exec(createFTSTable); err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}

	// Create indexes for efficient querying
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);",
		"CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);",
		"CREATE INDEX IF NOT EXISTS idx_logs_tracking_id ON logs(tracking_id);",
		"CREATE INDEX IF NOT EXISTS idx_logs_created_at ON logs(created_at);",
	}

	for _, indexSQL := range indexes {
		if _, err := s.db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Create triggers to keep FTS5 table in sync
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS logs_ai AFTER INSERT ON logs BEGIN
			INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS logs_ad AFTER DELETE ON logs BEGIN
			INSERT INTO logs_fts(logs_fts, rowid, message) VALUES('delete', old.id, old.message);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS logs_au AFTER UPDATE ON logs BEGIN
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
	query := `
	INSERT INTO logs (timestamp, level, tracking_id, message)
	VALUES (?, ?, ?, ?)
	`

	result, err := s.db.Exec(query, entry.Timestamp, entry.Level, entry.TrackingID, entry.Message)
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

	baseQuery := "SELECT id, timestamp, level, tracking_id, message FROM logs"

	// Handle full-text search
	if query.Text != "" {
		baseQuery = `
		SELECT l.id, l.timestamp, l.level, l.tracking_id, l.message 
		FROM logs l 
		JOIN logs_fts fts ON l.id = fts.rowid 
		WHERE logs_fts MATCH ?`
		args = append(args, query.Text)
	}

	// Add other filters
	if query.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, query.Level)
	}

	if query.TrackingID != "" {
		conditions = append(conditions, "tracking_id = ?")
		args = append(args, query.TrackingID)
	}

	if query.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, query.StartTime)
	}

	if query.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, query.EndTime)
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
		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Level, &entry.TrackingID, &entry.Message)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
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
	SELECT id, timestamp, level, tracking_id, message 
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
		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Level, &entry.TrackingID, &entry.Message)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
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