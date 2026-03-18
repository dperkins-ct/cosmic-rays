package handler

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dperkins/cosmic-rays/internal/config"
	"github.com/dperkins/cosmic-rays/pkg/detector"
	"github.com/dperkins/cosmic-rays/pkg/injection"
	"github.com/dperkins/cosmic-rays/pkg/memory"
	"github.com/dperkins/cosmic-rays/pkg/output"
)

// ExperimentHandler encapsulates all the application logic
type ExperimentHandler struct {
	config *config.Config
}

// NewExperimentHandler creates a new experiment handler
func NewExperimentHandler(cfg *config.Config) *ExperimentHandler {
	return &ExperimentHandler{
		config: cfg,
	}
}

// Experiment represents the complete memory corruption detection experiment
type Experiment struct {
	config        *config.Config
	memoryManager *memory.Manager
	scanner       *detector.Scanner
	logger        *output.Logger
	startTime     time.Time
	stopChan      chan struct{}
}

// GenerateDefaultConfig creates and saves a default configuration file
func GenerateDefaultConfig(filename string) error {
	cfg := config.DefaultConfig()
	return cfg.SaveToFile(filename)
}

// LoadConfig loads and validates configuration from a file
func LoadConfig(filename string) (*config.Config, error) {
	cfg, err := config.LoadFromFile(filename)
	if err != nil {
		// If config file doesn't exist, create default
		if os.IsNotExist(err) {
			fmt.Printf("Configuration file %s not found, creating default configuration...\n", filename)
			cfg = config.DefaultConfig()
			if saveErr := cfg.SaveToFile(filename); saveErr != nil {
				fmt.Printf("Warning: Could not save default config: %v\n", saveErr)
			}
		} else {
			return nil, err
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// InitializeExperiment sets up all components needed for the experiment
func (h *ExperimentHandler) InitializeExperiment() (*Experiment, error) {
	cfg := h.config

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Initialize memory manager
	memMgr, err := memory.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize memory manager: %w", err)
	}

	// Initialize detector
	scanner := detector.NewScanner(cfg, memMgr)

	// Initialize logger
	logger, err := output.NewLogger(cfg.OutputDir, cfg.LogLevel)
	if err != nil {

		memMgr.Cleanup()
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return &Experiment{
		config:        cfg,
		memoryManager: memMgr,
		scanner:       scanner,
		logger:        logger,
		startTime:     time.Now(),
		stopChan:      make(chan struct{}),
	}, nil
}

// RunExperiment runs the cosmic ray detection experiment with context
func (h *ExperimentHandler) RunExperiment(ctx context.Context, w io.Writer, experiment *Experiment) error {
	// Create a context that will cancel after the experiment duration
	expCtx, cancel := context.WithTimeout(ctx, experiment.config.Duration.Duration)
	defer cancel()

	// Run the experiment
	resultChan := make(chan error, 1)
	go func() {
		resultChan <- experiment.RunWithContext(expCtx)
	}()

	// Wait for completion or context cancellation
	select {
	case err := <-resultChan:
		if err != nil {
			return fmt.Errorf("experiment failed: %w", err)
		}
		fmt.Fprintf(w, "\n=== EXPERIMENT COMPLETED SUCCESSFULLY ===\n")
		h.printHumanResults(w, experiment)
	case <-ctx.Done():
		fmt.Fprintf(w, "\n=== EXPERIMENT INTERRUPTED ===\n")
		experiment.Stop()
		<-resultChan // Wait for experiment to stop
		h.printHumanResults(w, experiment)
	}

	return nil
}

// Run executes the main application logic with context
func (h *ExperimentHandler) Run(ctx context.Context, w io.Writer) error {
	// Initialize experiment components
	experiment, err := h.InitializeExperiment()
	if err != nil {
		return fmt.Errorf("failed to initialize experiment: %w", err)
	}
	defer experiment.Cleanup()

	// Run the experiment with context
	return h.RunExperiment(ctx, w, experiment)
}

// printHumanResults formats and displays human-readable experiment results
func (h *ExperimentHandler) printHumanResults(w io.Writer, experiment *Experiment) {
	stats := experiment.scanner.GetStats()

	fmt.Fprintf(w, "\n=== EXPERIMENT RESULTS ===\n")
	// Parse memory size for display
	memSize, err := experiment.config.ParseMemorySize()
	if err != nil {
		memSize = 0
	}
	fmt.Fprintf(w, "Memory Monitored: %.1f MB\n", float64(memSize)/(1024*1024))
	fmt.Fprintf(w, "Duration: %v\n", experiment.config.Duration.Duration)
	fmt.Fprintf(w, "Total Scans: %v\n", stats.ScanCount)
	fmt.Fprintf(w, "Events Per Minute: %.2f\n", stats.EventsPerMinute)
	fmt.Fprintf(w, "\n--- Detection Analysis ---\n")
	fmt.Fprintf(w, "Total Events: %v\n", stats.EventCount)
	fmt.Fprintf(w, "Scans Per Minute: %.2f\n", stats.ScansPerMinute)

	// Display injection statistics if available
	if stats.InjectionStats != nil {
		if injStats, ok := stats.InjectionStats.(injection.InjectionStats); ok {
			fmt.Fprintf(w, "\n--- Fault Injection Analysis ---\n")
			fmt.Fprintf(w, "Injection Profile: %s\n", injStats.ActiveProfile)
			fmt.Fprintf(w, "Total Injections: %d\n", injStats.TotalInjected)
			fmt.Fprintf(w, "Injection Rate: %.2f per minute\n", injStats.InjectionRate)
			if !injStats.LastInjection.IsZero() {
				fmt.Fprintf(w, "Last Injection: %v\n", injStats.LastInjection.Format("15:04:05"))
			}
		}
	}

	if stats.EventCount > 0 {
		fmt.Fprintf(w, "\n*** MEMORY EVENTS DETECTED! ***\n")
		fmt.Fprintf(w, "NOTE: Events may be injected faults (in demo mode) or\n")
		fmt.Fprintf(w, "genuine memory corruption. Attribution analysis\n")
		fmt.Fprintf(w, "provides heuristic likelihood estimates.\n")
		fmt.Fprintf(w, "\nFor true cosmic ray detection, consider:\n")
		fmt.Fprintf(w, "- ECC memory to distinguish single vs multi-bit errors\n")
		fmt.Fprintf(w, "- Memory protection to prevent program interference\n")
		fmt.Fprintf(w, "- Longer observation periods (days/weeks)\n")
		fmt.Fprintf(w, "- Statistical analysis of flip patterns\n")
	} else {
		fmt.Fprintf(w, "\n*** No bit flips detected during monitoring period ***\n")
		fmt.Fprintf(w, "This suggests good memory stability or short observation time.\n")
	}

	// Log to file as well
	// Convert ScanStats to map format for logger
	statsMap := map[string]interface{}{
		"scan_count":          stats.ScanCount,
		"event_count":         stats.EventCount,
		"last_scan":           stats.LastScan,
		"scans_per_minute":    stats.ScansPerMinute,
		"events_per_minute":   stats.EventsPerMinute,
		"attribution_enabled": stats.AttributionEnabled,
		"running_time":        stats.RunningTime,
		"injection_stats":     stats.InjectionStats,
	}
	experiment.logger.LogStatistics(statsMap)
	fmt.Fprintf(w, "\nDetailed statistics logged to output directory.\n")
}

// RunWithContext starts the experiment detection process with context support
func (e *Experiment) RunWithContext(ctx context.Context) error {
	// Start the detection process
	if err := e.scanner.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scanner: %w", err)
	}

	e.logger.Info("Cosmic ray detection experiment started", map[string]interface{}{
		"memory_size": e.config.MemorySize,
		"duration":    e.config.Duration.Duration,
		"patterns":    e.config.PatternsToUse,
	})

	// Main experiment loop - run until context is done
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			e.logger.Info("Experiment completed - duration reached", map[string]interface{}{
				"duration": e.config.Duration.Duration,
			})
		} else {
			e.logger.Info("Experiment stopped by user", nil)
		}
		return nil
	case <-e.stopChan:
		e.logger.Info("Experiment stopped by user", nil)
		return nil
	}
}

func (e *Experiment) Run() error {
	return e.RunWithContext(context.Background())
}

// Stop gracefully stops the experiment
func (e *Experiment) Stop() {
	close(e.stopChan)
}

// Cleanup releases all resources used by the experiment
func (e *Experiment) Cleanup() {
	if e.scanner != nil {
		e.scanner.Close()
	}
	if e.memoryManager != nil {
		e.memoryManager.Cleanup()
	}
	if e.logger != nil {
		e.logger.Close()
	}
}
