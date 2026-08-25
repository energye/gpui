package main

// 按 pelican-bike.html 参考实现逐元素移植：
// 舞台坐标系 = SVG viewBox 1200×700，每帧按窗口尺寸等比缩放（min(w/1200,h/700)）居中。
// 绘制走 PaintContext 公开 API：帧首推 T(ox,oy)·S(s) 基矩阵后全部用 SVG 原始坐标，
// 描边宽度随 CTM 缩放（Skia/Cairo 惯例），组旋转用 Save/RotateAbout/RestoreCanvas 嵌套，
// 与 SVG transform 组语义一一对应。静态路径只在初始化时构建一次。

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

// ---- 舞台与几何常量（SVG/JS 原始数值）----

const (
	stageW = 1200.0
	stageH = 700.0

	baseV          = 270.0 // 基础车速 px/s
	wheelR         = 72.0  // 轮半径
	crankR         = 40.0  // 曲柄半径
	bbX            = 620.0 // 五通（曲柄中心）
	bbY            = 530.0
	axleRX, axleRY = 505.0, 514.0 // 后轮轴
	axleFX, axleFY = 775.0, 514.0 // 前轮轴
	hipNX, hipNY   = 576.0, 386.0 // 近侧髋
	hipFX, hipFY   = 566.0, 382.0 // 远侧髋
	legL1, legL2   = 100.0, 100.0 // 大腿 / 小腿长度

	sunX, sunY = 1072.0, 112.0

	hudPadLR   = 16.0  // HUD 药丸左右内边距
	hudBtnSize = 30.0  // 暂停按钮直径
	hudGap     = 10.0  // 控件间距
	hudSliderW = 130.0 // 滑条长（与 input[type=range] width 一致）
	hudLabelW  = 46.0  // 速度标签宽（b{min-width:46px}）
)

var hudW = hudPadLR*2 + hudBtnSize + hudGap + hudSliderW + hudGap + hudLabelW // ≈258
const hudH = 48.0

// 视差层参数（JS layers）：系数 f、平铺周期 p
type parallaxLayer struct {
	f, p float64
}

var (
	layerHills  = parallaxLayer{.22, 1200}
	layerTrees  = parallaxLayer{.50, 800}
	layerDashes = parallaxLayer{1.0, 130}
	layerTufts  = parallaxLayer{1.35, 600}
)

// ---- 小工具 ----

func rad(deg float64) float64 { return deg * math.Pi / 180 }

func rgb(hex int64) (float64, float64, float64) {
	return float64((hex>>16)&0xff) / 255, float64((hex>>8)&0xff) / 255, float64(hex&0xff) / 255
}

func frac01(x float64) float64 {
	m := math.Mod(x, 1)
	if m < 0 {
		m++
	}
	return m
}

// smilAB 复刻 SMIL values="A;B;A" 线性插值：返回偏向 B 的比例 u∈[0,1]（三角波）。
func smilAB(t, dur, begin float64) float64 {
	p := frac01((t - begin) / dur)
	if p < .5 {
		return p * 2
	}
	return 2 - p*2
}

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- 仿真状态 ----

// pelicanSim 双时钟模型：ambientT 对应页面时间（CSS 动画 / SMIL 永不停），
// 骑行量（dist/wheelDeg/crank）只在非暂停时推进（对应 JS frame() 的 paused 早退）。
type pelicanSim struct {
	ambientT float64
	dist     float64
	wheelDeg float64
	crank    float64 // 弧度，初始 π/4 与 JS 一致
	speed    float64 // ×0.3..×2.5
	paused   bool
	frozen   bool // PELICAN_T 确定性模式：tick 不推进
}

func newPelicanSim() *pelicanSim { return &pelicanSim{crank: math.Pi * .25, speed: 1} }

// freezeFromEnv 读 PELICAN_T（秒）：设置时把仿真状态固定到该时刻并停止推进，
// 用于确定性单帧渲染（诊断对比用）。
func (s *pelicanSim) freezeFromEnv() {
	v := os.Getenv("PELICAN_T")
	if v == "" {
		return
	}
	t, err := strconv.ParseFloat(v, 64)
	if err != nil || t < 0 {
		return
	}
	s.frozen = true
	s.ambientT = t
	dist := baseV * t
	s.dist = dist
	s.wheelDeg = math.Mod(dist/wheelR*180/math.Pi, 360)
	s.crank = math.Pi*.25 + dist/wheelR/2.6
}

func (s *pelicanSim) tick(dt float64) {
	if s == nil || s.frozen {
		return
	}
	if dt > .05 { // 与 JS 一致：切后台回来不跳帧
		dt = .05
	}
	s.ambientT += dt
	if s.paused {
		return
	}
	v := baseV * s.speed
	s.dist += v * dt
	s.wheelDeg = math.Mod(s.wheelDeg+v*dt/wheelR*180/math.Pi, 360)
	s.crank += v * dt / wheelR / 2.6
}

// pedal 返回两侧踏板位置：p1 近侧，p2 远侧（对侧）。
func (s *pelicanSim) pedal() (p1x, p1y, p2x, p2y float64) {
	c, sn := math.Cos(s.crank), math.Sin(s.crank)
	return bbX + c*crankR, bbY + sn*crankR, bbX - c*crankR, bbY - sn*crankR
}

