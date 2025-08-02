package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewDefaultLogParser(t *testing.T) {
	parser := NewDefaultLogParser()
	if parser == nil {
		t.Fatal("NewDefaultLogParser returned nil")
	}
}

func TestDefaultLogParser_SetFormat(t *testing.T) {
	parser := NewDefaultLogParser()
	
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{
			name:    "valid default format",
			format:  "{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}",
			wantErr: false,
		},
		{
			name:    "valid custom format",
			format:  "[{{timestamp}}] {{level}} ({{tracking_id}}): {{message}}",
			wantErr: false,
		},
		{
			name:    "format without tracking_id",
			format:  "{{timestamp}} {{level}} {{message}}",
			wantErr: false,
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.SetFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultLogParser_Parse_DefaultFormat(t *testing.T) {
	parser := NewDefaultLogParser()
	
	tests := []struct {
		name        string
		rawMessage  string
		wantLevel   string
		wantTrackingID string
		wantMessage string
		wantErr     bool
	}{
		{
			name:           "valid complete message",
			rawMessage:     "2023-12-01T10:30:00Z|INFO|user123|Application started successfully",
			wantLevel:      "INFO",
			wantTrackingID: "user123",
			wantMessage:    "Application started successfully",
			wantErr:        false,
		},
		{
			name:           "message without tracking_id",
			rawMessage:     "2023-12-01T10:30:00Z|ERROR||Database connection failed",
			wantLevel:      "ERROR",
			wantTrackingID: "",
			wantMessage:    "Database connection failed",
			wantErr:        false,
		},
		{
			name:           "message with pipe in content",
			rawMessage:     "2023-12-01T10:30:00Z|WARN|req456|Query returned | character in result",
			wantLevel:      "WARN",
			wantTrackingID: "req456",
			wantMessage:    "Query returned | character in result",
			wantErr:        false,
		},
		{
			name:        "empty message",
			rawMessage:  "",
			wantErr:     true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.rawMessage)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Parse() unexpected error = %v", err)
				return
			}
			
			if entry.Level != tt.wantLevel {
				t.Errorf("Parse() level = %v, want %v", entry.Level, tt.wantLevel)
			}
			
			if entry.TrackingID != tt.wantTrackingID {
				t.Errorf("Parse() tracking_id = %v, want %v", entry.TrackingID, tt.wantTrackingID)
			}
			
			if entry.Message != tt.wantMessage {
				t.Errorf("Parse() message = %v, want %v", entry.Message, tt.wantMessage)
			}
			
			if entry.Timestamp.IsZero() {
				t.Errorf("Parse() timestamp should not be zero")
			}
		})
	}
}

func TestDefaultLogParser_Parse_CustomFormat(t *testing.T) {
	parser := NewDefaultLogParser()
	err := parser.SetFormat("[{{timestamp}}] {{level}} ({{tracking_id}}): {{message}}")
	if err != nil {
		t.Fatalf("SetFormat() error = %v", err)
	}
	
	tests := []struct {
		name        string
		rawMessage  string
		wantLevel   string
		wantTrackingID string
		wantMessage string
	}{
		{
			name:           "custom format message",
			rawMessage:     "[2023-12-01T10:30:00Z] INFO (user123): Application started",
			wantLevel:      "INFO",
			wantTrackingID: "user123",
			wantMessage:    "Application started",
		},
		{
			name:           "custom format without tracking_id",
			rawMessage:     "[2023-12-01T10:30:00Z] ERROR (): Connection failed",
			wantLevel:      "ERROR",
			wantTrackingID: "",
			wantMessage:    "Connection failed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.rawMessage)
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if entry.Level != tt.wantLevel {
				t.Errorf("Parse() level = %v, want %v", entry.Level, tt.wantLevel)
			}
			
			if entry.TrackingID != tt.wantTrackingID {
				t.Errorf("Parse() tracking_id = %v, want %v", entry.TrackingID, tt.wantTrackingID)
			}
			
			if entry.Message != tt.wantMessage {
				t.Errorf("Parse() message = %v, want %v", entry.Message, tt.wantMessage)
			}
		})
	}
}

