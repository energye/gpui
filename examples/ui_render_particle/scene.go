package main

import (
	"math"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/rendering"
)

// particleScene 组装粒子舞台：根 AbsoluteBox + 全屏自定义绘制节点 + 文字标签 + LiveHUD。
type particleScene struct {
	Root  *rendering.AbsoluteBox
	stage *particleStage
	hud   *wrkit.LiveHUD
}

// particleStage 是粒子舞台节点：OnPaint 里用 PaintContext 绘制全部粒子。
// 每帧 tick 推进 sim 状态并 MarkNeedsPaint（无 RepaintBoundary，整窗重绘 =
// 游戏全屏特效的常态；保留边界缓存对全屏粒子无意义）。
type particleStage struct {
	box *rendering.RenderBox
	sim *particleSim
}

func newParticleScene(winW, winH float64) *particleScene {
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.04, G: 0.05, B: 0.09, A: 1}

	s := &particleScene{Root: root}

	// 粒子舞台（全屏）
	stage := &particleStage{sim: newParticleSim(winW, winH)}
	box := rendering.NewRenderBox()
	box.FixedWidth = winW
	box.FixedHeight = winH
	box.OnPaint = stage.paint
	stage.box = box
	s.stage = stage
	root.Place(box, 0, 0)

	// 说明标签（需要默认字体；无字体时节点仍存在但不绘制文字）
	face, _, faceErr := wrkit.EnsureUIFace()
	labels := []struct {
		x, y float64
		text string
		r, g, b, a float64
	}{
		{20, 28, "火球术 — 同心盘立体球 + 径向渐变光晕 + 拖尾火花", 1, 0.55, 0.2, 0.95},
		{20, 52, "冰霜新星 — 扩散环 + 放射冰晶 + 环绕粒子", 0.5, 0.85, 1, 0.95},
		{20, 76, "魔法旋涡 — 螺旋粒子群 + 中心紫光（90 粒）", 0.8, 0.5, 1, 0.95},
		{20, 100, "星空 — 加色混合闪烁（140 粒）", 0.75, 0.8, 0.9, 0.9},
	}
	for _, l := range labels {
		t := rendering.NewRenderText(l.text)
		t.FontSize = 13
		t.R, t.G, t.B, t.A = l.r, l.g, l.b, l.a
		if faceErr == nil {
			t.SetFace(face)
		}
		root.Place(t, l.x, l.y)
	}

	// LiveHUD 底部指标带
	s.hud = wrkit.NewLiveHUD(winW, 64)
	root.Place(s.hud.Box, 0, winH-64)

	return s
}

func (s *particleScene) onTick(dt float64) {
	if s == nil || s.stage == nil {
		return
	}
	s.stage.sim.tick(dt)
	s.stage.box.MarkNeedsPaint()
}

// particleSim 是粒子模拟状态：每帧更新，paint 只读。
type particleSim struct {
	winW, winH float64
	t          float64

	// 火球：沿椭圆轨道移动，火花角度基准
	fireX, fireY float64

	// 冰环：扩散相位（0..1 循环）
	icePhase float64

	// 星空：固定种子点（闪烁用 sin(t+seed)）
	stars [140][2]float64
}

func newParticleSim(winW, winH float64) *particleSim {
	s := &particleSim{winW: winW, winH: winH}
	for i := range s.stars {
		seed := float64(i) * 0.61803
		s.stars[i][0] = math.Mod(seed*winW*1.3, winW)
		s.stars[i][1] = math.Mod(seed*winH*2.1, winH)
	}
	return s
}

func (s *particleSim) tick(dt float64) {
	if s == nil {
		return
	}
	s.t += dt
	// 火球：椭圆轨道
	s.fireX = 780 + math.Sin(s.t*0.8)*70
	s.fireY = 460 + math.Cos(s.t*0.5)*40
	// 冰环相位循环
	s.icePhase = math.Mod(s.t, 1.0)
}

// paint 用 PaintContext 绘制一帧粒子。坐标逻辑像素，Y-down，舞台原点 (0,0)。
func (st *particleStage) paint(pc *rendering.PaintContext, size rendering.Size) {
	sim := st.sim
	if sim == nil {
		return
	}
	w, h := size.Width, size.Height
	t := sim.t

	// 背景：顶部深蓝 → 底部暗紫（三条半透明色带）
	rendering.FillRect(pc, 0, 0, w, h, 0.03, 0.04, 0.09, 1)
	rendering.FillRect(pc, 0, h/3, w, h*2/3, 0.05, 0.03, 0.10, 0.6)
	rendering.FillRect(pc, 0, h*2/3, w, h/3, 0.08, 0.02, 0.12, 0.5)

	// 星空：闪烁小亮点
	for i, p := range sim.stars {
		seed := float64(i) * 0.61803
		tw := 0.4 + 0.6*math.Abs(math.Sin(t*6.0+seed*12.0))
		r := 0.8 + math.Mod(seed*9, 0.7)
		rendering.FillCircle(pc, p[0], p[1], r, 0.9, 0.92, 1, tw*0.55)
	}

	// 火球术（右下轨道）
	st.paintFireball(pc, sim)

	// 冰霜新星（左上）
	st.paintIceNova(pc, sim)

	// 魔法旋涡（中心）
	st.paintVortex(pc, sim)
}

