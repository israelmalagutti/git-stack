package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config represents the gs configuration
type Config struct {
	Version     string    `json:"version"`
	Trunk       string    `json:"trunk"`
	Initialized time.Time `json:"initialized"`
}

// Load reads the config from the specified path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("gs not initialized (run 'gs init')")
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Save writes the config to the specified path
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// NewConfig creates a new config with default values
func NewConfig(trunk string) *Config {
	return &Config{
		Version:     "1.0.0",
		Trunk:       trunk,
		Initialized: time.Now(),
	}
}

// IsInitialized checks if gs is initialized in the given path
func IsInitialized(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
