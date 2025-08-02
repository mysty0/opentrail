package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"opentrail/internal/types"
)

// MockParser implements the LogParser interface for testing
type MockParser struct {
	parseFunc     func(string) (*types.LogEntry, error)
	setFormatFunc func(string) error
}

func (m *MockParser) Parse(rawMessage string) (*types.LogEntry, error) {
	if m.parseFunc != nil {
		return m.parseFunc(rawMessage)
	}
	entry := &types.LogEntry{
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		ProcID:    "123",
		MsgID:     "test",
		Message:   rawMessage,
		CreatedAt: time.Now(),
	}
	entry.SetPriority(134) // facility 16, severity 6
	return entry, nil
}

func (m *MockParser) SetFormat(format string) error {
	if m.setFormatFunc != nil {
		return m.setFormatFunc(format)
	}
	return nil
}

// MockStorage implements the LogStorage interface for testing
type MockStorage struct {
	storeFunc     func(*types.LogEntry) error
	searchFunc    func(types.SearchQuery) ([]*types.LogEntry, error)
	getRecentFunc func(int) ([]*types.LogEntry, error)
	cleanupFunc   func(int) error
	
	storedLogs []types.LogEntry
	mutex      sync.RWMutex
}

func (m *MockStorage) Store(entry *types.LogEntry) error {
	if m.storeFunc != nil {
		return m.storeFunc(entry)
	}
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.storedLogs = append(m.storedLogs, *entry)
	return nil
}

func (m *MockStorage) Search(query types.SearchQuery) ([]*types.LogEntry, error) {
	if m.searchFunc != nil {
		return m.searchFunc(query)
	}
	
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	var results []*types.LogEntry
	for i := range m.storedLogs {
		results = append(results, &m.storedLogs[i])
	}
	return results, nil
}

func (m *MockStorage) GetRecent(limit int) ([]*types.LogEntry, error) {
	if m.getRecentFunc != nil {
		return m.getRecentFunc(limit)
	}
	
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	var results []*types.LogEntry
	start := len(m.storedLogs) - limit
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(m.storedLogs); i++ {
		results = append(results, &m.storedLogs[i])
	}
	return results, nil
}

func (m *MockStorage) Cleanup(retentionDays int) error {
	if m.cleanupFunc != nil {
		return m.cleanupFunc(retentionDays)
	}
	return nil
}

func (m *MockStorage) Close() error {
	return nil
}

func (m *MockStorage) GetStoredLogs() []types.LogEntry {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	logs := make([]types.LogEntry, len(m.storedLogs))
	copy(logs, m.storedLogs)
	return logs
}

func TestNewLogService(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	
	service := NewLogService(parser, storage)
	
	if service == nil {
		t.Fatal("NewLogService returned nil")
	}
	
	if service.parser != parser {
		t.Error("Parser not set correctly")
	}
	
	if service.storage != storage {
		t.Error("Storage not set correctly")
	}
	
	if service.batchSize != DefaultBatchSize {
		t.Errorf("Expected batch size %d, got %d", DefaultBatchSize, service.batchSize)
	}
	
	if service.batchTimeout != DefaultBatchTimeout {
		t.Errorf("Expected batch timeout %v, got %v", DefaultBatchTimeout, service.batchTimeout)
	}
	
	stats := service.GetStats()
	if stats.IsRunning {
		t.Error("Service should not be running initially")
	}
}

func TestLogService_StartStop(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Test starting the service
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	
	stats := service.GetStats()
	if !stats.IsRunning {
		t.Error("Service should be running after start")
	}
	
	// Test starting already running service
	err = service.Start()
	if err == nil {
		t.Error("Expected error when starting already running service")
	}
	
	// Test stopping the service
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}
	
	stats = service.GetStats()
	if stats.IsRunning {
		t.Error("Service should not be running after stop")
	}
	
	// Test stopping already stopped service
	err = service.Stop()
	if err != nil {
		t.Errorf("Unexpected error when stopping already stopped service: %v", err)
	}
}

