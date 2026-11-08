//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/energye/gpui/render/filters"
)

// P3.8 批 4b-1（SVG 滤镜接线）钉测。五个新原语 feMorphology / feTile /
// feDisplacementMap / feComponentTransfer / feImage + 滤镜原子区域机制。
// resvg v0.42 语料样本（svg+期望 png）入 testdata/p38/filter4b/；正用例做
// 48×48 感知网格分级（同 scripts/svgconformance 口径：identical ≤3 /
// close ≤24）+ 关键像素逻辑探针；负用例钉透明输出/直通语义。

const (
	gradeIdentical = 3.0
	gradeClose     = 24.0
)

// gradeAgainstPNG renders the parsed doc at the PNG's size and returns the
// mean per-channel grid delta vs the resvg expectation (lower is closer).
func gradeAgainstPNG(t *testing.T, doc *Document, pngPath string) float64 {
	t.Helper()
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatalf("open expectation: %v", err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode expectation: %v", err)
	}
	b := want.Bounds()
	img := doc.Render(b.Dx(), b.Dy())
	if img == nil {
		t.Fatal("render returned nil")
	}
	const n = 48
	var g1, g2 [n * n][4]float64
	var c1, c2 [n * n]int
	for y := 0; y < b.Dy(); y++ {
		gy := y * n / b.Dy()
		for x := 0; x < b.Dx(); x++ {
			gx := x * n / b.Dx()
			r1, g_1, b1, a1 := img.At(x, y).RGBA()
			i := gy*n + gx
			g1[i][0] += float64(r1 >> 8)
			g1[i][1] += float64(g_1 >> 8)
			g1[i][2] += float64(b1 >> 8)
			g1[i][3] += float64(a1 >> 8)
			c1[i]++
			r2, g_2, b2, a2 := want.At(b.Min.X+x, b.Min.Y+y).RGBA()
			g2[i][0] += float64(r2 >> 8)
			g2[i][1] += float64(g_2 >> 8)
			g2[i][2] += float64(b2 >> 8)
			g2[i][3] += float64(a2 >> 8)
			c2[i]++
		}
	}
	sum, cnt := 0.0, 0.0
	for i := range g1 {
		if c1[i] == 0 || c2[i] == 0 {
			continue
		}
		for c := 0; c < 4; c++ {
			d := g1[i][c]/float64(c1[i]) - g2[i][c]/float64(c2[i])
			if d < 0 {
				d = -d
			}
			sum += d
			cnt++
		}
	}
	return sum / cnt
}

func gradeSample(t *testing.T, name string) {
	t.Helper()
	doc := mustParseFile(t, "filter4b/"+name+".svg")
	if g := gradeAgainstPNG(t, doc, filepath.Join("testdata", "p38", "filter4b", name+".png")); g > gradeClose {
		t.Errorf("%s: grid delta %.1f > close %.0f", name, g, gradeClose)
	} else {
		t.Logf("%s: grid delta %.2f", name, g)
	}
}

// --- feMorphology ---

func TestP384bMorphologyParse(t *testing.T) {
	doc := mustParseFile(t, "filter4b/operator=dilate.svg")
	f := filterOf(t, doc, "filter1")
	p := f.Primitives[0]
	if p.Kind != "feMorphology" || p.Operator != "dilate" {
		t.Fatalf("primitive = %s op=%q", p.Kind, p.Operator)
	}
	if len(p.Radius) != 1 || p.Radius[0] != 10 {
		t.Fatalf("radius = %v", p.Radius)
	}
}

func TestP384bMorphologyDilate(t *testing.T) {
	// dilate 10：径向渐变核向外扩——中心保持渐变核的绿意（seagreen 核
	// dilate 后仍是绿系，不翻白），整幅与 resvg 期望分级 ≤ close。
	doc := mustParseFile(t, "filter4b/operator=dilate.svg")
	img := doc.Render(200, 200)
	if c := img.RGBAAt(100, 100); c.A == 0 || c.G < 100 || c.R >= c.G {
		t.Errorf("dilate center = %+v, want green-tinted gradient core", c)
	}
	gradeSample(t, "operator=dilate")
}

func TestP384bMorphologyAnisotropic(t *testing.T) {
	// radius="10 1"：x 膨胀 10、y 膨胀 1（异向窗口）。
	gradeSample(t, "radius=10-1")
}

