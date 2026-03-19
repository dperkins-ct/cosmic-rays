package memory

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newBlock(size int) *Block {
	data := make([]byte, size)
	return &Block{offset: 0, size: int64(size), data: data}
}

func newBlockAt(offset int64, size int) *Block {
	data := make([]byte, size)
	return &Block{offset: offset, size: int64(size), data: data}
}

func repeatPattern(pat []byte, size int) []byte {
	out := make([]byte, size)
	for i := range out {
		out[i] = pat[i%len(pat)]
	}
	return out
}

// ---------------------------------------------------------------------------
// Block.GetSize
// ---------------------------------------------------------------------------

func TestBlock_GetSize(t *testing.T) {
	tests := map[string]struct {
		size int
		want int64
	}{
		"1 KB block":  {size: 1024, want: 1024},
		"1 MB block":  {size: 1024 * 1024, want: 1024 * 1024},
		"single byte": {size: 1, want: 1},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := newBlock(tc.size).GetSize(); got != tc.want {
				t.Errorf("GetSize=%d, want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Block.WritePattern
// ---------------------------------------------------------------------------

func TestBlock_WritePattern(t *testing.T) {
	tests := map[string]struct {
		blockSize int
		pattern   []byte
		wantErr   bool
		checkFn   func(data, pat []byte) bool
	}{
		"4-byte pattern fills evenly": {
			blockSize: 8,
			pattern:   []byte{0xAA, 0x55, 0xAA, 0x55},
			checkFn: func(data, pat []byte) bool {
				for i, b := range data {
					if b != pat[i%len(pat)] {
						return false
					}
				}
				return true
			},
		},
		"pattern longer than block": {
			blockSize: 3,
			pattern:   []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			checkFn: func(data, pat []byte) bool {
				for i, b := range data {
					if b != pat[i%len(pat)] {
						return false
					}
				}
				return true
			},
		},
		"single byte pattern repeats": {
			blockSize: 4,
			pattern:   []byte{0xFF},
			checkFn: func(data, _ []byte) bool {
				for _, b := range data {
					if b != 0xFF {
						return false
					}
				}
				return true
			},
		},
		"empty pattern returns error": {
			blockSize: 4,
			pattern:   []byte{},
			wantErr:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newBlock(tc.blockSize)
			err := b.WritePattern(tc.pattern)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkFn != nil && !tc.checkFn(b.data, tc.pattern) {
				t.Errorf("data after WritePattern does not match expected; data=%v", b.data)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Block.VerifyPattern
// ---------------------------------------------------------------------------

func TestBlock_VerifyPattern(t *testing.T) {
	pat := []byte{0xAA, 0x55}

	tests := map[string]struct {
		setup      func(*Block)
		pattern    []byte
		wantFlips  int
		wantErr    bool
		checkFlip0 *BitFlip // optional first-flip assertions (nil = skip)
	}{
		"no corruption zero flips": {
			setup:     func(_ *Block) {},
			pattern:   pat,
			wantFlips: 0,
		},
		"one byte changed returns one flip": {
			setup:      func(b *Block) { b.data[0] = 0x00 },
			pattern:    pat,
			wantFlips:  1,
			checkFlip0: &BitFlip{Offset: 0, OriginalValue: 0xAA, CurrentValue: 0x00},
		},
		"two bytes changed returns two flips": {
			setup:     func(b *Block) { b.data[0] = 0x00; b.data[1] = 0x00 },
			pattern:   pat,
			wantFlips: 2,
		},
		"empty pattern returns error": {
			setup:   func(_ *Block) {},
			pattern: []byte{},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newBlock(4)
			if len(tc.pattern) > 0 {
				if err := b.WritePattern(tc.pattern); err != nil {
					t.Fatalf("WritePattern setup: %v", err)
				}
			}
			tc.setup(b)

			flips, err := b.VerifyPattern(tc.pattern)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(flips) != tc.wantFlips {
				t.Errorf("got %d flips, want %d", len(flips), tc.wantFlips)
			}
			if tc.checkFlip0 != nil && len(flips) > 0 {
				f := flips[0]
				if f.Offset != tc.checkFlip0.Offset {
					t.Errorf("flip[0].Offset=%d, want %d", f.Offset, tc.checkFlip0.Offset)
				}
				if f.OriginalValue != tc.checkFlip0.OriginalValue {
					t.Errorf("flip[0].OriginalValue=0x%02X, want 0x%02X", f.OriginalValue, tc.checkFlip0.OriginalValue)
				}
				if f.CurrentValue != tc.checkFlip0.CurrentValue {
					t.Errorf("flip[0].CurrentValue=0x%02X, want 0x%02X", f.CurrentValue, tc.checkFlip0.CurrentValue)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Block.RepairFlips
// ---------------------------------------------------------------------------

func TestBlock_RepairFlips(t *testing.T) {
	pat := []byte{0xAA, 0x55}

	tests := map[string]struct {
		corruptIdx []int
		corruptVal []byte
	}{
		"repair single flip":      {corruptIdx: []int{0}, corruptVal: []byte{0x00}},
		"repair multiple flips":   {corruptIdx: []int{0, 2}, corruptVal: []byte{0x11, 0x22}},
		"empty flip list (no-op)": {corruptIdx: []int{}, corruptVal: []byte{}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newBlock(4)
			if err := b.WritePattern(pat); err != nil {
				t.Fatalf("WritePattern: %v", err)
			}
			want := repeatPattern(pat, 4)

			flips := make([]BitFlip, len(tc.corruptIdx))
			for i, idx := range tc.corruptIdx {
				orig := b.data[idx]
				b.data[idx] = tc.corruptVal[i]
				flips[i] = BitFlip{Offset: int64(idx), OriginalValue: orig, CurrentValue: tc.corruptVal[i], DetectedAt: time.Now()}
			}

			b.RepairFlips(flips, want)

			for i, got := range b.data {
				if got != want[i] {
					t.Errorf("data[%d]=0x%02X, want 0x%02X after repair", i, got, want[i])
				}
			}
		})
	}
}

func TestBlock_RepairFlips_OutOfBoundsOffset(t *testing.T) {
	tests := map[string]struct {
		offset int64
	}{
		"positive out of bounds": {offset: 9999},
		"negative offset":        {offset: -1},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newBlock(4)
			pat := []byte{0xAA}
			if err := b.WritePattern(pat); err != nil {
				t.Fatalf("WritePattern: %v", err)
			}
			// Should not panic
			b.RepairFlips([]BitFlip{{Offset: tc.offset, OriginalValue: 0xAA, CurrentValue: 0x00}}, pat)
			for i, got := range b.data {
				if got != 0xAA {
					t.Errorf("data[%d] changed to 0x%02X unexpectedly", i, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Manager.AllocateBlocks
// ---------------------------------------------------------------------------

func TestManager_AllocateBlocks(t *testing.T) {
	tests := map[string]struct {
		memSize   int
		numBlocks int
		wantErr   bool
	}{
		"4 equal blocks from 4096 bytes": {memSize: 4096, numBlocks: 4},
		"1 block":                        {memSize: 1024, numBlocks: 1},
		"more blocks than bytes":         {memSize: 4, numBlocks: 16, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m := &Manager{memory: make([]byte, tc.memSize), size: int64(tc.memSize)}
			err := m.AllocateBlocks(tc.numBlocks)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := len(m.GetBlocks()); got != tc.numBlocks {
				t.Errorf("block count=%d, want %d", got, tc.numBlocks)
			}
			var total int64
			for _, bl := range m.GetBlocks() {
				total += bl.GetSize()
			}
			if total != int64(tc.memSize) {
				t.Errorf("blocks cover %d bytes, want %d", total, tc.memSize)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Manager.RecordBitFlip
// ---------------------------------------------------------------------------

func TestManager_RecordBitFlip(t *testing.T) {
	tests := map[string]struct {
		numFlips int
	}{
		"single flip":    {numFlips: 1},
		"multiple flips": {numFlips: 5},
		"zero flips":     {numFlips: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m := &Manager{}
			for i := 0; i < tc.numFlips; i++ {
				m.RecordBitFlip(BitFlip{Offset: int64(i)})
			}
			if m.bitFlipCount != int64(tc.numFlips) {
				t.Errorf("bitFlipCount=%d, want %d", m.bitFlipCount, tc.numFlips)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Manager.GetMemory / GetSize
// ---------------------------------------------------------------------------

func TestManager_GetMemoryAndSize(t *testing.T) {
	tests := map[string]struct {
		size int
	}{
		"1 KB": {size: 1024},
		"4 KB": {size: 4096},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m := &Manager{memory: make([]byte, tc.size), size: int64(tc.size)}
			if got := m.GetSize(); got != int64(tc.size) {
				t.Errorf("GetSize=%d, want %d", got, tc.size)
			}
			if got := len(m.GetMemory()); got != tc.size {
				t.Errorf("len(GetMemory)=%d, want %d", got, tc.size)
			}
		})
	}
}
