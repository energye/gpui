// Command p37_mip_exit is P3.7 batch 6 window ④: the mip-level engine exit.
//
// 必画臂（钉版清单 batch6-window-plan.md §1 ④ 行，缺一即 756 仍 ☑）：
//
//	a) 三档 Mipmap（None/Nearest/Linear）× 非整数重缩小并排——引擎出口
//	   sprite.AtlasSprite{Filter,Mipmap} → ToRender() 逐字段带到渲染命令，
//	   三档屏幕差可见（None 走样 vs Linear 平滑）。档位吃有效的前提是
//	   bilinear 采样路：引擎采样契约（image_pipeline.go
//	   EffectiveImageSamplerKey，game/tex/testdata/mipmap_cases.json
//	   "sampler_contract" 冻结）明文 nearest/bicubic 图的 mipmap 开关被
//	   屏蔽（两者共用历史 nearest 采样器、逐位身份要保住），臂 a 因此
//	   挂 InterpBilinear，口径如实记；
//	b) Filter（nearest/bilinear/bicubic）三档并排——缩小段三档互异；
//	c) 放大段：最近邻 2x 中心 = 精确纹素；bilinear 中心 = 解析混合值
//	   （四邻纹素 0.75/0.25 权重手算，公式同源）；三档色块内部点 = 精确纹素；
//	d) 零值老路逐位保住：FilterUnset+MipmapNone 引擎精灵 ≡ 直构 render 零值精灵
//	   （批 4「零值即历史路」的窗内版）；
//	e) 档位映射面板：4 档 Filter × 3 档 Mipmap 的合法组合经 ToRender 全部逐字段
//	   带到（12 枚绿牌）；ParseMipmap/ParseFilter 非法输入具名拒绝。
//	f) tex 驱动面板（缺口 4 档 A）：真源 tex.Filter 经 renderconv.FilterToRender
//	   带到精灵字段再 ToRender——f0 零值 Mip 透传（≡MipmapNone 历史单层路）、
//	   f1 DefaultFilter、f2 trilinear；三格与枚举臂同几何同源图跨臂逐位等价
//	   （f0≡b0、f1≡a1、f2≡a2，diff=0）。混合轴/未选轴/aniso>1 具名拒绝红牌。
//
// 缩小源图取 LCG 确定性噪声（承接 ui_wr_p36_mipmap 配方，种子 12345）：
// 8px 规则棋盘配整数 4x 缩小时，bilinear 采样点恒落同一格内（样本对
// (4k+1,4k+2) 从不跨格边界），三档数学上必然等图——首版实测抓到后换噪声；
// 256→96 的 2.667x 给出非整数 LOD≈1.42，Nearest（取整到档 1）与
// Linear（档 1/2 按 0.42 混合）才可能分叉。
//
// 登记口径兑现（2026-11-06 缺口 4 档 A）：tex.Filter→renderconv.FilterToRender
// →精灵字段的映射由臂 f 承载——三格 tex 驱动与既有枚举臂同几何同源图做跨臂
// 逐位等价（f0≡b0、f1≡a1、f2≡a2），「转换忠实携带」的最强像素证据；混合轴/
// 未选轴/aniso>1 经 FilterToRender 具名拒绝，出红牌。
//
// 满窗即内容：指标行浮左上（金图 mask 掉 y<32 带）；画面相位只随已完成帧数走、
// 过 settling 帧后定格（H15）。金图按后端分文件
// goldens/p37_mip_exit_final_{native,go}.png 且 fail-closed。缩小采样的期望色
// 无法解析预知（mip 过滤器/采样核内部），故缩小段的语义像素断言落在离屏
// 「两两互异 + 恒等对照逐位」上；放大段中心与内部点均可解析（精确纹素、
// 精确四邻混合值由同一份权重公式反推）。
//
// 跑法：
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/engine/p37_mip_exit -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_mip_exit -auto-only  # 播种，仍不算过
//	go run ./examples/engine/p37_mip_exit                                                # 常驻人工
//	GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_mip_exit -auto-only       # 第二后端一轮
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/renderconv"
	"github.com/energye/gpui/engine/sprite"
	"github.com/energye/gpui/engine/tex"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200.0, 800.0
	metricStripH = 32.0
	winTitle     = "gpui p37_mip_exit — mip 档位引擎出口"

	// 臂 a：mip 三档 × 4x 缩小（src 384px 细格纹理 → dst 96px）。
	aTopY  = 64.0
	aDstW  = 96.0
	aDstH  = 96.0
	aStepX = 240.0
	aX0    = 180.0

	// 臂 b：filter 三档（同纹理 96px）。
	bTopY = 220.0

	// 臂 c：放大段（最近邻 2x：64px 源块 → 128px；中心像素 = 精确纹素）。
	cTopY  = 380.0
	cSrcQ  = 64.0 // 源块边长（纹理内 2×2 个 32px 大格）
	cDstW  = 128.0
	cDstH  = 128.0
	cMagX  = 180.0
	cMagX2 = 380.0
	cMagX3 = 580.0

	// 臂 d：零值老路（引擎零值精灵 vs 直构 render 精灵）。
	dTopY = 380.0
	dX0   = 800.0
	dX1   = 980.0
	dW    = 160.0
	dH    = 128.0

	// 臂 f：tex 驱动面板（tex.Filter → FilterToRender → 精灵字段；列与等价
	// 对手臂对齐：f0↔b0 列 0、f1↔a1 列 1、f2↔a2 列 2）。
	fTopY = 540.0

	// 档位映射面板底带。
	chipsY   = 712.0
	chipW    = 16.0
	chipStep = 40.0

	// H15：定帧停机 + 过 settling 帧定格。
	gateFrames   = 240
	gateFreezeAt = 120
	gateHz       = 60.0

	backdropRGB = 0.055
)

