package gguf

import (
	"fmt"
	"math"
	"strings"
)

// MoEInfo describes Mixture-of-Experts metadata extracted from GGUF keys.
type MoEInfo struct {
	TotalExperts  uint64 // {arch}.expert_count
	ActiveExperts uint64 // {arch}.expert_used_count (0 = unknown)
	ExpertFFNSize uint64 // {arch}.expert_feed_forward_length (0 = unknown)
}

// ModelInfo is the derived geometry summary Gumi reasons about.
//
// Every field is read from actual GGUF metadata — no model catalog involved.
type ModelInfo struct {
	Path         string
	FileSize     uint64 // bytes on disk
	Architecture string // general.architecture, e.g. "qwen3moe"
	Name         string // general.name

	ParamCount uint64 // exact sum of tensor element counts
	QuantLabel string // e.g. "Q4_K_M", derived from general.file_type
	FileType   int32

	LayerCount    int64 // {arch}.block_count
	TrainContext  int64 // {arch}.context_length
	HiddenSize    int64 // {arch}.embedding_length
	HeadCount     int64 // {arch}.attention.head_count
	KVHeadCount   int64 // {arch}.attention.head_count_kv (defaults to HeadCount)
	HeadDim       int64 // attention.key_length else HiddenSize/HeadCount
	RopeFreqBase  float64
	RopeScaling   string  // "" = none declared
	RopeScaleFact float64 // rope.scaling.factor when present

	MoE *MoEInfo // nil for dense models

	WeightBytes uint64 // computed from tensor dims + ggml type sizes
	ExpertBytes uint64 // subset of WeightBytes held in *_exps.* tensors
}

// Inspect parses the GGUF file at path and derives a ModelInfo.
func Inspect(path string) (*ModelInfo, error) {
	fs, size, err := openAndStat(path)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	f, err := Parse(fs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return derive(f, path, size), nil
}

func derive(f *File, path string, fileSize uint64) *ModelInfo {
	m := &ModelInfo{
		Path:         path,
		FileSize:     fileSize,
		Architecture: f.GetString("general.architecture"),
		Name:         f.GetString("general.name"),
	}
	if m.Architecture == "" {
		m.Architecture = "unknown"
	}
	a := m.Architecture

	if ft, ok := f.GetI64("general.file_type"); ok {
		m.FileType = int32(ft)
	}
	m.QuantLabel = fileTypeLabel(m.FileType)

	getInt := func(suffix string) (int64, bool) {
		v, ok := f.GetU64(a + suffix)
		return int64(v), ok
	}

	m.LayerCount, _ = getInt(".block_count")
	m.TrainContext, _ = getInt(".context_length")
	m.HiddenSize, _ = getInt(".embedding_length")
	m.HeadCount, _ = getInt(".attention.head_count")

	if kvh, ok := getInt(".attention.head_count_kv"); ok {
		m.KVHeadCount = kvh
	} else {
		m.KVHeadCount = m.HeadCount // MHA fallback per GGUF convention
	}
	if keyLen, ok := getInt(".attention.key_length"); ok && keyLen > 0 {
		m.HeadDim = keyLen
	} else if m.HeadCount > 0 {
		m.HeadDim = m.HiddenSize / m.HeadCount
	}
	m.RopeFreqBase, _ = f.GetF64(a + ".rope.freq_base")
	if s := f.GetString(a + ".rope.scaling.type"); s != "" {
		m.RopeScaling = s
	} else if v, ok := f.GetU64(a + ".rope.scaling.type"); ok {
		m.RopeScaling = fmt.Sprintf("%d", v)
	}
	m.RopeScaleFact, _ = f.GetF64(a + ".rope.scaling.factor")

	if experts, ok := getInt(".expert_count"); ok && experts > 0 {
		mo := &MoEInfo{TotalExperts: uint64(experts)}
		mo.ActiveExperts, _ = f.GetU64(a + ".expert_used_count")
		if v, ok := f.GetU64(a + ".expert_feed_forward_length"); ok {
			mo.ExpertFFNSize = v
		}
		m.MoE = mo
	}

	for _, ti := range f.Tensors {
		n := elemCount(ti.Dims)
		m.ParamCount += n
		b := tensorBytes(ti.TypeID, n)
		m.WeightBytes += b
		if strings.Contains(ti.Name, "_exps.") || strings.HasSuffix(ti.Name, "_exps.weight") {
			m.ExpertBytes += b
		}
	}
	return m
}

func elemCount(dims []uint64) uint64 {
	n := uint64(1)
	for _, d := range dims {
		n *= d
	}
	return n
}

// FormatParams renders an approximate human parameter count, e.g. "~30.5B".
func FormatParams(n uint64) string {
	f := float64(n)
	switch {
	case n == 0:
		return "unknown"
	case f >= 1e12:
		return fmt.Sprintf("~%.1fT", f/1e12)
	case f >= 1e9:
		return fmt.Sprintf("~%.1fB", f/1e9)
	case f >= 1e6:
		return fmt.Sprintf("~%.1fM", f/1e6)
	default:
		return fmt.Sprintf("%d", n)
	}
}

var _ = math.MaxInt32 // placeholder to keep math available for future use
