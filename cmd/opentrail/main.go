package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"opentrail/internal/config"
	"opentrail/internal/interfaces"
	"opentrail/internal/parser"
	"opentrail/internal/server"
	"opentrail/internal/service"
	"opentrail/internal/storage"
	"opentrail/internal/types"
	"opentrail/web"
)

// Build information (set by build script)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Application represents the main application
type Application struct {
	config     *types.Config
	storage    interfaces.LogStorage
	parser     interfaces.LogParser
	logService interfaces.LogService
	tcpServer  *server.TCPServer
	httpServer *server.HTTPServer
	
	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[OpenTrail] ")
	
	// Display version information
	log.Printf("OpenTrail v%s (built %s, commit %s)", Version, BuildTime, GitCommit)
	
	// Create application instance
	app, err := NewApplication()
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}
	
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start the application
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
	
	log.Printf("OpenTrail started successfully")
	log.Printf("TCP server listening on port %d", app.config.TCPPort)
	log.Printf("Web interface available at http://localhost:%d", app.config.HTTPPort)
	if app.config.AuthEnabled {
		log.Printf("Authentication enabled for web interface")
	}
	
	// Wait for shutdown signal
	<-sigChan
	log.Printf("Shutdown signal received, stopping application...")
	
	// Graceful shutdown
	if err := app.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
		os.Exit(1)
	}
	
	log.Printf("OpenTrail stopped successfully")
}

// NewApplication creates a new application instance
func NewApplication() (*Application, error) {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Create context for application lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	
	app := &Application{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
	
	// Initialize components
	if err := app.initializeComponents(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}
	
	return app, nil
}

// initializeComponents initializes all application components
func (app *Application) initializeComponents() error {
	// Initialize storage
	sqliteStorage, err := storage.NewSQLiteStorage(app.config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	app.storage = sqliteStorage
	
	// Initialize parser
	logParser := parser.NewDefaultLogParser()
	if err := logParser.SetFormat(app.config.LogFormat); err != nil {
		return fmt.Errorf("failed to set log format: %w", err)
	}
	app.parser = logParser
	
	// Initialize log service
	logService := service.NewLogService(logParser, sqliteStorage)
	app.logService = logService
	
	// Initialize TCP server
	tcpServer := server.NewTCPServer(app.config, logService)
	app.tcpServer = tcpServer
	
	// Initialize HTTP server with embedded static files
	httpServer := server.NewHTTPServerWithStaticFiles(app.config, logService, web.GetStaticFS())
	app.httpServer = httpServer
	
	return nil
}

// Start starts all application components
func (app *Application) Start() error {
	log.Printf("Starting OpenTrail components...")
	
	// Start log service first
	if err := app.logService.Start(); err != nil {
		return fmt.Errorf("failed to start log service: %w", err)
	}
	
	// Start TCP server
	if err := app.tcpServer.Start(); err != nil {
		app.logService.Stop()
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	
	// Start HTTP server
	if err := app.httpServer.Start(); err != nil {
		app.tcpServer.Stop()
		app.logService.Stop()
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	
	return nil
}

// Stop gracefully stops all application components
func (app *Application) Stop() error {
	log.Printf("Stopping OpenTrail components...")
	
	// Cancel application context
	app.cancel()
	
	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	// Stop components in reverse order
	var errors []error
	
	// Stop HTTP server
	if app.httpServer != nil {
		if err := app.httpServer.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("HTTP server stop error: %w", err))
		}
	}
	
	// Stop TCP server
	if app.tcpServer != nil {
		if err := app.tcpServer.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("TCP server stop error: %w", err))
		}
	}
	
	// Stop log service
	if app.logService != nil {
		if err := app.logService.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("log service stop error: %w", err))
		}
	}
	
	// Close storage
	if app.storage != nil {
		if err := app.storage.Close(); err != nil {
			errors = append(errors, fmt.Errorf("storage close error: %w", err))
		}
	}
	
	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines finished
	case <-shutdownCtx.Done():
		errors = append(errors, fmt.Errorf("shutdown timeout exceeded"))
	}
	
	// Return combined errors if any
	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}
	
	return nil
}

// GetStats returns application statistics
func (app *Application) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	if app.logService != nil {
		stats["log_service"] = app.logService.GetStats()
	}
	
	if app.tcpServer != nil {
		stats["tcp_server"] = app.tcpServer.GetStats()
	}
	
	if app.httpServer != nil {
		stats["http_server"] = app.httpServer.GetStats()
	}
	
	return stats
}