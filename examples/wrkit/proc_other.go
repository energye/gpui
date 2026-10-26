//go:build !linux

package wrkit

// ProcessCPUJiffies is unavailable on non-Linux hosts; callers must treat
// ok=false as "no CPU curve" (never as zero CPU).
func ProcessCPUJiffies() (float64, bool) { return 0, false }

// ProcessCPUTicksPerSec returns 0 when the host has no /proc CPU source.
func ProcessCPUTicksPerSec() float64 { return 0 }
