package detector

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/dperkins/cosmic-rays/internal/config"
	"github.com/dperkins/cosmic-rays/pkg/injection"
	"github.com/dperkins/cosmic-rays/pkg/memory"
)

// Event represents a memory corruption event (neutral detection)
type Event struct {
	Type        EventType              `json:"type"`
	Offset      int64                  `json:"offset"`
	OldValue    byte                   `json:"old_value"`
	NewValue    byte                   `json:"new_value"`
	Timestamp   time.Time              `json:"timestamp"`
	PatternType string                 `json:"pattern_type"`
	BitPosition int                    `json:"bit_position"`
	Statistics  map[string]interface{} `json:"statistics,omitempty"`

	// Additional fields for scanning events
	BlockIndex int              `json:"block_index,omitempty"`
	Pattern    string           `json:"pattern,omitempty"`
	BitFlips   []memory.BitFlip `json:"bit_flips,omitempty"`
}

// EventType constants for different scanner events
type EventType string

const (
	EventStarted      EventType = "started"
	EventStopped      EventType = "stopped"
	EventBitFlip      EventType = "bit_flip"
	EventError        EventType = "error"
	EventScanComplete EventType = "scan_complete"
)

// PatternGenerator generates memory patterns for testing
type PatternGenerator struct {
	patterns map[string]func([]byte)
}

// NewPatternGenerator creates a new pattern generator
func NewPatternGenerator() *PatternGenerator {
	return &PatternGenerator{
		patterns: map[string]func([]byte){
			"alternating": generateAlternating,
			"checksum":    generateChecksum,
			"random":      generateRandom,
			"known":       generateKnown,
		},
	}
}

// Generate creates a pattern of the specified type and size
func (pg *PatternGenerator) Generate(patternType string, size int) ([]byte, error) {
	generator, exists := pg.patterns[patternType]
	if !exists {
		return nil, fmt.Errorf("unknown pattern type: %s", patternType)
	}

	data := make([]byte, size)
	generator(data)
	return data, nil
}

// Pattern generation functions
func generateAlternating(data []byte) {
	for i := range data {
		if i%2 == 0 {
			data[i] = 0xAA
		} else {
			data[i] = 0x55
		}
	}
}

func generateChecksum(data []byte) {
	for i := range data {
		data[i] = byte(i % 256)
	}
}

func generateRandom(data []byte) {
	// Simple pseudo-random pattern
	seed := uint32(42)
	for i := range data {
		seed = seed*1103515245 + 12345
		data[i] = byte(seed >> 16)
	}
}

func generateKnown(data []byte) {
	pattern := []byte{0x42, 0xEF, 0xBE, 0xAD}
	for i := range data {
		data[i] = pattern[i%len(pattern)]
	}
}

// Attribution represents heuristic analysis about the source of corruption
type Attribution struct {
	Event               Event                  `json:"event"`
	IsInjected          bool                   `json:"is_injected"`           // Known to be injected by us
	CosmicRayLikelihood float64                `json:"cosmic_ray_likelihood"` // 0-1 heuristic score
	Factors             map[string]interface{} `json:"factors"`               // Attribution factors
	Confidence          string                 `json:"confidence"`            // "low", "medium", "high"
}

// InternalStats holds detailed scanning statistics with thread-safe access
type InternalStats struct {
	mu               sync.RWMutex
	TotalScans       int64
	TotalBitFlips    int64
	SingleBitFlips   int64
	MultipleBitFlips int64
	BytesScanned     int64
	LastScanTime     time.Time
	ScanDuration     time.Duration
}

// Scanner detects memory corruption events
type Scanner struct {
	config         *config.Config
	memoryManager  *memory.Manager
	patterns       []string // List of pattern names to use
	active         bool
	mutex          sync.RWMutex
	eventListeners []EventListener

	// Statistics
	scanCount     int64
	eventCount    int64
	lastScan      time.Time
	scanStartTime time.Time
	stats         *InternalStats

	// Heuristic attribution (when enabled)
	attributor *HeuristicAttributor

	// Pattern generation
	generator *PatternGenerator

	// Runtime state
	mu           sync.RWMutex
	isRunning    bool
	listeners    []EventListener
	scanInterval time.Duration
}

// EventListener interface for handling detected events
type EventListener interface {
	OnEvent(event Event)
	OnAttributedEvent(attribution Attribution)
}

