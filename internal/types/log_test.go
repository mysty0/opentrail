package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLogEntry_JSONSerialization(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC)
	structuredData := map[string]interface{}{
		"exampleSDID@32473": map[string]string{
			"iut":         "3",
			"eventSource": "Application",
			"eventID":     "1011",
		},
	}
	
	entry := &LogEntry{
		ID:             123,
		Priority:       134, // facility 16, severity 6
		Facility:       16,
		Severity:       6,
		Version:        1,
		Timestamp:      timestamp,
		Hostname:       "web01",
		AppName:        "nginx",
		ProcID:         "1234",
		MsgID:          "access",
		StructuredData: structuredData,
		Message:        "Test log message",
		CreatedAt:      createdAt,
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
	if unmarshaled.Priority != entry.Priority {
		t.Errorf("Expected priority %d, got %d", entry.Priority, unmarshaled.Priority)
	}
	if unmarshaled.Facility != entry.Facility {
		t.Errorf("Expected facility %d, got %d", entry.Facility, unmarshaled.Facility)
	}
	if unmarshaled.Severity != entry.Severity {
		t.Errorf("Expected severity %d, got %d", entry.Severity, unmarshaled.Severity)
	}
	if unmarshaled.Version != entry.Version {
		t.Errorf("Expected version %d, got %d", entry.Version, unmarshaled.Version)
	}
	if !unmarshaled.Timestamp.Equal(entry.Timestamp) {
		t.Errorf("Expected timestamp %v, got %v", entry.Timestamp, unmarshaled.Timestamp)
	}
	if unmarshaled.Hostname != entry.Hostname {
		t.Errorf("Expected hostname %s, got %s", entry.Hostname, unmarshaled.Hostname)
	}
	if unmarshaled.AppName != entry.AppName {
		t.Errorf("Expected app name %s, got %s", entry.AppName, unmarshaled.AppName)
	}
	if unmarshaled.ProcID != entry.ProcID {
		t.Errorf("Expected proc ID %s, got %s", entry.ProcID, unmarshaled.ProcID)
	}
	if unmarshaled.MsgID != entry.MsgID {
		t.Errorf("Expected msg ID %s, got %s", entry.MsgID, unmarshaled.MsgID)
	}
	if unmarshaled.Message != entry.Message {
		t.Errorf("Expected message %s, got %s", entry.Message, unmarshaled.Message)
	}
	if !unmarshaled.CreatedAt.Equal(entry.CreatedAt) {
		t.Errorf("Expected created at %v, got %v", entry.CreatedAt, unmarshaled.CreatedAt)
	}
}

func TestSearchQuery_JSONSerialization(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	facility := 16
	severity := 3
	minSeverity := 6
	
	query := &SearchQuery{
		Text:                "error",
		Facility:            &facility,
		Severity:            &severity,
		MinSeverity:         &minSeverity,
		Hostname:            "web01",
		AppName:             "nginx",
		ProcID:              "1234",
		MsgID:               "access",
		StructuredDataQuery: "eventID",
		StartTime:           &startTime,
		EndTime:             &endTime,
		Limit:               100,
		Offset:              0,
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
	if unmarshaled.Facility == nil || *unmarshaled.Facility != *query.Facility {
		t.Errorf("Expected facility %v, got %v", query.Facility, unmarshaled.Facility)
	}
	if unmarshaled.Severity == nil || *unmarshaled.Severity != *query.Severity {
		t.Errorf("Expected severity %v, got %v", query.Severity, unmarshaled.Severity)
	}
	if unmarshaled.MinSeverity == nil || *unmarshaled.MinSeverity != *query.MinSeverity {
		t.Errorf("Expected min severity %v, got %v", query.MinSeverity, unmarshaled.MinSeverity)
	}
	if unmarshaled.Hostname != query.Hostname {
		t.Errorf("Expected hostname %s, got %s", query.Hostname, unmarshaled.Hostname)
	}
	if unmarshaled.AppName != query.AppName {
		t.Errorf("Expected app name %s, got %s", query.AppName, unmarshaled.AppName)
	}
	if unmarshaled.ProcID != query.ProcID {
		t.Errorf("Expected proc ID %s, got %s", query.ProcID, unmarshaled.ProcID)
	}
	if unmarshaled.MsgID != query.MsgID {
		t.Errorf("Expected msg ID %s, got %s", query.MsgID, unmarshaled.MsgID)
	}
	if unmarshaled.StructuredDataQuery != query.StructuredDataQuery {
		t.Errorf("Expected structured data query %s, got %s", query.StructuredDataQuery, unmarshaled.StructuredDataQuery)
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
	if unmarshaled.Facility != nil {
		t.Errorf("Expected nil facility, got %v", unmarshaled.Facility)
	}
	if unmarshaled.Hostname != "" {
		t.Errorf("Expected empty hostname, got %s", unmarshaled.Hostname)
	}
	if unmarshaled.StartTime != nil {
		t.Errorf("Expected nil start time, got %v", unmarshaled.StartTime)
	}
	if unmarshaled.Limit != 50 {
		t.Errorf("Expected limit 50, got %d", unmarshaled.Limit)
	}
}

func TestLogEntry_PriorityHelpers(t *testing.T) {
	entry := &LogEntry{}
	
	// Test SetPriority
	entry.SetPriority(134) // facility 16, severity 6
	
	if entry.Priority != 134 {
		t.Errorf("Expected priority 134, got %d", entry.Priority)
	}
	if entry.Facility != 16 {
		t.Errorf("Expected facility 16, got %d", entry.Facility)
	}
	if entry.Severity != 6 {
		t.Errorf("Expected severity 6, got %d", entry.Severity)
	}
	
	// Test GetFacility
	if entry.GetFacility() != 16 {
		t.Errorf("Expected GetFacility() to return 16, got %d", entry.GetFacility())
	}
	
	// Test GetSeverity
	if entry.GetSeverity() != 6 {
		t.Errorf("Expected GetSeverity() to return 6, got %d", entry.GetSeverity())
	}
	
	// Test different priority values
	testCases := []struct {
		priority int
		facility int
		severity int
	}{
		{0, 0, 0},     // facility 0, severity 0
		{7, 0, 7},     // facility 0, severity 7
		{8, 1, 0},     // facility 1, severity 0
		{15, 1, 7},    // facility 1, severity 7
		{191, 23, 7},  // facility 23, severity 7
	}
	
	for _, tc := range testCases {
		entry.SetPriority(tc.priority)
		if entry.GetFacility() != tc.facility {
			t.Errorf("Priority %d: expected facility %d, got %d", tc.priority, tc.facility, entry.GetFacility())
		}
		if entry.GetSeverity() != tc.severity {
			t.Errorf("Priority %d: expected severity %d, got %d", tc.priority, tc.severity, entry.GetSeverity())
		}
	}
}