package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

// RFC5424Parser implements RFC5424 syslog protocol parsing
type RFC5424Parser struct {
	strictMode bool // Whether to reject malformed messages
}

// NewRFC5424Parser creates a new RFC5424 parser
func NewRFC5424Parser(strictMode bool) interfaces.LogParser {
	return &RFC5424Parser{
		strictMode: strictMode,
	}
}

// SetFormat is a no-op for RFC5424 parser as it has a fixed format
func (p *RFC5424Parser) SetFormat(format string) error {
	// RFC5424 has a fixed format, so this is a no-op
	return nil
}

// Parse converts a raw RFC5424 log message string into a LogEntry
func (p *RFC5424Parser) Parse(rawMessage string) (*types.LogEntry, error) {
	if rawMessage == "" {
		return nil, fmt.Errorf("raw message cannot be empty")
	}
	
	return p.parseRFC5424(rawMessage)
}

// parseRFC5424 parses messages according to RFC5424 specification
func (p *RFC5424Parser) parseRFC5424(rawMessage string) (*types.LogEntry, error) {
	// RFC5424 format: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [STRUCTURED-DATA] MSG
	
	// Parse PRI (priority) part
	pri, remaining, err := p.parsePRI(rawMessage)
	if err != nil {
		if p.strictMode {
			return nil, fmt.Errorf("RFC5424 parse error: %w", err)
		}
		return p.fallbackParse(rawMessage), nil
	}
	
	// Parse VERSION
	version, remaining, err := p.parseVersion(remaining)
	if err != nil {
		if p.strictMode {
			return nil, fmt.Errorf("RFC5424 parse error: %w", err)
		}
		return p.fallbackParse(rawMessage), nil
	}
	
	// Parse TIMESTAMP
	timestamp, remaining, err := p.parseTimestamp(remaining)
	if err != nil {
		if p.strictMode {
			return nil, fmt.Errorf("RFC5424 parse error: %w", err)
		}
		timestamp = time.Now()
	}
	
	// Parse HOSTNAME
	hostname, remaining := p.parseField(remaining)
	
	// Parse APP-NAME
	appName, remaining := p.parseField(remaining)
	
	// Parse PROCID
	procID, remaining := p.parseField(remaining)
	
	// Parse MSGID
	msgID, remaining := p.parseField(remaining)
	
	// Parse STRUCTURED-DATA
	structuredData, remaining, err := p.parseStructuredData(remaining)
	if err != nil && p.strictMode {
		return nil, fmt.Errorf("RFC5424 structured data parse error: %w", err)
	}
	
	// Remaining is the MSG part
	message := strings.TrimSpace(remaining)
	if message == "-" {
		message = "" // Nil value for message
	}
	
	// Create LogEntry
	entry := &types.LogEntry{
		Version:        version,
		Timestamp:      timestamp,
		Hostname:       hostname,
		AppName:        appName,
		ProcID:         procID,
		MsgID:          msgID,
		StructuredData: structuredData,
		Message:        message,
		CreatedAt:      time.Now(),
	}
	
	// Set priority and extract facility/severity
	entry.SetPriority(pri)
	
	return entry, nil
}

// parsePRI extracts the priority value from the beginning of the message
func (p *RFC5424Parser) parsePRI(rawMessage string) (int, string, error) {
	if !strings.HasPrefix(rawMessage, "<") {
		return 0, rawMessage, fmt.Errorf("missing PRI part")
	}
	
	priEnd := strings.Index(rawMessage, ">")
	if priEnd == -1 {
		return 0, rawMessage, fmt.Errorf("invalid PRI format")
	}
	
	priStr := rawMessage[1:priEnd]
	pri, err := strconv.Atoi(priStr)
	if err != nil {
		return 0, rawMessage, fmt.Errorf("invalid PRI value: %s", priStr)
	}
	
	if pri < 0 || pri > 191 {
		return 0, rawMessage, fmt.Errorf("PRI value out of range: %d", pri)
	}
	
	return pri, strings.TrimSpace(rawMessage[priEnd+1:]), nil
}

// parseVersion extracts the version field
func (p *RFC5424Parser) parseVersion(remaining string) (int, string, error) {
	parts := strings.SplitN(remaining, " ", 2)
	if len(parts) < 1 {
		return 0, remaining, fmt.Errorf("missing VERSION field")
	}
	
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, remaining, fmt.Errorf("invalid VERSION field: %s", parts[0])
	}
	
	if version != 1 {
		return 0, remaining, fmt.Errorf("unsupported RFC5424 version: %d", version)
	}
	
	if len(parts) == 2 {
		return version, strings.TrimSpace(parts[1]), nil
	}
	return version, "", nil
}

