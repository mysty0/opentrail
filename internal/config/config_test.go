package config

import (
	"flag"
	"os"
	"testing"

	"opentrail/internal/types"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Create a new flag set for testing
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	
	// Clear environment variables
	clearTestEnvVars()

	config, err := LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify default values
	if config.TCPPort != 2253 {
		t.Errorf("Expected TCPPort 2253, got %d", config.TCPPort)
	}
	if config.HTTPPort != 8080 {
		t.Errorf("Expected HTTPPort 8080, got %d", config.HTTPPort)
	}
	if config.WebSocketPort != 8081 {
		t.Errorf("Expected WebSocketPort 8081, got %d", config.WebSocketPort)
	}
	if config.DatabasePath != "logs.db" {
		t.Errorf("Expected DatabasePath 'logs.db', got '%s'", config.DatabasePath)
	}
	if config.LogFormat != "{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}" {
		t.Errorf("Expected default LogFormat, got '%s'", config.LogFormat)
	}
	if config.RetentionDays != 30 {
		t.Errorf("Expected RetentionDays 30, got %d", config.RetentionDays)
	}
	if config.MaxConnections != 100 {
		t.Errorf("Expected MaxConnections 100, got %d", config.MaxConnections)
	}
	if config.AuthUsername != "" {
		t.Errorf("Expected empty AuthUsername, got '%s'", config.AuthUsername)
	}
	if config.AuthPassword != "" {
		t.Errorf("Expected empty AuthPassword, got '%s'", config.AuthPassword)
	}
	if config.AuthEnabled != false {
		t.Errorf("Expected AuthEnabled false, got %t", config.AuthEnabled)
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	// Create a new flag set for testing
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	
	// Set environment variables
	os.Setenv("OPENTRAIL_TCP_PORT", "9999")
	os.Setenv("OPENTRAIL_HTTP_PORT", "8888")
	os.Setenv("OPENTRAIL_WEBSOCKET_PORT", "7777")
	os.Setenv("OPENTRAIL_DATABASE_PATH", "/tmp/test.db")
	os.Setenv("OPENTRAIL_LOG_FORMAT", "{{level}}: {{message}}")
	os.Setenv("OPENTRAIL_RETENTION_DAYS", "60")
	os.Setenv("OPENTRAIL_MAX_CONNECTIONS", "200")
	os.Setenv("OPENTRAIL_AUTH_USERNAME", "testuser")
	os.Setenv("OPENTRAIL_AUTH_PASSWORD", "testpass")
	os.Setenv("OPENTRAIL_AUTH_ENABLED", "true")

	defer clearTestEnvVars()

	config, err := LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify environment variable values
	if config.TCPPort != 9999 {
		t.Errorf("Expected TCPPort 9999, got %d", config.TCPPort)
	}
	if config.HTTPPort != 8888 {
		t.Errorf("Expected HTTPPort 8888, got %d", config.HTTPPort)
	}
	if config.WebSocketPort != 7777 {
		t.Errorf("Expected WebSocketPort 7777, got %d", config.WebSocketPort)
	}
	if config.DatabasePath != "/tmp/test.db" {
		t.Errorf("Expected DatabasePath '/tmp/test.db', got '%s'", config.DatabasePath)
	}
	if config.LogFormat != "{{level}}: {{message}}" {
		t.Errorf("Expected LogFormat '{{level}}: {{message}}', got '%s'", config.LogFormat)
	}
	if config.RetentionDays != 60 {
		t.Errorf("Expected RetentionDays 60, got %d", config.RetentionDays)
	}
	if config.MaxConnections != 200 {
		t.Errorf("Expected MaxConnections 200, got %d", config.MaxConnections)
	}
	if config.AuthUsername != "testuser" {
		t.Errorf("Expected AuthUsername 'testuser', got '%s'", config.AuthUsername)
	}
	if config.AuthPassword != "testpass" {
		t.Errorf("Expected AuthPassword 'testpass', got '%s'", config.AuthPassword)
	}
	if config.AuthEnabled != true {
		t.Errorf("Expected AuthEnabled true, got %t", config.AuthEnabled)
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{timestamp}}|{{level}}|{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
		AuthUsername:   "admin",
		AuthPassword:   "secret",
		AuthEnabled:    true,
	}

	err := validateConfig(config)
	if err != nil {
		t.Errorf("validateConfig() failed for valid config: %v", err)
	}
}

func TestValidateConfig_InvalidTCPPort(t *testing.T) {
	config := &types.Config{
		TCPPort:        0,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for invalid TCP port")
	}
	if !contains(err.Error(), "tcp-port must be between 1 and 65535") {
		t.Errorf("Expected TCP port validation error, got: %v", err)
	}
}

func TestValidateConfig_InvalidHTTPPort(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       70000,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for invalid HTTP port")
	}
	if !contains(err.Error(), "http-port must be between 1 and 65535") {
		t.Errorf("Expected HTTP port validation error, got: %v", err)
	}
}

