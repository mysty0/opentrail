# Requirements Document

## Introduction

This feature migrates the existing log collection system from supporting multiple formats (custom template format and RFC3164) to exclusively supporting RFC5424 (The Syslog Protocol). The migration includes a complete database schema restructure to accommodate all RFC5424 fields and removes legacy format support. This is a breaking change that requires a hard reset of the database without migration support, prioritizing clean implementation over backward compatibility.

## Requirements

### Requirement 1

**User Story:** As a system administrator, I want the log system to exclusively support RFC5424 format, so that I have a standardized, modern syslog protocol with rich structured data support.

#### Acceptance Criteria

1. WHEN the system starts THEN the system SHALL only accept RFC5424 formatted log messages
2. WHEN a non-RFC5424 message is received THEN the system SHALL reject it with appropriate error logging
3. WHEN the parser is initialized THEN the system SHALL remove all legacy format support (custom template and RFC3164)
4. WHEN the system processes logs THEN the system SHALL extract all RFC5424 fields including priority, version, timestamp, hostname, app-name, procid, msgid, structured-data, and message

### Requirement 2

**User Story:** As a developer, I want the database schema to store all RFC5424 fields natively, so that I can query and filter logs based on any RFC5424 component without parsing.

#### Acceptance Criteria

1. WHEN the database is initialized THEN the system SHALL create a new schema with separate columns for all RFC5424 fields
2. WHEN a log entry is stored THEN the system SHALL populate facility, severity, version, timestamp, hostname, app_name, proc_id, msg_id, structured_data, and message fields
3. WHEN structured data is present THEN the system SHALL store it as JSON for efficient querying
4. WHEN the database is created THEN the system SHALL include indexes on commonly queried fields (timestamp, hostname, app_name, facility, severity)
5. WHEN the system starts THEN the system SHALL drop and recreate the database without attempting migration

### Requirement 3

**User Story:** As a system administrator, I want enhanced filtering capabilities based on RFC5424 fields, so that I can efficiently locate specific log entries using structured criteria.

#### Acceptance Criteria

1. WHEN I filter by facility THEN the system SHALL show only logs from the specified facility
2. WHEN I filter by severity THEN the system SHALL show logs at or above the specified severity level
3. WHEN I filter by hostname THEN the system SHALL show only logs from the specified host
4. WHEN I filter by app-name THEN the system SHALL show only logs from the specified application
5. WHEN I filter by proc-id THEN the system SHALL show only logs from the specified process
6. WHEN I filter by msg-id THEN the system SHALL show only logs with the specified message identifier
7. WHEN I filter by structured data elements THEN the system SHALL support JSON-based queries on structured data fields

### Requirement 4

**User Story:** As a developer, I want the web interface to display all RFC5424 fields in an organized manner, so that I can see the complete context of each log entry.

#### Acceptance Criteria

1. WHEN logs are displayed THEN the system SHALL show facility, severity, hostname, app-name, proc-id, msg-id in addition to timestamp and message
2. WHEN structured data is present THEN the system SHALL display it in a readable JSON format
3. WHEN the interface loads THEN the system SHALL provide filter controls for all major RFC5424 fields
4. WHEN logs are viewed THEN the system SHALL maintain the terminal-like appearance while accommodating additional fields
5. WHEN structured data is complex THEN the system SHALL provide expandable/collapsible views for readability

### Requirement 5

**User Story:** As a system administrator, I want the migration to be a clean break from legacy formats, so that I don't have to deal with compatibility issues or data inconsistencies.

#### Acceptance Criteria

1. WHEN the system starts with existing data THEN the system SHALL delete the old database file and create a new one
2. WHEN the migration occurs THEN the system SHALL not attempt to preserve or convert existing log data
3. WHEN legacy format parsers are removed THEN the system SHALL have no code paths for custom template or RFC3164 parsing
4. WHEN the migration is complete THEN the system SHALL only contain RFC5424-related parsing and storage code

### Requirement 6

**User Story:** As a developer, I want comprehensive RFC5424 validation, so that only properly formatted messages are accepted and stored.

#### Acceptance Criteria

1. WHEN a message is received THEN the system SHALL validate it conforms to RFC5424 specification
2. WHEN the priority field is invalid THEN the system SHALL reject the message
3. WHEN the version field is not "1" THEN the system SHALL reject the message
4. WHEN the timestamp format is invalid THEN the system SHALL reject the message
5. WHEN structured data syntax is malformed THEN the system SHALL reject the message
6. WHEN validation fails THEN the system SHALL log the rejection reason and continue processing other messages

### Requirement 7

**User Story:** As a system administrator, I want the API endpoints to support RFC5424 field queries, so that external tools can efficiently search logs using structured criteria.

#### Acceptance Criteria

1. WHEN I query the API THEN the system SHALL accept filter parameters for facility, severity, hostname, app_name, proc_id, msg_id
2. WHEN I query structured data THEN the system SHALL support JSON path queries for structured data elements
3. WHEN API responses are returned THEN the system SHALL include all RFC5424 fields in the JSON response
4. WHEN multiple RFC5424 filters are applied THEN the system SHALL combine them with AND logic
5. WHEN severity filtering is used THEN the system SHALL support both exact match and minimum severity level queries