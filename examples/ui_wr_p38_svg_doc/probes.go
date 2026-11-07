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

var probeKeys = []string{
	"par_meet_nine_aligns",
	"par_slice_nine_flags",
	"units_resolve_values",
	"inherit_attrs_clean",
	"use_refs_kinds",
	"nested_viewport_pars",
	"warnings_fail_closed",
}

// Shared expected colors (0..1 floats, matching the SVG hex sources).
var (
	colCellBG   = hex01("#20242c")
	colCardInk  = hex01("#171a20")
	colRed      = hex01("#e24a4a")
	colGreen    = hex01("#3ccf7a")
	colBlue     = hex01("#4a90e2")
	colYellow   = hex01("#f0c000")
	colWarm     = hex01("#f0a000")
	colChip     = hex01("#123456")
	colEmBar    = hex01("#e28b4a")
	colPctBar   = hex01("#9ae24a")
	colHalfRed  = blend01(colRed, colCardInk, 0.5)
)

func hex01(s string) [3]float64 {
	var r, g, b uint8
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return [3]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255}
}

func blend01(a, b [3]float64, t float64) [3]float64 {
	var out [3]float64
	for i := range a {
		out[i] = a[i]*t + b[i]*(1-t)
	}
	return out
}

func nearRGB(got, want [3]float64, tol float64) bool {
	for i := range got {
		if math.Abs(got[i]-want[i]) > tol {
			return false
		}
	}
	return true
}

// --- value-level logic probes (CPU subprocess, GOGPU_RENDER_MODE=cpu) ---

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	// Re-parse the six card sources at 1:1 (mirrors ensureBaked).
	srcs := [6]string{parGridSVG(false), parGridSVG(true), unitsSVG, inheritSVG, useSVG, nestedSVG}
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

	// 1) meet 九值：九个嵌套视口的 Align 序列 = 行主序九值。
	aligns := nestedAligns(docs[0])
	wantAligns := []svg.ParAlign{
		svg.ParAlignXMinYMin, svg.ParAlignXMidYMin, svg.ParAlignXMaxYMin,
		svg.ParAlignXMinYMid, svg.ParAlignXMidYMid, svg.ParAlignXMaxYMid,
		svg.ParAlignXMinYMax, svg.ParAlignXMidYMax, svg.ParAlignXMaxYMax,
	}
	add("par_meet_nine_aligns", alignsEqual(aligns, wantAligns), fmt.Sprintf("got %v", aligns))

	// 2) slice 九值：九个嵌套视口全 Slice=true，Align 同序。
	slices := nestedFlags(docs[1])
	sliceOK := len(slices) == 9
	for _, s := range slices {
		sliceOK = sliceOK && s.slice && s.alignMatches(wantAligns)
	}
	add("par_slice_nine_flags", sliceOK, fmt.Sprintf("got %v", slices))

	// 3) 单位换算：1in=96、2em=40（fs=20）、4ex=40、50%=172、r10%=对角 10%。
	uOK, uDetail := unitChecks(docs[2])
	add("units_resolve_values", uOK, uDetail)

	// 4) 继承卡：原始属性保留（缺省=继承），解析零 Warning。
	d4 := docs[3]
	rawDash := firstRectAttr(d4, "stroke-dasharray")
	add("inherit_attrs_clean", rawDash == "8 4" && len(d4.Warnings) == 0,
		fmt.Sprintf("dash=%q warnings=%d", rawDash, len(d4.Warnings)))

	// 5) 引用卡：#chip/#box 可查；use kind = symbol / element。
	d5 := docs[4]
	chip, box := d5.ElementByID("chip"), d5.ElementByID("box")
	kinds := useKinds(d5)
	add("use_refs_kinds", chip != nil && box != nil && kinds == "symbol,element",
		fmt.Sprintf("chip=%v box=%v kinds=%s", chip != nil, box != nil, kinds))

	// 6) 嵌套视口：两枚嵌套 <svg>，第一枚 xMaxYMax meet，第二枚 xMidYMid slice。
	nested := nestedFlags(docs[5])
	nestedOK := len(nested) == 2 &&
		nested[0].align == svg.ParAlignXMaxYMax && !nested[0].slice &&
		nested[1].align == svg.ParAlignXMidYMid && nested[1].slice
	add("nested_viewport_pars", nestedOK, fmt.Sprintf("got %v", nested))

	// 7) fail-closed：悬空 url() 产 unresolved-paint Warning（不画、非黑）。
	found := false
	for _, w := range d5.Warnings {
		if w.Reason == "unresolved-paint" {
			found = true
		}
	}
	add("warnings_fail_closed", found, fmt.Sprintf("warnings=%d", len(d5.Warnings)))

	return out
}

