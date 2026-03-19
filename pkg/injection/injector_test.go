package injection

import (
	"testing"
	"time"
	"unsafe"

	"github.com/dperkins/cosmic-rays/internal/config"
)

// memPtr returns the uintptr of the first byte of a slice, matching the
// unsafe cast used in NewManager (allocator.go).
func memPtr(mem []byte) uintptr {
	if len(mem) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&mem[0]))
}

// ---------------------------------------------------------------------------
// NewInjector
// ---------------------------------------------------------------------------

func TestNewInjector(t *testing.T) {
	tests := map[string]struct {
		cfg config.InjectionConfig
	}{
		"enabled with fixed seed": {
			cfg: config.InjectionConfig{Enabled: true, Profile: "single", Rate: 1.0, RandomSeed: 42},
		},
		"disabled config": {
			cfg: config.InjectionConfig{Enabled: false},
		},
		"zero seed uses crypto rand": {
			cfg: config.InjectionConfig{Enabled: true, Profile: "single", Rate: 1.0, RandomSeed: 0},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mem := make([]byte, 1024)
			inj := NewInjector(tc.cfg, memPtr(mem), int64(len(mem)))
			if inj == nil {
				t.Fatal("NewInjector returned nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Injector.IsInjectionEnabled
// ---------------------------------------------------------------------------

func TestInjector_IsInjectionEnabled(t *testing.T) {
	tests := map[string]struct {
		configEnabled bool
		setActive     bool
		want          bool
	}{
		"config enabled active true":   {configEnabled: true, setActive: true, want: true},
		"config enabled active false":  {configEnabled: true, setActive: false, want: false},
		"config disabled active false": {configEnabled: false, setActive: false, want: false},
		"config disabled active true":  {configEnabled: false, setActive: true, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mem := make([]byte, 64)
			inj := NewInjector(config.InjectionConfig{Enabled: tc.configEnabled}, memPtr(mem), int64(len(mem)))
			inj.active = tc.setActive
			if got := inj.IsInjectionEnabled(); got != tc.want {
				t.Errorf("IsInjectionEnabled=%v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Injector.GetInjectionHistory
// ---------------------------------------------------------------------------

func TestInjector_GetInjectionHistory(t *testing.T) {
	tests := map[string]struct {
		preload int
	}{
		"empty history": {preload: 0},
		"three points":  {preload: 3},
		"ten points":    {preload: 10},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mem := make([]byte, 64)
			inj := NewInjector(config.InjectionConfig{Enabled: true, RandomSeed: 1}, memPtr(mem), int64(len(mem)))
			for i := 0; i < tc.preload; i++ {
				inj.injectionPoints = append(inj.injectionPoints, InjectionPoint{
					Offset: int64(i), Timestamp: time.Now(),
				})
				inj.injectedCount++
			}
			history := inj.GetInjectionHistory()
			if len(history) != tc.preload {
				t.Errorf("len(history)=%d, want %d", len(history), tc.preload)
			}
			// Returned slice must be a copy.
			if len(history) > 0 {
				history[0].Offset = 9999
				if inj.GetInjectionHistory()[0].Offset == 9999 {
					t.Error("GetInjectionHistory should return a copy, not an alias")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Injector.GetStats
// ---------------------------------------------------------------------------

func TestInjector_GetStats(t *testing.T) {
	tests := map[string]struct {
		injectedCount int64
		profile       string
	}{
		"zero injections single profile": {injectedCount: 0, profile: "single"},
		"five injections burst profile":  {injectedCount: 5, profile: "burst"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mem := make([]byte, 64)
			inj := NewInjector(
				config.InjectionConfig{Enabled: true, Profile: tc.profile, RandomSeed: 1},
				memPtr(mem), int64(len(mem)),
			)
			inj.injectedCount = tc.injectedCount
			stats := inj.GetStats()
			if stats.TotalInjected != tc.injectedCount {
				t.Errorf("TotalInjected=%d, want %d", stats.TotalInjected, tc.injectedCount)
			}
			if stats.ActiveProfile != tc.profile {
				t.Errorf("ActiveProfile=%q, want %q", stats.ActiveProfile, tc.profile)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Injector.Stop
// ---------------------------------------------------------------------------

func TestInjector_Stop(t *testing.T) {
	tests := map[string]struct {
		startActive bool
		wantActive  bool
	}{
		"stops active injector":         {startActive: true, wantActive: false},
		"stop already-stopped is no-op": {startActive: false, wantActive: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mem := make([]byte, 64)
			inj := NewInjector(config.InjectionConfig{Enabled: true, RandomSeed: 1}, memPtr(mem), int64(len(mem)))
			inj.active = tc.startActive
			inj.Stop()
			if inj.active != tc.wantActive {
				t.Errorf("active after Stop=%v, want %v", inj.active, tc.wantActive)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Injector.injectSingleBitFlip (unexported)
// ---------------------------------------------------------------------------

func TestInjector_injectSingleBitFlip(t *testing.T) {
	tests := map[string]struct {
		memSize    int
		setActive  bool
		wantChange bool // expect memory to have changed
	}{
		"active injector flips a bit":    {memSize: 64, setActive: true, wantChange: true},
		"inactive injector does nothing": {memSize: 64, setActive: false, wantChange: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mem := make([]byte, tc.memSize)
			// Store a copy of original state
			orig := make([]byte, tc.memSize)
			copy(orig, mem)

			inj := NewInjector(
				config.InjectionConfig{Enabled: true, RandomSeed: 42},
				memPtr(mem), int64(len(mem)),
			)
			inj.active = tc.setActive

			if err := inj.injectSingleBitFlip(); err != nil {
				t.Fatalf("injectSingleBitFlip: %v", err)
			}

			changed := false
			for i := range mem {
				if mem[i] != orig[i] {
					changed = true
					break
				}
			}
			if changed != tc.wantChange {
				t.Errorf("memory changed=%v, want %v", changed, tc.wantChange)
			}
			if tc.setActive {
				if inj.injectedCount != 1 {
					t.Errorf("injectedCount=%d, want 1", inj.injectedCount)
				}
				if len(inj.injectionPoints) != 1 {
					t.Errorf("len(injectionPoints)=%d, want 1", len(inj.injectionPoints))
				}
			}
		})
	}
}
