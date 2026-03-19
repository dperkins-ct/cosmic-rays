package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Duration.UnmarshalJSON
// ---------------------------------------------------------------------------

func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		"valid string 30m":          {input: `"30m"`, want: 30 * time.Minute},
		"valid string 1h30m":        {input: `"1h30m"`, want: 90 * time.Minute},
		"valid string 1s":           {input: `"1s"`, want: time.Second},
		"valid nanoseconds integer": {input: `5000000000`, want: 5 * time.Second},
		"zero nanoseconds":          {input: `0`, want: 0},
		"invalid duration string":   {input: `"not-a-duration"`, wantErr: true},
		"invalid json bare word":    {input: `invalid`, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tc.input), &d)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Duration != tc.want {
				t.Errorf("got %v, want %v", d.Duration, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseMemoryString (unexported – white-box)
// ---------------------------------------------------------------------------

func TestParseMemoryString(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    int64
		wantErr bool
	}{
		"1MB":                {input: "1MB", want: 1024 * 1024},
		"256MB":              {input: "256MB", want: 256 * 1024 * 1024},
		"1GB":                {input: "1GB", want: 1024 * 1024 * 1024},
		"1KB":                {input: "1KB", want: 1024},
		"512B":               {input: "512B", want: 512},
		"plain integer":      {input: "4096", want: 4096},
		"unknown unit":       {input: "100XB", wantErr: true},
		"non-numeric string": {input: "abc", wantErr: true},
		"empty string":       {input: "", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseMemoryString(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config.ParseMemorySize
// ---------------------------------------------------------------------------

func TestConfig_ParseMemorySize(t *testing.T) {
	tests := map[string]struct {
		cfg     Config
		wantMin int64
		wantErr bool
	}{
		"explicit 1MB":          {cfg: Config{MemorySize: "1MB"}, wantMin: 1024 * 1024},
		"explicit 512MB":        {cfg: Config{MemorySize: "512MB"}, wantMin: 512 * 1024 * 1024},
		"auto with 10 percent":  {cfg: Config{MemorySize: "auto", MemorySizeAuto: "10%"}, wantMin: 1},
		"invalid explicit size": {cfg: Config{MemorySize: "badsize"}, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tc.cfg.ParseMemorySize()
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got < tc.wantMin {
				t.Errorf("got %d, want >= %d", got, tc.wantMin)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config.Validate
// ---------------------------------------------------------------------------

func newValidDemoConfig() *Config {
	cfg := DefaultDemoConfig()
	cfg.MemorySize = "1MB"
	return cfg
}

func newValidListenConfig() *Config {
	cfg := DefaultListenConfig()
	cfg.MemorySize = "1MB"
	return cfg
}

func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		buildCfg func() *Config
		wantErr  bool
	}{
		"valid demo config":   {buildCfg: newValidDemoConfig},
		"valid listen config": {buildCfg: newValidListenConfig},
		"invalid mode": {
			buildCfg: func() *Config {
				c := newValidDemoConfig()
				c.Mode = "invalid"
				return c
			},
			wantErr: true,
		},
		"invalid memory size": {
			buildCfg: func() *Config {
				c := newValidDemoConfig()
				c.MemorySize = "badsize"
				return c
			},
			wantErr: true,
		},
		"empty patterns list": {
			buildCfg: func() *Config {
				c := newValidDemoConfig()
				c.PatternsToUse = []string{}
				return c
			},
			wantErr: true,
		},
		"invalid pattern name": {
			buildCfg: func() *Config {
				c := newValidDemoConfig()
				c.PatternsToUse = []string{"unknown_pattern"}
				return c
			},
			wantErr: true,
		},
		"invalid scan strategy": {
			buildCfg: func() *Config {
				c := newValidDemoConfig()
				c.ScanStrategy = "warp-speed"
				return c
			},
			wantErr: true,
		},
		"demo mode without injection": {
			buildCfg: func() *Config {
				c := newValidDemoConfig()
				c.Injection.Enabled = false
				return c
			},
			wantErr: true,
		},
		"listen mode with injection enabled": {
			buildCfg: func() *Config {
				c := newValidListenConfig()
				c.Injection.Enabled = true
				c.Injection.Profile = "single"
				return c
			},
			wantErr: true,
		},
		"demo mode with invalid injection profile": {
			buildCfg: func() *Config {
				c := newValidDemoConfig()
				c.Injection.Profile = "ultra-burst"
				return c
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.buildCfg().Validate()
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DefaultDemoConfig / DefaultListenConfig
// ---------------------------------------------------------------------------

func TestDefaultDemoConfig(t *testing.T) {
	tests := map[string]struct {
		check   func(*Config) bool
		wantMsg string
	}{
		"mode is demo":           {check: func(c *Config) bool { return c.Mode == "demo" }, wantMsg: "Mode should be demo"},
		"injection enabled":      {check: func(c *Config) bool { return c.Injection.Enabled }, wantMsg: "Injection should be enabled"},
		"patterns non-empty":     {check: func(c *Config) bool { return len(c.PatternsToUse) > 0 }, wantMsg: "Should have patterns"},
		"scan interval positive": {check: func(c *Config) bool { return c.ScanInterval.Duration > 0 }, wantMsg: "ScanInterval must be positive"},
	}

	cfg := DefaultDemoConfig()
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if !tc.check(cfg) {
				t.Error(tc.wantMsg)
			}
		})
	}
}

func TestDefaultListenConfig(t *testing.T) {
	tests := map[string]struct {
		check   func(*Config) bool
		wantMsg string
	}{
		"mode is listen":     {check: func(c *Config) bool { return c.Mode == "listen" }, wantMsg: "Mode should be listen"},
		"injection disabled": {check: func(c *Config) bool { return !c.Injection.Enabled }, wantMsg: "Injection must be disabled"},
		"patterns non-empty": {check: func(c *Config) bool { return len(c.PatternsToUse) > 0 }, wantMsg: "Should have patterns"},
	}

	cfg := DefaultListenConfig()
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if !tc.check(cfg) {
				t.Error(tc.wantMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SaveToFile / LoadFromFile round-trip
// ---------------------------------------------------------------------------

func TestConfig_SaveToFile(t *testing.T) {
	tests := map[string]struct {
		buildCfg func() *Config
	}{
		"demo config saves without error":   {buildCfg: newValidDemoConfig},
		"listen config saves without error": {buildCfg: newValidListenConfig},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "cfg.json")
			if err := tc.buildCfg().SaveToFile(path); err != nil {
				t.Fatalf("SaveToFile: %v", err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("expected file on disk: %v", err)
			}
		})
	}
}

func TestLoadFromFile_NonExistent(t *testing.T) {
	tests := map[string]struct {
		path    string
		wantErr bool
	}{
		"missing file": {path: "/tmp/does-not-exist-cosmic-test-12345.json", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			os.Remove(tc.path)
			_, err := LoadFromFile(tc.path)
			if tc.wantErr && err == nil {
				t.Error("expected error for non-existent file, got nil")
			}
		})
	}
}
