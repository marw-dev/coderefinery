package metrics

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	// HTTP Metriken
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// Business Metriken (können wir später nutzen)
	SearchDuration      *prometheus.HistogramVec
}

func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests processed",
			},
			[]string{"method", "path", "status"},
		),

		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "Duration of HTTP requests in seconds",
				Buckets:   prometheus.DefBuckets, // Standard-Buckets (.005, .01, .025, ...)
			},
			[]string{"method", "path"},
		),

		SearchDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "search_operation_duration_seconds",
				Help:      "Duration of vector search operations",
				Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"type"}, // z.B. "vector", "hybrid"
		),
	}
}

// Middleware für Gin Framework
func (m *Metrics) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath() // Nutzt das Route-Pattern (z.B. /query), nicht die exakte URL, um Kardinalität gering zu halten
		if path == "" {
			path = "unknown"
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())
		method := c.Request.Method

		m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}
