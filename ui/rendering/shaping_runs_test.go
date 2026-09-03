package rendering

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

func testShapingMultiFace(t *testing.T) text.Face {
	t.Helper()
	face, _, err := text.LoadMultiFace(16)
	if err != nil || face == nil {
		t.Skipf("LoadMultiFace unavailable: %v", err)
	}
	return face
}

// M0 item 8: Runs()∩Segments() must split same-face different-script text
// (arabic has glyphs in DejaVu, so Runs() alone yields 1 run and no shaping).
func TestIntersectRuns_ArabicLatin(t *testing.T) {
	face := testShapingMultiFace(t)
	runs := itemizeRuns("مرحبا hello", face)
	if len(runs) < 2 {
		t.Fatalf("runs=%d want ≥2 (arabic+latin must split)", len(runs))
	}
	rtl := false
	for _, r := range runs {
		if r.rtl {
			rtl = true
		}
		g := text.Shape("مرحبا hello"[r.start:r.end], r.face)
		if len(g) == 0 {
			t.Fatalf("run [%d,%d) shapes to 0 glyphs", r.start, r.end)
		}
	}
	if !rtl {
		t.Fatalf("no RTL run flagged")
	}
}

// M0 item 8: RTL runs come back in visual order with logical clusters kept.
func TestIntersectRuns_RTLVisual(t *testing.T) {
	face := testShapingMultiFace(t)
	_, _, glyphs := buildCaretsForLine("مرحبا", face)
	if len(glyphs) == 0 {
		t.Fatalf("no glyphs for arabic run")
	}
	if glyphs[0].X != 0 {
		t.Fatalf("first visual glyph X=%v want 0", glyphs[0].X)
	}
	lastLogical := 0
	for _, g := range glyphs {
		if g.Cluster > lastLogical {
			lastLogical = g.Cluster
		}
	}
	if glyphs[0].Cluster != lastLogical {
		t.Fatalf("first visual cluster=%d want last logical=%d", glyphs[0].Cluster, lastLogical)
	}
}

// M0 items 9-10: with shaping open, MultiFace layouts carry real GIDs and
// caret end matches glyph end (bulk precondition). Paint routing itself
// stays per-rune for sourceless faces until M2 (bulk+MultiFace draws blank),
// locked by TestPaintMultiFace_Ink below.
func TestPaintUsesShapedX_MultiFaceBulk(t *testing.T) {
	face := testShapingMultiFace(t)
	var b strings.Builder
	for b.Len() < 12000 {
		b.WriteString("a世界bHello你好")
	}
	lay := BuildTextLayout(b.String(), face, 16, 0, 1.2)
	if len(lay.Lines) == 0 {
		t.Fatalf("no lines")
	}
	for i, ln := range lay.Lines {
		if len(ln.Glyphs) == 0 {
			t.Fatalf("line %d: 0 glyphs", i)
		}
		if ln.Glyphs[0].GID == 0 {
			t.Fatalf("line %d: GID 0 (fallback → CPU逐字 branch)", i)
		}
		last := ln.Glyphs[len(ln.Glyphs)-1]
		glyphEnd := last.X + last.XAdvance
		caretEnd := ln.Carets[len(ln.Carets)-1].X
		if d := glyphEnd - caretEnd; d < -1 || d > 1 {
			t.Fatalf("line %d: caret end %.2f vs glyph end %.2f", i, caretEnd, glyphEnd)
		}
	}
}

// M0 item 8 熔断: caret build must be O(n) — per-rune cost stable across N.
// Ratio reading: (T(1e5)/1e5)/(T(1e3)/1e3) ≈ 1 for O(n), ≈ 100 for O(n²).
// Median-of-5 per size: robust to single-spike jitter on shared boxes, still
// trips on real quadratic blowup (100x >> 1.5 either way).
func caretBuildPerRune(t *testing.T, face text.Face, n int) float64 {
	t.Helper()
	var b strings.Builder
	for len([]rune(b.String())) < n {
		b.WriteString("a世界bHello你好")
	}
	rs := []rune(b.String())[:n]
	line := string(rs)
	_, _, _ = buildCaretsForLine(line, face)
	const reps = 5
	ds := make([]float64, 0, reps)
	for i := 0; i < reps; i++ {
		t0 := time.Now()
		_, _, _ = buildCaretsForLine(line, face)
		ds = append(ds, float64(time.Since(t0).Nanoseconds())/float64(n))
	}
	sort.Float64s(ds)
	return ds[len(ds)/2]
}

