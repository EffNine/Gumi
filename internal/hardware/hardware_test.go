package hardware

import (
	"testing"
)

func TestParseCPUInfo(t *testing.T) {
	data := []byte(`processor	: 0
model name	: Test CPU @ 3.0GHz
physical id	: 0
core id		: 0

processor	: 1
model name	: Test CPU @ 3.0GHz
physical id	: 0
core id		: 1

processor	: 2
model name	: Test CPU @ 3.0GHz
physical id	: 1
core id		: 0
`)
	cpu := parseCPUInfo(data)
	if cpu.ModelName != "Test CPU @ 3.0GHz" {
		t.Errorf("model = %q", cpu.ModelName)
	}
	if cpu.LogicalCores != 3 {
		t.Errorf("logical = %d", cpu.LogicalCores)
	}
	if cpu.PhysicalCores != 3 {
		t.Errorf("physical = %d, want 3 unique (phys,core) pairs", cpu.PhysicalCores)
	}
	if cpu.Threads() != 3 {
		t.Errorf("threads = %d", cpu.Threads())
	}
}

func TestParseMemInfo(t *testing.T) {
	data := []byte(`MemTotal:       32872520 kB
MemFree:         4523100 kB
MemAvailable:   24689000 kB
Buffers:          223400 kB
`)
	m := parseMemInfo(data)
	want32GB := uint64(32872520) << 10
	if m.TotalBytes != want32GB {
		t.Errorf("total = %d, want %d", m.TotalBytes, want32GB)
	}
	wantAvail := uint64(24689000) << 10
	if m.AvailableBytes != wantAvail {
		t.Errorf("available = %d, want %d (MemAvailable preferred over MemFree)",
			m.AvailableBytes, wantAvail)
	}
}

func TestParseNvidiaSMI(t *testing.T) {
	out := "NVIDIA GeForce RTX 5070, 12282, 512, 570.86, 12.0\n"
	gpus := parseNvidiaSMI(out)
	if len(gpus) != 1 {
		t.Fatalf("got %d gpus", len(gpus))
	}
	g := gpus[0]
	if g.Vendor != "nvidia" || g.Name != "NVIDIA GeForce RTX 5070" {
		t.Errorf("identity wrong: %+v", g)
	}
	if g.VRAMTotalBytes != 12282<<20 {
		t.Errorf("total = %d", g.VRAMTotalBytes)
	}
	if free := g.VRAMFreeBytes; free != (12282-512)<<20 {
		t.Errorf("free = %d", free)
	}
	if g.ComputeCapability != "12.0" {
		t.Errorf("compute cap = %q", g.ComputeCapability)
	}
	if g.DriverVersion != "570.86" {
		t.Errorf("driver = %q", g.DriverVersion)
	}
}

func TestParseROCmSMI(t *testing.T) {
	out := `{"card_list":[{"id":0,"VRAM TOTAL MEMORY (B)":17179869184,"VRAM USED MEMORY (B)":1073741824,"Vram Product Name":"AMD Radeon Test"}]}`
	gpus := parseROCmSMI(out)
	if len(gpus) != 1 {
		t.Fatalf("got %d gpus", len(gpus))
	}
	g := gpus[0]
	if g.Vendor != "amd" || g.Name != "AMD Radeon Test" {
		t.Errorf("identity wrong: %+v", g)
	}
	if g.VRAMTotalBytes != 16<<30 {
		t.Errorf("total = %d", g.VRAMTotalBytes)
	}
	if g.VRAMFreeBytes != 15<<30 {
		t.Errorf("free = %d", g.VRAMFreeBytes)
	}
}

func TestParseLspciFallback(t *testing.T) {
	out := `01:00.0 VGA compatible controller [0300]: NVIDIA Corporation AD104 [10de:2705] (rev a1)
02:00.0 VGA compatible controller [0300]: Advanced Micro Devices, Inc. [AMD/ATI] Raphael [1002:164e]
03:00.0 Ethernet controller [0200]: Realtek RTL8111 [10ec:8168]
`
	gpus := parseLspciFallback(out, nil)
	if len(gpus) != 2 {
		t.Fatalf("got %d gpus, want 2 (nvidia+amd, not ethernet)", len(gpus))
	}
	vendors := map[string]bool{}
	for _, g := range gpus {
		vendors[g.Vendor] = true
		if g.VRAMTotalBytes != 0 {
			t.Error("lspci must not fabricate VRAM numbers")
		}
	}
	if !vendors["nvidia"] || !vendors["amd"] {
		t.Errorf("vendors = %v", vendors)
	}
}

func TestFSTypeKnownValues(t *testing.T) {
	cases := map[int64]string{
		0xEF53:     "ext2/ext3/ext4",
		0x9123683E: "btrfs",
		0x58465342: "xfs",
		0x01021994: "tmpfs",
		0x69636C65: "overlayfs",
	}
	for magic, want := range cases {
		name, ok := linuxFSTypes[magic]
		if !ok || name != want {
			t.Errorf("magic %#X resolved to %q (ok=%v), want %q", magic, name, ok, want)
		}
	}
	for magic := range networkFS {
		if !networkFS[magic] {
			t.Error("networkFS values must be true")
		}
	}
	if !networkFS[0xFF534D4D] {
		t.Error("nfs must be flagged as network fs (no mmap)")
	}
}

func TestSummaryNeverPanicsWithoutGPU(t *testing.T) {
	info := &Info{OS: "linux", Arch: "amd64"}
	s := info.Summary()
	if s == "" {
		t.Error("summary empty")
	}
	if info.HasGPU() {
		t.Error("no GPU info must report HasGPU=false")
	}
}
