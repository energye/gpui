package main

import (
	"encoding/base64"
	"fmt"
	"image"
	"math"
	"os"
	"strings"

	"github.com/energye/gpui/render/svg"
)

// 全局快照引用（main 里 loadImage 后注入，供 alpha 类探针用）。
var (
	snapImage image.Image
	bxGlobal  float64
	byGlobal  float64
	dprGlobal float64
)

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

var probeKeys = []string{
	"grad_spread_three",
	"grad_focal_parse",
	"grad_stroke_transform",
	"clip_union_and_ring",
	"mask_luminance_parse",
	"pattern_tile_parse",
	"image_data_uri",
	"paint_fail_closed",
}

// Shared expected colors (0..1 floats, matching the card SVG hex sources).
var (
	colCardInk = hex01("#171a20")
	colRed     = hex01("#e24a4a")
	colBlue    = hex01("#4a90e2")
)

func hex01(s string) [3]float64 {
	var r, g, b uint8
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return [3]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255}
}

func nearRGB(got, want [3]float64, tol float64) bool {
	for i := range got {
		if math.Abs(got[i]-want[i]) > tol {
			return false
		}
	}
	return true
}

func patternImageSVGResolved() string {
	raw, err := os.ReadFile("render/svg/testdata/p38/img_rgba.png")
	if err != nil {
		return patternImageSVG
	}
	return strings.Replace(patternImageSVG, "__DATA_URI__", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(raw), 1)
}

// --- value-level logic probes (CPU subprocess, GOGPU_RENDER_MODE=cpu) ---

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	srcs := [6]string{gradSpreadSVG, radialFocalSVG, gradTransformSVG, clipSVG, maskSVG, patternImageSVGResolved()}
	docs := make([]*svg.Document, 6)
	parseOK := true
	for i, src := range srcs {
		d, err := svg.Parse([]byte(src))
		if err != nil || d == nil {
			add("parse_all_six", false, fmt.Sprintf("card %d: %v", i, err))
			parseOK = false
			continue
		}
		docs[i] = d
	}
	if parseOK {
		add("parse_all_six", true, "6 docs parsed")
	} else {
		return out
	}

	// 1) 三 spread：pad/repeat/reflect + href 继承。
	defs := docs[0].Elements[1].(*svg.DefsElement)
	l0 := defs.Children[0].(*svg.LinearGradientElement)
	l1 := defs.Children[1].(*svg.LinearGradientElement)
	l2 := defs.Children[2].(*svg.LinearGradientElement)
	ok3 := l0.Spread == svg.SpreadPad && l1.Spread == svg.SpreadRepeat && l2.Spread == svg.SpreadReflect &&
		l1.HRef == "#pad" && l2.HRef == "#pad"
	add("grad_spread_three", ok3, fmt.Sprintf("pad=%v rep=%v(%q) ref=%v", l0.Spread, l1.Spread, l1.HRef, l2.Spread))

	// 2) 径向焦点。
	defs1 := docs[1].Elements[1].(*svg.DefsElement)
	r0 := defs1.Children[0].(*svg.RadialGradientElement)
	add("grad_focal_parse", r0.HasFocus && r0.FX.Value == 110 && r0.CX.Value == 172 && r0.R.Value == 150,
		fmt.Sprintf("fx=%v cx=%v r=%v", r0.FX, r0.CX, r0.R))

	// 3) 渐变描边卡：tilted 继承 + gradientTransform。
	defs2 := docs[2].Elements[1].(*svg.DefsElement)
	tilted := defs2.Children[1].(*svg.LinearGradientElement)
	add("grad_stroke_transform", tilted.HRef == "#ramp" && tilted.Transform == "rotate(90 172 126)",
		fmt.Sprintf("href=%q transform=%q", tilted.HRef, tilted.Transform))

	// 4) clipPath 两枚。
	defs3 := docs[3].Elements[1].(*svg.DefsElement)
	cp0 := defs3.Children[0].(*svg.ClipPathElement)
	cp1 := defs3.Children[1].(*svg.ClipPathElement)
	add("clip_union_and_ring", len(cp0.Children) == 2 && cp0.Units == svg.UnitsUserSpaceOnUse && len(cp1.Children) == 1,
		fmt.Sprintf("two=%d ring=%d", len(cp0.Children), len(cp1.Children)))

	// 5) mask 三段。
	defs4 := docs[4].Elements[1].(*svg.DefsElement)
	mk := defs4.Children[0].(*svg.MaskElement)
	add("mask_luminance_parse", len(mk.Children) == 3 && mk.Units == svg.UnitsUserSpaceOnUse,
		fmt.Sprintf("children=%d units=%v", len(mk.Children), mk.Units))

	// 6) pattern tile。
	defs5 := docs[5].Elements[1].(*svg.DefsElement)
	pt := defs5.Children[0].(*svg.PatternElement)
	add("pattern_tile_parse", len(pt.Children) == 4 && pt.W.Value == 16,
		fmt.Sprintf("children=%d w=%v", len(pt.Children), pt.W))

	// 7) image data URI（Elements: [bg rect, defs, fill rect, image]）。
	imgEl := docs[5].Elements[3].(*svg.ImageElement)
	add("image_data_uri", imgEl.HasImage && imgEl.ImgW == 4 && imgEl.ImgH == 2,
		fmt.Sprintf("has=%v %dx%d", imgEl.HasImage, imgEl.ImgW, imgEl.ImgH))

	// 8) fail-closed：六卡零 Warning。
	total := 0
	for _, d := range docs {
		total += len(d.Warnings)
	}
	add("paint_fail_closed", total == 0, fmt.Sprintf("warnings=%d", total))

	return out
}