func TestLogService_ProcessLog(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Configure for faster testing
	service.SetBatchSize(1) // Process immediately
	service.SetBatchTimeout(10 * time.Millisecond)
	
	// Test processing log when service is not running
	err := service.ProcessLog("test message")
	if err == nil {
		t.Error("Expected error when processing log with stopped service")
	}
	
	// Start the service
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	// Test processing a log message
	err = service.ProcessLog("test message")
	if err != nil {
		t.Errorf("Failed to process log: %v", err)
	}
	
	// Wait for batch processing
	time.Sleep(100 * time.Millisecond)
	
	// Check if the log was stored
	storedLogs := storage.GetStoredLogs()
	if len(storedLogs) == 0 {
		t.Error("No logs were stored")
		return
	}
	
	if storedLogs[0].Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", storedLogs[0].Message)
	}
}

func TestLogService_ProcessLogBatch(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Configure for faster testing
	service.SetBatchSize(1) // Process immediately
	service.SetBatchTimeout(10 * time.Millisecond)
	
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	messages := []string{"message1", "message2", "message3"}
	err = service.ProcessLogBatch(messages)
	if err != nil {
		t.Errorf("Failed to process log batch: %v", err)
	}
	
	// Wait for batch processing
	time.Sleep(100 * time.Millisecond)
	
	storedLogs := storage.GetStoredLogs()
	if len(storedLogs) != len(messages) {
		t.Errorf("Expected %d logs, got %d", len(messages), len(storedLogs))
	}
}

func TestLogService_BatchProcessing(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Set small batch size for testing
	service.SetBatchSize(2)
	service.SetBatchTimeout(50 * time.Millisecond)
	
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	// Send messages that should trigger batch processing
	service.ProcessLog("message1")
	service.ProcessLog("message2") // This should trigger batch processing
	
	// Wait for batch processing
	time.Sleep(200 * time.Millisecond)
	
	storedLogs := storage.GetStoredLogs()
	if len(storedLogs) != 2 {
		t.Errorf("Expected 2 logs after batch processing, got %d", len(storedLogs))
	}
	
	// Test timeout-based batch processing
	service.ProcessLog("message3")
	
	// Wait for timeout
	time.Sleep(200 * time.Millisecond)
	
	storedLogs = storage.GetStoredLogs()
	if len(storedLogs) != 3 {
		t.Errorf("Expected 3 logs after timeout batch processing, got %d", len(storedLogs))
	}
}

func TestLogService_Backpressure(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Set very small queue size to test backpressure
	service.SetQueueSize(2)
	service.logQueue = make(chan string, 2)
	
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	// Fill the queue
	service.ProcessLog("message1")
	service.ProcessLog("message2")
	
	// This should trigger backpressure
	err = service.ProcessLog("message3")
	if err == nil {
		t.Error("Expected backpressure error when queue is full")
	}
	
	stats := service.GetStats()
	if stats.FailedLogs == 0 {
		t.Error("Expected failed logs count to increase due to backpressure")
	}
}

