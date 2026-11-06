// Command p37_persp_batch is P3.7 batch 6 window ①: the projective batch leg.
//
// 必画五臂（缺一即批 1a–1d 仍记 ☑，不得改回 ✅）：
//
//	a) 同段连续 ≥2 张同透视 quad，中间不夹任何别的东西 —— 批 1a 买到的合批，
//	   在窗内逐帧用 Context.ImageBatchStats 量（PerspDraws=1 / MaxPerspRun=N）；
//	b) 透视腿带 Tint（批 1b：「按深度投影」与「被灯光染色」从前互斥）；
//	c) 采样核三态：透视+最近邻合并成段、透视+bicubic 具名拒绝、仿射+bicubic 合法；
//	d) 图集批绘腿吃四角 + 权重，角点与权重来自引擎出口 camera.ProjectSpriteQuad
//	   （批 1c 的 2.5D 正路，不是手喂 quad）；
//	e) 画布级 4×4 投影层 Set/Push/PopProjection（批 1d：本窗之前全仓零调用者）。
//
// 加厚内容（批 6 规矩：窗演一族能力，不是演一颗精灵）：投影层下的远处告示牌
// （带圆角裁剪）、眼后整块拒绝、非法权重具名拒绝、暗带与亮带对照。
//
// 满窗即内容：指标行浮在画面左上，金图 mask 掉那条带；画面相位只随已完成帧数
// 走、过 settling 帧后定格（H15），金图按后端分文件
// goldens/p37_persp_batch_final_{native,go}.png 且 fail-closed（缺基线判红，
// 播种那一跑也不算过）。像素断言坐标由「画面所用同一份投影」反推（P3.5 教训）。
//
// 跑法：
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/engine/p37_persp_batch -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_persp_batch -auto-only  # 播种，仍不算过
//	go run ./examples/engine/p37_persp_batch                                                 # 常驻人工
//	GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_persp_batch -auto-only        # 第二后端一轮
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200.0, 800.0
	metricStripH = 32.0
	winTitle     = "gpui p37_persp_batch — 透视批绘腿"
	focal        = 520.0

	// 地面板带：六张同规格梯形，深度 near/far 决定透视量。
	platesPerRow = 6
	plateStepX   = 180.0
	plateFirstX  = 110.0
	plateHW      = 78.0
	plateHH      = 46.0
	plateNear    = 300.0
	plateFar     = 520.0
	rowAY        = 360.0 // 合并段（臂 a）
	rowBY        = 470.0 // 夹描边段（臂 a 的反面）
	rowCY        = 580.0 // 染色对照段（臂 b）

	// 图集批绘段（臂 d，走引擎出口）
	atlasRowY    = 686.0
	atlasSprites = 5
	atlasStepX   = 200.0
	atlasFirstX  = 150.0
	atlasHW      = 54.0
	atlasHH      = 34.0
	atlasTile    = 64.0

	refusalY   = 756.0 // 具名拒绝态带（臂 c + 眼后 + 非法权重）
	billboardY = 250.0 // 投影层下的远处告示牌（臂 e）

	// H15：定帧停机 + 过 settling 帧定格。
	gateFrames   = 240
	gateFreezeAt = 120
	gateHz       = 60.0

	// 面板底色与琥珀染色系数：探针期望由这两个常数算出来，
	// 与画面用的是同一份数（不从屏幕反推结论）。
	plateProbeLum          = 0.75 // 纹理中段亮度（mul8(250, shade≈187) / 255）
	amberR, amberG, amberB = 1.0, 0.62, 0.28

	// 容差（写死，不设暗档）：
	backdropRGB    = 0.06
	tintSlackSteps = 6 // GPU 预乘 + AA，染色探针允许六档
	bandSlackSteps = 8 // 明暗带对照同上
	mergeBitExact  = 0 // 合并段与拆开段画的是同一批 quad ⇒ 必须逐位相同
)

var testdataDir = filepath.Join("examples", "engine", "p37_persp_batch", "testdata")

// perspFace：臂 e「投影下文字 fail-closed」臂用字（ui_render_base_text 同族
// LoadMultiFace，退系统默认；缺字则臂探针判红，不静默跳——AGENTS 禁假绿）。
var perspFace text.Face

func initFace() {
	if f, _, err := text.LoadMultiFace(16); err == nil {
		perspFace = f
		return
	}
	if f, _, err := rendering.TryLoadDefaultFace(16); err == nil {
		perspFace = f
	}
}

type quad struct {
	corners [4]render.Point
	weights [4]float64
}

func (q quad) center() render.Point {
	var x, y float64
	for _, c := range q.corners {
		x += c.X
		y += c.Y
	}
	return render.Pt(x/4, y/4)
}