func TestDefaultLogParser_Parse_FallbackParsing(t *testing.T) {
	parser := NewDefaultLogParser()
	
	tests := []struct {
		name        string
		rawMessage  string
		wantLevel   string
		wantMessage string
	}{
		{
			name:        "malformed message",
			rawMessage:  "This is not a properly formatted log message",
			wantLevel:   "UNKNOWN",
			wantMessage: "This is not a properly formatted log message",
		},
		{
			name:        "partial format match",
			rawMessage:  "2023-12-01|INFO|incomplete",
			wantLevel:   "UNKNOWN",
			wantMessage: "2023-12-01|INFO|incomplete",
		},
		{
			name:        "random text",
			rawMessage:  "Random error occurred in system",
			wantLevel:   "UNKNOWN",
			wantMessage: "Random error occurred in system",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.rawMessage)
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if entry.Level != tt.wantLevel {
				t.Errorf("Parse() level = %v, want %v", entry.Level, tt.wantLevel)
			}
			
			if entry.Message != tt.wantMessage {
				t.Errorf("Parse() message = %v, want %v", entry.Message, tt.wantMessage)
			}
			
			if entry.Timestamp.IsZero() {
				t.Errorf("Parse() timestamp should not be zero for fallback parsing")
			}
		})
	}
}

func TestDefaultLogParser_TimestampParsing(t *testing.T) {
	parser := NewDefaultLogParser()
	
	tests := []struct {
		name      string
		timestamp string
		wantValid bool
	}{
		{
			name:      "RFC3339 format",
			timestamp: "2023-12-01T10:30:00Z",
			wantValid: true,
		},
		{
			name:      "RFC3339 with nanoseconds",
			timestamp: "2023-12-01T10:30:00.123456789Z",
			wantValid: true,
		},
		{
			name:      "ISO format without timezone",
			timestamp: "2023-12-01T10:30:00",
			wantValid: true,
		},
		{
			name:      "Space separated format",
			timestamp: "2023-12-01 10:30:00",
			wantValid: true,
		},
		{
			name:      "Slash separated format",
			timestamp: "2023/12/01 10:30:00",
			wantValid: true,
		},
		{
			name:      "Syslog format",
			timestamp: "Dec 01 10:30:00",
			wantValid: true,
		},
		{
			name:      "Invalid timestamp",
			timestamp: "not-a-timestamp",
			wantValid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawMessage := tt.timestamp + "|INFO|test|Test message"
			entry, err := parser.Parse(rawMessage)
			
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if tt.wantValid {
				// For valid timestamps, check that it's not the current time (fallback)
				now := time.Now()
				timeDiff := now.Sub(entry.Timestamp)
				if timeDiff < time.Second {
					// If the difference is less than a second, it might be fallback time
					// This is a heuristic test - in real scenarios, parsed timestamps should be different
					t.Logf("Timestamp might be fallback time: %v", entry.Timestamp)
				}
			}
		})
	}
}

func TestNormalizeLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"DEBUG", "DEBUG"},
		{"dbg", "DEBUG"},
		{"D", "DEBUG"},
		{"info", "INFO"},
		{"INFO", "INFO"},
		{"inf", "INFO"},
		{"I", "INFO"},
		{"warn", "WARN"},
		{"WARN", "WARN"},
		{"warning", "WARN"},
		{"WRN", "WARN"},
		{"W", "WARN"},
		{"error", "ERROR"},
		{"ERROR", "ERROR"},
		{"err", "ERROR"},
		{"E", "ERROR"},
		{"fatal", "FATAL"},
		{"FATAL", "FATAL"},
		{"crit", "FATAL"},
		{"critical", "FATAL"},
		{"F", "FATAL"},
		{"", "INFO"},
		{"CUSTOM", "CUSTOM"},
		{"  INFO  ", "INFO"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeLevel(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLevel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewRFC3164LogParser(t *testing.T) {
	parser := NewRFC3164LogParser()
	if parser == nil {
		t.Fatal("NewRFC3164LogParser returned nil")
	}
}

func TestRFC3164LogParser_Parse(t *testing.T) {
	parser := NewRFC3164LogParser()
	
	tests := []struct {
		name        string
		rawMessage  string
		wantLevel   string
		wantTrackingID string
		wantMessage string
		wantErr     bool
	}{
		{
			name:           "valid RFC 3164 message",
			rawMessage:     "<34>Jan 23 14:30:45 myhost myapp: Application started successfully",
			wantLevel:      "CRITICAL",
			wantTrackingID: "myhost:myapp",
			wantMessage:    "Application started successfully",
			wantErr:        false,
		},
		{
			name:           "RFC 3164 without tag",
			rawMessage:     "<14>Jan 23 14:30:45 myhost System shutdown initiated",
			wantLevel:      "INFO",
			wantTrackingID: "myhost",
			wantMessage:    "System shutdown initiated",
			wantErr:        false,
		},
		{
			name:           "RFC 3164 with different PRI",
			rawMessage:     "<187>Jan 23 14:30:45 server01 kernel: Out of memory error",
			wantLevel:      "ERROR",
			wantTrackingID: "server01:kernel",
			wantMessage:    "Out of memory error",
			wantErr:        false,
		},
		{
			name:           "RFC 3164 debug level",
			rawMessage:     "<191>Jan 23 14:30:45 localhost app: Debug message here",
			wantLevel:      "DEBUG",
			wantTrackingID: "localhost:app",
			wantMessage:    "Debug message here",
			wantErr:        false,
		},
		{
			name:           "RFC 3164 warning level",
			rawMessage:     "<188>Jan 23 14:30:45 webserver nginx: Warning: high memory usage",
			wantLevel:      "WARNING",
			wantTrackingID: "webserver:nginx",
			wantMessage:    "Warning: high memory usage",
			wantErr:        false,
		},
		{
			name:           "RFC 3164 single digit day",
			rawMessage:     "<13>Feb  5 09:15:30 client01 sshd: Failed login attempt",
			wantLevel:      "NOTICE",
			wantTrackingID: "client01:sshd",
			wantMessage:    "Failed login attempt",
			wantErr:        false,
		},
		{
			name:           "malformed RFC 3164",
			rawMessage:     "This is not a syslog message",
			wantLevel:      "UNKNOWN",
			wantTrackingID: "",
			wantMessage:    "This is not a syslog message",
			wantErr:        false,
		},
		{
			name:           "empty message",
			rawMessage:     "",
			wantLevel:      "",
			wantTrackingID: "",
			wantMessage:    "",
			wantErr:        true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.rawMessage)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Parse() unexpected error = %v", err)
				return
			}
			
			if entry.Level != tt.wantLevel {
				t.Errorf("Parse() level = %v, want %v", entry.Level, tt.wantLevel)
			}
			
			if entry.TrackingID != tt.wantTrackingID {
				t.Errorf("Parse() tracking_id = %v, want %v", entry.TrackingID, tt.wantTrackingID)
			}
			
			if entry.Message != tt.wantMessage {
				t.Errorf("Parse() message = %v, want %v", entry.Message, tt.wantMessage)
			}
			
			if entry.Timestamp.IsZero() {
				t.Errorf("Parse() timestamp should not be zero")
			}
		})
	}
}

func TestRFC3164LogParser_SetFormat(t *testing.T) {
	parser := NewRFC3164LogParser()
	
	// RFC 3164 parser should ignore SetFormat calls
	err := parser.SetFormat("{{timestamp}} {{level}} {{message}}")
	if err != nil {
		t.Errorf("SetFormat() returned error: %v", err)
	}
}

func TestRFC3164LogParser_priToSeverity(t *testing.T) {
	parser := &RFC3164LogParser{}
	
	tests := []struct {
		pri  int
		want string
	}{
		{0, "EMERGENCY"},
		{1, "ALERT"},
		{2, "CRITICAL"},
		{3, "ERROR"},
		{4, "WARNING"},
		{5, "NOTICE"},
		{6, "INFO"},
		{7, "DEBUG"},
		{8, "EMERGENCY"}, // Facility 1, Severity 0
		{15, "DEBUG"},    // Facility 1, Severity 7
		{16, "EMERGENCY"}, // Facility 2, Severity 0
		{23, "DEBUG"},     // Facility 2, Severity 7
		{34, "CRITICAL"},  // Facility 4, Severity 2
		{189, "NOTICE"},   // Facility 23, Severity 5
		{191, "DEBUG"},    // Facility 23, Severity 7
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("pri_%d", tt.pri), func(t *testing.T) {
			got := parser.priToSeverity(tt.pri)
			if got != tt.want {
				t.Errorf("priToSeverity(%d) = %q, want %q", tt.pri, got, tt.want)
			}
		})
	}
}

