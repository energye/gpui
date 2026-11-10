package main

import (
	"fmt"
	"image"
	"math"
	"strings"

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
	"parse_all_five",
	"switch_single_child",
	"css_materialized_order",
	"image_sources",
	"use_symbol_targets",
	"origin_pattern_attrs",
	"marker_element",
}

var (
	colBG    = hex01("#171a20")
	colGreen = hex01("#3ccf7a")
	colBlue  = hex01("#4a90e2")
	colAmber = hex01("#e2b84a")
	colGrey  = hex01("#808080")
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

	docs, err := parseCards()
	if err != nil {
		add("parse_all_five", false, err.Error())
		return out
	}
	// 卡 2 的 <style type="nope-css"> 跳过带一条具名警告（unsupported-selector
	// 族）——预期恰一条，其余三卡零警告。
	wantWarn := [5]int{0, 1, 0, 0, 0}
	warnOK := true
	for i, d := range docs {
		if len(d.Warnings) != wantWarn[i] {
			warnOK = false
			add("parse_all_five", false, fmt.Sprintf("card %d warnings=%d want %d (%v)",
				i, len(d.Warnings), wantWarn[i], d.Warnings))
		}
	}
	if warnOK {
		add("parse_all_five", true, "5 docs parsed, warnings as expected")
	} else {
		return out
	}

	// 1) switch 语义：选中子元素恰一个（条件命中者），display:none 的选中项
	// 也在（渲染为空、不回退）——usvg switch.rs convert。
	sw1, _ := docs[0].ElementByID("sw1").(*svg.GroupElement)
	sw1ID := ""
	if sw1 != nil && len(sw1.Children) == 1 {
		if r, ok := sw1.Children[0].(*svg.RectElement); ok {
			sw1ID = r.Attrs.ID
		}
	}
	ok1 := sw1 != nil && len(sw1.Children) == 1 && sw1ID == "sw1_hit"
	sw2, _ := docs[0].ElementByID("sw2").(*svg.GroupElement)
	sw2ID := ""
	if sw2 != nil && len(sw2.Children) == 1 {
		if r, ok := sw2.Children[0].(*svg.RectElement); ok {
			sw2ID = r.Attrs.ID
		}
	}
	ok2 := sw2 != nil && len(sw2.Children) == 1 && sw2ID == "sw2_hidden"
	add("switch_single_child", ok1 && ok2, fmt.Sprintf("sw1=%v(sw1_hit) sw2=%v(sw2_hidden 选中不回退)",
		ok1, ok2))

	// 2) CSS 物化：#css_spec 同时被 #id 规则与 .class 规则命中——specificity
	// 排序（simplecss sort_by_cached_key）让 id 规则在 style 串中靠后（后写
	// 者胜）；物化串同时含两个 fill 声明。
	spec, _ := docs[1].ElementByID("css_spec").(*svg.RectElement)
	okSpec := spec != nil
	idPos, clsPos := -1, -1
	if okSpec {
		idPos = strings.Index(spec.Attrs.Style, "fill:#3ccf7a")
		clsPos = strings.Index(spec.Attrs.Style, "fill:#e24a4a")
		okSpec = idPos >= 0 && clsPos >= 0 && idPos > clsPos
	}
	add("css_materialized_order", okSpec, fmt.Sprintf("style=%q id@%d > class@%d",
		specAttrs(spec), idPos, clsPos))

	// 3) image 数据源：外链解码（固有尺寸 63×61）、SVG 子文档、显式 0 高仍解码。
	ext, _ := docs[2].ElementByID("img_ext").(*svg.ImageElement)
	src, _ := docs[2].ElementByID("img_svg").(*svg.ImageElement)
	zero, _ := docs[2].ElementByID("img_zero").(*svg.ImageElement)
	okExt := ext != nil && ext.HasImage && ext.ImgW == 63 && ext.ImgH == 61
	okSrc := src != nil && src.SubDoc != nil
	okZero := zero != nil && zero.HasImage
	add("image_sources", okExt && okSrc && okZero,
		fmt.Sprintf("ext=%v(%dx%d) subdoc=%v zero=%v", okExt, iW(ext), iH(ext), okSrc, okZero))

	// 4) use/symbol 目标与属性：use 经 RefKind() 分类指向 symbol；sym_r
	// 自身 opacity=0.5；origin_rect 携带 transform-origin=center；pattern
	// 内容单位 oBB。
	useOp, _ := docs[3].ElementByID("use_op").(*svg.UseElement)
	okUse := useOp != nil && useOp.RefKind() == svg.RefKindSymbol
	symR, _ := docs[3].ElementByID("sym_r").(*svg.SymbolElement)
	okSym := symR != nil && symR.Attrs.Opacity == "0.5"
	origin, _ := docs[3].ElementByID("origin_rect").(*svg.RectElement)
	okOrigin := origin != nil && origin.Attrs.TransformOrigin == "center"
	patt, _ := docs[3].ElementByID("patt1").(*svg.PatternElement)
	okPatt := patt != nil && patt.ContentUnits == svg.UnitsObjectBoundingBox
	add("use_symbol_targets", okUse && okSym,
		fmt.Sprintf("use→symbol=%v sym_op=%v", okUse, okSym))
	add("origin_pattern_attrs", okOrigin && okPatt, fmt.Sprintf("origin=%v patt_oBB=%v", okOrigin, okPatt))

	// 5) marker：<marker> 解析为 MarkerElement（内容呈树），polyline 的
	// start/mid/end 三个 ref 都挂在属性上。
	ms, _ := docs[4].ElementByID("ms").(*svg.MarkerElement)
	ml, _ := docs[4].ElementByID("marker_line").(*svg.PolylineElement)
	okMarker := ms != nil && len(ms.Children) > 0 && ml != nil &&
		ml.Attrs.MarkerStart == "url(#ms)" && ml.Attrs.MarkerMid == "url(#mm)" && ml.Attrs.MarkerEnd == "url(#me)"
	markerDetail := "marker element missing"
	if okMarker {
		markerDetail = fmt.Sprintf("marker children=%d refs=start/mid/end", len(ms.Children))
	}
	add("marker_element", okMarker, markerDetail)

	return out
}

