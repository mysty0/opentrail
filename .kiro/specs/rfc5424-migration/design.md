# Design Document

## Overview

The RFC5424 migration transforms the log collection system from a multi-format parser to a dedicated RFC5424 syslog protocol implementation. This design eliminates all legacy parsing code and implements a clean, standards-compliant syslog receiver with enhanced database schema and filtering capabilities. The migration includes a hard database reset to ensure clean implementation without backward compatibility concerns.

## Architecture

The updated architecture maintains the same high-level structure but with RFC5424-specific components:

```
┌─────────────────┐    ┌─────────────────┐
│   TCP Server    │    │   Web Server    │
│ (RFC5424 Only)  │    │  (Enhanced UI)  │
└─────────┬───────┘    └─────────┬───────┘
          │                      │
          └──────────┬───────────┘
                     │
          ┌─────────────────┐
          │  Log Service    │
          │ (RFC5424 Logic) │
          └─────────┬───────┘
                    │
          ┌─────────────────┐
          │  Storage Layer  │
          │(RFC5424 Schema) │
          └─────────────────┘
```

Key architectural changes:
- RFC5424Parser replaces all existing parsers
- Enhanced LogEntry type with all RFC5424 fields
- New database schema with RFC5424-specific columns
- Updated API endpoints supporting RFC5424 field queries
- Enhanced web UI with RFC5424 field display and filtering

## Components and Interfaces

### 1. Enhanced Log Model

```go
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
```

### 2. RFC5424 Parser Interface

```go
type RFC5424Parser struct {
    strictMode bool // Whether to reject malformed messages
}

func NewRFC5424Parser(strictMode bool) *RFC5424Parser
func (p *RFC5424Parser) Parse(rawMessage string) (*LogEntry, error)
func (p *RFC5424Parser) SetFormat(format string) error // No-op for RFC5424, remove this field from the interface
```

### 3. Enhanced Search Query

```go
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
```


## Data Models

### Database Schema

```sql
-- Main RFC5424 logs table
CREATE TABLE logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    
    -- RFC5424 Header Fields
    priority INTEGER NOT NULL,
    facility INTEGER NOT NULL,
    severity INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    timestamp DATETIME NOT NULL,
    hostname TEXT,
    app_name TEXT,
    proc_id TEXT,
    msg_id TEXT,
    
    -- Structured Data and Message
    structured_data TEXT, -- JSON string
    message TEXT NOT NULL,
    
    -- System Fields
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- FTS5 virtual table for full-text search on message
CREATE VIRTUAL TABLE logs_fts USING fts5(
    message,
    content='logs',
    content_rowid='id'
);

-- Indexes for efficient RFC5424 field queries
CREATE INDEX idx_logs_timestamp ON logs(timestamp);
CREATE INDEX idx_logs_facility ON logs(facility);
CREATE INDEX idx_logs_severity ON logs(severity);
CREATE INDEX idx_logs_hostname ON logs(hostname);
CREATE INDEX idx_logs_app_name ON logs(app_name);
CREATE INDEX idx_logs_proc_id ON logs(proc_id);
CREATE INDEX idx_logs_msg_id ON logs(msg_id);
CREATE INDEX idx_logs_priority ON logs(priority);
CREATE INDEX idx_logs_created_at ON logs(created_at);

-- Composite indexes for common query patterns
CREATE INDEX idx_logs_facility_severity ON logs(facility, severity);
CREATE INDEX idx_logs_hostname_app_name ON logs(hostname, app_name);
CREATE INDEX idx_logs_timestamp_severity ON logs(timestamp, severity);

-- FTS5 sync triggers
CREATE TRIGGER logs_ai AFTER INSERT ON logs BEGIN
    INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
END;

CREATE TRIGGER logs_ad AFTER DELETE ON logs BEGIN
    INSERT INTO logs_fts(logs_fts, rowid, message) VALUES('delete', old.id, old.message);
END;

CREATE TRIGGER logs_au AFTER UPDATE ON logs BEGIN
    INSERT INTO logs_fts(logs_fts, rowid, message) VALUES('delete', old.id, old.message);
    INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
END;
```

