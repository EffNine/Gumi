package backend

import "testing"

func TestParseConfigSpecValid(t *testing.T) {
	cases := []struct {
		spec string
		want Config
	}{
		{
			spec: "ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128",
			want: Config{GPULayers: 33, ContextTokens: 8192, KVCacheType: "q8_0",
				FlashAttention: true, BatchSize: 512, UBatchSize: 128, MMap: true},
		},
		{
			spec: "ngl=max,kv=q4_0,no-fa,no-mmap,mlock,exps-cpu,t=12",
			want: Config{GPULayers: MaxGPULayers, KVCacheType: "q4_0",
				MMap: false, MLock: true, ExpertsOnCPU: true, Threads: 12},
		},
		{
			spec: "c=4096",
			want: Config{ContextTokens: 4096, MMap: true},
		},
		{
			spec: "", // caller filters empties, parser errors
		},
	}
	for _, tc := range cases[:3] {
		got, err := ParseConfigSpec(tc.spec)
		if err != nil {
			t.Errorf("ParseConfigSpec(%q): %v", tc.spec, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseConfigSpec(%q) = %+v, want %+v", tc.spec, got, tc.want)
		}
	}
	if _, err := ParseConfigSpec(""); err == nil {
		t.Error("empty spec must error")
	}
}

func TestParseConfigSpecInvalid(t *testing.T) {
	bad := []string{
		"foo=1",    // unknown key
		"ngl=-3",   // negative
		"ngl=abc",  // not a number
		"c=0",      // non-positive context
		"kv=q6_k",  // unsupported precision
		"fa=true",  // boolean keys take no value
		"b=0",      // non-positive batch
		"t=x",      // bad threads
		"mmap=yes", // boolean keys take no value
	}
	for _, spec := range bad {
		if _, err := ParseConfigSpec(spec); err == nil {
			t.Errorf("ParseConfigSpec(%q) accepted invalid spec", spec)
		}
	}
}

// The spec parser must never grow sampler keys: paired verification depends
// on sampling being forced centrally.
func TestParseConfigSpecRejectsSamplerKeys(t *testing.T) {
	for _, key := range []string{"temp", "temperature", "top_p", "top-k", "top_k", "min_p", "seed"} {
		spec := key + "=0.7"
		if _, err := ParseConfigSpec(spec); err == nil {
			t.Errorf("sampler key %q must be rejected", key)
		}
	}
}
