package types

import "time"

// LogEntry represents a single RFC5424 log entry in the system
type LogEntry struct {
	ID            int64                  `json:"id"`
	
	// RFC5424 Header Fields
	Priority      int                    `json:"priority"`      // PRI field (facility * 8 + severity)
	Facility      int                    `json:"facility"`      // Extracted from priority
	Severity      int                    `json:"severity"`      // Extracted from priority
	Version       int                    `json:"version"`       // Always 1 for RFC5424
	Timestamp     time.Time              `json:"timestamp"`     // RFC3339 format
	Hostname      string                 `json:"hostname"`      // Source hostname
	AppName       string                 `json:"app_name"`      // Application name
	ProcID        string                 `json:"proc_id"`       // Process ID
	MsgID         string                 `json:"msg_id"`        // Message ID
	
	// Structured Data and Message
	StructuredData map[string]interface{} `json:"structured_data"` // JSON representation
	Message       string                 `json:"message"`       // The actual log message
	
	// System Fields
	CreatedAt     time.Time              `json:"created_at"`    // When stored in DB
}

// GetFacility extracts facility from priority field
func (l *LogEntry) GetFacility() int {
	return l.Priority >> 3
}

// GetSeverity extracts severity from priority field
func (l *LogEntry) GetSeverity() int {
	return l.Priority & 7
}

// SetPriority sets priority and updates facility/severity fields
func (l *LogEntry) SetPriority(priority int) {
	l.Priority = priority
	l.Facility = l.GetFacility()
	l.Severity = l.GetSeverity()
}

// SearchQuery represents parameters for searching RFC5424 logs
type SearchQuery struct {
	// Text search
	Text          string     `json:"text,omitempty"`
	
	// RFC5424 specific filters
	Facility      *int       `json:"facility,omitempty"`
	Severity      *int       `json:"severity,omitempty"`
	MinSeverity   *int       `json:"min_severity,omitempty"`
	Hostname      string     `json:"hostname,omitempty"`
	AppName       string     `json:"app_name,omitempty"`
	ProcID        string     `json:"proc_id,omitempty"`
	MsgID         string     `json:"msg_id,omitempty"`
	
	// Structured data queries (JSON path)
	StructuredDataQuery string `json:"structured_data_query,omitempty"`
	
	// Time range
	StartTime     *time.Time `json:"start_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	
	// Pagination
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
}