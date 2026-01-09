package loadbalancer

import (
	"errors"
	"log"
	"net"
	"tcp_lb/backend"
	"tcp_lb/config"
	"tcp_lb/proxy"
	"time"
)

// LoadBalancer is the main struct that coordinates all load balancing operations.
type LoadBalancer struct {
	config     *config.Config
	pool       *backend.Pool
	algorithm  Algorithm
	listener   net.Listener
	healthStop chan struct{}
}

// New creates a LoadBalancer from configuration.
func New(cfg *config.Config) *LoadBalancer {
	backendPool := backend.NewPool()

	for _, b := range cfg.Backends {
		backendPool.AddBackend(backend.NewBackendWithWeight(b.Address, b.Weight))
	}

	loadbalancer := &LoadBalancer{
		config:     cfg,
		pool:       backendPool,
		algorithm:  NewRoundRobin(),
		healthStop: make(chan struct{}),
	}

	return loadbalancer
}

// SetAlgorithm changes the load balancing algorithm.
func (lb *LoadBalancer) SetAlgorithm(algo Algorithm) {
	lb.algorithm = algo
}

// Start begins accepting TCP connections on the configured address.
func (lb *LoadBalancer) Start() error {
	addr := lb.config.ListenAddr
	listener, err := net.Listen("tcp", addr)

	if err != nil {
		return err
	}

	lb.listener = listener

	go lb.startHealthChecker()

	for {
		conn, err := lb.listener.Accept()
		if err != nil {

			if errors.Is(err, net.ErrClosed) {
				return nil
			}

			log.Printf("Accept error: %v\n", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		go lb.handleConnection(conn)
	}
}

// Stop gracefully shuts down the load balancer.
func (lb *LoadBalancer) Stop() error {
	close(lb.healthStop)

	if lb.listener != nil {
		return lb.listener.Close()
	}

	return nil
}

// handleConnection routes a client connection to a backend using the configured algorithm.
func (lb *LoadBalancer) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// Try up to pool size times to find a working backend
	maxRetries := lb.pool.Size()
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		nextBackend := lb.algorithm.NextBackend(lb.pool)
		if nextBackend == nil {
			log.Println("No backend available for connection")
			return
		}

		backendConn, err := nextBackend.Dial(lb.config.ConnectTimeout)
		if err != nil {
			// Mark backend as unhealthy (passive health check)
			nextBackend.SetAlive(false)
			log.Printf("Backend %s is down, marking unhealthy (attempt %d/%d)",
				nextBackend.Address, attempt+1, maxRetries)
			lastErr = err
			continue // Try another backend
		}

		// Success - track and proxy the connection
		nextBackend.AddConnection(backendConn)
		defer nextBackend.RemoveConnection(backendConn)
		defer backendConn.Close()

		proxy.Proxy(clientConn, backendConn)
		return
	}

	log.Printf("All backends failed, last error: %v", lastErr)
}

// GetPool returns the backend pool.
func (lb *LoadBalancer) GetPool() *backend.Pool {
	return lb.pool
}
