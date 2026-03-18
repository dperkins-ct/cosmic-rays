package memory

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Manager handles memory allocation and management for the experiment
type Manager struct {
	size       int64
	alignment  int
	lockMemory bool
	blocks     []*Block
}

// Block represents a contiguous memory block for testing
type Block struct {
	data     []byte
	size     int64
	locked   bool
	checksum uint64
}

// NewManager creates a new memory manager
func NewManager(size int64, alignment int, lockMemory bool) (*Manager, error) {
	if size <= 0 {
		return nil, fmt.Errorf("size must be positive")
	}

	if alignment <= 0 || alignment&(alignment-1) != 0 {
		return nil, fmt.Errorf("alignment must be a positive power of 2")
	}

	// Check available memory
	var memInfo runtime.MemStats
	runtime.ReadMemStats(&memInfo)

	if uint64(size) > memInfo.Sys {
		return nil, fmt.Errorf("requested size (%d) exceeds available system memory (%d)",
			size, memInfo.Sys)
	}

	return &Manager{
		size:       size,
		alignment:  alignment,
		lockMemory: lockMemory,
		blocks:     make([]*Block, 0),
	}, nil
}

// AllocateBlocks allocates the specified amount of memory in blocks
func (m *Manager) AllocateBlocks(numBlocks int) error {
	if numBlocks <= 0 {
		return fmt.Errorf("number of blocks must be positive")
	}

	blockSize := m.size / int64(numBlocks)
	if blockSize == 0 {
		return fmt.Errorf("total size too small for %d blocks", numBlocks)
	}

	for i := 0; i < numBlocks; i++ {
		block, err := m.allocateBlock(blockSize)
		if err != nil {
			// Clean up previously allocated blocks
			m.Cleanup()
			return fmt.Errorf("failed to allocate block %d: %w", i, err)
		}
		m.blocks = append(m.blocks, block)
	}

	return nil
}

// allocateBlock allocates a single memory block
func (m *Manager) allocateBlock(size int64) (*Block, error) {
	// Allocate aligned memory
	data := make([]byte, size+int64(m.alignment))

	// Align the memory
	alignedPtr := uintptr(unsafe.Pointer(&data[0]))
	alignedPtr = (alignedPtr + uintptr(m.alignment) - 1) &^ (uintptr(m.alignment) - 1)
	alignedData := unsafe.Slice((*byte)(unsafe.Pointer(alignedPtr)), size)

	block := &Block{
		data: alignedData,
		size: size,
	}

	// Lock memory if requested (macOS uses mlock)
	if m.lockMemory {
		r1, _, errno := syscall.Syscall(syscall.SYS_MLOCK,
			uintptr(unsafe.Pointer(&alignedData[0])),
			uintptr(size), 0)
		if r1 != 0 || errno != 0 {
			return nil, fmt.Errorf("failed to lock memory: %v", errno)
		}
		block.locked = true
	}

	return block, nil
}

// GetBlocks returns all allocated memory blocks
func (m *Manager) GetBlocks() []*Block {
	return m.blocks
}

// GetTotalSize returns the total allocated memory size
func (m *Manager) GetTotalSize() int64 {
	return m.size
}

// WritePattern writes a data pattern to a specific block
func (b *Block) WritePattern(pattern []byte) error {
	if len(pattern) == 0 {
		return fmt.Errorf("pattern cannot be empty")
	}

	// Repeat pattern to fill the entire block
	for i := int64(0); i < b.size; i++ {
		b.data[i] = pattern[i%int64(len(pattern))]
	}

	// Update checksum
	b.checksum = b.calculateChecksum()

	return nil
}

// VerifyPattern checks if the memory block still contains the expected pattern
func (b *Block) VerifyPattern(pattern []byte) ([]BitFlip, error) {
	if len(pattern) == 0 {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	var flips []BitFlip

	for i := int64(0); i < b.size; i++ {
		expected := pattern[i%int64(len(pattern))]
		actual := b.data[i]

		if expected != actual {
			// Analyze bit differences
			xor := expected ^ actual
			for bit := 0; bit < 8; bit++ {
				if xor&(1<<bit) != 0 {
					flips = append(flips, BitFlip{
						Offset:      i,
						BitPosition: bit,
						Expected:    (expected >> bit) & 1,
						Actual:      (actual >> bit) & 1,
					})
				}
			}
		}
	}

	return flips, nil
}

// BitFlip represents a detected bit flip
type BitFlip struct {
	Offset      int64 // Byte offset in the block
	BitPosition int   // Bit position (0-7)
	Expected    byte  // Expected bit value (0 or 1)
	Actual      byte  // Actual bit value (0 or 1)
}

// String returns a human-readable representation of the bit flip
func (bf *BitFlip) String() string {
	return fmt.Sprintf("BitFlip{Offset: %d, Bit: %d, Expected: %d, Actual: %d}",
		bf.Offset, bf.BitPosition, bf.Expected, bf.Actual)
}

// calculateChecksum calculates a simple checksum for the block
func (b *Block) calculateChecksum() uint64 {
	var checksum uint64
	for i := int64(0); i < b.size; i++ {
		checksum += uint64(b.data[i])
		checksum = (checksum << 1) | (checksum >> 63) // Rotate left
	}
	return checksum
}

// GetChecksum returns the current checksum
func (b *Block) GetChecksum() uint64 {
	return b.checksum
}

// VerifyChecksum checks if the current data matches the stored checksum
func (b *Block) VerifyChecksum() bool {
	return b.calculateChecksum() == b.checksum
}

// GetSize returns the block size in bytes
func (b *Block) GetSize() int64 {
	return b.size
}

// GetData returns the raw data slice (use with caution)
func (b *Block) GetData() []byte {
	return b.data
}

// Cleanup releases all allocated memory and unlocks if necessary
func (m *Manager) Cleanup() {
	for _, block := range m.blocks {
		if block.locked {
			// Unlock memory
			syscall.Syscall(syscall.SYS_MUNLOCK,
				uintptr(unsafe.Pointer(&block.data[0])),
				uintptr(block.size), 0)
		}
	}
	m.blocks = nil
	runtime.GC() // Force garbage collection to free memory
}

// GetStats returns memory allocation statistics
func (m *Manager) GetStats() map[string]interface{} {
	var totalSize int64
	lockedBlocks := 0

	for _, block := range m.blocks {
		totalSize += block.size
		if block.locked {
			lockedBlocks++
		}
	}

	return map[string]interface{}{
		"total_blocks":  len(m.blocks),
		"total_size":    totalSize,
		"locked_blocks": lockedBlocks,
		"alignment":     m.alignment,
		"memory_locked": m.lockMemory,
	}
}
