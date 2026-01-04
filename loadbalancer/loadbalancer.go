package loadbalancer

import (
	"errors"
	"fmt"
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

// New creates a new LoadBalancer with the given configuration.
// It initializes the backend pool from the config but does NOT start listening.
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
// Call this before Start() to use a different algorithm.
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

			fmt.Printf("Accept error: %v\n", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		go lb.handleConnection(conn)
	}
}

// Stop gracefully shuts down the load balancer.
// It stops accepting new connections and signals the health checker to stop.
func (lb *LoadBalancer) Stop() error {
	close(lb.healthStop)

	if lb.listener != nil {
		return lb.listener.Close()
	}

	return nil
}

// handleConnection processes a single client connection.
// It selects a backend, establishes a connection to it, and proxies data bidirectionally.
func (lb *LoadBalancer) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	nextBackend := lb.algorithm.NextBackend(lb.pool)
	if nextBackend == nil {
		fmt.Println("No backend available for connection")
		return
	}

	nextBackend.IncrementConnections()
	defer nextBackend.DecrementConnections()

	backendConn, err := nextBackend.Dial(lb.config.ConnectTimeout)
	if err != nil {
		fmt.Println("Could not establish a connection to the server: ", nextBackend.Address)
		return
	}
	defer backendConn.Close()

	proxy.Proxy(clientConn, backendConn)
}

// GetPool returns the backend pool for external access (e.g., stats).
func (lb *LoadBalancer) GetPool() *backend.Pool {
	return lb.pool
}
