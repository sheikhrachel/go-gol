package utils

import (
	"encoding/json"
	"github.com/pkg/errors"
	"os"
	"time"
)

// Config holds the configuration for the game
type Config struct {
	Width               int           `json:"width"`
	Height              int           `json:"height"`
	FrameRate           time.Duration `json:"frame_rate"`
	AutoRestart         bool          `json:"auto_restart"`
	StagnationThreshold int           `json:"stagnation_threshold"`
	UseParallel         bool          `json:"use_parallel"`
	UseMemoryPool       bool          `json:"use_memory_pool"`
	UseBoundedGrid      bool          `json:"use_bounded_grid"`
	MaxGenerations      int           `json:"max_generations"`
	RandomDensity       float64       `json:"random_density"`
	InjectionCount      int           `json:"injection_count"`
	Interactive         bool          `json:"interactive"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Width:               60,
		Height:              30,
		FrameRate:           150 * time.Millisecond,
		AutoRestart:         true,
		StagnationThreshold: 5,
		UseParallel:         true,
		UseMemoryPool:       true,
		UseBoundedGrid:      true, // Enable active region optimization
		MaxGenerations:      1000,
		RandomDensity:       0.15,
		InjectionCount:      3,
		Interactive:         false,
	}
}

// LoadConfig loads configuration from JSON file
func LoadConfig(filename string) (Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(filename)
	if err != nil {
		return config, errors.Wrapf(err, "[LoadConfig] failed to read file: %+v", filename)
	}

	if err = json.Unmarshal(data, &config); err != nil {
		return config, errors.Wrapf(err, "[LoadConfig] failed to unmarshal data from file: %+v", filename)
	}

	return config, nil
}
