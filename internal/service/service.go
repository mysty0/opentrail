package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

const (
	// DefaultBatchSize is the default number of logs to process in a batch
	DefaultBatchSize = 100
	// DefaultBatchTimeout is the default timeout for batch processing
	DefaultBatchTimeout = 100 * time.Millisecond
	// DefaultQueueSize is the default size of the processing queue
	DefaultQueueSize = 10000
	// MaxSubscribers is the maximum number of concurrent subscribers
	MaxSubscribers = 100
)

// LogService implements the central log processing service
type LogService struct {
	parser  interfaces.LogParser
	storage interfaces.LogStorage

	// Configuration
	batchSize    int
	batchTimeout time.Duration
	queueSize    int

	// Processing queue and batch management
	logQueue    chan string
	batchBuffer []string
	batchMutex  sync.Mutex
	batchTimer  *time.Timer

	// Real-time subscriptions
	subscribers    map[chan *types.LogEntry]bool
	subscribersMux sync.RWMutex

	// Service lifecycle
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isRunning  bool
	runningMux sync.RWMutex

	// Statistics
	stats      interfaces.ServiceStats
	statsMutex sync.RWMutex
}

// NewLogService creates a new LogService instance
func NewLogService(parser interfaces.LogParser, storage interfaces.LogStorage) *LogService {
	ctx, cancel := context.WithCancel(context.Background())

	return &LogService{
		parser:       parser,
		storage:      storage,
		batchSize:    DefaultBatchSize,
		batchTimeout: DefaultBatchTimeout,
		queueSize:    DefaultQueueSize,
		logQueue:     make(chan string, DefaultQueueSize),
		batchBuffer:  make([]string, 0, DefaultBatchSize),
		subscribers:  make(map[chan *types.LogEntry]bool),
		ctx:          ctx,
		cancel:       cancel,
		stats: interfaces.ServiceStats{
			IsRunning: false,
		},
	}
}

// SetBatchSize configures the batch processing size
func (s *LogService) SetBatchSize(size int) {
	if size > 0 {
		s.batchSize = size
	}
}

// SetBatchTimeout configures the batch processing timeout
func (s *LogService) SetBatchTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.batchTimeout = timeout
	}
}

// SetQueueSize configures the processing queue size
func (s *LogService) SetQueueSize(size int) {
	if size > 0 {
		s.queueSize = size
	}
}

// Start starts the service background processes
func (s *LogService) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if s.isRunning {
		return fmt.Errorf("service is already running")
	}

	// Start the batch processor
	s.wg.Add(1)
	go s.batchProcessor()

	s.isRunning = true
	s.updateStats(func(stats *interfaces.ServiceStats) {
		stats.IsRunning = true
	})

	return nil
}

// Stop gracefully stops the service
func (s *LogService) Stop() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if !s.isRunning {
		return nil
	}

	// Cancel context to signal shutdown
	s.cancel()

	// Close the log queue to stop accepting new logs
	close(s.logQueue)

	// Wait for all goroutines to finish
	s.wg.Wait()

	// Process any remaining logs in the batch buffer
	s.processBatch()

	// Close all subscriber channels
	s.subscribersMux.Lock()
	for ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = make(map[chan *types.LogEntry]bool)
	s.subscribersMux.Unlock()

	s.isRunning = false
	s.updateStats(func(stats *interfaces.ServiceStats) {
		stats.IsRunning = false
		stats.ActiveSubscribers = 0
	})

	return nil
}

// ProcessLog processes a single raw log message
func (s *LogService) ProcessLog(rawMessage string) error {
	s.runningMux.RLock()
	if !s.isRunning {
		s.runningMux.RUnlock()
		return fmt.Errorf("service is not running")
	}
	s.runningMux.RUnlock()

	select {
	case s.logQueue <- rawMessage:
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("service is shutting down")
	default:
		// Queue is full, implement backpressure
		s.updateStats(func(stats *interfaces.ServiceStats) {
			stats.FailedLogs++
		})
		return fmt.Errorf("log queue is full, dropping message")
	}
}

// ProcessLogBatch processes multiple raw log messages in a batch
func (s *LogService) ProcessLogBatch(rawMessages []string) error {
	for _, msg := range rawMessages {
		if err := s.ProcessLog(msg); err != nil {
			return err
		}
	}
	return nil
}

// Search retrieves log entries based on the provided query
func (s *LogService) Search(query types.SearchQuery) ([]*types.LogEntry, error) {
	return s.storage.Search(query)
}

// GetRecent retrieves the most recent log entries
func (s *LogService) GetRecent(limit int) ([]*types.LogEntry, error) {
	return s.storage.GetRecent(limit)
}

