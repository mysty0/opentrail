package parser

import (
	"fmt"
	"regexp"
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