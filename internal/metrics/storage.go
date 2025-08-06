package metrics

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// StorageMetrics holds all storage-related Prometheus metrics
type StorageMetrics struct {
	// Write operations
	WriteRequestsTotal    prometheus.Counter
	WriteRequestsDuration prometheus.Histogram
	WriteErrorsTotal      prometheus.Counter
	WriteTimeoutsTotal    prometheus.Counter

	// Batch operations
	BatchesProcessedTotal prometheus.Counter
	BatchSizeHistogram    prometheus.Histogram
	BatchProcessingTime   prometheus.Histogram
	BatchQueueSize        prometheus.Gauge
	BatchBufferSize       prometheus.Gauge

	// Read operations
	ReadRequestsTotal    prometheus.Counter
	ReadRequestsDuration prometheus.Histogram
	ReadErrorsTotal      prometheus.Counter

	// Database operations
	DatabaseConnectionsActive prometheus.Gauge
	DatabaseTransactionsTotal prometheus.Counter
	DatabaseTransactionTime   prometheus.Histogram

	// Queue metrics
	QueueFullErrors  prometheus.Counter
	QueueUtilization prometheus.Gauge
	QueueWaitTime    prometheus.Histogram

	// Performance metrics
	ThroughputTPS prometheus.Gauge
	LatencyP95    prometheus.Gauge
	LatencyP99    prometheus.Gauge
}

var (
	storageMetricsInstance *StorageMetrics
	storageMetricsOnce     sync.Once
)

// GetStorageMetrics returns the singleton instance of storage metrics
func GetStorageMetrics() *StorageMetrics {
	storageMetricsOnce.Do(func() {
		storageMetricsInstance = newStorageMetrics()
	})
	return storageMetricsInstance
}

// NewStorageMetrics creates and registers all storage metrics (deprecated, use GetStorageMetrics)
func NewStorageMetrics() *StorageMetrics {
	return GetStorageMetrics()
}

// newStorageMetrics creates and registers all storage metrics (internal)
func newStorageMetrics() *StorageMetrics {
	return &StorageMetrics{
		// Write operations
		WriteRequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_write_requests_total",
			Help: "Total number of write requests to storage",
		}),
		WriteRequestsDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "opentrail_storage_write_duration_seconds",
			Help:    "Duration of write requests to storage",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~32s
		}),
		WriteErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_write_errors_total",
			Help: "Total number of write errors",
		}),
		WriteTimeoutsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_write_timeouts_total",
			Help: "Total number of write timeouts",
		}),

		// Batch operations
		BatchesProcessedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_batches_processed_total",
			Help: "Total number of batches processed",
		}),
		BatchSizeHistogram: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "opentrail_storage_batch_size",
			Help:    "Size of processed batches",
			Buckets: prometheus.LinearBuckets(1, 10, 20), // 1 to 200
		}),
		BatchProcessingTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "opentrail_storage_batch_processing_seconds",
			Help:    "Time taken to process a batch",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12), // 1ms to ~4s
		}),
		BatchQueueSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "opentrail_storage_batch_queue_size",
			Help: "Current size of the batch queue",
		}),
		BatchBufferSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "opentrail_storage_batch_buffer_size",
			Help: "Current size of the batch buffer",
		}),

		// Read operations
		ReadRequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_read_requests_total",
			Help: "Total number of read requests to storage",
		}),
		ReadRequestsDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "opentrail_storage_read_duration_seconds",
			Help:    "Duration of read requests to storage",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		}),
		ReadErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_read_errors_total",
			Help: "Total number of read errors",
		}),

		// Database operations
		DatabaseConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "opentrail_storage_db_connections_active",
			Help: "Number of active database connections",
		}),
		DatabaseTransactionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_db_transactions_total",
			Help: "Total number of database transactions",
		}),
		DatabaseTransactionTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "opentrail_storage_db_transaction_seconds",
			Help:    "Duration of database transactions",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
		}),

		// Queue metrics
		QueueFullErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "opentrail_storage_queue_full_errors_total",
			Help: "Total number of queue full errors",
		}),
		QueueUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "opentrail_storage_queue_utilization_ratio",
			Help: "Queue utilization as a ratio (0-1)",
		}),
		QueueWaitTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "opentrail_storage_queue_wait_seconds",
			Help:    "Time spent waiting in queue",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
		}),

		// Performance metrics
		ThroughputTPS: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "opentrail_storage_throughput_tps",
			Help: "Current throughput in transactions per second",
		}),
		LatencyP95: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "opentrail_storage_latency_p95_seconds",
			Help: "95th percentile latency",
		}),
		LatencyP99: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "opentrail_storage_latency_p99_seconds",
			Help: "99th percentile latency",
		}),
	}
}

