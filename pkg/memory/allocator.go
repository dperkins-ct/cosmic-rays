package memory

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/dperkins/cosmic-rays/internal/config"
	"github.com/dperkins/cosmic-rays/pkg/injection"
)

// Manager manages memory allocation and monitoring for the experiment
type Manager struct {
	memory       []byte
	size         int64
	useMlock     bool
	useProtect   bool
	isLocked     bool
	isProtected  bool
	allocTime    time.Time
	bitFlipCount int64
	injector     *injection.Injector
	mode         string

	// Memory protection state
	originalProtection uintptr

	// Pattern tracking
	blocks []*Block
}

// Block represents a memory block with a specific pattern for tracking
type Block struct {
	offset   int64
	size     int64
	pattern  string
	checksum uint64
	data     []byte
	locked   bool
}

// BitFlip represents a detected bit flip event
type BitFlip struct {
	Offset        int64     `json:"offset"`
	OriginalValue byte      `json:"original_value"`
	CurrentValue  byte      `json:"current_value"`
	DetectedAt    time.Time `json:"detected_at"`
	IsInjected    bool      `json:"is_injected"` // Was this injected by us?
	Confidence    float64   `json:"confidence"`  // Confidence it's a cosmic ray (0-1)
}

// MemoryStats provides detailed information about memory usage and protection
type MemoryStats struct {
	AllocatedSize    int64            `json:"allocated_size"`
	ActualSize       int64            `json:"actual_size"`
	UseMlock         bool             `json:"use_mlock"`
	UseProtection    bool             `json:"use_protection"`
	IsLocked         bool             `json:"is_locked"`
	IsProtected      bool             `json:"is_protected"`
	AllocTime        time.Time        `json:"alloc_time"`
	BitFlipsDetected int64            `json:"bit_flips_detected"`
	InjectionEnabled bool             `json:"injection_enabled"`
	Mode             string           `json:"mode"`
	SystemInfo       SystemMemoryInfo `json:"system_info"`
}

// SystemMemoryInfo provides system memory context
type SystemMemoryInfo struct {
	TotalSystemMemory uint64  `json:"total_system_memory"`
	AvailableMemory   uint64  `json:"available_memory"`
	UsedPercentage    float64 `json:"used_percentage"`
	PageSize          int     `json:"page_size"`
}

// NewManager creates a new memory manager with enhanced capabilities
func NewManager(cfg *config.Config) (*Manager, error) {
	if cfg.MemorySize == "" {
		return nil, fmt.Errorf("memory size must be specified")
	}

	// Parse memory size (handles auto-sizing)
	memorySize, err := cfg.ParseMemorySize()
	if err != nil {
		return nil, fmt.Errorf("invalid memory size: %w", err)
	}

	// Don't check system memory availability - let Go handle allocation errors naturally
	// The previous check was using incorrect Go runtime metrics

	// Allocate memory (this will fail with OOM if not enough memory)
	memory := make([]byte, memorySize)

	manager := &Manager{
		memory:     memory,
		size:       memorySize,
		useMlock:   cfg.UseLockedMemory,
		useProtect: cfg.UseProtectedMemory,
		allocTime:  time.Now(),
		mode:       cfg.Mode,
	}

	// Apply memory locking if requested
	if cfg.UseLockedMemory {
		if err := manager.lockMemory(); err != nil {
			// Don't fail, just warn - degrade gracefully
			fmt.Printf("Warning: Failed to lock memory: %v (continuing without mlock)\n", err)
		}
	}

	// Apply memory protection if requested
	if cfg.UseProtectedMemory {
		if err := manager.protectMemory(); err != nil {
			// Don't fail, just warn - degrade gracefully
			fmt.Printf("Warning: Failed to protect memory: %v (continuing without mprotect)\n", err)
		}
	}

	// Initialize injection system if enabled
	if cfg.Injection.Enabled {
		memPtr := uintptr(unsafe.Pointer(&memory[0]))
		injector := injection.NewInjector(cfg.Injection, memPtr, memorySize)
		manager.injector = injector
	}

	return manager, nil
}

// Start initializes the memory manager and starts injection if enabled
func (m *Manager) Start(ctx context.Context) error {
	if m.injector != nil {
		if err := m.injector.Start(ctx); err != nil {
			return fmt.Errorf("failed to start fault injection: %w", err)
		}
	}
	return nil
}

// Stop shuts down the memory manager
func (m *Manager) Stop() error {
	if m.injector != nil {
		m.injector.Stop()
	}

	// Remove memory protection
	if m.isProtected {
		m.unprotectMemory()
	}

	// Unlock memory
	if m.isLocked {
		m.unlockMemory()
	}

	return nil
}

// GetMemory returns access to the managed memory
func (m *Manager) GetMemory() []byte {
	return m.memory
}

// GetSize returns the size of managed memory
func (m *Manager) GetSize() int64 {
	return m.size
}

// GetStats returns comprehensive memory statistics
func (m *Manager) GetStats() MemoryStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	sysInfo := SystemMemoryInfo{
		TotalSystemMemory: memStats.Sys,
		AvailableMemory:   memStats.Sys - memStats.Alloc,
		UsedPercentage:    float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		PageSize:          syscall.Getpagesize(),
	}

	injectionEnabled := false
	if m.injector != nil {
		injectionEnabled = m.injector.IsInjectionEnabled()
	}

	return MemoryStats{
		AllocatedSize:    m.size,
		ActualSize:       int64(len(m.memory)),
		UseMlock:         m.useMlock,
		UseProtection:    m.useProtect,
		IsLocked:         m.isLocked,
		IsProtected:      m.isProtected,
		AllocTime:        m.allocTime,
		BitFlipsDetected: m.bitFlipCount,
		InjectionEnabled: injectionEnabled,
		Mode:             m.mode,
		SystemInfo:       sysInfo,
	}
}

