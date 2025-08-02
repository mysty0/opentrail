package parser

import (
	"fmt"
	"testing"
)

func TestNewRFC5424Parser(t *testing.T) {
	parser := NewRFC5424Parser(true)
	if parser == nil {
		t.Fatal("NewRFC5424Parser returned nil")
	}
}

func TestRFC5424Parser_SetFormat(t *testing.T) {
	parser := NewRFC5424Parser(true)
	
	// RFC5424 parser should ignore SetFormat calls
	err := parser.SetFormat("{{timestamp}} {{level}} {{message}}")
	if err != nil {
		t.Errorf("SetFormat() returned error: %v", err)
	}
}

func TestRFC5424Parser_Parse_ValidMessages(t *testing.T) {
	parser := NewRFC5424Parser(true)
	
	tests := []struct {
		name        string
		rawMessage  string
		wantPri     int
		wantFacility int
		wantSeverity int
		wantVersion int
		wantHostname string
		wantAppName string
		wantProcID  string
		wantMsgID   string
		wantMessage string
		wantErr     bool
	}{
		{
			name:         "basic RFC5424 message",
			rawMessage:   "<165>1 2023-10-15T14:30:45.123Z web01 nginx 1234 access - User login successful",
			wantPri:      165,
			wantFacility: 20,
			wantSeverity: 5,
			wantVersion:  1,
			wantHostname: "web01",
			wantAppName:  "nginx",
			wantProcID:   "1234",
			wantMsgID:    "access",
			wantMessage:  "User login successful",
			wantErr:      false,
		},
		{
			name:         "message with nil values",
			rawMessage:   "<134>1 2023-10-15T14:30:45Z - - - - - System startup complete",
			wantPri:      134,
			wantFacility: 16,
			wantSeverity: 6,
			wantVersion:  1,
			wantHostname: "",
			wantAppName:  "",
			wantProcID:   "",
			wantMsgID:    "",
			wantMessage:  "System startup complete",
			wantErr:      false,
		},
		{
			name:         "message with structured data",
			rawMessage:   `<165>1 2023-10-15T14:30:45Z web01 nginx 1234 access [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"] User login successful`,
			wantPri:      165,
			wantFacility: 20,
			wantSeverity: 5,
			wantVersion:  1,
			wantHostname: "web01",
			wantAppName:  "nginx",
			wantProcID:   "1234",
			wantMsgID:    "access",
			wantMessage:  "User login successful",
			wantErr:      false,
		},
		{
			name:         "message without MSG part",
			rawMessage:   "<134>1 2023-10-15T14:30:45Z localhost app 123 test -",
			wantPri:      134,
			wantFacility: 16,
			wantSeverity: 6,
			wantVersion:  1,
			wantHostname: "localhost",
			wantAppName:  "app",
			wantProcID:   "123",
			wantMsgID:    "test",
			wantMessage:  "",
			wantErr:      false,
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
			
			if entry.Priority != tt.wantPri {
				t.Errorf("Parse() priority = %v, want %v", entry.Priority, tt.wantPri)
			}
			
			if entry.Facility != tt.wantFacility {
				t.Errorf("Parse() facility = %v, want %v", entry.Facility, tt.wantFacility)
			}
			
			if entry.Severity != tt.wantSeverity {
				t.Errorf("Parse() severity = %v, want %v", entry.Severity, tt.wantSeverity)
			}
			
			if entry.Version != tt.wantVersion {
				t.Errorf("Parse() version = %v, want %v", entry.Version, tt.wantVersion)
			}
			
			if entry.Hostname != tt.wantHostname {
				t.Errorf("Parse() hostname = %v, want %v", entry.Hostname, tt.wantHostname)
			}
			
			if entry.AppName != tt.wantAppName {
				t.Errorf("Parse() app_name = %v, want %v", entry.AppName, tt.wantAppName)
			}
			
			if entry.ProcID != tt.wantProcID {
				t.Errorf("Parse() proc_id = %v, want %v", entry.ProcID, tt.wantProcID)
			}
			
			if entry.MsgID != tt.wantMsgID {
				t.Errorf("Parse() msg_id = %v, want %v", entry.MsgID, tt.wantMsgID)
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

func TestRFC5424Parser_Parse_StructuredData(t *testing.T) {
	parser := NewRFC5424Parser(true)
	
	tests := []struct {
		name           string
		rawMessage     string
		wantSDElements int
		wantSDID       string
		wantParams     map[string]string
	}{
		{
			name:           "single structured data element",
			rawMessage:     `<165>1 2023-10-15T14:30:45Z web01 nginx 1234 access [exampleSDID@32473 iut="3" eventSource="Application"] Test message`,
			wantSDElements: 1,
			wantSDID:       "exampleSDID@32473",
			wantParams: map[string]string{
				"iut":         "3",
				"eventSource": "Application",
			},
		},
		{
			name:           "multiple structured data elements",
			rawMessage:     `<165>1 2023-10-15T14:30:45Z web01 nginx 1234 access [exampleSDID@32473 iut="3"][origin@32473 ip="192.168.1.1"] Test message`,
			wantSDElements: 2,
		},
		{
			name:           "no structured data",
			rawMessage:     `<165>1 2023-10-15T14:30:45Z web01 nginx 1234 access - Test message`,
			wantSDElements: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.rawMessage)
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if tt.wantSDElements == 0 {
				if entry.StructuredData != nil {
					t.Errorf("Expected no structured data, got %v", entry.StructuredData)
				}
				return
			}
			
			if entry.StructuredData == nil {
				t.Errorf("Expected structured data, got nil")
				return
			}
			
			if len(entry.StructuredData) != tt.wantSDElements {
				t.Errorf("Expected %d structured data elements, got %d", tt.wantSDElements, len(entry.StructuredData))
			}
			
			if tt.wantSDID != "" {
				sdElement, exists := entry.StructuredData[tt.wantSDID]
				if !exists {
					t.Errorf("Expected structured data element %s not found", tt.wantSDID)
					return
				}
				
				params, ok := sdElement.(map[string]string)
				if !ok {
					t.Errorf("Structured data element is not map[string]string")
					return
				}
				
				for key, expectedValue := range tt.wantParams {
					if actualValue, exists := params[key]; !exists || actualValue != expectedValue {
						t.Errorf("Expected param %s=%s, got %s=%s", key, expectedValue, key, actualValue)
					}
				}
			}
		})
	}
}

func TestRFC5424Parser_Parse_InvalidMessages(t *testing.T) {
	strictParser := NewRFC5424Parser(true)
	lenientParser := NewRFC5424Parser(false)
	
	tests := []struct {
		name       string
		rawMessage string
		strictErr  bool
		lenientErr bool
	}{
		{
			name:       "missing PRI",
			rawMessage: "1 2023-10-15T14:30:45Z web01 nginx 1234 access - Test message",
			strictErr:  true,
			lenientErr: false,
		},
		{
			name:       "invalid PRI format",
			rawMessage: "<abc>1 2023-10-15T14:30:45Z web01 nginx 1234 access - Test message",
			strictErr:  true,
			lenientErr: false,
		},
		{
			name:       "invalid version",
			rawMessage: "<165>2 2023-10-15T14:30:45Z web01 nginx 1234 access - Test message",
			strictErr:  true,
			lenientErr: false,
		},
		{
			name:       "invalid timestamp",
			rawMessage: "<165>1 invalid-timestamp web01 nginx 1234 access - Test message",
			strictErr:  true,
			lenientErr: false,
		},
		{
			name:       "empty message",
			rawMessage: "",
			strictErr:  true,
			lenientErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name+" (strict)", func(t *testing.T) {
			_, err := strictParser.Parse(tt.rawMessage)
			if tt.strictErr && err == nil {
				t.Errorf("Expected error in strict mode but got none")
			}
			if !tt.strictErr && err != nil {
				t.Errorf("Unexpected error in strict mode: %v", err)
			}
		})
		
		t.Run(tt.name+" (lenient)", func(t *testing.T) {
			_, err := lenientParser.Parse(tt.rawMessage)
			if tt.lenientErr && err == nil {
				t.Errorf("Expected error in lenient mode but got none")
			}
			if !tt.lenientErr && err != nil {
				t.Errorf("Unexpected error in lenient mode: %v", err)
			}
		})
	}
}

func TestRFC5424Parser_Parse_TimestampFormats(t *testing.T) {
	parser := NewRFC5424Parser(true)
	
	tests := []struct {
		name      string
		timestamp string
		wantValid bool
	}{
		{
			name:      "RFC3339 with nanoseconds",
			timestamp: "2023-10-15T14:30:45.123456789Z",
			wantValid: true,
		},
		{
			name:      "RFC3339 with milliseconds",
			timestamp: "2023-10-15T14:30:45.123Z",
			wantValid: true,
		},
		{
			name:      "RFC3339 without fractional seconds",
			timestamp: "2023-10-15T14:30:45Z",
			wantValid: true,
		},
		{
			name:      "RFC3339 with timezone",
			timestamp: "2023-10-15T14:30:45+02:00",
			wantValid: true,
		},
		{
			name:      "nil timestamp",
			timestamp: "-",
			wantValid: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawMessage := "<165>1 " + tt.timestamp + " web01 nginx 1234 access - Test message"
			entry, err := parser.Parse(rawMessage)
			
			if !tt.wantValid {
				if err == nil {
					t.Errorf("Expected error for invalid timestamp but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if entry.Timestamp.IsZero() && tt.timestamp != "-" {
				t.Errorf("Parse() timestamp should not be zero for valid timestamp")
			}
		})
	}
}

func TestRFC5424Parser_Parse_PriorityCalculation(t *testing.T) {
	parser := NewRFC5424Parser(true)
	
	tests := []struct {
		priority int
		facility int
		severity int
	}{
		{0, 0, 0},     // facility 0, severity 0
		{7, 0, 7},     // facility 0, severity 7
		{8, 1, 0},     // facility 1, severity 0
		{15, 1, 7},    // facility 1, severity 7
		{134, 16, 6},  // facility 16, severity 6
		{191, 23, 7},  // facility 23, severity 7
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("priority_%d", tt.priority), func(t *testing.T) {
			rawMessage := fmt.Sprintf("<%d>1 2023-10-15T14:30:45Z web01 nginx 1234 access - Test message", tt.priority)
			entry, err := parser.Parse(rawMessage)
			
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			
			if entry.Priority != tt.priority {
				t.Errorf("Expected priority %d, got %d", tt.priority, entry.Priority)
			}
			
			if entry.Facility != tt.facility {
				t.Errorf("Expected facility %d, got %d", tt.facility, entry.Facility)
			}
			
			if entry.Severity != tt.severity {
				t.Errorf("Expected severity %d, got %d", tt.severity, entry.Severity)
			}
		})
	}
}

func TestRFC5424Parser_FallbackParse(t *testing.T) {
	parser := NewRFC5424Parser(false) // lenient mode
	
	rawMessage := "This is not a valid RFC5424 message"
	entry, err := parser.Parse(rawMessage)
	
	if err != nil {
		t.Errorf("Fallback parse should not return error, got: %v", err)
	}
	
	if entry == nil {
		t.Fatal("Fallback parse returned nil entry")
	}
	
	if entry.Message != rawMessage {
		t.Errorf("Expected fallback message to be original message, got: %s", entry.Message)
	}
	
	if entry.Version != 1 {
		t.Errorf("Expected fallback version to be 1, got: %d", entry.Version)
	}
	
	if entry.Priority != 134 { // default priority
		t.Errorf("Expected fallback priority to be 134, got: %d", entry.Priority)
	}
}