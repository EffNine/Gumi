// Package gguf implements a minimal, dependency-free reader for GGUF model
// files. It parses the header, all metadata key/value pairs, and tensor
// information records. Tensor data is never read.
//
// Supported versions: GGUF v2 and v3 (identical layout).
package gguf

import (
	"fmt"
	"math"
)

// Magic is the little-endian encoding of "GGUF".
const Magic uint32 = 0x46554747

// MetaType enumerates GGUF metadata value types.
type MetaType uint32

const (
	TypeUInt8 MetaType = iota
	TypeInt8
	TypeUInt16
	TypeInt16
	TypeUInt32
	TypeInt32
	TypeFloat32
	TypeBool
	TypeString
	TypeArray
	TypeUInt64
	TypeInt64
	TypeFloat64
)

func (t MetaType) String() string {
	switch t {
	case TypeUInt8:
		return "uint8"
	case TypeInt8:
		return "int8"
	case TypeUInt16:
		return "uint16"
	case TypeInt16:
		return "int16"
	case TypeUInt32:
		return "uint32"
	case TypeInt32:
		return "int32"
	case TypeFloat32:
		return "float32"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeArray:
		return "array"
	case TypeUInt64:
		return "uint64"
	case TypeInt64:
		return "int64"
	case TypeFloat64:
		return "float64"
	}
	return fmt.Sprintf("meta_type(%d)", uint32(t))
}

// KVPair is one metadata key with its typed value.
type KVPair struct {
	Key   string
	Value any // integer types, float32/float64, bool, string, or []any
}

// TensorInfo describes one tensor record from the tensor directory.
type TensorInfo struct {
	Name   string
	Dims   []uint64 // as stored (order differs per spec); product is invariant
	TypeID uint32
	Offset uint64
}

// File is the parsed content of a GGUF file (metadata only, no tensor data).
type File struct {
	Version     uint32
	TensorCount uint64
	KV          []KVPair
	Tensors     []TensorInfo

	kvIndex map[string]any
}

// Get returns the value for a metadata key, or nil when absent.
func (f *File) Get(key string) any {
	if f.kvIndex == nil {
		f.kvIndex = make(map[string]any, len(f.KV))
		for _, kv := range f.KV {
			f.kvIndex[kv.Key] = kv.Value
		}
	}
	return f.kvIndex[key]
}

// GetString returns a string metadata value or "".
func (f *File) GetString(key string) string {
	if s, ok := f.Get(key).(string); ok {
		return s
	}
	return ""
}

// GetI64 returns a signed integer value coerced from any integer type.
func (f *File) GetI64(key string) (int64, bool) {
	switch n := f.Get(key).(type) {
	case uint8:
		return int64(n), true
	case int8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case int16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case int32:
		return int64(n), true
	case uint64:
		if n <= math.MaxInt64 {
			return int64(n), true
		}
	case int64:
		return n, true
	}
	return 0, false
}

// GetU64 returns an unsigned integer value coerced from any integer type.
func (f *File) GetU64(key string) (uint64, bool) {
	v := f.Get(key)
	switch n := v.(type) {
	case uint8:
		return uint64(n), true
	case int8:
		if n >= 0 {
			return uint64(n), true
		}
	case uint16:
		return uint64(n), true
	case int16:
		if n >= 0 {
			return uint64(n), true
		}
	case uint32:
		return uint64(n), true
	case int32:
		if n >= 0 {
			return uint64(n), true
		}
	case uint64:
		return n, true
	case int64:
		if n >= 0 {
			return uint64(n), true
		}
	}
	return 0, false
}

// GetF64 returns a float value coerced from float32/float64.
func (f *File) GetF64(key string) (float64, bool) {
	switch x := f.Get(key).(type) {
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}