// plate 是画面与探针共用的唯一投影出口（不从屏幕反推结论）。
//
// 注意中心必须固定：早先版本把「想要的屏幕落点」同时当成投影中心，于是移动 cx
// 只是移动坐标系原点，画面上什么都没动——常驻相看不出动画、金图也抓不住相位回归
// （实测：把振幅 18 放大到 180 才在比对区看出 10898 px 差）。现与图集腿同一把
// 尺子：先把屏幕落点按中段深度折回世界系，再让固定中心的投影器把它投回来。
func plate(sx, sy, hw, hh, near, far float64) (quad, bool) {
	m, ok := render.ProjectorMatrix4(focal, render.Pt(winW/2, winH/2), render.Point{})
	if !ok {
		return quad{}, false
	}
	k := (focal + (near+far)/2) / focal
	world := [4]render.Point{
		{X: (sx-winW/2)*k - hw*k, Y: (sy-winH/2)*k - hh*k},
		{X: (sx-winW/2)*k + hw*k, Y: (sy-winH/2)*k - hh*k},
		{X: (sx-winW/2)*k + hw*k, Y: (sy-winH/2)*k + hh*k},
		{X: (sx-winW/2)*k - hw*k, Y: (sy-winH/2)*k + hh*k},
	}
	c, w, ok := m.ProjectQuad(world, [4]float64{far, far, near, near})
	return quad{corners: c, weights: w}, ok
}

func plateRow(cy, pan float64) []quad {
	out := make([]quad, 0, platesPerRow)
	for i := 0; i < platesPerRow; i++ {
		q, ok := plate(plateFirstX+float64(i)*plateStepX+pan, cy, plateHW, plateHH, plateNear, plateFar)
		if ok {
			out = append(out, q)
		}
	}
	return out
}

// newPlateTexture 生成横条测试图（亮上暗下），便于用中心像素判断「画没画上」。
func newPlateTexture(w, h int, r, g, b uint8) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		panic(fmt.Sprintf("p37_persp_batch: NewImageBuf: %v", err))
	}
	for y := 0; y < h; y++ {
		shade := uint8(120 + 135*y/h)
		for x := 0; x < w; x++ {
			if err := buf.SetRGBA(x, y, mul8(r, shade), mul8(g, shade), mul8(b, shade), 255); err != nil {
				panic(fmt.Sprintf("p37_persp_batch: SetRGBA: %v", err))
			}
		}
	}
	return buf
}

// newSheetTexture 生成四色块图集（图集腿按 Src 子矩形取块）。
func newSheetTexture() *render.ImageBuf {
	const side = int(atlasTile)
	buf, err := render.NewImageBuf(side*2, side*2, render.FormatRGBA8)
	if err != nil {
		panic(fmt.Sprintf("p37_persp_batch: sheet: %v", err))
	}
	colors := [4][3]uint8{{235, 90, 60}, {70, 200, 120}, {80, 130, 240}, {230, 205, 70}}
	for ty := 0; ty < 2; ty++ {
		for tx := 0; tx < 2; tx++ {
			c := colors[ty*2+tx]
			for y := 0; y < side; y++ {
				for x := 0; x < side; x++ {
					if err := buf.SetRGBA(tx*side+x, ty*side+y, c[0], c[1], c[2], 255); err != nil {
						panic(fmt.Sprintf("p37_persp_batch: sheet pixel: %v", err))
					}
				}
			}
		}
	}
	return buf
}

func mul8(v, s uint8) uint8 { return uint8(uint16(v) * uint16(s) / 255) }

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func drawQuad(dc render.Canvas, tex *render.ImageBuf, q quad, opts render.QuadDrawOptions) error {
	return dc.DrawImageQuad(tex, q.corners, opts)
}

func perspOpts() render.QuadDrawOptions {
	return render.QuadDrawOptions{Perspective: true, Weights: [4]float64{}, Opacity: 1,
		Interpolation: render.InterpNearest}
}

// paintMergedRun：臂 a —— 六张连续同透视 quad，中间只有图像命令。
func paintMergedRun(dc render.Canvas, tex *render.ImageBuf, plates []quad) error {
	for i, q := range plates {
		opts := perspOpts()
		opts.Weights = q.weights
		if err := drawQuad(dc, tex, q, opts); err != nil {
			return fmt.Errorf("merged plate %d: %w", i, err)
		}
	}
	return nil
}

// paintSplitRun：臂 a 的反面 —— 同样六张 quad，但每张**之前**先画一条落在面板
// 间隙里的分隔刻度。顺序是有讲究的：刻度画在 quad 之后会让本带第一张接上上一段
// 的尾巴（实测最长段变 7，读数不再对应「这一段」）；画在前头则每段都被路径命令
// 切断，窗内逐帧读数就是干净的「六段各一张」。刻度落在间隙里，不压面板像素。
func paintSplitRun(dc render.Canvas, tex *render.ImageBuf, plates []quad, _ float64) error {
	for i, q := range plates {
		c := q.center()
		dc.SetRGBA(0.95, 0.95, 0.98, 1)
		dc.SetLineWidth(2)
		dc.DrawLine(c.X+plateStepX/2, rowBY-16, c.X+plateStepX/2, rowBY+16)
		if err := dc.Stroke(); err != nil {
			return fmt.Errorf("split tick %d: %w", i, err)
		}
		opts := perspOpts()
		opts.Weights = q.weights
		if err := drawQuad(dc, tex, q, opts); err != nil {
			return fmt.Errorf("split plate %d: %w", i, err)
		}
	}
	return nil
}

