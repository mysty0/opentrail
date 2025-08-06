# OpenTrail Load Testing Guide

This guide explains how to set up and run load tests for OpenTrail's SQLite batching optimization with Prometheus metrics monitoring.

## Quick Start

### 1. Build the Load Test Tool

```bash
go build -o loadtest cmd/loadtest/main.go
```

### 2. Run a Basic Load Test

```bash
./loadtest -duration=60s -writers=10 -readers=5 -batch-size=100 -metrics-port=8080
```

### 3. Monitor Metrics

Visit `http://localhost:8080/metrics` to see Prometheus metrics.

## Load Test Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `-db` | `loadtest.db` | Database file path |
| `-duration` | `60s` | Test duration |
| `-writers` | `10` | Number of concurrent writers |
| `-readers` | `5` | Number of concurrent readers |
| `-batch-size` | `100` | Batch size for writes |
| `-batch-timeout` | `10ms` | Batch timeout |
| `-queue-size` | `10000` | Write queue size |
| `-metrics-port` | `8080` | Metrics server port |
| `-log-interval` | `5s` | Stats logging interval |

## VPS Load Testing Setup

### 1. Server Setup

```bash
# On your VPS
git clone <your-repo>
cd opentrail
go build -o loadtest cmd/loadtest/main.go

# Run the load test
./loadtest -duration=300s -writers=50 -readers=20 -batch-size=200 -metrics-port=8080
```

### 2. Prometheus Setup

Create `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'opentrail'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 5s
```

Run Prometheus:

```bash
docker run -p 9090:9090 -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml prom/prometheus
```

### 3. Grafana Setup

```bash
docker run -d -p 3000:3000 grafana/grafana
```

1. Open http://localhost:3000 (admin/admin)
2. Add Prometheus data source: http://localhost:9090
3. Import the dashboard from `grafana-dashboard.json`

## Key Metrics to Monitor

### Throughput Metrics
- `opentrail_storage_write_requests_total` - Total write requests
- `opentrail_storage_read_requests_total` - Total read requests
- `opentrail_storage_batches_processed_total` - Total batches processed

### Latency Metrics
- `opentrail_storage_write_duration_seconds` - Write request latency
- `opentrail_storage_read_duration_seconds` - Read request latency
- `opentrail_storage_batch_processing_seconds` - Batch processing time

### Queue Metrics
- `opentrail_storage_batch_queue_size` - Current queue size
- `opentrail_storage_queue_utilization_ratio` - Queue utilization (0-1)
- `opentrail_storage_queue_full_errors_total` - Queue full errors

### Error Metrics
- `opentrail_storage_write_errors_total` - Write errors
- `opentrail_storage_write_timeouts_total` - Write timeouts
- `opentrail_storage_read_errors_total` - Read errors

## Performance Targets

Based on the SQLite batching optimization requirements:

### Target Performance
- **Write Throughput**: 5000+ TPS under concurrent load
- **Write Latency P95**: < 100ms
- **Write Latency P99**: < 200ms
- **Queue Utilization**: < 80% under normal load
- **Error Rate**: < 1% of total requests

### Load Test Scenarios

#### Scenario 1: Moderate Load
```bash
./loadtest -duration=300s -writers=20 -readers=10 -batch-size=100
```
Expected: 1000-2000 TPS

#### Scenario 2: High Load
```bash
./loadtest -duration=300s -writers=50 -readers=20 -batch-size=200
```
Expected: 3000-5000 TPS

#### Scenario 3: Burst Load
```bash
./loadtest -duration=300s -writers=100 -readers=30 -batch-size=300
```
Expected: 5000+ TPS

#### Scenario 4: Sustained Load
```bash
./loadtest -duration=1800s -writers=30 -readers=15 -batch-size=150
```
Expected: Stable performance over 30 minutes

## Monitoring During Load Tests

### Real-time Monitoring
```bash
# Watch metrics in real-time
watch -n 1 'curl -s http://localhost:8080/metrics | grep opentrail_storage'

# Monitor system resources
htop
iostat -x 1
```

### Key Indicators

1. **Healthy System**:
   - Queue utilization < 80%
   - Error rate < 1%
   - Latency P95 < 100ms
   - Steady throughput

2. **System Under Stress**:
   - Queue utilization > 90%
   - Increasing error rate
   - Latency spikes
   - Throughput degradation

3. **System Overloaded**:
   - Queue full errors
   - High timeout rate
   - Latency > 1s
   - Throughput collapse

## Troubleshooting

### High Latency
- Reduce batch timeout
- Increase batch size
- Check disk I/O

### Queue Full Errors
- Increase queue size
- Reduce write rate
- Optimize batch processing

### Low Throughput
- Increase batch size
- Reduce batch timeout
- Add more writers
- Check WAL mode configuration

### Memory Issues
- Reduce queue size
- Reduce batch size
- Monitor batch buffer size

## Example VPS Test Commands

```bash
# Light load test (baseline)
./loadtest -duration=60s -writers=5 -readers=2 -batch-size=50

# Medium load test
./loadtest -duration=300s -writers=25 -readers=10 -batch-size=150

# Heavy load test
./loadtest -duration=600s -writers=75 -readers=25 -batch-size=250

# Stress test
./loadtest -duration=300s -writers=150 -readers=50 -batch-size=500
```

Monitor the metrics at `http://your-vps-ip:8080/metrics` and use Grafana for visualization.

## Expected Results

With proper configuration, you should see:
- **Concurrent writes**: 700-1000+ TPS (as shown in integration tests)
- **Sequential writes**: Lower TPS due to batching overhead (expected)
- **Mixed workload**: Balanced performance with good concurrency
- **Error rate**: < 1% under normal conditions
- **Queue utilization**: 50-80% under optimal load