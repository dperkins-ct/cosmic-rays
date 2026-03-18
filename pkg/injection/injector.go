package injection

import (
	"context"
	"crypto/rand"
	"fmt"
	mathrand "math/rand"
	"sync"
	"time"
	"unsafe"

	"github.com/dperkins/cosmic-rays/internal/config"
)

// Injector handles controlled fault injection for demo purposes
type Injector struct {
	config    config.InjectionConfig
	rng       *mathrand.Rand
	memory    uintptr
	memoryLen int64
	active    bool
	mutex     sync.RWMutex

	// Statistics
	injectedCount   int64
	lastInjection   time.Time
	injectionPoints []InjectionPoint
}

// InjectionPoint records where and when a fault was injected
type InjectionPoint struct {
	Offset        int64     `json:"offset"`
	OriginalValue byte      `json:"original_value"`
	InjectedValue byte      `json:"injected_value"`
	Timestamp     time.Time `json:"timestamp"`
	Pattern       string    `json:"pattern"`
	BitPosition   uint      `json:"bit_position"`
}

// InjectionStats provides statistics about injection activity
type InjectionStats struct {
	TotalInjected int64            `json:"total_injected"`
	LastInjection time.Time        `json:"last_injection"`
	InjectionRate float64          `json:"injection_rate"` // injections per minute
	RecentPoints  []InjectionPoint `json:"recent_points"`
	ActiveProfile string           `json:"active_profile"`
}

// NewInjector creates a new fault injector
func NewInjector(cfg config.InjectionConfig, memory uintptr, memoryLen int64) *Injector {
	var rng *mathrand.Rand
	if cfg.RandomSeed != 0 {
		rng = mathrand.New(mathrand.NewSource(cfg.RandomSeed))
	} else {
		// Use crypto/rand for truly random seed
		var seed int64
		seedBytes := make([]byte, 8)
		if _, err := rand.Read(seedBytes); err == nil {
			seed = int64(seedBytes[0]) | int64(seedBytes[1])<<8 | int64(seedBytes[2])<<16 |
				int64(seedBytes[3])<<24 | int64(seedBytes[4])<<32 | int64(seedBytes[5])<<40 |
				int64(seedBytes[6])<<48 | int64(seedBytes[7])<<56
		} else {
			seed = time.Now().UnixNano()
		}
		rng = mathrand.New(mathrand.NewSource(seed))
	}

	return &Injector{
		config:          cfg,
		rng:             rng,
		memory:          memory,
		memoryLen:       memoryLen,
		injectionPoints: make([]InjectionPoint, 0, 1000), // circular buffer
	}
}

// Start begins the fault injection process
func (i *Injector) Start(ctx context.Context) error {
	if !i.config.Enabled {
		return nil
	}

	i.mutex.Lock()
	if i.active {
		i.mutex.Unlock()
		return fmt.Errorf("injector already active")
	}
	i.active = true
	i.mutex.Unlock()

	go i.runInjectionLoop(ctx)
	return nil
}

// Stop stops the fault injection process
func (i *Injector) Stop() {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.active = false
}

// GetStats returns current injection statistics
func (i *Injector) GetStats() InjectionStats {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	// Calculate recent injection rate (last 5 minutes)
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	recentCount := 0
	var recentPoints []InjectionPoint

	for _, point := range i.injectionPoints {
		if point.Timestamp.After(fiveMinAgo) {
			recentCount++
			if len(recentPoints) < 10 { // return up to 10 recent points
				recentPoints = append(recentPoints, point)
			}
		}
	}

	rate := float64(recentCount) * 12.0 // convert to per-hour rate

	return InjectionStats{
		TotalInjected: i.injectedCount,
		LastInjection: i.lastInjection,
		InjectionRate: rate,
		RecentPoints:  recentPoints,
		ActiveProfile: i.config.Profile,
	}
}

// runInjectionLoop is the main injection control loop
func (i *Injector) runInjectionLoop(ctx context.Context) {
	switch i.config.Profile {
	case "single":
		i.runSingleProfile(ctx)
	case "multi":
		i.runMultiProfile(ctx)
	case "burst":
		i.runBurstProfile(ctx)
	case "mixed":
		i.runMixedProfile(ctx)
	default:
		// fallback to single
		i.runSingleProfile(ctx)
	}
}

