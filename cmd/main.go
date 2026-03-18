package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/dperkins/cosmic-rays/internal/handler"
)

const (
	version = "1.3.0"
	banner  = `
   ____ ___  ____  __  __ ___ ____   ____      _ __   ______  
  / ___/ _ \/ ___||  \/  |_ _/ ___| |  _ \    / \\ \ / / ___| 
 | |  | | | \___ \| |\/| || | |     | |_) |  / _ \\ V /\___ \ 
 | |__| |_| |___) | |  | || | |___  |  _ <  / ___ \| |  ___) |
  \____\___/|____/|_|  |_|___\____| |_| \_\/_/   \_\_| |____/ 
                                                                       
Cosmic Ray Memory Bit-Flip Detection System v%s
A scientific experiment to detect cosmic ray induced memory errors.
`
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Stdout, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, w io.Writer, args []string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	// Command line flags
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)
	var (
		configFile     = fs.String("config", "config.json", "Configuration file path")
		generateConfig = fs.Bool("generate-config", false, "Generate default configuration file and exit")
		showVersion    = fs.Bool("version", false, "Show version and exit")
		quiet          = fs.Bool("quiet", false, "Suppress banner and non-essential output")
	)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *showVersion {
		fmt.Fprintf(w, "cosmic-rays v%s\n", version)
		return nil
	}

	if !*quiet {
		fmt.Fprintf(w, banner, version)
	}

	// Generate default configuration if requested
	if *generateConfig {
		if err := handler.GenerateDefaultConfig(*configFile); err != nil {
			return fmt.Errorf("failed to generate config: %w", err)
		}
		fmt.Fprintf(w, "Default configuration saved to %s\n", *configFile)
		return nil
	}

	// Load configuration
	cfg, err := handler.LoadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if !*quiet {
		fmt.Fprintf(w, "Loaded configuration: %v\n", cfg)
		fmt.Fprintf(w, "Starting cosmic ray detection experiment...\n\n")
	}

	// Create experiment handler and run the application
	experimentHandler := handler.NewExperimentHandler(cfg)
	if err := experimentHandler.Run(ctx, w); err != nil {
		return fmt.Errorf("application failed: %w", err)
	}

	return nil
}
