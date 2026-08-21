package main

// 按 group-jump-rope.html 参考实现逐行移植：
// 画布 960×600 等比放大 1.25 映射到窗口 1200×800（垂直居中，stageOy=25），
// 所有几何常量保持参考实现的原始数值，运动数学与参考实现一致。
// 渲染走 PaintContext 公开 API（GPU 合批），绳为单条连续 Path + 中点二次贝塞尔平滑。

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// ---- 坐标映射：HTML 舞台坐标（960×600）→ 窗口坐标 ----
// 每帧按当前窗口尺寸等比缩放（取 min(w/960,h/600)）并居中，随窗口大小自适应。
// curFit 由 paint() 开头计算；sx/sy/sw 只在绘制线程内使用。
type stageFit struct{ s, ox, oy float64 }

var curFit stageFit

func sx(x float64) float64 { return curFit.ox + x*curFit.s }
func sy(y float64) float64 { return curFit.oy + y*curFit.s }
func sw(v float64) float64 { return v * curFit.s }

// stageFitFor 计算窗口 (w,h) 下 960×600 舞台的等比缩放与居中偏移。
func stageFitFor(w, h float64) stageFit {
	s := w / 960
	if hs := h / 600; hs < s {
		s = hs
	}
	return stageFit{s: s, ox: (w - 960*s) / 2, oy: (h - 600*s) / 2}
}

// ---- 参考实现几何常量（HTML 坐标系，未缩放）----
const (
	groundY    = 520.0              // 地面线
	ankleStand = 496.0              // 站立脚踝高
	ropeRMax   = 184.0              // 绳最大摆幅（手高 332 + 184 = 516 触地）
	crankR     = 15.0               // 摇绳手曲柄半径
	omega      = 2 * math.Pi / 0.95 // 0.95s 一圈
	airHalf    = 0.95               // 滞空半角（弧度）
	jumpApex   = 46.0               // 跳跃最高点
	tuckLift   = 24.0               // 滞空收腿上提
	jumperCX   = 480.0              // 中间人站位
	turnBaseLX = 216.0              // 左摇绳人持绳手基准位
	turnBaseLY = 332.0
	turnBaseRX = 744.0 // 右摇绳人持绳手基准位
)

// freezePhase 读 FREEZE_PHASE（0..1，锁定循环相位供单帧目检；未设置返回 -1 不冻结）。
func freezePhase() float64 {
	if v := os.Getenv("FREEZE_PHASE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f < 1 {
			return f
		}
	}
	return -1
}

// 配色（参考实现取色，hex → 线性 RGB 浮点）。
var (
	colWallTop    = [4]float64{0.9647, 0.9373, 0.8902, 1}
	colWallBottom = [4]float64{0.9176, 0.8745, 0.7843, 1}
	colFloor      = [4]float64{0.7882, 0.7059, 0.5569, 1}
	colFloorEdge  = [4]float64{0.6588, 0.5608, 0.3882, 1}
	colInk        = [4]float64{0.2275, 0.2549, 0.3137, 1} // 计数字 / 摇绳人鞋
	colBodyL      = [4]float64{0.1804, 0.6196, 0.5608, 1} // 左摇绳人 #2e9e8f
	colBodyLOut   = [4]float64{0.1255, 0.4471, 0.4000, 1}
	colBodyR      = [4]float64{0.8784, 0.4824, 0.1804, 1} // 右摇绳人 #e07b2e
	colBodyROut   = [4]float64{0.6588, 0.3529, 0.1255, 1}
	colSkin       = [4]float64{0.9490, 0.7882, 0.6039, 1} // #f2c99a
	colSkinOut    = [4]float64{0.8275, 0.6275, 0.4235, 1}
	colHairL      = [4]float64{0.2902, 0.2000, 0.1255, 1} // #4a3320
	colHairR      = [4]float64{0.4196, 0.2275, 0.1216, 1} // #6b3a1f
	colEye        = [4]float64{0.1686, 0.1294, 0.0941, 1} // #2b2118
	colLegs       = [4]float64{0.2157, 0.2784, 0.3098, 1} // #37474f
	colShin       = [4]float64{0.1843, 0.2392, 0.2667, 1} // #2f3d44
	colShoe       = [4]float64{0.8510, 0.3098, 0.3098, 1} // #d94f4f
	colShoeOut    = [4]float64{0.6588, 0.2275, 0.2275, 1}
	colTorsoJ     = [4]float64{0.2902, 0.5647, 0.8510, 1} // 中间人 #4a90d9
	colTorsoJOut  = [4]float64{0.2078, 0.4118, 0.6235, 1}
	colMouth      = [4]float64{0.6902, 0.4157, 0.2902, 1} // #b06a4a
	colRope       = [4]float64{0.4784, 0.2902, 0.1294, 1} // #7a4a21
	colGrip       = [4]float64{0.2000, 0.2000, 0.2000, 1} // 把手 #333
)