// RecordWriteRequest records a write request with its duration and outcome
func (m *StorageMetrics) RecordWriteRequest(duration time.Duration, err error) {
	m.WriteRequestsTotal.Inc()
	m.WriteRequestsDuration.Observe(duration.Seconds())

	if err != nil {
		m.WriteErrorsTotal.Inc()
		if isTimeoutError(err) {
			m.WriteTimeoutsTotal.Inc()
		}
	}
}

// RecordBatchProcessed records a processed batch
func (m *StorageMetrics) RecordBatchProcessed(size int, duration time.Duration) {
	m.BatchesProcessedTotal.Inc()
	m.BatchSizeHistogram.Observe(float64(size))
	m.BatchProcessingTime.Observe(duration.Seconds())
}

// UpdateBatchQueueSize updates the current batch queue size
func (m *StorageMetrics) UpdateBatchQueueSize(size int) {
	m.BatchQueueSize.Set(float64(size))
}

// UpdateBatchBufferSize updates the current batch buffer size
func (m *StorageMetrics) UpdateBatchBufferSize(size int) {
	m.BatchBufferSize.Set(float64(size))
}

// RecordReadRequest records a read request with its duration and outcome
func (m *StorageMetrics) RecordReadRequest(duration time.Duration, err error) {
	m.ReadRequestsTotal.Inc()
	m.ReadRequestsDuration.Observe(duration.Seconds())

	if err != nil {
		m.ReadErrorsTotal.Inc()
	}
}

// RecordDatabaseTransaction records a database transaction
func (m *StorageMetrics) RecordDatabaseTransaction(duration time.Duration) {
	m.DatabaseTransactionsTotal.Inc()
	m.DatabaseTransactionTime.Observe(duration.Seconds())
}

// RecordQueueFullError records when the queue is full
func (m *StorageMetrics) RecordQueueFullError() {
	m.QueueFullErrors.Inc()
}

// UpdateQueueUtilization updates the queue utilization ratio
func (m *StorageMetrics) UpdateQueueUtilization(used, total int) {
	if total > 0 {
		ratio := float64(used) / float64(total)
		m.QueueUtilization.Set(ratio)
	}
}

// RecordQueueWaitTime records time spent waiting in queue
func (m *StorageMetrics) RecordQueueWaitTime(duration time.Duration) {
	m.QueueWaitTime.Observe(duration.Seconds())
}

// UpdateThroughput updates the current throughput metric
func (m *StorageMetrics) UpdateThroughput(tps float64) {
	m.ThroughputTPS.Set(tps)
}

// UpdateLatencyPercentiles updates latency percentile metrics
func (m *StorageMetrics) UpdateLatencyPercentiles(p95, p99 time.Duration) {
	m.LatencyP95.Set(p95.Seconds())
	m.LatencyP99.Set(p99.Seconds())
}

// UpdateDatabaseConnections updates the active database connections count
func (m *StorageMetrics) UpdateDatabaseConnections(count int) {
	m.DatabaseConnectionsActive.Set(float64(count))
}

// Helper function to check if error is a timeout
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "timed out")
}
