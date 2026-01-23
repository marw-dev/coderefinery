package unit

import (
	"context"
	"testing"

	"coderefinery/internal/config"
	"coderefinery/internal/infrastructure/tracing"

	"github.com/stretchr/testify/assert"
)

func TestTracing_InitDisabled(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled: false,
	}

	shutdown, err := tracing.InitTracer(cfg)

	assert.NoError(t, err)
	assert.NotNil(t, shutdown)

	// Shutdown sollte keinen Fehler werfen
	err = shutdown(context.Background())
	assert.NoError(t, err)
}

func TestTracing_InitEnabled_InvalidEndpoint(t *testing.T) {
	// Wir testen, dass er versucht zu verbinden (oder zumindest keinen Config-Fehler wirft)
	// Da OTLP HTTP im Hintergrund verbindet, wirft InitTracer oft keinen Fehler direkt,
	// es sei denn die URL ist komplett ungültig geparst.

	cfg := config.TracingConfig{
		Enabled:      true,
		Provider:     "otlp",
		Endpoint:     "localhost:12345", // Existiert nicht
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	shutdown, err := tracing.InitTracer(cfg)

	// OTLP New() validiert meist nur die URL Syntax, verbindet aber asynchron (je nach Lib Version).
	// Wenn es keinen Error gibt, ist das auch okay für diesen Test.
	// Wichtig ist, dass keine Panic auftritt.
	if err == nil {
		assert.NotNil(t, shutdown)
		_ = shutdown(context.Background())
	}
}
