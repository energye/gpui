package render

import (
	"fmt"
	"os"
	"time"
)

// P4.3 stage-1 cost attribution (2026-11-10). GPUI_P43_TIMING=1 emits one
// "P43TIMING ..." line per measured segment to stderr. Off by default.
// Temporary diagnostic: removed after P4.3 acceptance; the captured data and
// the attribution live in evidence/P4.3/stage1-timing/.
var p43TimingOn = os.Getenv("GPUI_P43_TIMING") == "1"

func p43Now() time.Time                  { return time.Now() }
func p43Since(t time.Time) time.Duration { return time.Since(t) }
func p43Ms(d time.Duration) float64      { return d.Seconds() * 1000 }

func p43Log(format string, args ...any) {
	if p43TimingOn {
		fmt.Fprintf(os.Stderr, "P43TIMING "+format+"\n", args...)
	}
}