func TestValidateConfig_SamePorts(t *testing.T) {
	config := &types.Config{
		TCPPort:        8080,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for same TCP and HTTP ports")
	}
	if !contains(err.Error(), "tcp-port and http-port cannot be the same") {
		t.Errorf("Expected same ports validation error, got: %v", err)
	}
}

func TestValidateConfig_EmptyDatabasePath(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "   ",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for empty database path")
	}
	if !contains(err.Error(), "database-path cannot be empty") {
		t.Errorf("Expected database path validation error, got: %v", err)
	}
}

func TestValidateConfig_InvalidLogFormat(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{timestamp}}|{{level}}",
		RetentionDays:  30,
		MaxConnections: 100,
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for log format without {{message}}")
	}
	if !contains(err.Error(), "log-format must contain {{message}} placeholder") {
		t.Errorf("Expected log format validation error, got: %v", err)
	}
}

func TestValidateConfig_InvalidRetentionDays(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  0,
		MaxConnections: 100,
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for invalid retention days")
	}
	if !contains(err.Error(), "retention-days must be at least 1") {
		t.Errorf("Expected retention days validation error, got: %v", err)
	}
}

func TestValidateConfig_InvalidMaxConnections(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 0,
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for invalid max connections")
	}
	if !contains(err.Error(), "max-connections must be at least 1") {
		t.Errorf("Expected max connections validation error, got: %v", err)
	}
}

func TestValidateConfig_AuthEnabledWithoutUsername(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
		AuthEnabled:    true,
		AuthPassword:   "secret",
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for auth enabled without username")
	}
	if !contains(err.Error(), "auth-username cannot be empty when auth-enabled is true") {
		t.Errorf("Expected auth username validation error, got: %v", err)
	}
}

func TestValidateConfig_AuthEnabledWithoutPassword(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
		AuthEnabled:    true,
		AuthUsername:   "admin",
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() should fail for auth enabled without password")
	}
	if !contains(err.Error(), "auth-password cannot be empty when auth-enabled is true") {
		t.Errorf("Expected auth password validation error, got: %v", err)
	}
}

func TestValidateConfig_AutoEnableAuth(t *testing.T) {
	config := &types.Config{
		TCPPort:        2253,
		HTTPPort:       8080,
		WebSocketPort:  8081,
		DatabasePath:   "logs.db",
		LogFormat:      "{{message}}",
		RetentionDays:  30,
		MaxConnections: 100,
		AuthEnabled:    false,
		AuthUsername:   "admin",
		AuthPassword:   "secret",
	}

	err := validateConfig(config)
	if err != nil {
		t.Errorf("validateConfig() failed: %v", err)
	}

	// Should auto-enable auth when both username and password are provided
	if !config.AuthEnabled {
		t.Error("Expected AuthEnabled to be auto-enabled when username and password are provided")
	}
}

func TestGetIntFromEnv_ValidValue(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	result := getIntFromEnv("TEST_INT", 10)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}
}

func TestGetIntFromEnv_InvalidValue(t *testing.T) {
	os.Setenv("TEST_INT", "invalid")
	defer os.Unsetenv("TEST_INT")

	result := getIntFromEnv("TEST_INT", 10)
	if result != 10 {
		t.Errorf("Expected default value 10, got %d", result)
	}
}

func TestGetIntFromEnv_MissingValue(t *testing.T) {
	result := getIntFromEnv("MISSING_INT", 15)
	if result != 15 {
		t.Errorf("Expected default value 15, got %d", result)
	}
}

func TestGetBoolFromEnv_ValidValues(t *testing.T) {
	testCases := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"TRUE", true},
		{"FALSE", false},
	}

	for _, tc := range testCases {
		os.Setenv("TEST_BOOL", tc.value)
		result := getBoolFromEnv("TEST_BOOL", false)
		if result != tc.expected {
			t.Errorf("For value '%s', expected %t, got %t", tc.value, tc.expected, result)
		}
		os.Unsetenv("TEST_BOOL")
	}
}