// jumpropeScene 组装舞台：根 AbsoluteBox + 全屏自定义绘制节点 + 标签 + 计数 + LiveHUD。
type jumpropeScene struct {
	Root    *rendering.AbsoluteBox
	stage   *jumpropeStage
	hud     *wrkit.LiveHUD
	sim     *ropeSim
	counter *rendering.RenderText
}

func newJumpropeScene(winW, winH float64) *jumpropeScene {
	root := rendering.NewAbsoluteBox(winW, winH)

	s := &jumpropeScene{Root: root}

	// 全屏舞台
	s.sim = newRopeSim(freezePhase())
	stage := &jumpropeStage{sim: s.sim}
	box := rendering.NewRenderBox()
	box.FixedWidth = winW
	box.FixedHeight = winH
	box.OnPaint = stage.paint
	stage.box = box
	s.stage = stage
	root.Place(box, 0, 0)

	face, _, faceErr := wrkit.EnsureUIFace()

	// 说明标签
	labels := []struct {
		x, y float64
		text string
	}{
		{20, 28, "双人摇长绳 · 中央连续跳绳 — 按 group-jump-rope.html 参考实现移植，单一仿真时钟驱动"},
		{20, 52, "绳：钟摆模型 sin(πt)^0.72·cosφ 挂在双手轴上，中点二次贝塞尔平滑；前后分层不穿身"},
		{20, 76, "FREEZE_PHASE=0..1 冻结相位单帧目检；RUN_SECONDS 可调；SNAPSHOT=xxx.png 读回 PNG"},
	}
	for _, l := range labels {
		t := rendering.NewRenderText(l.text)
		t.FontSize = 13
		t.R, t.G, t.B, t.A = 0.23, 0.25, 0.31, 0.95
		if faceErr == nil {
			t.SetFace(face)
		}
		root.Place(t, l.x, l.y)
	}

	// 跳绳计数（右上，随圈数更新；字号与位置随窗口自适应）
	s.counter = rendering.NewRenderText("跳绳计数：0")
	f0 := stageFitFor(winW, winH)
	s.counter.FontSize = 22 * f0.s
	s.counter.R, s.counter.G, s.counter.B, s.counter.A = colInk[0], colInk[1], colInk[2], 1
	if faceErr == nil {
		s.counter.SetFace(face)
	}
	root.Place(s.counter, f0.ox+700*f0.s, f0.oy+30*f0.s)

	// LiveHUD 底部指标带
	s.hud = wrkit.NewLiveHUD(winW, 64)
	root.Place(s.hud.Box, 0, winH-64)

	return s
}

func (s *jumpropeScene) onTick(dt float64) {
	if s == nil || s.stage == nil {
		return
	}
	s.sim.tick(dt)
	s.counter.SetText(fmt.Sprintf("跳绳计数：%d", s.sim.count))
	s.stage.box.MarkNeedsPaint()
}

// onResize 窗口尺寸变化：更新根/舞台/HUD 尺寸，按新窗口重排计数与 HUD 位置。
// 绘制内容的大小由 paint() 开头的 stageFitFor 按当前尺寸自适应。
func (s *jumpropeScene) onResize(w, h float64) {
	if s == nil || w <= 0 || h <= 0 {
		return
	}
	s.Root.FixedWidth, s.Root.FixedHeight = w, h
	if s.stage != nil && s.stage.box != nil {
		s.stage.box.FixedWidth, s.stage.box.FixedHeight = w, h
	}
	if s.hud != nil && s.hud.Box != nil {
		s.hud.Box.FixedWidth = w
		s.Root.Place(s.hud.Box, 0, h-64)
	}
	f := stageFitFor(w, h)
	s.Root.Place(s.counter, f.ox+700*f.s, f.oy+30*f.s)
	s.counter.FontSize = 22 * f.s
	s.Root.MarkNeedsLayout()
}