// paintTintRun：臂 b —— 前三张琥珀染色、后三张不染色，各自连成一段。
func paintTintRun(dc render.Canvas, tex *render.ImageBuf, plates []quad) error {
	amber := render.RGBA{R: amberR, G: amberG, B: amberB, A: 1}
	for i, q := range plates {
		opts := perspOpts()
		opts.Weights = q.weights
		if i < platesPerRow/2 {
			opts.Tint = amber
		}
		if err := drawQuad(dc, tex, q, opts); err != nil {
			return fmt.Errorf("tint plate %d: %w", i, err)
		}
	}
	return nil
}

// atlasSpritesThroughEngine 用引擎出口把世界矩形折成屏幕四角 + 权重（批 1c 正路）。
func atlasSpritesThroughEngine(pan float64) ([]render.AtlasSprite, int) {
	proj, err := camera.NewProjector(focal, core.V2(winW/2, winH/2), core.Vec2{})
	if err != nil {
		return nil, 0
	}
	// 世界系以投影中心为原点：先把想要的屏幕落点按中段深度折回世界系，
	// 出口回来的四角才会落在这一带上。图集腿走 DrawAtlasEx —— 老口按批 1c
	// 的 D-4 契约不读精灵自带的透视字段。
	midDepth := (plateNear + plateFar) / 2
	k := (focal + midDepth) / focal
	sprites := make([]render.AtlasSprite, 0, atlasSprites)
	for i := 0; i < atlasSprites; i++ {
		cx := atlasFirstX + float64(i)*atlasStepX + pan
		world := core.Rect{
			X: (cx-winW/2)*k - atlasHW*k, Y: (atlasRowY-winH/2)*k - atlasHH*k,
			W: atlasHW * 2 * k, H: atlasHH * 2 * k,
		}
		corners, weights, ok := proj.ProjectSpriteQuad(world, [4]float64{plateFar, plateFar, plateNear, plateNear})
		if !ok {
			continue
		}
		tile := i % 4
		sprites = append(sprites, render.AtlasSprite{
			SrcX: float64((tile % 2) * int(atlasTile)), SrcY: float64((tile / 2) * int(atlasTile)),
			SrcW: atlasTile, SrcH: atlasTile,
			Perspective: true, Corners: corners, Weights: weights,
			Filter: render.InterpNearest,
		})
	}
	return sprites, len(sprites)
}

// paintRefusalBand：臂 c + 眼后 + 非法权重 —— 拒绝态要在画面里看得见（被拒的那块
// 保持底色，成功的两块正常出图）。返回各哨兵命中情况。
type refusalFacts struct {
	perspBicubicRefused bool
	badWeightsRefused   bool
	behindEyeRefused    bool
	affineBicubicOK     bool
}

func paintRefusalBand(dc render.Canvas, tex *render.ImageBuf) refusalFacts {
	var f refusalFacts
	q, ok := plate(180, refusalY, 40, 26, plateNear, plateFar)
	if ok {
		opts := perspOpts()
		opts.Interpolation = render.InterpBicubic
		opts.Weights = q.weights
		f.perspBicubicRefused = errors.Is(drawQuad(dc, tex, q, opts), render.ErrQuadPerspectiveBicubic)
	}
	if ok {
		opts := perspOpts()
		opts.Weights = [4]float64{1, math.NaN(), 1, 1}
		f.badWeightsRefused = errors.Is(drawQuad(dc, tex, q, opts), render.ErrQuadBadWeights)
	}
	// 眼后：把深度压到焦点之后，投影出口必须整块拒绝（ok=false），
	// 这一格不许被画上一笔（画家序 behind-eye 语义，与图集腿同款）。
	if _, okB := plate(420, refusalY, 40, 26, -focal*2, plateFar); !okB {
		f.behindEyeRefused = true
	}
	if aff, okA := plate(660, refusalY, 40, 26, plateNear, plateFar); okA {
		opts := render.QuadDrawOptions{Interpolation: render.InterpBicubic, Opacity: 1}
		f.affineBicubicOK = drawQuad(dc, tex, aff, opts) == nil
	}
	// 合法透视对照：同一形状走最近邻，必须画上。
	if q2, ok2 := plate(900, refusalY, 40, 26, plateNear, plateFar); ok2 {
		opts := perspOpts()
		opts.Weights = q2.weights
		_ = drawQuad(dc, tex, q2, opts)
	}
	return f
}

