package interfaces

import "opentrail/internal/types"

// LogParser defines the interface for parsing raw log messages
type LogParser interface {
	// Parse converts a raw log message string into a LogEntry
	Parse(rawMessage string) (*types.LogEntry, error)
	
	// SetFormat configures the parser to use a specific log format
	SetFormat(format string) error
}