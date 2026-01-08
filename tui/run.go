package tui

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"tcp_lb/backend"
	"tcp_lb/config"
	"tcp_lb/loadbalancer"
)

// Run starts the TUI application with all components.
func Run() error {
	// Ensure TERM is set for WSL2 compatibility
	if os.Getenv("TERM") == "" {
		os.Setenv("TERM", "xterm-256color")
	}

	// Completely silence backend server logs to prevent TUI corruption
	log.SetOutput(io.Discard)
	log.SetFlags(0)

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		fmt.Printf("Could not load config.json: %v\n", err)
		fmt.Println("Using default configuration")
		cfg = config.DefaultConfig()
	}

	// Create and start load balancer
	lb := loadbalancer.New(cfg)
	go func() {
		if err := lb.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Load balancer error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Start backend servers (silently)
	// We must use the backends FROM the pool so they share the same sync.Cond and state
	for _, b := range lb.GetPool().GetBackends() {
		go func(b *backend.Backend) {
			backend.StartServer(b)
		}(b)
	}

	// Give servers and lb time to start
	time.Sleep(200 * time.Millisecond)

	// Start backend failure simulation
	go lb.GetPool().SimulateRandomBackendFailureAndRecoveryLoop()

	// Create and run TUI
	app := NewApp(lb.GetPool(), cfg)
	if err := app.Run(); err != nil {
		lb.Stop()
		return err
	}

	// Cleanup
	lb.Stop()
	return nil
}