func TestP384bMorphologyZeroRadiusClears(t *testing.T) {
	// radius="0"：resvg 语义输出透明（radialGradient 内容被清），黑帧仍在。
	doc := mustParseFile(t, "filter4b/zero-radius.svg")
	img := doc.Render(200, 200)
	if a := img.RGBAAt(100, 100).A; a != 0 {
		t.Errorf("zero-radius center alpha = %d, want 0 (cleared)", a)
	}
	maxA := uint8(0)
	for x := 0; x < 5; x++ {
		if a := img.RGBAAt(x, 100).A; a > maxA {
			maxA = a
		}
	}
	if maxA == 0 {
		t.Error("frame stroke missing on left edge")
	}
}

// --- feTile（+ 子区域机制：feFlood 子区域 → feOffset 继承 → feTile cell）---

func TestP384bTileSubregion(t *testing.T) {
	// cell = feFlood 子区域(4,4,10,10)+offset(5,5)，feTile 平铺锚定 cell
	// 原点、裁到自身子区域 (50,50,100,100)。链路语义（子区域继承、无名
	// 前驱缺省 in、平铺相位）以 resvg 期望图分级为准（实测 delta 0.43，
	// 与 resvg 期望几乎逐位一致）。
	gradeSample(t, "with-subregion-1")
}

// --- feDisplacementMap ---

func TestP384bDisplacementSimple(t *testing.T) {
	// 语料用例只画黑帧（resvg 自注：各家实现差异大到无法出题）——
	// 接线冒烟：滤镜解析执行不崩、帧在。
	doc := mustParseFile(t, "filter4b/simple-case.svg")
	img := doc.Render(200, 200)
	maxA := uint8(0)
	for x := 0; x < 5; x++ {
		if a := img.RGBAAt(x, 100).A; a > maxA {
			maxA = a
		}
	}
	if maxA == 0 {
		t.Error("frame stroke missing on left edge")
	}
	if c := img.RGBAAt(100, 100); c.A != 0 {
		t.Errorf("interior = %+v, want untouched transparent", c)
	}
}

// --- feComponentTransfer ---

func TestP384bTransferLUT(t *testing.T) {
	// resvg component_transfer.rs 公式对拍（关键采样点）。
	mk := func(typ string, tv []float64) [4]*TransferFunc {
		fn := &TransferFunc{Type: typ, TableValues: tv, Slope: 1, Amplitude: 1, Exponent: 1}
		return [4]*TransferFunc{fn}
	}
	// identity → nil（直通）
	r, _, _, _ := compileTransferLUTs(mk("identity", nil))
	if r != nil {
		t.Errorf("identity LUT = %v, want nil (passthrough)", r)
	}
	// table [1,0]：c=0 → 1，c=1 → 0，c=0.5 → 0.5
	r, _, _, _ = compileTransferLUTs(mk("table", []float64{1, 0}))
	if r[0] != 255 || r[255] != 0 || r[128] != 127 {
		t.Errorf("table [1,0]: lut[0]=%d lut[255]=%d lut[128]=%d (resvg truncates)", r[0], r[255], r[128])
	}
	// discrete [0.25, 0.75]：c<0.5 → 0.25*255≈63，c≥0.5 → 0.75*255≈191
	r, _, _, _ = compileTransferLUTs(mk("discrete", []float64{0.25, 0.75}))
	if r[0] != 63 || r[200] != 191 {
		t.Errorf("discrete: lut[0]=%d lut[200]=%d", r[0], r[200])
	}
	// linear slope=0.5 intercept=0.25：c=1 → 0.75（slopes override）
	lin := &TransferFunc{Type: "linear", Slope: 0.5, Intercept: 0.25}
	r, _, _, _ = compileTransferLUTs([4]*TransferFunc{lin})
	if r[255] != 191 {
		t.Errorf("linear: lut[255]=%d, want 191", r[255])
	}
	// gamma amplitude=1 exponent=2 offset=0：c=0.5 → 0.25
	ga := &TransferFunc{Type: "gamma", Amplitude: 1, Exponent: 2}
	r, _, _, _ = compileTransferLUTs([4]*TransferFunc{ga})
	if r[128] != 64 {
		t.Errorf("gamma: lut[128]=%d, want 64", r[128])
	}
	// 空 tableValues → identity（resvg is_dummy）
	r, _, _, _ = compileTransferLUTs(mk("table", nil))
	if r != nil {
		t.Errorf("empty table LUT = %v, want nil", r)
	}
}