// HeuristicAttributor provides cosmic ray attribution heuristics
type HeuristicAttributor struct {
	config               *config.Config
	injector             *injection.Injector
	recentInjections     []injection.InjectionPoint
	injectionHistorySize int
}

// ScanStats provides scanner statistics
type ScanStats struct {
	ScanCount          int64         `json:"scan_count"`
	EventCount         int64         `json:"event_count"`
	LastScan           time.Time     `json:"last_scan"`
	ScansPerMinute     float64       `json:"scans_per_minute"`
	EventsPerMinute    float64       `json:"events_per_minute"`
	AttributionEnabled bool          `json:"attribution_enabled"`
	RunningTime        time.Duration `json:"running_time"`
}

// NewScanner creates a new memory corruption scanner
func NewScanner(cfg *config.Config, memMgr *memory.Manager) *Scanner {
	scanner := &Scanner{
		config:         cfg,
		memoryManager:  memMgr,
		patterns:       cfg.PatternsToUse, // Use patterns from config
		active:         false,
		eventListeners: make([]EventListener, 0),
		scanStartTime:  time.Now(),
		generator:      NewPatternGenerator(), stats: &InternalStats{},
		listeners:    make([]EventListener, 0),
		scanInterval: cfg.ScanInterval.Duration}

	// Initialize heuristic attributor if enabled
	if cfg.EnableAttribution {
		scanner.attributor = &HeuristicAttributor{
			config:               cfg,
			injector:             memMgr.GetInjector(),
			recentInjections:     make([]injection.InjectionPoint, 0, 100),
			injectionHistorySize: 100,
		}
	}

	// Initialize memory patterns for each pattern type
	scanner.initializePatterns()

	return scanner
}

// AddEventListener adds a listener for detection events
func (s *Scanner) AddEventListener(listener EventListener) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.eventListeners = append(s.eventListeners, listener)
}

// Start begins the scanning process
func (s *Scanner) Start(ctx context.Context) error {
	s.mutex.Lock()
	if s.active {
		s.mutex.Unlock()
		return fmt.Errorf("scanner already active")
	}
	s.active = true
	s.scanStartTime = time.Now()
	s.mutex.Unlock()

	// Initialize memory with patterns before starting
	if err := s.initializeMemory(); err != nil {
		s.mutex.Lock()
		s.active = false
		s.mutex.Unlock()
		return fmt.Errorf("failed to initialize memory: %w", err)
	}

	// Notify listeners that scanning started
	s.notifyListeners(Event{
		Type:      EventStarted,
		Timestamp: time.Now(),
	})

	go s.scanLoop(ctx)
	return nil
}

// Stop stops the scanning process
func (s *Scanner) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.active = false
}

// GetStats returns current scanning statistics
func (s *Scanner) GetStats() ScanStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	runningTime := time.Since(s.scanStartTime)
	minutesRunning := runningTime.Minutes()

	scansPerMin := float64(0)
	eventsPerMin := float64(0)
	if minutesRunning > 0 {
		scansPerMin = float64(s.scanCount) / minutesRunning
		eventsPerMin = float64(s.eventCount) / minutesRunning
	}

	return ScanStats{
		ScanCount:          s.scanCount,
		EventCount:         s.eventCount,
		LastScan:           s.lastScan,
		ScansPerMinute:     scansPerMin,
		EventsPerMinute:    eventsPerMin,
		AttributionEnabled: s.config.EnableAttribution,
		RunningTime:        runningTime,
	}
}

// scanLoop is the main scanning loop
func (s *Scanner) scanLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.ScanInterval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.notifyListeners(Event{
				Type:      EventStopped,
				Timestamp: time.Now(),
			})
			return
		case <-ticker.C:
			s.mutex.RLock()
			if !s.active {
				s.mutex.RUnlock()
				s.notifyListeners(Event{
					Type:      EventStopped,
					Timestamp: time.Now(),
				})
				return
			}
			s.mutex.RUnlock()

			if err := s.performScan(); err != nil {
				log.Printf("Scan error: %v", err)
				s.notifyListeners(Event{
					Type:       EventError,
					Timestamp:  time.Now(),
					Statistics: map[string]interface{}{"error": err.Error()},
				})
			}
		}
	}
}

// scanFull performs a complete scan of all memory
func (s *Scanner) scanFull(memory []byte) {
	for _, patternName := range s.patterns {
		pattern, err := s.generator.Generate(patternName, len(memory))
		if err != nil {
			log.Printf("Failed to generate pattern %s: %v", patternName, err)
			continue
		}
		s.scanPattern(memory, patternName, pattern)
	}
}