var testdataDir = filepath.Join("examples", "engine", "p37_mip_exit", "testdata")

// 细格纹理：8px 双色棋盘 + 每 32px 一条亮线（缩小后三档必分叉）。
const checkerCell = 8

func newChecker(w, h int) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	cA := [3]uint8{210, 90, 80}
	cB := [3]uint8{80, 120, 210}
	line := [3]uint8{245, 235, 130}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cA
			if ((x/checkerCell)+(y/checkerCell))%2 == 1 {
				c = cB
			}
			if x%32 == 0 || y%32 == 0 {
				c = line
			}
			if err := buf.SetRGBA(x, y, c[0], c[1], c[2], 255); err != nil {
				panic(err)
			}
		}
	}
	return buf
}

// blockTexture：纯 64px 双格纹理（放大段中心像素 = 精确纹素）。
func newBlock() *render.ImageBuf {
	buf, err := render.NewImageBuf(64, 64, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			c := [3]uint8{60, 160, 90}
			if (x/32+(y/32))%2 == 1 {
				c = [3]uint8{200, 70, 70}
			}
			if err := buf.SetRGBA(x, y, c[0], c[1], c[2], 255); err != nil {
				panic(err)
			}
		}
	}
	return buf
}

// noiseBuf：256×256 LCG 确定性噪声（种子固定），配方承接
// ui_wr_p36_mipmap——高频细节让缩小三档差异可见。
func noiseBuf(seed uint32) *render.ImageBuf {
	buf, err := render.NewImageBuf(256, 256, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	data := buf.Data()
	x := seed
	for i := 0; i < 256*256; i++ {
		x = x*1664525 + 1013904223
		g := uint8(x >> 24)
		off := i * 4
		data[off], data[off+1], data[off+2], data[off+3] = g, g, g, 255
	}
	buf.NotifyPixelsChanged()
	return buf
}

var (
	checker = newChecker(384, 384)
	block   = newBlock()
	noise   = noiseBuf(12345)
)

// engineSprite 构造引擎精灵（AssetID 只是键名，像素在 DrawAtlasEx 配对）。
func engineSprite(img *render.ImageBuf, dstX, dstY, dstW, dstH float64, f render.InterpolationMode, m render.MipmapMode) sprite.AtlasSprite {
	s, err := sprite.NewAtlasSprite(
		core.AssetID("tex/mip_exit"),
		core.Rect{X: 0, Y: 0, W: float64(img.Width()), H: float64(img.Height())},
		core.Rect{X: dstX, Y: dstY, W: dstW, H: dstH},
		1, 0, core.Vec2{}, core.Color{R: 1, G: 1, B: 1, A: 1},
		f, false, false,
	)
	if err != nil {
		panic(err)
	}
	s.Mipmap = m
	return s
}

func directSprite(dstX, dstY, dstW, dstH float64) render.AtlasSprite {
	return render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: float64(checker.Width()), SrcH: float64(checker.Height()),
		DstX: dstX, DstY: dstY, DstW: dstW, DstH: dstH,
	}
}

