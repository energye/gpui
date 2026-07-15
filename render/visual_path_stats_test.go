package render

import (
	"regexp"
	"strconv"
	"testing"
)

var pathStatsRe = regexp.MustCompile(`gpu_ops=(\d+)\s+cpu_fallback_ops=(\d+)`)

// ParseRenderPathStatsLog extracts P1.0 counters from visualcmd stdout/stderr.
func ParseRenderPathStatsLog(log string) (gpuOps, cpuFallback int, ok bool) {
	m := pathStatsRe.FindStringSubmatch(log)
	if m == nil {
		return 0, 0, false
	}
	g, err1 := strconv.Atoi(m[1])
	c, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return g, c, true
}

// RequireGPUPathStats enforces P1.0 observability for GPU visual runs.
// When requireGPU is true, gpu_ops must be > 0.
func RequireGPUPathStats(t *testing.T, log string, requireGPU bool) {
	t.Helper()
	gpuOps, cpuFallback, ok := ParseRenderPathStatsLog(log)
	if !ok {
		t.Fatalf("missing path stats log line (gpu_ops/cpu_fallback_ops) in:\n%s", log)
	}
	t.Logf("path_stats gpu_ops=%d cpu_fallback_ops=%d", gpuOps, cpuFallback)
	if requireGPU && gpuOps <= 0 {
		t.Fatalf("expected gpu_ops>0 for GPU visual path, got gpu_ops=%d cpu_fallback_ops=%d", gpuOps, cpuFallback)
	}
}

func TestParseRenderPathStatsLog(t *testing.T) {
	g, c, ok := ParseRenderPathStatsLog("accelerator=sdf-gpu\ngpu_ops=12 cpu_fallback_ops=3\n")
	if !ok || g != 12 || c != 3 {
		t.Fatalf("parse = %d,%d,%v want 12,3,true", g, c, ok)
	}
}
