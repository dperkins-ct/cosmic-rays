package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
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
	// Operation Mode
	Mode               string `json:"mode"`                 // "demo" or "listen"
	MemorySize         string `json:"memory_size"`          // Memory size ("256MB", "auto", etc)
	MemorySizeAuto     string `json:"memory_size_auto"`     // Auto-sizing option ("10%", etc)
	MemoryAlignment    int    `json:"memory_alignment"`     // Memory alignment in bytes
	UseLockedMemory    bool   `json:"use_locked_memory"`    // Use mlock (degrades gracefully)
	UseProtectedMemory bool   `json:"use_protected_memory"` // Use mprotect (degrades gracefully)

	// Experiment parameters
	Duration      Duration `json:"duration"`        // Experiment duration
	ScanInterval  Duration `json:"scan_interval"`   // How often to scan memory
	ScanStrategy  string   `json:"scan_strategy"`   // "full", "sampled", "adaptive"
	SampleRate    float64  `json:"sample_rate"`     // For sampled scanning (0.0-1.0)
	PatternsToUse []string `json:"patterns_to_use"` // Which patterns to test

	// Detection vs Attribution separation
	EnableAttribution    bool    `json:"enable_attribution"`    // Enable heuristic attribution analysis
	AttributionThreshold float64 `json:"attribution_threshold"` // Confidence threshold for attribution
	EnableECCTelemetry   bool    `json:"enable_ecc_telemetry"`  // Attempt ECC telemetry (platform dependent)

	// Fault Injection (Demo Mode)
	Injection InjectionConfig `json:"injection"`

	// Output settings
	OutputDir           string   `json:"output_dir"`           // Directory for output files
	LogLevel            string   `json:"log_level"`            // Logging level
	EnableVisualization bool     `json:"enable_visualization"` // Generate plots
	ReportInterval      Duration `json:"report_interval"`      // How often to generate reports
	QuietMode           bool     `json:"quiet_mode"`           // Suppress non-essential output

	// Geographic/Environmental (for heuristics only)
	Location LocationConfig `json:"location"`
}

// InjectionConfig controls fault injection for demo mode
type InjectionConfig struct {
	Enabled        bool     `json:"enabled"`         // Enable fault injection
	Profile        string   `json:"profile"`         // "single", "multi", "burst", "mixed"
	Rate           float64  `json:"rate"`            // Injections per minute
	BurstSize      int      `json:"burst_size"`      // Number of flips in burst mode
	BurstInterval  Duration `json:"burst_interval"`  // Time between bursts
	RandomSeed     int64    `json:"random_seed"`     // Random seed (0 = random)
	TargetPatterns []string `json:"target_patterns"` // Specific patterns to target
}

// LocationConfig holds geographic data for heuristic analysis
type LocationConfig struct {
	Latitude  float64 `json:"latitude"`  // Geographic latitude
	Longitude float64 `json:"longitude"` // Geographic longitude
	Altitude  float64 `json:"altitude"`  // Altitude in meters
	Enabled   bool    `json:"enabled"`   // Include location-based heuristics
}

// DefaultConfig returns a demo configuration with laptop-safe defaults
func DefaultConfig() *Config {
	return DefaultDemoConfig()
}

// DefaultDemoConfig returns a demo configuration optimized for laptops
func DefaultDemoConfig() *Config {
	return &Config{
		Mode:                 "demo",
		MemorySize:           "256MB",
		MemorySizeAuto:       "auto",
		MemoryAlignment:      4096,
		UseLockedMemory:      false, // Safe default - will attempt with graceful degradation
		UseProtectedMemory:   false, // Safe default
		Duration:             Duration{15 * time.Minute},
		ScanInterval:         Duration{time.Second},
		ScanStrategy:         "full",
		SampleRate:           1.0,
		PatternsToUse:        []string{"alternating", "checksum", "random", "known"},
		EnableAttribution:    true,
		AttributionThreshold: 0.7,   // 70% confidence for heuristic attribution
		EnableECCTelemetry:   false, // Will attempt but degrade gracefully
		Injection: InjectionConfig{
			Enabled:        true,
			Profile:        "mixed",
			Rate:           5.0, // 5 injections per minute for quick demo
			BurstSize:      3,
			BurstInterval:  Duration{30 * time.Second},
			RandomSeed:     0,
			TargetPatterns: []string{},
		},
		OutputDir:           "./output",
		LogLevel:            "info",
		EnableVisualization: true,
		ReportInterval:      Duration{2 * time.Minute},
		QuietMode:           false,
		Location: LocationConfig{
			Latitude:  39.7392, // Denver (high altitude example)
			Longitude: -104.9903,
			Altitude:  1609.0, // 1 mile high
			Enabled:   true,
		},
	}
}

