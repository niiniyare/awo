// Package metrics registers Prometheus metrics for the Awo framework.
//
// Exposed at /metrics via the Prometheus default registry.
//
// Key metrics:
//   - http_request_duration_seconds (histogram, labels: method, path, status)
//   - db_query_duration_seconds     (histogram, labels: entity, operation)
//   - workflow_started_total        (counter, labels: workflow_type, tenant)
//   - workflow_failed_total         (counter, labels: workflow_type, tenant)
//   - cache_hit_total               (counter, labels: cache_key_type)
//   - cache_miss_total              (counter, labels: cache_key_type)
package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

var (
	// HTTPRequestDuration records HTTP request latency.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// DBQueryDuration records database query latency per entity+operation.
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"entity", "operation"},
	)

	// WorkflowStarted counts successful Temporal workflow starts.
	WorkflowStarted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflow_started_total",
			Help: "Total number of Temporal workflows started",
		},
		[]string{"workflow_type", "tenant"},
	)

	// WorkflowFailed counts failed Temporal workflow start attempts.
	WorkflowFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflow_failed_total",
			Help: "Total number of Temporal workflow start failures",
		},
		[]string{"workflow_type", "tenant"},
	)

	// CacheHit counts cache hits by key type.
	CacheHit = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hit_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_key_type"},
	)

	// CacheMiss counts cache misses by key type.
	CacheMiss = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_miss_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_key_type"},
	)
)

// Middleware returns a Fiber middleware that records HTTP request metrics.
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start).Seconds()

		HTTPRequestDuration.WithLabelValues(
			c.Method(),
			c.Route().Path,
			strconv.Itoa(c.Response().StatusCode()),
		).Observe(duration)

		return err
	}
}

// Handler returns a Fiber handler that serves the Prometheus /metrics endpoint.
func Handler() fiber.Handler {
	h := fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())
	return func(c *fiber.Ctx) error {
		h(c.Context())
		return nil
	}
}

// RecordDBQuery records a database query duration.
// Call with defer: defer metrics.RecordDBQuery("finance_invoice", "Get")(time.Now())
func RecordDBQuery(entity, operation string) func(time.Time) {
	return func(start time.Time) {
		DBQueryDuration.WithLabelValues(entity, operation).Observe(time.Since(start).Seconds())
	}
}