func specAttrs(r *svg.RectElement) string {
	if r == nil {
		return ""
	}
	return r.Attrs.Style
}

func iW(e *svg.ImageElement) int {
	if e == nil {
		return 0
	}
	return e.ImgW
}

func iH(e *svg.ImageElement) int {
	if e == nil {
		return 0
	}
	return e.ImgH
}

// --- GPU snapshot pixel probes (card-local coords, +innerX/innerY offsets) ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_sw_hit", "pixel_sw_nofallback", "pixel_sw_cond_hidden", "pixel_sw_sibling",
		"pixel_css_id", "pixel_css_attr", "pixel_css_child", "pixel_css_specificity",
		"pixel_css_important", "pixel_css_type",
		"pixel_img_ext", "pixel_img_svg_orange", "pixel_img_svg_white", "pixel_img_zero_skip",
		"pixel_use_opacity", "pixel_sym_opacity", "pixel_origin_rotated",
		"pixel_patt_grey", "pixel_patt_green", "pixel_patt_phase",
		"pixel_marker_start", "pixel_marker_mid", "pixel_marker_end", "pixel_marker_units",
	}
	if img == nil {
		for _, k := range keys {
			add(k, false, "snapshot unavailable")
		}
		return out
	}

	read := func(card int, x, y float64) ([3]float64, bool) {
		iy := innerY
		if card == 4 {
			iy = markerInnerY
		}
		ax := bx + cardAt[card][0] + innerX + x
		ay := by + cardAt[card][1] + iy + y
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

	// 卡 1：命中绿 / 选中 display:none 空 / 条件失败隐藏 / 兄弟绿。
	check("pixel_sw_hit", 0, 100, 95, colGreen)
	check("pixel_sw_nofallback", 0, 270, 95, colBG)
	check("pixel_sw_cond_hidden", 0, 435, 95, colBG)
	check("pixel_sw_sibling", 0, 435, 185, colGreen)

	// 卡 2：六块语义色。
	check("pixel_css_id", 1, 100, 90, colGreen)
	check("pixel_css_attr", 1, 270, 90, colBlue)
	check("pixel_css_child", 1, 435, 90, colAmber)
	check("pixel_css_specificity", 1, 100, 190, colGreen) // id 胜 class
	check("pixel_css_important", 1, 270, 190, colGreen)   // 声明序后者胜
	check("pixel_css_type", 1, 435, 190, colAmber)        // 表跳过→表现属性

	// 卡 3：外链橙带 / SVG 源橙底白圆 / 零高跳过。
	gotExt, okExt := read(2, 34, 111)
	add("pixel_img_ext", okExt && gotExt[0] > 0.9 && gotExt[1] > 0.45 && gotExt[1] < 0.75 && gotExt[2] < 0.15,
		fmt.Sprintf("got=%v want orange band", gotExt))
	check("pixel_img_svg_orange", 2, 215, 70, hex01("#e2884a"))
	check("pixel_img_svg_white", 2, 255, 105, [3]float64{1, 1, 1})
	check("pixel_img_zero_skip", 2, 425, 105, colBG)

	// 卡 4：两层 0.5 混合 / 旋转菱形 / pattern 三相位。
	blend := func(c [3]float64) [3]float64 {
		var o [3]float64
		for i := range c {
			o[i] = 0.5*c[i] + 0.5*colBG[i]
		}
		return o
	}
	check("pixel_use_opacity", 3, 90, 95, blend(colGreen))
	check("pixel_sym_opacity", 3, 240, 95, blend(hex01("#e24a4a")))
	check("pixel_origin_rotated", 3, 297, 126, colBlue)
	check("pixel_patt_grey", 3, 340, 70, colGrey)
	check("pixel_patt_green", 3, 380, 110, colGreen)
	check("pixel_patt_phase", 3, 380, 70, colBG)

	// 卡 5：marker-start/mid/end 在折线三顶点，markerUnits=strokeWidth 的
	// 末端红方块按 stroke-width=4 放大（4×4 → 16×16）。
	check("pixel_marker_start", 4, 60, 120, colGreen)
	check("pixel_marker_mid", 4, 264, 40, colAmber)
	check("pixel_marker_end", 4, 468, 120, colBlue)
	check("pixel_marker_units", 4, 476, 28, hex01("#e24a4a"))

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