// RecordBitFlip records a detected bit flip
func (m *Manager) RecordBitFlip(flip BitFlip) {
	m.bitFlipCount++
}

// GetInjector returns the fault injector (may be nil)
func (m *Manager) GetInjector() *injection.Injector {
	return m.injector
}

// lockMemory attempts to lock memory pages to prevent swapping
func (m *Manager) lockMemory() error {
	if len(m.memory) == 0 {
		return fmt.Errorf("no memory allocated")
	}

	// Get pointer to memory
	// Note: ptr not actually needed for syscall.Mlock in Go

	// Use mlock to lock pages in memory
	if err := syscall.Mlock(m.memory); err != nil {
		return fmt.Errorf("mlock failed: %w", err)
	}

	m.isLocked = true
	return nil
}

// unlockMemory unlocks previously locked memory pages
func (m *Manager) unlockMemory() error {
	if !m.isLocked || len(m.memory) == 0 {
		return nil
	}

	if err := syscall.Munlock(m.memory); err != nil {
		return fmt.Errorf("munlock failed: %w", err)
	}

	m.isLocked = false
	return nil
}

// GetBlocks returns the current memory blocks for pattern tracking
func (m *Manager) GetBlocks() []*Block {
	return m.blocks
}

// AllocateBlocks divides memory into blocks with different patterns
func (m *Manager) AllocateBlocks(numBlocks int) error {
	if len(m.memory) == 0 {
		return fmt.Errorf("no memory allocated")
	}

	blockSize := int64(len(m.memory)) / int64(numBlocks)
	if blockSize < 1 {
		return fmt.Errorf("memory too small to divide into %d blocks", numBlocks)
	}

	m.blocks = make([]*Block, numBlocks)
	for i := 0; i < numBlocks; i++ {
		offset := int64(i) * blockSize
		size := blockSize
		if i == numBlocks-1 {
			// Last block gets any remaining bytes
			size = int64(len(m.memory)) - offset
		}

		m.blocks[i] = &Block{
			offset:  offset,
			size:    size,
			data:    m.memory[offset : offset+size],
			pattern: "", // Will be set by pattern generator
			locked:  false,
		}
	}

	return nil
}

// protectMemory makes memory pages read-only to detect unauthorized writes
func (m *Manager) protectMemory() error {
	if len(m.memory) == 0 {
		return fmt.Errorf("no memory allocated")
	}

	// Store original protection for restoration
	m.originalProtection = syscall.PROT_READ | syscall.PROT_WRITE

	// Make pages read-only (this will be temporarily lifted during injection)
	if err := syscall.Mprotect(m.memory, syscall.PROT_READ); err != nil {
		return fmt.Errorf("mprotect failed: %w", err)
	}

	m.isProtected = true
	return nil
}

// unprotectMemory restores original memory protection
func (m *Manager) unprotectMemory() error {
	if !m.isProtected || len(m.memory) == 0 {
		return nil
	}

	if err := syscall.Mprotect(m.memory, int(m.originalProtection)); err != nil {
		return fmt.Errorf("mprotect restoration failed: %w", err)
	}

	m.isProtected = false
	return nil
}

// calculateChecksum computes a basic checksum for the block
func (b *Block) calculateChecksum() uint64 {
	var sum uint64
	for _, dataByte := range b.data {
		sum += uint64(dataByte)
	}
	return sum
}

// WritePattern writes a repeating pattern to the block's data
func (b *Block) WritePattern(pattern []byte) error {
	if len(pattern) == 0 {
		return fmt.Errorf("pattern cannot be empty")
	}

	for i := range b.data {
		b.data[i] = pattern[i%len(pattern)]
	}

	b.pattern = fmt.Sprintf("pattern_%d", len(pattern))
	b.checksum = b.calculateChecksum()
	return nil
}

// VerifyPattern checks if the current data matches the expected pattern and returns bit flips
func (b *Block) VerifyPattern(pattern []byte) ([]BitFlip, error) {
	if len(pattern) == 0 {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	var bitFlips []BitFlip
	for i, actual := range b.data {
		expected := pattern[i%len(pattern)]
		if actual != expected {
			bitFlips = append(bitFlips, BitFlip{
				Offset:        b.offset + int64(i),
				OriginalValue: expected,
				CurrentValue:  actual,
				DetectedAt:    time.Now(),
				IsInjected:    false, // Will be set by injector tracking
				Confidence:    0.5,   // Default confidence
			})
		}
	}

	return bitFlips, nil
}

// RepairFlips restores the expected byte value for each detected bit flip,
// so subsequent scans do not re-count the same corruption as a new event.
func (b *Block) RepairFlips(flips []BitFlip, pattern []byte) {
	if len(pattern) == 0 {
		return
	}
	for _, flip := range flips {
		i := int(flip.Offset - b.offset)
		if i >= 0 && i < len(b.data) {
			b.data[i] = pattern[i%len(pattern)]
		}
	}
}

// GetSize returns the size of the block in bytes
func (b *Block) GetSize() int64 {
	return b.size
}

// Cleanup releases all resources and unlocks memory
func (m *Manager) Cleanup() {
	for _, block := range m.blocks {
		// Unlock memory if it was locked
		if block.locked {
			syscall.Munlock(block.data)
		}
	}
	m.blocks = nil
}