// scanSampled performs sampling-based scanning
func (s *Scanner) scanSampled(memory []byte) {
	sampleSize := int(float64(len(memory)) * s.config.SampleRate)
	if sampleSize < 1024 {
		sampleSize = 1024 // minimum sample size
	}

	// TODO: implement smarter sampling strategy
	for _, patternName := range s.patterns {
		pattern, err := s.generator.Generate(patternName, sampleSize)
		if err != nil {
			log.Printf("Failed to generate pattern %s: %v", patternName, err)
			continue
		}
		s.scanPattern(memory[:sampleSize], patternName, pattern)
	}
}

// scanAdaptive adjusts scan strategy based on system load and event rate
func (s *Scanner) scanAdaptive(memory []byte) {
	// For now, default to full scan - adaptive logic can be added later
	s.scanFull(memory)
}

// scanPattern scans memory for changes from expected pattern
func (s *Scanner) scanPattern(memory []byte, patternName string, expectedPattern []byte) {
	patternLen := len(expectedPattern)
	if patternLen == 0 {
		return
	}

	for i := 0; i < len(memory); i += patternLen {
		end := i + patternLen
		if end > len(memory) {
			end = len(memory)
		}

		for j := i; j < end && j-i < patternLen; j++ {
			expected := expectedPattern[j-i]
			actual := memory[j]

			if expected != actual {
				// Found a difference - create event
				event := Event{
					Offset:      int64(j),
					OldValue:    expected,
					NewValue:    actual,
					Timestamp:   time.Now(),
					PatternType: patternName,
					BitPosition: s.findBitDifference(expected, actual),
				}

				s.handleDetectedEvent(event)
			}
		}
	}
}

// findBitDifference finds which bit position differs between two bytes
func (s *Scanner) findBitDifference(old, new byte) int {
	diff := old ^ new
	position := 0
	for diff > 1 {
		diff >>= 1
		position++
	}
	return position
}

// handleDetectedEvent processes a detected corruption event
func (s *Scanner) handleDetectedEvent(event Event) {
	s.mutex.Lock()
	s.eventCount++
	s.mutex.Unlock()

	// First, notify listeners of the raw detection event
	s.notifyEventListeners(event)

	// If attribution is enabled, perform heuristic analysis
	if s.attributor != nil {
		attribution := s.attributor.AttributeEvent(event)
		s.notifyAttributionListeners(attribution)
	}

	// Record with memory manager
	bitFlip := memory.BitFlip{
		Offset:        event.Offset,
		OriginalValue: event.OldValue,
		CurrentValue:  event.NewValue,
		DetectedAt:    event.Timestamp,
		IsInjected:    false, // will be updated by attribution
		Confidence:    0.0,   // will be updated by attribution
	}
	s.memoryManager.RecordBitFlip(bitFlip)
}

// notifyEventListeners notifies all listeners of a detection event
func (s *Scanner) notifyEventListeners(event Event) {
	s.mutex.RLock()
	listeners := make([]EventListener, len(s.eventListeners))
	copy(listeners, s.eventListeners)
	s.mutex.RUnlock()

	for _, listener := range listeners {
		go func(l EventListener) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Event listener panic: %v", r)
				}
			}()
			l.OnEvent(event)
		}(listener)
	}
}

// notifyAttributionListeners notifies all listeners of an attribution result
func (s *Scanner) notifyAttributionListeners(attribution Attribution) {
	s.mutex.RLock()
	listeners := make([]EventListener, len(s.eventListeners))
	copy(listeners, s.eventListeners)
	s.mutex.RUnlock()

	for _, listener := range listeners {
		go func(l EventListener) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Attribution listener panic: %v", r)
				}
			}()
			l.OnAttributedEvent(attribution)
		}(listener)
	}
}

// initializePatterns creates expected memory patterns for each pattern type
func (s *Scanner) initializePatterns() {
	memory := s.memoryManager.GetMemory()

	// Use the first pattern to initialize memory
	if len(s.patterns) > 0 {
		pattern, err := s.generator.Generate(s.patterns[0], len(memory))
		if err != nil {
			log.Printf("Failed to generate initial pattern: %v", err)
			return
		}
		copy(memory, pattern)
	}
}

