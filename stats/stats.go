package stats

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"tcp_lb/backend"
	"time"
)

// Server provides an HTTP endpoint for viewing load balancer statistics.
type Server struct {
	pool       *backend.Pool
	listenAddr string
	server     *http.Server
	startTime  time.Time
}

// NewServer creates a new stats server.
func NewServer(pool *backend.Pool, listenAddr string) *Server {
	return &Server{
		pool:       pool,
		listenAddr: listenAddr,
		startTime:  time.Now(),
	}
}

// Start begins serving HTTP requests for statistics.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    s.listenAddr,
		Handler: mux,
	}

	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the stats server.
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// StatsResponse is the JSON response for /stats endpoint.
type StatsResponse struct {
	UptimeSeconds   int64                  `json:"uptime_seconds"`
	TotalBackends   int                    `json:"total_backends"`
	HealthyBackends int                    `json:"healthy_backends"`
	Backends        []BackendStatsResponse `json:"backends"`
}

// BackendStatsResponse is the JSON response for each backend in /stats.
type BackendStatsResponse struct {
	Address           string `json:"address"`
	Alive             bool   `json:"alive"`
	ActiveConnections int    `json:"active_connections"`
	TotalConnections  int64  `json:"total_connections"`
}

// handleStats handles /stats requests and returns backend statistics.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	backendStats := s.pool.GetAllStats()

	healthyCount := 0
	backendResponses := make([]BackendStatsResponse, 0, len(backendStats))

	for _, b := range backendStats {
		if b.Alive {
			healthyCount++
		}

		backendResponses = append(backendResponses, BackendStatsResponse{
			Address:           b.Address,
			Alive:             b.Alive,
			ActiveConnections: b.ActiveConnections,
			TotalConnections:  b.TotalConnections,
		})
	}

	response := StatsResponse{
		UptimeSeconds:   int64(time.Since(s.startTime).Seconds()),
		TotalBackends:   len(backendStats),
		HealthyBackends: healthyCount,
		Backends:        backendResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthResponse is the JSON response for /health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// handleHealth handles /health requests and reports overall health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	healthyBackends := s.pool.GetHealthyBackends()

	w.Header().Set("Content-Type", "application/json")

	if len(healthyBackends) > 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthResponse{Status: "healthy"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(HealthResponse{Status: "unhealthy"})
	}
}

// GlobalStats tracks statistics across all backends.
type GlobalStats struct {
	TotalConnections   int64
	ActiveConnections  int64
	TotalBytesSent     int64
	TotalBytesReceived int64
	StartTime          time.Time
	mu                 sync.RWMutex
}

// NewGlobalStats creates a new GlobalStats instance.
func NewGlobalStats() *GlobalStats {
	return &GlobalStats{
		StartTime: time.Now(),
	}
}

// IncrementConnections atomically increments both total and active connections.
func (gs *GlobalStats) IncrementConnections() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.TotalConnections++
	gs.ActiveConnections++
}

// DecrementActiveConnections atomically decrements the active connection count.
func (gs *GlobalStats) DecrementActiveConnections() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.ActiveConnections <= 0 {
		return
	}

	gs.ActiveConnections--
}

// AddBytesSent atomically adds to the total bytes sent counter.
func (gs *GlobalStats) AddBytesSent(bytes int64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.TotalBytesSent += bytes
}

// AddBytesReceived atomically adds to the total bytes received counter.
func (gs *GlobalStats) AddBytesReceived(bytes int64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.TotalBytesReceived += bytes
}

// GetSnapshot returns a copy of the current statistics.
func (gs *GlobalStats) GetSnapshot() GlobalStats {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return GlobalStats{
		TotalConnections:   gs.TotalConnections,
		ActiveConnections:  gs.ActiveConnections,
		TotalBytesSent:     gs.TotalBytesSent,
		TotalBytesReceived: gs.TotalBytesReceived,
		StartTime:          gs.StartTime,
	}
}

// Uptime returns how long the load balancer has been running.
func (gs *GlobalStats) Uptime() time.Duration {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return time.Since(gs.StartTime)
}
