package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed, labeled by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method and path.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	IngestionJobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ingestion_job_duration_seconds",
			Help:    "Ingestion job duration in seconds, labeled by terminal status (completed/failed).",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600},
		},
		[]string{"status"},
	)

	AgentToolExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_tool_executions_total",
			Help: "Total agent tool executions, labeled by tool name and outcome (success/failed).",
		},
		[]string{"tool", "outcome"},
	)

	WorkerPoolQueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_pool_queue_depth",
			Help: "Current number of ingestion jobs waiting in the worker pool's queue.",
		},
	)
)
