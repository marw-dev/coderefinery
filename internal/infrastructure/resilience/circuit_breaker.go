package resilience

import (
	"errors"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerConfig defines the parameters for the breaker
type CircuitBreakerConfig struct {
	Name             string
	MaxRequests      uint32
	Timeout          time.Duration
	FailureThreshold float64
	Interval         time.Duration
}

// CircuitBreaker wraps gobreaker
type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

// NewCircuitBreaker creates a breaker with specific configuration
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	// Set defaults if zero
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 5
	}
	if cfg.Interval == 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 0.6
	}

	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= cfg.FailureThreshold
		},
	}
	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

// NewProductionCircuitBreaker returns a breaker with standard production settings
func NewProductionCircuitBreaker(name string) *CircuitBreaker {
	return NewCircuitBreaker(CircuitBreakerConfig{
		Name:             name,
		Timeout:          30 * time.Second,
		FailureThreshold: 0.6,
	})
}

func (c *CircuitBreaker) Execute(fn func() (any, error)) (any, error) {
	return c.cb.Execute(fn)
}

func IsOpenError(err error) bool {
	return errors.Is(err, gobreaker.ErrOpenState)
}
