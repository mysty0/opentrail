package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

// DefaultLogParser implements the LogParser interface
type DefaultLogParser struct {
	format           string
	formatRegex      *regexp.Regexp
	timestampFormats []string
}

// RFC3164LogParser implements RFC 3164 BSD syslog protocol parsing
type RFC3164LogParser struct {
	timestampFormats []string
}

// NewDefaultLogParser creates a new parser with the default format
func NewDefaultLogParser() interfaces.LogParser {
	parser := &DefaultLogParser{
		timestampFormats: []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006/01/02 15:04:05",
			"Jan 02 15:04:05",
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05.000000Z",
		},
	}
	
	// Set default format
	_ = parser.SetFormat("{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}")
	
	return parser
}

// NewRFC3164LogParser creates a new parser for RFC 3164 format
func NewRFC3164LogParser() interfaces.LogParser {
	return &RFC3164LogParser{
		timestampFormats: []string{
			"Jan 02 15:04:05",           // Standard syslog format
			"Jan  2 15:04:05",           // Single digit day with space
			"2006-01-02T15:04:05Z07:00", // RFC3339 with timezone
			"2006-01-02T15:04:05",       // RFC3339 without timezone
			"2006-01-02 15:04:05",       // Space separated
		},
	}
}

// SetFormat configures the parser to use a specific log format
func (p *DefaultLogParser) SetFormat(format string) error {
	if format == "" {
		return fmt.Errorf("format cannot be empty")
	}
	
	p.format = format
	
	// Convert template format to regex
	regexPattern := regexp.QuoteMeta(format)
	regexPattern = strings.ReplaceAll(regexPattern, `\{\{timestamp\}\}`, `([^|]*)`)
	regexPattern = strings.ReplaceAll(regexPattern, `\{\{level\}\}`, `([^|]*)`)
	regexPattern = strings.ReplaceAll(regexPattern, `\{\{tracking_id\}\}`, `([^|]*)`)
	regexPattern = strings.ReplaceAll(regexPattern, `\{\{message\}\}`, `(.*)`)
	
	var err error
	p.formatRegex, err = regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		return fmt.Errorf("invalid format pattern: %w", err)
	}
	
	return nil
}

// Parse converts a raw log message string into a LogEntry
func (p *DefaultLogParser) Parse(rawMessage string) (*types.LogEntry, error) {
	if rawMessage == "" {
		return nil, fmt.Errorf("raw message cannot be empty")
	}
	
	// Try to parse with the configured format
	if p.formatRegex != nil {
		if entry, err := p.parseWithFormat(rawMessage); err == nil {
			return entry, nil
		}
	}
	
	// Fallback parsing for malformed messages
	return p.fallbackParse(rawMessage), nil
}

// parseWithFormat attempts to parse the message using the configured format
func (p *DefaultLogParser) parseWithFormat(rawMessage string) (*types.LogEntry, error) {
	matches := p.formatRegex.FindStringSubmatch(rawMessage)
	if matches == nil {
		return nil, fmt.Errorf("message does not match format")
	}
	
	// Extract components based on the format template
	components := p.extractComponents(matches[1:])
	
	entry := &types.LogEntry{
		Level:      normalizeLevel(components["level"]),
		TrackingID: components["tracking_id"],
		Message:    components["message"],
	}
	
	// Parse timestamp
	if timestampStr := components["timestamp"]; timestampStr != "" {
		if ts, err := p.parseTimestamp(timestampStr); err == nil {
			entry.Timestamp = ts
		} else {
			entry.Timestamp = time.Now()
		}
	} else {
		entry.Timestamp = time.Now()
	}
	
	return entry, nil
}

// extractComponents maps regex matches to component names based on format
func (p *DefaultLogParser) extractComponents(matches []string) map[string]string {
	components := make(map[string]string)
	
	// Determine order of components in the format
	formatParts := []string{"timestamp", "level", "tracking_id", "message"}
	templateOrder := make([]string, 0, 4)
	
	for _, part := range formatParts {
		template := "{{" + part + "}}"
		if strings.Contains(p.format, template) {
			templateOrder = append(templateOrder, part)
		}
	}
	
	// Map matches to components
	for i, match := range matches {
		if i < len(templateOrder) {
			components[templateOrder[i]] = strings.TrimSpace(match)
		}
	}
	
	return components
}

