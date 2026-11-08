package main

import (
	"fmt"
	"image"
	"math"
)

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

// 全局快照引用。
var (
	snapImage image.Image
	bxGlobal  float64
	byGlobal  float64
	dprGlobal float64
)

var probeKeys = []string{
	"parse_zero_warnings",
	"reference_loaded",
	"grade_within_close",
	"panels_alive",
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	b := ensureBaked()
	if b.err != "" {
		add("parse_zero_warnings", false, b.err)
		add("reference_loaded", false, b.err)
		add("grade_within_close", false, b.err)
		add("panels_alive", false, b.err)
		return out
	}

	add("parse_zero_warnings", len(b.doc.Warnings) == 0,
		fmt.Sprintf("warnings=%d", len(b.doc.Warnings)))
	add("reference_loaded", b.ref != nil, "resvg 0.42 reference PNG")

	// The gate metric: conformance-style grid grade vs the resvg reference.
	// ≤3 identical, ≤24 close — the whole-surface document lands in close
	// with margin (11.x at authoring time; the exact value is reported).
	g := b.grade
	add("grade_within_close", g <= 24, fmt.Sprintf("grade=%.2f (identical≤3 close≤24)", g))
	add("panels_alive", b.img != nil && b.ref != nil, "both panels baked")

	return out
}

// --- GPU snapshot pixel probes (document-fraction coords over our panel) ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_hero_radial", "pixel_badge_green", "pixel_mask_blue",
		"pixel_pattern_tile", "pixel_header_dark",
	}
	if img == nil {
		for _, k := range keys {
			add(k, false, "snapshot unavailable")
		}
		return out
	}

	// (u,v) are document fractions of the 1024×640 showcase; mapped onto
	// the OUR panel (panelAt[0]) through the display scale.
	read := func(u, v float64) ([3]float64, bool) {
		ax := bx + panelAt[0][0] + u*panelW
		ay := by + panelAt[0][1] + v*panelH
		px, py := int(math.Round(ax*dpr)), int(math.Round(ay*dpr))
		b := img.Bounds()
		if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
			return [3]float64{}, false
		}
		r, g, bl, _ := img.At(px, py).RGBA()
		return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255}, true
	}
	check := func(key string, u, v float64, want [3]float64) {
		got, ok := read(u, v)
		if !ok {
			add(key, false, "sample outside snapshot")
			return
		}
		okc := true
		for i := range want {
			if math.Abs(got[i]-want[i]) > 0.10 {
				okc = false
			}
		}
		add(key, okc, fmt.Sprintf("got=%v want≈%v", got, want))
	}

	// hero 花冠内、暗色叠加路径之外的暖白区（径向 t≈0.15）。
	gotHero, okHero := read(0.234, 0.203)
	add("pixel_hero_radial", okHero && gotHero[0] > 0.85 && gotHero[1] > 0.70 && gotHero[2] > 0.55 && gotHero[2] < 0.95,
		fmt.Sprintf("got=%v want warm-white hero", gotHero))
	// badge 绿（use 实例：徽章圆外的绿角）。
	check("pixel_badge_green", 0.371, 0.178, [3]float64{0.235, 0.812, 0.478})
	// mask 面板蓝（luminance 径向全白中心→不衰减）。
	check("pixel_mask_blue", 0.825, 0.508, [3]float64{0.29, 0.565, 0.886})
	// pattern 砖（相位绿块；整面 parity 由 grade 承载）。
	gotPatt, okPatt := read(0.678, 0.886)
	add("pixel_pattern_tile", okPatt && gotPatt[1] > 0.6 && gotPatt[0] < 0.45,
		fmt.Sprintf("got=%v want green tile", gotPatt))
	// 头带暗底（turbulence 35% 叠加后仍暗）。
	gotH, okH := read(0.5, 0.03)
	add("pixel_header_dark", okH && gotH[0] < 0.45 && gotH[1] < 0.45,
		fmt.Sprintf("got=%v want dark header", gotH))

	return out
}
