# Parser Package

This package provides log parsing functionality for the OpenTrail system, supporting both custom formats and RFC 3164 BSD syslog protocol.

## Features

### DefaultLogParser
- **Custom format parsing**: Supports configurable log formats using template variables
- **Template variables**: `{{timestamp}}`, `{{level}}`, `{{tracking_id}}`, `{{message}}`
- **Flexible timestamp parsing**: Supports multiple timestamp formats
- **Fallback parsing**: Gracefully handles malformed messages

### RFC3164LogParser
- **RFC 3164 compliant**: Full support for BSD syslog protocol as defined in RFC 3164
- **PRI parsing**: Correctly parses priority values and maps to severity levels
- **Header parsing**: Extracts timestamp and hostname from syslog headers
- **Tag extraction**: Optional tag extraction from message content
- **Robust parsing**: Handles edge cases and malformed messages gracefully

## Usage

### DefaultLogParser

```go
parser := parser.NewDefaultLogParser()

// Parse with default format
entry, err := parser.Parse("2023-12-01T10:30:00Z|INFO|user123|Application started")

// Configure custom format
err := parser.SetFormat("[{{timestamp}}] {{level}} ({{tracking_id}}): {{message}}")
entry, err := parser.Parse("[2023-12-01T10:30:00Z] INFO (user123): Application started")
```

### RFC3164LogParser

```go
parser := parser.NewRFC3164LogParser()

// Parse RFC 3164 syslog message
entry, err := parser.Parse("<34>Jan 23 14:30:45 myhost myapp: Application started successfully")

// Results:
// - Level: CRITICAL (PRI 34 = facility 4, severity 2)
// - TrackingID: "myhost:myapp"
// - Message: "Application started successfully"
// - Timestamp: parsed from syslog format
```

## RFC 3164 Format Details

RFC 3164 messages follow the format: `<PRI>TIMESTAMP HOSTNAME [TAG:] MESSAGE`

- **PRI**: Priority value (facility * 8 + severity)
- **TIMESTAMP**: Standard syslog timestamp format (MMM DD HH:MM:SS)
- **HOSTNAME**: Source hostname
- **TAG**: Optional tag/program name
- **MESSAGE**: Actual log message content

### Severity Mapping

The parser maps RFC 3164 severity values as follows:

| Severity | Level |
|----------|--------|
| 0        | EMERGENCY |
| 1        | ALERT |
| 2        | CRITICAL |
| 3        | ERROR |
| 4        | WARNING |
| 5        | NOTICE |
| 6        | INFO |
| 7        | DEBUG |

## Examples

### Valid RFC 3164 Messages

```
<13>Feb  5 09:15:30 client01 sshd: Failed login attempt
<34>Jan 23 14:30:45 server nginx: Warning: high memory usage
<14>Dec 12 10:00:00 localhost kernel: Out of memory error
<191>Mar 15 16:45:12 webserver app: Debug: processing request
```

### Error Handling

Both parsers handle malformed messages gracefully by falling back to basic parsing:

```go
// Malformed RFC 3164 message
entry, _ := parser.Parse("This is not a syslog message")
// Results in: Level="UNKNOWN", Message="This is not a syslog message"
```