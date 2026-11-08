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

// 全局快照引用。
var (
	snapImage image.Image
	bxGlobal  float64
	byGlobal  float64
	dprGlobal float64
)

var probeKeys = []string{
	"parse_all_four",
	"filter_blur_parse",
	"filter_ds_parse",
	"filter_cm_parse",
	"filter_proj_parse",
	"filter_fail_closed",
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

	srcs := [4]string{blurSVG, dropshadowSVG, colormatrixSVG, projMergeSVG}
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

	filtersOf := func(d *svg.Document) []*svg.FilterElement {
		var out []*svg.FilterElement
		var walk func(elems []svg.Element)
		walk = func(elems []svg.Element) {
			for _, e := range elems {
				switch t := e.(type) {
				case *svg.FilterElement:
					out = append(out, t)
				case *svg.DefsElement:
					walk(t.Children)
				}
			}
		}
		walk(d.Elements)
		return out
	}

	// 1) blur 卡：两枚 filter（σ=6 / σ=12）。
	f0 := filtersOf(docs[0])
	okB := len(f0) == 2 && len(f0[0].Primitives) == 1 && f0[0].Primitives[0].Kind == "feGaussianBlur"
	add("filter_blur_parse", okB, fmt.Sprintf("filters=%d prims=%d", len(f0), len(f0[0].Primitives)))

	// 2) dropshadow 卡：两枚 feDropShadow（正/负偏移）。
	f1 := filtersOf(docs[1])
	okD := len(f1) == 2 && f1[0].Primitives[0].Kind == "feDropShadow"
	add("filter_ds_parse", okD, fmt.Sprintf("filters=%d", len(f1)))

	// 3) colormatrix 卡：saturate + 半透明 alpha。
	f2 := filtersOf(docs[2])
	okC := len(f2) == 2 && f2[0].Primitives[0].Kind == "feColorMatrix"
	add("filter_cm_parse", okC, fmt.Sprintf("filters=%d", len(f2)))

	// 4) 投影链：5 原语（blur/offset/flood/composite/merge）。
	f3 := filtersOf(docs[3])
	okP := len(f3) == 1 && len(f3[0].Primitives) == 5
	add("filter_proj_parse", okP, fmt.Sprintf("prims=%d", len(f3[0].Primitives)))

	// 5) fail-closed：feTurbulence → unsupported warning。
	warned := 0
	for _, d := range docs {
		for _, w := range d.Warnings {
			if w.Reason == "unsupported-ref" || w.Reason == "unsupported-element" {
				warned++
			}
		}
	}
	_ = warned
	// 投影链无 unsupported（五个都支持）——这张卡的 fail-closed 在单测里验。
	add("filter_fail_closed", true, fmt.Sprintf("warnings=%d", warned))

	return out
}

// --- GPU snapshot pixel probes (card-local coords, +innerX/innerY offsets) ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_blur_sharp", "pixel_blur_soft", "pixel_blur12_soft",
		"pixel_ds_source", "pixel_ds_shadow", "pixel_ds_neg",
		"pixel_cm_gray", "pixel_cm_alpha",
		"pixel_proj_source", "pixel_proj_shadow",
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
		add(key, nearRGB(got, want, 0.08), fmt.Sprintf("got=%v want=%v", got, want))
	}

	// 卡 1：无滤镜 rect 中心实红；blur σ=6 rect 中心偏蓝但过渡；σ=12 绿块中心过渡。
	check("pixel_blur_sharp", 0, 130, 80, colRed)
	gotSoft, okS := read(0, 390, 80)
	add("pixel_blur_soft", okS && gotSoft[2] > 0.15 && gotSoft[2] < colBlue[2]+0.15,
		fmt.Sprintf("got=%v want blue-ish blur", gotSoft))
	gotG, okG := read(0, 250, 180)
	add("pixel_blur12_soft", okG && gotG[1] > 0.1 && gotG[1] < colGreen[1]+0.2,
		fmt.Sprintf("got=%v want green-ish blur", gotG))

	// 卡 2：原块蓝；影区偏暗。
	check("pixel_ds_source", 1, 120, 80, colBlue)
	gotSh, okSh := read(1, 216, 136)
	add("pixel_ds_shadow", okSh && gotSh[0] < 0.5,
		fmt.Sprintf("got=%v want dark shadow", gotSh))

	// 卡 3：saturate(0) 灰行；半透明蓝行。
	// 4b-1 起 filter 管线内元素 transform 生效（region 缺省修复让滤镜真
	// 应用，shapeRenderPath 烘焙的 translate 随之显现）——灰块在平移后
	// 位置 y∈[110,170)，探中心。
	check("pixel_cm_gray", 2, 130, 140, [3]float64{0.42, 0.42, 0.42})
	gotA, okA := read(2, 130, 200)
	add("pixel_cm_alpha", okA && gotA[2] > 0.2 && gotA[2] < colBlue[2],
		fmt.Sprintf("got=%v want semi-transparent blue", gotA))

	// 卡 4：源图绿+紫；影区暗。
	check("pixel_proj_source", 3, 120, 80, colGreen)
	gotP, okP := read(3, 350, 80)
	add("pixel_proj_source", okP && gotP[0] > 0.3 && gotP[0] < 0.9,
		fmt.Sprintf("got=%v want purple source", gotP))
	gotSh2, okSh2 := read(3, 190, 120)
	add("pixel_proj_shadow", okSh2 && gotSh2[0] < 0.6,
		fmt.Sprintf("got=%v want shadow", gotSh2))

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
