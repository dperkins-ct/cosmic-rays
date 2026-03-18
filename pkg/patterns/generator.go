package patterns

import (
	"crypto/rand"
	"fmt"
	"math"
	"time"
)

// Generator creates various data patterns for memory testing
type Generator struct {
	seed int64
}

// NewGenerator creates a new pattern generator
func NewGenerator() *Generator {
	return &Generator{
		seed: time.Now().UnixNano(),
	}
}

// PatternType represents the type of pattern to generate
type PatternType string

const (
	PatternAlternating PatternType = "alternating"
	PatternChecksum    PatternType = "checksum"
	PatternRandom      PatternType = "random"
	PatternKnown       PatternType = "known"
)

// Generate creates a pattern of the specified type and size
func (g *Generator) Generate(patternType PatternType, size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("size must be positive")
	}

	switch patternType {
	case PatternAlternating:
		return g.generateAlternating(size), nil
	case PatternChecksum:
		return g.generateChecksum(size), nil
	case PatternRandom:
		return g.generateRandom(size)
	case PatternKnown:
		return g.generateKnown(size), nil
	default:
		return nil, fmt.Errorf("unknown pattern type: %s", patternType)
	}
}

// generateAlternating creates alternating bit patterns (0x55, 0xAA, etc.)
func (g *Generator) generateAlternating(size int) []byte {
	pattern := make([]byte, size)

	// Use different alternating patterns
	patterns := []byte{0x55, 0xAA, 0x33, 0xCC, 0x0F, 0xF0}

	for i := 0; i < size; i++ {
		// Change pattern every 1024 bytes to detect regional errors
		patternIndex := (i / 1024) % len(patterns)
		pattern[i] = patterns[patternIndex]
	}

	return pattern
}

// generateChecksum creates a pattern where every 8th byte is a checksum of the previous 7
func (g *Generator) generateChecksum(size int) []byte {
	pattern := make([]byte, size)

	for i := 0; i < size; i++ {
		if i%8 == 7 {
			// Calculate checksum of previous 7 bytes
			sum := byte(0)
			for j := i - 7; j < i; j++ {
				if j >= 0 {
					sum ^= pattern[j]
				}
			}
			pattern[i] = sum
		} else {
			// Use a predictable sequence based on position
			pattern[i] = byte((int64(i)*37 + g.seed) % 256)
		}
	}

	return pattern
}

// generateRandom creates cryptographically strong random data
func (g *Generator) generateRandom(size int) ([]byte, error) {
	pattern := make([]byte, size)

	if _, err := rand.Read(pattern); err != nil {
		return nil, fmt.Errorf("failed to generate random data: %w", err)
	}

	return pattern, nil
}

// generateKnown creates a known pattern with specific sequences that are easy to validate
func (g *Generator) generateKnown(size int) []byte {
	pattern := make([]byte, size)

	// Use a combination of known sequences
	sequences := [][]byte{
		{0x00, 0xFF, 0x00, 0xFF},                         // Alternating bytes
		{0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80}, // Powers of 2
		{0xDE, 0xAD, 0xBE, 0xEF},                         // DEADBEEF
		{0xCA, 0xFE, 0xBA, 0xBE},                         // CAFEBABE
	}

	for i := 0; i < size; i++ {
		seqIndex := (i / 64) % len(sequences)
		sequence := sequences[seqIndex]
		pattern[i] = sequence[i%len(sequence)]
	}

	return pattern
}

// ValidatePattern checks if a given data buffer matches the expected pattern
func (g *Generator) ValidatePattern(data []byte, patternType PatternType) (ValidationResult, error) {
	expectedPattern, err := g.Generate(patternType, len(data))
	if err != nil {
		return ValidationResult{}, err
	}

	result := ValidationResult{
		PatternType:  patternType,
		TotalBytes:   len(data),
		CorruptBytes: 0,
		BitFlips:     make([]BitFlipDetail, 0),
		IsValid:      true,
	}

	for i := 0; i < len(data); i++ {
		if data[i] != expectedPattern[i] {
			result.CorruptBytes++
			result.IsValid = false

			// Analyze bit differences
			xor := data[i] ^ expectedPattern[i]
			for bit := 0; bit < 8; bit++ {
				if xor&(1<<bit) != 0 {
					result.BitFlips = append(result.BitFlips, BitFlipDetail{
						ByteOffset:  i,
						BitPosition: bit,
						ExpectedBit: (expectedPattern[i] >> bit) & 1,
						ActualBit:   (data[i] >> bit) & 1,
						FlipType:    classifyBitFlip(expectedPattern[i], data[i]),
					})
				}
			}
		}
	}

	result.CorruptionRate = float64(result.CorruptBytes) / float64(result.TotalBytes)
	result.BitFlipRate = float64(len(result.BitFlips)) / float64(result.TotalBytes*8)

	return result, nil
}

