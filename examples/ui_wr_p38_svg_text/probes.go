package main

import (
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render/svg"
)

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

// 全局快照引用（main 里 loadImage 后注入，供探针采样）。
var (
	snapImage image.Image
	bxGlobal  float64
	byGlobal  float64
	dprGlobal float64
)

var probeKeys = []string{
	"parse_all_four",
	"anchor_parse",
	"tspan_parse",
	"clip_mask_on_text",
	"rotate_parse",
	"text_fail_closed",
}

var (
	colCardInk = hex01("#171a20")
	colRed     = hex01("#e24a4a")
	colBlue    = hex01("#4a90e2")
	colGreen   = hex01("#3ccf7a")
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

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	srcs := [4]string{textAnchorSVG, tspanSVG, textPaintSVG, textRotateSVG}
	docs := make([]*svg.Document, 4)
	parseOK := true
	for i, src := range srcs {
		d, err := svg.Parse([]byte(src))
		if err != nil || d == nil {
			add("parse_all_four", false, fmt.Sprintf("card %d: %v", i, err))
			parseOK = false
			continue
		}
		docs[i] = d
	}
	if parseOK {
		add("parse_all_four", true, "4 docs parsed")
	} else {
		return out
	}

	// 按类型扫描（卡 3 有 defs 注入，索引不固定）。
	textsOf := func(d *svg.Document) []*svg.TextElement {
		var out []*svg.TextElement
		var walk func(elems []svg.Element)
		walk = func(elems []svg.Element) {
			for _, e := range elems {
				switch t := e.(type) {
				case *svg.TextElement:
					out = append(out, t)
				case *svg.GroupElement:
					walk(t.Children)
				case *svg.DefsElement:
					walk(t.Children)
				}
			}
		}
		walk(d.Elements)
		return out
	}

	// 1) anchor 三态 + 多值属性解析。
	t0 := textsOf(docs[0])
	add("anchor_parse", len(t0) == 3 && t0[0].X != nil && t0[2].Attrs.TextAnchor == "end",
		fmt.Sprintf("texts=%d anchor2=%q", len(t0), t0[2].Attrs.TextAnchor))

	// 2) tspan：dx/x,dy 列表 + 子元素数。
	t1 := textsOf(docs[1])
	add("tspan_parse", len(t1) >= 1 && len(t1[0].Children) == 2,
		fmt.Sprintf("children=%d", len(t1[0].Children)))

	// 3) 文字 paint 管线：第三卡 text 带 clip-path + mask。
	t2 := textsOf(docs[2])
	okCM := false
	for _, te := range t2 {
		if te.Attrs.ClipPath == "url(#half)" {
			okCM = true
		}
	}
	okMask := false
	for _, te := range t2 {
		if te.Attrs.Mask == "url(#lum)" {
			okMask = true
		}
	}
	add("clip_mask_on_text", okCM && okMask,
		fmt.Sprintf("clip=%v mask=%v texts=%d", okCM, okMask, len(t2)))

	// 4) rotate 逐字符。
	t3 := textsOf(docs[3])
	okRot := false
	for _, te := range t3 {
		if len(te.Rotate) == 2 && te.Rotate[0] == 45 {
			okRot = true
		}
	}
	add("rotate_parse", okRot, fmt.Sprintf("texts=%d", len(t3)))

	// 5) fail-closed：缺字体卡带 unsupported-ref Warning。
	total := 0
	for _, d := range docs {
		total += len(d.Warnings)
	}
	add("text_fail_closed", total >= 1, fmt.Sprintf("warnings=%d", total))

	return out
}

// --- GPU snapshot pixel probes (card-local coords, +innerX/innerY offsets) ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_anchor_start", "pixel_anchor_mid", "pixel_anchor_end",
		"pixel_tspan_dx", "pixel_tspan_xy", "pixel_spacing",
		"pixel_grad_zone", "pixel_clip_zone", "pixel_mask_hidden",
		"pixel_rotated_ink", "pixel_missing_refused",
	}
	if img == nil {
		for _, k := range keys {
			add(k, false, "snapshot unavailable")
		}
		return out
	}

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

	// 卡 1（anchor）：AHEM 24px 方块，基线 60 ⇒ 方块 y∈[52,76)。
	// start x=20：块1 [20,44)；middle x=264 行宽 48 → 起点 240：块 [240,288)；
	// end x=508 行宽 48 → 起点 460：块 [460,508)。
	check("pixel_anchor_start", 0, 30, 44, colRed)
	check("pixel_anchor_mid", 0, 260, 114, colBlue)
	check("pixel_anchor_end", 0, 480, 184, colGreen)

	// 卡 2（tspan）：主行基线 60；'b' dx=30 → 起点 74（'a' [20,44) + 30）。
	check("pixel_tspan_dx", 1, 80, 44, colGreen)
	// 'c' x=300 dy=40 → 基线 100、x=300（盒 80.8..104.8，采样避中线镂空 89.2..90.8）。
	check("pixel_tspan_xy", 1, 310, 84, colBlue)
	// letter-spacing 12：'h' [20,44)、'i' [56,80)（基线 180，盒 160.8..184.8）。
	check("pixel_spacing", 1, 62, 164, hex01("#f0c000"))

	// 卡 3（paint 管线）：绿 text 盖 x<120；渐变文字在 x>120 露出蓝段；
	// mask 黑段 x∈[284,528) 隐藏 → 卡底。
	check("pixel_clip_zone", 2, 60, 84, colGreen)
	// 渐变 'abcdefgh' 尾 [116,212)：clip 外取 'h' 中段 x=206（t=0.86，蓝段
	// 尾部）；语义断言"蓝主导且非底色"，避免把逐通道线性插值期望钉死。
	gotG, okG := read(2, 206, 87)
	add("pixel_grad_zone", okG && gotG[2] > gotG[0] && gotG[2] > 0.6,
		fmt.Sprintf("got=%v want blue-dominant gradient tail", gotG))
	gotM, okM := read(2, 400, 170)
	add("pixel_mask_hidden", okM && nearRGB(gotM, colCardInk, 0.05),
		fmt.Sprintf("got=%v want card bg", gotM))

	// 卡 4（rotate）：旋转后墨迹在原点 (120,90) 上方偏左区域，取非底色即过；
	// 缺字体文本不画 → (340,180) 卡底。
	gotR, okR := read(3, 130, 104)
	add("pixel_rotated_ink", okR && !nearRGB(gotR, colCardInk, 0.05),
		fmt.Sprintf("got=%v want ink", gotR))
	gotF, okF := read(3, 340, 180)
	add("pixel_missing_refused", okF && nearRGB(gotF, colCardInk, 0.05),
		fmt.Sprintf("got=%v want card bg", gotF))

	return out
}

func pixelAt(img image.Image, x, y int) ([3]float64, bool) {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return [3]float64{}, false
	}
	r, g, bl, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255}, true
}
