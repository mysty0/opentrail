package types

import "time"

// LogEntry represents a single log entry in the system
type LogEntry struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Level      string    `json:"level"`
	TrackingID string    `json:"tracking_id"`
	Message    string    `json:"message"`
}

// SearchQuery represents parameters for searching logs
type SearchQuery struct {
	Text       string     `json:"text,omitempty"`
	Level      string     `json:"level,omitempty"`
	TrackingID string     `json:"tracking_id,omitempty"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}