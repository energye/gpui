package rendering_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func sampleRGB(c color.Color) (r, g, b uint32) {
	rr, gg, bb, _ := c.RGBA()
	return rr, gg, bb
}

// TestPaintContext_PushClipRRect_ClipsOutsideCorners drives the shipped
// PaintContext.PushClipRRect / PopClip path: content drawn under a rounded
// clip must not paint the hard-rect corner (outside the rrect arc), and Pop
// must restore so later draws are unclipped.
func TestPaintContext_PushClipRRect_ClipsOutsideCorners(t *testing.T) {
	const W, H = 120, 120
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	// Origin offset so Abs() is exercised (not only 0,0).
	const ox, oy = 10.0, 10.0
	pc := rendering.NewPaintContext(dc, 1).WithOrigin(ox, oy)

	// Local rrect: (10,10)-(90,90) → absolute (20,20)-(100,100), radius 20.
	const lx, ly, lw, lh, rad = 10.0, 10.0, 80.0, 80.0, 20.0
	pc.PushClipRRect(lx, ly, lw, lh, rad)

	// Paint a solid red full local canvas; only the rrect interior may show.
	dc.SetRGBA(1, 0, 0, 1)
	ax0, ay0 := pc.Abs(0, 0)
	dc.DrawRectangle(ax0, ay0, 200, 200)
	_ = dc.Fill()

	pc.PopClip()

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}

	// Center of rrect (absolute 60,60) must be red.
	cr, cg, cb := sampleRGB(img.At(60, 60))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("center (60,60)=#%04x%04x%04x want red (clip interior)", cr, cg, cb)
	}

	// Hard-rect top-left corner of the clip bounds is outside a radius-20 rrect.
	// Absolute clip origin is (20,20); pixel (21,21) sits in the rounded corner void.
	// If PushClipRRect were a no-op or only a hard rect, this would be red (high R, low G/B).
	tr, tg, tb := sampleRGB(img.At(21, 21))
	if tg < 0xA000 || tb < 0xA000 {
		t.Fatalf("rrect corner void (21,21)=#%04x%04x%04x want near-white (not red fill)", tr, tg, tb)
	}

	// Far outside absolute clip bounds must stay white.
	or, og, ob := sampleRGB(img.At(5, 5))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (5,5)=#%04x%04x%04x want white", or, og, ob)
	}

	// After PopClip, drawing at the former corner void must paint (stack restored).
	dc.SetRGBA(0, 0, 1, 1)
	dc.DrawRectangle(20, 20, 4, 4)
	_ = dc.Fill()
	img = dc.Image()
	br, bg, bb := sampleRGB(img.At(21, 21))
	if bb < 0xC000 || br > 0x4000 || bg > 0x4000 {
		t.Fatalf("after PopClip (21,21)=#%04x%04x%04x want blue (stack restored)", br, bg, bb)
	}
}

// TestPaintContext_PushClipRRect_ZeroRadiusFallsBackToRect ensures radius<=0
// still clips to the hard rect via the same UI entry point.
func TestPaintContext_PushClipRRect_ZeroRadiusFallsBackToRect(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	pc.PushClipRRect(20, 20, 40, 40, 0)

	dc.SetRGBA(0, 0.8, 0, 1)
	dc.DrawRectangle(0, 0, 80, 80)
	_ = dc.Fill()
	pc.PopClip()

	img := dc.Image()
	gr, gg, gb := sampleRGB(img.At(40, 40))
	if gg < 0xA000 {
		t.Fatalf("inside rect clip (40,40)=#%04x%04x%04x want green", gr, gg, gb)
	}
	or, og, ob := sampleRGB(img.At(5, 5))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (5,5)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestRenderClipRRect_HitTest proves clipping is honored by hit testing (R13):
// points outside the clip are rejected, points inside hit children, and a
// child that overflows the clip is only hittable inside the clip bounds.
func TestRenderClipRRect_HitTest(t *testing.T) {
	child := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	child.SetDebugName("clipped")
	clip := rendering.NewRenderClipRRect(child)
	clip.SetRadius(4)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(clip, 10, 10)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)

	cases := []struct {
		name string
		p    rendering.Point
		want string
	}{
		{"inside hits child", rendering.Point{X: 20, Y: 20}, "clipped"},
		// (10,10) is the clip's local (0,0): inside the AABB but outside the
		// r=4 TL corner circle (d²=32>16) → rejected by exact RRect.contains
		// (Flutter parity; the old AABB test expected a hit here).
		{"corner pixel rejected", rendering.Point{X: 10, Y: 10}, ""},
		// Local (39,39) is in the BR corner quadrant at d²=18 > 16 → hole.
		{"br corner hole rejected", rendering.Point{X: 10 + 39, Y: 10 + 39}, ""},
		// Straight edge points are unaffected by corner circles.
		{"straight right edge", rendering.Point{X: 10 + 39, Y: 10 + 20}, "clipped"},
		{"outside clip x rejected", rendering.Point{X: 10 + 50, Y: 20}, ""},
		{"outside clip y rejected", rendering.Point{X: 20, Y: 10 + 50}, ""},
		{"far outside nil", rendering.Point{X: 95, Y: 95}, ""},
	}
	for _, tc := range cases {
		hit := root.HitTest(tc.p)
		if rendering.HitDebugName(hit) != tc.want {
			t.Fatalf("%s: hit=%v want %q", tc.name, rendering.HitDebugName(hit), tc.want)
		}
	}
}

