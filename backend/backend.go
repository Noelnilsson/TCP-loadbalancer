package backend

import (
	"errors"
	"net"
	"sync"
	"time"
)

// ErrBackendDown is returned when attempting to dial a backend that is simulated down.
var ErrBackendDown = errors.New("backend is down")

// Backend represents a single backend server that can receive proxied connections.
// It tracks health status, active connections, and statistics.
type Backend struct {
	Address          string                // The backend address in "host:port" format
	Weight           int                   // Weight for weighted round-robin algorithm
	Alive            bool                  // Whether the backend is currently healthy
	SimulatedDown    bool                  // True if backend is down due to simulation (health check won't override)
	connections      map[net.Conn]struct{} // Set of currently active connections
	TotalConnections int64                 // Total connections handled (for stats)
	LastHealthCheck  time.Time             // When the last health check was performed
	mu               sync.RWMutex          // Protects all mutable fields above
	cond             *sync.Cond            // Condition variable for simulating backend failure
}

// NewBackend creates a new Backend with the given address.
// The backend starts in an alive state with weight 1.
func NewBackend(address string) *Backend {
	b := &Backend{
		Address:         address,
		Weight:          1,
		Alive:           true,
		connections:     make(map[net.Conn]struct{}),
		LastHealthCheck: time.Now(),
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// NewBackendWithWeight creates a new Backend with a custom weight.
// Weight determines how much traffic this backend receives in weighted round-robin.
func NewBackendWithWeight(address string, weight int) *Backend {
	b := &Backend{
		Address:         address,
		Weight:          weight,
		Alive:           true,
		connections:     make(map[net.Conn]struct{}),
		LastHealthCheck: time.Now(),
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// getAddress returns the backend address
// This method is thread-safe.
func (b *Backend) getAddress() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Address
}

// getAWeight returns the backend weight used for Weighted-Roud-Robin
// This method is thread-safe.
func (b *Backend) GetWeight() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Weight
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

	if alive {
		b.cond.Broadcast() // wake up waiting goroutines
	} else {
		// If we are "killing" the server, strictly close all current connections
		for conn := range b.connections {
			conn.Close()
		}
		// Re-initialize map to clear references (though Close() usually suffices, cleaning map is good hygiene)
		b.connections = make(map[net.Conn]struct{})
	}
}

// SetSimulatedDown marks the backend as down due to simulation.
// When simulated down, Dial() will fail and health checks will not override Alive.
// IMPORTANT: This does NOT immediately set Alive=false - the LB must discover
// the failure through a failed connection attempt (realistic behavior).
// This method is thread-safe.
func (b *Backend) SetSimulatedDown(down bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.SimulatedDown = down

	if down {
		// Going down: close existing connections but DON'T set Alive=false
		// The LB will discover the server is down when Dial() fails
		for conn := range b.connections {
			conn.Close()
		}
		b.connections = make(map[net.Conn]struct{})
	} else {
		// Recovering: just clear SimulatedDown, let health check set Alive=true
		// Wake up waiting goroutines so they can serve new connections
		b.cond.Broadcast()
	}
}

// AddConnection adds a connection to the tracking map.
func (b *Backend) AddConnection(conn net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.connections[conn] = struct{}{}
	b.TotalConnections++
}

// RemoveConnection removes a connection from the tracking map.
func (b *Backend) RemoveConnection(conn net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.connections, conn)
}

// GetActiveConnections returns the current number of active connections.
// This method is thread-safe.
func (b *Backend) GetActiveConnections() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.connections)
}

// GetStats returns a snapshot of the backend's statistics.
// Returns: address, alive status, active connections, total connections
func (b *Backend) GetStats() (string, bool, int, int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Address, b.Alive, len(b.connections), b.TotalConnections
}

// GetLastHealthCheck returns the timestamp of the last health check.
// This method is thread-safe.
func (b *Backend) GetLastHealthCheck() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.LastHealthCheck
}

// CheckHealth attempts to establish a TCP connection to the backend.
// If successful, marks the backend as alive.
// If failed (including SimulatedDown), marks it as dead.
// The timeout parameter controls how long to wait for the connection.
//
// This is the core health check logic used by the health checker goroutine.
func (b *Backend) CheckHealth(timeout time.Duration) bool {
	// Use Dial() to respect SimulatedDown flag
	conn, err := b.Dial(timeout)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.LastHealthCheck = time.Now()

	if err == nil {
		conn.Close()
		b.Alive = true
		b.cond.Broadcast() // Wake up any goroutines waiting for recovery
		return true
	}

	// Dial failed (server down or SimulatedDown) - mark unhealthy
	b.Alive = false
	return false
}

// Dial creates a new TCP connection to this backend.
// Returns the connection or an error if the backend is unreachable.
// Returns ErrBackendDown if the backend is simulated down.
// This is used by the proxy to establish connections for client requests.
func (b *Backend) Dial(timeout time.Duration) (net.Conn, error) {
	b.mu.RLock()
	if b.SimulatedDown {
		b.mu.RUnlock()
		return nil, ErrBackendDown
	}
	b.mu.RUnlock()

	return net.DialTimeout("tcp", b.Address, timeout)
}