// ropeSim 是动画仿真状态：phi 为绳相位角，其余姿态全部是它的纯函数。
type ropeSim struct {
	simT       float64
	phi        float64
	count      int
	prevPhiMod float64
	freeze     float64 // ≥0 冻结相位（FREEZE_PHASE 调试开关）
}

func newRopeSim(freeze float64) *ropeSim {
	s := &ropeSim{phi: 0.02, prevPhiMod: 0.02, freeze: freeze}
	if freeze >= 0 {
		s.phi = freeze * 2 * math.Pi
		s.prevPhiMod = s.phi
	}
	return s
}

func (s *ropeSim) tick(dt float64) {
	if s == nil || s.freeze >= 0 {
		return
	}
	s.simT += dt
	s.phi += omega * dt
	pm := s.phaseMod()
	if pm < s.prevPhiMod {
		s.count++
	}
	s.prevPhiMod = pm
}

func (s *ropeSim) phaseMod() float64 {
	m := math.Mod(s.phi, 2*math.Pi)
	if m < 0 {
		m += 2 * math.Pi
	}
	return m
}

// jump 返回中间人跳跃状态（与参考实现同式）：
// j 离地高度、crouch 地面屈膝量（起跳前/落地后高斯包络）、tuck 滞空收腿包络。
func (s *ropeSim) jump() (j, crouch, tuck float64, airborne bool) {
	pm := s.phaseMod()
	if pm < airHalf || pm > 2*math.Pi-airHalf {
		airborne = true
		var fromTO float64
		if pm >= math.Pi {
			fromTO = pm - (2*math.Pi - airHalf)
		} else {
			fromTO = pm + airHalf
		}
		u := fromTO / (2 * airHalf)
		j = 4 * jumpApex * u * (1 - u)
		tuck = math.Sin(math.Pi * u)
	} else {
		a := pm - airHalf
		b := (2*math.Pi - airHalf) - pm
		crouch = 0.5*math.Exp(-(a/0.55)*(a/0.55)) + 0.85*math.Exp(-(b/0.55)*(b/0.55))
	}
	return
}

// handL/handR：摇绳手持绳手。曲柄圆周运动，左右水平反相、垂直同相（参考实现式）。
func (s *ropeSim) handL() (float64, float64) {
	return turnBaseLX + crankR*math.Cos(s.phi), turnBaseLY + crankR*math.Sin(s.phi)
}

func (s *ropeSim) handR() (float64, float64) {
	return turnBaseRX - crankR*math.Cos(s.phi), turnBaseLY + crankR*math.Sin(s.phi)
}

// ropeFront 绳是否在人前层：sinφ≥0 时绳从上往前下方扫（参考实现判定）。
// 翻转发生在 φ=0/π 的摆幅极值点（绳贴地/过顶），换层无视觉跳变。
func (s *ropeSim) ropeFront() bool { return math.Sin(s.phi) >= 0 }

type ropePt struct{ x, y float64 }

// ropePts 绳采样点（HTML 坐标）：挂在双手连线轴上，
// 摆幅 R = ropeRMax·sin(πt)^0.72（0.72 次幂让绳形更饱满），
// y 偏移 = R·cosφ（φ=0 垂到地面，φ=π 甩过头顶），x 叠加微幅横摆。
func (s *ropeSim) ropePts() []ropePt {
	lhx, lhy := s.handL()
	rhx, rhy := s.handR()
	const n = 56
	pts := make([]ropePt, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		bx := lhx + (rhx-lhx)*t
		by := lhy + (rhy-lhy)*t
		r := ropeRMax * math.Pow(math.Sin(math.Pi*t), 0.72)
		pts[i] = ropePt{
			x: bx + 5*math.Sin(math.Pi*t)*math.Sin(s.simT*1.1),
			y: by + r*math.Cos(s.phi),
		}
	}
	return pts
}

// jumpropeStage 是舞台节点：按「背景 → 后层绳 → 人物 → 前层绳」顺序绘制。
type jumpropeStage struct {
	box *rendering.RenderBox
	sim *ropeSim
}

