package rendering

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
)

// wrapSingle builds a no-wrap WrapResult for one hard line.
func wrapSingle(s string) text.WrapResult {
	return text.WrapResult{Text: s, Start: 0, End: len(s)}
}

// 遗留#1 (§9):单行长行整形 O(L)。行内整形复用:只重整变更 run + 拼接前后缀。

// rowsEqualReuse compares an incrementally reused row against a full rebuild.
// Ints/strings/glyph IDs must match exactly; X may differ by ≤1e-9px because
// suffix X is shifted (old+dW) rather than re-accumulated: a last-ulp effect
// around 1e-12px, eight orders below the 0.5px caret gates.
func rowsEqualReuse(t *testing.T, got, want TextLayoutLine) {
	t.Helper()
	if got.StartByte != want.StartByte || got.EndByte != want.EndByte {
		t.Fatalf("行起止不一致: got [%d,%d) want [%d,%d)",
			got.StartByte, got.EndByte, want.StartByte, want.EndByte)
	}
	if len(got.Carets) != len(want.Carets) {
		t.Fatalf("caret 数不一致: got %d want %d", len(got.Carets), len(want.Carets))
	}
	for i := range got.Carets {
		if got.Carets[i].ByteOff != want.Carets[i].ByteOff {
			t.Fatalf("caret[%d] 字节偏移不一致: got %d want %d",
				i, got.Carets[i].ByteOff, want.Carets[i].ByteOff)
		}
		if math.Abs(got.Carets[i].X-want.Carets[i].X) > 1e-9 {
			t.Fatalf("caret[%d] X 超差: got %v want %v",
				i, got.Carets[i].X, want.Carets[i].X)
		}
	}
	if len(got.Glyphs) != len(want.Glyphs) {
		t.Fatalf("glyph 数不一致: got %d want %d", len(got.Glyphs), len(want.Glyphs))
	}
	for i := range got.Glyphs {
		g, w := got.Glyphs[i], want.Glyphs[i]
		if g.GID != w.GID || g.Cluster != w.Cluster || g.IsCJK != w.IsCJK {
			t.Fatalf("glyph[%d] 非坐标字段不一致: got %+v want %+v", i, g, w)
		}
		if math.Abs(g.X-w.X) > 1e-9 || math.Abs(g.XAdvance-w.XAdvance) > 1e-9 {
			t.Fatalf("glyph[%d] X 超差: got X=%v adv=%v want X=%v adv=%v",
				i, g.X, g.XAdvance, w.X, w.XAdvance)
		}
	}
	if math.Abs(got.Width-want.Width) > 1e-9 {
		t.Fatalf("行宽超差: got %v want %v", got.Width, want.Width)
	}
	if len(got.GlyphRuns) != len(want.GlyphRuns) {
		t.Fatalf("run 分区数不一致: got %d want %d", len(got.GlyphRuns), len(want.GlyphRuns))
	}
	for i := range got.GlyphRuns {
		g, w := got.GlyphRuns[i], want.GlyphRuns[i]
		if g.Start != w.Start || g.End != w.End || g.IsColor != w.IsColor ||
			g.TextStart != w.TextStart || g.TextEnd != w.TextEnd {
			t.Fatalf("grun[%d] 不一致: got %+v want %+v", i, g, w)
		}
		if g.Face != w.Face {
			t.Fatalf("grun[%d] face 不一致", i)
		}
	}
}

func TestReuseShapedRow_Equiv(t *testing.T) {
	face := m5DejaVuFace(t)
	lh := lineHeightFor(face, 16, 1.2)
	lines := map[string]string{
		"latin":     "The quick brown fox jumps over the lazy dog and then keeps running far away",
		"cjk":       "Hello世界abc你好def世界ghi",
		"arabic":    "مرحبا بالعالم hello world",
		"emoji":     "a😀b🇨🇳c👨\u200d👩\u200d👧d",
		"combining": "cafée composite école",
		"punct":     "a, b. c; d: e! f? g-h_i/j",
	}
	edits := []struct {
		name string
		at   int // rune index
		del  int // runes to delete
		ins  string
	}{
		{"subMid", 5, 1, "X"},
		{"insMid", 5, 0, "XYZ"},
		{"delMid", 5, 2, ""},
		{"subHead", 0, 1, "Q"},
		{"insHead", 0, 0, ">>"},
		{"delTail", -3, 2, ""},
		{"insTail", -1, 0, "<<"},
	}
	for lname, line := range lines {
		rs := []rune(line)
		oldRow := materializeWrappedRow(wrapSingle(line), face, lh)
		for _, e := range edits {
			at := e.at
			if at < 0 {
				at = len(rs) + at + 1
			}
			if at < 0 || at > len(rs) {
				continue
			}
			bo := 0
			for i := 0; i < at && bo < len(line); i++ {
				_, sz := utf8.DecodeRuneInString(line[bo:])
				bo += sz
			}
			ebo := bo
			for i := 0; i < e.del && ebo < len(line); i++ {
				_, sz := utf8.DecodeRuneInString(line[ebo:])
				ebo += sz
			}
			newLine := line[:bo] + e.ins + line[ebo:]
			want := materializeWrappedRow(wrapSingle(newLine), face, lh)
			got, reused := tryReuseShapedRow(oldRow, line, newLine, face, lh)
			if !reused {
				t.Fatalf("%s/%s: 长行外也应复用，实际回退", lname, e.name)
			}
			rowsEqualReuse(t, got, want)
		}
	}
}

// TestReuseShapedRow_Scale locks the legacy itself: single-char edit on a
// 50k-char single line must reuse prefix/suffix runs instead of reshaping
// the whole line.
func TestReuseShapedRow_Scale(t *testing.T) {
	face := m5DejaVuFace(t)
	lh := lineHeightFor(face, 16, 1.2)
	const n = 50000
	base := strings.Repeat("a世", n/2)
	mid := len(base) / 2
	edited := base[:mid] + "X" + base[mid+1:]
	oldRow := materializeWrappedRow(wrapSingle(base), face, lh)
	want := materializeWrappedRow(wrapSingle(edited), face, lh)
	got, reused := tryReuseShapedRow(oldRow, base, edited, face, lh)
	if !reused {
		t.Fatalf("5万字单行单字改动未复用：仍是整行重整 O(L)")
	}
	rowsEqualReuse(t, got, want)
}

// TestReuseShapedRow_Fallback documents the guard: estimate rows (nil face)
// carry no run info and must fall back instead of reusing.
func TestReuseShapedRow_Fallback(t *testing.T) {
	oldRow := materializeWrappedRow(wrapSingle("hello world"), nil, 12)
	if _, reused := tryReuseShapedRow(oldRow, "hello world", "hello WORLD", nil, 12); reused {
		t.Fatalf("无 face 估算行不应复用")
	}
}

// TestReuseCachedRow_Cap locks the memory bound: over-limit lines skip the
// segment cache and fall back to the legacy full path (slow, never wrong).
func TestReuseCachedRow_Cap(t *testing.T) {
	big := strings.Repeat("a世", segCacheMaxBytes/4+8)
	c := newLayoutCache()
	if _, _, ok := c.tryReuseCachedRow(TextLayoutLine{}, big, big+"X", nil, 12); ok {
		t.Fatalf("超限行应回退全量，不应进分段缓存")
	}
	if c.segLine != "" || c.segSegs != nil {
		t.Fatalf("回退不应留缓存")
	}
}
