// Package testgguf synthesizes minimal GGUF files for tests across packages.
package testgguf

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Tensor is one tensor record to embed.
type Tensor struct {
	Name   string
	Dims   []uint64
	TypeID uint32
}

// Builder assembles a GGUF v3 image.
type Builder struct {
	arch    string
	kvs     []kv
	tensors []Tensor
}

type kv struct {
	key string
	typ uint32
	val func(buf []byte) []byte
}

// New starts a builder for the given general.architecture.
func New(arch string) *Builder { return &Builder{arch: arch} }

func (b *Builder) Str(k, v string) *Builder {
	b.kvs = append(b.kvs, kv{k, 8, func([]byte) []byte {
		out := make([]byte, 8+len(v))
		binary.LittleEndian.PutUint64(out, uint64(len(v)))
		copy(out[8:], v)
		return out
	}})
	return b
}

func (b *Builder) U64(k string, v uint64) *Builder {
	b.kvs = append(b.kvs, kv{k, 10, func([]byte) []byte {
		out := make([]byte, 8)
		binary.LittleEndian.PutUint64(out, v)
		return out
	}})
	return b
}

func (b *Builder) I32(k string, v int32) *Builder {
	b.kvs = append(b.kvs, kv{k, 5, func([]byte) []byte {
		out := make([]byte, 4)
		binary.LittleEndian.PutUint32(out, uint32(v))
		return out
	}})
	return b
}

func (b *Builder) F32(k string, v float32) *Builder {
	b.kvs = append(b.kvs, kv{k, 6, func([]byte) []byte {
		out := make([]byte, 4)
		binary.LittleEndian.PutUint32(out, f32bits(v))
		return out
	}})
	return b
}

func (b *Builder) Bool(k string, v bool) *Builder {
	n := uint8(0)
	if v {
		n = 1
	}
	b.kvs = append(b.kvs, kv{k, 7, func([]byte) []byte { return []byte{n} }})
	return b
}

func f32bits(f float32) uint32 { return math.Float32bits(f) }

// Arch sets common geometry keys derived from arch.
func (b *Builder) Arch() *Builder {
	return b.Str("general.architecture", b.arch).
		Str("general.name", "test-model-"+b.arch)
}

func (b *Builder) Geometry(blockCount, ctxLen, embLen, heads, kvHeads int64) *Builder {
	p := b.arch + "."
	return b.U64(p+"block_count", uint64(blockCount)).
		U64(p+"context_length", uint64(ctxLen)).
		U64(p+"embedding_length", uint64(embLen)).
		U64(p+"attention.head_count", uint64(heads)).
		U64(p+"attention.head_count_kv", uint64(kvHeads))
}

func (b *Builder) Rope(base float32) *Builder {
	return b.F32(b.arch+".rope.freq_base", base)
}

func (b *Builder) MoE(totalExperts, activeExperts, expertFFN int64) *Builder {
	p := b.arch
	return b.U64(p+".expert_count", uint64(totalExperts)).
		U64(p+".expert_used_count", uint64(activeExperts)).
		U64(p+".expert_feed_forward_length", uint64(expertFFN))
}

func (b *Builder) FileType(t int32) *Builder { return b.I32("general.file_type", t) }

func (b *Builder) Tensor(name string, dims []uint64, typeID uint32) *Builder {
	b.tensors = append(b.tensors, Tensor{Name: name, Dims: dims, TypeID: typeID})
	return b
}

// Bytes renders the complete GGUF image.
func (b *Builder) Bytes() []byte {
	var out []byte
	putU32 := func(v uint32) { var x [4]byte; binary.LittleEndian.PutUint32(x[:], v); out = append(out, x[:]...) }
	putU64 := func(v uint64) { var x [8]byte; binary.LittleEndian.PutUint64(x[:], v); out = append(out, x[:]...) }

	putU32(0x46554747) // magic
	putU32(3)          // version
	putU64(uint64(len(b.tensors)))
	putU64(uint64(len(b.kvs)))
	for _, k := range b.kvs {
		putU64(uint64(len(k.key)))
		out = append(out, k.key...)
		putU32(k.typ)
		out = append(out, k.val(nil)...)
	}
	for _, t := range b.tensors {
		putU64(uint64(len(t.Name)))
		out = append(out, t.Name...)
		putU32(uint32(len(t.Dims)))
		for _, d := range t.Dims {
			putU64(d)
		}
		putU32(t.TypeID)
		putU64(0) // offset
	}
	return out
}

// WriteFile writes the model to a temp file and returns its path.
func (b *Builder) WriteFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