// knee 两段式逆运动学：给定髋与踝，求膝（取偏前方一支），公式与 JS 相同。
func knee(hx, hy, tx, ty, l1, l2 float64) (kx, ky float64) {
	dx, dy := tx-hx, ty-hy
	raw := math.Hypot(dx, dy)
	if raw == 0 {
		raw = 1
	}
	d := clampf(raw, math.Abs(l1-l2)+.5, l1+l2-.5)
	ux, uy := dx/raw, dy/raw
	a := (l1*l1 - l2*l2 + d*d) / (2 * d)
	h := math.Sqrt(math.Max(0, l1*l1-a*a))
	mx, my := hx+ux*a, hy+uy*a
	k1x, k1y := mx+uy*h, my-ux*h
	k2x, k2y := mx-uy*h, my+ux*h
	if k1x > k2x {
		return k1x, k1y
	}
	return k2x, k2y
}

// ---- 场景组装 ----

type pelicanScene struct {
	Root *rendering.AbsoluteBox

	stageBox *rendering.RenderBox
	hudBox   *rendering.RenderBox

	sim *pelicanSim

	face       text.Face
	speedLabel *rendering.RenderText
	tip        *rendering.RenderText

	titleGlyphs []titleGlyph // 主标题（含仿粗体副本）
	subGlyphs   []titleGlyph
	titleTotal  float64
	subTotal    float64

	hudX, hudY     float64 // HUD 药丸在窗口中的位置（右下 fixed）
	btnHover       bool
	draggingSlider bool
}

// titleGlyph 标题单字节点；dup 为仿粗体副本（右移重画一遍模拟 font-weight:700）。
type titleGlyph struct {
	node *rendering.RenderText
	dup  *rendering.RenderText
	dx   float64 // 距标题起点的累计步进（含 letter-spacing，舞台 px）
}

func newPelicanScene(winW, winH float64) *pelicanScene {
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.47, G: 0.78, B: 0.96, A: 1}

	sc := &pelicanScene{Root: root, sim: newPelicanSim()}
	sc.sim.freezeFromEnv()
	face, _, faceErr := wrkit.EnsureUIFace()
	sc.face = face

	// 全屏舞台
	sc.stageBox = rendering.NewRenderBox()
	sc.stageBox.FixedWidth = winW
	sc.stageBox.FixedHeight = winH
	sc.stageBox.OnPaint = sc.paintStage
	root.Place(sc.stageBox, 0, 0)

	// 标题（SVG 内文字：鹈鹕骑行记 / PELICAN RIDER，带 letter-spacing 居中）
	sc.titleGlyphs, sc.titleTotal = newTitleLine("鹈鹕骑行记", 30, 6, 0x17456b, .85, face, true)
	sc.subGlyphs, sc.subTotal = newTitleLine("PELICAN RIDER", 14, 4, 0x4a7ba6, .8, face, false)
	for i := range sc.titleGlyphs {
		root.Place(sc.titleGlyphs[i].node, 0, -100)
		if d := sc.titleGlyphs[i].dup; d != nil {
			root.Place(d, 0, -100)
		}
	}
	for i := range sc.subGlyphs {
		root.Place(sc.subGlyphs[i].node, 0, -100)
	}

	// 左下提示（position:fixed left/bottom）
	sc.tip = rendering.NewRenderText("空格 暂停 · ↑/↓ 调速")
	sc.tip.FontSize = 12
	tr, tg, tb := rgb(0x8fa6c4)
	sc.tip.R, sc.tip.G, sc.tip.B, sc.tip.A = tr, tg, tb, 1
	if faceErr == nil {
		sc.tip.SetFace(face)
	}
	root.Place(sc.tip, 18, winH-32)

	// 速度标签（HUD 内动态文本）
	sc.speedLabel = rendering.NewRenderText("×1.0")
	sc.speedLabel.FontSize = 14
	lr, lg, lb := rgb(0x222233)
	sc.speedLabel.R, sc.speedLabel.G, sc.speedLabel.B, sc.speedLabel.A = lr, lg, lb, 1
	if faceErr == nil {
		sc.speedLabel.SetFace(face)
	}
	root.Place(sc.speedLabel, 0, -100)

	// HUD 药丸（右下 fixed）
	sc.hudBox = rendering.NewRenderBox()
	sc.hudBox.FixedWidth = hudW
	sc.hudBox.FixedHeight = hudH
	sc.hudBox.OnPaint = sc.paintHUD
	root.Place(sc.hudBox, winW-18-hudW, winH-18-hudH)
	sc.hudX, sc.hudY = winW-18-hudW, winH-18-hudH

	sc.relayout(winW, winH)
	return sc
}

// newTitleLine 构建一行带 letter-spacing 的逐字节点并测量总宽（舞台 px）。
func newTitleLine(s string, fontSize, letterSpacing float64, hex int64, alpha float64, face text.Face, bold bool) ([]titleGlyph, float64) {
	r, g, b := rgb(hex)
	var out []titleGlyph
	dx := 0.0
	for _, rn := range s {
		ch := string(rn)
		mk := func() *rendering.RenderText {
			t := rendering.NewRenderText(ch)
			t.FontSize = fontSize
			t.R, t.G, t.B, t.A = r, g, b, alpha
			if face != nil {
				t.SetFace(face)
			}
			return t
		}
		glyph := titleGlyph{node: mk(), dx: dx}
		if bold {
			glyph.dup = mk()
		}
		out = append(out, glyph)
		dx += glyph.node.MeasureWidth(ch) + letterSpacing
	}
	total := dx - letterSpacing
	if total < 0 {
		total = 0
	}
	return out, total
}