// parseTimestamp attempts to parse timestamp using multiple formats
func (p *DefaultLogParser) parseTimestamp(timestampStr string) (time.Time, error) {
	timestampStr = strings.TrimSpace(timestampStr)
	
	for _, format := range p.timestampFormats {
		if ts, err := time.Parse(format, timestampStr); err == nil {
			return ts, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestampStr)
}

// fallbackParse creates a LogEntry for malformed messages
func (p *DefaultLogParser) fallbackParse(rawMessage string) *types.LogEntry {
	return &types.LogEntry{
		Timestamp:  time.Now(),
		Level:      "UNKNOWN",
		TrackingID: "",
		Message:    rawMessage,
	}
}

// normalizeLevel standardizes log level strings
func normalizeLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))
	
	// Map common variations to standard levels
	switch level {
	case "DEBUG", "DBG", "D":
		return "DEBUG"
	case "INFO", "INF", "I":
		return "INFO"
	case "WARN", "WARNING", "WRN", "W":
		return "WARN"
	case "ERROR", "ERR", "E":
		return "ERROR"
	case "FATAL", "CRIT", "CRITICAL", "F":
		return "FATAL"
	default:
		if level == "" {
			return "INFO"
		}
		return level
	}
}

// RFC 3164 specific methods

// SetFormat configures the parser to use a specific log format (RFC 3164 doesn't use this)
func (p *RFC3164LogParser) SetFormat(format string) error {
	// RFC 3164 has a fixed format, so this is a no-op
	return nil
}

// Parse converts a raw RFC 3164 syslog message string into a LogEntry
func (p *RFC3164LogParser) Parse(rawMessage string) (*types.LogEntry, error) {
	if rawMessage == "" {
		return nil, fmt.Errorf("raw message cannot be empty")
	}
	
	return p.parseRFC3164(rawMessage)
}

// parseRFC3164 parses messages according to RFC 3164 BSD syslog protocol
func (p *RFC3164LogParser) parseRFC3164(rawMessage string) (*types.LogEntry, error) {
	// RFC 3164 format: <PRI>TIMESTAMP HOSTNAME TAG: MESSAGE
	// or: <PRI>TIMESTAMP HOSTNAME MESSAGE
	
	// Parse PRI (priority) part
	pri, timestamp, hostname, tag, message, err := p.parseRFC3164Parts(rawMessage)
	if err != nil {
		// Fallback to basic parsing if RFC 3164 format fails
		return &types.LogEntry{
			Timestamp:  time.Now(),
			Level:      "UNKNOWN",
			TrackingID: "",
			Message:    rawMessage,
		}, nil
	}
	
	// Convert PRI to severity level
	level := p.priToSeverity(pri)
	
	// Parse timestamp
	parsedTime, err := p.parseSyslogTimestamp(timestamp)
	if err != nil {
		parsedTime = time.Now()
	}
	
	// Combine hostname and tag for tracking ID if available
	trackingID := ""
	if hostname != "" {
		trackingID = hostname
		if tag != "" {
			trackingID = trackingID + ":" + tag
		}
	}
	
	return &types.LogEntry{
		Timestamp:  parsedTime,
		Level:      level,
		TrackingID: trackingID,
		Message:    message,
	}, nil
}

// parseRFC3164Parts extracts components from RFC 3164 message
func (p *RFC3164LogParser) parseRFC3164Parts(rawMessage string) (int, string, string, string, string, error) {
	// Remove leading/trailing whitespace
	rawMessage = strings.TrimSpace(rawMessage)
	
	// Check for PRI part
	if !strings.HasPrefix(rawMessage, "<") {
		return 0, "", "", "", rawMessage, fmt.Errorf("no PRI part found")
	}
	
	// Find closing > for PRI
	priEnd := strings.Index(rawMessage, ">")
	if priEnd == -1 {
		return 0, "", "", "", rawMessage, fmt.Errorf("invalid PRI format")
	}
	
	// Parse PRI value
	priStr := rawMessage[1:priEnd]
	pri, err := strconv.Atoi(priStr)
	if err != nil {
		return 0, "", "", "", rawMessage, fmt.Errorf("invalid PRI value")
	}
	
	// Remaining message after PRI
	remaining := strings.TrimSpace(rawMessage[priEnd+1:])
	
	// RFC 3164 format: MMM DD HH:MM:SS HOSTNAME [TAG:] MESSAGE
	// Use regex to extract components more accurately
	re := regexp.MustCompile(`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(.+)$`)
	matches := re.FindStringSubmatch(remaining)
	
	if len(matches) != 4 {
		return pri, "", "", "", remaining, fmt.Errorf("invalid RFC 3164 format")
	}
	
	timestamp := matches[1]
	hostname := matches[2]
	content := matches[3]
	
	// Parse tag and message from content
	tag, message := p.extractTagAndMessage(content)
	
	return pri, timestamp, hostname, tag, message, nil
}