### RFC5424 Message Format

RFC5424 messages follow this structure:
```
<PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [STRUCTURED-DATA] MSG
```

Example:
```
<165>1 2023-10-15T14:30:45.123Z web01 nginx 1234 access [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"] User login successful
```

Components:
- **PRI**: Priority (facility * 8 + severity)
- **VERSION**: Always "1" for RFC5424
- **TIMESTAMP**: RFC3339 format with optional fractional seconds
- **HOSTNAME**: Source hostname (or "-" if unavailable)
- **APP-NAME**: Application name (or "-" if unavailable)
- **PROCID**: Process ID (or "-" if unavailable)
- **MSGID**: Message type identifier (or "-" if unavailable)
- **STRUCTURED-DATA**: Key-value pairs in [SD-ID@PEN param="value"] format
- **MSG**: The actual log message (optional)

## Error Handling

### RFC5424 Parsing Errors

```go
type RFC5424ParseError struct {
    Field   string
    Value   string
    Message string
}

func (e *RFC5424ParseError) Error() string {
    return fmt.Sprintf("RFC5424 parse error in %s field '%s': %s", e.Field, e.Value, e.Message)
}
```

Error handling strategy:
- **Strict Mode**: Reject malformed messages entirely
- **Lenient Mode**: Accept messages with minor format issues, using defaults
- **Validation Errors**: Log detailed error information for debugging
- **Connection Errors**: Continue accepting new connections after parse failures

### Storage Errors

- **JSON Parsing**: Handle malformed structured data gracefully
- **Field Validation**: Ensure RFC5424 field constraints are met
- **Transaction Failures**: Implement proper rollback and retry logic

## Testing Strategy

### Unit Tests

- RFC5424 parser with valid and invalid messages
- Structured data parsing and JSON conversion
- Database schema creation
- Search query building with RFC5424 fields
- Priority calculation and facility/severity extraction

### Integration Tests

- End-to-end RFC5424 message processing
- API endpoints with RFC5424 field filtering
- WebSocket streaming with enhanced log data
- Multi-field search combinations

### Performance Tests

- High-volume RFC5424 message parsing
- Complex structured data processing
- Database queries with multiple RFC5424 filters
- JSON-based structured data queries
- FTS5 performance with enhanced schema

### Validation Tests

- RFC5424 specification compliance
- Structured data format validation
- Priority field calculation accuracy
- Timestamp parsing with various RFC3339 formats
- Edge cases and malformed message handling

## Implementation Considerations

### RFC5424 Compliance

- **Strict Parsing**: Implement full RFC5424 specification compliance
- **Structured Data**: Support multiple SD-IDs with proper escaping
- **Character Encoding**: Handle UTF-8 encoding properly
- **Field Limits**: Respect RFC5424 field length limitations
- **Timestamp Precision**: Support fractional seconds and timezone handling

### Performance Optimizations

- **Batch Processing**: Group database writes for better performance
- **JSON Indexing**: Consider JSON1 extension for structured data queries
- **Memory Management**: Efficient parsing without excessive allocations
- **Connection Pooling**: Optimize database connection usage
- **Query Optimization**: Use appropriate indexes for RFC5424 field queries

### Security Considerations

- **Input Validation**: Sanitize all RFC5424 fields before storage
- **Structured Data**: Prevent JSON injection attacks
- **Field Length Limits**: Enforce maximum field sizes
- **SQL Injection**: Use parameterized queries for all database operations
- **DoS Protection**: Implement rate limiting for malformed messages

### Migration Safety

- **Validation**: Verify new schema creation before deleting old data
- **Logging**: Comprehensive logging of migration steps
- **Error Recovery**: Handle partial migration failures gracefully

### Web Interface Enhancements

- **Field Display**: Organize RFC5424 fields in logical groups
- **Structured Data**: Expandable JSON viewer for complex data
- **Filter UI**: Intuitive controls for all RFC5424 field filters
- **Performance**: Efficient rendering of additional field data
- **Responsive Design**: Maintain usability with expanded data display