package unit

import (
	"coderefinery/internal/infrastructure/resilience"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_Recovery(t *testing.T) {
	// 1. Setup: Use the unified constructor with test-friendly timeout
	cfg := resilience.CircuitBreakerConfig{
		Name:    "test-breaker-recovery",
		Timeout: 100 * time.Millisecond,
	}
	cb := resilience.NewCircuitBreaker(cfg)

	simulatedErr := errors.New("service unavailable")

	// 2. Action: Force OPEN State (requires >= 3 requests)
	for range 5 {
		_, err := cb.Execute(func() (any, error) {
			return nil, simulatedErr
		})
		assert.Error(t, err)
	}

	// 3. Verify: Breaker is open
	_, err := cb.Execute(func() (any, error) {
		return "blocked", nil
	})
	assert.Error(t, err)
	assert.True(t, resilience.IsOpenError(err))

	// 4. Action: Wait for Timeout
	time.Sleep(150 * time.Millisecond)

	// 5. Action: Recovery (Half-Open -> Closed)
	result, err := cb.Execute(func() (any, error) {
		return "service recovered", nil
	})

	// 6. Assertions
	assert.NoError(t, err)
	assert.Equal(t, "service recovered", result)

	// 7. Verify Closed
	result2, err2 := cb.Execute(func() (any, error) {
		return "normal operation", nil
	})
	assert.NoError(t, err2)
	assert.Equal(t, "normal operation", result2)
}