// paintProjectionLayer：臂 e —— Push/Pop 投影层 + 圆角裁剪下的远处告示牌，
// 复位后投影必须回到恒等。drawText=true 时多两笔文字（fail-closed 证据臂）：
// 投影段那一笔应被引擎整笔拒画（TextSkips++、像素零落墨），Pop 后恒等段
// 同一串字必须正常在场。
func paintProjectionLayer(dc render.Canvas, tex *render.ImageBuf, drawText bool) (pushedOK, restoredOK bool) {
	m, ok := render.ProjectorMatrix4(focal*1.4, render.Pt(990, billboardY), render.Point{})
	if !ok {
		return false, false
	}
	before := dc.Projection()
	dc.Push()
	dc.PushProjection(m)
	pushedOK = dc.Projection() != before
	dc.ClipRoundRect(900, billboardY-70, 220, 140, 24)
	if q, okq := plate(990, billboardY, 60, 44, plateNear, plateFar); okq {
		opts := perspOpts()
		opts.Weights = q.weights
		_ = drawQuad(dc, tex, q, opts)
	}
	if drawText && perspFace != nil {
		dc.SetFont(perspFace)
		dc.SetRGB(0.95, 0.95, 0.98)
		dc.DrawString("投影下文字应整笔缺席", 916, billboardY+58)
	}
	dc.Pop()
	restoredOK = dc.Projection() == before
	if drawText && perspFace != nil {
		dc.SetFont(perspFace)
		dc.SetRGB(0.95, 0.95, 0.98)
		dc.DrawString("恒等段文字在场", 916, billboardY+80)
	}
	return pushedOK, restoredOK
}

// paintBackdrop 画天幕与地面板底色（形状命令，排在图像段之前，不参与合批计数）。
func paintBackdrop(dc render.Canvas, w, h float64) {
	dc.ClearWithColor(render.RGBA{R: backdropRGB, G: backdropRGB, B: backdropRGB + 0.02, A: 1})
	dc.SetRGBA(0.10, 0.11, 0.16, 1)
	dc.DrawRectangle(0, 300, w, h-300)
	_ = dc.Fill()
	dc.SetRGBA(0.16, 0.17, 0.24, 1)
	dc.DrawRectangle(0, 300, w, 3)
	_ = dc.Fill()
}

// paintAll 是窗与所有离屏探针共用的唯一画面函数。
func paintAll(dc render.Canvas, tex, sheet *render.ImageBuf, w, h, pan float64) refusalFacts {
	paintBackdrop(dc, w, h)
	if err := paintMergedRun(dc, tex, plateRow(rowAY, pan)); err != nil {
		panic(err)
	}
	if err := paintSplitRun(dc, tex, plateRow(rowBY, -pan), rowBY+plateHH+22); err != nil {
		panic(err)
	}
	if err := paintTintRun(dc, tex, plateRow(rowCY, pan)); err != nil {
		panic(err)
	}
	if sprites, n := atlasSpritesThroughEngine(-pan); n > 0 {
		_, _ = dc.DrawAtlasEx(sheet, sprites, render.AtlasDrawOptions{})
	}
	facts := paintRefusalBand(dc, tex)
	paintProjectionLayer(dc, tex, true)
	return facts
}

func newCtx(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	return dc
}

func pixelAt(dc *render.Context, x, y int) (uint8, uint8, uint8, uint8) {
	img := dc.Image()
	cr, cg, cb, ca := img.At(x, y).RGBA()
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)
}

func censusOf(dc *render.Context, paint func()) (render.ImageBatchStats, error) {
	dc.ResetImageBatchStats()
	paint()
	if err := dc.FlushGPU(); err != nil {
		return render.ImageBatchStats{}, err
	}
	return dc.ImageBatchStats(), nil
}

// ---------------------------------------------------------------------------
// 探针：逻辑（合批账、哨兵、投影栈）+ 像素（同一段并排逐位相同）
// ---------------------------------------------------------------------------

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