// generatePattern creates a memory pattern of the specified type
func (s *Scanner) generatePattern(patternType string, size int) []byte {
	pattern := make([]byte, size)

	switch patternType {
	case "alternating":
		for i := 0; i < size; i++ {
			if i%2 == 0 {
				pattern[i] = 0xAA // 10101010
			} else {
				pattern[i] = 0x55 // 01010101
			}
		}
	case "checksum":
		// Simple checksum pattern
		for i := 0; i < size; i++ {
			pattern[i] = byte(i % 256)
		}
	case "random":
		// Pseudo-random but deterministic
		for i := 0; i < size; i++ {
			pattern[i] = byte((i*7 + 13) % 256)
		}
	case "known":
		// Known cosmic ray test pattern
		for i := 0; i < size; i++ {
			pattern[i] = 0x00 // all zeros, sensitive to cosmic rays
		}
	default:
		// Default to alternating
		for i := 0; i < size; i++ {
			pattern[i] = 0xAA
		}
	}

	return pattern
}

// AttributeEvent attempts to determine if an event was caused by cosmic rays
func (a *HeuristicAttributor) AttributeEvent(event Event) Attribution {
	attribution := Attribution{
		Event:   event,
		Factors: make(map[string]interface{}),
	}

	// Check if this was a known injection
	isInjected := a.checkIfInjected(event)
	attribution.IsInjected = isInjected
	attribution.Factors["is_injected"] = isInjected

	if isInjected {
		// Known injection - definitely not cosmic ray
		attribution.CosmicRayLikelihood = 0.0
		attribution.Confidence = "high"
		attribution.Factors["source"] = "fault_injection"
	} else {
		// Perform heuristic analysis
		likelihood := a.calculateCosmicRayLikelihood(event)
		attribution.CosmicRayLikelihood = likelihood

		if likelihood > 0.8 {
			attribution.Confidence = "high"
		} else if likelihood > 0.5 {
			attribution.Confidence = "medium"
		} else {
			attribution.Confidence = "low"
		}
	}

	return attribution
}

// checkIfInjected determines if an event matches known injection points
func (a *HeuristicAttributor) checkIfInjected(event Event) bool {
	if a.injector == nil || !a.injector.IsInjectionEnabled() {
		return false
	}

	// Get recent injection history
	history := a.injector.GetInjectionHistory()

	// Check if this event matches an injection within reasonable time window
	timeWindow := 10 * time.Second // reasonable detection delay
	for _, injection := range history {
		if math.Abs(float64(injection.Offset-event.Offset)) < 1.0 &&
			event.Timestamp.Sub(injection.Timestamp) < timeWindow &&
			event.Timestamp.After(injection.Timestamp) {
			return true
		}
	}

	return false
}

// calculateCosmicRayLikelihood provides a heuristic score for cosmic ray attribution
func (a *HeuristicAttributor) calculateCosmicRayLikelihood(event Event) float64 {
	likelihood := 0.5 // baseline likelihood

	// Single bit flips are more likely to be cosmic rays
	hamming := a.calculateHammingDistance(event.OldValue, event.NewValue)
	if hamming == 1 {
		likelihood += 0.3
	} else {
		likelihood -= 0.2 // multi-bit flips less likely to be cosmic rays
	}

	// Factor in altitude if location data is available
	if a.config.Location.Enabled && a.config.Location.Altitude > 1000 {
		// Higher altitude = more cosmic rays
		altitudeFactor := math.Min(a.config.Location.Altitude/10000.0, 0.2)
		likelihood += altitudeFactor
	}

	// Ensure likelihood stays in [0,1] range
	if likelihood < 0 {
		likelihood = 0
	}
	if likelihood > 1 {
		likelihood = 1
	}

	return likelihood
}

// calculateHammingDistance calculates the Hamming distance between two bytes
func (a *HeuristicAttributor) calculateHammingDistance(a1, b1 byte) int {
	diff := a1 ^ b1
	count := 0
	for diff != 0 {
		count += int(diff & 1)
		diff >>= 1
	}
	return count
}

// initializeMemory sets up memory blocks with patterns
func (s *Scanner) initializeMemory() error {
	blocks := s.memoryManager.GetBlocks()
	if len(blocks) == 0 {
		// Allocate blocks if none exist
		numPatterns := len(s.patterns)
		if numPatterns == 0 {
			numPatterns = 4 // Default to 4 blocks
		}

		if err := s.memoryManager.AllocateBlocks(numPatterns); err != nil {
			return err
		}
		blocks = s.memoryManager.GetBlocks()
	}

	// Initialize each block with a different pattern
	for i, block := range blocks {
		patternType := s.patterns[i%len(s.patterns)]
		pattern, err := s.generator.Generate(patternType, 1024) // Use 1KB pattern that repeats
		if err != nil {
			return fmt.Errorf("failed to generate pattern %s: %w", patternType, err)
		}

		if err := block.WritePattern(pattern); err != nil {
			return fmt.Errorf("failed to write pattern to block %d: %w", i, err)
		}
	}

	return nil
}

