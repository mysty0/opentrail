package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"opentrail/internal/types"
)

// LoadConfig loads configuration from command-line flags and environment variables
// with sensible defaults
func LoadConfig() (*types.Config, error) {
	return LoadConfigWithFlagSet(flag.CommandLine)
}

// LoadConfigWithFlagSet loads configuration using a specific flag set
// This allows for better testing by avoiding global flag conflicts
func LoadConfigWithFlagSet(fs *flag.FlagSet) (*types.Config, error) {
	config := &types.Config{}

	// Define command-line flags with defaults
	tcpPort := fs.Int("tcp-port", 2253, "TCP port for log ingestion")
	httpPort := fs.Int("http-port", 8080, "HTTP port for web interface")
	databasePath := fs.String("database-path", "logs.db", "Path to SQLite database file")
	logFormat := fs.String("log-format", "{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}", "Log parsing format")
	retentionDays := fs.Int("retention-days", 30, "Number of days to retain logs")
	maxConnections := fs.Int("max-connections", 100, "Maximum number of concurrent TCP connections")
	authUsername := fs.String("auth-username", "", "Username for HTTP Basic Auth (empty disables auth)")
	authPassword := fs.String("auth-password", "", "Password for HTTP Basic Auth")
	authEnabled := fs.Bool("auth-enabled", false, "Enable HTTP Basic Authentication")

	// Only parse if this is the global command line
	if fs == flag.CommandLine {
		fs.Parse(os.Args[1:])
	}

	// Load from environment variables (override flags)
	config.TCPPort = getIntFromEnv("OPENTRAIL_TCP_PORT", *tcpPort)
	config.HTTPPort = getIntFromEnv("OPENTRAIL_HTTP_PORT", *httpPort)
	config.DatabasePath = getStringFromEnv("OPENTRAIL_DATABASE_PATH", *databasePath)
	config.LogFormat = getStringFromEnv("OPENTRAIL_LOG_FORMAT", *logFormat)
	config.RetentionDays = getIntFromEnv("OPENTRAIL_RETENTION_DAYS", *retentionDays)
	config.MaxConnections = getIntFromEnv("OPENTRAIL_MAX_CONNECTIONS", *maxConnections)
	config.AuthUsername = getStringFromEnv("OPENTRAIL_AUTH_USERNAME", *authUsername)
	config.AuthPassword = getStringFromEnv("OPENTRAIL_AUTH_PASSWORD", *authPassword)
	config.AuthEnabled = getBoolFromEnv("OPENTRAIL_AUTH_ENABLED", *authEnabled)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// validateConfig validates the configuration and applies business rules
func validateConfig(config *types.Config) error {
	// Validate port ranges
	if config.TCPPort < 1 || config.TCPPort > 65535 {
		return fmt.Errorf("tcp-port must be between 1 and 65535, got %d", config.TCPPort)
	}
	if config.HTTPPort < 1 || config.HTTPPort > 65535 {
		return fmt.Errorf("http-port must be between 1 and 65535, got %d", config.HTTPPort)
	}

	// Ensure ports are different
	if config.TCPPort == config.HTTPPort {
		return fmt.Errorf("tcp-port and http-port cannot be the same (%d)", config.TCPPort)
	}

	// Validate database path is not empty
	if strings.TrimSpace(config.DatabasePath) == "" {
		return fmt.Errorf("database-path cannot be empty")
	}

	// Validate log format contains required placeholders
	if !strings.Contains(config.LogFormat, "{{message}}") {
		return fmt.Errorf("log-format must contain {{message}} placeholder")
	}

	// Validate retention days
	if config.RetentionDays < 1 {
		return fmt.Errorf("retention-days must be at least 1, got %d", config.RetentionDays)
	}

	// Validate max connections
	if config.MaxConnections < 1 {
		return fmt.Errorf("max-connections must be at least 1, got %d", config.MaxConnections)
	}

	// Validate authentication settings
	if config.AuthEnabled {
		if strings.TrimSpace(config.AuthUsername) == "" {
			return fmt.Errorf("auth-username cannot be empty when auth-enabled is true")
		}
		if strings.TrimSpace(config.AuthPassword) == "" {
			return fmt.Errorf("auth-password cannot be empty when auth-enabled is true")
		}
	}

	// Auto-enable auth if both username and password are provided
	if !config.AuthEnabled && config.AuthUsername != "" && config.AuthPassword != "" {
		config.AuthEnabled = true
	}

	return nil
}

// Helper functions for environment variable parsing

func getStringFromEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntFromEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolFromEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}