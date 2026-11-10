//go:build !nogpu

package gpu

import (
	"fmt"
	"os"
	"time"
)

// P4.3 stage-1 cost attribution: env-gated per-segment timing inside
// runGPUFilterGraphEx. Off by default; removed after P4.3 acceptance.
var filterGraphTimingOn = os.Getenv("GPUI_P43_TIMING") == "1"

type fgTiming struct {
	Setup, Upload, Encode, Submit, Map time.Duration
}

func fgNow() time.Time                  { return time.Now() }
func fgSince(t time.Time) time.Duration { return time.Since(t) }
func fgMs(d time.Duration) float64      { return d.Seconds() * 1000 }

func fgEmit(w, h int, wantPixels bool, tm *fgTiming, total time.Duration) {
	if !filterGraphTimingOn {
		return
	}
	fmt.Fprintf(os.Stderr,
		"P43TIMING graph w=%d h=%d wantPixels=%v setup=%.3fms upload=%.3fms encode=%.3fms submit=%.3fms map=%.3fms total=%.3fms\n",
		w, h, wantPixels,
		fgMs(tm.Setup), fgMs(tm.Upload), fgMs(tm.Encode), fgMs(tm.Submit), fgMs(tm.Map), fgMs(total))
}
