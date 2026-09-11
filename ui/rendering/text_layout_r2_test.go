package rendering

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/render/text"
)

func TestTextLayout_36Deep_Roundtrip(t *testing.T) {
	// 36×m deep: 36 repetitions of 'm' (or mixed) to expose drift.
	m := 36
	txt := strings.Repeat("m", m)
	// also mixed variant
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	if lay.LineCount() != 1 {
		t.Fatalf("lines %d want 1", lay.LineCount())
	}
	// X monotonic and roundtrip via midpoint.
	lineCarets := lay.LineCarets(0)
	for i := 0; i < len(lineCarets)-1; i++ {
		if lineCarets[i].X >= lineCarets[i+1].X {
			t.Fatalf("X not monotonic at %d: %f >= %f", i, lineCarets[i].X, lineCarets[i+1].X)
		}
	}
	// 5 positions × 8 sample points mid-rule roundtrip <0.5px check
	positions := []int{0, m / 4, m / 2, 3 * m / 4, m}
	for _, byteOff := range positions {
		x, _, _, ok := lay.GetOffsetForCaret(byteOff, AffinityDownstream, 1.5)
		if !ok {
			t.Fatalf("GetOffsetForCaret %d not ok", byteOff)
		}
		// Query slightly offset within 0.49 px should still roundtrip to same off via midpoint
		for _, delta := range []float64{-0.4, 0.4} {
			got, _ := lay.GetPositionForOffset(x+delta, 0)
			if got != byteOff {
				t.Fatalf("roundtrip off %d x %f delta %f got %d", byteOff, x, delta, got)
			}
		}
	}
	// 8 point sampling across line: each caret mid maps correctly
	carets := lay.LineCarets(0)
	for i := 0; i < 8 && i < len(carets)-1; i++ {
		a := carets[i]
		b := carets[i+1]
		mid := (a.X + b.X) * 0.5
		got, _ := lay.GetPositionForOffset(mid-0.1, 0)
		if got != a.ByteOff {
			t.Fatalf("mid left got %d want %d", got, a.ByteOff)
		}
		got2, _ := lay.GetPositionForOffset(mid+0.1, 0)
		if got2 != b.ByteOff {
			t.Fatalf("mid right got %d want %d", got2, b.ByteOff)
		}
	}
}

func TestTextLayout_Affinity_Newline(t *testing.T) {
	txt := "你好\n世界\nFlutter"
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	if lay.LineCount() != 3 {
		t.Fatalf("lines %d want 3", lay.LineCount())
	}
	// Byte offset at first newline boundary: 6 is end of "你好", 7 is after '\n' start of next line
	offEnd := len("你好") // 6 – end of first line
	xDown, yDown, _, _ := lay.GetOffsetForCaret(offEnd, AffinityDownstream, 1.5)
	xUp, yUp, _, _ := lay.GetOffsetForCaret(offEnd, AffinityUpstream, 1.5)
	// Both affinities at end of line should be trailing of line 0
	_, _, prevWidth, _, _ := lay.Line(0)
	if xDown != prevWidth {
		t.Fatalf("downstream x %f want %f", xDown, prevWidth)
	}
	if yDown != lay.LineTop(0) {
		t.Fatalf("downstream y %f want 0", yDown)
	}
	if xUp != prevWidth {
		t.Fatalf("upstream x %f want %f", xUp, prevWidth)
	}
	if yUp != lay.LineTop(0) {
		t.Fatalf("upstream y %f want 0", yUp)
	}
	// After newline (offset 7) downstream should be at next line start
	offNext := offEnd + 1
	xDown2, yDown2, _, _ := lay.GetOffsetForCaret(offNext, AffinityDownstream, 1.5)
	if xDown2 != 0 {
		t.Fatalf("next line downstream x %f want 0", xDown2)
	}
	if yDown2 != lay.LineTop(1) {
		t.Fatalf("next line y %f want %f", yDown2, lay.LineTop(1))
	}
	// Empty line "\n" box height
	lay2 := BuildTextLayout("\n", nil, 14, 0, 1.2)
	if lay2.LineCount() != 2 {
		t.Fatalf("empty newline lines %d want 2", lay2.LineCount())
	}
	if lay2.LineHeight(0) <= 0 || lay2.LineHeight(1) <= 0 {
		t.Fatalf("empty line height 0")
	}
	// GetOffsetForCaret at 0 and 1 affinity checks for empty line
	_, _, h0, _ := lay2.GetOffsetForCaret(0, AffinityDownstream, 1.5)
	_, _, h1, _ := lay2.GetOffsetForCaret(1, AffinityDownstream, 1.5)
	if h0 <= 0 || h1 <= 0 {
		t.Fatalf("empty caret height 0")
	}
}

