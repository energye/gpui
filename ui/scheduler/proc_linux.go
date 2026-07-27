//go:build linux

package scheduler

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// ReadRSSKB returns process VmRSS in KiB from /proc/self/status, or 0 if unavailable.
func ReadRSSKB() int64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		n, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

// readCPUTime returns utime+stime jiffies from /proc/self/stat (fields 14 and 15, 1-based).
func readCPUTime() (jiffies float64, ok bool) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}
	// comm may contain spaces/parens; split after last ") ".
	s := string(b)
	idx := strings.LastIndex(s, ") ")
	if idx < 0 {
		return 0, false
	}
	fields := strings.Fields(s[idx+2:])
	// After comm: state is fields[0], utime is fields[11], stime fields[12] (0-based in this slice).
	// /proc/self/stat: pid comm state ... utime(14) stime(15) — after ") " index 0=state, 11=utime, 12=stime.
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

// clockTicks returns USER_HZ (usually 100).
func clockTicks() float64 {
	// Avoid cgo sysconf; 100 is the Linux default USER_HZ.
	return 100
}
