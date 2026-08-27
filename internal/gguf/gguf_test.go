package gguf

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/EffNine/gumi/internal/testgguf"
)

// qwen3MoEBuilder returns a Qwen3-30B-A3B-shaped synthetic model.
func qwen3MoEBuilder() *testgguf.Builder {
	b := testgguf.New("qwen3moe").Arch().
		Geometry(48, 40960, 2048, 16, 4).
		Rope(1000000).
		MoE(128, 8, 768).
		FileType(15) // Q4_K_M
	// token embedding: 151936 x 2048 in Q4_K (id 12)
	b.Tensor("token_embd.weight", []uint64{2048, 151936}, 12)
	// one attention tensor per layer proxy (F16, id 1)
	b.Tensor("blk.0.attn_q.weight", []uint64{2048, 2048}, 1)
	// expert tensor: [768, 2048, 128] in Q4_K
	b.Tensor("blk.0.ffn_gate_exps.weight", []uint64{768, 2048, 128}, 12)
	return b
}

func TestInspectDerivesGeometry(t *testing.T) {
	path := qwen3MoEBuilder().WriteFile(t)
	m, err := Inspect(path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if m.Architecture != "qwen3moe" {
		t.Errorf("architecture = %q", m.Architecture)
	}
	if m.Name != "test-model-qwen3moe" {
		t.Errorf("name = %q", m.Name)
	}
	if m.LayerCount != 48 || m.TrainContext != 40960 || m.HiddenSize != 2048 {
		t.Errorf("geometry: layers=%d ctx=%d hidden=%d", m.LayerCount, m.TrainContext, m.HiddenSize)
	}
	if m.HeadCount != 16 || m.KVHeadCount != 4 || m.HeadDim != 128 {
		t.Errorf("heads: h=%d kv=%d dim=%d", m.HeadCount, m.KVHeadCount, m.HeadDim)
	}
	if m.QuantLabel != "Q4_K_M" {
		t.Errorf("quant label = %q", m.QuantLabel)
	}
	if m.RopeFreqBase == 0 {
		t.Error("rope freq base missing")
	}
	if m.MoE == nil {
		t.Fatal("MoE metadata missing")
	}
	if m.MoE.TotalExperts != 128 || m.MoE.ActiveExperts != 8 || m.MoE.ExpertFFNSize != 768 {
		t.Errorf("MoE = %+v", m.MoE)
	}

	wantParams := uint64(2048)*151936 + 2048*2048 + 768*2048*128
	if m.ParamCount != wantParams {
		t.Errorf("param count = %d, want %d", m.ParamCount, wantParams)
	}
	if got := FormatParams(m.ParamCount); got != "~201.5M" {
		t.Logf("params formatted as %s (informational)", got)
	}
	if !strings.Contains(FormatParams(30_500_000_000), "B") {
		t.Error("expected B-suffixed param format for billions")
	}
	if m.ExpertBytes == 0 {
		t.Error("expert bytes not detected from _exps. tensors")
	}
	if m.ExpertBytes >= m.WeightBytes {
		t.Error("expert bytes should be a subset of weight bytes")
	}
	if m.WeightBytes == 0 {
		t.Error("weight bytes not computed")
	}
}

func TestKVBytesPerTokenExact(t *testing.T) {
	path := qwen3MoEBuilder().WriteFile(t)
	m, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	// 2 (K and V) x 48 layers x 4 KV heads x 128 head_dim = 49152 elems;
	// at f16 (2 B) that is 98304 bytes = the spec's 96 KiB/token.
	if got := m.KVBytesPerToken("f16"); got != 98304 {
		t.Errorf("f16 KV bytes/token = %d, want 98304", got)
	}
	// q8_0: ceil(49152/32)=1536 blocks x 34 bytes = 52224.
	if got := m.KVBytesPerToken("q8_0"); got != 52224 {
		t.Errorf("q8_0 KV bytes/token = %d, want 52224", got)
	}
	// Spec example sanity: 96 KiB/token for Qwen3-30B geometry at f16.
	if m.KVBytesPerToken("f16") != 96<<10 {
		t.Errorf("spec example mismatch")
	}
	if m.KVBytesPerToken("bogus") != 0 {
		t.Error("unknown kv type must return 0")
	}
}

func TestDenseModelDefaultsKVHeads(t *testing.T) {
	// Dense MHA model: no attention.head_count_kv key at all.
	b := testgguf.New("llama").Arch().
		U64("llama.block_count", 16).
		U64("llama.context_length", 4096).
		U64("llama.embedding_length", 4096).
		U64("llama.attention.head_count", 32).
		I32("general.file_type", 1)
	b.Tensor("tok_embeddings.weight", []uint64{4096, 32000}, 1)
	path := b.WriteFile(t)
	m, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.KVHeadCount != 32 {
		t.Errorf("KVHeadCount = %d, want fallback to HeadCount (32)", m.KVHeadCount)
	}
	if m.HeadDim != 4096/32 {
		t.Errorf("HeadDim = %d, want %d", m.HeadDim, 4096/32)
	}
	if m.QuantLabel != "F16" {
		t.Errorf("QuantLabel = %q", m.QuantLabel)
	}
	if m.MoE != nil {
		t.Error("dense model must have nil MoE")
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		errText string
	}{
		{"bad magic", []byte("NOPE0000"), "not a GGUF file"},
		{"unsupported version", v1File(), "unsupported GGUF version"},
		{"truncated header", []byte("GGUF"), "read version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(tt.data))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errText)
			}
			if !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("error = %v, want containing %q", err, tt.errText)
			}
			if f != nil {
				t.Error("file must be nil on error")
			}
		})
	}
}

// TestWriteFixture writes a synthetic Qwen3-MoE-shaped GGUF when
// GUMI_WRITE_FIXTURE=1. Used by sandbox smoke tests to exercise the CLI
// against a realistic file without downloading models.
func TestWriteFixture(t *testing.T) {
	path := os.Getenv("GUMI_FIXTURE_PATH")
	if os.Getenv("GUMI_WRITE_FIXTURE") == "" || path == "" {
		t.Skip("set GUMI_WRITE_FIXTURE=1 and GUMI_FIXTURE_PATH to generate a fixture")
	}
	b := qwen3MoEBuilder()
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("fixture written to %s", path)
}

func v1File() []byte {
	var out []byte
	putU32 := func(v uint32) { var x [4]byte; binary.LittleEndian.PutUint32(x[:], v); out = append(out, x[:]...) }
	putU32(0x46554747)
	putU32(1)
	return out
}

func TestRoundTripAllValueTypes(t *testing.T) {
	b := testgguf.New("llama").Arch().
		U64("x.uint", 42).
		I32("x.int", -7).
		Bool("x.bool", true).
		Str("x.str", "hello").
		F32("x.float", 2.5)
	path := b.WriteFile(t)
	m, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	// Re-parse raw to check typed access.
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	f, err := Parse(fh)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := f.GetU64("x.uint"); !ok || v != 42 {
		t.Errorf("uint = %d %v", v, ok)
	}
	if v, ok := f.GetI64("x.int"); !ok || v != -7 {
		t.Errorf("int = %d %v", v, ok)
	}
	if v := f.Get("x.bool"); v != true {
		t.Errorf("bool = %v", v)
	}
	if v := f.GetString("x.str"); v != "hello" {
		t.Errorf("string = %q", v)
	}
	if v, ok := f.GetF64("x.float"); !ok || v != 2.5 {
		t.Errorf("float = %v %v", v, ok)
	}
	_ = m
}
