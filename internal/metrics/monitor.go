package metrics

import (
	"context"
	"log"
	"sync"
	"time"
)

// PerformanceMonitor tracks real-time performance metrics
type PerformanceMonitor struct {
	metrics *StorageMetrics

	// Tracking variables
	lastWriteCount float64
	lastReadCount  float64
	lastTimestamp  time.Time

	// Moving averages
	writeLatencies []time.Duration
	readLatencies  []time.Duration
	maxSamples     int

	mutex sync.RWMutex
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(metrics *StorageMetrics) *PerformanceMonitor {
	return &PerformanceMonitor{
		metrics:        metrics,
		lastTimestamp:  time.Now(),
		writeLatencies: make([]time.Duration, 0, 1000),
		readLatencies:  make([]time.Duration, 0, 1000),
		maxSamples:     1000,
	}
}

// StartMonitoring begins the performance monitoring loop
func (pm *PerformanceMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.updateMetrics()
		}
	}
}

// updateMetrics calculates and updates derived metrics
func (pm *PerformanceMonitor) updateMetrics() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(pm.lastTimestamp).Seconds()

	if elapsed == 0 {
		return
	}

	// Calculate TPS (this is a simplified calculation)
	// In a real implementation, you'd get the actual counter values
	currentTime := now
	pm.lastTimestamp = currentTime

	// Update throughput metrics (placeholder - would need actual counter values)
	// pm.metrics.UpdateThroughput(tps)

	// Calculate latency percentiles
	if len(pm.writeLatencies) > 0 {
		p95, p99 := pm.calculatePercentiles(pm.writeLatencies)
		pm.metrics.UpdateLatencyPercentiles(p95, p99)
	}
}

// calculatePercentiles calculates the 95th and 99th percentiles
func (pm *PerformanceMonitor) calculatePercentiles(latencies []time.Duration) (time.Duration, time.Duration) {
	if len(latencies) == 0 {
		return 0, 0
	}

	// Simple percentile calculation (in production, use a proper algorithm)
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Sort the latencies (simple bubble sort for small datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p95Index := int(float64(len(sorted)) * 0.95)
	p99Index := int(float64(len(sorted)) * 0.99)

	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	if p99Index >= len(sorted) {
		p99Index = len(sorted) - 1
	}

	return sorted[p95Index], sorted[p99Index]
}

// RecordWriteLatency records a write operation latency
func (pm *PerformanceMonitor) RecordWriteLatency(latency time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.writeLatencies = append(pm.writeLatencies, latency)

	// Keep only the most recent samples
	if len(pm.writeLatencies) > pm.maxSamples {
		pm.writeLatencies = pm.writeLatencies[1:]
	}
}

// RecordReadLatency records a read operation latency
func (pm *PerformanceMonitor) RecordReadLatency(latency time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.readLatencies = append(pm.readLatencies, latency)

	// Keep only the most recent samples
	if len(pm.readLatencies) > pm.maxSamples {
		pm.readLatencies = pm.readLatencies[1:]
	}
}

// GetCurrentStats returns current performance statistics
func (pm *PerformanceMonitor) GetCurrentStats() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	stats := make(map[string]interface{})

	if len(pm.writeLatencies) > 0 {
		p95, p99 := pm.calculatePercentiles(pm.writeLatencies)
		stats["write_latency_p95_ms"] = float64(p95.Nanoseconds()) / 1e6
		stats["write_latency_p99_ms"] = float64(p99.Nanoseconds()) / 1e6
		stats["write_samples"] = len(pm.writeLatencies)
	}

	if len(pm.readLatencies) > 0 {
		p95, p99 := pm.calculatePercentiles(pm.readLatencies)
		stats["read_latency_p95_ms"] = float64(p95.Nanoseconds()) / 1e6
		stats["read_latency_p99_ms"] = float64(p99.Nanoseconds()) / 1e6
		stats["read_samples"] = len(pm.readLatencies)
	}

	return stats
}

// LogCurrentStats logs the current performance statistics
func (pm *PerformanceMonitor) LogCurrentStats() {
	stats := pm.GetCurrentStats()

	log.Printf("Performance Stats: %+v", stats)
}
