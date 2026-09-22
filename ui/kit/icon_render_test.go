package kit

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/prim"
)

func TestIcon_PRD_Render(t *testing.T) {
	resetIconSources()
	// ICO-04/SetSize layout square; hit == layout == paint.
	lay := ComputeIconLayout(ResolveIconSize(IconProps{Size: 24, SizeSet: true}))
	if lay.Edge != 24 {
		t.Fatalf("edge=%v want 24", lay.Edge)
	}
	if !lay.Hit(12, 12) || lay.Hit(24, 12) || lay.Hit(-1, 0) {
		t.Fatalf("hit must equal layout square")
	}
	// ICO-12 custom painter wins over name.
	in := NewIcon("custom-heart")
	in.SetState(func(i *IconInstance) {
		i.props.CustomPainter, i.props.CustomPainterID = "heart-key", "heart-key"
		i.customKey = "heart-key"
	})
	spec := in.ResolveIconPaintSpec()
	if spec.CustomKey == "" {
		t.Fatalf("custom key must survive resolve")
	}
	if IconPainterForSpec(spec) == nil {
		t.Fatalf("custom painter must exist")
	}
	// ICO-13 single offline source resolves.
	fam := CreateFromIconfont(IconfontOptions{Sources: []string{"s1"}})
	fam.Register("s1", "icon-example", "check")
	off := fam.NewIcon("icon-example")
	if off.CustomPainter == "" {
		t.Fatalf("iconfont type must map to painter")
	}
	// ICO-14 multi source: last wins; same-source second register
	// must not drop the earlier key (window font+dup share s1).
	fam.Register("s1", "icon-dup", "check")
	fam.Register("s2", "icon-dup", "close")
	dup := fam.NewIcon("icon-dup")
	if dup.CustomPainter != "close" {
		t.Fatalf("multi source last must win, got %q", dup.CustomPainter)
	}
	if again := fam.NewIcon("icon-example"); again.CustomPainter != "check" {
		t.Fatalf("same-source merge dropped icon-example, got %q", again.CustomPainter)
	}
	// Built-in registry covers all 24 P0 names.
	for _, n := range prim.IconGlyphNames() {
		if !prim.IsKnownIconGlyph(n) {
			t.Fatalf("registry misses %q", n)
		}
	}
	if prim.IsKnownIconGlyph("no-such-icon-xyz") {
		t.Fatalf("unknown must miss")
	}
	// Line width follows 16px clamp.
	if w := prim.IconLineWidth(16); w < 1.6 || w > 2.5 {
		t.Fatalf("line width %v outside 1.6..2.5", w)
	}
}