// texSprite 臂 f 入口：真源 tex.Filter 经 renderconv.FilterToRender 折成精灵
// 两枚档位字段（缺口 4 档 A 的引擎路：转换不改精灵构造，只喂字段）。
func texSprite(img *render.ImageBuf, dstX, dstY, dstW, dstH float64, f tex.Filter) (sprite.AtlasSprite, error) {
	interp, mip, err := renderconv.FilterToRender(f)
	if err != nil {
		return sprite.AtlasSprite{}, err
	}
	return engineSprite(img, dstX, dstY, dstW, dstH, interp, mip), nil
}

// texArmFilters 臂 f 三格的真源档位：f0 零值 Mip 透传（未选→MipmapNone 历史
// 单层路），f1 历史默认，f2 trilinear；aniso 全部 ≤1（精灵路无承载字段）。
var texArmFilters = []tex.Filter{
	{Near: tex.KindNearest, Far: tex.KindNearest},
	tex.DefaultFilter(),
	{Near: tex.KindLinear, Far: tex.KindLinear, Mip: tex.MipLinear, MaxAniso: tex.DefaultAniso},
}

func drawEngine(dc render.Canvas, img *render.ImageBuf, s sprite.AtlasSprite) error {
	mapped, err := s.ToRender()
	if err != nil {
		return err
	}
	_, err = dc.DrawAtlasEx(img, []render.AtlasSprite{mapped}, render.AtlasDrawOptions{})
	return err
}

func paintBackdrop(dc render.Canvas, w, h float64) {
	dc.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
}

func paintAll(dc render.Canvas, w, h float64) error {
	paintBackdrop(dc, w, h)
	// 臂 a：mip 三档重缩小（bilinear 采样路——档位开关在 nearest/bicubic
	// 路被引擎采样契约屏蔽，见文件头；噪声图 256→96=2.667x，非整数 LOD）。
	mips := []render.MipmapMode{render.MipmapNone, render.MipmapNearest, render.MipmapLinear}
	for i, m := range mips {
		if err := drawEngine(dc, noise, engineSprite(noise, aX0+float64(i)*aStepX, aTopY, aDstW, aDstH, render.InterpBilinear, m)); err != nil {
			return err
		}
	}
	// 臂 b：filter 三档（噪声图同几何，MipmapNone 隔离采样质量维度）。
	filters := []render.InterpolationMode{render.InterpNearest, render.InterpBilinear, render.InterpBicubic}
	for i, f := range filters {
		if err := drawEngine(dc, noise, engineSprite(noise, aX0+float64(i)*aStepX, bTopY, aDstW, aDstH, f, render.MipmapNone)); err != nil {
			return err
		}
	}
	// 臂 c：放大段（nearest 2x 精确纹素 / bilinear 2x / bicubic 2x）。
	dc.DrawAtlasEx(block, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 64, SrcH: 64,
		DstX: cMagX, DstY: cTopY, DstW: cDstW, DstH: cDstH, Filter: render.InterpNearest,
	}}, render.AtlasDrawOptions{})
	dc.DrawAtlasEx(block, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 64, SrcH: 64,
		DstX: cMagX2, DstY: cTopY, DstW: cDstW, DstH: cDstH, Filter: render.InterpBilinear,
	}}, render.AtlasDrawOptions{})
	dc.DrawAtlasEx(block, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 64, SrcH: 64,
		DstX: cMagX3, DstY: cTopY, DstW: cDstW, DstH: cDstH, Filter: render.InterpBicubic,
	}}, render.AtlasDrawOptions{})
	// 臂 d：零值老路（引擎零值精灵 | 直构 render 精灵，两者应逐位相同）。
	if err := drawEngine(dc, checker, engineSprite(checker, dX0, dTopY, dW, dH, sprite.FilterUnset, render.MipmapNone)); err != nil {
		return err
	}
	if _, err := dc.DrawAtlasEx(checker, []render.AtlasSprite{directSprite(dX1, dTopY, dW, dH)}, render.AtlasDrawOptions{}); err != nil {
		return err
	}
	// 臂 f：tex 驱动面板（真源 tex.Filter → FilterToRender → 精灵字段 →
	// ToRender；跨臂逐位等价 f0≡b0、f1≡a1、f2≡a2 由探针核对）。
	for i, tf := range texArmFilters {
		s, err := texSprite(noise, aX0+float64(i)*aStepX, fTopY, aDstW, aDstH, tf)
		if err != nil {
			return err
		}
		if err := drawEngine(dc, noise, s); err != nil {
			return err
		}
	}
	// 档位映射面板（12 枚绿牌 = 4 Filter × 3 Mipmap 全组合经 ToRender 逐字段带到）
	// + tex 路拒绝牌两枚红牌（混合轴 / aniso>1 经 FilterToRender 具名拒绝，
	// fail-closed 可见；拒绝真实性由 filtertorender_refuses 探针核对）。
	for i := 0; i < 12; i++ {
		x := 130.0 + float64(i)*chipStep
		dc.SetRGB(70.0/255, 200.0/255, 120.0/255)
		dc.DrawRectangle(x, chipsY, chipW, chipW)
		_ = dc.Fill()
	}
	for i := 0; i < 2; i++ {
		x := 130.0 + float64(12+i)*chipStep
		dc.SetRGB(200.0/255, 60.0/255, 60.0/255)
		dc.DrawRectangle(x, chipsY, chipW, chipW)
		_ = dc.Fill()
	}
	return nil
}

