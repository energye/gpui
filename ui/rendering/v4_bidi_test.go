package rendering

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
)

// v4BidiLine builds one over-long logical line mixing RTL (Arabic, Hebrew),
// LTR (Latin) and CJK, from the testdata seed. Plan §2.3 keeps bidi at a
// line-level fallback (no cross-block re-ordering is promised), but the D1
// long-line window must still be usable: no crash, valid UTF-8, shaped
// glyphs, a truncation marker, and queryable carets.
func v4BidiLine(t *testing.T, n int) string {
	t.Helper()
	b, err := os.ReadFile("testdata/v4_bidi_seed.txt")
	if err != nil {
		t.Skipf("V4 bidi needs testdata seed: %v", err)
	}
	seed := strings.TrimRight(string(b), "\r\n")
	if seed == "" {
		t.Skipf("V4 bidi testdata seed is empty")
	}
	var sb strings.Builder
	sb.Grow(n + len(seed))
	for sb.Len() < n {
		sb.WriteString(seed)
	}
	s := sb.String()[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// TestV4_BidiLongLineFallback is the bidi probe required for §2.2 full-gate
// acceptance: an over-long RTL/LTR/CJK line on the D1 window must not panic
// (emoji.Segment / itemizeRuns used to crash on invalid UTF-8), must keep
// valid UTF-8, must produce shaped glyphs + the ellipsis marker, and must
// answer caret queries. It does NOT assert cross-block visual re-ordering
// (§2.3 non-goal).
func TestV4_BidiLongLineFallback(t *testing.T) {
	const n = 1000000
	base := v4BidiLine(t, n)
	face := v4Face(t)
	rt := NewRenderText(base)
	rt.SetFace(face)
	const visW = 904.0
	rt.SetViewportHint(0, visW)

	lay := rt.TextLayout()
	if lay == nil || lay.LineCount() == 0 {
		t.Fatalf("V4 RED: bidi long line produced no layout")
	}
	s, e, _, _, ok := lay.Line(0)
	if !ok || e <= s {
		t.Fatalf("V4 RED: bidi long line has no row")
	}
	if !utf8.ValidString(lay.Text[s:e]) {
		t.Fatalf("V4 RED: bidi window text is invalid UTF-8")
	}
	g := lay.LineGlyphs(0)
	if len(g) == 0 {
		t.Fatalf("V4 RED: bidi long line produced no glyphs")
	}
	// The marker must survive the mixed-direction run partitioning.
	var wantGID text.GlyphID
	if el := text.LayoutText(textEllipsis, face, text.LayoutOptions{}); el != nil && len(el.Lines) > 0 && len(el.Lines[0].Glyphs) > 0 {
		wantGID = el.Lines[0].Glyphs[0].GID
	}
	marker := false
	for _, gl := range g {
		if gl.GID == wantGID && gl.X >= visW-24 && gl.X <= visW+4 {
			marker = true
		}
	}
	if !marker {
		t.Fatalf("V4 RED: bidi long line lost the truncation marker (glyphs=%d)", len(g))
	}
	// Caret queries must stay functional on full-text offsets.
	if _, _, ok := lay.CaretForOffset(s + 8); !ok {
		t.Fatalf("V4 RED: bidi long line caret lookup failed")
	}
}
