package config

import (
	"fmt"
	"os"
)

// ExampleLoadConfig demonstrates how to use the configuration loading
func ExampleLoadConfig() {
	// Set some environment variables for demonstration
	os.Setenv("OPENTRAIL_TCP_PORT", "3000")
	os.Setenv("OPENTRAIL_AUTH_USERNAME", "admin")
	os.Setenv("OPENTRAIL_AUTH_PASSWORD", "secret")
	os.Setenv("OPENTRAIL_AUTH_ENABLED", "true")
	defer func() {
		os.Unsetenv("OPENTRAIL_TCP_PORT")
		os.Unsetenv("OPENTRAIL_AUTH_USERNAME")
		os.Unsetenv("OPENTRAIL_AUTH_PASSWORD")
		os.Unsetenv("OPENTRAIL_AUTH_ENABLED")
	}()

	// Load configuration (this would normally be called from main)
	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// Display some configuration values
	fmt.Printf("TCP Port: %d\n", config.TCPPort)
	fmt.Printf("HTTP Port: %d\n", config.HTTPPort)
	fmt.Printf("Database Path: %s\n", config.DatabasePath)
	fmt.Printf("Auth Enabled: %t\n", config.AuthEnabled)

	// Output:
	// TCP Port: 3000
	// HTTP Port: 8080
	// Database Path: logs.db
	// Auth Enabled: true
}