// relayout 窗口尺寸变化：更新各盒子尺寸并把标题/HUD/提示摆到位。
func (sc *pelicanScene) relayout(w, h float64) {
	if sc == nil || w <= 0 || h <= 0 {
		return
	}
	sc.Root.FixedWidth, sc.Root.FixedHeight = w, h
	if sc.stageBox != nil {
		sc.stageBox.FixedWidth, sc.stageBox.FixedHeight = w, h
	}

	f := stageFitFor(w, h)
	placeLine := func(glyphs []titleGlyph, total, baselineY, fs float64) {
		startX := f.ox + f.s*(stageW/2) - f.s*total/2
		y := f.oy + f.s*(baselineY-fs*0.88) // 基线 → 节点顶
		for _, gl := range glyphs {
			x := startX + f.s*gl.dx
			sc.Root.Place(gl.node, x, y)
			if gl.dup != nil {
				sc.Root.Place(gl.dup, x+f.s*1.1, y) // 仿粗体右移 ~3% 字号
			}
		}
	}
	placeLine(sc.titleGlyphs, sc.titleTotal, 54, 30)
	placeLine(sc.subGlyphs, sc.subTotal, 82, 14)

	sc.hudX, sc.hudY = w-18-hudW, h-18-hudH
	sc.Root.Place(sc.hudBox, sc.hudX, sc.hudY)
	sc.Root.Place(sc.speedLabel, sc.hudX+hudPadLR+hudBtnSize+hudGap+hudSliderW+hudGap+8, sc.hudY+15)
	sc.Root.Place(sc.tip, 18, h-32)
	sc.Root.MarkNeedsLayout()
}

func (sc *pelicanScene) onResize(w, h float64) { sc.relayout(w, h) }

func (sc *pelicanScene) onTick(dt float64) {
	if sc == nil {
		return
	}
	sc.sim.tick(dt)
	sc.stageBox.MarkNeedsPaint()
	sc.hudBox.MarkNeedsPaint()
}

// ---- 交互（对应 HTML HUD 按钮 / 滑条 / 键盘）----

func (sc *pelicanScene) togglePause() { sc.sim.paused = !sc.sim.paused }

func (sc *pelicanScene) setSpeed(v float64) {
	v = clampf(math.Round(v*10)/10, 0.3, 2.5) // step=0.1 吸附
	if v == sc.sim.speed {
		return
	}
	sc.sim.speed = v
	sc.speedLabel.SetText(fmt.Sprintf("×%.1f", v))
}

func (sc *pelicanScene) sliderX0() float64 {
	return sc.hudX + hudPadLR + hudBtnSize + hudGap
}

func (sc *pelicanScene) btnCenter() (float64, float64) {
	return sc.hudX + hudPadLR + hudBtnSize/2, sc.hudY + hudH/2
}

func (sc *pelicanScene) onKey(ev platform.Event) {
	if !ev.Pressed {
		return
	}
	switch {
	case ev.KeyCode == ' ' || ev.Rune == ' ': // XK_Space
		sc.togglePause()
	case ev.KeyCode == 65362: // XK_Up
		sc.setSpeed(sc.sim.speed + .1)
	case ev.KeyCode == 65364: // XK_Down
		sc.setSpeed(sc.sim.speed - .1)
	}
}

func (sc *pelicanScene) onPointer(ev platform.Event) {
	bx, by := sc.btnCenter()
	inBtn := math.Hypot(ev.X-bx, ev.Y-by) <= hudBtnSize/2+4
	inSlider := ev.X >= sc.sliderX0()-8 && ev.X <= sc.sliderX0()+hudSliderW+8 &&
		ev.Y >= sc.hudY-6 && ev.Y <= sc.hudY+hudH+6

	switch ev.Pointer {
	case platform.PointerDown:
		if inBtn {
			sc.togglePause()
			return
		}
		if inSlider {
			sc.draggingSlider = true
			sc.setSpeed(0.3 + math.Round(clampf((ev.X-sc.sliderX0())/hudSliderW, 0, 1)*22)*0.1)
		}
	case platform.PointerMove:
		sc.btnHover = inBtn
		if sc.draggingSlider {
			sc.setSpeed(0.3 + math.Round(clampf((ev.X-sc.sliderX0())/hudSliderW, 0, 1)*22)*0.1)
		}
	case platform.PointerUp:
		sc.draggingSlider = false
	}
}

// ---- 舞台绘制 ----

// stageFit 计算舞台到窗口的等比缩放与居中偏移。
type stageFit struct{ s, ox, oy float64 }

func stageFitFor(w, h float64) stageFit {
	s := w / stageW
	if hs := h / stageH; hs < s {
		s = hs
	}
	return stageFit{s: s, ox: (w - stageW*s) / 2, oy: (h - stageH*s) / 2}
}

var (
	// 静态路径：纯舞台坐标，只构建一次（帧内经基矩阵缩放）。
	pathHillTile    = buildHillTile()
	pathFrameRed    = buildPath("M505,514 L620,530 M505,514 L578,404 M620,530 L578,404 M582,420 L742,402 M620,530 L744,406")
	pathForkCurve   = buildPath("M752,432 Q768,470 775,514")
	pathHandlebar   = buildPath("M750,400 L744,368 M744,368 Q726,356 712,362")
	pathChainLines  = buildPath("M620,509.5 L505,505.5 M620,550.5 L505,522.5")
	pathFishTail    = buildPath("M14,0 L26,-8 L26,8 Z")
	pathTailFeather = buildPath("M474,312 L424,296 L452,322 L420,330 L460,350 C470,338 474,326 474,312 Z")
	pathBelly       = buildPath("M470,346 Q545,404 620,340 Q600,382 540,386 Q488,382 470,346 Z")
	pathScarfA      = buildPath("M612,290 L562,318 L592,320 Z")
	pathScarfB      = buildPath("M612,290 L556,306 L590,312 Z")
	pathNeck        = buildPath("M598,300 C636,286 646,252 664,230")
	pathCrest       = buildPath("M672,196 Q660,182 646,186 Q662,188 668,198 Z")
	pathBeakPouch   = buildPath("M708,224 Q736,262 774,270 Q824,264 858,234 Q806,252 768,251 Q734,246 708,224 Z")
	pathBeakUpper   = buildPath("M706,210 Q792,208 860,232 Q796,226 708,224 Z")
	pathMouth       = buildPath("M710,222 Q790,220 856,232")
	pathWing        = buildPath("M562,304 C522,298 494,318 490,346 C514,364 550,360 568,342 C576,328 574,312 562,304 Z")
	pathFeatherLn   = buildPath("M560,312 Q528,318 512,338 M566,320 Q540,328 528,344")
	pathBasketBody  = buildPath("M752,392 L806,392 L796,434 L760,434 Z")
	pathBasketWeave = buildPath("M762,393 L766,433 M774,393 L776,433 M786,393 L786,433 M797,393 L793,433")
	pathTuftBlades  = buildTuftBlades()
)

