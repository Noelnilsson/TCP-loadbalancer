package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds all configuration for the load balancer.
// `json:<name>` help maps JSON fields to struct fields
type Config struct {
	ListenAddr          string          `json:"listen_addr"`
	Backends            []BackendConfig `json:"backends"`
	HealthCheckInterval time.Duration   `json:"health_check_interval_seconds"`
	ConnectTimeout      time.Duration   `json:"connect_timeout_seconds"`
}

// BackendConfig holds configuration for a single backend server.
type BackendConfig struct {
	Address string `json:"address"`
	Weight  int    `json:"weight"`
}

// LoadConfig reads configuration from a JSON file at the given path.
// Returns an error if the file cannot be read or parsed.
func LoadConfig(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := new(Config)
	if err = json.Unmarshal(fileBytes, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	config.HealthCheckInterval *= time.Second
	config.ConnectTimeout *= time.Second

	return config, nil
}

// DefaultConfig returns a Config with sensible default values.
// Useful for testing or when no config file is provided.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr: ":8080",
		Backends: []BackendConfig{
			{Address: "localhost:9001", Weight: 1},
			{Address: "localhost:9002", Weight: 1},
		},
		HealthCheckInterval: 10 * time.Second,
		ConnectTimeout:      5 * time.Second,
	}
}