func (st *jumpropeStage) paint(pc *rendering.PaintContext, size rendering.Size) {
	sim := st.sim
	if sim == nil {
		return
	}
	w, h := size.Width, size.Height

	// 自适应：按当前窗口尺寸计算舞台缩放（等比、居中）
	curFit = stageFitFor(w, h)

	// 全帧圆头/圆角连接：肢体、绳的端头和拐点都平滑
	rendering.SetStrokeStyle(pc, 0, render.LineCapRound, render.LineJoinRound)

	// 背景：墙面渐变（整窗）+ 地板 + 踢脚线（地板线跟随舞台缩放位置）
	rendering.FillLinearGradient(pc, 0, 0, w, h,
		0, 0, 0, h,
		colWallTop[0], colWallTop[1], colWallTop[2], 1,
		colWallBottom[0], colWallBottom[1], colWallBottom[2], 1)
	rendering.FillRect(pc, 0, sy(groundY), w, h-sy(groundY),
		colFloor[0], colFloor[1], colFloor[2], 1)
	rendering.FillRect(pc, 0, sy(groundY)-sw(1.5), w, sw(3),
		colFloorEdge[0], colFloorEdge[1], colFloorEdge[2], 1)

	// 影子（贴地，先画）
	paintEllipse(pc, 196, 522, 42, 7, [4]float64{0, 0, 0, 0.12})
	paintEllipse(pc, 764, 522, 42, 7, [4]float64{0, 0, 0, 0.12})
	j, _, _, _ := sim.jump()
	jrx := 52 - j*0.55
	if jrx < 30 {
		jrx = 30
	}
	paintEllipse(pc, jumperCX, 522, jrx, 8, [4]float64{0, 0, 0, 0.15 - j*0.0018})

	// 后层绳（含把手）
	pts := sim.ropePts()
	if !sim.ropeFront() {
		strokeRopeLayer(pc, pts)
	}

	// 人物
	paintTurner(pc, sim, -1)
	paintTurner(pc, sim, +1)
	paintJumper(pc, sim)

	// 前层绳（含把手）
	if sim.ropeFront() {
		strokeRopeLayer(pc, pts)
	}
}

// strokeRopeLayer 描当层的绳 + 两端把手。绳曲线用中点二次贝塞尔平滑（参考实现式）。
func strokeRopeLayer(pc *rendering.PaintContext, pts []ropePt) {
	n := len(pts) - 1
	p := render.NewPath()
	p.MoveTo(sx(pts[0].x), sy(pts[0].y))
	for q := 1; q < n; q++ {
		mx := (pts[q].x + pts[q+1].x) / 2
		my := (pts[q].y + pts[q+1].y) / 2
		p.QuadraticTo(sx(pts[q].x), sy(pts[q].y), sx(mx), sy(my))
	}
	p.LineTo(sx(pts[n].x), sy(pts[n].y))
	defer p.Clear()
	rendering.StrokePath(pc, p, sw(4), colRope[0], colRope[1], colRope[2], 1)

	// 把手：沿绳端切向的短深色柄
	for _, end := range [2]int{0, n} {
		var dx, dy float64
		if end == 0 {
			dx, dy = pts[1].x-pts[0].x, pts[1].y-pts[0].y
		} else {
			dx, dy = pts[end-1].x-pts[end].x, pts[end-1].y-pts[end].y
		}
		dl := math.Hypot(dx, dy)
		if dl < 1e-6 {
			dl = 1
		}
		rendering.StrokeLine(pc, sx(pts[end].x), sy(pts[end].y),
			sx(pts[end].x+dx/dl*14), sy(pts[end].y+dy/dl*14), sw(5.5),
			colGrip[0], colGrip[1], colGrip[2], 1)
	}
}

