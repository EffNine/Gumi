package hardware

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

func unmarshalJSON(data []byte, v any) error { return json.Unmarshal(data, v) }

// MeasureMemoryBandwidth estimates host memory bandwidth with a streaming
// copy micro-benchmark. It reports the best of several passes, which tracks
// peak sustainable bandwidth better than the mean. Result is an estimate.
func MeasureMemoryBandwidth() (Bandwidth, error) {
	const elems = 8 << 20 // 8M float64s = 64 MiB per buffer
	src := make([]float64, elems)
	dst := make([]float64, elems)
	for i := range src {
		src[i] = float64(i % 1024)
	}

	var best float64
	passes := 7
	for p := 0; p < passes; p++ {
		start := time.Now()
		n := copy(dst, src)
		dur := time.Since(start)
		if n != elems {
			return Bandwidth{}, fmt.Errorf("short copy: %d", n)
		}
		if p == 0 {
			continue // warmup pass
		}
		// Each pass reads and writes 64 MiB.
		gbps := float64(2*elems*8) / dur.Seconds() / 1e9
		if gbps > best {
			best = gbps
		}
	}
	if best == 0 || math.IsInf(best, 0) || math.IsNaN(best) {
		return Bandwidth{}, fmt.Errorf("bandwidth measurement produced no usable sample")
	}
	return Bandwidth{GBps: best, Measured: true}, nil
}
