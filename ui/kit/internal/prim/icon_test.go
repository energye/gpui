package prim

import "testing"

func TestIcon_GlyphRegistry(t *testing.T) {
	names := IconGlyphNames()
	if len(names) != 24 {
		t.Fatalf("glyphs=%d want 24", len(names))
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("dup %q", n)
		}
		seen[n] = true
		if !IsKnownIconGlyph(n) {
			t.Fatalf("known misses %q", n)
		}
	}
	if IsKnownIconGlyph("no-such-icon-xyz") {
		t.Fatalf("unknown must miss")
	}
}

func TestIcon_LineWidth(t *testing.T) {
	if w := IconLineWidth(16); w < 1.6 || w > 2.5 {
		t.Fatalf("16px width %v outside 1.6..2.5", w)
	}
	if w := IconLineWidth(48); w <= IconLineWidth(16) {
		t.Fatalf("large glyph must use wider stroke")
	}
}
