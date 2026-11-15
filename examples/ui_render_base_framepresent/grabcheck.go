// grabcheck.go — screen-grab diff evidence: PNG-vs-PNG region compare between
// live grabs (or grab vs snapshot). All coordinates are logical 1200-wide;
// both images must share the same physical size (grab = client area).
package main

import (
	"fmt"
	"image"

	"github.com/energye/gpui/examples/wrkit"
)

// chanDiff is the ±2/-channel tolerance used by the point evidence.
func chanDiff(a, b uint32) bool {
	d := int(a) - int(b)
	if d < 0 {
		d = -d
	}
	return d > 2
}

// grabDamageEvidence reads the before/after damage grabs and reports the four
// semantic points: both patch centers changed, the gap and the legend kept.
func grabDamageEvidence(before, after string) string {
	a, err := wrkit.LoadPNG(before)
	if err != nil {
		return "err:" + err.Error()
	}
	b, err := wrkit.LoadPNG(after)
	if err != nil {
		return "err:" + err.Error()
	}
	if a.Bounds().Dx() != b.Bounds().Dx() {
		return "size-mismatch"
	}
	dpr := float64(a.Bounds().Dx()) / wrkit.DWinW
	at := func(img image.Image, lx, ly float64) (uint32, uint32, uint32) {
		r, g, bl, _ := img.At(int(lx*dpr+0.5), int(ly*dpr+0.5)).RGBA()
		return r >> 8, g >> 8, bl >> 8
	}
	changed := func(lx, ly float64) bool {
		ar, ag, ab := at(a, lx, ly)
		br, bg, bb := at(b, lx, ly)
		return chanDiff(ar, br) || chanDiff(ag, bg) || chanDiff(ab, bb)
	}
	pl, pr := damagePatchRects()
	plc := image.Point{(pl.Min.X + pl.Max.X) / 2, (pl.Min.Y + pl.Max.Y) / 2}
	prc := image.Point{(pr.Min.X + pr.Max.X) / 2, (pr.Min.Y + pr.Max.Y) / 2}
	gapX := (float64(pl.Max.X) + float64(pr.Min.X)) / 2
	gapY := (float64(pl.Min.Y) + float64(pr.Max.Y)) / 2
	res := fmt.Sprintf("patchL=%v patchR=%v gap_kept=%v legend_kept=%v",
		changed(float64(plc.X), float64(plc.Y)),
		changed(float64(prc.X), float64(prc.Y)),
		!changed(gapX, gapY),
		!changed(wrkit.DLegX+30, wrkit.DLegY+200))
	return res
}