type nestedFlag struct {
	align svg.ParAlign
	slice bool
}

func (n nestedFlag) String() string {
	return fmt.Sprintf("{align:%d slice:%v}", n.align, n.slice)
}

func (n nestedFlag) alignMatches(want []svg.ParAlign) bool {
	for _, w := range want {
		if n.align == w {
			return true
		}
	}
	return false
}

func nestedAligns(d *svg.Document) []svg.ParAlign {
	var out []svg.ParAlign
	for _, e := range d.Elements {
		if s, ok := e.(*svg.SVGElement); ok {
			out = append(out, s.Par.Align)
		}
	}
	return out
}

func nestedFlags(d *svg.Document) []nestedFlag {
	var out []nestedFlag
	for _, e := range d.Elements {
		if s, ok := e.(*svg.SVGElement); ok {
			out = append(out, nestedFlag{align: s.Par.Align, slice: s.Par.Slice})
		}
	}
	return out
}

func alignsEqual(a, b []svg.ParAlign) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func unitChecks(d *svg.Document) (ok bool, detail string) {
	rects := collectRects(d.Elements)
	if len(rects) < 9 {
		return false, fmt.Sprintf("rects=%d", len(rects))
	}
	rs := svg.RefSpace{ViewportW: 344, ViewportH: 252, FontSize: 20}
	in := rects[1].W.Resolve(rs, svg.LengthAxisX)          // 1in = 96（[0]=底色）
	em := rects[6].W.Resolve(rs, svg.LengthAxisX)          // 2em = 40
	ex := rects[7].W.Resolve(rs, svg.LengthAxisX)          // 4ex = 40
	pct := rects[8].W.Resolve(rs, svg.LengthAxisX)         // 50% = 172
	circ, _ := d.Elements[8].(*svg.CircleElement)
	r := circ.R.Resolve(rs, svg.LengthAxisDiagonal) // 10% of diag
	// 对角 = sqrt((344²+252²)/2) = sqrt(90920) = 301.5294…，10% ⇒ 30.15294。
	ok = near1(in, 96) && near1(em, 40) && near1(ex, 40) && near1(pct, 172) && math.Abs(r-30.15294) < 1e-4
	return ok, fmt.Sprintf("in=%.3f em=%.3f ex=%.3f pct=%.3f r=%.4f", in, em, ex, pct, r)
}

