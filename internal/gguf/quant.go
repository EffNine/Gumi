package gguf

import (
	"fmt"
	"io"
	"math"
	"os"
)

// ggmlTypeInfo carries the block layout of a ggml tensor type.
type ggmlTypeInfo struct {
	Name      string
	BlockSize uint32 // elements per quant block (1 for F32/F16/BF16)
	TypeSize  uint32 // bytes per block
}

// ggmlTypes maps GGML tensor type ids to their storage layouts.
// Ids follow ggml.h; unknown ids fall back to name "type_<id>" and are
// excluded from weight-byte estimates rather than guessed.
var ggmlTypes = map[uint32]ggmlTypeInfo{
	0:  {"F32", 1, 4},
	1:  {"F16", 1, 2},
	2:  {"Q4_0", 32, 18},
	3:  {"Q4_1", 32, 20},
	6:  {"Q5_0", 32, 22},
	7:  {"Q5_1", 32, 24},
	8:  {"Q8_0", 32, 34},
	9:  {"Q8_1", 32, 36},
	10: {"Q2_K", 256, 84},
	11: {"Q3_K", 256, 110},
	12: {"Q4_K", 256, 144},
	13: {"Q5_K", 256, 176},
	14: {"Q6_K", 256, 210},
	15: {"Q8_K", 256, 292},
	16: {"IQ2_XXS", 256, 66},
	17: {"IQ2_XS", 256, 74},
	18: {"IQ3_XXS", 256, 98},
	19: {"IQ1_S", 256, 50},
	20: {"IQ4_NL", 32, 20},
	21: {"IQ3_S", 256, 110},
	22: {"IQ2_S", 256, 66},
	23: {"IQ4_XS", 256, 88},
	24: {"I8_0", 32, 34}, // IQ1_M in some builds; unused for byte math below via lookup miss tolerance
	25: {"IQ4_NL_4", 32, 20},
	26: {"IQ4_XS_LEGACY", 256, 88},
	28: {"IQ1_M", 256, 56},
	30: {"BF16", 1, 2},
}

// TypeName returns the ggml type name for a type id.
func TypeName(id uint32) string {
	if t, ok := ggmlTypes[id]; ok {
		return t.Name
	}
	return fmt.Sprintf("type_%d", id)
}

// tensorBytes estimates stored bytes for nElements of the given ggml type.
// Returns 0 for unknown types (never guesses).
func tensorBytes(typeID uint32, nElements uint64) uint64 {
	t, ok := ggmlTypes[typeID]
	if !ok {
		return 0
	}
	bs := uint64(t.BlockSize)
	sz := uint64(t.TypeSize)
	blocks := (nElements + bs - 1) / bs
	return blocks * sz
}

// fileTypeLabel converts general.file_type into its canonical label.
var fileTypes = map[int32]string{
	0: "F32", 1: "F16",
	2: "Q4_0", 3: "Q4_1",
	7: "Q8_0", 8: "Q5_0", 9: "Q5_1",
	10: "Q2_K", 11: "Q3_K_S", 12: "Q3_K_M", 13: "Q3_K_L",
	14: "Q4_K_S", 15: "Q4_K_M",
	16: "Q5_K_S", 17: "Q5_K_M", 18: "Q6_K",
	19: "IQ2_XXS", 20: "IQ2_XS", 21: "Q2_K_S", 22: "IQ3_XXS",
	23: "IQ1_S", 24: "IQ4_NL", 25: "IQ3_S", 26: "IQ2_S", 27: "IQ4_XS",
	28: "IQ1_M", 29: "BF16",
	30: "Q4_0_4_4", 31: "Q4_0_4_8", 32: "Q4_0_8_8",
}

func fileTypeLabel(t int32) string {
	if s, ok := fileTypes[t]; ok {
		return s
	}
	return fmt.Sprintf("FILETYPE_%d", t)
}

// kvElemLayout returns [blockSize, bytesPerBlock] for a KV cache precision.
// Quantized KV uses ggml block layouts (e.g. q8_0 stores 32 elems in 34 bytes).
var kvLayouts = map[string][2]float64{
	"f16":  {1, 2}, // block size 1, 2 bytes per block (=per elem)
	"bf16": {1, 2},
	"q8_0": {32, 34},
	"q4_0": {32, 18},
}

// SupportedKVTypes lists KV precisions Gumi can reason about.
var SupportedKVTypes = []string{"f16", "q8_0", "q4_0"}

// ValidKVType reports whether kv is a supported KV cache precision.
func ValidKVType(kv string) bool {
	_, ok := kvLayouts[kv]
	return ok
}

// KVBytesPerToken computes the exact KV cache footprint per token:
//
//	2 (K and V) x layers x kv_heads x head_dim scaled by element layout.
//
// Returns 0 when geometry required for the computation is missing.
func (m *ModelInfo) KVBytesPerToken(kv string) uint64 {
	layout, ok := kvLayouts[kv]
	if !ok || m.LayerCount <= 0 || m.KVHeadCount <= 0 || m.HeadDim <= 0 {
		return 0
	}
	elemsPerToken := 2 * uint64(m.LayerCount) * uint64(m.KVHeadCount) * uint64(m.HeadDim)
	bs := uint64(layout[0])
	per := layout[1]
	blocks := (elemsPerToken + bs - 1) / bs
	return uint64(math.Round(float64(blocks) * per))
}

func openAndStat(path string) (io.ReadCloser, uint64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	if st.IsDir() {
		return nil, 0, fmt.Errorf("%s is a directory", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	return f, uint64(st.Size()), nil
}