// buildPath 按紧凑指令串建路径（仅本示例用的极简解析：M/L/Q/C/Z 大写绝对指令）。
func buildPath(d string) *render.Path {
	p := render.NewPath()
	i := 0
	num := func() float64 {
		for i < len(d) && d[i] != '-' && d[i] != '.' && (d[i] < '0' || d[i] > '9') {
			i++
		}
		start := i
		for i < len(d) && ((d[i] >= '0' && d[i] <= '9') || d[i] == '-' || d[i] == '.') {
			i++
		}
		v, _ := strconv.ParseFloat(d[start:i], 64)
		return v
	}
	for i < len(d) {
		c := d[i]
		switch c {
		case 'M':
			i++
			x, y := num(), num()
			p.MoveTo(x, y)
		case 'L':
			i++
			p.LineTo(num(), num())
		case 'Q':
			i++
			cx, cy, x, y := num(), num(), num(), num()
			p.QuadraticTo(cx, cy, x, y)
		case 'C':
			i++
			c1x, c1y, c2x, c2y, x, y := num(), num(), num(), num(), num(), num()
			p.CubicTo(c1x, c1y, c2x, c2y, x, y)
		case 'Z':
			i++
			p.Close()
		default:
			i++
		}
	}
	return p
}

// buildHillTile 远山瓦片：首尾同高 556，平铺无缝。
func buildHillTile() *render.Path {
	return buildPath("M0,556 C70,472 190,470 285,542 C335,578 385,548 440,520 " +
		"C520,480 605,490 662,530 C708,562 762,558 812,536 " +
		"C882,504 952,506 1012,540 C1058,566 1108,556 1150,548 " +
		"C1172,544 1192,550 1200,556 L1200,700 L0,700 Z")
}

// buildTuftBlades 前景草丛瓦片的叶片描边路径（周期 600）。
func buildTuftBlades() *render.Path {
	p := render.NewPath()
	blade := func(mx, my, qx, qy, ex, ey float64) {
		p.MoveTo(mx, my)
		p.QuadraticTo(mx+qx, my+qy, mx+ex, my+ey)
	}
	blade(34, 694, 5, -18, 2, -30)
	blade(52, 696, -5, -16, -2, -27)
	blade(148, 692, 4, -16, 1, -26)
	blade(164, 694, -4, -14, -1, -23)
	blade(300, 695, 6, -20, 3, -32)
	blade(320, 696, -5, -17, -2, -28)
	blade(430, 693, 4, -15, 1, -25)
	blade(446, 695, -4, -14, -1, -22)
	blade(545, 694, 5, -17, 2, -28)
	return p
}

func (sc *pelicanScene) paintStage(pc *rendering.PaintContext, size rendering.Size) {
	sim := sc.sim
	if sim == nil {
		return
	}
	w, h := size.Width, size.Height
	f := stageFitFor(w, h)

	rendering.SetStrokeStyle(pc, 0, render.LineCapRound, render.LineJoinRound)

	// 基矩阵：舞台坐标 → 屏幕（之后所有绘制都用 SVG 原始坐标）
	pc.PushTransform(render.Translate(f.ox, f.oy).Multiply(render.Scale(f.s, f.s)))
	defer pc.PopTransform()

	t := sim.ambientT

	sc.paintSky(pc)
	sc.paintSun(pc, t)
	sc.paintClouds(pc, t)
	sc.paintBirds(pc, t)
	sc.paintParallaxGround(pc, sim)
	sc.paintShadowDust(pc, t)

	p1x, p1y, p2x, p2y := sim.pedal()
	sc.paintFarLimb(pc, sim, p2x, p2y)
	sc.paintBike(pc, sim)
	sc.paintRider(pc, t, sim.crank)
	sc.paintNearLimb(pc, sim, p1x, p1y)
	sc.paintTufts(pc, sim)
}

func (sc *pelicanScene) paintSky(pc *rendering.PaintContext) {
	topR, topG, topB := rgb(0x79c7f5)
	botR, botG, botB := rgb(0xe6f7ff)
	rendering.FillLinearGradient(pc, 0, 0, stageW, stageH,
		0, 0, 0, stageH,
		topR, topG, topB, 1, botR, botG, botB, 1)
}

func (sc *pelicanScene) paintSun(pc *rendering.PaintContext, t float64) {
	hr, hg, hb := rgb(0xffdf6b)
	cr, cg, cb := rgb(0xffd93b)
	rendering.FillCircle(pc, sunX, sunY, 88, hr, hg, hb, .28)
	rendering.FillCircle(pc, sunX, sunY, 46, cr, cg, cb, 1)

	// 光芒整组绕太阳心旋转（SMIL rotate 0→360 / 60s）
	pc.Save()
	pc.RotateAbout(rad(360*frac01(t/60)), sunX, sunY)
	rays := [8][4]float64{
		{0, -60, 0, -84}, {0, 60, 0, 84}, {-60, 0, -84, 0}, {60, 0, 84, 0},
		{-43, -43, -59, -59}, {43, 43, 59, 59}, {-43, 43, -59, 59}, {43, -43, 59, -59},
	}
	for _, ln := range rays {
		rendering.StrokeLine(pc, sunX+ln[0], sunY+ln[1], sunX+ln[2], sunY+ln[3], 7, cr, cg, cb, 1)
	}
	pc.RestoreCanvas()
}

