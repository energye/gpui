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
		"pixel_chip_red", "pixel_chip_green", "pixel_band_top",
		"pixel_title_grad", "pixel_body_text",
		"pixel_pattern_no_seam",
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

	// hero 花冠内、暗色叠加路径之外的暖白区（径向 t≈0.15）。批 4g：烘焙
	// 改 1:1（570×357）后该分数点的渐变采样有亚像素差（B 0.556→0.549），
	// 下界 0.55→0.54（仍排除暗色叠加路径）。
	gotHero, okHero := read(0.234, 0.203)
	add("pixel_hero_radial", okHero && gotHero[0] > 0.85 && gotHero[1] > 0.70 && gotHero[2] > 0.54 && gotHero[2] < 0.95,
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

	// 批 4d 人工对照归因的五个钉（文档坐标 → 面板分数）：
	// chips：currentColor 两小方块 (769,609)/(799,609)——修复前双双黑块。
	check("pixel_chip_red", 0.751, 0.9516, [3]float64{0.886, 0.290, 0.290})
	check("pixel_chip_green", 0.780, 0.9516, [3]float64{0.235, 0.812, 0.478})
	// 反射带上段 (500,570)：渐变+opacity 0.55 混底色 ≈ 263d5a——修复前无
	// opacity（4a6983）。
	check("pixel_band_top", 0.488, 0.8906, [3]float64{0.149, 0.239, 0.353})
	// 标题渐变文字 (30,30)：titleGrad 蓝段起头，蓝主导且亮——修复前整篇
	// 文字被静默丢弃。
	gotT, okT := read(0.0293, 0.0469)
	add("pixel_title_grad", okT && gotT[2] > 0.6 && gotT[2] > gotT[0],
		fmt.Sprintf("got=%v want gradient title ink", gotT))
	// 正文文字行 (436,490)：基线 500 上方墨迹 #c8d0da（避开中线镂空）。
	gotB, okB := read(0.426, 0.7656)
	add("pixel_body_text", okB && gotB[0] > 0.6 && gotB[1] > 0.6 && gotB[2] > 0.6,
		fmt.Sprintf("got=%v want light body text", gotB))

	// pattern 格边竖缝（P3.8 批②）：tilePatt 绿带（rect 680,555 320×70 在
	// 570×357 1:1 烘焙里的设备行 313..319）逐列与左右邻列比；tile 首列若被
	// 半透明 AA 边污染会出现周期性暗列（修复前设备 x≈416,443,523,550）。
	// 判据与隔离测 TestP38PatternTileSeamGPU 同：列和均值暗 >20/行。
	seamOK, seamDetail := patternSeamFree(img, bx+panelAt[0][0], by+panelAt[0][1], dpr)
	add("pixel_pattern_no_seam", seamOK, seamDetail)

	return out
}

// patternSeamFree scans the tiled green band in the OUR panel for periodic
// darker columns (tile-boundary seams). Coordinates are bake-image device
// pixels inside the 570×357 panel; (ox, oy) is the panel image origin in the
// snapshot (logical), dpr the snapshot scale.
func patternSeamFree(img image.Image, ox, oy, dpr float64) (bool, string) {
	const (
		imgX0, imgX1 = 385, 551
		imgY0, imgY1 = 313, 319
	)
	b := img.Bounds()
	colSum := func(x int) int {
		s := 0
		for y := imgY0; y <= imgY1; y++ {
			px := int(math.Round((ox + float64(x)) * dpr))
			py := int(math.Round((oy + float64(y)) * dpr))
			if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
				return -1
			}
			r, g, bl, _ := img.At(px, py).RGBA()
			s += int(r>>8) + int(g>>8) + int(bl>>8)
		}
		return s
	}
	rows := imgY1 - imgY0 + 1
	var bad []int
	for x := imgX0 + 1; x <= imgX1-1; x++ {
		c, l, r := colSum(x), colSum(x-1), colSum(x+1)
		if c < 0 || l < 0 || r < 0 {
			return false, "sample outside snapshot"
		}
		if l-c > 20*rows && r-c > 20*rows {
			bad = append(bad, x)
		}
	}
	if len(bad) > 0 {
		return false, fmt.Sprintf("periodic seam columns=%v (want none)", bad)
	}
	return true, fmt.Sprintf("no periodic seam over x=%d..%d", imgX0, imgX1)
}