// Subscribe creates a subscription for real-time log updates
func (s *LogService) Subscribe() <-chan *types.LogEntry {
	s.subscribersMux.Lock()
	defer s.subscribersMux.Unlock()

	// Check if we've reached the maximum number of subscribers
	if len(s.subscribers) >= MaxSubscribers {
		// Return a closed channel to indicate failure
		ch := make(chan *types.LogEntry)
		close(ch)
		return ch
	}

	ch := make(chan *types.LogEntry, 100) // Buffered channel to prevent blocking
	s.subscribers[ch] = true

	s.updateStats(func(stats *interfaces.ServiceStats) {
		stats.ActiveSubscribers = len(s.subscribers)
	})

	return ch
}

// Unsubscribe removes a subscription
func (s *LogService) Unsubscribe(subscription <-chan *types.LogEntry) {
	s.subscribersMux.Lock()
	defer s.subscribersMux.Unlock()

	// Find and remove the subscription channel
	for ch := range s.subscribers {
		if ch == subscription {
			delete(s.subscribers, ch)
			close(ch)

			s.updateStats(func(stats *interfaces.ServiceStats) {
				stats.ActiveSubscribers = len(s.subscribers)
			})
			break
		}
	}
}

// GetStats returns service statistics
func (s *LogService) GetStats() interfaces.ServiceStats {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()

	stats := s.stats
	stats.QueueSize = len(s.logQueue)

	return stats
}

// batchProcessor runs in a separate goroutine to process logs in batches
func (s *LogService) batchProcessor() {
	defer s.wg.Done()

	s.batchTimer = time.NewTimer(s.batchTimeout)
	defer s.batchTimer.Stop()

	for {
		select {
		case rawMessage, ok := <-s.logQueue:
			if !ok {
				// Channel closed, process remaining batch and exit
				return
			}

			s.batchMutex.Lock()
			s.batchBuffer = append(s.batchBuffer, rawMessage)

			// Process batch if it's full
			if len(s.batchBuffer) >= s.batchSize {
				s.processBatch()
				s.resetBatchTimer()
			}
			s.batchMutex.Unlock()

		case <-s.batchTimer.C:
			//fmt.Println("batch timer", s.batchTimeout)
			// Timeout reached, process current batch
			s.batchMutex.Lock()
			if len(s.batchBuffer) > 0 {
				s.processBatch()
			}
			s.resetBatchTimer()
			s.batchMutex.Unlock()

		case <-s.ctx.Done():
			// Service is shutting down
			return
		}
	}
}

// processBatch processes the current batch of log messages
func (s *LogService) processBatch() {
	if len(s.batchBuffer) == 0 {
		return
	}

	batch := make([]string, len(s.batchBuffer))
	copy(batch, s.batchBuffer)
	s.batchBuffer = s.batchBuffer[:0] // Clear the buffer
	//fmt.Println("processing batch size: ", len(batch))
	// Process each log in the batch
	for _, rawMessage := range batch {
		if err := s.processLogMessage(rawMessage); err != nil {
			log.Printf("Error processing log message: %v", err)
			s.updateStats(func(stats *interfaces.ServiceStats) {
				stats.FailedLogs++
			})
		} else {
			s.updateStats(func(stats *interfaces.ServiceStats) {
				stats.ProcessedLogs++
			})
		}
	}
	//fmt.Println("processing batch done")
}

// processLogMessage processes a single log message
func (s *LogService) processLogMessage(rawMessage string) error {
	// Parse the log message
	logEntry, err := s.parser.Parse(rawMessage)
	if err != nil {
		return fmt.Errorf("failed to parse log message: %w", err)
	}

	// Store the log entry
	if err := s.storage.Store(logEntry); err != nil {
		return fmt.Errorf("failed to store log entry: %w", err)
	}

	// Notify subscribers
	s.notifySubscribers(logEntry)

	return nil
}

// notifySubscribers sends the log entry to all active subscribers
func (s *LogService) notifySubscribers(logEntry *types.LogEntry) {
	s.subscribersMux.RLock()
	defer s.subscribersMux.RUnlock()

	for ch := range s.subscribers {
		select {
		case ch <- logEntry:
			// Successfully sent
		default:
			// Channel is full, skip this subscriber to prevent blocking
			log.Printf("Subscriber channel is full, skipping notification")
		}
	}
}

// resetBatchTimer resets the batch processing timer
func (s *LogService) resetBatchTimer() {
	if !s.batchTimer.Stop() {
		select {
		case <-s.batchTimer.C:
		default:
		}
	}
	s.batchTimer.Reset(s.batchTimeout)
}

// updateStats safely updates the service statistics
func (s *LogService) updateStats(updateFunc func(*interfaces.ServiceStats)) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()
	updateFunc(&s.stats)
}