type cloudDef struct {
	dur, delay, alpha float64
	ellipses          [3][4]float64 // cx,cy,rx,ry
}

var cloudDefs = []cloudDef{
	{55, 0, .92, [3][4]float64{{150, 118, 54, 20}, {188, 103, 38, 17}, {114, 106, 30, 14}}},
	{75, -32, .85, [3][4]float64{{80, 185, 44, 16}, {112, 173, 30, 13}, {50, 176, 24, 11}}},
	{95, -61, .80, [3][4]float64{{220, 62, 62, 22}, {264, 46, 42, 18}, {178, 50, 34, 15}}},
}

func (sc *pelicanScene) paintClouds(pc *rendering.PaintContext, t float64) {
	// CSS drift：translateX 1320 → -460 线性循环，负 delay 表示已播 |delay| 秒
	for _, c := range cloudDefs {
		x := lerp(1320, -460, frac01((t-c.delay)/c.dur))
		pc.PushTransform(render.Translate(x, 0))
		for _, e := range c.ellipses {
			rendering.FillOval(pc, e[0]-e[2], e[1]-e[3], e[2]*2, e[3]*2, 1, 1, 1, c.alpha)
		}
		pc.PopTransform()
	}
}

type birdDef struct {
	flyDur, flyDelay, strokeW, flapDur, dyFrom, dyTo, baseY float64
}

var birdDefs = []birdDef{
	{38, 0, 3, .6, -10, -3, 140},
	{52, -21, 2.5, .55, -8, -2, 190},
}

func (sc *pelicanScene) paintBirds(pc *rendering.PaintContext, t float64) {
	sr, sg, sb := rgb(0x3c5a78)
	for _, b := range birdDefs {
		x := lerp(-120, 1340, frac01((t-b.flyDelay)/b.flyDur))
		g := lerp(b.dyFrom, b.dyTo, smilAB(t, b.flapDur, 0))
		p := render.NewPath()
		p.MoveTo(x, b.baseY)
		p.QuadraticTo(x+8, b.baseY+g, x+16, b.baseY)
		p.QuadraticTo(x+24, b.baseY+g, x+32, b.baseY)
		rendering.StrokePath(pc, p, b.strokeW, sr, sg, sb, 1)
	}
}

func (sc *pelicanScene) paintParallaxGround(pc *rendering.PaintContext, sim *pelicanSim) {
	off := func(l parallaxLayer) float64 {
		m := math.Mod(sim.dist*l.f, l.p)
		if m < 0 {
			m += l.p
		}
		return -m
	}

	// 远山 ×2 平铺
	hr, hg, hb := rgb(0xb9e28c)
	oh := off(layerHills)
	for _, tx := range [2]float64{oh, oh + layerHills.p} {
		pc.PushTransform(render.Translate(tx, 0))
		rendering.FillPath(pc, pathHillTile, hr, hg, hb, 1)
		pc.PopTransform()
	}

	// 树木 ×2 平铺
	ot := off(layerTrees)
	for _, tx := range [2]float64{ot, ot + layerTrees.p} {
		pc.PushTransform(render.Translate(tx, 0))
		sc.paintTreeTile(pc)
		pc.PopTransform()
	}

	// 草地 / 公路
	gr, gg, gb := rgb(0x6cc24a)
	rendering.FillRect(pc, 0, 544, stageW, 156, gr, gg, gb, 1)
	rd, rg_, rb := rgb(0x45454e)
	rendering.FillRect(pc, 0, 588, stageW, 68, rd, rg_, rb, 1)
	er, eg, eb := rgb(0x63636e)
	rendering.FillRect(pc, 0, 588, stageW, 5, er, eg, eb, 1)
	br, bg2, bb := rgb(0x2e2e35)
	rendering.FillRect(pc, 0, 649, stageW, 5, br, bg2, bb, 1)

	// 车道虚线（周期 130）
	dr, dg, db := rgb(0xf4f4f4)
	od := off(layerDashes)
	first := math.Floor(-od/130) - 1 // 覆盖左缘的起始序号
	for i := first; ; i++ {
		x := i*130 + od
		if x > stageW {
			break
		}
		rendering.FillRoundRect(pc, x, 618, 64, 9, 4, dr, dg, db, .9)
	}

	vr, vg, vb := rgb(0x58b53e)
	rendering.FillRect(pc, 0, 656, stageW, 44, vr, vg, vb, 1)
}

func (sc *pelicanScene) paintTreeTile(pc *rendering.PaintContext) {
	rr := func(x, y, w2, h2 float64, hex int64) {
		r, g, b := rgb(hex)
		rendering.FillRoundRect(pc, x, y, w2, h2, 3, r, g, b, 1)
	}
	cc := func(cx, cy, rad2 float64, hex int64) {
		r, g, b := rgb(hex)
		rendering.FillCircle(pc, cx, cy, rad2, r, g, b, 1)
	}
	el := func(cx, cy, rx, ry float64, hex int64) {
		r, g, b := rgb(hex)
		rendering.FillOval(pc, cx-rx, cy-ry, rx*2, ry*2, r, g, b, 1)
	}
	rr(84, 484, 12, 62, 0x7a5230)
	cc(90, 468, 30, 0x2f8f3a)
	cc(72, 481, 19, 0x37a144)
	cc(108, 480, 19, 0x37a144)
	el(252, 534, 36, 17, 0x2f8f3a)
	el(284, 538, 26, 13, 0x37a144)
	rr(392, 468, 13, 78, 0x6e4a2c)
	cc(398, 450, 34, 0x2c8437)
	cc(374, 466, 21, 0x339a40)
	cc(420, 464, 22, 0x339a40)
	el(565, 536, 30, 15, 0x35993f)
	el(700, 540, 24, 12, 0x2f8f3a)
}

