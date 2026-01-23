package resilience

import (
	"errors"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreaker wraps gobreaker to provide a simpler interface
type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

// Standard Constructor (Production settings)
func NewCircuitBreaker(name string) *CircuitBreaker {
	// 30 Sekunden Timeout für Produktion
	return NewCircuitBreakerWithTimeout(name, 30*time.Second)
}

// Constructor mit konfigurierbarem Timeout (für Tests)
func NewCircuitBreakerWithTimeout(name string, timeout time.Duration) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 5,                // Max requests in half-open state
		Interval:    10 * time.Second, // Cyclic period of the closed state
		Timeout:     timeout,          // Open state duration (Variable!)
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			// Trip wenn >= 3 Requests und > 60% Fehlerquote
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	}
	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

// Execute führt eine Funktion im Kontext des Circuit Breakers aus
func (c *CircuitBreaker) Execute(fn func() (any, error)) (any, error) {
	return c.cb.Execute(fn)
}

// Hilfsfunktion um zu prüfen, ob der Fehler vom Breaker kommt
func IsOpenError(err error) bool {
	return errors.Is(err, gobreaker.ErrOpenState)
}
