package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"tcp_lb/backend"
	"tcp_lb/config"
	"tcp_lb/loadbalancer"
	"tcp_lb/stats"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		fmt.Printf("Could not load config.json: %v\n", err)
		fmt.Println("Using default configuration")
		cfg = config.DefaultConfig()
	}

	// Start backend servers
	for _, b := range cfg.Backends {
		go func(address string) {
			if err := backend.StartServer(address); err != nil {
				log.Printf("Backend server error: %v\n", err)
			}
		}(b.Address)
	}

	// Create load balancer
	lb := loadbalancer.New(cfg)

	// Start stats server
	statsServer := stats.NewServer(lb.GetPool(), ":8081")
	go func() {
		if err := statsServer.Start(); err != nil {
			log.Printf("Stats server error: %v\n", err)
		}
	}()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		lb.Stop()
		statsServer.Stop()
		os.Exit(0)
	}()

	// Start load balancer
	fmt.Printf("Load balancer listening on %s\n", cfg.ListenAddr)
	fmt.Printf("Stats server listening on :8081\n")
	fmt.Printf("Backends: %d started\n", len(cfg.Backends))

	if err := lb.Start(); err != nil {
		log.Fatal(err)
	}
}