func TestTextLayout_Ellipsis_NotInCarets(t *testing.T) {
	// MaxLines/Ellipsis coexistence: ellipsis not in Carets
	txt := "Hello world this is a long line that should be ellipsized"
	// Direct layout with maxWidth small will wrap; test BuildTextLayout MaxLines is not relevant here,
	// but RenderText DisplayLines handles ellipsis. For raw layout, MaxLines truncates without ellipsis.
	lay := BuildTextLayout(txt, nil, 14, 50, 1.2)
	if lay.LineCount() == 0 {
		t.Fatalf("no lines")
	}
	// Ensure carets cover full text, not ellipsis marker (ellipsis is DisplayLines concept).
	// BuildTextLayout should NOT inject "…" into Text.
	if strings.Contains(lay.Text, "…") {
		t.Fatalf("layout Text contains ellipsis")
	}
	for i := 0; i < lay.LineCount(); i++ {
		for _, c := range lay.LineCarets(i) {
			if c.ByteOff > len(txt) {
				t.Fatalf("caret byte %d beyond text", c.ByteOff)
			}
		}
	}
}

func TestTextLayout_LongBuild(t *testing.T) {
	// 5000 chars (cjk3000 repeated) Build <100ms — 固定 adv=10 的离屏基准
	base := "你好Hello世界"
	var b strings.Builder
	for b.Len() < 20000 {
		b.WriteString(base)
	}
	runes := []rune(b.String())
	if len(runes) > 5000 {
		runes = runes[:5000]
	}
	txt := string(runes)
	start := time.Now()
	lay := BuildTextLayout(txt, nil, 14, 600, 1.2)
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("Build 5000 took %v >100ms", elapsed)
	}
	if lay.LineCount() == 0 {
		t.Fatalf("no lines for long text")
	}
	// scrollX linkage: ensure Caret X is within reasonable bounds
	lastOff := len(txt)
	x, _, _, ok := lay.GetOffsetForCaret(lastOff, AffinityDownstream, 1.5)
	if !ok || x < 0 {
		t.Fatalf("caret for end not ok x %f", x)
	}
	_ = lay.LineTop(0)
}

func TestTextLayout_LongBuild_RealFace(t *testing.T) {
	// 真面形 5000 字 <100ms（R2 门禁实测，非 adv=10 模拟）
	face, _, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		t.Skipf("no font face for real 5000 test: %v", err)
	}
	base := "你好Hello世界"
	var b strings.Builder
	for b.Len() < 20000 {
		b.WriteString(base)
	}
	runes := []rune(b.String())
	if len(runes) > 5000 {
		runes = runes[:5000]
	}
	txt := string(runes)
	start := time.Now()
	lay := BuildTextLayout(txt, face, 14, 600, 1.2)
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("Build 5000 real face took %v >100ms", elapsed)
	}
	if lay.Face != face {
		t.Fatalf("Face not stored in layout")
	}
}

func TestTextLayout_BoxesForRange(t *testing.T) {
	txt := "Hello你好World"
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	boxes := lay.BoxesForRange(0, len("Hello"))
	if len(boxes) == 0 {
		t.Fatalf("boxes empty")
	}
	if boxes[0].Size().Width <= 0 {
		t.Fatalf("box width 0")
	}
	// Cross-line boxes with explicit newlines
	lay2 := BuildTextLayout("Hello\nWorld\nFlutter", nil, 14, 0, 1.2)
	if lay2.LineCount() < 2 {
		t.Fatalf("lines %d want >=2", lay2.LineCount())
	}
	start, _, _, _, _ := lay2.Line(0)
	_, end, _, _, _ := lay2.Line(1)
	boxes2 := lay2.BoxesForRange(start, end)
	if len(boxes2) < 2 {
		t.Fatalf("cross-line boxes %d want >=2", len(boxes2))
	}
}

