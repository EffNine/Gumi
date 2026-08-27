package gguf

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Parse reads a complete GGUF header, metadata section, and tensor directory.
// It never seeks into tensor data, so it works on plain files and streams.
func Parse(r io.Reader) (*File, error) {
	rd := &reader{br: bufio.NewReaderSize(r, 1<<16)}

	magic, err := rd.u32()
	if err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if magic != Magic {
		return nil, fmt.Errorf("not a GGUF file (magic 0x%08X)", magic)
	}
	version, err := rd.u32()
	if err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version < 2 || version > 3 {
		return nil, fmt.Errorf("unsupported GGUF version %d (supported: 2, 3)", version)
	}
	tensorCount, err := rd.u64()
	if err != nil {
		return nil, fmt.Errorf("read tensor count: %w", err)
	}
	kvCount, err := rd.u64()
	if err != nil {
		return nil, fmt.Errorf("read metadata count: %w", err)
	}

	f := &File{Version: version, TensorCount: tensorCount}

	for i := uint64(0); i < kvCount; i++ {
		key, err := rd.str()
		if err != nil {
			return nil, fmt.Errorf("metadata %d: read key: %w", i, err)
		}
		t, err := rd.u32()
		if err != nil {
			return nil, fmt.Errorf("metadata %q: read type: %w", key, err)
		}
		val, err := rd.value(MetaType(t))
		if err != nil {
			return nil, fmt.Errorf("metadata %q: %w", key, err)
		}
		f.KV = append(f.KV, KVPair{Key: key, Value: val})
	}

	for i := uint64(0); i < tensorCount; i++ {
		var ti TensorInfo
		ti.Name, err = rd.str()
		if err != nil {
			return nil, fmt.Errorf("tensor %d: read name: %w", i, err)
		}
		nDims, err := rd.u32()
		if err != nil {
			return nil, fmt.Errorf("tensor %q: read dims: %w", ti.Name, err)
		}
		if nDims == 0 || nDims > 8 {
			return nil, fmt.Errorf("tensor %q: invalid dim count %d", ti.Name, nDims)
		}
		ti.Dims = make([]uint64, nDims)
		for j := range ti.Dims {
			if ti.Dims[j], err = rd.u64(); err != nil {
				return nil, fmt.Errorf("tensor %q: read dim: %w", ti.Name, err)
			}
		}
		if ti.TypeID, err = rd.u32(); err != nil {
			return nil, fmt.Errorf("tensor %q: read type: %w", ti.Name, err)
		}
		if ti.Offset, err = rd.u64(); err != nil {
			return nil, fmt.Errorf("tensor %q: read offset: %w", ti.Name, err)
		}
		f.Tensors = append(f.Tensors, ti)
	}
	return f, nil
}

type reader struct {
	br  *bufio.Reader
	buf [8]byte
}

func (r *reader) need(n int) error {
	_, err := io.ReadFull(r.br, r.buf[:n])
	return err
}

func (r *reader) u8() (uint8, error) {
	if err := r.need(1); err != nil {
		return 0, err
	}
	return r.buf[0], nil
}

func (r *reader) u32() (uint32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(r.buf[:4]), nil
}

func (r *reader) u64() (uint64, error) {
	if err := r.need(8); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(r.buf[:8]), nil
}

func (r *reader) str() (string, error) {
	n, err := r.u64()
	if err != nil {
		return "", err
	}
	if n > maxStringBytes {
		return "", fmt.Errorf("string length %d exceeds limit %d", n, maxStringBytes)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r.br, b); err != nil {
		return "", err
	}
	return string(b), nil
}

const maxStringBytes = 64 << 20 // sanity cap for tokenizer arrays

func (r *reader) value(t MetaType) (any, error) {
	switch t {
	case TypeUInt8:
		return r.u8()
	case TypeInt8:
		v, err := r.u8()
		return int8(v), err
	case TypeUInt16:
		b := r.buf[:2]
		if _, err := io.ReadFull(r.br, b); err != nil {
			return nil, err
		}
		return binary.LittleEndian.Uint16(b), nil
	case TypeInt16:
		b := r.buf[:2]
		if _, err := io.ReadFull(r.br, b); err != nil {
			return nil, err
		}
		return int16(binary.LittleEndian.Uint16(b)), nil
	case TypeUInt32:
		return r.u32()
	case TypeInt32:
		v, err := r.u32()
		return int32(v), err
	case TypeFloat32:
		v, err := r.u32()
		return float32FromBits(v), err
	case TypeBool:
		v, err := r.u8()
		return v != 0, err
	case TypeString:
		return r.str()
	case TypeArray:
		elemType, err := r.u32()
		if err != nil {
			return nil, err
		}
		count, err := r.u64()
		if err != nil {
			return nil, err
		}
		if count > maxArrayElements {
			return nil, fmt.Errorf("array length %d exceeds limit %d", count, maxArrayElements)
		}
		arr := make([]any, 0, min(count, 1024))
		for i := uint64(0); i < count; i++ {
			v, err := r.value(MetaType(elemType))
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	case TypeUInt64:
		return r.u64()
	case TypeInt64:
		v, err := r.u64()
		return int64(v), err
	case TypeFloat64:
		v, err := r.u64()
		return float64FromBits(v), err
	default:
		return nil, fmt.Errorf("unknown metadata type id %d", uint32(t))
	}
}

const maxArrayElements = 4 << 20

func float32FromBits(b uint32) float32 { return math.Float32frombits(b) }

func float64FromBits(b uint64) float64 { return math.Float64frombits(b) }

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
