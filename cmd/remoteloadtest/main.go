package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"opentrail/internal/metrics"
	"opentrail/internal/types"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	targetHost  = flag.String("host", "localhost", "Target OpenTrail host")
	httpPort    = flag.Int("http-port", 8080, "Target HTTP port")
	tcpPort     = flag.Int("tcp-port", 514, "Target TCP port")
	duration    = flag.Duration("duration", 60*time.Second, "Test duration")
	writers     = flag.Int("writers", 10, "Number of concurrent writers")
	readers     = flag.Int("readers", 5, "Number of concurrent readers")
	metricsPort = flag.Int("metrics-port", 8081, "Local metrics server port")
	logInterval = flag.Duration("log-interval", 5*time.Second, "Stats logging interval")
	protocol    = flag.String("protocol", "tcp", "Protocol to use for sending logs (tcp or http)")
	writeDelay  = flag.Duration("write-delay", 1*time.Millisecond, "Delay between writes")
	readDelay   = flag.Duration("read-delay", 10*time.Millisecond, "Delay between reads")
)

type RemoteClient struct {
	httpBaseURL string
	tcpAddr     string
	httpClient  *http.Client
}

type TCPWriter struct {
	conn net.Conn
	mu   sync.Mutex
}

func main() {
	flag.Parse()

	log.Printf("Starting remote load test against %s (HTTP:%d, TCP:%d) with %d writers, %d readers for %v",
		*targetHost, *httpPort, *tcpPort, *writers, *readers, *duration)

	// Start local metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Local metrics server starting on port %d", *metricsPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *metricsPort), nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Create remote client
	client := &RemoteClient{
		httpBaseURL: fmt.Sprintf("http://%s:%d", *targetHost, *httpPort),
		tcpAddr:     fmt.Sprintf("%s:%d", *targetHost, *tcpPort),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Test connectivity
	if err := client.testConnectivity(); err != nil {
		log.Fatalf("Failed to connect to target: %v", err)
	}

	// Create metrics
	storageMetrics := metrics.NewStorageMetrics()
	monitor := metrics.NewPerformanceMonitor(storageMetrics)

	// Start monitoring
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	go monitor.StartMonitoring(ctx, 1*time.Second)

	// Start stats logging
	go func() {
		ticker := time.NewTicker(*logInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				monitor.LogCurrentStats()
			}
		}
	}()

	// Run load test
	runRemoteLoadTest(ctx, client, monitor)

	log.Println("Remote load test completed")
	monitor.LogCurrentStats()
}

func (c *RemoteClient) testConnectivity() error {
	// Test HTTP connectivity
	resp, err := c.httpClient.Get(c.httpBaseURL + "/health")
	if err != nil {
		return fmt.Errorf("HTTP connectivity test failed: %v", err)
	}
	resp.Body.Close()

	// Test TCP connectivity
	conn, err := net.DialTimeout("tcp", c.tcpAddr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("TCP connectivity test failed: %v", err)
	}
	conn.Close()

	log.Printf("Successfully connected to target at %s", *targetHost)
	return nil
}

func runRemoteLoadTest(ctx context.Context, client *RemoteClient, monitor *metrics.PerformanceMonitor) {
	var wg sync.WaitGroup

	// Start writers
	for i := 0; i < *writers; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			runRemoteWriter(ctx, client, monitor, writerID)
		}(i)
	}

	// Start readers
	for i := 0; i < *readers; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			runRemoteReader(ctx, client, monitor, readerID)
		}(i)
	}

	wg.Wait()
}

func runRemoteWriter(ctx context.Context, client *RemoteClient, monitor *metrics.PerformanceMonitor, writerID int) {
	count := 0
	var tcpWriter *TCPWriter

	// For TCP protocol, establish persistent connection
	if *protocol == "tcp" {
		conn, err := net.DialTimeout("tcp", client.tcpAddr, 5*time.Second)
		if err != nil {
			log.Printf("Writer %d failed to establish TCP connection: %v", writerID, err)
			return
		}
		tcpWriter = &TCPWriter{conn: conn}
		defer func() {
			tcpWriter.conn.Close()
			log.Printf("Writer %d closed TCP connection after %d writes", writerID, count)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("Writer %d completed %d writes", writerID, count)
			return
		default:
			entry := generateLogEntry(writerID, count)

			start := time.Now()
			var err error

			if *protocol == "http" {
				err = client.sendLogHTTP(entry)
			} else {
				err = tcpWriter.sendLog(entry)
			}

			latency := time.Since(start)
			monitor.RecordWriteLatency(latency)

			if err != nil {
				log.Printf("Writer %d error: %v", writerID, err)
				// For TCP, try to reconnect on error
				if *protocol == "tcp" && tcpWriter != nil {
					log.Printf("Writer %d attempting to reconnect TCP connection", writerID)
					tcpWriter.conn.Close()
					if conn, reconnectErr := net.DialTimeout("tcp", client.tcpAddr, 5*time.Second); reconnectErr == nil {
						tcpWriter.conn = conn
						log.Printf("Writer %d successfully reconnected", writerID)
					} else {
						log.Printf("Writer %d failed to reconnect: %v", writerID, reconnectErr)
						return
					}
				}
			} else {
				count++
			}

			time.Sleep(*writeDelay)
		}
	}
}

