package backend

import (
	"net"
	"sync"
	"time"
)

// Backend represents a single backend server that can receive proxied connections.
// It tracks health status, active connections, and statistics.
type Backend struct {
	Address           string       // The backend address in "host:port" format
	Weight            int          // Weight for weighted round-robin algorithm
	Alive             bool         // Whether the backend is currently healthy
	ActiveConnections int          // Number of currently active connections
	TotalConnections  int64        // Total connections handled (for stats)
	LastHealthCheck   time.Time    // When the last health check was performed
	mu                sync.RWMutex // Protects all mutable fields above
}

// NewBackend creates a new Backend with the given address.
// The backend starts in an alive state with weight 1.
func NewBackend(address string) *Backend {
	return &Backend{
		Address:         address,
		Weight:          1,
		Alive:           true,
		LastHealthCheck: time.Now(),
	}
}

// NewBackendWithWeight creates a new Backend with a custom weight.
// Weight determines how much traffic this backend receives in weighted round-robin.
func NewBackendWithWeight(address string, weight int) *Backend {
	return &Backend{
		Address:         address,
		Weight:          weight,
		Alive:           true,
		LastHealthCheck: time.Now(),
	}
}

// IsAlive returns true if the backend is currently marked as healthy.
// This method is thread-safe.
func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Alive
}

// SetAlive updates the backend's health status.
// This method is thread-safe.
func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Alive = alive
}

// IncrementConnections atomically increases the active connection count.
// Call this when a new client connection is assigned to this backend.
// Also increments TotalConnections for statistics.
func (b *Backend) IncrementConnections() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.ActiveConnections++
	b.TotalConnections++
}

// DecrementConnections atomically decreases the active connection count.
// Call this when a client connection to this backend is closed.
func (b *Backend) DecrementConnections() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ActiveConnections <= 0 {
		return
	}

	b.ActiveConnections--
}

// GetActiveConnections returns the current number of active connections.
// This method is thread-safe.
func (b *Backend) GetActiveConnections() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.ActiveConnections
}

// GetStats returns a snapshot of the backend's statistics.
// Returns: address, alive status, active connections, total connections
func (b *Backend) GetStats() (string, bool, int, int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Address, b.Alive, b.ActiveConnections, b.TotalConnections
}

// GetLastHealthCheck returns the timestamp of the last health check.
// This method is thread-safe.
func (b *Backend) GetLastHealthCheck() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.LastHealthCheck
}

// CheckHealth attempts to establish a TCP connection to the backend.
// If successful, marks the backend as alive. If failed, marks it as dead.
// The timeout parameter controls how long to wait for the connection.
//
// This is the core health check logic used by the health checker goroutine.
func (b *Backend) CheckHealth(timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", b.Address, timeout)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.LastHealthCheck = time.Now()

	if err == nil {
		conn.Close()
		b.Alive = true
		return true
	}

	b.Alive = false
	return false
}

// Dial creates a new TCP connection to this backend.
// Returns the connection or an error if the backend is unreachable.
// This is used by the proxy to establish connections for client requests.
func (b *Backend) Dial(timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("tcp", b.Address, timeout)
}
