package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	napAgentsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nap_agents_total",
		Help: "Total number of registered agents by status.",
	}, []string{"status"})

	napRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nap_requests_total",
		Help: "Total HTTP requests by method, path, and response status.",
	}, []string{"method", "path", "status"})

	napRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nap_request_duration_seconds",
		Help:    "Request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	napHealthChecksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nap_health_checks_total",
		Help: "Total health check probes by result.",
	}, []string{"result"})

	napLedgerEntriesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nap_ledger_entries_total",
		Help: "Total trust ledger entries appended.",
	})

	napWebhookDeliveriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nap_webhook_deliveries_total",
		Help: "Total webhook deliveries by success status.",
	}, []string{"status"})
)

// PrometheusMiddleware returns a Gin middleware that records per-request metrics.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		napRequestsTotal.WithLabelValues(method, path, status).Inc()
		napRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}

// MetricsHandler returns a Gin handler that serves Prometheus metrics.
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// RecordHealthCheck records a health check probe result.
func RecordHealthCheck(success bool) {
	if success {
		napHealthChecksTotal.WithLabelValues("success").Inc()
	} else {
		napHealthChecksTotal.WithLabelValues("failure").Inc()
	}
}

// RecordLedgerAppend records a trust ledger entry append.
func RecordLedgerAppend() {
	napLedgerEntriesTotal.Inc()
}

// RecordWebhookDelivery records a webhook delivery attempt.
func RecordWebhookDelivery(success bool) {
	if success {
		napWebhookDeliveriesTotal.WithLabelValues("success").Inc()
	} else {
		napWebhookDeliveriesTotal.WithLabelValues("failure").Inc()
	}
}

// SetAgentsGauge sets the agent count gauge for a given status.
func SetAgentsGauge(status string, count float64) {
	napAgentsTotal.WithLabelValues(status).Set(count)
}