func runProbes(tex, sheet *render.ImageBuf) probeReport {
	var rep probeReport
	const w, h = 1200, 800

	// 臂 a：合并段一 draw 吃六张。
	dc := newCtx(w, h)
	st, err := censusOf(dc, func() {
		paintBackdrop(dc, w, h)
		if e := paintMergedRun(dc, tex, plateRow(rowAY, 0)); e != nil {
			panic(e)
		}
	})
	if err != nil {
		rep.pixelErr = fmt.Errorf("merged probe flush: %w", err)
	}
	rep.add("merged_run_one_draw", st.PerspDraws == 1 && st.MaxPerspRun == platesPerRow,
		fmt.Sprintf("draws=%d max_run=%d", st.PerspDraws, st.MaxPerspRun))
	rep.extra = map[string]any{}
	rep.extra["merged_persp_draws"] = st.PerspDraws
	rep.extra["merged_persp_max_run"] = st.MaxPerspRun
	rep.extra["merged_draws"] = st.Draws

	// 臂 a 反面：夹一笔描边就断链（H12 第一次被量化）。
	dc2 := newCtx(w, h)
	st2, err := censusOf(dc2, func() {
		paintBackdrop(dc2, w, h)
		if e := paintSplitRun(dc2, tex, plateRow(rowBY, 0), rowBY+plateHH+22); e != nil {
			panic(e)
		}
	})
	if err != nil {
		rep.pixelErr = fmt.Errorf("split probe flush: %w", err)
	}
	rep.add("interleaved_path_breaks_run", st2.PerspDraws == platesPerRow && st2.MaxPerspRun == 1,
		fmt.Sprintf("draws=%d max_run=%d", st2.PerspDraws, st2.MaxPerspRun))
	rep.extra["split_persp_draws"] = st2.PerspDraws
	rep.extra["split_persp_max_run"] = st2.MaxPerspRun
	dc.Close()
	dc2.Close()

	// 合批不改画面：同一批几何，连排与夹描边两种命令序，在面板落点区必须逐位相同。
	merged := newCtx(w, h)
	split := newCtx(w, h)
	paintAllQuiet(merged, tex, sheet, w, h, 0, true)
	paintAllQuiet(split, tex, sheet, w, h, 0, false)
	diff := countDiff(merged, split, rowAY)
	rep.add("merge_keeps_picture_bitwise", diff == 0, fmt.Sprintf("band diff px=%d want %d", diff, mergeBitExact))
	rep.extra["merge_vs_split_diff_px"] = diff
	merged.Close()
	split.Close()

	// 臂 b：染色 vs 不染色，亮带中心必须被压向琥珀色，且不染色那张保持原样。
	tinted := newCtx(w, h)
	paintAllQuiet(tinted, tex, sheet, w, h, 0, false)
	qs := plateRow(rowCY, 0)
	tr, tg, tb, ta := pixelAt(tinted, int(qs[0].center().X), int(qs[0].center().Y))
	wr, wg, wb, wa := pixelAt(tinted, int(qs[platesPerRow-1].center().X), int(qs[platesPerRow-1].center().Y))
	// 染色是 straight 域逐通道乘：tint=(1,0.62,0.28) 下红通道保持、绿蓝按比例压下去。
	wantG := int(float64(wr)*0.62 + 0.5)
	wantB := int(float64(wr)*0.28 + 0.5)
	tintOK := absDiff(int(tr), int(wr)) <= tintSlackSteps &&
		absDiff(int(tg), wantG) <= tintSlackSteps &&
		absDiff(int(tb), wantB) <= tintSlackSteps && ta == wa
	rep.add("tint_lights_projective_leg", tintOK,
		fmt.Sprintf("tinted=(%d,%d,%d,%d) want≈(%d,%d,%d,%d) plain=(%d,%d,%d,%d)",
			tr, tg, tb, ta, wr, wantG, wantB, wa, wr, wg, wb, wa))
	rep.extra["tint_probe_rgba"] = []int{int(tr), int(tg), int(tb), int(ta)}
	rep.extra["plain_probe_rgba"] = []int{int(wr), int(wg), int(wb), int(wa)}
	tinted.Close()

	// 等权透视 ≡ 仿射（逐位），真透视必须与仿射不同。
	eq := newCtx(w, h)
	aff := newCtx(w, h)
	persp := newCtx(w, h)
	oneQuad := func(dc *render.Context, perspective bool, weights [4]float64) {
		paintBackdrop(dc, w, h)
		q, ok := plate(600, 400, 120, 80, plateNear, plateFar)
		if !ok {
			panic("plate refused")
		}
		opts := render.QuadDrawOptions{Perspective: perspective, Weights: weights,
			Interpolation: render.InterpNearest, Opacity: 1}
		if e := drawQuad(dc, tex, q, opts); e != nil {
			panic(e)
		}
		_ = dc.FlushGPU()
	}
	oneQuad(eq, true, [4]float64{1, 1, 1, 1})
	oneQuad(aff, false, [4]float64{})
	qReal, _ := plate(600, 400, 120, 80, plateNear, plateFar)
	oneQuad(persp, true, qReal.weights)
	rep.add("equal_weights_equal_affine", countDiff(eq, aff, 400) == 0, "见 diff")
	rep.add("honest_perspective_differs", countDiff(persp, aff, 400) > 0, "见 diff")
	rep.extra["equal_weights_diff_px"] = countDiff(eq, aff, 400)
	rep.extra["honest_perspective_diff_px"] = countDiff(persp, aff, 400)
	eq.Close()
	aff.Close()
	persp.Close()

	// 臂 c + 眼后 + 非法权重：四类表态各就各位。
	ref := newCtx(w, h)
	paintBackdrop(ref, w, h)
	facts := paintRefusalBand(ref, tex)
	_ = ref.FlushGPU()
	br, bg, bb, _ := pixelAt(ref, 420, int(refusalY))
	rep.add("perspective_bicubic_refused", facts.perspBicubicRefused, fmt.Sprintf("%v", facts.perspBicubicRefused))
	rep.add("bad_weights_refused", facts.badWeightsRefused, fmt.Sprintf("%v", facts.badWeightsRefused))
	rep.add("behind_eye_refused", facts.behindEyeRefused, fmt.Sprintf("%v", facts.behindEyeRefused))
	rep.add("affine_bicubic_still_draws", facts.affineBicubicOK, fmt.Sprintf("%v", facts.affineBicubicOK))
	rep.add("behind_eye_painted_nothing", br < 200 && bg < 200 && bb < 200,
		fmt.Sprintf("eye region=(%d,%d,%d) 应保持底色", br, bg, bb))
	ref.Close()

	// 臂 d：图集批绘腿连排五张必须也走同一段合并。
	atl := newCtx(w, h)
	var atlasFacts render.AtlasDrawResult
	st3, err := censusOf(atl, func() {
		paintBackdrop(atl, w, h)
		sprites, n := atlasSpritesThroughEngine(0)
		if n == 0 {
			panic("引擎出口整批被拒")
		}
		res, e := atl.DrawAtlasEx(sheet, sprites, render.AtlasDrawOptions{})
		atlasFacts = res
		if e != nil {
			panic(e)
		}
	})
	if err != nil {
		rep.pixelErr = fmt.Errorf("atlas probe flush: %w", err)
	}
	rep.add("atlas_engine_leg_merges",
		st3.PerspDraws == 1 && st3.MaxPerspRun == atlasSprites &&
			atlasFacts.Drawn == atlasSprites && !atlasFacts.Degraded && !atlasFacts.Skipped,
		fmt.Sprintf("draws=%d max_run=%d drawn=%d degraded=%v skipped=%v",
			st3.PerspDraws, st3.MaxPerspRun, atlasFacts.Drawn, atlasFacts.Degraded, atlasFacts.Skipped))
	rep.extra["atlas_persp_draws"] = st3.PerspDraws
	rep.extra["atlas_persp_max_run"] = st3.MaxPerspRun
	rep.extra["atlas_drawn"] = atlasFacts.Drawn
	rep.extra["atlas_degraded"] = atlasFacts.Degraded
	rep.extra["atlas_reason"] = atlasFacts.Reason
	atl.Close()

	// 臂 e：投影层四动词 + 复位恒等 + 计数 + 「投影下文字 fail-closed」双跑对拍
	// （带字 vs 不带字两个离屏 ctx：投影段必须零差=拒画彻底，恒等段必须见墨）。
	proj := newCtx(w, h)
	proj.ResetProjectionStats()
	paintBackdrop(proj, w, h)
	pushed, restored := paintProjectionLayer(proj, tex, true)
	_ = proj.FlushGPU()
	ps := proj.ProjectionStats()
	rep.add("projection_stack_push_restore", pushed && restored, fmt.Sprintf("pushed=%v restored=%v", pushed, restored))
	rep.extra["projection_stats"] = map[string]int{
		"clip_degrades": ps.ClipDegrades, "path_expansions": ps.PathExpansions,
		"text_skips": ps.TextSkips, "interp_downgrades": ps.InterpDowngrades,
	}
	ctrl := newCtx(w, h)
	paintBackdrop(ctrl, w, h)
	paintProjectionLayer(ctrl, tex, false)
	_ = ctrl.FlushGPU()
	projInk := regionInk(ctrl.Image(), proj.Image(), 916, int(billboardY+42), 210, 22)
	identInk := regionInk(ctrl.Image(), proj.Image(), 916, int(billboardY+68), 210, 22)
	textFC := perspFace != nil && ps.TextSkips >= 1 && projInk == 0 && identInk > 40
	rep.add("text_under_projection_failclosed", textFC, fmt.Sprintf(
		"text_skips=%d 投影段墨差=%d want0 恒等段墨差=%d want>40 face=%v", ps.TextSkips, projInk, identInk, perspFace != nil))
	rep.extra["text_failclosed_ink"] = map[string]int{"proj_strip": projInk, "identity_strip": identInk}
	proj.Close()
	ctrl.Close()

	// 引擎出口 fail-closed：非有限深度必须整块拒绝，不画也不炸。
	badProj, errb := camera.NewProjector(focal, core.V2(600, 400), core.Vec2{})
	if errb == nil {
		_, _, okBad := badProj.ProjectSpriteQuad(core.Rect{X: 100, Y: 100, W: 80, H: 40},
			[4]float64{math.NaN(), 300, 300, 300})
		rep.add("engine_export_rejects_nonfinite", !okBad, fmt.Sprintf("ok=%v want false", okBad))
	}
	return rep
}