// ValidationResult contains the results of pattern validation
type ValidationResult struct {
	PatternType    PatternType     `json:"pattern_type"`
	TotalBytes     int             `json:"total_bytes"`
	CorruptBytes   int             `json:"corrupt_bytes"`
	CorruptionRate float64         `json:"corruption_rate"`
	BitFlips       []BitFlipDetail `json:"bit_flips"`
	BitFlipRate    float64         `json:"bit_flip_rate"`
	IsValid        bool            `json:"is_valid"`
}

// BitFlipDetail provides detailed information about a bit flip
type BitFlipDetail struct {
	ByteOffset  int      `json:"byte_offset"`
	BitPosition int      `json:"bit_position"`
	ExpectedBit byte     `json:"expected_bit"`
	ActualBit   byte     `json:"actual_bit"`
	FlipType    FlipType `json:"flip_type"`
}

// FlipType classifies the type of bit flip
type FlipType string

const (
	FlipSingle   FlipType = "single"   // Single bit flip
	FlipMultiple FlipType = "multiple" // Multiple bit flips in same byte
	FlipBurst    FlipType = "burst"    // Burst error pattern
)

// classifyBitFlip determines the type of bit flip
func classifyBitFlip(expected, actual byte) FlipType {
	xor := expected ^ actual

	// Count number of different bits
	bitCount := 0
	for i := 0; i < 8; i++ {
		if xor&(1<<i) != 0 {
			bitCount++
		}
	}

	if bitCount == 1 {
		return FlipSingle
	} else if bitCount > 1 {
		return FlipMultiple
	}

	return FlipSingle // Fallback
}

// GetStatistics returns statistical information about the validation results
func (vr *ValidationResult) GetStatistics() map[string]interface{} {
	singleFlips := 0
	multipleFlips := 0

	for _, flip := range vr.BitFlips {
		switch flip.FlipType {
		case FlipSingle:
			singleFlips++
		case FlipMultiple:
			multipleFlips++
		}
	}

	return map[string]interface{}{
		"total_bytes":     vr.TotalBytes,
		"corrupt_bytes":   vr.CorruptBytes,
		"corruption_rate": vr.CorruptionRate,
		"total_bit_flips": len(vr.BitFlips),
		"bit_flip_rate":   vr.BitFlipRate,
		"single_flips":    singleFlips,
		"multiple_flips":  multipleFlips,
		"is_valid":        vr.IsValid,
	}
}

// PatternInfo provides information about a pattern
type PatternInfo struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Entropy       float64 `json:"entropy"`
	Detectability float64 `json:"detectability"`
}

// GetPatternInfo returns information about different pattern types
func GetPatternInfo(patternType PatternType) PatternInfo {
	switch patternType {
	case PatternAlternating:
		return PatternInfo{
			Name:          "Alternating",
			Description:   "Alternating bit patterns (0x55, 0xAA, etc.) that are easy to detect when corrupted",
			Entropy:       1.0, // Low entropy, very predictable
			Detectability: 1.0, // High detectability
		}
	case PatternChecksum:
		return PatternInfo{
			Name:          "Checksum",
			Description:   "Pattern with embedded checksums every 8 bytes for error detection",
			Entropy:       4.0, // Medium entropy
			Detectability: 0.9, // High detectability with self-validation
		}
	case PatternRandom:
		return PatternInfo{
			Name:          "Random",
			Description:   "Cryptographically strong random data",
			Entropy:       8.0, // Maximum entropy
			Detectability: 0.7, // Lower detectability, requires baseline comparison
		}
	case PatternKnown:
		return PatternInfo{
			Name:          "Known Sequences",
			Description:   "Well-known bit patterns (DEADBEEF, powers of 2, etc.)",
			Entropy:       2.0,  // Low-medium entropy
			Detectability: 0.95, // Very high detectability
		}
	default:
		return PatternInfo{
			Name:          "Unknown",
			Description:   "Unknown pattern type",
			Entropy:       0.0,
			Detectability: 0.0,
		}
	}
}

// CalculateEntropy calculates the Shannon entropy of a byte array
func CalculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0.0
	}

	// Count byte frequencies
	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}

	// Calculate entropy
	entropy := 0.0
	length := float64(len(data))

	for _, count := range freq {
		if count > 0 {
			p := float64(count) / length
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}
