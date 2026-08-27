package backend

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const modernHelp = `-m,    --model MODEL
-fa,   --flash-attn [on|off|auto]       set Flash Attention use
-ctk,  --cache-type-k TYPE              KV cache data type for K
                                        allowed values: f32, f16, bf16, q8_0, q4_0, q4_1, iq4_nl, q5_0, q5_1
                                        (default: f16)
-ctv,  --cache-type-v TYPE              KV cache data type for V
                                        allowed values: f32, f16, bf16, q8_0, q4_0, q4_1, iq4_nl, q5_0, q5_1
-ngl,  --gpu-layers, --n-gpu-layers N   max. number of layers to store in VRAM
-b,    --batch-size N                   logical maximum batch size
-ub,   --ubatch-size N                  physical maximum batch size
--mlock, --no-mmap
-ot,   --override-tensor <pattern>=<buffer>
-st,   --single-turn                    run conversation for a single turn only`

// A minimal legacy-style help: no -fa choices line, no cache-type listing.
const legacyHelp = `-m, --model MODEL
-ngl, --n-gpu-layers N
-b, --batch-size N
--mlock
-no-cnv`

func TestParseCapabilitiesModernBuild(t *testing.T) {
	caps := ParseCapabilities(modernHelp)
	if !caps.Discovered {
		t.Fatal("modern help must be discovered")
	}
	if !caps.FlashAttention || !caps.FAValueStyle {
		t.Errorf("FA detection wrong: %+v", caps)
	}
	if !caps.GPULayers || !caps.Batch || !caps.UBatch || !caps.MLock || !caps.MMap {
		t.Errorf("basic flags missed: %+v", caps)
	}
	if !caps.OverrideTensor || !caps.SingleTurn {
		t.Errorf("placement/single-turn missed: %+v", caps)
	}
	for _, want := range []string{"q8_0", "q4_0"} {
		if !caps.SupportedKV(want) {
			t.Errorf("%s must be supported", want)
		}
	}
	if caps.SupportedKV("q2_k") {
		t.Error("non-listed type must not be claimed as supported")
	}
	if got := caps.SupportedKVTunables(); len(got) != 2 || got[0] != "q8_0" || got[1] != "q4_0" {
		t.Errorf("tunables = %v (fidelity order)", got)
	}
}

func TestParseCapabilitiesLegacyBuild(t *testing.T) {
	caps := ParseCapabilities(legacyHelp)
	if !caps.Discovered {
		t.Fatal("legacy help must be discovered")
	}
	if caps.FlashAttention {
		t.Error("legacy help lists no flash attention")
	}
	if !caps.Discovered || !caps.GPULayers || !caps.Batch {
		t.Errorf("basic flags missed: %+v", caps)
	}
	if len(caps.KVTypes) != 0 {
		t.Errorf("no cache-type section means unknown/absent types, got %v", caps.KVTypes)
	}
	if caps.SupportedKV("q8_0") {
		t.Error("build without -ctk must not claim quantized KV support")
	}
	if caps.OverrideTensor {
		t.Error("legacy build has no override-tensor")
	}
}

func TestParseCapabilitiesEmpty(t *testing.T) {
	caps := ParseCapabilities("")
	if caps.Discovered {
		t.Error("empty help must stay undiscovered")
	}
	if !caps.SupportedKV("q8_0") {
		t.Error("undiscovered capabilities are permissive-with-retry")
	}
}

func TestValidateAgainstCaps(t *testing.T) {
	caps := ParseCapabilities(modernHelp)
	cfg := Config{KVCacheType: "q8_0", ExpertsOnCPU: true}
	if err := validateAgainstCaps(cfg, caps); err != nil {
		t.Errorf("supported config rejected: %v", err)
	}

	bad := Config{KVCacheType: "q6_k"}
	if err := validateAgainstCaps(bad, caps); !errors.Is(err, ErrUnsupported) {
		t.Errorf("unsupported KV must fail loudly, got %v", err)
	}

	minimal := ParseCapabilities(legacyHelp)
	if err := validateAgainstCaps(Config{ExpertsOnCPU: true}, minimal); !errors.Is(err, ErrUnsupported) {
		t.Errorf("expert placement without -ot must fail loudly, got %v", err)
	}

	undiscovered := Capabilities{}
	if err := validateAgainstCaps(Config{KVCacheType: "q9_magic"}, undiscovered); err != nil {
		t.Errorf("undiscovered caps defer to retry chain, got %v", err)
	}
}

func TestLlamaCLIExposesCapabilitiesAfterProbe(t *testing.T) {
	var src CapabilitySource = NewLlamaCLI("")
	l := src.(*LlamaCLI)
	// Without an available binary the probe fails; capabilities stay
	// undiscovered and permissive.
	_ = l.Available(context.Background())
	caps := l.Capabilities()
	if caps.Discovered && strings.TrimSpace(caps.KVTypes[0]) == "" {
		t.Error("discovered caps must parse real type names")
	}
}