func TestCaretBuild_NoQuadratic(t *testing.T) {
	face := testShapingMultiFace(t)
	c1 := caretBuildPerRune(t, face, 1000)
	c2 := caretBuildPerRune(t, face, 10000)
	c3 := caretBuildPerRune(t, face, 100000)
	t.Logf("per-rune ns: 1e3=%.0f 1e4=%.0f 1e5=%.0f", c1, c2, c3)
	if c3/c1 > 1.5 {
		t.Fatalf("per-rune cost ratio %.2f want ≤1.5 (O(n²) signal)", c3/c1)
	}
	if c2/c1 > 1.5 {
		t.Fatalf("per-rune cost ratio %.2f want ≤1.5 (O(n²) signal)", c2/c1)
	}
}

func BenchmarkCaretBuild(b *testing.B) {
	face, _, err := text.LoadMultiFace(16)
	if err != nil || face == nil {
		b.Skipf("LoadMultiFace unavailable: %v", err)
	}
	for _, n := range []int{1000, 10000, 100000} {
		var sb strings.Builder
		for len([]rune(sb.String())) < n {
			sb.WriteString("a世界bHello你好")
		}
		rs := []rune(sb.String())[:n]
		line := string(rs)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, _ = buildCaretsForLine(line, face)
			}
		})
	}
}

// BenchmarkShapeLine compares batched run shaping (M0 item 8 path) against
// naive per-rune shaping on mixed text, for the CI trend record.
// Measured 2026-09-03 (warm S6.5): 1.9x @1e3, 2.7x @1e4 — below the plan's
// 5x premise (both arms hit the result cache; the 5x assumed uncached
// shaping). No assert: kept as trend data, not a gate.
func BenchmarkShapeLine(b *testing.B) {
	face, _, err := text.LoadMultiFace(16)
	if err != nil || face == nil {
		b.Skipf("LoadMultiFace unavailable: %v", err)
	}
	mf, ok := face.(*text.MultiFace)
	if !ok {
		b.Skipf("not a MultiFace: %T", face)
	}
	for _, n := range []int{1000, 10000} {
		var sb strings.Builder
		for len([]rune(sb.String())) < n {
			sb.WriteString("a世界bHello你好")
		}
		line := string([]rune(sb.String())[:n])
		runs := itemizeRuns(line, face)
		if len(runs) == 0 {
			b.Fatalf("no runs")
		}
		// Per-rune faces resolved once outside the timer (same as Runs()).
		fruns := mf.Runs(line)
		type rr struct {
			face text.Face
			s    string
		}
		var perRune []rr
		for _, fr := range fruns {
			for _, r := range fr.Text {
				perRune = append(perRune, rr{fr.Face, string(r)})
			}
		}
		b.Run(fmt.Sprintf("batched/n=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, r := range runs {
					_ = text.Shape(line[r.start:r.end], r.face)
				}
			}
		})
		b.Run(fmt.Sprintf("perrune/n=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, pr := range perRune {
					_ = text.Shape(pr.s, pr.face)
				}
			}
		})
	}
}

// TestPaintMultiFace_Ink locks the pixel restoration: MultiFace text must
// leave ink (bulk+MultiFace draws blank, so paint stays per-rune until M2).
func TestPaintMultiFace_Ink(t *testing.T) {
	face := testShapingMultiFace(t)
	txt := NewRenderText("hello world")
	txt.FontSize = 16
	txt.SetFace(face)
	dc := render.NewContext(400, 100)
	defer dc.Close()
	dc.BeginFrame()
	pc := NewPaintContext(dc, 1)
	txt.Paint(pc)
	img := dc.Image()
	bg := img.At(0, 0)
	ink := 0
	for y := 0; y < 100; y++ {
		for x := 0; x < 400; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			br, bg2, bb, _ := bg.RGBA()
			dr := int(r) - int(br)
			dg := int(g) - int(bg2)
			db := int(b) - int(bb)
			if dr < 0 {
				dr = -dr
			}
			if dg < 0 {
				dg = -dg
			}
			if db < 0 {
				db = -db
			}
			if dr+dg+db > 30000 {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Fatalf("MultiFace paint left no ink")
	}
}
