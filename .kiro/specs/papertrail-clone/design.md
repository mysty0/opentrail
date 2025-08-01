# Design Document

## Overview

The Papertrail clone will be implemented as a single Go binary that combines three main components: a TCP log ingestion server, an SQLite-based storage layer with FTS5 full-text search, and a web-based terminal-like UI. The architecture follows a clean separation of concerns with concurrent goroutines handling different aspects of the system.

## Architecture

The system uses a layered architecture with the following components:

```
┌─────────────────┐    ┌─────────────────┐
│   TCP Server    │    │   Web Server    │
│  (Log Ingestion)│    │     (UI)        │
└─────────┬───────┘    └─────────┬───────┘
          │                      │
          └──────────┬───────────┘
                     │
          ┌─────────────────┐
          │  Log Service    │
          │  (Business Logic)│
          └─────────┬───────┘
                    │
          ┌─────────────────┐
          │  Storage Layer  │
          │ (SQLite + FTS5) │
          └─────────────────┘
```

The application will use goroutines for concurrent processing:
- Main goroutine: Application lifecycle and coordination
- TCP server goroutine: Accept and handle incoming log connections
- Web server goroutine: Serve HTTP requests and WebSocket connections
- Log processor goroutines: Parse and store incoming log messages
- Database writer goroutine: Batch write operations for performance

## Components and Interfaces

### 1. Configuration Management

```go
type Config struct {
    TCPPort        int
    HTTPPort       int
    DatabasePath   string
    LogFormat      string
    RetentionDays  int
    MaxConnections int
    AuthUsername   string
    AuthPassword   string
    AuthEnabled    bool
}
```

Configuration will be loaded from command-line flags with environment variable fallbacks.

### 2. Log Model

```go
type LogEntry struct {
    ID         int64     `json:"id"`
    Timestamp  time.Time `json:"timestamp"`
    Level      string    `json:"level"`
    TrackingID string    `json:"tracking_id"`
    Message    string    `json:"message"`
}
```

### 3. Storage Interface

```go
type LogStorage interface {
    Store(entry *LogEntry) error
    Search(query SearchQuery) ([]*LogEntry, error)
    GetRecent(limit int) ([]*LogEntry, error)
    Cleanup(retentionDays int) error
}

type SearchQuery struct {
    Text       string
    Level      string
    TrackingID string
    StartTime  *time.Time
    EndTime    *time.Time
    Limit      int
    Offset     int
}
```

### 4. Log Parser Interface

```go
type LogParser interface {
    Parse(rawMessage string) (*LogEntry, error)
    SetFormat(format string) error
}
```

### 5. TCP Server Component

The TCP server will:
- Listen on the configured port
- Accept multiple concurrent connections
- Read newline-delimited log messages
- Pass messages to the log service for processing
- Handle connection cleanup and error recovery

### 6. Web Server Component

The web server will provide:
- Static file serving for the UI assets
- REST API endpoints for log querying
- WebSocket endpoint for real-time log streaming
- Health check endpoint
- Basic authentication middleware (when enabled)

API Endpoints:
- `GET /api/logs` - Query logs with filtering parameters (requires auth if enabled)
- `GET /api/logs/stream` - WebSocket for real-time log updates (requires auth if enabled)
- `GET /api/health` - Health check endpoint (no auth required)
- `GET /` - Serve the main UI (requires auth if enabled)

Authentication:
- When `AuthEnabled` is true, all UI and API endpoints except `/api/health` require HTTP Basic Authentication
- Credentials are configured via `AuthUsername` and `AuthPassword` in the config
- WebSocket connections will authenticate using the `Authorization` header
- Failed authentication attempts return 401 Unauthorized

### 7. Log Service Component

The central business logic component that:
- Coordinates between TCP server and storage
- Manages log parsing and validation
- Handles batch processing for performance
- Implements backpressure mechanisms
- Manages real-time subscriptions for the UI

## Data Models

### Database Schema

```sql
-- Main logs table
CREATE TABLE logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    level TEXT NOT NULL,
    tracking_id TEXT,
    message TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- FTS5 virtual table for full-text search
CREATE VIRTUAL TABLE logs_fts USING fts5(
    message,
    content='logs',
    content_rowid='id'
);

-- Indexes for efficient querying
CREATE INDEX idx_logs_timestamp ON logs(timestamp);
CREATE INDEX idx_logs_level ON logs(level);
CREATE INDEX idx_logs_tracking_id ON logs(tracking_id);
CREATE INDEX idx_logs_created_at ON logs(created_at);

-- Triggers to keep FTS5 table in sync
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

### Log Format Parsing

Default format: `timestamp|level|tracking_id|message`

The parser will support configurable formats using Go's template-like syntax:
- `{{timestamp}}` - ISO 8601 timestamp
- `{{level}}` - Log level (DEBUG, INFO, WARN, ERROR, FATAL)
- `{{tracking_id}}` - User/session/request identifier
- `{{message}}` - The actual log message

Example formats:
- `{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}`
- `[{{timestamp}}] {{level}} ({{tracking_id}}): {{message}}`
- `{{level}} {{timestamp}} {{message}}` (no tracking ID)

## Error Handling

### TCP Server Error Handling
- Connection errors: Log and continue accepting new connections
- Parse errors: Store raw message with "PARSE_ERROR" level
- Database errors: Implement retry logic with exponential backoff
- Resource exhaustion: Implement connection limits and backpressure

### Storage Error Handling
- Database lock errors: Retry with exponential backoff
- Disk space errors: Log error and attempt cleanup of old logs
- Corruption errors: Log error and attempt database repair
- Transaction errors: Rollback and retry individual operations

### Web Server Error Handling
- Invalid query parameters: Return 400 with error message
- Database query errors: Return 500 with generic error message
- WebSocket errors: Attempt reconnection with exponential backoff
- Rate limiting: Implement per-client request limits

## Testing Strategy

### Unit Tests
- Log parser with various input formats
- Storage layer with mock database
- Configuration loading and validation
- Error handling scenarios
- Search query building and execution

### Integration Tests
- End-to-end log ingestion and retrieval
- TCP server with multiple concurrent connections
- Web API with various query parameters
- Database schema creation and migration
- Real-time WebSocket streaming

### Performance Tests
- High-volume log ingestion (1000+ logs/second)
- Large database query performance
- Memory usage under sustained load
- Connection handling limits
- FTS5 search performance with large datasets

### Manual Testing
- UI responsiveness and usability
- Real-time log streaming accuracy
- Filter combinations and edge cases
- Browser compatibility
- Mobile device compatibility

## Implementation Considerations

### Performance Optimizations
- Batch database writes to reduce I/O
- Connection pooling for database operations
- Efficient JSON serialization for API responses
- Gzip compression for HTTP responses
- WebSocket message batching for high-frequency updates

### Security Considerations
- Input validation and sanitization
- SQL injection prevention through prepared statements
- Rate limiting on API endpoints
- CORS configuration for web interface
- HTTP Basic Authentication for web interface (configurable)
- Secure credential storage and validation
- Protection against timing attacks in authentication
- HTTPS support recommendation for production deployments

### Deployment Considerations
- Single binary with embedded static assets
- Graceful shutdown handling
- Signal handling for log rotation
- Configuration validation on startup
- Health check endpoint for monitoring

### Scalability Considerations
- SQLite limitations for concurrent writes
- Memory usage monitoring and limits
- Log retention and cleanup strategies
- Database vacuum operations
- Connection limits and backpressure mechanisms