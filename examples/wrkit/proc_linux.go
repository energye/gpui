//go:build linux

package wrkit

import (
	"os"
	"strconv"
	"strings"
)

// ProcessCPUJiffies returns process-wide utime+stime from /proc/self/stat.
// Example measurement helper only: step-curve windows need a CPU delta per
// phase, which scheduler.ProcessTracker (cumulative average) cannot provide.
// /proc/self/stat utime/stime are USER_HZ ticks (Linux default 100).
func ProcessCPUJiffies() (float64, bool) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}
	s := string(b)
	idx := strings.LastIndex(s, ") ")
	if idx < 0 {
		return 0, false
	}
	fields := strings.Fields(s[idx+2:])
	// After ") ": 0=state ... 11=utime, 12=stime (fields 14/15 1-based).
	if len(fields) < 13 {
		return 0, false
	}
	ut, err1 := strconv.ParseFloat(fields[11], 64)
	st, err2 := strconv.ParseFloat(fields[12], 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return ut + st, true
}

// ProcessCPUTicksPerSec returns Linux USER_HZ (100 on every supported
// configuration here; scheduler.proc_linux uses the same constant).
func ProcessCPUTicksPerSec() float64 { return 100 }
