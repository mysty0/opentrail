package interfaces

import "opentrail/internal/types"

// LogStorage defines the interface for log storage operations
type LogStorage interface {
	// Store saves a log entry to the storage backend
	Store(entry *types.LogEntry) error
	
	// Search retrieves log entries based on the provided query
	Search(query types.SearchQuery) ([]*types.LogEntry, error)
	
	// GetRecent retrieves the most recent log entries up to the specified limit
	GetRecent(limit int) ([]*types.LogEntry, error)
	
	// Cleanup removes log entries older than the specified retention period
	Cleanup(retentionDays int) error
	
	// Close closes the storage connection
	Close() error
}