// paintAllQuiet 只给离屏探针用（需要 FlushGPU，那不在 Canvas 上）。
func paintAllQuiet(dc *render.Context, tex, sheet *render.ImageBuf, w, h, pan float64, mergedOnlyRowA bool) {
	paintBackdrop(dc, w, h)
	if err := paintMergedRun(dc, tex, plateRow(rowAY, pan)); err != nil {
		panic(err)
	}
	if err := paintSplitRun(dc, tex, plateRow(rowBY, pan), rowBY+plateHH+22); err != nil {
		panic(err)
	}
	if mergedOnlyRowA {
		return
	}
	if err := paintTintRun(dc, tex, plateRow(rowCY, pan)); err != nil {
		panic(err)
	}
	if sprites, n := atlasSpritesThroughEngine(-pan); n > 0 {
		_, _ = dc.DrawAtlasEx(sheet, sprites, render.AtlasDrawOptions{})
	}
	paintRefusalBand(dc, tex)
	paintProjectionLayer(dc, tex, true)
	_ = dc.FlushGPU()
}

// regionInk 数 a、b 两张回读图在矩形带内的差异像素数——文字 fail-closed 臂的
// 墨量对照（带字跑 vs 不带字跑）：投影段应 0（整笔被拒），恒等段应见墨。
func regionInk(a, b image.Image, x0, y0, wd, ht int) int {
	n := 0
	for y := y0; y < y0+ht; y++ {
		for x := x0; x < x0+wd; x++ {
			if x < 0 || y < 0 || x >= a.Bounds().Dx() || y >= a.Bounds().Dy() {
				continue
			}
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				n++
			}
		}
	}
	return n
}