func (sc *pelicanScene) paintShadowDust(pc *rendering.PaintContext, t float64) {
	// 车影
	rendering.FillOval(pc, 645-205, 592-13, 410, 26, 0, 0, 0, .14)

	// 后轮扬尘：translate(0,0)→(-92,-40)，r 5→16，opacity .5→0，dur .9s 三相
	fr, fg, fb := rgb(0xdde8cf)
	for _, beg := range [3]float64{0, .3, .6} {
		u := frac01((t - beg) / .9)
		rendering.FillCircle(pc, 452-92*u, 586-40*u, 5+11*u, fr, fg, fb, .5*(1-u))
	}
}

// drawLeg IK 腿：hip→knee→ankle 折线 + 脚椭圆（脚踩踏板上方 9px，再偏 (+13,+4)）。
func drawLeg(pc *rendering.PaintContext, hipX, hipY, ankX, ankY float64, strokeHex int64, lw float64, footRX, footRY float64, footHex, footStrokeHex int64) {
	kx, ky := knee(hipX, hipY, ankX, ankY, legL1, legL2)
	p := render.NewPath()
	p.MoveTo(hipX, hipY)
	p.LineTo(kx, ky)
	p.LineTo(ankX, ankY)
	r, g, b := rgb(strokeHex)
	rendering.StrokePath(pc, p, lw, r, g, b, 1)
	fr, fg, fb := rgb(footHex)
	sr, sg, sb := rgb(footStrokeHex)
	rendering.FillOval(pc, ankX+13-footRX, ankY+4-footRY, footRX*2, footRY*2, fr, fg, fb, 1)
	rendering.StrokeOval(pc, ankX+13-footRX, ankY+4-footRY, footRX*2, footRY*2, 1.5, sr, sg, sb, 1)
}

func (sc *pelicanScene) paintFarLimb(pc *rendering.PaintContext, sim *pelicanSim, px, py float64) {
	drawLeg(pc, hipFX, hipFY, px, py-9, 0xdd8f3a, 13, 15, 6.5, 0xd9822b, 0xc26e1d)
	ar, ag, ab := rgb(0x3a3a3a)
	rendering.StrokeLine(pc, bbX, bbY, px, py, 8, ar, ag, ab, 1)
	pr, pg, pb := rgb(0x262626)
	rendering.FillRoundRect(pc, px-13, py-4.5, 26, 9, 3, pr, pg, pb, 1)
}

func (sc *pelicanScene) paintNearLimb(pc *rendering.PaintContext, sim *pelicanSim, px, py float64) {
	ar, ag, ab := rgb(0x4a4a4a)
	rendering.StrokeLine(pc, bbX, bbY, px, py, 9, ar, ag, ab, 1)
	pr, pg, pb := rgb(0x141414)
	rendering.FillRoundRect(pc, px-15, py-5, 30, 10, 3, pr, pg, pb, 1)
	drawLeg(pc, hipNX, hipNY, px, py-9, 0xf4a94f, 15, 17, 7, 0xf08c2e, 0xd97f26)
}

