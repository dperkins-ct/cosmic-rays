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
	det, err := detector.NewScanner(memMgr, cfg.ScanInterval, cfg.PatternsToUse)
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
	// Run the experiment
	resultChan := make(chan error, 1)
	go func() {
		resultChan <- experiment.Run()
	}()

	// Wait for completion or context cancellation
	select {
	case err := <-resultChan:
		if err != nil {
			return fmt.Errorf("experiment failed: %w", err)
		}
		fmt.Fprintf(w, "Experiment completed successfully!\n")
	case <-ctx.Done():
		fmt.Fprintf(w, "\nReceived shutdown signal, shutting down gracefully...\n")
		experiment.Stop()
		<-resultChan // Wait for experiment to stop
		fmt.Fprintf(w, "Shutdown complete.\n")
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

// Run starts the experiment detection process
func (e *Experiment) Run() error {
	// Start the detection process
	if err := e.detector.Start(e.stopChan); err != nil {
		return fmt.Errorf("failed to start detector: %w", err)
	}

	e.logger.Info("Cosmic ray detection experiment started", map[string]interface{}{
		"memory_size": e.config.MemorySize,
		"duration":    e.config.Duration,
		"patterns":    e.config.PatternsToUse,
	})

	// Main experiment loop - will be implemented with actual detection logic
	// For now, just run for the specified duration
	select {
	case <-e.stopChan:
		e.logger.Info("Experiment stopped by user", nil)
	}

	return nil
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