// performScan scans all memory blocks for bit flips
func (s *Scanner) performScan() error {
	// Update scan counter
	s.mutex.Lock()
	s.scanCount++
	s.lastScan = time.Now()
	s.mutex.Unlock()

	startTime := time.Now()
	blocks := s.memoryManager.GetBlocks()

	var totalBitFlips []memory.BitFlip
	var bytesScanned int64

	for blockIndex, block := range blocks {
		patternType := s.patterns[blockIndex%len(s.patterns)]
		pattern, err := s.generator.Generate(patternType, 1024)
		if err != nil {
			return fmt.Errorf("failed to regenerate pattern for validation: %w", err)
		}

		bitFlips, err := block.VerifyPattern(pattern)
		if err != nil {
			return fmt.Errorf("failed to verify pattern in block %d: %w", blockIndex, err)
		}

		bytesScanned += block.GetSize()

		if len(bitFlips) > 0 {
			totalBitFlips = append(totalBitFlips, bitFlips...)

			// Notify listeners of bit flips
			for _, flip := range bitFlips {
				s.notifyListeners(Event{
					Type:        EventBitFlip,
					Offset:      flip.Offset,
					OldValue:    flip.OriginalValue,
					NewValue:    flip.CurrentValue,
					Timestamp:   flip.DetectedAt,
					PatternType: patternType,
				})
			}
		}
	}

	// Update statistics
	s.updateStats(totalBitFlips, bytesScanned, time.Since(startTime))

	// Notify listeners of scan completion
	// Convert ScanStats to map for Event.Statistics field
	scanStats := s.GetStats()
	statsMap := map[string]interface{}{
		"scan_count":          scanStats.ScanCount,
		"event_count":         scanStats.EventCount,
		"last_scan":           scanStats.LastScan,
		"scans_per_minute":    scanStats.ScansPerMinute,
		"events_per_minute":   scanStats.EventsPerMinute,
		"attribution_enabled": scanStats.AttributionEnabled,
		"running_time":        scanStats.RunningTime,
	}
	s.notifyListeners(Event{
		Type:       EventScanComplete,
		Timestamp:  time.Now(),
		Statistics: statsMap,
	})

	return nil
}

// updateStats updates scanning statistics
func (s *Scanner) updateStats(bitFlips []memory.BitFlip, bytesScanned int64, duration time.Duration) {
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()

	s.stats.TotalScans++
	s.stats.TotalBitFlips += int64(len(bitFlips))
	s.stats.BytesScanned += bytesScanned
	s.stats.LastScanTime = time.Now()
	s.stats.ScanDuration = duration

	// Classify bit flips
	for range bitFlips {
		s.stats.SingleBitFlips++ // For now, treat all as single bit flips
		// TODO: Add logic to detect multiple bit flips in same byte
	}
}

// notifyListeners sends an event to all registered listeners
func (s *Scanner) notifyListeners(event Event) {
	s.mu.RLock()
	listeners := make([]EventListener, len(s.eventListeners))
	copy(listeners, s.eventListeners)
	s.mu.RUnlock()

	for _, listener := range listeners {
		go func(l EventListener, e Event) {
			defer func() {
				if r := recover(); r != nil {
					// Log error but continue
					fmt.Printf("Error in event listener: %v\n", r)
				}
			}()
			l.OnEvent(e)
		}(listener, event)
	}
}

// IsRunning returns whether the scanner is currently running
func (s *Scanner) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// Close stops the scanner and cleans up
func (s *Scanner) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isRunning = false
	s.eventListeners = nil
}

// SetScanInterval changes the scanning interval (only when not running)
func (s *Scanner) SetScanInterval(interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("cannot change scan interval while scanner is running")
	}

	if interval <= 0 {
		return fmt.Errorf("scan interval must be positive")
	}

	s.scanInterval = interval
	return nil
}

// GetScanInterval returns the current scan interval
func (s *Scanner) GetScanInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanInterval
}