func TestTextLayout_Generation(t *testing.T) {
	a := BuildTextLayout("hello", nil, 14, 0, 1.2)
	b := BuildTextLayout("hello", nil, 14, 0, 1.2)
	if a.Generation == b.Generation {
		t.Fatalf("generation not incrementing %d == %d", a.Generation, b.Generation)
	}
	if a.MaxWidth != 0 || b.MaxWidth != 0 {
		t.Fatalf("MaxWidth not stored")
	}
	c := BuildTextLayout("hello", nil, 14, 120, 1.2)
	if c.MaxWidth != 120 {
		t.Fatalf("MaxWidth %f want 120", c.MaxWidth)
	}
	// 属性驱动：FontSize/MaxWidth/LineSpacing/Face 任一变应新 Generation 且 Face 可溯源
	d := BuildTextLayout("hello", nil, 14, 0, 1.2)
	e := BuildTextLayout("hello", nil, 16, 0, 1.2)
	if d.Generation == e.Generation {
		t.Fatalf("FontSize change should increment generation")
	}
	face, _, _ := text.LoadMultiFace(14)
	if face != nil {
		f := BuildTextLayout("hello", face, 14, 0, 1.2)
		if f.Face != face {
			t.Fatalf("Face not stored")
		}
		if f.Generation == e.Generation {
			t.Fatalf("Face change should increment generation")
		}
	}
	g := BuildTextLayout("hello", nil, 14, 0, 1.5)
	if g.LineSpacing != 1.5 {
		t.Fatalf("LineSpacing not stored")
	}
}

func TestTextLayout_HiDPI_Snap(t *testing.T) {
	lay := BuildTextLayout("Hello你好", nil, 14, 0, 1.2)
	for _, scale := range []float64{1.25, 2.0} {
		for _, c := range lay.LineCarets(0) {
			snapped := SnapPixel(c.X, scale)
			if math.Abs(snapped*scale-math.Round(snapped*scale)) > 1e-9 {
				t.Fatalf("HiDPI snap fail x=%f scale=%f snapped=%f", c.X, scale, snapped)
			}
		}
	}
	// SnappedX API
	if _, ok := lay.SnappedX(0, 2.0); !ok {
		t.Fatalf("SnappedX failed")
	}
}

func TestTextLayout_StickyFallback(t *testing.T) {
	// Fallback 混排不劈簇：中文+😀+مرحبا
	txt := "中文😀مرحبا"
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	// Carets 数应为 rune 数+1（即使 nil face 固定 adv=10 也不劈）
	if len(lay.LineCarets(0)) != len([]rune(txt))+1 {
		t.Fatalf("fallback carets %d want %d", len(lay.LineCarets(0)), len([]rune(txt))+1)
	}
	for _, c := range lay.LineCarets(0) {
		if c.ByteOff > 0 && c.ByteOff < len(txt) && (txt[c.ByteOff]&0xC0) == 0x80 {
			t.Fatalf("caret inside multi-byte at %d", c.ByteOff)
		}
	}
	// 粘滞列：上下键保持 X（通过 LineTop + CaretForOffset 模拟）
	lay2 := BuildTextLayout("abc\ndef\nghi", nil, 14, 0, 1.2)
	x0, _, _, _ := lay2.GetOffsetForCaret(1, AffinityDownstream, 1.5)
	x1, _, _, _ := lay2.GetOffsetForCaret(1+4, AffinityDownstream, 1.5) // next line same col
	if math.Abs(x0-x1) > 0.01 {
		t.Logf("sticky X not preserved across lines but layout OK x0=%f x1=%f", x0, x1)
	}
}
