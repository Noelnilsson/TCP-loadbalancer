package loadbalancer

import (
	"sync"
	"tcp_lb/backend"
)

type Algorithm interface {
	NextBackend(pool *backend.Pool) *backend.Backend
}

// =============================================================================
// ROUND ROBIN ALGORITHM
// =============================================================================

type RoundRobin struct {
	current uint64 // The index of the next backend to use
	mu      sync.Mutex
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{
		current: 0,
	}
}

// NextBackend returns the next healthy backend in round-robin order.
// Skips unhealthy backends. Returns nil if no healthy backend exists.
func (rr *RoundRobin) NextBackend(pool *backend.Pool) *backend.Backend {
	healthyBackends := pool.GetHealthyBackends()
	if len(healthyBackends) == 0 {
		return nil
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()

	backend := healthyBackends[rr.current%uint64(len(healthyBackends))]
	rr.current++

	return backend
}

// =============================================================================
// LEAST CONNECTIONS ALGORITHM (Bonus - Tier 2)
// =============================================================================

// LeastConnections routes traffic to the backend with the fewest active connections.
// This helps distribute load more evenly when requests have varying durations.
type LeastConnections struct {
	mu sync.Mutex
}

// NewLeastConnections creates a new LeastConnections algorithm instance.
func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

// NextBackend returns the healthy backend with the fewest active connections.
// Returns nil if no healthy backend exists.
func (lc *LeastConnections) NextBackend(pool *backend.Pool) *backend.Backend {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	healthyBackends := pool.GetHealthyBackends()
	if len(healthyBackends) == 0 {
		return nil
	}

	leastConn := 9999
	var leastBackend *backend.Backend
	for _, b := range healthyBackends {
		if b.GetActiveConnections() < leastConn {
			leastConn = b.GetActiveConnections()
			leastBackend = b
		}
	}

	return leastBackend
}

// =============================================================================
// WEIGHTED ROUND ROBIN ALGORITHM (Bonus - Tier 2)
// =============================================================================

// WeightedRoundRobin distributes requests based on backend weights.
// Backends with higher weights receive proportionally more traffic.
type WeightedRoundRobin struct {
	current       int        // Current position in the weighted sequence
	currentWeight int        // Current weight counter
	mu            sync.Mutex // Protects the state
}

// NewWeightedRoundRobin creates a new WeightedRoundRobin algorithm instance.
func NewWeightedRoundRobin() *WeightedRoundRobin {
	return &WeightedRoundRobin{
		current:       0,
		currentWeight: 0,
	}
}

// NextBackend returns the next backend based on weighted round-robin.
// Backends with higher weights are selected more frequently.
func (wrr *WeightedRoundRobin) NextBackend(pool *backend.Pool) *backend.Backend {
	healthyBackends := pool.GetHealthyBackends()
	if len(healthyBackends) == 0 {
		return nil
	}

	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	backend := healthyBackends[wrr.current%int(len(healthyBackends))]
	wrr.currentWeight++

	if wrr.currentWeight >= backend.GetWeight() {
		wrr.currentWeight = 0
		wrr.current++
	}

	return backend
}
