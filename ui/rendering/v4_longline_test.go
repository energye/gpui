package rendering

import (
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/render/text"
)

// V4 red-light gate: docs/ENGINE_TEXT_LARGE_SCALE_PLAN.md §2.2 requires
// "字符击键 p95 ≤16ms（含 5e6 字单行）". V0 (§2.1/§6) measured 1.804s @1e6
// and 10.412s @5e6 on the production SetTextSpan+TextLayout path — 113x and
// 650x over the gate. These tests assert the gate BEFORE V4 is implemented,
// so they must be red; implementing V4 (block shape cache + prefix widths)
// turns them green.
//
// Measurement discipline (V0 §2.2 / §9): every keystroke edits a DIFFERENT
// position, so the whole-line string is unique and cannot hit the line cache
// (alternating the same position under-reports ~800x). Only the edit itself
// is timed; the restore is not.
const v4KeystrokeGate = 16 * time.Millisecond

// v4LongLine builds one logical line of n bytes from the testdata seed.
// Data lives in testdata/ (V0 §8: seed + loop index, no hardcoded standard
// data in test code).
func v4LongLine(t *testing.T, n int) string {
	t.Helper()
	b, err := os.ReadFile("testdata/v4_longline_seed.txt")
	if err != nil {
		t.Skipf("V4 red-light needs testdata seed: %v", err)
	}
	seed := strings.TrimRight(string(b), "\r\n")
	if seed == "" {
		t.Skipf("V4 testdata seed is empty")
	}
	var sb strings.Builder
	sb.Grow(n + len(seed))
	for sb.Len() < n {
		sb.WriteString(seed)
	}
	return sb.String()[:n]
}

// v4ReplaceASCII replaces one byte at/after pos, skipping UTF-8 continuation
// and lead bytes so the edit always lands on an ASCII rune. Each keystroke
// therefore produces a distinct whole-line string.
func v4ReplaceASCII(s string, pos int, ch byte) (string, int) {
	b := []byte(s)
	for pos < len(b) && b[pos] >= 0x80 {
		pos++
	}
	if pos >= len(b) {
		pos = len(b) - 1
	}
	b[pos] = ch
	return string(b), pos
}

// v4Keystroke measures ONE production keystroke on a single logical line.
func v4Keystroke(rt *RenderText, base string, pos int) time.Duration {
	edited, at := v4ReplaceASCII(base, pos, 'X')
	t0 := time.Now()
	rt.SetTextSpan(edited, at, at+1, at, at+1)
	_ = rt.TextLayout()
	d := time.Since(t0)
	// Restore to the base so the next keystroke starts clean. The base string
	// is identical every time, so this restore may hit the cache — it is
	// deliberately excluded from the gate measurement.
	rt.SetTextSpan(base, at, at+1, at, at+1)
	_ = rt.TextLayout()
	return d
}

func v4Face(t *testing.T) text.Face {
	t.Helper()
	return m5DejaVuFace(t)
}

// TestV4_SingleLineKeystroke_1e6 is the fast red-light: 1e6-char single line,
// three distinct edit positions, median of the three.
func TestV4_SingleLineKeystroke_1e6(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	_ = rt.TextLayout() // first full layout (warm-up, not gated)

	ds := make([]time.Duration, 0, 3)
	for i := 1; i <= 3; i++ {
		ds = append(ds, v4Keystroke(rt, base, i*n/4))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	med := ds[len(ds)/2]
	t.Logf("V4 1e6-char single-line keystroke median = %v (gate <=%v)", med, v4KeystrokeGate)
	if med > v4KeystrokeGate {
		t.Fatalf("V4 RED: 1e6-char keystroke %v exceeds 16ms gate (%.0fx over on this corpus). V0 §2.1 measured 1.804s on the 'a世' corpus (113x) — different corpus, reference only",
			med, float64(med)/float64(v4KeystrokeGate))
	}
}

// TestV4_SingleLineKeystroke_5e6 is the gate's headline case (plan §2.1 L3).
// V0 §8 ran only ONE unique edit at this size to stay inside the 7GB machine
// memory budget, so this test does the same: one keystroke, no repeated runs.
func TestV4_SingleLineKeystroke_5e6(t *testing.T) {
	runtime.GC()
	const n = 5000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	_ = rt.TextLayout() // first full layout (~10s / ~1.5GB, V0 §2.1), not gated

	d := v4Keystroke(rt, base, n/2)
	t.Logf("V4 5e6-char single-line keystroke = %v (gate <=%v)", d, v4KeystrokeGate)
	if d > v4KeystrokeGate {
		t.Fatalf("V4 RED: 5e6-char keystroke %v exceeds 16ms gate (%.0fx over on this corpus). V0 §2.1 measured 10.412s on the 'a世' corpus (650x) — different corpus, reference only",
			d, float64(d)/float64(v4KeystrokeGate))
	}
}
