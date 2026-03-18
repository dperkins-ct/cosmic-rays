package detector

import (
	"fmt"
	"sync"
	"time"

	"github.com/dperkins/cosmic-rays/pkg/memory"
	"github.com/dperkins/cosmic-rays/pkg/patterns"
)

// Scanner continuously monitors memory blocks for bit flips
type Scanner struct {
	memoryManager *memory.Manager
	generator     *patterns.Generator
	scanInterval  time.Duration
	patterns      []patterns.PatternType
	isRunning     bool
	mu            sync.RWMutex
	listeners     []EventListener
	stats         ScanStats
}

// ScanStats tracks comprehensive scanning statistics with temporal analysis
type ScanStats struct {
	TotalScans          int64         `json:"total_scans"`
	TotalBitFlips       int64         `json:"total_bit_flips"`
	SingleBitFlips      int64         `json:"single_bit_flips"`
	MultipleBitFlips    int64         `json:"multiple_bit_flips"`
	ECCCorrectableFlips int64         `json:"ecc_correctable_flips"`
	CosmicRayCandidate  int64         `json:"cosmic_ray_candidates"`
	BurstEvents         int64         `json:"burst_events"`
	LastScanTime        time.Time     `json:"last_scan_time"`
	ScanDuration        time.Duration `json:"scan_duration"`
	BytesScanned        int64         `json:"bytes_scanned"`
	BitFlipRate         float64       `json:"bit_flip_rate"`
	CosmicRayRate       float64       `json:"cosmic_ray_rate"`
	IsRunning           bool          `json:"is_running"`
	mu                  sync.RWMutex
}

// Event represents a memory event (bit flip, scan completion, etc.)
type Event struct {
	Type           EventType            `json:"type"`
	Timestamp      time.Time            `json:"timestamp"`
	BlockIndex     int                  `json:"block_index,omitempty"`
	Pattern        patterns.PatternType `json:"pattern,omitempty"`
	BitFlips       []memory.BitFlip     `json:"bit_flips,omitempty"`
	Statistics     interface{}          `json:"statistics,omitempty"`
	CosmicRayScore float64              `json:"cosmic_ray_score,omitempty"`
}

// EventType represents the type of event
type EventType string

const (
	EventBitFlip       EventType = "bit_flip"
	EventScanComplete  EventType = "scan_complete"
	EventCosmicRay     EventType = "cosmic_ray_candidate"
	EventBurst         EventType = "burst_detected"
	EventECCCorrection EventType = "ecc_correction"
	EventError         EventType = "error"
	EventStarted       EventType = "started"
	EventStopped       EventType = "stopped"
)

// EventListener receives events from the scanner
type EventListener interface {
	OnEvent(event Event)
}

// NewScanner creates a new memory scanner
func NewScanner(memMgr *memory.Manager, scanInterval time.Duration, patternNames []string) (*Scanner, error) {
	if memMgr == nil {
		return nil, fmt.Errorf("memory manager cannot be nil")
	}

	if scanInterval <= 0 {
		return nil, fmt.Errorf("scan interval must be positive")
	}

	// Convert pattern names to types
	patternTypes := make([]patterns.PatternType, len(patternNames))
	for i, name := range patternNames {
		patternTypes[i] = patterns.PatternType(name)
	}

	return &Scanner{
		memoryManager: memMgr,
		generator:     patterns.NewGenerator(),
		scanInterval:  scanInterval,
		patterns:      patternTypes,
		listeners:     make([]EventListener, 0),
		stats:         ScanStats{},
	}, nil
}

// AddListener adds an event listener
func (s *Scanner) AddListener(listener EventListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// Start begins the scanning process
func (s *Scanner) Start(stopChan <-chan struct{}) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("scanner is already running")
	}
	s.isRunning = true
	s.mu.Unlock()

	// Initialize memory with patterns
	if err := s.initializeMemory(); err != nil {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
		return fmt.Errorf("failed to initialize memory: %w", err)
	}

	// Notify listeners that scanning started
	s.notifyListeners(Event{
		Type:      EventStarted,
		Timestamp: time.Now(),
	})

	// Start scanning loop
	ticker := time.NewTicker(s.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			s.mu.Lock()
			s.isRunning = false
			s.mu.Unlock()

			s.notifyListeners(Event{
				Type:      EventStopped,
				Timestamp: time.Now(),
			})
			return nil

		case <-ticker.C:
			if err := s.performScan(); err != nil {
				s.notifyListeners(Event{
					Type:       EventError,
					Timestamp:  time.Now(),
					Statistics: map[string]interface{}{"error": err.Error()},
				})
			}
		}
	}
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
			s.notifyListeners(Event{
				Type:       EventBitFlip,
				Timestamp:  time.Now(),
				BlockIndex: blockIndex,
				Pattern:    patternType,
				BitFlips:   bitFlips,
			})

			// Re-initialize the corrupted block
			if err := block.WritePattern(pattern); err != nil {
				return fmt.Errorf("failed to reinitialize block %d after corruption: %w", blockIndex, err)
			}
		}
	}

	// Update statistics
	s.updateStats(totalBitFlips, bytesScanned, time.Since(startTime))

	// Notify listeners of scan completion
	s.notifyListeners(Event{
		Type:       EventScanComplete,
		Timestamp:  time.Now(),
		Statistics: s.GetStats(),
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
	listeners := make([]EventListener, len(s.listeners))
	copy(listeners, s.listeners)
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

// GetStats returns current scanning statistics
func (s *Scanner) GetStats() map[string]interface{} {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	var bitFlipRate float64
	if s.stats.BytesScanned > 0 {
		bitFlipRate = float64(s.stats.TotalBitFlips) / float64(s.stats.BytesScanned*8)
	}

	return map[string]interface{}{
		"total_scans":        s.stats.TotalScans,
		"total_bit_flips":    s.stats.TotalBitFlips,
		"single_bit_flips":   s.stats.SingleBitFlips,
		"multiple_bit_flips": s.stats.MultipleBitFlips,
		"last_scan_time":     s.stats.LastScanTime,
		"scan_duration":      s.stats.ScanDuration,
		"bytes_scanned":      s.stats.BytesScanned,
		"bit_flip_rate":      bitFlipRate,
		"is_running":         s.isRunning,
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
	s.listeners = nil
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