// DefaultListenConfig returns a long-running monitoring configuration
func DefaultListenConfig() *Config {
	return &Config{
		Mode:                 "listen",
		MemorySize:           "2GB",
		MemorySizeAuto:       "5%", // Use up to 5% of available memory
		MemoryAlignment:      4096,
		UseLockedMemory:      true, // More aggressive for long runs
		UseProtectedMemory:   true,
		Duration:             Duration{24 * time.Hour},
		ScanInterval:         Duration{5 * time.Second}, // Less frequent for stability
		ScanStrategy:         "sampled",
		SampleRate:           0.1, // Sample 10% of memory each scan
		PatternsToUse:        []string{"alternating", "checksum", "random", "known"},
		EnableAttribution:    true,
		AttributionThreshold: 0.95, // Higher confidence for real monitoring
		EnableECCTelemetry:   true,
		Injection: InjectionConfig{
			Enabled: false, // No injection in listen mode
		},
		OutputDir:           "./output",
		LogLevel:            "info",
		EnableVisualization: true,
		ReportInterval:      Duration{time.Hour},
		QuietMode:           false,
		Location: LocationConfig{
			Enabled: false, // Disable by default for privacy
		},
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

// Validate ensures the configuration is valid and applies smart defaults
func (c *Config) Validate() error {
	// Validate mode
	if c.Mode != "demo" && c.Mode != "listen" {
		return fmt.Errorf("mode must be 'demo' or 'listen', got '%s'", c.Mode)
	}

	// Validate memory size
	memSize, err := c.ParseMemorySize()
	if err != nil {
		return fmt.Errorf("invalid memory_size: %w", err)
	}
	if memSize <= 0 {
		return fmt.Errorf("memory size must be positive")
	}

	// Validate patterns
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
			return fmt.Errorf("invalid pattern '%s', valid patterns: alternating, checksum, random, known", pattern)
		}
	}

	// Validate scan strategy
	validStrategies := map[string]bool{
		"full":     true,
		"sampled":  true,
		"adaptive": true,
	}
	if !validStrategies[c.ScanStrategy] {
		return fmt.Errorf("invalid scan_strategy '%s', valid strategies: full, sampled, adaptive", c.ScanStrategy)
	}

	// Validate injection config
	if c.Mode == "demo" && !c.Injection.Enabled {
		return fmt.Errorf("injection should be enabled in demo mode for meaningful results")
	}
	if c.Mode == "listen" && c.Injection.Enabled {
		return fmt.Errorf("injection should be disabled in listen mode for authentic monitoring")
	}

	if c.Injection.Enabled {
		validProfiles := map[string]bool{
			"single": true,
			"multi":  true,
			"burst":  true,
			"mixed":  true,
		}
		if !validProfiles[c.Injection.Profile] {
			return fmt.Errorf("invalid injection profile '%s', valid profiles: single, multi, burst, mixed", c.Injection.Profile)
		}
	}

	return nil
}

// ParseMemorySize converts the string memory size to bytes, handling auto-sizing
func (c *Config) ParseMemorySize() (int64, error) {
	if c.MemorySize == "auto" {
		if c.MemorySizeAuto == "" {
			c.MemorySizeAuto = "10%" // Default to 10% of available memory
		}
		return c.parseAutoMemorySize()
	}

	return parseMemoryString(c.MemorySize)
}

// parseAutoMemorySize calculates memory size based on available system memory
func (c *Config) parseAutoMemorySize() (int64, error) {
	var memInfo runtime.MemStats
	runtime.ReadMemStats(&memInfo)

	availableMemory := int64(memInfo.Sys)

	// Parse percentage or fixed size
	if len(c.MemorySizeAuto) > 0 && c.MemorySizeAuto[len(c.MemorySizeAuto)-1] == '%' {
		percentStr := c.MemorySizeAuto[:len(c.MemorySizeAuto)-1]
		var percent float64
		if _, err := fmt.Sscanf(percentStr, "%f", &percent); err != nil {
			return 0, fmt.Errorf("invalid percentage in memory_size_auto: %s", c.MemorySizeAuto)
		}
		if percent <= 0 || percent > 50 {
			return 0, fmt.Errorf("memory percentage must be between 0%% and 50%%, got %f%%", percent)
		}
		return int64(float64(availableMemory) * (percent / 100.0)), nil
	}

	// Fixed size
	return parseMemoryString(c.MemorySizeAuto)
}

// parseMemoryString converts strings like "256MB", "2GB" to bytes
func parseMemoryString(sizeStr string) (int64, error) {
	var size float64
	var unit string

	n, err := fmt.Sscanf(sizeStr, "%f%s", &size, &unit)
	if err != nil || n != 2 {
		// Try without unit (assume bytes)
		var sizeInt int64
		if n, err := fmt.Sscanf(sizeStr, "%d", &sizeInt); err == nil && n == 1 {
			return sizeInt, nil
		}
		return 0, fmt.Errorf("invalid memory size format: %s (use format like '256MB' or '2GB')", sizeStr)
	}

	multiplier := int64(1)
	switch unit {
	case "B", "b":
		multiplier = 1
	case "KB", "kb":
		multiplier = 1024
	case "MB", "mb":
		multiplier = 1024 * 1024
	case "GB", "gb":
		multiplier = 1024 * 1024 * 1024
	case "TB", "tb":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("invalid memory size unit: %s (use B, KB, MB, GB, or TB)", unit)
	}

	return int64(size * float64(multiplier)), nil
}
