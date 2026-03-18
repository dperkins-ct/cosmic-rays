package handler

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dperkins/cosmic-rays/internal/config"
	"github.com/dperkins/cosmic-rays/pkg/detector"
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

// Experiment represents the complete cosmic ray detection experiment
type Experiment struct {
	config        *config.Config
	memoryManager *memory.Manager
	detector      *detector.Scanner
	logger        *output.Logger
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
	memMgr, err := memory.NewManager(cfg.MemorySize, cfg.MemoryAlignment, cfg.UseLockedMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize memory manager: %w", err)
	}

	// Initialize detector
	det, err := detector.NewScanner(memMgr, cfg.ScanInterval.Duration, cfg.PatternsToUse)
	if err != nil {
		memMgr.Cleanup()
		return nil, fmt.Errorf("failed to initialize detector: %w", err)
	}

	// Initialize logger
	logger, err := output.NewLogger(cfg.OutputDir, cfg.LogLevel)
	if err != nil {
		det.Close()
		memMgr.Cleanup()
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return &Experiment{
		config:        cfg,
		memoryManager: memMgr,
		detector:      det,
		logger:        logger,
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
	stats := experiment.detector.GetStats()

	fmt.Fprintf(w, "\n=== EXPERIMENT RESULTS ===\n")
	fmt.Fprintf(w, "Memory Monitored: %.1f MB\n", float64(experiment.config.MemorySize)/(1024*1024))
	fmt.Fprintf(w, "Duration: %v\n", experiment.config.Duration.Duration)
	fmt.Fprintf(w, "Total Scans: %v\n", stats["total_scans"])
	fmt.Fprintf(w, "Bytes Scanned: %v\n", stats["bytes_scanned"])
	fmt.Fprintf(w, "\n--- Bit Flip Analysis ---\n")
	fmt.Fprintf(w, "Total Bit Flips: %v\n", stats["total_bit_flips"])
	fmt.Fprintf(w, "Single Bit Flips: %v\n", stats["single_bit_flips"])
	fmt.Fprintf(w, "Multi Bit Flips: %v\n", stats["multiple_bit_flips"])
	fmt.Fprintf(w, "Flip Rate: %.6f per bit\n", stats["bit_flip_rate"])

	if flips, ok := stats["total_bit_flips"].(int64); ok && flips > 0 {
		fmt.Fprintf(w, "\n*** POTENTIAL COSMIC RAY EVENTS DETECTED! ***\n")
		fmt.Fprintf(w, "NOTE: Most detected 'flips' are likely memory initialization\n")
		fmt.Fprintf(w, "artifacts rather than actual cosmic ray events.\n")
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
	experiment.logger.LogStatistics(stats)
	fmt.Fprintf(w, "\nDetailed statistics logged to output directory.\n")
}

// RunWithContext starts the experiment detection process with context support
func (e *Experiment) RunWithContext(ctx context.Context) error {
	// Start the detection process
	if err := e.detector.Start(e.stopChan); err != nil {
		return fmt.Errorf("failed to start detector: %w", err)
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
	if e.detector != nil {
		e.detector.Close()
	}
	if e.memoryManager != nil {
		e.memoryManager.Cleanup()
	}
	if e.logger != nil {
		e.logger.Close()
	}
}
