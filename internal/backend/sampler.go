package backend

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// vramPoller and rssReader are seams for tests.
var (
	vramPoller = pollVRAMUsed
	rssReader  = readProcessRSS
)

// sampler polls VRAM/RSS while the child process runs. Polling is best-effort;
// failures leave metrics unknown instead of fabricating numbers.
type sampler struct {
	mu        sync.Mutex
	pid       int
	stop      chan struct{}
	done      sync.WaitGroup
	cancelRun context.CancelFunc

	peakVRAM uint64
	peakRAM  uint64
	baseline uint64
}

func startSampler(cancelRun context.CancelFunc) *sampler {
	s := &sampler{stop: make(chan struct{}), cancelRun: cancelRun}
	s.baseline, _ = vramPoller()
	s.done.Add(1)
	go func() {
		defer s.done.Done()
		s.loop()
	}()
	return s
}

func (s *sampler) SetPid(pid int) {
	s.mu.Lock()
	s.pid = pid
	s.mu.Unlock()
}

func (s *sampler) Stop() {
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
	s.done.Wait()
}

func (s *sampler) Peaks() (vram, ram uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peakVRAM > s.baseline {
		vram = s.peakVRAM - s.baseline
	}
	return vram, s.peakRAM
}

func (s *sampler) loop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if used, ok := vramPoller(); ok {
				s.mu.Lock()
				if used > s.peakVRAM {
					s.peakVRAM = used
				}
				s.mu.Unlock()
			}
			s.mu.Lock()
			pid := s.pid
			s.mu.Unlock()
			if pid > 0 {
				if rss, ok := rssReader(pid); ok {
					s.mu.Lock()
					if rss > s.peakRAM {
						s.peakRAM = rss
					}
					s.mu.Unlock()
				}
			}
		}
	}
}

// pollVRAMUsed sums memory.used across NVIDIA GPUs via nvidia-smi.
func pollVRAMUsed() (uint64, bool) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=memory.used", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return 0, false
	}
	var total uint64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		mib, err := strconv.ParseUint(line, 10, 64)
		if err != nil {
			continue
		}
		total += mib << 20
	}
	if total == 0 {
		return 0, false
	}
	return total, true
}