// extractTimestamp extracts the timestamp from RFC 3164 format
func (p *RFC3164LogParser) extractTimestamp(remaining string) (string, string) {
	// RFC 3164 timestamp format: MMM DD HH:MM:SS
	// Example: "Jan 23 14:30:45"
	
	parts := strings.Fields(remaining)
	if len(parts) < 3 {
		return "", remaining
	}
	
	// Check if we have a valid timestamp format
	possibleTimestamp := strings.Join(parts[:3], " ")
	
	// Try to parse as syslog timestamp
	for _, format := range p.timestampFormats {
		if format == "Jan 02 15:04:05" || format == "Jan  2 15:04:05" {
			// For syslog format, check if it matches the pattern
			if len(possibleTimestamp) >= 12 && len(possibleTimestamp) <= 15 {
				// Basic validation for MMM DD HH:MM:SS format
				if len(parts) >= 3 {
					return possibleTimestamp, strings.TrimSpace(remaining[len(possibleTimestamp):])
				}
			}
		} else {
			if _, err := time.Parse(format, possibleTimestamp); err == nil {
				return possibleTimestamp, strings.TrimSpace(remaining[len(possibleTimestamp):])
			}
		}
	}
	
	return "", remaining
}

// extractHostname extracts the hostname from remaining message
func (p *RFC3164LogParser) extractHostname(remaining string) (string, string) {
	parts := strings.Fields(remaining)
	if len(parts) > 0 {
		return parts[0], strings.TrimSpace(remaining[len(parts[0]):])
	}
	return "", remaining
}

// extractTagAndMessage separates tag from message content
func (p *RFC3164LogParser) extractTagAndMessage(remaining string) (string, string) {
	// Look for : as separator between tag and message
	colonIndex := strings.Index(remaining, ":")
	if colonIndex == -1 {
		return "", remaining
	}
	
	tag := strings.TrimSpace(remaining[:colonIndex])
	message := strings.TrimSpace(remaining[colonIndex+1:])
	
	return tag, message
}

// parseSyslogTimestamp attempts to parse syslog timestamp formats
func (p *RFC3164LogParser) parseSyslogTimestamp(timestampStr string) (time.Time, error) {
	timestampStr = strings.TrimSpace(timestampStr)
	
	// Handle current year for syslog format
	currentYear := time.Now().Year()
	yearStr := strconv.Itoa(currentYear)
	
	for _, format := range p.timestampFormats {
		if format == "Jan 02 15:04:05" || format == "Jan  2 15:04:05" {
			// Prepend current year to syslog format
			fullFormat := yearStr + " " + format
			fullTimestamp := yearStr + " " + timestampStr
			
			if ts, err := time.Parse(fullFormat, fullTimestamp); err == nil {
				return ts, nil
			}
		} else {
			if ts, err := time.Parse(format, timestampStr); err == nil {
				return ts, nil
			}
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse syslog timestamp: %s", timestampStr)
}

// priToSeverity converts RFC 3164 PRI value to severity level string
func (p *RFC3164LogParser) priToSeverity(pri int) string {
	// RFC 3164 PRI = facility * 8 + severity
	// Severity values:
	// 0: Emergency
	// 1: Alert
	// 2: Critical
	// 3: Error
	// 4: Warning
	// 5: Notice
	// 6: Info
	// 7: Debug
	
	severity := pri & 7 // Get last 3 bits for severity
	
	switch severity {
	case 0:
		return "EMERGENCY"
	case 1:
		return "ALERT"
	case 2:
		return "CRITICAL"
	case 3:
		return "ERROR"
	case 4:
		return "WARNING"
	case 5:
		return "NOTICE"
	case 6:
		return "INFO"
	case 7:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}