func (sc *pelicanScene) paintBike(pc *rendering.PaintContext, sim *pelicanSim) {
	tireR, tireG, tireB := rgb(0x23252b)
	rimR, rimG, rimB := rgb(0xcfd4da)
	spokeR, spokeG, spokeB := rgb(0xb9bfc7)

	// 两轮：外胎 / 钢圈 / 辐条（随车速旋转）/ 轴皮
	for _, wheel := range [2][2]float64{{axleRX, axleRY}, {axleFX, axleFY}} {
		ax, ay := wheel[0], wheel[1]
		rendering.StrokeCircle(pc, ax, ay, 72, 11, tireR, tireG, tireB, 1)
		rendering.StrokeCircle(pc, ax, ay, 56, 4, rimR, rimG, rimB, 1)
		pc.Save()
		pc.RotateAbout(rad(sim.wheelDeg), ax, ay)
		spokes := render.NewPath()
		spokes.MoveTo(ax-56, ay)
		spokes.LineTo(ax+56, ay)
		spokes.MoveTo(ax, ay-56)
		spokes.LineTo(ax, ay+56)
		spokes.MoveTo(ax-40, ay-40)
		spokes.LineTo(ax+40, ay+40)
		spokes.MoveTo(ax-40, ay+40)
		spokes.LineTo(ax+40, ay-40)
		rendering.StrokePath(pc, spokes, 3, spokeR, spokeG, spokeB, 1)
		pc.RestoreCanvas()
		hubR, hubG, hubB := rgb(0x333333)
		rendering.FillCircle(pc, ax, ay, 7, hubR, hubG, hubB, 1)
	}

	// 车架（红）
	fr, fg, fb := rgb(0xe63946)
	rendering.StrokePath(pc, pathFrameRed, 10, fr, fg, fb, 1)
	fdr, fdg, fdb := rgb(0xc92f3c)
	rendering.StrokeLine(pc, 742, 396, 752, 432, 12, fdr, fdg, fdb, 1)
	rendering.StrokePath(pc, pathForkCurve, 9, fr, fg, fb, 1)

	// 把立 / 车把 / 车铃
	hbr, hbg, hbb := rgb(0x2f2f2f)
	rendering.StrokePath(pc, pathHandlebar, 7, hbr, hbg, hbb, 1)
	rendering.FillCircle(pc, 712, 362, 5, 0.067, 0.067, 0.067, 1) // #111111
	bellR, bellG, bellB := rgb(0xf6c445)
	bellSR, bellSG, bellSB := rgb(0xc99a1e)
	rendering.FillCircle(pc, 731, 360, 5.5, bellR, bellG, bellB, 1)
	rendering.StrokeCircle(pc, 731, 360, 5.5, 1.5, bellSR, bellSG, bellSB, 1)

	// 车座
	sadR, sadG, sadB := rgb(0x333333)
	rendering.StrokeLine(pc, 578, 404, 574, 392, 6, sadR, sadG, sadB, 1)
	sad2R, sad2G, sad2B := rgb(0x23252b)
	rendering.FillOval(pc, 572-34, 388-8, 68, 16, sad2R, sad2G, sad2B, 1)

	// 链条 / 牙盘
	chnR, chnG, chnB := rgb(0x3a3a3a)
	rendering.StrokePath(pc, pathChainLines, 3, chnR, chnG, chnB, 1)
	rendering.FillCircle(pc, axleRX, axleRY, 9, chnR, chnG, chnB, 1)
	rendering.StrokeCircle(pc, bbX, bbY, 21, 6, chnR, chnG, chnB, 1)

	// 车筐里的鱼（先画，被筐沿遮住下半）
	pc.Save()
	pc.Translate(790, 382)
	pc.RotateAbout(rad(-24), 0, 0)
	fishR, fishG, fishB := rgb(0x6fc9e6)
	fishSR, fishSG, fishSB := rgb(0x3f9fc0)
	rendering.FillOval(pc, -16, -7, 32, 14, fishR, fishG, fishB, 1)
	rendering.StrokeOval(pc, -16, -7, 32, 14, 1.5, fishSR, fishSG, fishSB, 1)
	rendering.FillPath(pc, pathFishTail, fishR, fishG, fishB, 1)
	rendering.StrokePath(pc, pathFishTail, 1.5, fishSR, fishSG, fishSB, 1)
	rendering.FillCircle(pc, -8, -2, 1.8, 1/255., 2/255., 3/255., 1) // #123
	pc.RestoreCanvas()

	// 车筐
	bodyR, bodyG, bodyB := rgb(0xbd8f52)
	stR, stG, stB := rgb(0x8a6430)
	rendering.FillPath(pc, pathBasketBody, bodyR, bodyG, bodyB, 1)
	rendering.StrokePath(pc, pathBasketBody, 2, stR, stG, stB, 1)
	rendering.StrokePath(pc, pathBasketWeave, 1.6, stR, stG, stB, .7)
	rimR2, rimG2, rimB2 := rgb(0xa87c42)
	rendering.FillRoundRect(pc, 749, 386, 60, 8, 3.5, rimR2, rimG2, rimB2, 1)
}

func (sc *pelicanScene) paintRider(pc *rendering.PaintContext, t, crank float64) {
	// 身体随踩踏轻微起伏（JS rider translateY sin(crank*2)*2.6）
	pc.PushTransform(render.Translate(0, math.Sin(crank*2)*2.6))
	defer pc.PopTransform()

	// 尾羽
	tfr, tfg, tfb := rgb(0xf1f1e6)
	tfsR, tfsG, tfsB := rgb(0xdedcd0)
	rendering.FillPath(pc, pathTailFeather, tfr, tfg, tfb, 1)
	rendering.StrokePath(pc, pathTailFeather, 1.5, tfsR, tfsG, tfsB, 1)

	// 身体（椭圆整体旋转 -13°）
	pc.Save()
	pc.RotateAbout(rad(-13), 545, 332)
	bdR, bdG, bdB := rgb(0xfbfaf1)
	bdsR, bdsG, bdsB := rgb(0xe2e0d2)
	rendering.FillOval(pc, 545-84, 332-58, 168, 116, bdR, bdG, bdB, 1)
	rendering.StrokeOval(pc, 545-84, 332-58, 168, 116, 2, bdsR, bdsG, bdsB, 1)
	pc.RestoreCanvas()

	// 腹部阴影
	blr, blg, blb := rgb(0xefeee0)
	rendering.FillPath(pc, pathBelly, blr, blg, blb, .9)

	// 围巾（两片三角各自绕颈点摆动）+ 结
	scarf := func(p *render.Path, fromDeg, toDeg, dur float64, hex int64) {
		pc.Save()
		pc.RotateAbout(rad(lerp(fromDeg, toDeg, smilAB(t, dur, 0))), 612, 290)
		r, g, b := rgb(hex)
		rendering.FillPath(pc, p, r, g, b, 1)
		pc.RestoreCanvas()
	}
	scarf(pathScarfA, 3, -8, .7, 0xc73e43)
	scarf(pathScarfB, -4, 10, .55, 0xe5484d)
	knR, knG, knB := rgb(0xd94343)
	rendering.FillCircle(pc, 612, 290, 7, knR, knG, knB, 1)

	// 脖子（深色描边打底 + 白色双层）
	nksR, nksG, nksB := rgb(0xe2e0d2)
	nkR, nkG, nkB := rgb(0xfbfaf1)
	rendering.StrokePath(pc, pathNeck, 30, nksR, nksG, nksB, 1)
	rendering.StrokePath(pc, pathNeck, 24, nkR, nkG, nkB, 1)

	// 头 + 头冠
	rendering.FillCircle(pc, 688, 216, 27, nkR, nkG, nkB, 1)
	rendering.StrokeCircle(pc, 688, 216, 27, 2, nksR, nksG, nksB, 1)
	rendering.FillPath(pc, pathCrest, tfr, tfg, tfb, 1)

	// 大嘴 + 喉囊
	bpR, bpG, bpB := rgb(0xf2a65a)
	bpsR, bpsG, bpsB := rgb(0xd9822b)
	rendering.FillPath(pc, pathBeakPouch, bpR, bpG, bpB, 1)
	rendering.StrokePath(pc, pathBeakPouch, 1.5, bpsR, bpsG, bpsB, 1)
	buR, buG, buB := rgb(0xf4a03c)
	busR, busG, busB := rgb(0xd97f26)
	rendering.FillPath(pc, pathBeakUpper, buR, buG, buB, 1)
	rendering.StrokePath(pc, pathBeakUpper, 1.5, busR, busG, busB, 1)
	mR, mG, mB := rgb(0xc96f1f)
	rendering.StrokePath(pc, pathMouth, 1.5, mR, mG, mB, .6)

	// 眼睛
	rendering.FillCircle(pc, 700, 207, 5, 0.149, 0.133, 0.122, 1) // #26221f
	rendering.FillCircle(pc, 702, 204.5, 1.8, 1, 1, 1, 1)

	// 翅膀（绕肩点扇动）+ 羽毛线
	pc.Save()
	pc.RotateAbout(rad(lerp(-2, 6, smilAB(t, 1.1, 0))), 562, 304)
	wgR, wgG, wgB := rgb(0xf3f2e6)
	rendering.FillPath(pc, pathWing, wgR, wgG, wgB, 1)
	rendering.StrokePath(pc, pathWing, 2, tfsR, tfsG, tfsB, 1)
	pc.RestoreCanvas()
	rendering.StrokePath(pc, pathFeatherLn, 2, tfsR, tfsG, tfsB, 1)
}

