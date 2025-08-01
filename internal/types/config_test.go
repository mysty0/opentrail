package types

import (
	"encoding/json"
	"testing"
)

func TestConfig_JSONSerialization(t *testing.T) {
	config := &Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		DatabasePath:   "/tmp/logs.db",
		LogFormat:      "{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
		AuthUsername:   "admin",
		AuthPassword:   "secret",
		AuthEnabled:    true,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal Config: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Config
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Config: %v", err)
	}

	// Verify all fields
	if unmarshaled.TCPPort != config.TCPPort {
		t.Errorf("Expected TCPPort %d, got %d", config.TCPPort, unmarshaled.TCPPort)
	}
	if unmarshaled.HTTPPort != config.HTTPPort {
		t.Errorf("Expected HTTPPort %d, got %d", config.HTTPPort, unmarshaled.HTTPPort)
	}
	if unmarshaled.DatabasePath != config.DatabasePath {
		t.Errorf("Expected DatabasePath %s, got %s", config.DatabasePath, unmarshaled.DatabasePath)
	}
	if unmarshaled.LogFormat != config.LogFormat {
		t.Errorf("Expected LogFormat %s, got %s", config.LogFormat, unmarshaled.LogFormat)
	}
	if unmarshaled.RetentionDays != config.RetentionDays {
		t.Errorf("Expected RetentionDays %d, got %d", config.RetentionDays, unmarshaled.RetentionDays)
	}
	if unmarshaled.MaxConnections != config.MaxConnections {
		t.Errorf("Expected MaxConnections %d, got %d", config.MaxConnections, unmarshaled.MaxConnections)
	}
	if unmarshaled.AuthUsername != config.AuthUsername {
		t.Errorf("Expected AuthUsername %s, got %s", config.AuthUsername, unmarshaled.AuthUsername)
	}
	if unmarshaled.AuthPassword != config.AuthPassword {
		t.Errorf("Expected AuthPassword %s, got %s", config.AuthPassword, unmarshaled.AuthPassword)
	}
	if unmarshaled.AuthEnabled != config.AuthEnabled {
		t.Errorf("Expected AuthEnabled %t, got %t", config.AuthEnabled, unmarshaled.AuthEnabled)
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	config := &Config{}

	// Test that zero values are handled correctly
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal empty Config: %v", err)
	}

	var unmarshaled Config
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty Config: %v", err)
	}

	// Verify zero values
	if unmarshaled.TCPPort != 0 {
		t.Errorf("Expected TCPPort 0, got %d", unmarshaled.TCPPort)
	}
	if unmarshaled.HTTPPort != 0 {
		t.Errorf("Expected HTTPPort 0, got %d", unmarshaled.HTTPPort)
	}
	if unmarshaled.DatabasePath != "" {
		t.Errorf("Expected empty DatabasePath, got %s", unmarshaled.DatabasePath)
	}
	if unmarshaled.AuthEnabled != false {
		t.Errorf("Expected AuthEnabled false, got %t", unmarshaled.AuthEnabled)
	}
}