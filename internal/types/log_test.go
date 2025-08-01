package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLogEntry_JSONSerialization(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	entry := &LogEntry{
		ID:         123,
		Timestamp:  timestamp,
		Level:      "INFO",
		TrackingID: "user-456",
		Message:    "Test log message",
	}

	// Test JSON marshaling
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal LogEntry: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled LogEntry
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal LogEntry: %v", err)
	}

	// Verify fields
	if unmarshaled.ID != entry.ID {
		t.Errorf("Expected ID %d, got %d", entry.ID, unmarshaled.ID)
	}
	if !unmarshaled.Timestamp.Equal(entry.Timestamp) {
		t.Errorf("Expected timestamp %v, got %v", entry.Timestamp, unmarshaled.Timestamp)
	}
	if unmarshaled.Level != entry.Level {
		t.Errorf("Expected level %s, got %s", entry.Level, unmarshaled.Level)
	}
	if unmarshaled.TrackingID != entry.TrackingID {
		t.Errorf("Expected tracking ID %s, got %s", entry.TrackingID, unmarshaled.TrackingID)
	}
	if unmarshaled.Message != entry.Message {
		t.Errorf("Expected message %s, got %s", entry.Message, unmarshaled.Message)
	}
}

func TestSearchQuery_JSONSerialization(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	
	query := &SearchQuery{
		Text:       "error",
		Level:      "ERROR",
		TrackingID: "user-123",
		StartTime:  &startTime,
		EndTime:    &endTime,
		Limit:      100,
		Offset:     0,
	}

	// Test JSON marshaling
	data, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal SearchQuery: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled SearchQuery
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SearchQuery: %v", err)
	}

	// Verify fields
	if unmarshaled.Text != query.Text {
		t.Errorf("Expected text %s, got %s", query.Text, unmarshaled.Text)
	}
	if unmarshaled.Level != query.Level {
		t.Errorf("Expected level %s, got %s", query.Level, unmarshaled.Level)
	}
	if unmarshaled.TrackingID != query.TrackingID {
		t.Errorf("Expected tracking ID %s, got %s", query.TrackingID, unmarshaled.TrackingID)
	}
	if unmarshaled.StartTime == nil || !unmarshaled.StartTime.Equal(*query.StartTime) {
		t.Errorf("Expected start time %v, got %v", query.StartTime, unmarshaled.StartTime)
	}
	if unmarshaled.EndTime == nil || !unmarshaled.EndTime.Equal(*query.EndTime) {
		t.Errorf("Expected end time %v, got %v", query.EndTime, unmarshaled.EndTime)
	}
	if unmarshaled.Limit != query.Limit {
		t.Errorf("Expected limit %d, got %d", query.Limit, unmarshaled.Limit)
	}
	if unmarshaled.Offset != query.Offset {
		t.Errorf("Expected offset %d, got %d", query.Offset, unmarshaled.Offset)
	}
}

func TestSearchQuery_EmptyFields(t *testing.T) {
	query := &SearchQuery{
		Text:  "test",
		Limit: 50,
	}

	data, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal SearchQuery: %v", err)
	}

	var unmarshaled SearchQuery
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SearchQuery: %v", err)
	}

	if unmarshaled.Text != "test" {
		t.Errorf("Expected text 'test', got %s", unmarshaled.Text)
	}
	if unmarshaled.Level != "" {
		t.Errorf("Expected empty level, got %s", unmarshaled.Level)
	}
	if unmarshaled.StartTime != nil {
		t.Errorf("Expected nil start time, got %v", unmarshaled.StartTime)
	}
	if unmarshaled.Limit != 50 {
		t.Errorf("Expected limit 50, got %d", unmarshaled.Limit)
	}
}