package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"opentrail/internal/metrics"
	"opentrail/internal/storage"
	"opentrail/internal/types"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dbPath       = flag.String("db", "loadtest.db", "Database path")
	duration     = flag.Duration("duration", 60*time.Second, "Test duration")
	writers      = flag.Int("writers", 10, "Number of concurrent writers")
	readers      = flag.Int("readers", 5, "Number of concurrent readers")
	batchSize    = flag.Int("batch-size", 100, "Batch size for writes")
	batchTimeout = flag.Duration("batch-timeout", 10*time.Millisecond, "Batch timeout")
	queueSize    = flag.Int("queue-size", 10000, "Queue size")
	metricsPort  = flag.Int("metrics-port", 8080, "Metrics server port")
	logInterval  = flag.Duration("log-interval", 5*time.Second, "Stats logging interval")
)

func main() {
	flag.Parse()

	log.Printf("Starting load test with %d writers, %d readers for %v", *writers, *readers, *duration)

	// Start metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Metrics server starting on port %d", *metricsPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *metricsPort), nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Create storage with batching
	config := storage.DefaultBatchConfig()
	config.BatchSize = *batchSize
	config.BatchTimeout = *batchTimeout
	config.QueueSize = *queueSize

	store, err := storage.NewBatchedSQLiteStorage(*dbPath, config)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	// Get the singleton metrics instance (already created by storage)
	storageMetrics := metrics.GetStorageMetrics()
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
	runLoadTest(ctx, store, monitor)

	log.Println("Load test completed")
	monitor.LogCurrentStats()
}

func runLoadTest(ctx context.Context, store interface{}, monitor *metrics.PerformanceMonitor) {
	var wg sync.WaitGroup

	// Start writers
	for i := 0; i < *writers; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			runWriter(ctx, store, monitor, writerID)
		}(i)
	}

	// Start readers
	for i := 0; i < *readers; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			runReader(ctx, store, monitor, readerID)
		}(i)
	}

	wg.Wait()
}

func runWriter(ctx context.Context, store interface{}, monitor *metrics.PerformanceMonitor, writerID int) {
	// Type assert to the storage interface
	storage := store.(interface {
		Store(*types.LogEntry) error
	})
	count := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("Writer %d completed %d writes", writerID, count)
			return
		default:
			entry := generateLogEntry(writerID, count)

			start := time.Now()
			err := storage.Store(entry)
			latency := time.Since(start)

			monitor.RecordWriteLatency(latency)

			if err != nil {
				log.Printf("Writer %d error: %v", writerID, err)
			} else {
				count++
			}

			// Small delay to prevent overwhelming
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func runReader(ctx context.Context, store interface{}, monitor *metrics.PerformanceMonitor, readerID int) {
	// Type assert to the storage interface
	storage := store.(interface {
		Search(types.SearchQuery) ([]*types.LogEntry, error)
	})
	count := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("Reader %d completed %d reads", readerID, count)
			return
		default:
			query := generateSearchQuery(readerID, count)

			start := time.Now()
			_, err := storage.Search(query)
			latency := time.Since(start)

			monitor.RecordReadLatency(latency)

			if err != nil {
				log.Printf("Reader %d error: %v", readerID, err)
			} else {
				count++
			}

			// Delay between reads
			time.Sleep(10 * time.Millisecond)
		}
	}
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
		MsgID:     fmt.Sprintf("loadtest-%d", count),
		Message:   fmt.Sprintf("Load test message from writer %d, count %d: %s", writerID, count, generateRandomText()),
		StructuredData: map[string]interface{}{
			"writer_id":  writerID,
			"count":      count,
			"timestamp":  time.Now().Unix(),
			"random_val": rand.Float64(),
			"test_type":  "load_test",
		},
	}
}

func generateSearchQuery(readerID, count int) types.SearchQuery {
	queries := []types.SearchQuery{
		{Text: "load test", Limit: 10},
		{Hostname: "web01", Limit: 5},
		{AppName: "nginx", Limit: 5},
		{Limit: 20},
		{Text: "message", Limit: 15},
	}

	return queries[count%len(queries)]
}

func generateRandomText() string {
	words := []string{
		"processing", "request", "response", "error", "success", "timeout", "connection",
		"database", "query", "transaction", "batch", "cache", "memory", "disk", "network",
		"authentication", "authorization", "validation", "parsing", "encoding", "decoding",
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
