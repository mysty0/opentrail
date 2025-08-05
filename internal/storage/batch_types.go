package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"opentrail/internal/types"
)

// BatchConfig holds configuration parameters for batched SQLite operations
type BatchConfig struct {
	// BatchSize is the number of entries to batch together before writing to database
	// Default: 100
	BatchSize int `json:"batch_size"`

	// BatchTimeout is the maximum time to wait before writing a partial batch
	// Default: 100ms
	BatchTimeout time.Duration `json:"batch_timeout"`

	// QueueSize is the size of the internal write queue buffer
	// Default: 10000
	QueueSize int `json:"queue_size"`

	// WALEnabled controls whether to use WAL (Write-Ahead Logging) mode
	// Default: true
	WALEnabled *bool `json:"wal_enabled"`

	// WriteTimeout is the maximum time to wait for a write operation to complete
	// Default: 5s
	WriteTimeout time.Duration `json:"write_timeout"`
}

// DefaultBatchConfig returns a BatchConfig with sensible default values
func DefaultBatchConfig() BatchConfig {
	walEnabled := true
	return BatchConfig{
		BatchSize:    100,
		BatchTimeout: 100 * time.Millisecond,
		QueueSize:    10000,
		WALEnabled:   &walEnabled,
		WriteTimeout: 5 * time.Second,
	}
}

// Validate checks the configuration values and returns an error if any are invalid
func (c *BatchConfig) Validate() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be greater than 0, got %d", c.BatchSize)
	}

	if c.BatchSize > 10000 {
		return fmt.Errorf("batch_size must be <= 10000 for memory efficiency, got %d", c.BatchSize)
	}

	if c.BatchTimeout <= 0 {
		return fmt.Errorf("batch_timeout must be greater than 0, got %v", c.BatchTimeout)
	}

	if c.BatchTimeout > 10*time.Second {
		return fmt.Errorf("batch_timeout must be <= 10s for responsiveness, got %v", c.BatchTimeout)
	}

	if c.QueueSize <= 0 {
		return fmt.Errorf("queue_size must be greater than 0, got %d", c.QueueSize)
	}

	if c.QueueSize > 100000 {
		return fmt.Errorf("queue_size must be <= 100000 for memory efficiency, got %d", c.QueueSize)
	}

	if c.WriteTimeout <= 0 {
		return fmt.Errorf("write_timeout must be greater than 0, got %v", c.WriteTimeout)
	}

	if c.WriteTimeout > 60*time.Second {
		return fmt.Errorf("write_timeout must be <= 60s for responsiveness, got %v", c.WriteTimeout)
	}

	return nil
}

// ApplyDefaults fills in any zero values with defaults
func (c *BatchConfig) ApplyDefaults() {
	defaults := DefaultBatchConfig()

	if c.BatchSize == 0 {
		c.BatchSize = defaults.BatchSize
	}

	if c.BatchTimeout == 0 {
		c.BatchTimeout = defaults.BatchTimeout
	}

	if c.QueueSize == 0 {
		c.QueueSize = defaults.QueueSize
	}

	// WALEnabled: Apply default only if not set
	if c.WALEnabled == nil {
		c.WALEnabled = defaults.WALEnabled
	}

	if c.WriteTimeout == 0 {
		c.WriteTimeout = defaults.WriteTimeout
	}
}

// writeRequest represents a single write operation to be processed asynchronously
type writeRequest struct {
	// entry is the log entry to be written to the database
	entry *types.LogEntry

	// resultChan is used to communicate the result back to the caller
	// The channel will receive exactly one writeResult before being closed
	resultChan chan writeResult

	// ctx allows for request-level cancellation
	ctx context.Context

	// resultSent ensures result is only sent once
	resultSent sync.Once

	// completed tracks whether the request has been completed
	completed    bool
	completedMux sync.RWMutex
}

// writeResult contains the result of a write operation
type writeResult struct {
	// id is the database ID assigned to the log entry (0 if error occurred)
	id int64

	// err contains any error that occurred during the write operation
	err error
}

// newWriteRequest creates a new write request with the given entry and context
func newWriteRequest(entry *types.LogEntry, ctx context.Context) *writeRequest {
	return &writeRequest{
		entry:      entry,
		resultChan: make(chan writeResult, 1),
		ctx:        ctx,
	}
}

// sendResult sends the result to the result channel and closes it
// This method is safe to call multiple times - subsequent calls are no-ops
func (wr *writeRequest) sendResult(id int64, err error) {
	wr.resultSent.Do(func() {
		wr.resultChan <- writeResult{id: id, err: err}
		close(wr.resultChan)

		// Mark as completed
		wr.completedMux.Lock()
		wr.completed = true
		wr.completedMux.Unlock()
	})
}

// waitForResult waits for the write result with the given timeout
// Returns the result or a timeout error
func (wr *writeRequest) waitForResult(timeout time.Duration) (int64, error) {
	select {
	case result := <-wr.resultChan:
		return result.id, result.err
	case <-time.After(timeout):
		return 0, fmt.Errorf("write operation timed out after %v", timeout)
	case <-wr.ctx.Done():
		return 0, fmt.Errorf("write operation cancelled: %w", wr.ctx.Err())
	}
}

// isCompleted returns true if the write request has been completed (result sent)
func (wr *writeRequest) isCompleted() bool {
	wr.completedMux.RLock()
	defer wr.completedMux.RUnlock()
	return wr.completed
}

// batchBuffer manages a collection of write requests for batch processing
type batchBuffer struct {
	// requests holds the current batch of write requests
	requests []*writeRequest

	// mutex protects concurrent access to the buffer
	mutex sync.Mutex

	// maxSize is the maximum number of requests this buffer can hold
	maxSize int
}

// newBatchBuffer creates a new batch buffer with the specified maximum size
func newBatchBuffer(maxSize int) *batchBuffer {
	return &batchBuffer{
		requests: make([]*writeRequest, 0, maxSize),
		maxSize:  maxSize,
	}
}

// add adds a write request to the buffer
// Returns true if the buffer is now full, false otherwise
func (bb *batchBuffer) add(req *writeRequest) bool {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	bb.requests = append(bb.requests, req)
	return len(bb.requests) >= bb.maxSize
}

// flush returns all requests in the buffer and clears it
// The returned slice should not be modified by the caller
func (bb *batchBuffer) flush() []*writeRequest {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	if len(bb.requests) == 0 {
		return nil
	}

	// Create a copy of the requests slice
	flushed := make([]*writeRequest, len(bb.requests))
	copy(flushed, bb.requests)

	// Reset the buffer
	bb.requests = bb.requests[:0]

	return flushed
}

// size returns the current number of requests in the buffer
func (bb *batchBuffer) size() int {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	return len(bb.requests)
}

// isEmpty returns true if the buffer contains no requests
func (bb *batchBuffer) isEmpty() bool {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	return len(bb.requests) == 0
}