// runSingleProfile injects single bit flips at regular intervals
func (i *Injector) runSingleProfile(ctx context.Context) {
	if i.config.Rate <= 0 {
		return
	}

	interval := time.Duration(60.0/i.config.Rate) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := i.injectSingleBitFlip(); err != nil {
				fmt.Printf("Injection error: %v\n", err)
			}
		}
	}
}

// runMultiProfile injects multiple bit flips in different locations
func (i *Injector) runMultiProfile(ctx context.Context) {
	if i.config.Rate <= 0 {
		return
	}

	interval := time.Duration(60.0/i.config.Rate) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Inject 2-4 bit flips in different locations
			count := 2 + i.rng.Intn(3)
			for j := 0; j < count; j++ {
				if err := i.injectSingleBitFlip(); err != nil {
					fmt.Printf("Injection error: %v\n", err)
				}
			}
		}
	}
}

// runBurstProfile injects bursts of bit flips followed by quiet periods
func (i *Injector) runBurstProfile(ctx context.Context) {
	if i.config.Rate <= 0 || i.config.BurstSize <= 0 {
		return
	}

	burstInterval := i.config.BurstInterval.Duration
	if burstInterval <= 0 {
		burstInterval = 30 * time.Second
	}

	ticker := time.NewTicker(burstInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Inject a burst of bit flips
			for j := 0; j < i.config.BurstSize; j++ {
				if err := i.injectSingleBitFlip(); err != nil {
					fmt.Printf("Injection error: %v\n", err)
				}
				// Small delay between flips in burst
				time.Sleep(time.Duration(50+i.rng.Intn(200)) * time.Millisecond)
			}
		}
	}
}

// runMixedProfile combines different injection patterns
func (i *Injector) runMixedProfile(ctx context.Context) {
	if i.config.Rate <= 0 {
		return
	}

	baseInterval := time.Duration(60.0/i.config.Rate) * time.Second
	ticker := time.NewTicker(baseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Randomly choose injection style
			switch i.rng.Intn(3) {
			case 0: // single
				i.injectSingleBitFlip()
			case 1: // multi (2-3 flips)
				count := 2 + i.rng.Intn(2)
				for j := 0; j < count; j++ {
					i.injectSingleBitFlip()
				}
			case 2: // mini-burst (burst of 2-4)
				burstSize := 2 + i.rng.Intn(3)
				for j := 0; j < burstSize; j++ {
					i.injectSingleBitFlip()
					if j < burstSize-1 {
						time.Sleep(time.Duration(10+i.rng.Intn(100)) * time.Millisecond)
					}
				}
			}
		}
	}
}

// injectSingleBitFlip injects a single bit flip at a random location
func (i *Injector) injectSingleBitFlip() error {
	i.mutex.RLock()
	if !i.active {
		i.mutex.RUnlock()
		return nil
	}
	i.mutex.RUnlock()

	// Choose random offset
	offset := i.rng.Int63n(i.memoryLen)

	// Get pointer to byte
	bytePtr := (*byte)(unsafe.Pointer(i.memory + uintptr(offset)))

	// Read current value
	originalValue := *bytePtr

	// Choose random bit to flip
	bitPos := uint(i.rng.Intn(8))

	// Flip the bit
	newValue := originalValue ^ (1 << bitPos)
	*bytePtr = newValue

	// Record injection point
	point := InjectionPoint{
		Offset:        offset,
		OriginalValue: originalValue,
		InjectedValue: newValue,
		Timestamp:     time.Now(),
		Pattern:       "unknown", // will be filled by pattern detector
		BitPosition:   bitPos,
	}

	i.mutex.Lock()
	i.injectedCount++
	i.lastInjection = point.Timestamp

	// Add to circular buffer
	if len(i.injectionPoints) >= cap(i.injectionPoints) {
		// Remove oldest
		i.injectionPoints = i.injectionPoints[1:]
	}
	i.injectionPoints = append(i.injectionPoints, point)
	i.mutex.Unlock()

	return nil
}

// IsInjectionEnabled returns whether injection is currently active
func (i *Injector) IsInjectionEnabled() bool {
	if i == nil {
		return false
	}
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.config.Enabled && i.active
}

// GetInjectionHistory returns the full injection history
func (i *Injector) GetInjectionHistory() []InjectionPoint {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	// Return a copy to prevent races
	history := make([]InjectionPoint, len(i.injectionPoints))
	copy(history, i.injectionPoints)
	return history
}
