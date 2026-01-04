package backend

import (
	"sync"
)

// Pool manages a collection of backend servers.
// It provides thread-safe access to the backends and tracks which ones are healthy.
type Pool struct {
	backends []*Backend   // All configured backends
	mu       sync.RWMutex // Protects the backends slice
}

// NewPool creates a new empty backend pool.
func NewPool() *Pool {
	return new(Pool)
}

// AddBackend adds a new backend to the pool.
// This method is thread-safe.
func (p *Pool) AddBackend(b *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = append(p.backends, b)
}

// RemoveBackend removes a backend from the pool by its address.
// Returns true if the backend was found and removed.
// This method is thread-safe.
func (p *Pool) RemoveBackend(address string) bool {
	p.mu.Lock()
	for i := 0; i < len(p.backends); i++ {
		if p.backends[i].Address == address {
			p.backends = append(p.backends[:i], p.backends[i+1:]...)

			p.mu.Unlock()
			return true
		}
	}
	p.mu.Unlock()
	return false
}

// GetBackends returns a copy of all backends in the pool.
// This method is thread-safe.
func (p *Pool) GetBackends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	backendsCopy := make([]*Backend, len(p.backends))
	copy(backendsCopy, p.backends)

	return backendsCopy
}

// GetHealthyBackends returns only the backends that are currently alive.
// This method is thread-safe.
func (p *Pool) GetHealthyBackends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var healthy []*Backend
	for _, b := range p.backends {
		if b.IsAlive() {
			healthy = append(healthy, b)
		}
	}

	return healthy
}

// GetBackendByAddress finds a backend by its address.
// Returns nil if not found.
// This method is thread-safe.
func (p *Pool) GetBackendByAddress(address string) *Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := 0; i < len(p.backends); i++ {
		if p.backends[i].Address == address {
			return p.backends[i]
		}
	}

	return nil
}

// Size returns the total number of backends in the pool.
// This method is thread-safe.
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.backends)
}


// HealthyCount returns the number of currently healthy backends.
// This method is thread-safe.
func (p *Pool) HealthyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var healthyCount int
	for _, b := range p.backends {
		if b.IsAlive() {
			healthyCount++
		}
	}

	return healthyCount
}

// MarkAllHealthy sets all backends in the pool to alive status.
// Useful for initialization or testing.
func (p *Pool) MarkAllHealthy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < len(p.backends); i++ {
		p.backends[i].Alive = true
	}
}

// GetAllStats returns statistics for all backends.
// Returns a slice of stats, one per backend.
func (p *Pool) GetAllStats() []BackendStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var backendStats []BackendStats
	for _, b := range p.backends {

		address, alive, activeConnections, totalConnections := b.GetStats()
		backendStats = append(backendStats, BackendStats{
			Address:           address,
			Alive:             alive,
			ActiveConnections: activeConnections,
			TotalConnections:  totalConnections,
		})
	}

	return backendStats
}

// BackendStats holds a snapshot of a backend's statistics.
// Used for reporting and the stats dashboard.
type BackendStats struct {
	Address           string
	Alive             bool
	ActiveConnections int
	TotalConnections  int64
}