func (st *particleStage) paintFireball(pc *rendering.PaintContext, sim *particleSim) {
	cx, cy := sim.fireX, sim.fireY

	// 尾迹：火球飞过的弧线（半透明橙）
	rendering.StrokeArc(pc, cx-150, cy-110, 200, 0.15, 1.15, 10, 0.9, 0.35, 0.05, 0.22)

	// 外层光晕：径向渐变（中心亮橙 → 透明），模拟火光柔光
	rendering.FillRadialGradient(pc, cx-120, cy-120, 240, 240, cx, cy, 20, 110,
		1, 0.35, 0.05, 0.30, 1, 0.25, 0.04, 0.0)

	// 球体本体：三层同心盘（亮心→橙→暗红边）= 2D 立体球
	rendering.FillCircle(pc, cx, cy, 46, 0.55, 0.08, 0.02, 0.9)
	rendering.FillCircle(pc, cx, cy, 34, 1, 0.45, 0.08, 0.95)
	rendering.FillCircle(pc, cx-8, cy-8, 20, 1, 0.85, 0.4, 1) // 高光偏左上

	// 溅射火花：沿反向拖出小粒子
	for i := 0; i < 26; i++ {
		seed := float64(i) * 0.47
		ang := 2.2 + seed*0.5
		dist := float64(i%8)*4.0 + 6
		px := cx - math.Cos(ang)*dist*1.4
		py := cy - math.Sin(ang)*dist*0.7 - float64(i)*2.5
		r := 5 - float64(i%4)*0.8
		alpha := 0.55 - float64(i)*0.012
		if alpha < 0.05 {
			alpha = 0.05
		}
		rendering.FillCircle(pc, px, py, r, 1, 0.55+seed*0.4, 0.1, alpha)
	}
}

func (st *particleStage) paintIceNova(pc *rendering.PaintContext, sim *particleSim) {
	cx, cy := 210.0, 210.0
	ph := sim.icePhase

	// 扩散的冰环（两层，随相位扩散后重置）
	for ring := 0; ring < 2; ring++ {
		rr := 40 + float64(ring)*28 + math.Mod(ph+float64(ring)*0.5, 1.0)*18
		rendering.StrokeCircle(pc, cx, cy, rr, 2.5, 0.5, 0.85, 1, 0.45-float64(ring)*0.15)
	}

	// 放射冰晶线（8 条，旋转）
	rot := sim.t * 1.2
	for i := 0; i < 8; i++ {
		ang := float64(i)*math.Pi/4 + rot
		ex := cx + math.Cos(ang)*95
		ey := cy + math.Sin(ang)*95
		rendering.StrokeLine(pc, cx, cy, ex, ey, 3, 0.7, 0.92, 1, 0.5)
	}

	// 环绕粒子：沿圆周分布的小冰晶
	for i := 0; i < 24; i++ {
		seed := float64(i) * 0.2618
		rr := 62 + 8*math.Sin(sim.t*3+seed*4)
		ang := seed*2*math.Pi + sim.t*1.5
		px := cx + math.Cos(ang)*rr
		py := cy + math.Sin(ang)*rr
		sz := 2.5 + math.Mod(seed*7, 2.5)
		rendering.FillCircle(pc, px, py, sz, 0.6, 0.9, 1, 0.85)
	}

	// 中心冰核
	rendering.FillCircle(pc, cx, cy, 18, 0.7, 0.95, 1, 0.95)
	rendering.FillCircle(pc, cx-4, cy-4, 9, 1, 1, 1, 1)
}

func (st *particleStage) paintVortex(pc *rendering.PaintContext, sim *particleSim) {
	cx, cy := 600.0, 400.0
	t := sim.t

	// 中心紫光（径向渐变光晕）
	rendering.FillRadialGradient(pc, cx-90, cy-90, 180, 180, cx, cy, 10, 80,
		0.7, 0.3, 1, 0.35, 0.6, 0.2, 1, 0.0)

	// 螺旋粒子群：绕中心旋转，越往外越散，色相渐变（紫→蓝→青→绿）
	n := 90
	for i := 0; i < n; i++ {
		seed := float64(i) / float64(n)
		spiral := seed * 4.5
		rr := 18 + seed*130
		ang := spiral*2*math.Pi + t*2.2 + seed*0.8
		px := cx + math.Cos(ang)*rr
		py := cy + math.Sin(ang)*rr
		hue := 0.75 - seed*0.35
		r, g, b := hsv(hue, 0.85, 0.95)
		sz := 3.0 + seed*4.0
		alpha := 0.9 - seed*0.35
		rendering.FillCircle(pc, px, py, sz, r, g, b, alpha)
	}

	// 漩涡中心核
	rendering.FillCircle(pc, cx, cy, 22, 0.85, 0.55, 1, 0.95)
	rendering.FillCircle(pc, cx-5, cy-5, 10, 1, 0.9, 1, 1)
}

func hsv(hue, s, v float64) (float64, float64, float64) {
	i := int(hue * 6)
	f := hue*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - f*s)
	tt := v * (1 - (1-f)*s)
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, tt, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, tt
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = tt, p, v
	case 5:
		r, g, b = v, p, q
	}
	return r, g, b
}
