package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Duration wraps time.Duration to support both string and nanosecond unmarshaling
type Duration struct {
	time.Duration
}

// UnmarshalJSON allows Duration to accept both string ("1m30s") and number (nanoseconds) formats
func (d *Duration) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first ("1m30s" format)
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		duration, err := time.ParseDuration(str)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", str, err)
		}
		d.Duration = duration
		return nil
	}

	// Fall back to nanoseconds (number format)
	var ns int64
	if err := json.Unmarshal(data, &ns); err != nil {
		return fmt.Errorf("duration must be a string (\"1m30s\") or number (nanoseconds): %w", err)
	}
	d.Duration = time.Duration(ns)
	return nil
}

// Config represents the experiment configuration
type Config struct {
	// Memory configuration
	MemorySize      int64 `json:"memory_size"`       // Memory to allocate in bytes
	MemoryAlignment int   `json:"memory_alignment"`  // Memory alignment in bytes
	UseLockedMemory bool  `json:"use_locked_memory"` // Use mlock to prevent swapping

	// Experiment parameters
	Duration      Duration `json:"duration"`        // Experiment duration
	ScanInterval  Duration `json:"scan_interval"`   // How often to scan memory
	PatternsToUse []string `json:"patterns_to_use"` // Which patterns to test

	// Detection settings
	EnableECCDetection bool    `json:"enable_ecc_detection"` // Detect ECC memory
	FlipThreshold      float64 `json:"flip_threshold"`       // Statistical threshold for cosmic ray detection

	// Output settings
	OutputDir           string   `json:"output_dir"`           // Directory for output files
	LogLevel            string   `json:"log_level"`            // Logging level
	EnableVisualization bool     `json:"enable_visualization"` // Generate plots
	ReportInterval      Duration `json:"report_interval"`      // How often to generate reports

	// Geographic/Environmental
	Latitude  float64 `json:"latitude"`  // Geographic latitude for cosmic ray flux correlation
	Longitude float64 `json:"longitude"` // Geographic longitude
	Altitude  float64 `json:"altitude"`  // Altitude in meters
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		MemorySize:          10 * 1024 * 1024 * 1024, // 10GB
		MemoryAlignment:     4096,                    // Page-aligned
		UseLockedMemory:     true,
		Duration:            Duration{24 * time.Hour},
		ScanInterval:        Duration{time.Second},
		PatternsToUse:       []string{"alternating", "checksum", "random", "known"},
		EnableECCDetection:  true,
		FlipThreshold:       0.95, // 95% confidence
		OutputDir:           "./output",
		LogLevel:            "info",
		EnableVisualization: true,
		ReportInterval:      Duration{time.Hour},
		Latitude:            37.7749, // Default to San Francisco
		Longitude:           -122.4194,
		Altitude:            52.0,
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	return config, nil
}

// SaveToFile saves the configuration to a JSON file
func (c *Config) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filename, err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.MemorySize <= 0 {
		return fmt.Errorf("memory size must be positive")
	}

	if c.MemorySize > 100*1024*1024*1024 { // 100GB limit
		return fmt.Errorf("memory size too large (>100GB)")
	}

	if c.Duration.Duration <= 0 {
		return fmt.Errorf("experiment duration must be positive")
	}

	if c.ScanInterval.Duration <= 0 {
		return fmt.Errorf("scan interval must be positive")
	}

	if len(c.PatternsToUse) == 0 {
		return fmt.Errorf("at least one pattern must be specified")
	}

	validPatterns := map[string]bool{
		"alternating": true,
		"checksum":    true,
		"random":      true,
		"known":       true,
	}

	for _, pattern := range c.PatternsToUse {
		if !validPatterns[pattern] {
			return fmt.Errorf("invalid pattern: %s", pattern)
		}
	}

	if c.FlipThreshold < 0 || c.FlipThreshold > 1 {
		return fmt.Errorf("flip threshold must be between 0 and 1")
	}

	return nil
}

// String returns a human-readable representation of the config
func (c *Config) String() string {
	return fmt.Sprintf("Config{MemorySize: %d bytes (%.1f GB), Duration: %v, ScanInterval: %v, Patterns: %v}",
		c.MemorySize,
		float64(c.MemorySize)/(1024*1024*1024),
		c.Duration.Duration,
		c.ScanInterval.Duration,
		c.PatternsToUse,
	)
}
