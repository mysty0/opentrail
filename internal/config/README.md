# Configuration Package

This package provides configuration management for the OpenTrail application, supporting both command-line flags and environment variables with sensible defaults.

## Usage

```go
import "opentrail/internal/config"

// Load configuration with defaults, command-line flags, and environment variables
config, err := config.LoadConfig()
if err != nil {
    log.Fatalf("Failed to load configuration: %v", err)
}
```

## Configuration Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `-tcp-port` | `OPENTRAIL_TCP_PORT` | `2253` | TCP port for log ingestion |
| `-http-port` | `OPENTRAIL_HTTP_PORT` | `8080` | HTTP port for web interface |
| `-database-path` | `OPENTRAIL_DATABASE_PATH` | `logs.db` | Path to SQLite database file |
| `-log-format` | `OPENTRAIL_LOG_FORMAT` | `{{timestamp}}\|{{level}}\|{{tracking_id}}\|{{message}}` | Log parsing format |
| `-retention-days` | `OPENTRAIL_RETENTION_DAYS` | `30` | Number of days to retain logs |
| `-max-connections` | `OPENTRAIL_MAX_CONNECTIONS` | `100` | Maximum concurrent TCP connections |
| `-auth-username` | `OPENTRAIL_AUTH_USERNAME` | `""` | Username for HTTP Basic Auth |
| `-auth-password` | `OPENTRAIL_AUTH_PASSWORD` | `""` | Password for HTTP Basic Auth |
| `-auth-enabled` | `OPENTRAIL_AUTH_ENABLED` | `false` | Enable HTTP Basic Authentication |

## Priority Order

Configuration values are loaded in the following priority order (highest to lowest):

1. Environment variables
2. Command-line flags
3. Default values

## Validation

The configuration is automatically validated with the following rules:

- TCP and HTTP ports must be between 1 and 65535
- TCP and HTTP ports must be different
- Database path cannot be empty
- Log format must contain the `{{message}}` placeholder
- Retention days must be at least 1
- Max connections must be at least 1
- If authentication is enabled, both username and password must be provided
- Authentication is automatically enabled if both username and password are provided

## Examples

### Command Line Usage

```bash
# Basic usage with defaults
./opentrail

# Custom ports
./opentrail -tcp-port 9999 -http-port 8888

# Enable authentication
./opentrail -auth-username admin -auth-password secret -auth-enabled

# Custom database location
./opentrail -database-path /var/log/opentrail.db
```

### Environment Variables

```bash
# Set via environment variables
export OPENTRAIL_TCP_PORT=9999
export OPENTRAIL_HTTP_PORT=8888
export OPENTRAIL_AUTH_USERNAME=admin
export OPENTRAIL_AUTH_PASSWORD=secret
export OPENTRAIL_AUTH_ENABLED=true
./opentrail
```

### Mixed Configuration

```bash
# Environment variables override flags
export OPENTRAIL_TCP_PORT=9999
./opentrail -tcp-port 8888  # TCP port will be 9999 (from env)
```