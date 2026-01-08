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
type LeastConnections struct{}

// NewLeastConnections creates a new LeastConnections algorithm instance.
func NewLeastConnections() *LeastConnections {
	// TODO: Implement this function
	return nil
}

// NextBackend returns the healthy backend with the fewest active connections.
// Returns nil if no healthy backend exists.
func (lc *LeastConnections) NextBackend(pool *backend.Pool) *backend.Backend {
	// TODO: Implement this function
	// 1. Get healthy backends from the pool
	// 2. If none available, return nil
	// 3. Iterate through all healthy backends
	// 4. Track the one with the minimum GetActiveConnections()
	// 5. Return that backend
	return nil
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
	// TODO: Implement this function
	return nil
}

// NextBackend returns the next backend based on weighted round-robin.
// Backends with higher weights are selected more frequently.
func (wrr *WeightedRoundRobin) NextBackend(pool *backend.Pool) *backend.Backend {
	// TODO: Implement this function
	return nil
}

// =============================================================================
// IP HASH ALGORITHM (Bonus - Tier 3)
// =============================================================================

// IPHash routes requests from the same client IP to the same backend.
// This provides "sticky sessions" without requiring application-level session management.
type IPHash struct{}

// NewIPHash creates a new IPHash algorithm instance.
func NewIPHash() *IPHash {
	// TODO: Implement this function
	return nil
}

// NextBackendForIP returns the backend for a specific client IP.
// The same IP will always map to the same backend (as long as it's healthy).
// If the mapped backend is unhealthy, falls back to the next healthy one.
func (ih *IPHash) NextBackendForIP(pool *backend.Pool, clientIP string) *backend.Backend {
	// TODO: Implement this function
	// 1. Get healthy backends from the pool
	// 2. If none available, return nil
	// 3. Hash the clientIP string to get a number (use hash/fnv)
	// 4. Use: index = hash % len(healthyBackends)
	// 5. Return the backend at that index
	return nil
}

// NextBackend implements the Algorithm interface.
// Note: This version doesn't have access to client IP, so it falls back to first healthy.
// Use NextBackendForIP directly when you have the client IP.
func (ih *IPHash) NextBackend(pool *backend.Pool) *backend.Backend {
	// TODO: Implement this function
	return nil
}
