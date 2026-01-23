package unit

import (
	"coderefinery/internal/infrastructure/resilience"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_Recovery(t *testing.T) {
	// 1. Setup: Breaker mit extrem kurzem Timeout (100ms)
	cb := resilience.NewCircuitBreakerWithTimeout("test-breaker-recovery", 100*time.Millisecond)
	simulatedErr := errors.New("service unavailable")

	// 2. Action: Wir zwingen den Breaker in den OPEN State
	// Wir brauchen mindestens 3 Requests für den Trip (laut Config)
	for range 5 {
		_, err := cb.Execute(func() (any, error) {
			return nil, simulatedErr
		})
		assert.Error(t, err)
	}

	// 3. Verify: Breaker muss jetzt offen sein
	_, err := cb.Execute(func() (any, error) {
		return "blocked", nil
	})
	assert.Error(t, err)
	assert.True(t, resilience.IsOpenError(err), "Circuit Breaker sollte offen sein")

	// 4. Action: Warten bis Timeout abgelaufen ist (wir warten etwas länger als 100ms)
	time.Sleep(150 * time.Millisecond)

	// 5. Action: Erfolgreiche Ausführung (Half-Open -> Closed)
	// Der erste Request nach dem Timeout darf durch. Wenn er klappt, schließt der Breaker.
	result, err := cb.Execute(func() (any, error) {
		return "service recovered", nil
	})

	// 6. Assertions
	assert.NoError(t, err, "Nach Timeout sollte Request durchgehen")
	assert.Equal(t, "service recovered", result)

	// 7. Verify: Breaker ist wieder komplett geschlossen
	result2, err2 := cb.Execute(func() (any, error) {
		return "normal operation", nil
	})
	assert.NoError(t, err2)
	assert.Equal(t, "normal operation", result2)
}

// Alternative: Integrationstest mit Mock-Zeit (falls Timeout nicht konfigurierbar)
func TestCircuitBreaker_Recovery_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 1. Setup
	cb := resilience.NewCircuitBreaker("test-breaker-recovery-integration")
	simulatedErr := errors.New("temporary failure")

	// 2. Trip the circuit breaker
	for range 5 {
		cb.Execute(func() (any, error) {
			return nil, simulatedErr
		})
	}

	// 3. Verify it's open
	_, err := cb.Execute(func() (any, error) {
		return "blocked", nil
	})
	assert.True(t, resilience.IsOpenError(err))

	// 4. Wait for actual timeout (30 Sekunden in Production)
	t.Log("Waiting for circuit breaker timeout...")
	time.Sleep(31 * time.Second)

	// 5. Attempt recovery
	result, err := cb.Execute(func() (any, error) {
		return "recovered", nil
	})

	// 6. Assertions
	assert.NoError(t, err, "Circuit Breaker sollte nach Timeout recovery erlauben")
	assert.Equal(t, "recovered", result)

	// 7. Verify continued operation
	for range 5 {
		result, err := cb.Execute(func() (any, error) {
			return "operational", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "operational", result)
	}
}

// Minimal-Test: Verifiziert nur das Konzept ohne echtes Warten
func TestCircuitBreaker_Recovery_Concept(t *testing.T) {
	// Dieser Test dokumentiert das erwartete Recovery-Verhalten
	// ohne tatsächlich zu warten

	cb := resilience.NewCircuitBreaker("test-concept")

	// Phase 1: CLOSED -> OPEN
	for range 5 {
		cb.Execute(func() (any, error) {
			return nil, errors.New("fail")
		})
	}

	_, err := cb.Execute(func() (any, error) {
		return "ok", nil
	})
	assert.True(t, resilience.IsOpenError(err),
		"Nach mehreren Fehlern sollte Circuit Breaker OPEN sein")

	// Phase 2: Dokumentation des erwarteten Verhaltens
	t.Log("In Production würde nach 30s ein HALF-OPEN State eintreten")
	t.Log("Erfolgreiche Requests im HALF-OPEN State schließen den Circuit Breaker")
	t.Log("Fehlgeschlagene Requests im HALF-OPEN State öffnen ihn erneut")

	// TODO: Echten Recovery-Test implementieren sobald Timeout konfigurierbar ist
}
