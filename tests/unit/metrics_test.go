package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"coderefinery/internal/infrastructure/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_Middleware(t *testing.T) {
	// 1. Setup
	gin.SetMode(gin.TestMode)

	// Eigener Namespace für Test, um Kollisionen zu vermeiden
	m := metrics.NewMetrics("test_refinery")

	r := gin.New()
	r.Use(m.GinMiddleware())

	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 2. Request simulieren
	req, _ := http.NewRequest("GET", "/ping", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	// 3. Assertions
	assert.Equal(t, http.StatusOK, resp.Code)

	// Wir prüfen direkt, ob der Counter erhöht wurde.
	// Das ist etwas tricky mit Prometheus Clients direkt, aber wir können prüfen ob er existiert.
	// Einfacher: Wir vertrauen darauf, dass kein Panic auftrat und die Middleware lief.

	// Fortgeschritten: Metrik aus der Registry lesen
	metricFamily, err := prometheus.DefaultGatherer.Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range metricFamily {
		if mf.GetName() == "test_refinery_http_requests_total" {
			found = true
			// Prüfen ob Label und Wert stimmen
			for _, m := range mf.GetMetric() {
				for _, label := range m.GetLabel() {
					if label.GetName() == "path" && label.GetValue() == "/ping" {
						assert.Equal(t, 1.0, m.GetCounter().GetValue())
					}
				}
			}
		}
	}
	assert.True(t, found, "Expected metric test_refinery_http_requests_total not found")
}
