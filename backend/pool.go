package backend

import (
	"math/rand"
	"sync"
	"time"
)

// EventType represents the type of pool event
type EventType int

const (
	EventBackendDown EventType = iota
	EventBackendRecovered
)

// PoolEvent represents an event that occurred in the pool
type PoolEvent struct {
	Type    EventType
	Backend string
	Time    time.Time
}

// EventCallback is a function that handles pool events
type EventCallback func(event PoolEvent)

// Pool manages a collection of backend servers.
type Pool struct {
	backends      []*Backend    // All configured backends
	mu            sync.RWMutex  // Protects the backends slice
	eventCallback EventCallback // Optional callback for events

	// Simulation state
	pausedBackend    string    // Address of currently paused backend (empty if none)
	pauseStartTime   time.Time // When the current pause started
	pauseDuration    time.Duration // How long the current pause will last
	nextPauseTime    time.Time // When the next pause cycle will start
}

// NewPool creates a new empty backend pool.
func NewPool() *Pool {
	return &Pool{
		nextPauseTime: time.Now().Add(5 * time.Second), // First pause after 5s initial delay
	}
}

// SetEventCallback sets the callback function for pool events.
func (p *Pool) SetEventCallback(callback EventCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.eventCallback = callback
}

// GetPauseState returns the current pause simulation state.
func (p *Pool) GetPauseState() (string, time.Time, time.Duration, time.Time) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pausedBackend, p.pauseStartTime, p.pauseDuration, p.nextPauseTime
}

// emitEvent sends an event to the callback if one is registered.
func (p *Pool) emitEvent(eventType EventType, backendAddr string) {
	p.mu.RLock()
	callback := p.eventCallback
	p.mu.RUnlock()

	if callback != nil {
		callback(PoolEvent{
			Type:    eventType,
			Backend: backendAddr,
			Time:    time.Now(),
		})
	}
}

// AddBackend adds a new backend to the pool.
func (p *Pool) AddBackend(b *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = append(p.backends, b)
}

// RemoveBackend removes a backend from the pool, returning true if found.
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
func (p *Pool) GetBackends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	backendsCopy := make([]*Backend, len(p.backends))
	copy(backendsCopy, p.backends)

	return backendsCopy
}

// GetHealthyBackends returns only the backends that are currently alive.
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

// GetBackendByAddress finds a backend by address, returning nil if not found.
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

// GetRandomBackend returns a random backend from the pool.
func (p *Pool) GetRandomBackend() *Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	backends := p.backends
	if len(backends) == 0 {
		return nil
	}

	return backends[rand.Intn(len(backends))]
}

// Size returns the total number of backends in the pool.
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.backends)
}

// HealthyCount returns the number of currently healthy backends.
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

// simulateRandomBackendFailureAndRecovery simulates a random backend failure and recovery.
func (p *Pool) simulateRandomBackendFailureAndRecovery() {
	randomBackend := p.GetRandomBackend()
	if randomBackend == nil {
		return
	}

	// Calculate pause duration (15-20 seconds)
	pauseDuration := time.Duration(15+rand.Intn(6)) * time.Second

	// Update pause state
	p.mu.Lock()
	p.pausedBackend = randomBackend.Address
	p.pauseStartTime = time.Now()
	p.pauseDuration = pauseDuration
	p.mu.Unlock()

	// Set backend to simulated down (health check won't override)
	randomBackend.SetSimulatedDown(true)
	p.emitEvent(EventBackendDown, randomBackend.Address)

	// Wait for pause duration
	time.Sleep(pauseDuration)

	// Recover backend from simulated down
	randomBackend.SetSimulatedDown(false)
	p.emitEvent(EventBackendRecovered, randomBackend.Address)

	// Clear pause state
	p.mu.Lock()
	p.pausedBackend = ""
	p.mu.Unlock()
}

// SimulateRandomBackendFailureAndRecoveryLoop simulates a random backend failure and recovery in a loop.
func (p *Pool) SimulateRandomBackendFailureAndRecoveryLoop() {
	// Initial delay before first pause
	time.Sleep(5 * time.Second)

	for {
		// Update next pause time
		p.mu.Lock()
		p.nextPauseTime = time.Now()
		p.mu.Unlock()

		p.simulateRandomBackendFailureAndRecovery()

		// Update next pause time for the gap
		p.mu.Lock()
		p.nextPauseTime = time.Now().Add(25 * time.Second)
		p.mu.Unlock()

		time.Sleep(25 * time.Second)
	}
}

// RestartSimulation resets simulation state and recovers any paused backends.
func (p *Pool) RestartSimulation() {
	p.mu.Lock()
	pausedAddr := p.pausedBackend
	p.pausedBackend = ""
	p.nextPauseTime = time.Now().Add(5 * time.Second)
	p.mu.Unlock()

	// If a backend was paused, recover it
	if pausedAddr != "" {
		backend := p.GetBackendByAddress(pausedAddr)
		if backend != nil {
			backend.SetSimulatedDown(false)
			p.emitEvent(EventBackendRecovered, pausedAddr)
		}
	}
}

// MarkAllHealthy sets all backends to alive status.
func (p *Pool) MarkAllHealthy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < len(p.backends); i++ {
		p.backends[i].Alive = true
	}
}

// GetAllStats returns statistics for all backends.
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
type BackendStats struct {
	Address           string
	Alive             bool
	ActiveConnections int
	TotalConnections  int64
}
