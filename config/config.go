package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds load balancer configuration.
type Config struct {
	ListenAddr          string          `json:"listen_addr"`
	Backends            []BackendConfig `json:"backends"`
	HealthCheckInterval time.Duration   `json:"health_check_interval_seconds"`
	ConnectTimeout      time.Duration   `json:"connect_timeout_seconds"`
}

// BackendConfig holds backend server configuration.
type BackendConfig struct {
	Address string `json:"address"`
	Weight  int    `json:"weight"`
}

// LoadConfig reads configuration from a JSON file.
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

// DefaultConfig returns default configuration values.
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
