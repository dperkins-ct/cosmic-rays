package patterns

import "testing"

// ---------------------------------------------------------------------------
// Generator.Generate
// ---------------------------------------------------------------------------

func TestGenerator_Generate(t *testing.T) {
	g := NewGenerator()

	tests := map[string]struct {
		patternType PatternType
		size        int
		wantErr     bool
		checkFn     func([]byte) bool
	}{
		"alternating 8 bytes first byte 0x55": {
			patternType: PatternAlternating,
			size:        8,
			checkFn:     func(b []byte) bool { return len(b) == 8 && b[0] == 0x55 },
		},
		"checksum 256 bytes correct length": {
			patternType: PatternChecksum,
			size:        256,
			checkFn:     func(b []byte) bool { return len(b) == 256 },
		},
		"known 8 bytes first byte 0x00": {
			patternType: PatternKnown,
			size:        8,
			checkFn:     func(b []byte) bool { return len(b) == 8 && b[0] == 0x00 },
		},
		"random 16 bytes correct length": {
			patternType: PatternRandom,
			size:        16,
			checkFn:     func(b []byte) bool { return len(b) == 16 },
		},
		"unknown pattern type returns error": {
			patternType: PatternType("cosmic"),
			size:        8,
			wantErr:     true,
		},
		"zero size returns error": {
			patternType: PatternAlternating,
			size:        0,
			wantErr:     true,
		},
		"negative size returns error": {
			patternType: PatternAlternating,
			size:        -1,
			wantErr:     true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := g.Generate(tc.patternType, tc.size)
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
				t.Errorf("pattern check failed for %q; got len=%d first byte=0x%02X", tc.patternType, len(got), got[0])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// classifyBitFlip (unexported)
// ---------------------------------------------------------------------------

func TestClassifyBitFlip(t *testing.T) {
	tests := map[string]struct {
		expected byte
		actual   byte
		want     FlipType
	}{
		"single bit 0 to 1 is FlipSingle":    {expected: 0x00, actual: 0x01, want: FlipSingle},
		"single bit 1 to 0 is FlipSingle":    {expected: 0xFF, actual: 0xFE, want: FlipSingle},
		"multiple bits changed FlipMultiple": {expected: 0x00, actual: 0x03, want: FlipMultiple},
		"all bits changed FlipMultiple":      {expected: 0x00, actual: 0xFF, want: FlipMultiple},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := classifyBitFlip(tc.expected, tc.actual); got != tc.want {
				t.Errorf("classifyBitFlip(0x%02X,0x%02X)=%q, want %q", tc.expected, tc.actual, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CalculateEntropy
// ---------------------------------------------------------------------------

func TestCalculateEntropy(t *testing.T) {
	tests := map[string]struct {
		data    []byte
		wantMin float64
		wantMax float64
	}{
		"empty slice returns 0": {
			data: []byte{}, wantMin: 0.0, wantMax: 0.0,
		},
		"all identical bytes returns 0": {
			data: makeBytes(256, 0x00), wantMin: 0.0, wantMax: 0.001,
		},
		"all 256 distinct values returns ~8": {
			data: allBytes256(), wantMin: 7.9, wantMax: 8.01,
		},
		"alternating 0xAA and 0x55 returns ~1": {
			data: alternating256(), wantMin: 0.9, wantMax: 1.1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := CalculateEntropy(tc.data)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("CalculateEntropy=%.4f, want in [%.4f,%.4f]", got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

func makeBytes(n int, val byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = val
	}
	return b
}

func allBytes256() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func alternating256() []byte {
	b := make([]byte, 256)
	for i := range b {
		if i%2 == 0 {
			b[i] = 0xAA
		} else {
			b[i] = 0x55
		}
	}
	return b
}

// ---------------------------------------------------------------------------
// GetPatternInfo
// ---------------------------------------------------------------------------

func TestGetPatternInfo(t *testing.T) {
	tests := map[string]struct {
		patternType PatternType
		wantName    string
	}{
		"alternating info": {patternType: PatternAlternating, wantName: "Alternating"},
		"checksum info":    {patternType: PatternChecksum, wantName: "Checksum"},
		"random info":      {patternType: PatternRandom, wantName: "Random"},
		"known info":       {patternType: PatternKnown, wantName: "Known Sequences"},
		"unknown type":     {patternType: PatternType("nope"), wantName: "Unknown"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			info := GetPatternInfo(tc.patternType)
			if info.Name != tc.wantName {
				t.Errorf("Name=%q, want %q", info.Name, tc.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Generator.ValidatePattern
// ---------------------------------------------------------------------------

func TestGenerator_ValidatePattern(t *testing.T) {
	g := NewGenerator()

	tests := map[string]struct {
		patternType PatternType
		mutateFn    func([]byte)
		wantValid   bool
	}{
		"clean alternating is valid": {
			patternType: PatternAlternating,
			mutateFn:    nil,
			wantValid:   true,
		},
		"corrupted alternating is invalid": {
			patternType: PatternAlternating,
			mutateFn:    func(b []byte) { b[0] = ^b[0] },
			wantValid:   false,
		},
		"clean known data is valid": {
			patternType: PatternKnown,
			mutateFn:    nil,
			wantValid:   true,
		},
		"multiple corruptions detected": {
			patternType: PatternAlternating,
			mutateFn:    func(b []byte) { b[0] = ^b[0]; b[1] = ^b[1] },
			wantValid:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := g.Generate(tc.patternType, 64)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if tc.mutateFn != nil {
				tc.mutateFn(data)
			}
			result, err := g.ValidatePattern(data, tc.patternType)
			if err != nil {
				t.Fatalf("ValidatePattern: %v", err)
			}
			if result.IsValid != tc.wantValid {
				t.Errorf("IsValid=%v, want %v (CorruptBytes=%d)", result.IsValid, tc.wantValid, result.CorruptBytes)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidationResult.GetStatistics
// ---------------------------------------------------------------------------

func TestValidationResult_GetStatistics(t *testing.T) {
	tests := map[string]struct {
		result    ValidationResult
		wantKeys  []string
		wantValid bool
	}{
		"clean result": {
			result:    ValidationResult{TotalBytes: 100, CorruptBytes: 0, IsValid: true, BitFlips: []BitFlipDetail{}},
			wantKeys:  []string{"total_bytes", "corrupt_bytes", "is_valid", "total_bit_flips"},
			wantValid: true,
		},
		"corrupt result": {
			result: ValidationResult{
				TotalBytes: 100, CorruptBytes: 2, IsValid: false,
				BitFlips: []BitFlipDetail{{FlipType: FlipSingle}, {FlipType: FlipMultiple}},
			},
			wantKeys:  []string{"total_bytes", "corrupt_bytes", "is_valid", "total_bit_flips"},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			stats := tc.result.GetStatistics()
			for _, key := range tc.wantKeys {
				if _, ok := stats[key]; !ok {
					t.Errorf("missing key %q", key)
				}
			}
			if stats["is_valid"] != tc.wantValid {
				t.Errorf("is_valid=%v, want %v", stats["is_valid"], tc.wantValid)
			}
			if stats["total_bit_flips"] != len(tc.result.BitFlips) {
				t.Errorf("total_bit_flips=%v, want %d", stats["total_bit_flips"], len(tc.result.BitFlips))
			}
		})
	}
}