func runRemoteReader(ctx context.Context, client *RemoteClient, monitor *metrics.PerformanceMonitor, readerID int) {
	count := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("Reader %d completed %d reads", readerID, count)
			return
		default:
			query := generateSearchQuery(readerID, count)

			start := time.Now()
			_, err := client.searchLogs(query)
			latency := time.Since(start)

			monitor.RecordReadLatency(latency)

			if err != nil {
				log.Printf("Reader %d error: %v", readerID, err)
			} else {
				count++
			}

			time.Sleep(*readDelay)
		}
	}
}

func (c *RemoteClient) sendLogHTTP(entry *types.LogEntry) error {
	// Convert to syslog format for HTTP endpoint
	syslogMsg := formatAsSyslog(entry)

	req, err := http.NewRequest("POST", c.httpBaseURL+"/logs", bytes.NewBufferString(syslogMsg))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *RemoteClient) sendLogTCP(entry *types.LogEntry) error {
	conn, err := net.DialTimeout("tcp", c.tcpAddr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	syslogMsg := formatAsSyslog(entry)
	_, err = conn.Write([]byte(syslogMsg + "\n"))
	return err
}

func (w *TCPWriter) sendLog(entry *types.LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	syslogMsg := formatAsSyslog(entry)
	_, err := w.conn.Write([]byte(syslogMsg + "\n"))
	return err
}

func (c *RemoteClient) searchLogs(query types.SearchQuery) ([]*types.LogEntry, error) {
	// Build query parameters
	params := fmt.Sprintf("?limit=%d", query.Limit)
	if query.Text != "" {
		params += fmt.Sprintf("&q=%s", query.Text)
	}
	if query.Hostname != "" {
		params += fmt.Sprintf("&hostname=%s", query.Hostname)
	}
	if query.AppName != "" {
		params += fmt.Sprintf("&appname=%s", query.AppName)
	}

	resp, err := c.httpClient.Get(c.httpBaseURL + "/api/search" + params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request failed with status: %d", resp.StatusCode)
	}

	var results []*types.LogEntry
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

func formatAsSyslog(entry *types.LogEntry) string {
	timestamp := entry.Timestamp.Format(time.RFC3339)

	// Basic syslog format: <priority>version timestamp hostname appname procid msgid message
	return fmt.Sprintf("<%d>%d %s %s %s %s %s %s",
		entry.Priority,
		entry.Version,
		timestamp,
		entry.Hostname,
		entry.AppName,
		entry.ProcID,
		entry.MsgID,
		entry.Message,
	)
}
func generateLogEntry(writerID, count int) *types.LogEntry {
	facilities := []int{0, 1, 2, 3, 16, 17, 18, 19}
	severities := []int{0, 1, 2, 3, 4, 5, 6, 7}
	hostnames := []string{"web01", "web02", "db01", "cache01", "api01"}
	appNames := []string{"nginx", "mysql", "redis", "api-server", "worker"}

	facility := facilities[rand.Intn(len(facilities))]
	severity := severities[rand.Intn(len(severities))]
	priority := facility*8 + severity

	return &types.LogEntry{
		Priority:  priority,
		Facility:  facility,
		Severity:  severity,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  hostnames[rand.Intn(len(hostnames))],
		AppName:   appNames[rand.Intn(len(appNames))],
		ProcID:    fmt.Sprintf("%d", writerID),
		MsgID:     fmt.Sprintf("remotetest-%d", count),
		Message:   fmt.Sprintf("Remote load test message from writer %d, count %d: %s", writerID, count, generateRandomText()),
		StructuredData: map[string]interface{}{
			"writer_id":  writerID,
			"count":      count,
			"timestamp":  time.Now().Unix(),
			"random_val": rand.Float64(),
			"test_type":  "remote_load_test",
		},
	}
}

func generateSearchQuery(readerID, count int) types.SearchQuery {
	queries := []types.SearchQuery{
		{Text: "remote load test", Limit: 10},
		{Hostname: "web01", Limit: 5},
		{AppName: "nginx", Limit: 5},
		{Limit: 20},
		{Text: "message", Limit: 15},
		{Text: "error", Limit: 8},
		{Hostname: "db01", AppName: "mysql", Limit: 3},
	}

	return queries[count%len(queries)]
}

func generateRandomText() string {
	words := []string{
		"processing", "request", "response", "error", "success", "timeout", "connection",
		"database", "query", "transaction", "batch", "cache", "memory", "disk", "network",
		"authentication", "authorization", "validation", "parsing", "encoding", "decoding",
		"remote", "client", "server", "latency", "throughput", "performance", "monitoring",
	}

	text := ""
	numWords := 3 + rand.Intn(5) // 3-7 words
	for i := 0; i < numWords; i++ {
		if i > 0 {
			text += " "
		}
		text += words[rand.Intn(len(words))]
	}

	return text
}