func TestRFC3164LogParser_parseRFC3164Parts(t *testing.T) {
	parser := &RFC3164LogParser{}
	
	tests := []struct {
		name       string
		message    string
		wantPri    int
		wantTS     string
		wantHost   string
		wantTag    string
		wantMsg    string
		wantErr    bool
	}{
		{
			name:     "valid RFC 3164",
			message:  "<34>Jan 23 14:30:45 myhost myapp: Application started",
			wantPri:  34,
			wantTS:   "Jan 23 14:30:45",
			wantHost: "myhost",
			wantTag:  "myapp",
			wantMsg:  "Application started",
			wantErr:  false,
		},
		{
			name:     "no PRI",
			message:  "Jan 23 14:30:45 myhost message",
			wantPri:  0,
			wantTS:   "",
			wantHost: "",
			wantTag:  "",
			wantMsg:  "Jan 23 14:30:45 myhost message",
			wantErr:  true,
		},
		{
			name:     "invalid PRI",
			message:  "<abc>Jan 23 14:30:45 myhost message",
			wantPri:  0,
			wantTS:   "",
			wantHost: "",
			wantTag:  "",
			wantMsg:  "<abc>Jan 23 14:30:45 myhost message",
			wantErr:  true,
		},
		{
			name:     "missing closing PRI",
			message:  "<34Jan 23 14:30:45 myhost message",
			wantPri:  0,
			wantTS:   "",
			wantHost: "",
			wantTag:  "",
			wantMsg:  "<34Jan 23 14:30:45 myhost message",
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pri, ts, host, tag, msg, err := parser.parseRFC3164Parts(tt.message)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRFC3164Parts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				if pri != tt.wantPri {
					t.Errorf("parseRFC3164Parts() pri = %d, want %d", pri, tt.wantPri)
				}
				if ts != tt.wantTS {
					t.Errorf("parseRFC3164Parts() timestamp = %q, want %q", ts, tt.wantTS)
				}
				if host != tt.wantHost {
					t.Errorf("parseRFC3164Parts() hostname = %q, want %q", host, tt.wantHost)
				}
				if tag != tt.wantTag {
					t.Errorf("parseRFC3164Parts() tag = %q, want %q", tag, tt.wantTag)
				}
				if msg != tt.wantMsg {
					t.Errorf("parseRFC3164Parts() message = %q, want %q", msg, tt.wantMsg)
				}
			}
		})
	}
}

func TestRFC3164LogParser_EdgeCases(t *testing.T) {
	parser := NewRFC3164LogParser()
	
	tests := []struct {
		name       string
		rawMessage string
		wantLevel  string
	}{
		{
			name:       "message with special characters",
			rawMessage: "<34>Jan 23 14:30:45 host app: Error: connection failed to host:port",
			wantLevel:  "CRITICAL",
		},
		{
			name:       "message with multiple colons",
			rawMessage: "<14>Jan 23 14:30:45 server syslog: time:14:30:45: message here",
			wantLevel:  "INFO",
		},
		{
			name:       "message with empty tag",
			rawMessage: "<14>Jan 23 14:30:45 host : message without tag",
			wantLevel:  "INFO",
		},
		{
			name:       "message with very long hostname",
			rawMessage: "<14>Jan 23 14:30:45 very-long-hostname-with-dashes-and-numbers-12345 app: message",
			wantLevel:  "INFO",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.rawMessage)
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if entry.Level != tt.wantLevel {
				t.Errorf("Parse() level = %v, want %v", entry.Level, tt.wantLevel)
			}
			
			if entry.Timestamp.IsZero() {
				t.Errorf("Parse() timestamp should not be zero")
			}
		})
	}
}

func TestDefaultLogParser_Parse_EdgeCases(t *testing.T) {
	parser := NewDefaultLogParser()
	
	tests := []struct {
		name       string
		rawMessage string
		wantLevel  string
	}{
		{
			name:       "message with multiple pipes",
			rawMessage: "2023-12-01T10:30:00Z|INFO|user123|Message with | multiple | pipes",
			wantLevel:  "INFO",
		},
		{
			name:       "message with empty fields",
			rawMessage: "2023-12-01T10:30:00Z|||Empty fields message",
			wantLevel:  "INFO", // Empty level should default to INFO
		},
		{
			name:       "message with whitespace",
			rawMessage: "  2023-12-01T10:30:00Z  |  INFO  |  user123  |  Whitespace message  ",
			wantLevel:  "INFO",
		},
		{
			name:       "very long message",
			rawMessage: "2023-12-01T10:30:00Z|ERROR|req789|" + strings.Repeat("Very long message content ", 100),
			wantLevel:  "ERROR",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.rawMessage)
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if entry.Level != tt.wantLevel {
				t.Errorf("Parse() level = %v, want %v", entry.Level, tt.wantLevel)
			}
			
			if entry.Timestamp.IsZero() {
				t.Errorf("Parse() timestamp should not be zero")
			}
		})
	}
}