func near1(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func collectRects(elems []svg.Element) []*svg.RectElement {
	var out []*svg.RectElement
	var walk func(elems []svg.Element)
	walk = func(elems []svg.Element) {
		for _, e := range elems {
			switch t := e.(type) {
			case *svg.RectElement:
				out = append(out, t)
			case *svg.GroupElement:
				walk(t.Children)
			}
		}
	}
	walk(elems)
	return out
}

func firstRectAttr(d *svg.Document, _ string) string {
	// 原始属性面：继承卡的第一个 rect 的 stroke-dasharray 挂在 <g> 上，
	// rect 自身无 stroke-dasharray —— 这里取根 g 的原文。
	for _, e := range d.Elements {
		if g, ok := e.(*svg.GroupElement); ok {
			return g.Attrs.StrokeDasharray
		}
	}
	return ""
}

func useKinds(d *svg.Document) string {
	var kinds []string
	for _, e := range d.Elements {
		if u, ok := e.(*svg.UseElement); ok {
			switch u.RefKind() {
			case svg.RefKindSymbol:
				kinds = append(kinds, "symbol")
			case svg.RefKindElement:
				kinds = append(kinds, "element")
			}
		}
	}
	if len(kinds) == 0 {
		return "none"
	}
	out := kinds[0]
	for _, k := range kinds[1:] {
		out += "," + k
	}
	return out
}

// --- GPU snapshot pixel probes (card-local coords, +innerX/innerY offsets) ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_par_meet_min", "pixel_par_meet_max", "pixel_par_meet_center_bg",
		"pixel_par_slice_min", "pixel_par_slice_right", "pixel_par_slice_bottom_blue",
		"pixel_unit_in_bar", "pixel_unit_in_end_bg", "pixel_unit_em_bar", "pixel_unit_em_end_bg", "pixel_unit_pct_bar",
		"pixel_inherit_fill", "pixel_inherit_current", "pixel_inherit_style", "pixel_inherit_halfalpha", "pixel_inherit_class",
		"pixel_use_symbol_body", "pixel_use_shadow_chip", "pixel_use_plain_chip",
		"pixel_dangling_bg", "pixel_contrast_green",
		"pixel_nested_max_blue", "pixel_nested_left_bg", "pixel_nested_slice_green",
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
		add(key, nearRGB(got, want, 0.05), fmt.Sprintf("got=%v want=%v", got, want))
	}

	// 卡 1（meet）：cell(0,0)=xMinYMin 红象限贴左上；cell(2,0)=xMaxYMin 红象限贴右；
	// cell(1,1)=xMidYMid 左侧留 bg（水平居中）。
	check("pixel_par_meet_min", 0, 18, 18, colRed)
	check("pixel_par_meet_max", 0, cellX[2]+54, cellY[0]+18, colRed)
	check("pixel_par_meet_center_bg", 0, cellX[1]+8, cellY[1]+40, colCellBG)

	// 卡 2（slice）：s=5.4，内容 108×108 横向铺满：xMinYMin 红在左上、
	// xMaxYMin 顶行右端是绿（TR 象限）、底部 y=70 处 x 中部是蓝（BL 象限裁剪保留）。
	check("pixel_par_slice_min", 1, 18, 18, colRed)
	check("pixel_par_slice_right", 1, cellX[2]+100, cellY[0]+18, colGreen)
	check("pixel_par_slice_bottom_blue", 1, cellX[0]+50, cellY[0]+70, colBlue)

	// 卡 3（单位）：条形终点内=色、终点外=bg；50% 条 = 172px。
	check("pixel_unit_in_bar", 2, 100, 20, colRed)
	check("pixel_unit_in_end_bg", 2, 112, 20, colCardInk)
	check("pixel_unit_em_bar", 2, 44, 148, colEmBar)
	check("pixel_unit_em_end_bg", 2, 60, 148, colCardInk)
	check("pixel_unit_pct_bar", 2, 170, 200, colPctBar)

	// 卡 4（继承）：继承填充 / currentColor / style 覆盖 / fill-opacity 混色 / class。
	check("pixel_inherit_fill", 3, 50, 45, colRed)
	check("pixel_inherit_current", 3, 160, 45, colGreen)
	check("pixel_inherit_style", 3, 265, 45, colYellow)
	check("pixel_inherit_halfalpha", 3, 50, 130, colHalfRed)
	check("pixel_inherit_class", 3, 160, 130, colWarm)

	// 卡 5（引用）：symbol 体内红外 + 阴影树 chip 蓝 + 独立 chip 蓝；
	// 悬空 url() 区域 = 卡底色（非黑非内容），右侧对照 rect 绿。
	check("pixel_use_symbol_body", 4, 30, 40, colRed)
	check("pixel_use_shadow_chip", 4, 60, 50, colChip)
	check("pixel_use_plain_chip", 4, 225, 30, colChip)
	check("pixel_dangling_bg", 4, 50, 160, colCardInk)
	check("pixel_contrast_green", 4, 160, 160, colGreen)

	// 卡 6（嵌套）：xMaxYMax meet 内容贴右（左端 bg）；xMidYMid slice 全宽绿。
	check("pixel_nested_max_blue", 5, 200, 60, colBlue)
	check("pixel_nested_left_bg", 5, 30, 60, colCardInk)
	check("pixel_nested_slice_green", 5, 130, 180, colGreen)

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
