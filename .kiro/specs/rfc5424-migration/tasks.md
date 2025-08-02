# Implementation Plan

- [x] 1. Replace LogEntry type with RFC5424 fields





  - Replace internal/types/log.go LogEntry struct with all RFC5424 fields (priority, facility, severity, version, hostname, app_name, proc_id, msg_id, structured_data)
  - Add helper methods for facility/severity extraction from priority
  - Update all tests to use new LogEntry structure
  - Replace entire internal/parser/parser.go with RFC5424Parser implementation
  - Remove all legacy parser code (DefaultLogParser, RFC3164LogParser)
  - Implement RFC5424 message parsing with structured data support
  - Update all parser tests for RFC5424 format only
  - Replace internal/storage/sqlite.go schema with RFC5424 fields and indexes
  - Remove old database file on startup (hard reset)
  - Update Store and Search methods for RFC5424 fields
  - Update all storage tests for new schema

- [x] 4. Update SearchQuery and API for RFC5424 filters





  - Add RFC5424 filter fields to SearchQuery in internal/types/log.go
  - Update HTTP API handlers in internal/server/http.go for RFC5424 query parameters
  - Update service layer in internal/service/service.go for RFC5424 processing
  - Update all API and service tests
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 5. Update web UI for RFC5424 display and filtering





  - Update HTML template and CSS to display all RFC5424 fields
  - Add filter controls for RFC5424 fields in web interface
  - Update JavaScript for RFC5424 filtering and WebSocket streaming
  - Update UI tests for RFC5424 functionality
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [ ] 6. Clean up configuration and add integration tests
  - Remove legacy format options from internal/config/config.go
  - Add end-to-end integration tests for complete RFC5424 flow
  - Update all remaining tests and remove legacy format references
  - _Requirements: 1.2, 1.3, 5.4_