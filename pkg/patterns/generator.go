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

// generateChecksum creates an enhanced pattern with both XOR and CRC validation
func (g *Generator) generateChecksum(size int) []byte {
	pattern := make([]byte, size)

	// Enhanced checksum pattern with multiple validation levels
	for i := 0; i < size; i++ {
		if i%16 == 15 {
			// Every 16th byte: CRC-8 of previous 15 bytes
			crc := byte(0)
			for j := i - 15; j < i; j++ {
				if j >= 0 {
					crc = crc8Table[crc^pattern[j]]
				}
			}
			pattern[i] = crc
		} else if i%8 == 7 {
			// Every 8th byte: XOR checksum of previous 7 bytes
			sum := byte(0)
			for j := i - 7; j < i; j++ {
				if j >= 0 {
					sum ^= pattern[j]
				}
			}
			pattern[i] = sum
		} else {
			// Predictable sequence based on position and seed
			pattern[i] = byte((int64(i)*37 + g.seed) % 256)
		}
	}

	return pattern
}

// CRC-8 lookup table for enhanced checksum validation
var crc8Table = [256]byte{
	0x00, 0x07, 0x0e, 0x09, 0x1c, 0x1b, 0x12, 0x15, 0x38, 0x3f, 0x36, 0x31, 0x24, 0x23, 0x2a, 0x2d,
	0x70, 0x77, 0x7e, 0x79, 0x6c, 0x6b, 0x62, 0x65, 0x48, 0x4f, 0x46, 0x41, 0x54, 0x53, 0x5a, 0x5d,
	0xe0, 0xe7, 0xee, 0xe9, 0xfc, 0xfb, 0xf2, 0xf5, 0xd8, 0xdf, 0xd6, 0xd1, 0xc4, 0xc3, 0xca, 0xcd,
	0x90, 0x97, 0x9e, 0x99, 0x8c, 0x8b, 0x82, 0x85, 0xa8, 0xaf, 0xa6, 0xa1, 0xb4, 0xb3, 0xba, 0xbd,
	0xc7, 0xc0, 0xc9, 0xce, 0xdb, 0xdc, 0xd5, 0xd2, 0xff, 0xf8, 0xf1, 0xf6, 0xe3, 0xe4, 0xed, 0xea,
	0xb7, 0xb0, 0xb9, 0xbe, 0xab, 0xac, 0xa5, 0xa2, 0x8f, 0x88, 0x81, 0x86, 0x93, 0x94, 0x9d, 0x9a,
	0x27, 0x20, 0x29, 0x2e, 0x3b, 0x3c, 0x35, 0x32, 0x1f, 0x18, 0x11, 0x16, 0x03, 0x04, 0x0d, 0x0a,
	0x57, 0x50, 0x59, 0x5e, 0x4b, 0x4c, 0x45, 0x42, 0x6f, 0x68, 0x61, 0x66, 0x73, 0x74, 0x7d, 0x7a,
	0x89, 0x8e, 0x87, 0x80, 0x95, 0x92, 0x9b, 0x9c, 0xb1, 0xb6, 0xbf, 0xb8, 0xad, 0xaa, 0xa3, 0xa4,
	0xf9, 0xfe, 0xf7, 0xf0, 0xe5, 0xe2, 0xeb, 0xec, 0xc1, 0xc6, 0xcf, 0xc8, 0xdd, 0xda, 0xd3, 0xd4,
	0x69, 0x6e, 0x67, 0x60, 0x75, 0x72, 0x7b, 0x7c, 0x51, 0x56, 0x5f, 0x58, 0x4d, 0x4a, 0x43, 0x44,
	0x19, 0x1e, 0x17, 0x10, 0x05, 0x02, 0x0b, 0x0c, 0x21, 0x26, 0x2f, 0x28, 0x3d, 0x3a, 0x33, 0x34,
	0x4e, 0x49, 0x40, 0x47, 0x52, 0x55, 0x5c, 0x5b, 0x76, 0x71, 0x78, 0x7f, 0x6a, 0x6d, 0x64, 0x63,
	0x3e, 0x39, 0x30, 0x37, 0x22, 0x25, 0x2c, 0x2b, 0x06, 0x01, 0x08, 0x0f, 0x1a, 0x1d, 0x14, 0x13,
	0xae, 0xa9, 0xa0, 0xa7, 0xb2, 0xb5, 0xbc, 0xbb, 0x96, 0x91, 0x98, 0x9f, 0x8a, 0x8d, 0x84, 0x83,
	0xde, 0xd9, 0xd0, 0xd7, 0xc2, 0xc5, 0xcc, 0xcb, 0xe6, 0xe1, 0xe8, 0xef, 0xfa, 0xfd, 0xf4, 0xf3,
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