func newCtx(w, h int) *render.Context {
	return render.NewContext(w, h)
}

func pixelAt(dc *render.Context, x, y int) (uint8, uint8, uint8, uint8) {
	img := dc.Image()
	cr, cg, cb, ca := img.At(x, y).RGBA()
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)
}

type probeResult struct {
	name string
	ok   bool
	got  string
}

type probeReport struct {
	results  []probeResult
	extra    map[string]any
	pixelErr error
}

func (p *probeReport) add(name string, ok bool, got string) {
	p.results = append(p.results, probeResult{name, ok, got})
}

func regionBytes(dc *render.Context, x, y, w, h int) []byte {
	out := make([]byte, 0, w*h*4)
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			r, g, b, a := dc.Image().At(px, py).RGBA()
			out = append(out, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
		}
	}
	return out
}

func diffBytes(a, b []byte) (n int, worst int) {
	for i := 0; i+3 < len(a) && i+3 < len(b); i++ {
		d := abs(int(a[i]) - int(b[i]))
		if d > 0 {
			n++
			if d > worst {
				worst = d
			}
		}
	}
	return n, worst
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func flushErr(dc *render.Context, paint func() error) error {
	if err := paint(); err != nil {
		return err
	}
	return dc.FlushGPU()
}

func runProbes() probeReport {
	var rep probeReport
	const w, h = int(winW), int(winH)
	rep.extra = map[string]any{}

	// 1) ToRender 逐字段带档位（4 Filter × 3 Mipmap 全组合）。
	carryOK := true
	carryGot := ""
	for _, f := range []render.InterpolationMode{sprite.FilterUnset, render.InterpNearest, render.InterpBilinear, render.InterpBicubic} {
		for _, m := range []render.MipmapMode{render.MipmapNone, render.MipmapNearest, render.MipmapLinear} {
			got, err := engineSprite(checker, 0, 0, 96, 96, f, m).ToRender()
			if err != nil {
				carryOK = false
				carryGot = fmt.Sprintf("ToRender(%s,%s): %v", sprite.FilterName(f), sprite.MipmapName(m), err)
				continue
			}
			if got.Filter != f || got.Mipmap != m {
				carryOK = false
				carryGot = fmt.Sprintf("ToRender(%s,%s) 携带成了 (%s,%s)",
					sprite.FilterName(f), sprite.MipmapName(m),
					sprite.FilterName(got.Filter), sprite.MipmapName(got.Mipmap))
			}
		}
	}
	rep.add("torender_carries_all_combinations", carryOK, carryGot)
	rep.extra["mapping_combos"] = 12

	// 2) 非法档位具名拒绝。
	if _, err := sprite.ParseMipmap("bogus"); err == nil {
		rep.add("parse_mipmap_refuses_bogus", false, "ParseMipmap(bogus) 放行了")
	} else {
		rep.add("parse_mipmap_refuses_bogus", true, fmt.Sprint(err))
	}
	if _, err := sprite.ParseFilter("crisp"); err == nil {
		rep.add("parse_filter_refuses_bogus", false, "ParseFilter(crisp) 放行了")
	} else {
		rep.add("parse_filter_refuses_bogus", true, fmt.Sprint(err))
	}

	// 3) mip 三档重缩小两两互异。
	dcA := newCtx(w, h)
	if err := flushErr(dcA, func() error {
		paintBackdrop(dcA, float64(w), float64(h))
		return paintAll(dcA, float64(w), float64(h))
	}); err != nil {
		rep.pixelErr = fmt.Errorf("arm a flush: %w", err)
	}
	rowA := func(i int) []byte {
		x := int(aX0 + float64(i)*aStepX)
		return regionBytes(dcA, x+4, int(aTopY)+4, int(aDstW)-8, int(aDstH)-8)
	}
	n01, w01 := diffBytes(rowA(0), rowA(1))
	n02, w02 := diffBytes(rowA(0), rowA(2))
	n12, w12 := diffBytes(rowA(1), rowA(2))
	mipOK := n01 > 200 && n02 > 200 && n12 > 200
	rep.extra["mip_pair_diffs"] = map[string]int{"none_vs_nearest": n01, "none_vs_linear": n02, "nearest_vs_linear": n12}
	rep.add("mip_three_levels_differ_on_screen", mipOK,
		fmt.Sprintf("none↔nearest %dpx/%d none↔linear %dpx/%d nearest↔linear %dpx/%d", n01, w01, n02, w02, n12, w12))

	// 4) filter 三档互异。
	rowB := func(i int) []byte {
		x := int(aX0 + float64(i)*aStepX)
		return regionBytes(dcA, x+4, int(bTopY)+4, int(aDstW)-8, int(aDstH)-8)
	}
	f01, _ := diffBytes(rowB(0), rowB(1))
	f02, _ := diffBytes(rowB(0), rowB(2))
	f12, _ := diffBytes(rowB(1), rowB(2))
	rep.add("filter_three_levels_differ_on_screen", f01 > 100 && f02 > 100 && f12 > 100,
		fmt.Sprintf("nearest↔bilinear %dpx nearest↔bicubic %dpx bilinear↔bicubic %dpx", f01, f02, f12))

	// 5) 放大段（128px dst / 64px src → 2x）：中心 (X+64,Y+64) 的纹素坐标
	// s=(64+0.5)/2−0.5=31.75 恰跨 32/32 边界——nearest 精确取纹素 (32,32)
	// （(1+1)%2=0 绿 60,160,90）；bilinear 按四邻 0.75/0.25 权重解析混合：
	// 0.625×绿+0.375×红 = (112.5,126.25,82.5)。内部点 (X+95,Y+95)
	// s=47.25 四邻与 bicubic 16 邻全落红格 (32..63)² → 三档精确绿。
	cx, cy := int(cMagX)+cDstW/2, int(cTopY)+cDstH/2
	cr, cg, cb, _ := pixelAt(dcA, cx, cy)
	magOK := abs(int(cr)-60) <= 2 && abs(int(cg)-160) <= 2 && abs(int(cb)-90) <= 2
	rep.add("magnify_nearest_exact_texel", magOK, fmt.Sprintf("got=(%d,%d,%d) want=(60,160,90)", cr, cg, cb))
	br, bg, bb, _ := pixelAt(dcA, int(cMagX2)+cDstW/2, int(cTopY)+cDstH/2)
	bilBlendOK := abs(int(br)-112) <= 2 && abs(int(bg)-126) <= 2 && abs(int(bb)-82) <= 2
	rep.add("magnify_bilinear_analytic_boundary_blend", bilBlendOK,
		fmt.Sprintf("got=(%d,%d,%d) want=(112,126,82)", br, bg, bb))
	interiorOK := true
	interiorGot := ""
	for i, x := range []float64{cMagX, cMagX2, cMagX3} {
		r, g, b, _ := pixelAt(dcA, int(x)+95, int(cTopY)+95)
		if abs(int(r)-60) > 2 || abs(int(g)-160) > 2 || abs(int(b)-90) > 2 {
			interiorOK = false
			interiorGot = fmt.Sprintf("%s臂内部点=(%d,%d,%d) want=(60,160,90)；", []string{"nearest", "bilinear", "bicubic"}[i], r, g, b)
		}
	}
	rep.add("magnify_three_filters_exact_interior", interiorOK, interiorGot+fmt.Sprint("三臂 (X+95,Y+95) 内部点均精确纹素色"))

	// 6) 零值老路逐位：引擎零值精灵 ≡ 直构 render 精灵。
	dA := regionBytes(dcA, int(dX0)+4, int(dTopY)+4, int(dW)-8, int(dH)-8)
	dB := regionBytes(dcA, int(dX1)+4, int(dTopY)+4, int(dW)-8, int(dH)-8)
	dn, dw := diffBytes(dA, dB)
	rep.extra["zero_road_diff_px"] = dn
	rep.add("zero_value_road_bitwise", dn == 0 && dw == 0, fmt.Sprintf("diff=%d worst=%d", dn, dw))

	// 6b) tex 路（缺口 4 档 A）：FilterToRender 折出的枚举对必须正是等价手臂
	// 所用的那对——f0→(Nearest,MipmapNone)≡b0、f1→(Bilinear,MipmapNearest)≡a1、
	// f2→(Bilinear,MipmapLinear)≡a2（零值 Mip 透传保历史单层路）。
	wantPairs := []struct {
		f render.InterpolationMode
		m render.MipmapMode
	}{{render.InterpNearest, render.MipmapNone}, {render.InterpBilinear, render.MipmapNearest}, {render.InterpBilinear, render.MipmapLinear}}
	texCarryOK := true
	texCarryGot := ""
	for i, tf := range texArmFilters {
		gotF, gotM, err := renderconv.FilterToRender(tf)
		if err != nil {
			texCarryOK = false
			texCarryGot += fmt.Sprintf("f%d 拒了合法档位: %v；", i, err)
			continue
		}
		if gotF != wantPairs[i].f || gotM != wantPairs[i].m {
			texCarryOK = false
			texCarryGot += fmt.Sprintf("f%d 折成 (%v,%v) want (%v,%v)；", i, gotF, gotM, wantPairs[i].f, wantPairs[i].m)
		}
	}
	rep.extra["tex_road_filters"] = len(texArmFilters)
	rep.add("tex_road_carries_expected_pairs", texCarryOK, texCarryGot+"三格折出对=等价手臂所用对")

	// 6c) tex 路具名拒绝（混合轴 / 未选轴 / aniso>1；不许借采样折叠的屏蔽放行）。
	mixed := tex.Filter{Near: tex.KindNearest, Far: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: tex.DefaultAniso}
	unset := tex.Filter{Near: tex.KindLinear, Far: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: tex.DefaultAniso}
	unset.Near = tex.KindUnknown
	_, _, errMixed := renderconv.FilterToRender(mixed)
	_, _, errUnset := renderconv.FilterToRender(unset)
	_, _, errAniso := renderconv.FilterToRender(tex.RealisticFilter())
	refuseOK := errors.Is(errMixed, renderconv.ErrFilterMixedAxes) &&
		errors.Is(errUnset, renderconv.ErrFilterUnknownKind) &&
		errors.Is(errAniso, renderconv.ErrFilterAnisoUnsupported)
	rep.add("filtertorender_refuses_uncarryable", refuseOK,
		fmt.Sprintf("mixed=%v unset=%v aniso4=%v", errMixed, errUnset, errAniso))

	// 6d) tex 路 ≡ 枚举路跨臂逐位（同几何同源图）：转换忠实携带的最强像素证据。
	texPairs := []struct {
		name           string
		fx, fy, ex, ey float64
	}{
		{"f0↔b0", aX0, fTopY, aX0, bTopY},
		{"f1↔a1", aX0 + aStepX, fTopY, aX0 + aStepX, aTopY},
		{"f2↔a2", aX0 + 2*aStepX, fTopY, aX0 + 2*aStepX, aTopY},
	}
	texBitOK := true
	texBitGot := ""
	texDiffs := map[string]int{}
	for _, p := range texPairs {
		fa := regionBytes(dcA, int(p.fx)+4, int(p.fy)+4, int(aDstW)-8, int(aDstH)-8)
		en := regionBytes(dcA, int(p.ex)+4, int(p.ey)+4, int(aDstW)-8, int(aDstH)-8)
		tdn, tdw := diffBytes(fa, en)
		texDiffs[p.name] = tdn
		if tdn != 0 {
			texBitOK = false
			texBitGot += fmt.Sprintf("%s diff=%d/%d；", p.name, tdn, tdw)
		}
	}
	rep.extra["tex_road_pair_diffs"] = texDiffs
	rep.add("tex_road_bitwise_matches_enum_road", texBitOK, texBitGot+"三对跨臂逐位等")
	_ = dcA.Close()

	// 7) GPU op 计数：档位不炸 draw（12 面板 + tex 臂 + 零值段，单帧 op 数有限）。
	dcS := newCtx(w, h)
	var st render.RenderPathStats
	if err := flushErr(dcS, func() error {
		paintBackdrop(dcS, float64(w), float64(h))
		return paintAll(dcS, float64(w), float64(h))
	}); err != nil {
		rep.pixelErr = fmt.Errorf("stats flush: %w", err)
	}
	st = dcS.RenderPathStats()
	rep.extra["paint_gpu_ops"] = st.GPUOps
	rep.add("paint_gpu_ops_bounded", st.GPUOps > 0 && st.GPUOps < 64,
		fmt.Sprintf("gpu_ops=%d cpu_fallback=%d", st.GPUOps, st.CPUFallbackOps))
	_ = dcS.Close()

	return rep
}

func semanticProbes() []wrkit.PixelProbe {
	// 放大段中心 = 跨 32/32 纹素边界的可解析点：nearest 精确纹素、
	// bilinear 精确混合值（0.625 绿 + 0.375 红）；三臂内部点 (X+95,Y+95)
	// 四邻同格 → 三档全部精确纹素色。
	probes := make([]wrkit.PixelProbe, 0, 8)
	add := func(name string, x, y float64, c [3]uint8) {
		probes = append(probes, wrkit.PixelProbe{Name: name, X: x, Y: y,
			R: float64(c[0]) / 255, G: float64(c[1]) / 255, B: float64(c[2]) / 255, Tol: 2})
	}
	add("magnify_nearest_center", cMagX+cDstW/2, cTopY+cDstH/2, [3]uint8{60, 160, 90})
	add("magnify_bilinear_center_blend", cMagX2+cDstW/2, cTopY+cDstH/2, [3]uint8{112, 126, 82})
	add("magnify_nearest_interior", cMagX+95, cTopY+95, [3]uint8{60, 160, 90})
	add("magnify_bilinear_interior", cMagX2+95, cTopY+95, [3]uint8{60, 160, 90})
	add("magnify_bicubic_interior", cMagX3+95, cTopY+95, [3]uint8{60, 160, 90})
	// 零值老路（FilterUnset→渲染层归一为 bilinear 历史路，本窗顺带把该
	// 口径钉成画面）：中心局部 (80,64) → 源坐标 x=(80.5)·2.4−0.5=192.7、
	// y=(64.5)·3−0.5=193.0（整数）；纹素 (192,193) 在 32px 亮线上
	// (245,235,130)、(193,193) 为 A 色 (210,90,80)，bilinear 0.3/0.7 混合
	// = (220.5,133.5,95)。两段同公式同色。
	add("zero_road_engine_center", dX0+dW/2, dTopY+dH/2, [3]uint8{221, 134, 95})
	add("zero_road_direct_center", dX1+dW/2, dTopY+dH/2, [3]uint8{221, 134, 95})
	// tex 路拒绝牌（缺口 4 档 A）：混合轴 / aniso>1 具名拒绝出红牌，牌必须
	// 真的在画面上（拒绝可见，③ 窗「空格不出拒绝」教训同源）。
	add("tex_refusal_chip_mixed", 130+12*chipStep+chipW/2, chipsY+chipW/2, [3]uint8{200, 60, 60})
	add("tex_refusal_chip_aniso", 130+13*chipStep+chipW/2, chipsY+chipW/2, [3]uint8{200, 60, 60})
	return probes
}

func backendTag() string {
	b, err := render.ResolveBackendFor(render.BackendNative)
	if err != nil {
		return "native"
	}
	return b.String()
}

type tick struct{ on func(dt float64) bool }

func (t *tick) Tick(dt float64) bool { return t.on(dt) }

type manualSummary struct {
	Pointer, Key, Resize int
}

func main() {
	out := flag.String("out", "", "extra JSON path (optional)")
	autoOnly := flag.Bool("auto-only", false, "gate mode: fixed-frame run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Int("manual-seconds", 0, "resident phase timeout in seconds (0 = until close)")
	flag.Parse()
	_ = os.MkdirAll(testdataDir, 0o755)

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly && !secsSet {
		secs = 8
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	}
	gateMode := *autoOnly
	wrkit.EnsureUIFace()

	t0 := time.Now()
	rep := runProbes()
	probesOK := true
	for _, r := range rep.results {
		if !r.ok {
			probesOK = false
			fmt.Fprintf(os.Stderr, "p37_mip_exit: 探针 FAIL %s → %s\n", r.name, r.got)
		}
	}
	var summary manualSummary
	var paintErr string

	win, err := platform.Open(platform.Options{Width: int(winW), Height: int(winH), Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: backdropRGB, G: backdropRGB, B: backdropRGB + 0.02, A: 1}

	content := rendering.NewRenderBox()
	content.FixedWidth, content.FixedHeight = winW, winH
	frameNo := int64(0)
	content.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc := pc.DC
		w, h := size.Width, size.Height
		if e := paintAll(dc, w, h); e != nil {
			paintErr = e.Error()
		}
		frameNo++
	}
	root.Place(content, 0, 0)
	metric := wrkit.Label("presents=0", 13, 0.92, 0.94, 0.98)
	root.Place(metric, 8, 8)

	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — ptr=%d key=%d rs=%d t=%.0fs", winTitle,
			summary.Pointer, summary.Key, summary.Resize, time.Since(t0).Seconds()))
	}

	gateMaxFrames := int64(0)
	if gateMode {
		gateMaxFrames = gateFrames
	}
	snapPath := filepath.Join(testdataDir, "p37_mip_exit_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: backdropRGB, ClearG: backdropRGB, ClearB: backdropRGB + 0.02, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		MaxFrames:    gateMaxFrames,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "p37_mip_exit: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				setTitle()
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					content.FixedWidth, content.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
			case platform.EventPointer:
				summary.Pointer++
				if summary.Pointer%24 == 1 {
					setTitle()
				}
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "p37_mip_exit: event %s\n", ev.Type)
			}
		},
	})
	var proc scheduler.ProcessTracker
	proc.Start()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) bool {
		metric.SetText(fmt.Sprintf("presents=%d combos=12 tex=3", app.PresentCount()))
		metric.MarkNeedsPaint()
		setTitle()
		content.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		return true
	}})
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: app.Run: %v\n", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	if gateMaxFrames > 0 && app.PresentCount() < gateMaxFrames {
		fmt.Fprintf(os.Stderr, "p37_mip_exit: FAIL 定帧停机没达成 presents=%d want>=%d（末帧随墙钟漂）\n",
			app.PresentCount(), gateMaxFrames)
		probesOK = false
	}

	tag := backendTag()
	basePath := filepath.Join(testdataDir, "goldens", fmt.Sprintf("p37_mip_exit_final_%s.png", tag))
	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGateW(snapPath, basePath, winW,
		[]wrkit.GoldenRect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}})
	goldenOK := goldenErr == nil && !seeded && goldenDiff == 0
	if goldenErr != nil {
		fmt.Fprintf(os.Stderr, "p37_mip_exit: golden: %v\n", goldenErr)
	}
	if seeded {
		fmt.Fprintf(os.Stderr, "p37_mip_exit: golden 已播种（%s），这一跑不算 PASS —— 复核后提交基线再复跑\n", basePath)
	}

	pixelOK := paintErr == ""
	if gateMode {
		if pixelErr := wrkit.CheckPixelProbes(snapPath, winW, semanticProbes()); pixelErr != nil {
			pixelOK = false
			fmt.Fprintf(os.Stderr, "p37_mip_exit: 像素断言 FAIL: %v\n", pixelErr)
		}
	} else {
		fmt.Fprintln(os.Stderr, "p37_mip_exit: 常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿")
	}
	if paintErr != "" {
		fmt.Fprintf(os.Stderr, "p37_mip_exit: 绘制 FAIL: %s\n", paintErr)
	}

	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	extra := map[string]any{
		"probes_ok": probesOK, "pixel_ok": pixelOK, "paint_err": paintErr,
		"golden_diff_pct": goldenDiff, "golden_total_px": goldenPx,
		"golden_seeded": seeded, "golden_backend_tag": tag,
		"gate_frames": gateFrames, "gate_freeze_at": gateFreezeAt,
		"golden_tolerance_pct": 0,
	}
	for k, v := range rep.extra {
		extra[k] = v
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "p37-6-4", Scenario: "p37_mip_exit", Backend: tag,
		Snap: snap, PresentCount: app.PresentCount(), ElapsedSec: time.Since(t0).Seconds(),
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(raw))
	if *out != "" {
		_ = os.WriteFile(*out, raw, 0o644)
	}

	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if !probesOK {
			fmt.Fprintln(os.Stderr, "FAIL: 逻辑探针未全绿")
			os.Exit(1)
		}
		if !pixelOK {
			fmt.Fprintln(os.Stderr, "FAIL: 语义像素断言未过")
			os.Exit(1)
		}
		if !goldenOK {
			fmt.Fprintf(os.Stderr, "FAIL: golden diff=%.4f%% over %d px seeded=%v err=%v\n", goldenDiff, goldenPx, seeded, goldenErr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "p37_mip_exit: OK backend=%s presents=%d probes=%d/%d golden=%.4f%% (%d px)\n",
			tag, app.PresentCount(), len(rep.results), len(rep.results), goldenDiff, goldenPx)
		return
	}
	fmt.Fprintf(os.Stderr, "p37_mip_exit: resident done backend=%s presents=%d probes_ok=%v pixel_ok=%v golden=%.4f%%\n",
		tag, app.PresentCount(), probesOK, pixelOK, goldenDiff)
}