// paintTurner 画摇绳人（side=-1 左 / +1 右）：圆角矩形躯干 + 圆头描边卡通风，
// 持绳臂肩→肘(中点下垂12)→手，外侧臂前后摆（与持绳臂反相）。
func paintTurner(pc *rendering.PaintContext, sim *ropeSim, side float64) {
	var body, bodyOut, hair [4]float64
	var headCX, rectX, outerSX, shX float64
	if side < 0 {
		body, bodyOut, hair = colBodyL, colBodyLOut, colHairL
		headCX, rectX, outerSX, shX = 170, 152, 154, 186
	} else {
		body, bodyOut, hair = colBodyR, colBodyROut, colHairR
		headCX, rectX, outerSX, shX = 790, 772, 806, 774
	}

	// 躯干圆角矩形
	torso := render.NewPath()
	torso.RoundedRectangle(sx(rectX), sy(342), sw(36), sw(96), sw(10))
	fillPath(pc, torso, body)
	strokePathOutline(pc, torso, 2, bodyOut)

	// 头 + 头发 + 眼睛
	head := render.NewPath()
	head.Circle(sx(headCX), sy(316), sw(18))
	fillPath(pc, head, colSkin)
	strokePathOutline(pc, head, 2, colSkinOut)

	hairP := render.NewPath()
	hairP.MoveTo(sx(headCX-18), sy(310))
	hairP.Arc(sx(headCX), sy(310), sw(18), math.Pi, 2*math.Pi)
	hairP.LineTo(sx(headCX+13), sy(305))
	hairP.QuadraticTo(sx(headCX), sy(294), sx(headCX-13), sy(305))
	hairP.Close()
	fillPath(pc, hairP, hair)

	for _, ex := range [2]float64{-6, 6} {
		rendering.FillCircle(pc, sx(headCX+ex), sy(316), sw(2.2),
			colEye[0], colEye[1], colEye[2], 1)
	}

	// 腿（圆头粗线）+ 鞋
	rendering.StrokeLine(pc, sx(headCX-8), sy(438), sx(headCX-12), sy(500), sw(12),
		colLegs[0], colLegs[1], colLegs[2], 1)
	rendering.StrokeLine(pc, sx(headCX+8), sy(438), sx(headCX+12), sy(500), sw(12),
		colLegs[0], colLegs[1], colLegs[2], 1)
	paintEllipse(pc, headCX-14, 508, 11, 6, colInk)
	paintEllipse(pc, headCX+14, 508, 11, 6, colInk)

	// 持绳臂：肩 → 肘（中点下垂 12）→ 手
	var hx, hy float64
	if side < 0 {
		hx, hy = sim.handL()
	} else {
		hx, hy = sim.handR()
	}
	ex, ey := (shX+hx)/2, (354+hy)/2+12
	arm := render.NewPath()
	arm.MoveTo(sx(shX), sy(354))
	arm.LineTo(sx(ex), sy(ey))
	arm.LineTo(sx(hx), sy(hy))
	strokePath(pc, arm, 10, body)

	// 外侧臂：Q 曲线，末端随相位前后摆 ±9
	outer := render.NewPath()
	outer.MoveTo(sx(outerSX), sy(354))
	outer.QuadraticTo(sx(outerSX+side*16), sy(376), sx(outerSX+side*14), sy(398+9*math.Sin(sim.phi)))
	strokePath(pc, outer, 10, body)
}