// countDiff 只在某一行带的板面落点区比像素（合并段 vs 拆开段必须逐位相同）。
func countDiff(a, b *render.Context, bandY float64) int {
	ia, ib := a.Image(), b.Image()
	diff := 0
	for _, q := range plateRow(bandY, 0) {
		c := q.center()
		for dy := -20; dy <= 20; dy += 2 {
			for dx := -30; dx <= 30; dx += 2 {
				x, y := int(c.X)+dx, int(c.Y)+dy
				if x < 0 || y < 0 || x >= ia.Bounds().Dx() || y >= ia.Bounds().Dy() {
					continue
				}
				ar, ag, ab, aa := ia.At(x, y).RGBA()
				br, bg, bb, ba := ib.At(x, y).RGBA()
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	return diff
}

func semanticProbes(pan float64) []wrkit.PixelProbe {
	probes := make([]wrkit.PixelProbe, 0, 8)
	for i, q := range plateRow(rowAY, pan) {
		c := q.center()
		probes = append(probes, wrkit.PixelProbe{
			Name: fmt.Sprintf("merged_plate_%d_drawn", i), X: c.X, Y: c.Y,
			R: 0.75, G: 0.75, B: 0.75, Tol: 16,
		})
	}
	tinted := plateRow(rowCY, pan)
	c0 := tinted[0].center()
	cLast := tinted[platesPerRow-1].center()
	probes = append(probes,
		wrkit.PixelProbe{Name: "tinted_plate_amber", X: c0.X, Y: c0.Y, R: plateProbeLum, G: plateProbeLum * amberG, B: plateProbeLum * amberB, Tol: tintSlackSteps},
		wrkit.PixelProbe{Name: "plain_plate_neutral", X: cLast.X, Y: cLast.Y, R: 0.75, G: 0.75, B: 0.75, Tol: 16},
		wrkit.PixelProbe{Name: "behind_eye_stays_backdrop", X: 420, Y: refusalY, R: 0.10, G: 0.11, B: 0.16, Tol: 6},
	)
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
	initFace()

	tex := newPlateTexture(64, 64, 250, 250, 252)
	sheet := newSheetTexture()
	t0 := time.Now()

	// 离屏证据先出（开窗后再拿 GPU 深度会失真）。
	rep := runProbes(tex, sheet)
	probesOK := true
	probeMap := map[string]any{}
	for _, r := range rep.results {
		probeMap[r.name] = r.ok
		if !r.ok {
			probesOK = false
			fmt.Fprintf(os.Stderr, "p37_persp_batch: 探针 FAIL %s → %s\n", r.name, r.got)
		}
	}
	var summary manualSummary
	var lastBatch render.ImageBatchStats
	var paintErr string
	var liveDC *render.Context

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
	pan := 0.0
	content.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		// 生产路径每帧是把 OnPaint 录进 *recording.Recorder 再回放的
		// （pipeline_app 的 recordFullFrame），只有 warm-up/直绘那一笔是
		// *render.Context。早先这里写的是「断言取不到就不画」，结果第二帧起
		// 整窗静止、合批账也停在 warm-up —— 绘制一律走 render.Canvas，
		// 活 Context 只用来读合批账（它不在 Canvas 上）。
		if live, okCtx := pc.DC.(*render.Context); okCtx {
			liveDC = live
		}
		dc := pc.DC
		// H15：相位只随已完成帧数走，过 settling 帧定格（末帧与墙钟无关）。
		idx := frameNo
		if gateMode && idx > gateFreezeAt {
			idx = gateFreezeAt
		}
		pan = math.Sin(float64(idx)/gateHz*1.1) * 18
		w, h := size.Width, size.Height
		paintBackdrop(dc, w, h)
		if e := paintMergedRun(dc, tex, plateRow(rowAY, pan)); e != nil {
			paintErr = e.Error()
		}
		if e := paintSplitRun(dc, tex, plateRow(rowBY, pan), rowBY+plateHH+22); e != nil {
			paintErr = e.Error()
		}
		if e := paintTintRun(dc, tex, plateRow(rowCY, pan)); e != nil {
			paintErr = e.Error()
		}
		if sprites, n := atlasSpritesThroughEngine(-pan); n > 0 {
			_, _ = dc.DrawAtlasEx(sheet, sprites, render.AtlasDrawOptions{})
		}
		paintRefusalBand(dc, tex)
		paintProjectionLayer(dc, tex, true)
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
	snapPath := filepath.Join(testdataDir, "p37_persp_batch_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: backdropRGB, ClearG: backdropRGB, ClearB: backdropRGB + 0.02, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		MaxFrames:    gateMaxFrames,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "p37_persp_batch: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "p37_persp_batch: resize %dx%d\n", ev.Width, ev.Height)
				setTitle()
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					content.FixedWidth, content.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
			case platform.EventPointer:
				summary.Pointer++
				if summary.Pointer%24 == 1 {
					fmt.Fprintf(os.Stderr, "p37_persp_batch: pointer kind=%v @(%.0f,%.0f)\n",
						ev.Pointer, ev.X, ev.Y)
					setTitle()
				}
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "p37_persp_batch: key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "p37_persp_batch: event %s\n", ev.Type)
			}
		},
	})
	var proc scheduler.ProcessTracker
	proc.Start()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) bool {
		// 每帧先收上一整帧的合批账再清零：录制回放同样落在同一个活 Context 上，
		// 所以窗内读数就是「这一扇窗真跑过合并腿」的证据（H12）。
		if liveDC != nil {
			lastBatch = liveDC.ResetImageBatchStats()
		}
		metric.SetText(fmt.Sprintf("presents=%d persp_draws=%d max_run=%d",
			app.PresentCount(), lastBatch.PerspDraws, lastBatch.MaxPerspRun))
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
		fmt.Fprintf(os.Stderr, "p37_persp_batch: FAIL 定帧停机没达成 presents=%d want>=%d（末帧随墙钟漂）\n",
			app.PresentCount(), gateMaxFrames)
		probesOK = false
	}

	// 金图按后端分文件，fail-closed：缺基线判红，播种那一跑也不算过。
	tag := backendTag()
	basePath := filepath.Join(testdataDir, "goldens", fmt.Sprintf("p37_persp_batch_final_%s.png", tag))
	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGateW(snapPath, basePath, winW,
		[]wrkit.GoldenRect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}})
	goldenOK := goldenErr == nil && !seeded && goldenDiff == 0
	if goldenErr != nil {
		fmt.Fprintf(os.Stderr, "p37_persp_batch: golden: %v\n", goldenErr)
	}
	if seeded {
		fmt.Fprintf(os.Stderr, "p37_persp_batch: golden 已播种（%s），这一跑不算 PASS —— 复核后提交基线再复跑\n", basePath)
	}

	pixelOK := paintErr == ""
	if gateMode {
		if pixelErr := wrkit.CheckPixelProbes(snapPath, winW, semanticProbes(pan)); pixelErr != nil {
			pixelOK = false
			fmt.Fprintf(os.Stderr, "p37_persp_batch: 像素断言 FAIL: %v\n", pixelErr)
		}
	} else {
		fmt.Fprintln(os.Stderr, "p37_persp_batch: 常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿")
	}
	if paintErr != "" {
		fmt.Fprintf(os.Stderr, "p37_persp_batch: 绘制 FAIL: %s\n", paintErr)
	}

	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	extra := map[string]any{
		"probes_ok": probesOK, "pixel_ok": pixelOK, "paint_err": paintErr,
		"golden_diff_pct": goldenDiff, "golden_total_px": goldenPx,
		"golden_seeded": seeded, "golden_backend_tag": tag,
		// 窗内上一整帧的合批账（逐帧读-清所得）。MaxRun>=2 才算「合并腿在这扇窗里
		// 真的跑过」——H12 要的就是这条，不看离屏探针代答。
		"window_frame_persp_draws": lastBatch.PerspDraws,
		"window_frame_max_run":     lastBatch.MaxPerspRun,
		"window_frame_draws":       lastBatch.Draws,
		"window_frame_quads":       lastBatch.Quads,
		"window_leg_merged":        lastBatch.MaxPerspRun >= 2,
		"gate_frames":              gateFrames, "gate_freeze_at": gateFreezeAt,
		"golden_tolerance_pct": 0,
	}
	for k, v := range rep.extra {
		extra[k] = v
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "p37-6-1", Scenario: "p37_persp_batch", Backend: tag,
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
		if lastBatch.MaxPerspRun < 2 {
			fmt.Fprintf(os.Stderr, "FAIL: 窗内没有发生任何合并（max_run=%d，want >=2）——H12 的硬要求落空\n",
				lastBatch.MaxPerspRun)
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
		fmt.Fprintf(os.Stderr, "p37_persp_batch: OK backend=%s presents=%d probes=%d/%d golden=%.4f%% (%d px) merged=%d/%d\n",
			tag, app.PresentCount(), len(rep.results), len(rep.results), goldenDiff, goldenPx,
			lastBatch.PerspDraws, lastBatch.MaxPerspRun)
		return
	}
	fmt.Fprintf(os.Stderr, "p37_persp_batch: resident done backend=%s presents=%d probes_ok=%v pixel_ok=%v golden=%.4f%%\n",
		tag, app.PresentCount(), probesOK, pixelOK, goldenDiff)
}

type manualSummary struct {
	Pointer, Key, Resize int
}
