package text

import (
	"math"
	"testing"

	"github.com/energye/gpui/render/text/emoji"
)

func p44PaintSum(n *emoji.PaintNode) float64 {
	if n == nil {
		return 0
	}
	sum := n.Alpha + n.X0 + n.Y0 + n.X1 + n.Y1 + n.X2 + n.Y2 + n.R0 + n.R1 +
		n.CenterX + n.CenterY + n.StartAngleDeg + n.EndAngleDeg +
		n.XX + n.YX + n.XY + n.YY + n.DX + n.DY + float64(n.GlyphID)
	for _, st := range n.Line.Stops {
		sum += st.Offset + st.Alpha
	}
	for _, c := range n.Children {
		sum += p44PaintSum(c.Node)
	}
	return sum + p44PaintSum(n.Child) + p44PaintSum(n.Source) + p44PaintSum(n.Backdrop)
}

// TestP44COLRv1VariableAxes pins the P4.4 contract on the real variable
// COLRv1 test font (Skia test_glyphs-glyf_colr_1_variable):
//   - default instance == static parse (deltas are zero),
//   - an axis extreme changes the paint graph (Var* deltas are applied).
func TestP44COLRv1VariableAxes(t *testing.T) {
	src, err := NewFontSourceFromFile("testdata/p44_colrv1_var.ttf")
	if err != nil {
		t.Skipf("variable COLRv1 test font missing: %v", err)
	}
	font := src.Parsed()
	vf, ok := font.(interface {
		COLRGlyphVar(uint16, int, []FontVariation) (*emoji.COLRGlyph, error)
	})
	if !ok {
		t.Fatal("ParsedFont does not expose COLRGlyphVar")
	}
	cf := font.(ColorFont)
	hi := NewFontVariation("SWPS", 90)

	scanned, varied := 0, 0
	for gid := uint16(0); gid < 300; gid++ {
		base, err := vf.COLRGlyphVar(gid, 0, nil)
		if err != nil || base == nil || base.Paint == nil {
			continue
		}
		scanned++
		if static, err := cf.COLRGlyph(gid, 0); err == nil && static != nil && static.Paint != nil {
			if d := math.Abs(p44PaintSum(base.Paint) - p44PaintSum(static.Paint)); d > 1e-9 {
				t.Fatalf("gid %d: default instance differs from static parse (d=%v)", gid, d)
			}
		}
		v, err := vf.COLRGlyphVar(gid, 0, []FontVariation{hi})
		if err != nil || v == nil || v.Paint == nil {
			continue
		}
		if math.Abs(p44PaintSum(v.Paint)-p44PaintSum(base.Paint)) > 1e-6 {
			varied++
		}
	}
	t.Logf("scanned=%d varied=%d", scanned, varied)
	if scanned == 0 {
		t.Fatal("no COLRv1 glyphs in the variable test font")
	}
	if varied == 0 {
		t.Fatal("axis extreme changed no paint field: Var* deltas are not applied")
	}
}
