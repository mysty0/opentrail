package types

// Config holds all configuration options for the application
type Config struct {
	TCPPort        int    `json:"tcp_port"`
	HTTPPort       int    `json:"http_port"`
	DatabasePath   string `json:"database_path"`
	LogFormat      string `json:"log_format"`
	RetentionDays  int    `json:"retention_days"`
	MaxConnections int    `json:"max_connections"`
	AuthUsername   string `json:"auth_username"`
	AuthPassword   string `json:"auth_password"`
	AuthEnabled    bool   `json:"auth_enabled"`
}