// --- GPU snapshot pixel probes (card-local coords, +innerX/innerY offsets) ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_grad_pad_left", "pixel_grad_pad_right",
		"pixel_grad_rep_restart", "pixel_grad_ref_mirror",
		"pixel_radial_hot", "pixel_radial_far",
		"pixel_tfill_left", "pixel_stroke_top",
		"pixel_clip_union_mid", "pixel_clip_ring_hole", "pixel_clip_ring_band",
		"pixel_mask_white", "pixel_mask_black", "pixel_mask_gray",
		"pixel_pattern_red", "pixel_pattern_blue", "pixel_image_left", "pixel_image_right",
	}
	if img == nil {
		for _, k := range keys {
			add(k, false, "snapshot unavailable")
		}
		return out
	}
	snapImage, bxGlobal, byGlobal, dprGlobal = img, bx, by, dpr

	read := func(card int, x, y float64) ([3]float64, bool) {
		ax := bx + cardAt[card][0] + innerX + x
		ay := by + cardAt[card][1] + innerY + y
		px, py := int(math.Round(ax*dpr)), int(math.Round(ay*dpr))
		return pixelAt(img, px, py)
	}
	check := func(key string, card int, x, y float64, want [3]float64) {
		got, ok := read(card, x, y)
		if !ok {
			add(key, false, "sample outside snapshot")
			return
		}
		add(key, nearRGB(got, want, 0.07), fmt.Sprintf("got=%v want=%v", got, want))
	}

	// 卡 1：端点+位置语义钉（spread 语义核心是位置；stop 中段色彩域
	// 的 GPU/CPU 差异属 P4.2「GPU/CPU 模糊一致」范围，不在批 2 硬钉）。
	// pad 带左端近红、右端蓝尾；repeat 在周期 90 重启（x=92 近红）；
	// reflect 镜像点 x=110 → t=0.778（线性域中段，GPU 已对拍一致）。
	// 线性域手算：x=12 → t=0.133 → (srgb(0.867),0,srgb(0.133))≈(0.94,0,0.42)；
	// x=92 → t=0.022 → ≈(0.99,0,0.17)。线性域下蓝分量在近端已显著（非纯 stop 色）。
	// 线性域精确期望（红 stop #e24a4a 的 r_lin=0.760，非 1.0）：
	// x=12 → t=0.133 → (srgb(0.659),srgb(0.0968),srgb(0.160))≈(0.835,0.341,0.435)；
	// x=92 → t=0.022 → ≈(0.883,0.30,0.32)。
	check("pixel_grad_pad_left", 0, 12, 46, [3]float64{0.835, 0.341, 0.435})
	check("pixel_grad_pad_right", 0, 330, 46, colBlue)
	check("pixel_grad_rep_restart", 0, 92, 126, [3]float64{0.883, 0.30, 0.32})
	check("pixel_grad_ref_mirror", 0, 110, 206, hex01("#6d80bf")) // resvg 0.42 实测（4c-4 sRGB stop 插值后；旧 #8277cc 为旧插值口径）

	// 卡 2：热点 (110,90) 近白；右下角 (330,240) 近黑。
	check("pixel_radial_hot", 1, 110, 90, hex01("#ffffff"))
	check("pixel_radial_far", 1, 330, 240, hex01("#000000"))

	// 卡 3：填充带左端近红（端点策略）；描边带顶 band y∈[128,144) 取 138
	// （内缘 144 不属带——批 1 同款教训）。
	check("pixel_tfill_left", 2, 12, 66, colRed)
	mid, okMid := read(2, 172, 138)
	add("pixel_stroke_top", okMid && !nearRGB(mid, colCardInk, 0.05),
		fmt.Sprintf("got=%v want non-bg", mid))

	// 卡 4：并集中缝仍画；环洞心透明；环带红。
	check("pixel_clip_union_mid", 3, 135, 66, colBlue)
	// 洞内露出的是卡底色 #171a20（bg rect 不透明）——探色不探 alpha。
	gotHole, okHole := read(3, 172, 192)
	add("pixel_clip_ring_hole", okHole && nearRGB(gotHole, colCardInk, 0.05),
		fmt.Sprintf("got=%v want card bg", gotHole))
	check("pixel_clip_ring_band", 3, 60, 192, colRed)

	// 卡 5：luminance 三段。
	check("pixel_mask_white", 4, 60, 126, colBlue)
	// 黑段内容被隐 → 露出卡底色 #171a20（探色不探 alpha）。
	gotB, okB := read(4, 172, 126)
	add("pixel_mask_black", okB && nearRGB(gotB, colCardInk, 0.05),
		fmt.Sprintf("got=%v want card bg", gotB))
	got, okG := read(4, 280, 126)
	add("pixel_mask_gray", okG && got[2] > 0.02 && got[2] < colBlue[2]-0.05,
		fmt.Sprintf("got=%v want dimmer blue", got))

	// 卡 6：pattern 棋盘红/蓝格 + image data URI 左红右蓝。
	check("pixel_pattern_red", 5, 14, 20, colRed)
	check("pixel_pattern_blue", 5, 22, 28, colBlue)
	// meet 内容带 x∈[102,242)（左右 letterbox 各 20px）——探点落带内。
	check("pixel_image_left", 5, 110, 200, colRed)
	check("pixel_image_right", 5, 240, 200, colBlue)

	return out
}

func readAlpha(card int, x, y float64) uint8 {
	if snapImage == nil {
		return 255
	}
	ax := bxGlobal + cardAt[card][0] + innerX + x
	ay := byGlobal + cardAt[card][1] + innerY + y
	px, py := int(math.Round(ax*dprGlobal)), int(math.Round(ay*dprGlobal))
	b := snapImage.Bounds()
	if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
		return 255
	}
	_, _, _, a := snapImage.At(px, py).RGBA()
	return uint8(a >> 8)
}

func pixelAt(img image.Image, x, y int) ([3]float64, bool) {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return [3]float64{}, false
	}
	r, g, bl, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255}, true
}
