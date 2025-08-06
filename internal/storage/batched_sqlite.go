package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/metrics"
	"opentrail/internal/types"

	_ "modernc.org/sqlite"
)

// BatchedSQLiteStorage implements the LogStorage interface using SQLite with batched writes
type BatchedSQLiteStorage struct {
	// Database connection
	db *sql.DB

	// Batching configuration
	config BatchConfig

	// Processing components
	writeQueue  chan *writeRequest
	batchBuffer *batchBuffer
	batchTimer  *time.Timer
	batchMutex  sync.Mutex

	// Lifecycle management
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isRunning  bool
	runningMux sync.RWMutex

	// Prepared statements for batch operations
	insertStmt *sql.Stmt
	batchStmt  *sql.Stmt

	// Metrics
	metrics *metrics.StorageMetrics
}

// NewBatchedSQLiteStorage creates a new batched SQLite storage instance
func NewBatchedSQLiteStorage(dbPath string, config BatchConfig) (interfaces.LogStorage, error) {
	// Apply defaults and validate configuration
	config.ApplyDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid batch configuration: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", dbPath+"?_fk=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Create storage instance
	storage := &BatchedSQLiteStorage{
		db:          db,
		config:      config,
		writeQueue:  make(chan *writeRequest, config.QueueSize),
		batchBuffer: newBatchBuffer(config.BatchSize),
		ctx:         ctx,
		cancel:      cancel,
		isRunning:   false,
		metrics:     metrics.GetStorageMetrics(),
	}

	// Configure WAL mode if enabled
	if *config.WALEnabled {
		if err := storage.configureWALMode(); err != nil {
			storage.cleanup()
			return nil, fmt.Errorf("failed to configure WAL mode: %w", err)
		}
	} else {
		// Configure basic settings for non-WAL mode
		if err := storage.configureBasicMode(); err != nil {
			storage.cleanup()
			return nil, fmt.Errorf("failed to configure basic mode: %w", err)
		}
	}

	// Initialize database schema
	if err := storage.initializeDatabase(); err != nil {
		storage.cleanup()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Prepare statements for batch operations
	if err := storage.prepareStatements(); err != nil {
		storage.cleanup()
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	// Start the batch processor
	if err := storage.start(); err != nil {
		storage.cleanup()
		return nil, fmt.Errorf("failed to start batch processor: %w", err)
	}

	return storage, nil
}

// configureWALMode configures SQLite to use WAL mode with optimized settings
func (s *BatchedSQLiteStorage) configureWALMode() error {
	// Enable WAL mode for better concurrency
	if _, err := s.db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set synchronous mode to NORMAL for better performance while maintaining durability
	if _, err := s.db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	// Configure WAL auto-checkpoint to run every 1000 pages
	if _, err := s.db.Exec("PRAGMA wal_autocheckpoint = 1000"); err != nil {
		return fmt.Errorf("failed to set WAL auto-checkpoint: %w", err)
	}

	// Set busy timeout to 5 seconds to handle concurrent access
	if _, err := s.db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Enable foreign key constraints
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Verify WAL mode is enabled
	var journalMode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("failed to verify journal mode: %w", err)
	}
	if journalMode != "wal" {
		return fmt.Errorf("failed to enable WAL mode, current mode: %s", journalMode)
	}

	return nil
}

// configureBasicMode configures SQLite with basic settings (non-WAL mode)
func (s *BatchedSQLiteStorage) configureBasicMode() error {
	// Use default journal mode (delete)
	if _, err := s.db.Exec("PRAGMA journal_mode = DELETE"); err != nil {
		return fmt.Errorf("failed to set journal mode: %w", err)
	}

	// Verify journal mode was set correctly
	var journalMode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("failed to verify journal mode: %w", err)
	}
	if journalMode != "delete" {
		return fmt.Errorf("failed to set DELETE mode, current mode: %s", journalMode)
	}

	// Set synchronous mode to FULL for safety in non-WAL mode
	if _, err := s.db.Exec("PRAGMA synchronous = FULL"); err != nil {
		return fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	// Set busy timeout to 5 seconds to handle concurrent access
	if _, err := s.db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Enable foreign key constraints
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return nil
}

// initializeDatabase creates the necessary tables and indexes
func (s *BatchedSQLiteStorage) initializeDatabase() error {
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

// prepareStatements prepares SQL statements for batch operations
func (s *BatchedSQLiteStorage) prepareStatements() error {
	// Prepare single insert statement
	insertSQL := `
	INSERT INTO logs (priority, facility, severity, version, timestamp, hostname, app_name, proc_id, msg_id, structured_data, message)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var err error
	s.insertStmt, err = s.db.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}

	// For batch operations, we'll use the same statement but in a transaction
	// The batchStmt will be the same as insertStmt for now
	s.batchStmt = s.insertStmt

	return nil
}

// start begins the batch processor goroutine
func (s *BatchedSQLiteStorage) start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if s.isRunning {
		return fmt.Errorf("batch processor is already running")
	}

	s.isRunning = true
	s.wg.Add(1)

	// Initialize batch timer
	s.batchTimer = time.NewTimer(s.config.BatchTimeout)
	s.batchTimer.Stop() // Stop it initially, will be started when first request arrives

	// Start the batch processor goroutine
	go s.batchProcessor()

	return nil
}

// batchProcessor is the main goroutine that handles batch processing
func (s *BatchedSQLiteStorage) batchProcessor() {
	defer s.wg.Done()
	defer s.batchTimer.Stop()

	for {
		select {
		case req := <-s.writeQueue:
			// Add request to batch buffer
			s.batchMutex.Lock()
			isFull := s.batchBuffer.add(req)

			// Start timer if this is the first request in the buffer
			if s.batchBuffer.size() == 1 {
				s.batchTimer.Reset(s.config.BatchTimeout)
			}
			s.batchMutex.Unlock()

			// Process batch if it's full
			if isFull {
				s.processBatch()
			}

		case <-s.batchTimer.C:
			// Timeout reached, process current batch
			s.processBatch()

		case <-s.ctx.Done():
			// Context cancelled, process remaining requests and exit
			s.processBatch() // Process any remaining requests
			return
		}
	}
}

// processBatch processes the current batch of write requests
func (s *BatchedSQLiteStorage) processBatch() {
	s.batchMutex.Lock()
	requests := s.batchBuffer.flush()
	s.batchTimer.Stop() // Stop the timer since we're processing
	s.batchMutex.Unlock()

	if len(requests) == 0 {
		return
	}

	// Record batch metrics
	batchStart := time.Now()
	batchSize := len(requests)

	// Update buffer size metric
	s.metrics.UpdateBatchBufferSize(0) // Buffer is now empty

	// Process the batch of requests with actual database operations
	s.processBatchRequests(requests)

	// Record batch processing completion
	s.metrics.RecordBatchProcessed(batchSize, time.Since(batchStart))
}

// processBatchRequests handles the actual processing of a batch of requests
func (s *BatchedSQLiteStorage) processBatchRequests(requests []*writeRequest) {
	// Check if context is cancelled before processing
	select {
	case <-s.ctx.Done():
		// Context cancelled, send cancellation errors to all requests
		for _, req := range requests {
			req.sendResult(0, fmt.Errorf("batch processing cancelled: %w", s.ctx.Err()))
		}
		return
	default:
		// Continue with processing
	}

	// Try batch write first
	if err := s.executeBatchWrite(requests); err != nil {
		// Batch write failed, try individual writes with retry logic
		s.retryIndividualWrites(requests, err)
	}
}

// executeBatchWrite performs a batch database write operation within a transaction
func (s *BatchedSQLiteStorage) executeBatchWrite(requests []*writeRequest) error {
	txStart := time.Now()

	// Begin transaction for batch write
	tx, err := s.db.Begin()
	if err != nil {
		// Transaction failed to begin, all requests need individual retry
		s.retryIndividualWrites(requests, err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Prepare statement within transaction
	stmt := tx.Stmt(s.insertStmt)
	defer stmt.Close()

	// Track requests that need individual retry
	var failedRequests []*writeRequest
	var successfulWrites []struct {
		request *writeRequest
		result  sql.Result
	}

	// Execute all inserts within the transaction
	for _, req := range requests {
		// Check if request context is cancelled
		select {
		case <-req.ctx.Done():
			req.sendResult(0, fmt.Errorf("request cancelled: %w", req.ctx.Err()))
			continue
		default:
		}

		// Convert structured data to JSON string
		structuredDataJSON, err := s.convertStructuredDataToJSON(req.entry.StructuredData)
		if err != nil {
			// Data conversion failed, this request needs individual retry
			failedRequests = append(failedRequests, req)
			continue
		}

		// Execute the insert
		result, err := stmt.Exec(
			req.entry.Priority,
			req.entry.Facility,
			req.entry.Severity,
			req.entry.Version,
			req.entry.Timestamp,
			req.entry.Hostname,
			req.entry.AppName,
			req.entry.ProcID,
			req.entry.MsgID,
			structuredDataJSON,
			req.entry.Message,
		)

		if err != nil {
			// Individual insert failed within transaction, needs individual retry
			failedRequests = append(failedRequests, req)
			continue
		}

		// Store successful write for ID assignment
		successfulWrites = append(successfulWrites, struct {
			request *writeRequest
			result  sql.Result
		}{req, result})
	}

	// If any requests failed, rollback and retry all individually
	if len(failedRequests) > 0 {
		tx.Rollback()
		// Add successful writes to failed list for individual retry
		for _, write := range successfulWrites {
			failedRequests = append(failedRequests, write.request)
		}
		// Retry all requests individually
		s.retryIndividualWrites(failedRequests, fmt.Errorf("batch contained failed requests"))
		return fmt.Errorf("batch contained %d failed requests", len(failedRequests))
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		// Transaction commit failed, retry all requests individually
		allRequests := make([]*writeRequest, len(successfulWrites))
		for i, write := range successfulWrites {
			allRequests[i] = write.request
		}
		s.retryIndividualWrites(allRequests, err)
		return fmt.Errorf("transaction commit failed: %w", err)
	}

	// Record successful transaction
	s.metrics.RecordDatabaseTransaction(time.Since(txStart))

	// Assign IDs to successful writes
	for _, write := range successfulWrites {
		id, err := write.result.LastInsertId()
		if err != nil {
			write.request.sendResult(0, fmt.Errorf("failed to get insert ID: %w", err))
			continue
		}
		write.request.sendResult(id, nil)
	}

	return nil
}

// retryIndividualWrites handles individual retry logic for failed batch operations
func (s *BatchedSQLiteStorage) retryIndividualWrites(requests []*writeRequest, batchErr error) {
	for _, req := range requests {
		// Check if request context is cancelled
		select {
		case <-req.ctx.Done():
			req.sendResult(0, fmt.Errorf("request cancelled: %w", req.ctx.Err()))
			continue
		default:
		}

		// Retry individual write
		if err := s.executeIndividualWrite(req); err != nil {
			req.sendResult(0, fmt.Errorf("individual retry failed after batch error (%v): %w", batchErr, err))
		}
	}
}

// executeIndividualWrite performs a single database write operation
func (s *BatchedSQLiteStorage) executeIndividualWrite(req *writeRequest) error {
	// Convert structured data to JSON string
	structuredDataJSON, err := s.convertStructuredDataToJSON(req.entry.StructuredData)
	if err != nil {
		req.sendResult(0, fmt.Errorf("failed to convert structured data: %w", err))
		return fmt.Errorf("failed to convert structured data: %w", err)
	}

	// Execute the insert
	result, err := s.insertStmt.Exec(
		req.entry.Priority,
		req.entry.Facility,
		req.entry.Severity,
		req.entry.Version,
		req.entry.Timestamp,
		req.entry.Hostname,
		req.entry.AppName,
		req.entry.ProcID,
		req.entry.MsgID,
		structuredDataJSON,
		req.entry.Message,
	)

	if err != nil {
		req.sendResult(0, fmt.Errorf("individual insert failed: %w", err))
		return fmt.Errorf("individual insert failed: %w", err)
	}

	// Get the assigned ID
	id, err := result.LastInsertId()
	if err != nil {
		req.sendResult(0, fmt.Errorf("failed to get insert ID: %w", err))
		return fmt.Errorf("failed to get insert ID: %w", err)
	}

	// Send successful result
	req.sendResult(id, nil)
	return nil
}

// convertStructuredDataToJSON converts structured data map to JSON string
func (s *BatchedSQLiteStorage) convertStructuredDataToJSON(data map[string]interface{}) (string, error) {
	if data == nil || len(data) == 0 {
		return "", nil
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal structured data: %w", err)
	}

	return string(jsonBytes), nil
}

// cleanup performs cleanup operations
func (s *BatchedSQLiteStorage) cleanup() {
	if s.insertStmt != nil {
		s.insertStmt.Close()
	}
	if s.batchStmt != nil && s.batchStmt != s.insertStmt {
		s.batchStmt.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
}

// Store saves a log entry to the database (non-blocking)
func (s *BatchedSQLiteStorage) Store(entry *types.LogEntry) error {
	start := time.Now()

	// Check if storage is running
	s.runningMux.RLock()
	if !s.isRunning {
		s.runningMux.RUnlock()
		err := fmt.Errorf("storage is not running")
		s.metrics.RecordWriteRequest(time.Since(start), err)
		return err
	}
	s.runningMux.RUnlock()

	// Update queue utilization metrics
	queueLen := len(s.writeQueue)
	s.metrics.UpdateQueueUtilization(queueLen, s.config.QueueSize)
	s.metrics.UpdateBatchQueueSize(queueLen)

	// Create write request with context
	ctx, cancel := context.WithTimeout(s.ctx, s.config.WriteTimeout)
	defer cancel()

	req := newWriteRequest(entry, ctx)

	// Try to send request to queue (non-blocking)
	select {
	case s.writeQueue <- req:
		// Request queued successfully
		// For performance, use a short timeout - if batch processes quickly we get ID
		// If not, we timeout but write is still queued (fire-and-forget for performance)
		timeout := s.config.BatchTimeout + 20*time.Millisecond

		queueStart := time.Now()
		id, err := req.waitForResult(timeout)
		s.metrics.RecordQueueWaitTime(time.Since(queueStart))

		duration := time.Since(start)
		s.metrics.RecordWriteRequest(duration, err)

		if err != nil {
			// Timeout or error - but write is still queued, so return success
			// Set placeholder ID for compatibility
			entry.ID = 0
			return nil
		}

		// Got ID successfully
		entry.ID = id
		return nil

	default:
		// Queue is full, apply backpressure
		err := fmt.Errorf("write queue is full, please try again later")
		s.metrics.RecordQueueFullError()
		s.metrics.RecordWriteRequest(time.Since(start), err)
		return err
	}
}

// Search retrieves log entries based on the provided query
func (s *BatchedSQLiteStorage) Search(query types.SearchQuery) ([]*types.LogEntry, error) {
	start := time.Now()
	var err error
	defer func() {
		s.metrics.RecordReadRequest(time.Since(start), err)
	}()
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

	rows, queryErr := s.db.Query(baseQuery, args...)
	if queryErr != nil {
		err = fmt.Errorf("failed to execute search query: %w", queryErr)
		return nil, err
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
func (s *BatchedSQLiteStorage) GetRecent(limit int) ([]*types.LogEntry, error) {
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
func (s *BatchedSQLiteStorage) Cleanup(retentionDays int) error {
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

	// Run VACUUM to reclaim space after cleanup, but only if WAL mode is enabled
	// In WAL mode, VACUUM is more efficient and doesn't block readers
	if rowsAffected > 0 {
		if *s.config.WALEnabled {
			// In WAL mode, checkpoint before VACUUM for better performance
			if err := s.checkpointWAL(); err != nil {
				// Log warning but don't fail cleanup
				fmt.Printf("Warning: failed to checkpoint WAL before VACUUM: %v\n", err)
			}
		}

		if _, err := s.db.Exec("VACUUM"); err != nil {
			return fmt.Errorf("failed to vacuum database: %w", err)
		}
	}

	return nil
}

// checkpointWAL performs a WAL checkpoint to ensure data is written to main database
func (s *BatchedSQLiteStorage) checkpointWAL() error {
	// Perform a RESTART checkpoint to ensure all WAL data is moved to main database
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(RESTART)"); err != nil {
		return fmt.Errorf("failed to checkpoint WAL: %w", err)
	}
	return nil
}

// Close closes the storage connection with proper batch processing shutdown
func (s *BatchedSQLiteStorage) Close() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if !s.isRunning {
		return nil // Already closed
	}

	// Cancel context to signal shutdown
	s.cancel()

	// Wait for batch processor to finish
	s.wg.Wait()

	// Perform final checkpoint before closing if WAL mode is enabled
	if *s.config.WALEnabled {
		if err := s.checkpointWAL(); err != nil {
			// Log the error but don't fail the close operation
			fmt.Printf("Warning: failed to checkpoint WAL during close: %v\n", err)
		}
	}

	// Close prepared statements and database
	s.cleanup()

	s.isRunning = false
	return nil
}