func TestLogService_Subscriptions(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Configure for faster testing
	service.SetBatchSize(1) // Process immediately
	service.SetBatchTimeout(10 * time.Millisecond)
	
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	// Test subscription
	subscription := service.Subscribe()
	if subscription == nil {
		t.Fatal("Subscribe returned nil channel")
	}
	
	stats := service.GetStats()
	if stats.ActiveSubscribers != 1 {
		t.Errorf("Expected 1 active subscriber, got %d", stats.ActiveSubscribers)
	}
	
	// Process a log and check if subscriber receives it
	go func() {
		service.ProcessLog("test message")
	}()
	
	select {
	case logEntry := <-subscription:
		if logEntry.Message != "test message" {
			t.Errorf("Expected message 'test message', got '%s'", logEntry.Message)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for log entry from subscription")
	}
	
	// Test unsubscribe
	service.Unsubscribe(subscription)
	
	stats = service.GetStats()
	if stats.ActiveSubscribers != 0 {
		t.Errorf("Expected 0 active subscribers after unsubscribe, got %d", stats.ActiveSubscribers)
	}
}

func TestLogService_MaxSubscribers(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	// Create maximum number of subscribers
	var subscriptions []<-chan *types.LogEntry
	for i := 0; i < MaxSubscribers; i++ {
		sub := service.Subscribe()
		subscriptions = append(subscriptions, sub)
	}
	
	stats := service.GetStats()
	if stats.ActiveSubscribers != MaxSubscribers {
		t.Errorf("Expected %d active subscribers, got %d", MaxSubscribers, stats.ActiveSubscribers)
	}
	
	// Try to create one more subscriber (should fail)
	failedSub := service.Subscribe()
	
	// Check if the channel is closed (indicating failure)
	select {
	case _, ok := <-failedSub:
		if ok {
			t.Error("Expected failed subscription to return closed channel")
		}
	default:
		t.Error("Expected failed subscription to return closed channel")
	}
	
	// Clean up subscriptions
	for _, sub := range subscriptions {
		service.Unsubscribe(sub)
	}
}

func TestLogService_ErrorHandling(t *testing.T) {
	// Test parser error handling
	parser := &MockParser{
		parseFunc: func(msg string) (*types.LogEntry, error) {
			if msg == "error" {
				return nil, fmt.Errorf("parse error")
			}
			entry := &types.LogEntry{
				Version:   1,
				Timestamp: time.Now(),
				Hostname:  "test-host",
				AppName:   "test-app",
				ProcID:    "123",
				MsgID:     "test",
				Message:   msg,
				CreatedAt: time.Now(),
			}
			entry.SetPriority(134) // facility 16, severity 6
			return entry, nil
		},
	}
	
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Configure for faster testing
	service.SetBatchSize(1) // Process immediately
	service.SetBatchTimeout(10 * time.Millisecond)
	
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	// Process a message that will cause a parse error
	service.ProcessLog("error")
	
	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	
	stats := service.GetStats()
	if stats.FailedLogs == 0 {
		t.Error("Expected failed logs count to increase due to parse error")
	}
	
	// Test storage error handling
	storage.storeFunc = func(entry *types.LogEntry) error {
		if entry.Message == "storage_error" {
			return fmt.Errorf("storage error")
		}
		return nil
	}
	
	service.ProcessLog("storage_error")
	
	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	
	stats = service.GetStats()
	if stats.FailedLogs < 2 {
		t.Error("Expected failed logs count to increase due to storage error")
	}
}

func TestLogService_ConcurrentProcessing(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()
	
	// Process logs concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 10
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				message := fmt.Sprintf("message_%d_%d", id, j)
				service.ProcessLog(message)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Wait for all messages to be processed
	time.Sleep(500 * time.Millisecond)
	
	storedLogs := storage.GetStoredLogs()
	expectedCount := numGoroutines * messagesPerGoroutine
	if len(storedLogs) != expectedCount {
		t.Errorf("Expected %d logs, got %d", expectedCount, len(storedLogs))
	}
	
	stats := service.GetStats()
	if stats.ProcessedLogs != int64(expectedCount) {
		t.Errorf("Expected %d processed logs in stats, got %d", expectedCount, stats.ProcessedLogs)
	}
}

func TestLogService_SearchAndGetRecent(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Test Search
	expectedLogs := []*types.LogEntry{
		{ID: 1, Message: "test1"},
		{ID: 2, Message: "test2"},
	}
	
	storage.searchFunc = func(query types.SearchQuery) ([]*types.LogEntry, error) {
		return expectedLogs, nil
	}
	
	query := types.SearchQuery{Text: "test"}
	results, err := service.Search(query)
	if err != nil {
		t.Errorf("Search failed: %v", err)
	}
	
	if len(results) != len(expectedLogs) {
		t.Errorf("Expected %d results, got %d", len(expectedLogs), len(results))
	}
	
	// Test GetRecent
	storage.getRecentFunc = func(limit int) ([]*types.LogEntry, error) {
		return expectedLogs[:limit], nil
	}
	
	results, err = service.GetRecent(1)
	if err != nil {
		t.Errorf("GetRecent failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestLogService_SearchWithRFC5424Filters(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Test RFC5424 field filtering
	expectedLogs := []*types.LogEntry{
		{
			ID:       1,
			Priority: 134, // facility 16, severity 6
			Facility: 16,
			Severity: 6,
			Hostname: "web01",
			AppName:  "nginx",
			ProcID:   "1234",
			MsgID:    "access",
			Message:  "test message",
		},
	}
	
	var capturedQuery types.SearchQuery
	storage.searchFunc = func(query types.SearchQuery) ([]*types.LogEntry, error) {
		capturedQuery = query
		return expectedLogs, nil
	}
	
	// Test search with RFC5424 filters
	facility := 16
	severity := 6
	minSeverity := 4
	query := types.SearchQuery{
		Facility:            &facility,
		Severity:            &severity,
		MinSeverity:         &minSeverity,
		Hostname:            "web01",
		AppName:             "nginx",
		ProcID:              "1234",
		MsgID:               "access",
		StructuredDataQuery: "test-data",
	}
	
	results, err := service.Search(query)
	if err != nil {
		t.Errorf("Search with RFC5424 filters failed: %v", err)
	}
	
	if len(results) != len(expectedLogs) {
		t.Errorf("Expected %d results, got %d", len(expectedLogs), len(results))
	}
	
	// Verify that the query was passed correctly to storage
	if capturedQuery.Facility == nil || *capturedQuery.Facility != facility {
		t.Errorf("Expected facility %d, got %v", facility, capturedQuery.Facility)
	}
	
	if capturedQuery.Severity == nil || *capturedQuery.Severity != severity {
		t.Errorf("Expected severity %d, got %v", severity, capturedQuery.Severity)
	}
	
	if capturedQuery.MinSeverity == nil || *capturedQuery.MinSeverity != minSeverity {
		t.Errorf("Expected min_severity %d, got %v", minSeverity, capturedQuery.MinSeverity)
	}
	
	if capturedQuery.Hostname != "web01" {
		t.Errorf("Expected hostname 'web01', got '%s'", capturedQuery.Hostname)
	}
	
	if capturedQuery.AppName != "nginx" {
		t.Errorf("Expected app_name 'nginx', got '%s'", capturedQuery.AppName)
	}
	
	if capturedQuery.ProcID != "1234" {
		t.Errorf("Expected proc_id '1234', got '%s'", capturedQuery.ProcID)
	}
	
	if capturedQuery.MsgID != "access" {
		t.Errorf("Expected msg_id 'access', got '%s'", capturedQuery.MsgID)
	}
	
	if capturedQuery.StructuredDataQuery != "test-data" {
		t.Errorf("Expected structured_data_query 'test-data', got '%s'", capturedQuery.StructuredDataQuery)
	}
}

func TestLogService_Configuration(t *testing.T) {
	parser := &MockParser{}
	storage := &MockStorage{}
	service := NewLogService(parser, storage)
	
	// Test SetBatchSize
	service.SetBatchSize(50)
	if service.batchSize != 50 {
		t.Errorf("Expected batch size 50, got %d", service.batchSize)
	}
	
	// Test invalid batch size
	service.SetBatchSize(0)
	if service.batchSize != 50 {
		t.Error("Batch size should not change for invalid value")
	}
	
	// Test SetBatchTimeout
	timeout := 2 * time.Second
	service.SetBatchTimeout(timeout)
	if service.batchTimeout != timeout {
		t.Errorf("Expected batch timeout %v, got %v", timeout, service.batchTimeout)
	}
	
	// Test invalid batch timeout
	service.SetBatchTimeout(0)
	if service.batchTimeout != timeout {
		t.Error("Batch timeout should not change for invalid value")
	}
	
	// Test SetQueueSize
	service.SetQueueSize(5000)
	if service.queueSize != 5000 {
		t.Errorf("Expected queue size 5000, got %d", service.queueSize)
	}
	
	// Test invalid queue size
	service.SetQueueSize(-1)
	if service.queueSize != 5000 {
		t.Error("Queue size should not change for invalid value")
	}
}