func TestP384bTransferRender(t *testing.T) {
	gradeSample(t, "type=table-on-blue")
	gradeSample(t, "type=gamma-on-blue")
}

func TestP384bTransferNoChildrenPassesThrough(t *testing.T) {
	// 无 feFunc* 子元素：全通道 identity，内容直通。
	doc := mustParseFile(t, "filter4b/no-children.svg")
	img := doc.Render(200, 200)
	gradeSample(t, "no-children")
	_ = img
}

func TestP384bTransferEmptyTableIdentity(t *testing.T) {
	gradeSample(t, "type=table-without-tableValues")
}

// --- feImage ---

func TestP384bFeImageElementLink(t *testing.T) {
	// href="#rect3"（defs 内绿方块 36,36,120,120）：子树按自身坐标渲染、
	// 平移到子区域原点 (10,10) → 绿块落 (46,46)-(166,166)。
	doc := mustParseFile(t, "filter4b/link-to-an-element.svg")
	img := doc.Render(200, 200)
	if c := img.RGBAAt(106, 106); c.G < 100 || c.R >= c.G {
		t.Errorf("feImage element-link center = %+v, want green", c)
	}
	gradeSample(t, "link-to-an-element")
}

func TestP384bFeImageSelfRecursiveTerminates(t *testing.T) {
	// self-recursive：feImage 引用带同一滤镜的元素——引用子树按"进行中
	// 滤镜跳过"渲染（resvg 语义），必须终止且红方块在场。
	doc := mustParseFile(t, "filter4b/self-recursive.svg")
	img := doc.Render(200, 200)
	if c := img.RGBAAt(100, 100); c.R < 180 {
		t.Errorf("self-recursive center = %+v, want red square", c)
	}
	gradeSample(t, "self-recursive")
}

func TestP384bFeImageSubregionPlacement(t *testing.T) {
	// feImage x=50 y=50：无坐标绿方块 (0,0,100,100) 平移到 (50,50)。
	doc := mustParseFile(t, "filter4b/with-x-y.svg")
	img := doc.Render(200, 200)
	if c := img.RGBAAt(100, 100); c.G < 100 || c.R >= c.G {
		t.Errorf("feImage placement center = %+v, want green", c)
	}
	gradeSample(t, "with-x-y")
}

func TestP384bFeImageEmptyHrefClears(t *testing.T) {
	// 无 href 的 feImage → 透明输出（resvg dummy primitive），黑帧仍在。
	doc := mustParseFile(t, "filter4b/empty.svg")
	img := doc.Render(200, 200)
	if a := img.RGBAAt(100, 100).A; a != 0 {
		t.Errorf("empty feImage center alpha = %d, want 0", a)
	}
	maxA := uint8(0)
	for x := 0; x < 5; x++ {
		if a := img.RGBAAt(x, 100).A; a > maxA {
			maxA = a
		}
	}
	if maxA == 0 {
		t.Error("frame stroke missing on left edge")
	}
}

// --- 子区域通用机制（userSpace 缺省 = 滤镜区域，不裁剪）---

func TestP384bSubregionDefaultIsFullRegion(t *testing.T) {
	// blur 溢出必须完整（primitiveUnits 零值陷阱回归钉：ContentUnits 缺省
	// 一旦误判为 oBB，blur 外溢会被裁掉——TestP38FilterBlur 抓回的洞）。
	doc := mustParseFile(t, "filter_blur.svg")
	img := doc.Render(120, 80)
	if c := img.RGBAAt(60, 66); c.R <= 23 && c.A > 0 && c.G == 26 {
		t.Errorf("blur spill clipped again: %+v", c)
	}
}

// filterOf 取 defs 里指定 id 的 FilterElement（钉测辅助）。
func filterOf(t *testing.T, doc *Document, id string) *FilterElement {
	t.Helper()
	e := doc.ElementByID(id)
	f, ok := e.(*FilterElement)
	if !ok {
		t.Fatalf("ElementByID(%s) = %T, want *FilterElement", id, e)
	}
	return f
}
