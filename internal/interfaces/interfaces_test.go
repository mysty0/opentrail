package interfaces

import (
	"opentrail/internal/types"
	"testing"
	"time"
)

// MockLogStorage is a test implementation of LogStorage interface
type MockLogStorage struct {
	entries []*types.LogEntry
}

func (m *MockLogStorage) Store(entry *types.LogEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *MockLogStorage) Search(query types.SearchQuery) ([]*types.LogEntry, error) {
	return m.entries, nil
}

func (m *MockLogStorage) GetRecent(limit int) ([]*types.LogEntry, error) {
	if limit > len(m.entries) {
		limit = len(m.entries)
	}
	return m.entries[:limit], nil
}

func (m *MockLogStorage) Cleanup(retentionDays int) error {
	return nil
}

func (m *MockLogStorage) Close() error {
	return nil
}

// MockLogParser is a test implementation of LogParser interface
type MockLogParser struct {
	format string
}

func (m *MockLogParser) Parse(rawMessage string) (*types.LogEntry, error) {
	return &types.LogEntry{
		ID:         1,
		Timestamp:  time.Now(),
		Level:      "INFO",
		TrackingID: "test",
		Message:    rawMessage,
	}, nil
}

func (m *MockLogParser) SetFormat(format string) error {
	m.format = format
	return nil
}

// TestLogStorageInterface verifies that MockLogStorage implements LogStorage
func TestLogStorageInterface(t *testing.T) {
	var storage LogStorage = &MockLogStorage{}

	// Test Store
	entry := &types.LogEntry{
		ID:         1,
		Timestamp:  time.Now(),
		Level:      "INFO",
		TrackingID: "test-123",
		Message:    "Test message",
	}

	err := storage.Store(entry)
	if err != nil {
		t.Errorf("Store failed: %v", err)
	}

	// Test GetRecent
	recent, err := storage.GetRecent(10)
	if err != nil {
		t.Errorf("GetRecent failed: %v", err)
	}
	if len(recent) != 1 {
		t.Errorf("Expected 1 recent entry, got %d", len(recent))
	}

	// Test Search
	query := types.SearchQuery{
		Text:  "test",
		Limit: 10,
	}
	results, err := storage.Search(query)
	if err != nil {
		t.Errorf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(results))
	}

	// Test Cleanup
	err = storage.Cleanup(30)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

// TestLogParserInterface verifies that MockLogParser implements LogParser
func TestLogParserInterface(t *testing.T) {
	var parser LogParser = &MockLogParser{}

	// Test SetFormat
	err := parser.SetFormat("{{timestamp}}|{{level}}|{{message}}")
	if err != nil {
		t.Errorf("SetFormat failed: %v", err)
	}

	// Test Parse
	entry, err := parser.Parse("2024-01-01T12:00:00Z|INFO|Test message")
	if err != nil {
		t.Errorf("Parse failed: %v", err)
	}
	if entry == nil {
		t.Error("Parse returned nil entry")
	}
	if entry.Message != "2024-01-01T12:00:00Z|INFO|Test message" {
		t.Errorf("Expected message to be raw input, got %s", entry.Message)
	}
}

// TestInterfaceContracts ensures our interfaces can be used polymorphically
func TestInterfaceContracts(t *testing.T) {
	// Test that we can assign concrete types to interface variables
	var storage LogStorage
	var parser LogParser

	storage = &MockLogStorage{}
	parser = &MockLogParser{}

	// Test that interface methods can be called
	entry := &types.LogEntry{
		ID:         1,
		Timestamp:  time.Now(),
		Level:      "DEBUG",
		TrackingID: "contract-test",
		Message:    "Interface contract test",
	}

	err := storage.Store(entry)
	if err != nil {
		t.Errorf("Interface contract failed for storage.Store: %v", err)
	}

	parsedEntry, err := parser.Parse("test message")
	if err != nil {
		t.Errorf("Interface contract failed for parser.Parse: %v", err)
	}
	if parsedEntry == nil {
		t.Error("Interface contract failed: parser.Parse returned nil")
	}
}