// paintJumper 画中间跳绳人：骨盆/肩/头随跳跃堆叠，地面屈膝蓄力缓冲，
// 滞空收腿张臂；腿大腿/小腿两段 + 鞋；填充躯干轮廓形 + 圆头四肢。
func paintJumper(pc *rendering.PaintContext, sim *ropeSim) {
	j, crouch, tuck, _ := sim.jump()

	CX := jumperCX
	pelvisY := 380 + crouch*14 - j
	shoulderY := pelvisY - 78 + crouch*5
	headCY := shoulderY - 34

	// 腿：髋 → 膝（外偏随屈膝加深）→ 踝，鞋随踝上提
	ankleY := ankleStand - j - tuck*tuckLift
	for _, side := range [2]float64{-1, 1} {
		hipX, hipY := CX+side*11, pelvisY
		kx := hipX + side*(7+crouch*9+tuck*10)
		ky := (hipY+ankleY)/2 + 4 + crouch*5

		thigh := render.NewPath()
		thigh.MoveTo(sx(hipX), sy(hipY))
		thigh.LineTo(sx(kx), sy(ky))
		strokePath(pc, thigh, 13, colLegs)

		shin := render.NewPath()
		shin.MoveTo(sx(kx), sy(ky))
		shin.LineTo(sx(hipX+side*2), sy(ankleY))
		strokePath(pc, shin, 10, colShin)

		shoe := render.NewPath()
		shoe.Ellipse(sx(hipX+side*5), sy(ankleY+10), sw(12), sw(6.5))
		fillPath(pc, shoe, colShoe)
		strokePathOutline(pc, shoe, 1.5, colShoeOut)
	}

	// 躯干：填充轮廓形（肩部圆弧 + 髋部圆弧）
	torso := render.NewPath()
	torso.MoveTo(sx(CX-22), sy(shoulderY+6))
	torso.QuadraticTo(sx(CX), sy(shoulderY-5), sx(CX+22), sy(shoulderY+6))
	torso.LineTo(sx(CX+18), sy(pelvisY))
	torso.QuadraticTo(sx(CX), sy(pelvisY+10), sx(CX-18), sy(pelvisY))
	torso.Close()
	fillPath(pc, torso, colTorsoJ)
	strokePathOutline(pc, torso, 2, colTorsoJOut)

	// 手臂：随跳跃自然张开（肘外偏 + 下垂）
	for _, side := range [2]float64{-1, 1} {
		shx, shy := CX+side*20, shoulderY+8
		hx := CX + side*(34+tuck*8)
		hy := shy + 52 - j*0.25 - tuck*14
		ex := (shx+hx)/2 + side*8
		ey := (shy+hy)/2 + 6
		arm := render.NewPath()
		arm.MoveTo(sx(shx), sy(shy))
		arm.LineTo(sx(ex), sy(ey))
		arm.LineTo(sx(hx), sy(hy))
		strokePath(pc, arm, 11, colTorsoJ)
	}

	// 头 + 头发 + 眼睛 + 嘴
	head := render.NewPath()
	head.Circle(sx(CX), sy(headCY), sw(24))
	fillPath(pc, head, colSkin)
	strokePathOutline(pc, head, 2, colSkinOut)

	hairP := render.NewPath()
	hairP.MoveTo(sx(CX-24), sy(headCY-6))
	hairP.Arc(sx(CX), sy(headCY-6), sw(24), math.Pi, 2*math.Pi)
	hairP.LineTo(sx(CX+20), sy(headCY-12))
	hairP.QuadraticTo(sx(CX), sy(headCY-34), sx(CX-20), sy(headCY-12))
	hairP.Close()
	fillPath(pc, hairP, colHairL)

	for _, ex := range [2]float64{-8, 8} {
		rendering.FillCircle(pc, sx(CX+ex), sy(headCY-2), sw(2.6),
			colEye[0], colEye[1], colEye[2], 1)
	}
	mouth := render.NewPath()
	mouth.MoveTo(sx(CX-7), sy(headCY+9))
	mouth.QuadraticTo(sx(CX), sy(headCY+15), sx(CX+7), sy(headCY+9))
	strokePath(pc, mouth, 2.5, colMouth)
}

// paintEllipse 画椭圆（cx,cy,rx,ry 为 HTML 坐标）。
func paintEllipse(pc *rendering.PaintContext, cx, cy, rx, ry float64, col [4]float64) {
	e := render.NewPath()
	e.Ellipse(sx(cx), sy(cy), sw(rx), sw(ry))
	fillPath(pc, e, col)
}

// fillPath / strokePath：路径绘制小包装（自动清空路径）。
func fillPath(pc *rendering.PaintContext, p *render.Path, col [4]float64) {
	defer p.Clear()
	rendering.FillPath(pc, p, col[0], col[1], col[2], col[3])
}

func strokePath(pc *rendering.PaintContext, p *render.Path, w float64, col [4]float64) {
	defer p.Clear()
	rendering.StrokePath(pc, p, sw(w), col[0], col[1], col[2], col[3])
}

// strokePathOutline 人物深色轮廓描边：先描加宽低透明度同色裙边再描本体，
// 把描边外缘的抗锯齿台阶扩成多级渐变（本体线保持锐利，不糊形状内部）。
func strokePathOutline(pc *rendering.PaintContext, p *render.Path, w float64, col [4]float64) {
	rendering.StrokePath(pc, p, sw(w)*2.2, col[0], col[1], col[2], col[3]*0.20)
	rendering.StrokePath(pc, p, sw(w), col[0], col[1], col[2], col[3])
	p.Clear()
}
