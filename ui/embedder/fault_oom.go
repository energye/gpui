package embedder

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

// faultOOM is the test-only OOM fault injector for the X12 power-loss path
// (§18.3 #11): the real exit chain (present fail → oomExit.Note → latch →
// quit → Run returns the human-readable reason) can only be exercised by
// feeding the present loop actual OOM-class errors, which needs either a
// dying GPU or this hook. Gated by GPUI_FAULT_OOM="after:count" — e.g.
// "60:3" injects 3 consecutive OOM-class present failures starting at the
// 60th submit. Empty/unset = fully inert (one atomic load per present).
// Default off; no production path reads this file's state.
type faultOOM struct {
	after  int64
	count  int64
	fired  atomic.Int64
	armed  bool
	inject atomic.Bool // true while the streak is being injected
}

var gpuFaultOOM = newFaultOOMFromEnvWith(os.Getenv("GPUI_FAULT_OOM"))

// newFaultOOMFromEnvWith parses the GPUI_FAULT_OOM spec ("after:count").
// Split from the env read so tests can drive the grammar directly.
func newFaultOOMFromEnvWith(spec string) *faultOOM {
	if spec == "" {
		return &faultOOM{}
	}
	parts := strings.SplitN(spec, ":", 2)
	after, err1 := strconv.ParseInt(parts[0], 10, 64)
	count := int64(1)
	if len(parts) == 2 {
		count, err1 = strconv.ParseInt(parts[1], 10, 64)
	}
	if err1 != nil || after < 0 || count <= 0 {
		return &faultOOM{}
	}
	return &faultOOM{after: after, count: count, armed: true}
}

// injectOOM returns the synthetic OOM error to substitute for this submit's
// outcome, or nil when the hook is disarmed/inert/out of budget. The streak
// is broken by any real non-OOM outcome? No — the substitution happens
// before the real outcome is noted, so the injected streak is guaranteed
// consecutive submits; real errors underneath are masked either way.
func (f *faultOOM) injectOOM(submit int64) error {
	if f == nil || !f.armed {
		return nil
	}
	if submit < f.after {
		return nil
	}
	if f.fired.Add(1) > f.count {
		return nil
	}
	f.inject.Store(true)
	return errors.New("CreateTexture failed: out of memory (GPUI_FAULT_OOM injected)")
}

// masking reports whether this submit is inside an injected streak (the
// caller keeps the injected error; instrumentation can label the frame).
func (f *faultOOM) masking() bool {
	return f != nil && f.inject.Load()
}