func (sc *pelicanScene) paintTufts(pc *rendering.PaintContext, sim *pelicanSim) {
	m := math.Mod(sim.dist*layerTufts.f, layerTufts.p)
	if m < 0 {
		m += layerTufts.p
	}
	ot := -m
	for _, tx := range [2]float64{ot, ot + layerTufts.p} {
		pc.PushTransform(render.Translate(tx, 0))
		tbr, tbg, tbb := rgb(0x2f7d2a)
		rendering.StrokePath(pc, pathTuftBlades, 4, tbr, tbg, tbb, 1)
		f1r, f1g, f1b := rgb(0xff7ab8)
		f2r, f2g, f2b := rgb(0xffd166)
		rendering.FillCircle(pc, 112, 682, 4, f1r, f1g, f1b, 1)
		rendering.FillCircle(pc, 380, 680, 4, f2r, f2g, f2b, 1)
		rendering.FillCircle(pc, 500, 684, 3.5, f1r, f1g, f1b, 1)
		pc.PopTransform()
	}
}

// ---- HUD 绘制（窗口坐标，对应 HTML #hud）----

// renderGolden 离屏软件光栅渲染当前仿真状态的单帧（GOLDEN_PNG 诊断模式）。
func (sc *pelicanScene) renderGolden(path string) {
	dc := render.NewContext(int(stageW), int(stageH))
	dc.SetRGBA(0.47, 0.78, 0.96, 1)
	dc.Clear()
	pc := rendering.NewPaintContext(dc, 1)
	sc.paintStage(pc, rendering.Size{Width: stageW, Height: stageH})
	if err := dc.SavePNG(path); err != nil {
		fmt.Fprintf(os.Stderr, "ui_render_pelican: golden save: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_render_pelican: golden -> %s (PELICAN_T=%s)\n", path, os.Getenv("PELICAN_T"))
}

func (sc *pelicanScene) paintHUD(pc *rendering.PaintContext, size rendering.Size) {
	// 药丸底
	rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, size.Height/2, 1, 1, 1, .85)

	// 暂停 / 播放按钮（hover 变红，对应 button:hover）
	bcx, bcy := hudPadLR+hudBtnSize/2, size.Height/2
	btnCol := int64(0x222233)
	if sc.btnHover {
		btnCol = 0xe63946
	}
	br, bg, bb := rgb(btnCol)
	rendering.FillCircle(pc, bcx, bcy, hudBtnSize/2, br, bg, bb, 1)
	if sc.sim.paused {
		// ▶
		p := render.NewPath()
		p.MoveTo(bcx-4, bcy-6)
		p.LineTo(bcx+6, bcy)
		p.LineTo(bcx-4, bcy+6)
		p.Close()
		rendering.FillPath(pc, p, 1, 1, 1, 1)
	} else {
		// ⏸ 两根白条
		rendering.FillRoundRect(pc, bcx-5.5, bcy-5, 3.6, 10, 1.2, 1, 1, 1, 1)
		rendering.FillRoundRect(pc, bcx+1.9, bcy-5, 3.6, 10, 1.2, 1, 1, 1, 1)
	}

	// 滑条：灰槽 + 已选段红色 + 红色滑块（accent-color:#e63946 的近似）
	tx0 := hudPadLR + hudBtnSize + hudGap
	ty := size.Height / 2
	rendering.FillRoundRect(pc, tx0, ty-2, hudSliderW, 4, 2, 0.85, 0.86, 0.89, 1)
	u := (sc.sim.speed - 0.3) / 2.2
	rendering.FillRoundRect(pc, tx0, ty-2, math.Max(hudSliderW*u, 2), 4, 2, br, bg, bb, 1)
	rendering.FillCircle(pc, tx0+hudSliderW*u, ty, 7, br, bg, bb, 1)
}