// TestRenderClipRRect_HitTest_ChildOverflow proves a child larger than the
// clip is only hittable within the clip bounds (points beyond clip → nil).
func TestRenderClipRRect_HitTest_ChildOverflow(t *testing.T) {
	big := rendering.NewRenderColorBox(80, 80, 0, 0, 1, 1)
	big.SetDebugName("big")
	clip := rendering.NewRenderClipRRect(big)
	clip.FixedWidth, clip.FixedHeight = 30, 30
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(clip, 0, 0)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)

	if hit := root.HitTest(rendering.Point{X: 10, Y: 10}); rendering.HitDebugName(hit) != "big" {
		t.Fatalf("inside clip: got %q want big", rendering.HitDebugName(hit))
	}
	if hit := root.HitTest(rendering.Point{X: 40, Y: 40}); hit == root || hit == nil {
		// beyond the clip the hit must NOT land on the clipped child; the
		// AbsoluteBox ancestor (no debug name) is the legal owner.
	} else {
		t.Fatalf("beyond clip but inside child: got %v want root or nil", rendering.HitDebugName(hit))
	}
}

// TestRenderClipRRect_HitTest_CornerHole pins the exact RRect containment
// (Flutter RRect.contains): a point inside the clip's AABB but outside the
// rounded corner circle must NOT hit, while the same-distance point on the
// straight edge must hit. Radius 10 on a 40×40 clip: (1,1) is in the TL
// corner quadrant at distance² = 81+81 > 100 → rejected; (5,0) is on the top
// edge but x < r with y=0 → also corner area, distance² = 25+100 = 125 → wait,
// (5,0): center offset (-5,-10)? No — center is (r,r)=(10,10), so (5-10)²+(0-10)²
// = 25+100 = 125 > 100 → rejected too. Use (9.5, 2): (−0.5)²+(−8)² = 64.25 ≤
// 100 → accepted.
func TestRenderClipRRect_HitTest_CornerHole(t *testing.T) {
	child := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	child.SetDebugName("clipped")
	clip := rendering.NewRenderClipRRect(child)
	clip.FixedWidth, clip.FixedHeight = 40, 40
	clip.SetRadius(10)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(clip, 0, 0)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)

	cases := []struct {
		name string
		p    rendering.Point
		want string
	}{
		// TL corner hole: (1,1) — inside AABB, outside quarter circle.
		{"corner hole rejected", rendering.Point{X: 1, Y: 1}, ""},
		// Just across the corner arc boundary: center (10,10), point (4,4):
		// 36+36=72 ≤ 100 → inside.
		{"inside near corner arc", rendering.Point{X: 4, Y: 4}, "clipped"},
		// Center of the shape obviously hits.
		{"center hits", rendering.Point{X: 20, Y: 20}, "clipped"},
		// Straight-edge point near the BR corner axis: (39,20) is not in any
		// corner quadrant (y between r and h-r) → hits despite x ≥ w-r.
		{"straight edge near corner hits", rendering.Point{X: 39, Y: 20}, "clipped"},
		// BR corner hole: (39,39) — (29,29) offset from center (30,30):
		// 1+1=2 ≤ 100 → actually inside! Use the extreme (39.5,39.5):
	}
	_ = cases

	// Explicit numeric checks against the unit-circle rule:
	// TL corner hole at (1,1): offset from center (10,10) = (-9,-9),
	// d² = 162 > 100 → rejected.
	if hit := root.HitTest(rendering.Point{X: 1, Y: 1}); rendering.HitDebugName(hit) != "" {
		t.Fatalf("TL corner hole (1,1) hit %v want nil", rendering.HitDebugName(hit))
	}
	// Inside the arc at (4,4): d² = 72 ≤ 100 → child hit.
	if hit := root.HitTest(rendering.Point{X: 4, Y: 4}); rendering.HitDebugName(hit) != "clipped" {
		t.Fatalf("inside arc (4,4) got %q want clipped", rendering.HitDebugName(hit))
	}
	// BR corner hole at (39,39): offset from center (30,30) = (9,9),
	// d² = 162 > 100 → rejected.
	if hit := root.HitTest(rendering.Point{X: 39, Y: 39}); rendering.HitDebugName(hit) != "" {
		t.Fatalf("BR corner hole (39,39) hit %v want nil", rendering.HitDebugName(hit))
	}
	// Straight edge beside the corner: (39,20) — y in [r,h-r], no corner test.
	if hit := root.HitTest(rendering.Point{X: 39, Y: 20}); rendering.HitDebugName(hit) != "clipped" {
		t.Fatalf("straight edge (39,20) got %q want clipped", rendering.HitDebugName(hit))
	}
	// Arc boundary equality: (10+7.071..., 10) ≈ on the circle → still inside
	// (<=). Use (17,10): d² = 49 ≤ 100.
	if hit := root.HitTest(rendering.Point{X: 17, Y: 10}); rendering.HitDebugName(hit) != "clipped" {
		t.Fatalf("on-arc interior (17,10) got %q want clipped", rendering.HitDebugName(hit))
	}
}
