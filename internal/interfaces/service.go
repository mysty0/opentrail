package interfaces

import "opentrail/internal/types"

// LogService defines the interface for the central log processing service
type LogService interface {
	// ProcessLog processes a raw log message through parsing and storage
	ProcessLog(rawMessage string) error
	
	// ProcessLogBatch processes multiple raw log messages in a batch
	ProcessLogBatch(rawMessages []string) error
	
	// Search retrieves log entries based on the provided query
	Search(query types.SearchQuery) ([]*types.LogEntry, error)
	
	// GetRecent retrieves the most recent log entries
	GetRecent(limit int) ([]*types.LogEntry, error)
	
	// Subscribe creates a subscription for real-time log updates
	Subscribe() <-chan *types.LogEntry
	
	// Unsubscribe removes a subscription
	Unsubscribe(subscription <-chan *types.LogEntry)
	
	// Start starts the service background processes
	Start() error
	
	// Stop gracefully stops the service
	Stop() error
	
	// GetStats returns service statistics
	GetStats() ServiceStats
}

// ServiceStats represents statistics about the log service
type ServiceStats struct {
	ProcessedLogs     int64 `json:"processed_logs"`
	FailedLogs        int64 `json:"failed_logs"`
	ActiveSubscribers int   `json:"active_subscribers"`
	QueueSize         int   `json:"queue_size"`
	IsRunning         bool  `json:"is_running"`
}