func TestGetBoolFromEnv_InvalidValue(t *testing.T) {
	os.Setenv("TEST_BOOL", "invalid")
	defer os.Unsetenv("TEST_BOOL")

	result := getBoolFromEnv("TEST_BOOL", true)
	if result != true {
		t.Errorf("Expected default value true, got %t", result)
	}
}

func TestGetStringFromEnv_ValidValue(t *testing.T) {
	os.Setenv("TEST_STRING", "hello")
	defer os.Unsetenv("TEST_STRING")

	result := getStringFromEnv("TEST_STRING", "default")
	if result != "hello" {
		t.Errorf("Expected 'hello', got '%s'", result)
	}
}

func TestGetStringFromEnv_MissingValue(t *testing.T) {
	result := getStringFromEnv("MISSING_STRING", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}
}

func TestLoadConfig_CommandLineFlags(t *testing.T) {
	// Create a new flag set for testing
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	
	// Clear environment variables
	clearTestEnvVars()

	// Define flags first (this will be done by LoadConfigWithFlagSet)
	tcpPort := fs.Int("tcp-port", 2253, "TCP port for log ingestion")
	httpPort := fs.Int("http-port", 8080, "HTTP port for web interface")
	webSocketPort := fs.Int("websocket-port", 8081, "WebSocket port for log ingestion")
	databasePath := fs.String("database-path", "logs.db", "Path to SQLite database file")
	logFormat := fs.String("log-format", "{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}", "Log parsing format")
	retentionDays := fs.Int("retention-days", 30, "Number of days to retain logs")
	maxConnections := fs.Int("max-connections", 100, "Maximum number of concurrent TCP connections")
	authUsername := fs.String("auth-username", "", "Username for HTTP Basic Auth (empty disables auth)")
	authPassword := fs.String("auth-password", "", "Password for HTTP Basic Auth")
	authEnabled := fs.Bool("auth-enabled", false, "Enable HTTP Basic Authentication")

	// Simulate command line arguments
	args := []string{
		"-tcp-port", "3000",
		"-http-port", "9000",
		"-websocket-port", "7777",
		"-database-path", "/custom/path.db",
		"-log-format", "{{level}}: {{message}}",
		"-retention-days", "45",
		"-max-connections", "150",
		"-auth-username", "admin",
		"-auth-password", "password123",
		"-auth-enabled",
	}

	err := fs.Parse(args)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Manually create config since we already parsed
	config := &types.Config{
		TCPPort:        getIntFromEnv("OPENTRAIL_TCP_PORT", *tcpPort),
		HTTPPort:       getIntFromEnv("OPENTRAIL_HTTP_PORT", *httpPort),
		WebSocketPort:  getIntFromEnv("OPENTRAIL_WEBSOCKET_PORT", *webSocketPort),
		DatabasePath:   getStringFromEnv("OPENTRAIL_DATABASE_PATH", *databasePath),
		LogFormat:      getStringFromEnv("OPENTRAIL_LOG_FORMAT", *logFormat),
		RetentionDays:  getIntFromEnv("OPENTRAIL_RETENTION_DAYS", *retentionDays),
		MaxConnections: getIntFromEnv("OPENTRAIL_MAX_CONNECTIONS", *maxConnections),
		AuthUsername:   getStringFromEnv("OPENTRAIL_AUTH_USERNAME", *authUsername),
		AuthPassword:   getStringFromEnv("OPENTRAIL_AUTH_PASSWORD", *authPassword),
		AuthEnabled:    getBoolFromEnv("OPENTRAIL_AUTH_ENABLED", *authEnabled),
	}

	err = validateConfig(config)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Verify flag values
	if config.TCPPort != 3000 {
		t.Errorf("Expected TCPPort 3000, got %d", config.TCPPort)
	}
	if config.HTTPPort != 9000 {
		t.Errorf("Expected HTTPPort 9000, got %d", config.HTTPPort)
	}
	if config.WebSocketPort != 7777 {
		t.Errorf("Expected WebSocketPort 7777, got %d", config.WebSocketPort)
	}
	if config.DatabasePath != "/custom/path.db" {
		t.Errorf("Expected DatabasePath '/custom/path.db', got '%s'", config.DatabasePath)
	}
	if config.LogFormat != "{{level}}: {{message}}" {
		t.Errorf("Expected LogFormat '{{level}}: {{message}}', got '%s'", config.LogFormat)
	}
	if config.RetentionDays != 45 {
		t.Errorf("Expected RetentionDays 45, got %d", config.RetentionDays)
	}
	if config.MaxConnections != 150 {
		t.Errorf("Expected MaxConnections 150, got %d", config.MaxConnections)
	}
	if config.AuthUsername != "admin" {
		t.Errorf("Expected AuthUsername 'admin', got '%s'", config.AuthUsername)
	}
	if config.AuthPassword != "password123" {
		t.Errorf("Expected AuthPassword 'password123', got '%s'", config.AuthPassword)
	}
	if config.AuthEnabled != true {
		t.Errorf("Expected AuthEnabled true, got %t", config.AuthEnabled)
	}
}

