//go:build !linux

package scheduler

// ReadRSSKB is unavailable off Linux; returns 0.
func ReadRSSKB() int64 { return 0 }

func readCPUTime() (jiffies float64, ok bool) { return 0, false }

func clockTicks() float64 { return 100 }