// parseTimestamp extracts and parses the timestamp field
func (p *RFC5424Parser) parseTimestamp(remaining string) (time.Time, string, error) {
	parts := strings.SplitN(remaining, " ", 2)
	if len(parts) < 1 {
		return time.Time{}, remaining, fmt.Errorf("missing TIMESTAMP field")
	}
	
	timestampStr := parts[0]
	if timestampStr == "-" {
		// Nil value
		if len(parts) == 2 {
			return time.Now(), strings.TrimSpace(parts[1]), nil
		}
		return time.Now(), "", nil
	}
	
	// Parse RFC3339 timestamp
	timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		// Try RFC3339 without nanoseconds
		timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return time.Time{}, remaining, fmt.Errorf("invalid TIMESTAMP format: %s", timestampStr)
		}
	}
	
	if len(parts) == 2 {
		return timestamp, strings.TrimSpace(parts[1]), nil
	}
	return timestamp, "", nil
}

// parseField extracts a single field (HOSTNAME, APP-NAME, PROCID, MSGID)
func (p *RFC5424Parser) parseField(remaining string) (string, string) {
	parts := strings.SplitN(remaining, " ", 2)
	if len(parts) < 1 {
		return "", remaining
	}
	
	field := parts[0]
	if field == "-" {
		field = "" // Nil value
	}
	
	if len(parts) == 2 {
		return field, strings.TrimSpace(parts[1])
	}
	return field, ""
}

// parseStructuredData extracts and parses structured data elements
func (p *RFC5424Parser) parseStructuredData(remaining string) (map[string]interface{}, string, error) {
	remaining = strings.TrimSpace(remaining)
	
	if !strings.HasPrefix(remaining, "[") {
		// No structured data - check if it starts with "-" (nil value)
		if strings.HasPrefix(remaining, "-") {
			// Skip the "-" and return the rest
			if len(remaining) > 1 {
				return nil, strings.TrimSpace(remaining[1:]), nil
			}
			return nil, "", nil
		}
		return nil, remaining, nil
	}
	
	structuredData := make(map[string]interface{})
	
	// Find all structured data elements
	for strings.HasPrefix(remaining, "[") {
		// Find the end of this structured data element
		depth := 0
		end := -1
		for i, r := range remaining {
			if r == '[' {
				depth++
			} else if r == ']' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		
		if end == -1 {
			return nil, remaining, fmt.Errorf("malformed structured data: missing closing bracket")
		}
		
		// Parse this structured data element
		element := remaining[1:end] // Remove [ and ]
		sdID, params, err := p.parseStructuredDataElement(element)
		if err != nil {
			return nil, remaining, fmt.Errorf("malformed structured data element: %w", err)
		}
		
		structuredData[sdID] = params
		
		// Move to next element or end
		remaining = strings.TrimSpace(remaining[end+1:])
	}
	
	return structuredData, remaining, nil
}

// parseStructuredDataElement parses a single structured data element
func (p *RFC5424Parser) parseStructuredDataElement(element string) (string, map[string]string, error) {
	// Format: SD-ID param1="value1" param2="value2"
	parts := strings.Fields(element)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("empty structured data element")
	}
	
	sdID := parts[0]
	params := make(map[string]string)
	
	// Parse parameters
	for i := 1; i < len(parts); i++ {
		param := parts[i]
		
		// Find the = separator
		eqIndex := strings.Index(param, "=")
		if eqIndex == -1 {
			continue // Skip malformed parameters
		}
		
		key := param[:eqIndex]
		value := param[eqIndex+1:]
		
		// Remove quotes from value
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
			// Unescape quotes and backslashes
			value = strings.ReplaceAll(value, `\"`, `"`)
			value = strings.ReplaceAll(value, `\\`, `\`)
			value = strings.ReplaceAll(value, `\]`, `]`)
		}
		
		params[key] = value
	}
	
	return sdID, params, nil
}

// fallbackParse creates a LogEntry for malformed messages
func (p *RFC5424Parser) fallbackParse(rawMessage string) *types.LogEntry {
	entry := &types.LogEntry{
		Version:        1,
		Timestamp:      time.Now(),
		Hostname:       "",
		AppName:        "",
		ProcID:         "",
		MsgID:          "",
		StructuredData: nil,
		Message:        rawMessage,
		CreatedAt:      time.Now(),
	}
	
	// Set default priority (facility 16 = local0, severity 6 = info)
	entry.SetPriority(134) // 16 * 8 + 6
	
	return entry
}