func TestLoadConfig_EnvironmentOverridesFlags(t *testing.T) {
	// Create a new flag set for testing
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	
	// Set environment variables
	os.Setenv("OPENTRAIL_TCP_PORT", "5555")
	os.Setenv("OPENTRAIL_HTTP_PORT", "6666")
	os.Setenv("OPENTRAIL_WEBSOCKET_PORT", "7777")
	defer clearTestEnvVars()

	// Define flags first
	tcpPort := fs.Int("tcp-port", 2253, "TCP port for log ingestion")
	httpPort := fs.Int("http-port", 8080, "HTTP port for web interface")
	webSocketPort := fs.Int("websocket-port", 8081, "WebSocket port for log ingestion")
	databasePath := fs.String("database-path", "logs.db", "Path to SQLite database file")
	logFormat := fs.String("log-format", "{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}", "Log parsing format")
	retentionDays := fs.Int("retention-days", 30, "Number of days to retain logs")
	maxConnections := fs.Int("max-connections", 100, "Maximum number of concurrent TCP connections")
	authUsername := fs.String("auth-username", "", "Username for HTTP Basic Auth (empty disables auth)")
	authPassword := fs.String("auth-password", "", "Password for HTTP Basic Auth")
	authEnabled := fs.Bool("auth-enabled", false, "Enable HTTP Basic Authentication")

	// Simulate command line arguments with different values
	args := []string{
		"-tcp-port", "3000",
		"-http-port", "9000",
	}

	err := fs.Parse(args)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Manually create config since we already parsed
	config := &types.Config{
		TCPPort:        getIntFromEnv("OPENTRAIL_TCP_PORT", *tcpPort),
		HTTPPort:       getIntFromEnv("OPENTRAIL_HTTP_PORT", *httpPort),
		WebSocketPort:  getIntFromEnv("OPENTRAIL_WEBSOCKET_PORT", *webSocketPort),
		DatabasePath:   getStringFromEnv("OPENTRAIL_DATABASE_PATH", *databasePath),
		LogFormat:      getStringFromEnv("OPENTRAIL_LOG_FORMAT", *logFormat),
		RetentionDays:  getIntFromEnv("OPENTRAIL_RETENTION_DAYS", *retentionDays),
		MaxConnections: getIntFromEnv("OPENTRAIL_MAX_CONNECTIONS", *maxConnections),
		AuthUsername:   getStringFromEnv("OPENTRAIL_AUTH_USERNAME", *authUsername),
		AuthPassword:   getStringFromEnv("OPENTRAIL_AUTH_PASSWORD", *authPassword),
		AuthEnabled:    getBoolFromEnv("OPENTRAIL_AUTH_ENABLED", *authEnabled),
	}

	err = validateConfig(config)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Environment variables should override flags
	if config.TCPPort != 5555 {
		t.Errorf("Expected TCPPort 5555 (from env), got %d", config.TCPPort)
	}
	if config.HTTPPort != 6666 {
		t.Errorf("Expected HTTPPort 6666 (from env), got %d", config.HTTPPort)
	}
	if config.WebSocketPort != 7777 {
		t.Errorf("Expected WebSocketPort 7777 (from env), got %d", config.WebSocketPort)
	}
}

// Helper functions for tests

func clearTestEnvVars() {
	envVars := []string{
		"OPENTRAIL_TCP_PORT",
		"OPENTRAIL_HTTP_PORT",
		"OPENTRAIL_WEBSOCKET_PORT",
		"OPENTRAIL_DATABASE_PATH",
		"OPENTRAIL_LOG_FORMAT",
		"OPENTRAIL_RETENTION_DAYS",
		"OPENTRAIL_MAX_CONNECTIONS",
		"OPENTRAIL_AUTH_USERNAME",
		"OPENTRAIL_AUTH_PASSWORD",
		"OPENTRAIL_AUTH_ENABLED",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}