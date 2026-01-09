package loadbalancer

import (
	"sync"
	"tcp_lb/backend"
	"time"
)

// startHealthChecker runs periodic health checks on all backends.
func (lb *LoadBalancer) startHealthChecker() {
	ticker := time.NewTicker(lb.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lb.checkAllBackends()
		case <-lb.healthStop:
			return
		}
	}
}

// checkAllBackends performs a health check on every backend in the pool.
func (lb *LoadBalancer) checkAllBackends() {
	backends := lb.pool.GetBackends()

	var wg sync.WaitGroup
	for _, b := range backends {
		wg.Add(1)
		go func(backend *backend.Backend) {
			defer wg.Done()
			backend.CheckHealth(lb.config.ConnectTimeout)
		}(b)
	}
	wg.Wait()
}

type HealthStatus struct {
	TotalBackends   int             
	HealthyBackends int             
	Backends        []BackendHealth 
}

type BackendHealth struct {
	Address      string        
	Alive        bool          
	LastCheck    time.Time     
	ResponseTime time.Duration 
}

// GetHealthStatus returns the current health status of all backends.
func (lb *LoadBalancer) GetHealthStatus() HealthStatus {
	backends := lb.pool.GetBackends()

	healthyCount := 0
	backendHealthList := make([]BackendHealth, 0, len(backends))

	for _, b := range backends {
		address, isAlive, _, _ := b.GetStats()
		lastCheck := b.GetLastHealthCheck()

		if isAlive {
			healthyCount++
		}

		backendHealthList = append(backendHealthList, BackendHealth{
			Address:      address,
			Alive:        isAlive,
			LastCheck:    lastCheck,
			ResponseTime: 0,
		})
	}

	return HealthStatus{
		TotalBackends:   len(backends),
		HealthyBackends: healthyCount,
		Backends:        backendHealthList,
	}
}
