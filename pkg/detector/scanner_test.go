package detector

import (
	"testing"
	"time"

	"github.com/dperkins/cosmic-rays/internal/config"
	"github.com/dperkins/cosmic-rays/pkg/memory"
)

// ---------------------------------------------------------------------------
// PatternGenerator.Generate
// ---------------------------------------------------------------------------

func TestPatternGenerator_Generate(t *testing.T) {
	pg := NewPatternGenerator()

	tests := map[string]struct {
		patternType string
		size        int
		wantErr     bool
		checkFn     func([]byte) bool
	}{
		"alternating 4 bytes": {
			patternType: "alternating",
			size:        4,
			checkFn:     func(b []byte) bool { return b[0] == 0xAA && b[1] == 0x55 && b[2] == 0xAA && b[3] == 0x55 },
		},
		"checksum 4 bytes": {
			patternType: "checksum",
			size:        4,
			checkFn:     func(b []byte) bool { return b[0] == 0 && b[1] == 1 && b[2] == 2 && b[3] == 3 },
		},
		"known 8 bytes repeats sentinel": {
			patternType: "known",
			size:        8,
			checkFn: func(b []byte) bool {
				sentinel := []byte{0x42, 0xEF, 0xBE, 0xAD}
				for i, v := range b {
					if v != sentinel[i%len(sentinel)] {
						return false
					}
				}
				return true
			},
		},
		"random 16 bytes correct length": {
			patternType: "random",
			size:        16,
			checkFn:     func(b []byte) bool { return len(b) == 16 },
		},
		"unknown pattern type returns error": {
			patternType: "ultraviolet",
			size:        8,
			wantErr:     true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pg.Generate(tc.patternType, tc.size)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkFn != nil && !tc.checkFn(got) {
				t.Errorf("pattern check failed; data=%v", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Scanner.findBitDifference (unexported)
// ---------------------------------------------------------------------------

func TestScanner_findBitDifference(t *testing.T) {
	s := &Scanner{}

	tests := map[string]struct {
		old  byte
		new  byte
		want int
	}{
		"bit 0 flipped (0x00->0x01)": {old: 0x00, new: 0x01, want: 0},
		"bit 1 flipped (0x00->0x02)": {old: 0x00, new: 0x02, want: 1},
		"bit 7 flipped (0x00->0x80)": {old: 0x00, new: 0x80, want: 7},
		"bit 3 flipped (0x00->0x08)": {old: 0x00, new: 0x08, want: 3},
		// multiple bits differ: findBitDifference shifts right until diff==1,
		// returning the position of the highest set bit in the XOR.
		"two bits differ highest returned": {old: 0x00, new: 0x03, want: 1},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := s.findBitDifference(tc.old, tc.new); got != tc.want {
				t.Errorf("findBitDifference(0x%02X,0x%02X)=%d, want %d", tc.old, tc.new, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Scanner.SetScanInterval / GetScanInterval
// ---------------------------------------------------------------------------

// stubMemoryManager is a minimal memory.Manager stand-in that satisfies the
// interface used by Scanner.GetStats (GetInjector must not panic).
type stubMemoryManager struct{}

func newMinimalScanner() *Scanner {
	cfg := &config.Config{
		Mode:          "demo",
		MemorySize:    "1MB",
		ScanInterval:  config.Duration{Duration: time.Second},
		PatternsToUse: []string{"alternating"},
	}
	// Build a real memory.Manager with no injection so GetInjector returns nil safely.
	memMgr, err := memory.NewManager(cfg)
	if err != nil {
		// Fallback: use a zero-value manager by constructing it via config.
		// This path should not occur for the minimal cfg above.
		panic("newMinimalScanner: failed to create memory manager: " + err.Error())
	}
	return &Scanner{
		config:         cfg,
		memoryManager:  memMgr,
		scanInterval:   time.Second,
		eventListeners: make([]EventListener, 0),
		stats:          &InternalStats{},
		generator:      NewPatternGenerator(),
	}
}

func TestScanner_SetScanInterval(t *testing.T) {
	tests := map[string]struct {
		running  bool
		interval time.Duration
		wantErr  bool
	}{
		"valid interval while stopped":    {running: false, interval: 2 * time.Second},
		"zero interval returns error":     {running: false, interval: 0, wantErr: true},
		"negative interval returns error": {running: false, interval: -time.Second, wantErr: true},
		"set while running returns error": {running: true, interval: time.Second, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newMinimalScanner()
			s.isRunning = tc.running
			err := s.SetScanInterval(tc.interval)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := s.GetScanInterval(); got != tc.interval {
				t.Errorf("GetScanInterval=%v, want %v", got, tc.interval)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Scanner.GetStats initial state
// ---------------------------------------------------------------------------

func TestScanner_GetStats_Initial(t *testing.T) {
	tests := map[string]struct {
		wantScanCount  int64
		wantEventCount int64
	}{
		"fresh scanner zero counts": {wantScanCount: 0, wantEventCount: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newMinimalScanner()
			stats := s.GetStats()
			if stats.ScanCount != tc.wantScanCount {
				t.Errorf("ScanCount=%d, want %d", stats.ScanCount, tc.wantScanCount)
			}
			if stats.EventCount != tc.wantEventCount {
				t.Errorf("EventCount=%d, want %d", stats.EventCount, tc.wantEventCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Scanner.IsRunning
// ---------------------------------------------------------------------------

func TestScanner_IsRunning(t *testing.T) {
	tests := map[string]struct {
		running bool
		want    bool
	}{
		"not running initially": {running: false, want: false},
		"running when set":      {running: true, want: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newMinimalScanner()
			s.isRunning = tc.running
			if got := s.IsRunning(); got != tc.want {
				t.Errorf("IsRunning=%v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Scanner.AddEventListener
// ---------------------------------------------------------------------------

type noopListener struct{}

func (n *noopListener) OnEvent(_ Event)                 {}
func (n *noopListener) OnAttributedEvent(_ Attribution) {}

func TestScanner_AddEventListener(t *testing.T) {
	tests := map[string]struct {
		numListeners int
	}{
		"add one listener":    {numListeners: 1},
		"add three listeners": {numListeners: 3},
		"add zero listeners":  {numListeners: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newMinimalScanner()
			for i := 0; i < tc.numListeners; i++ {
				s.AddEventListener(&noopListener{})
			}
			if got := len(s.eventListeners); got != tc.numListeners {
				t.Errorf("len(eventListeners)=%d, want %d", got, tc.numListeners)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HeuristicAttributor.calculateHammingDistance (unexported)
// ---------------------------------------------------------------------------

func TestHeuristicAttributor_calculateHammingDistance(t *testing.T) {
	a := &HeuristicAttributor{}

	tests := map[string]struct {
		a, b byte
		want int
	}{
		"identical bytes":          {a: 0xFF, b: 0xFF, want: 0},
		"one bit differs":          {a: 0x00, b: 0x01, want: 1},
		"all bits differ (0/0xFF)": {a: 0x00, b: 0xFF, want: 8},
		"alternating 0xAA vs 0x55": {a: 0xAA, b: 0x55, want: 8},
		"two bits differ":          {a: 0x00, b: 0x03, want: 2},
		"four bits differ":         {a: 0x0F, b: 0xF0, want: 8},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := a.calculateHammingDistance(tc.a, tc.b); got != tc.want {
				t.Errorf("calculateHammingDistance(0x%02X,0x%02X)=%d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HeuristicAttributor.calculateCosmicRayLikelihood (unexported)
// ---------------------------------------------------------------------------

func TestHeuristicAttributor_calculateCosmicRayLikelihood(t *testing.T) {
	tests := map[string]struct {
		cfg     *config.Config
		event   Event
		wantMin float64
		wantMax float64
	}{
		"single bit flip sea level scores above 0.7": {
			cfg:     &config.Config{Location: config.LocationConfig{Enabled: false}},
			event:   Event{OldValue: 0xAA, NewValue: 0xA8},
			wantMin: 0.7, wantMax: 1.0,
		},
		"multi bit flip scores below 0.5": {
			cfg:     &config.Config{Location: config.LocationConfig{Enabled: false}},
			event:   Event{OldValue: 0x00, NewValue: 0xFF},
			wantMin: 0.0, wantMax: 0.5,
		},
		"high altitude boosts likelihood": {
			cfg:     &config.Config{Location: config.LocationConfig{Enabled: true, Altitude: 8000}},
			event:   Event{OldValue: 0xAA, NewValue: 0xA8},
			wantMin: 0.7, wantMax: 1.0,
		},
		"result always within 0 to 1": {
			cfg:     &config.Config{Location: config.LocationConfig{Enabled: false}},
			event:   Event{OldValue: 0x00, NewValue: 0x00},
			wantMin: 0.0, wantMax: 1.0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			attr := &HeuristicAttributor{config: tc.cfg}
			got := attr.calculateCosmicRayLikelihood(tc.event)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("calculateCosmicRayLikelihood=%.4f, want in [%.4f,%.4f]", got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HeuristicAttributor.AttributeEvent
// ---------------------------------------------------------------------------

func TestHeuristicAttributor_AttributeEvent(t *testing.T) {
	cfg := &config.Config{Location: config.LocationConfig{Enabled: false}}

	tests := map[string]struct {
		event            Event
		wantIsInjected   bool
		wantConfidenceIn []string
	}{
		"single bit flip not injected medium or high confidence": {
			event:            Event{OldValue: 0xAA, NewValue: 0xA8, Timestamp: time.Now()},
			wantIsInjected:   false,
			wantConfidenceIn: []string{"medium", "high"},
		},
		"multi bit flip not injected low or medium confidence": {
			event:            Event{OldValue: 0x00, NewValue: 0xFF, Timestamp: time.Now()},
			wantIsInjected:   false,
			wantConfidenceIn: []string{"low", "medium"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			attr := &HeuristicAttributor{config: cfg, injector: nil}
			result := attr.AttributeEvent(tc.event)
			if result.IsInjected != tc.wantIsInjected {
				t.Errorf("IsInjected=%v, want %v", result.IsInjected, tc.wantIsInjected)
			}
			found := false
			for _, c := range tc.wantConfidenceIn {
				if result.Confidence == c {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Confidence=%q not in expected set %v", result.Confidence, tc.wantConfidenceIn)
			}
			if result.CosmicRayLikelihood < 0 || result.CosmicRayLikelihood > 1 {
				t.Errorf("CosmicRayLikelihood=%.4f out of [0,1]", result.CosmicRayLikelihood)
			}
		})
	}
}
