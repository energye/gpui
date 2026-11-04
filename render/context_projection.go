//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// //----------------------------------------

package render

// P3.7 批 1d · Canvas 级 4×4 投影层（并存式，P3.5 批 3 D4 挂账）。
//
// 形态（对齐 Skia 的限定域口径：SkM44 允许 CTM 带透视，但 SkCanvas 对透视 CTM
// 的能力是限定域——图像/矩形走 persp2d，路径与文字有额外约束；Godot 2D canvas
// 仿射-only，投影由引擎算好喂渲染器 = 本批「引擎算好、render 只收」同构）：
//   - 投影层是 4×4，作用在用户坐标 → 2×3 CTM 之前（设备 = totalMatrix ∘ projection）；
//   - 仅图像 quad / 批绘透视腿走真投影；路径/文字/裁剪按显式表态（降级/映射/跳过，
//     各带具名计数，禁止静默压扁）；
//   - 零值 = 恒等 ⇒ 全部消费点走历史路径逐位不变（`projectionActive` 快速位护栏）；
//   - Push/Pop 与 2×3 矩阵同拍保存/恢复（Skia SkM44 风格，拍板 D-1）。

// SetProjection replaces the projection layer with m (does not compose with the
// previous projection; compose with Matrix4.Mul yourself). m 非有限时按恒等处理
// 并忽略（fail-closed：不装坏矩阵）。零值/恒等矩阵 = 关闭投影层。
func (c *Context) SetProjection(m Matrix4) {
	if c == nil {
		return
	}
	if m == (Matrix4{}) {
		// 零值 Matrix4 不可用（P3.5 批 3 契约）：按文档口径当作恒等=关闭，绝不把
		// 全零矩阵装进链里（装了会让每个绘制点映射到空，画面直接画白）。
		m = Matrix4Identity()
	}
	if !m.IsIdentity() && !matrix4AllFinite(&m) {
		return
	}
	c.projection = m
	c.projectionActive = !m.IsIdentity()
}

// PushProjection saves the current projection layer and replaces it with m —
// the projection-only counterpart of Push(). PopProjection restores.
func (c *Context) PushProjection(m Matrix4) {
	if c == nil {
		return
	}
	// 存归一化快照（恒等态存 Matrix4Identity() 而不是零值）：零值 Matrix4 的
	// IsIdentity() 为 false，直接存 c.projection 会让 Pop 回来的「零值 + active=true」
	// 把上下文污染成装了全零投影矩阵（D6 状态钉实测：一次 Push/Pop 之后画面全白）。
	c.projectionStack = append(c.projectionStack, c.Projection())
	c.SetProjection(m)
}

// projRestore installs a projection snapshot, keeping the active flag consistent
// with it. 只有 Projection()/projSnapshot 的归一化值可以传进来。
func (c *Context) projRestore(m Matrix4) {
	c.projection = m
	c.projectionActive = !m.IsIdentity()
}

// PopProjection restores the projection layer saved by the last PushProjection
// (or Push, which snapshots it together with the matrix). Empty stack = no-op.
func (c *Context) PopProjection() {
	if c == nil || len(c.projectionStack) == 0 {
		return
	}
	m := c.projectionStack[len(c.projectionStack)-1]
	c.projectionStack = c.projectionStack[:len(c.projectionStack)-1]
	c.projRestore(m)
}

// Projection returns the current projection layer. 恒等时返回 Matrix4Identity()
// （零值上下文不给零值矩阵——Matrix4 零值不可用是 P3.5 批 3 的契约）。
func (c *Context) Projection() Matrix4 {
	if c == nil || !c.projectionActive {
		return Matrix4Identity()
	}
	return c.projection
}

// projActive 报告投影层是否非恒等（全部消费点的快速护栏位）。
// projFlattenTolerance 是投影链展平曲线用的容差（用户单位）。此前 5 个消费点各写
// 一遍 0.25 字面量（批 1d 去重），与 path_metrics.go「tolerance ≤0 → 0.25」的默认
// 展平容差同源。
const projFlattenTolerance = 0.25

func (c *Context) projActive() bool {
	return c != nil && c.projectionActive
}

// matrix4AllFinite reports whether all 16 entries are finite.
func matrix4AllFinite(m *Matrix4) bool {
	for i := range m.m {
		if v := m.m[i]; v != v || v > 1e308 || v < -1e308 { // NaN / ±Inf
			return false
		}
	}
	return true
}

// projDeviceFromBaked maps a point that is already in user-matrix space (path
// verbs bake c.matrix at emit time) — device = deviceMatrix ∘ projection ∘ p.
// Returns the per-point homogeneous w. ok=false when the point is behind the
// eye (w ≤ 0) — callers refuse.
func (c *Context) projDeviceFromBaked(p Point) (Point, float64, bool) {
	spt, pw, okP := c.projection.ProjectPoint(p.X, p.Y, 0)
	if !okP {
		return Point{}, 0, false
	}
	if c.deviceMatrix.IsIdentity() {
		return spt, pw, true
	}
	return c.deviceMatrix.TransformPoint(spt), pw, true
}

// projDeviceFromUser maps a user-space point — device = deviceMatrix ∘
// projection ∘ matrix ∘ user (拍板 B：投影作用在用户矩阵**之外**，模型几何先走
// 用户矩阵、再过相机投影，最后叠设备缩放；与路径/裁剪链同序，两条链不再分叉)。
func (c *Context) projDeviceFromUser(p Point) (Point, float64, bool) {
	return c.projDeviceFromBaked(c.matrix.TransformPoint(p))
}

// projTransformPoint maps one user point through the full chain (alias of
// projDeviceFromUser — quad/atlas/image callers hold user-space corners).
func (c *Context) projTransformPoint(p Point) (dev Point, w float64, ok bool) {
	return c.projDeviceFromUser(p)
}

// projMapQuad maps four user corners through the projection layer + CTM.
// All-or-nothing: any corner behind the eye or non-finite refuses the whole
// quad (same semantics as Matrix4.ProjectQuad / P3.5「眼后整块拒绝」).
// projActive callers only; identity callers keep their historic path.
func (c *Context) projMapQuad(corners [4]Point) (dev [4]Point, w [4]float64, ok bool) {
	for i := 0; i < 4; i++ {
		d, pw, okP := c.projDeviceFromUser(corners[i])
		if !okP {
			return [4]Point{}, [4]float64{}, false
		}
		dev[i] = d
		w[i] = pw
	}
	return dev, w, true
}

// projTotalAffine returns the effective user→device map for chains that can only
// express affine geometry, following 拍板 B 的链序：
//
//	identity projection ⇒ totalMatrix()（逐位同旧，存量护栏）
//	lifted affine projection ⇒ deviceMatrix ∘ projection ∘ matrix（提升态一并吃下）
//	projective projection ⇒ ok=false（2×3 表达不了，调用方必须显式表态）
//
// 静默压扁是被禁止的（matrix4.go Affine() 契约）。
func (c *Context) projTotalAffine() (Matrix, bool) {
	if !c.projectionActive {
		return c.totalMatrix(), true
	}
	aff, ok := c.projection.Affine()
	if !ok {
		return Matrix{}, false
	}
	if c.deviceMatrix.IsIdentity() {
		return aff.Multiply(c.matrix), true
	}
	return c.deviceMatrix.Multiply(aff).Multiply(c.matrix), true
}

// projTransformPath maps a path from **user-matrix (baked) space** into device
// space — path verbs bake c.matrix at emit time, so the incoming coordinates
// already carry the user matrix (拍板 B：投影在其外)。Projective maps preserve
// straight lines, so every curve is flattened to segments first (tolerance in
// user units) and only the samples are mapped — the result is the exact
// projected polyline. Callers holding a *user-space* path (clip builders) must
// bake it with c.matrix first. ok=false when any sample lands behind the eye
// (caller refuses/degrades per stance).
func (c *Context) projTransformPath(p *Path, tolerance float64) (*Path, bool) {
	if p == nil || len(p.verbs) == 0 {
		return NewPath(), true
	}
	out := NewPath()
	var cur, start Point
	started := false
	ok := true
	p.Iterate(func(verb PathVerb, coords []float64) {
		if !ok {
			return
		}
		switch verb {
		case MoveTo:
			if started {
				out.Close()
			}
			d, _, okP := c.projDeviceFromBaked(Pt(coords[0], coords[1]))
			if !okP {
				ok = false
				return
			}
			out.MoveTo(d.X, d.Y)
			start, cur, started = d, d, true
		case LineTo:
			d, _, okP := c.projDeviceFromBaked(Pt(coords[0], coords[1]))
			if !okP {
				ok = false
				return
			}
			out.LineTo(d.X, d.Y)
			cur = d
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			cur = projFlattenQuadMapped(c, cur, ctrl, pt, &ok, out)
		case CubicTo:
			c1 := Pt(coords[0], coords[1])
			c2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			last := cur
			projFlattenCubicMapped(c, cur, c1, c2, pt, tolerance, &ok, out, &last)
			cur = last
		case ConicTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			w := coords[4]
			last := cur
			projFlattenConicMapped(c, cur, ctrl, pt, w, tolerance, &ok, out, &last)
			cur = last
		case Close:
			out.Close()
			cur = start
		}
	})
	if !ok {
		return nil, false
	}
	return out, true
}

// projFlattenQuadMapped flattens one quad and maps each sample; returns the final point.
func projFlattenQuadMapped(c *Context, p0, p1, p2 Point, ok *bool, out *Path) Point {
	const steps = 16
	last := p0
	for i := 1; i <= steps; i++ {
		t := float64(i) / steps
		mt := 1 - t
		x := mt*mt*p0.X + 2*mt*t*p1.X + t*t*p2.X
		y := mt*mt*p0.Y + 2*mt*t*p1.Y + t*t*p2.Y
		d, _, okP := c.projDeviceFromBaked(Pt(x, y))
		if !okP {
			*ok = false
			return last
		}
		out.LineTo(d.X, d.Y)
		last = d
	}
	return last
}

// projFlattenCubicMapped flattens one cubic and maps each sample.
func projFlattenCubicMapped(c *Context, p0, p1, p2, p3 Point, _ float64, ok *bool, out *Path, last *Point) {
	const steps = 16
	for i := 1; i <= steps; i++ {
		t := float64(i) / steps
		mt := 1 - t
		x := mt*mt*mt*p0.X + 3*mt*mt*t*p1.X + 3*mt*t*t*p2.X + t*t*t*p3.X
		y := mt*mt*mt*p0.Y + 3*mt*mt*t*p1.Y + 3*mt*t*t*p2.Y + t*t*t*p3.Y
		d, _, okP := c.projDeviceFromBaked(Pt(x, y))
		if !okP {
			*ok = false
			return
		}
		out.LineTo(d.X, d.Y)
		*last = d
	}
}

// projFlattenConicMapped flattens one conic and maps each sample.
func projFlattenConicMapped(c *Context, p0, p1, p2 Point, w float64, _ float64, ok *bool, out *Path, last *Point) {
	const steps = 16
	b := NewConicBez(p0, p1, p2, w)
	for i := 1; i <= steps; i++ {
		pt := b.Eval(float64(i) / steps)
		d, _, okP := c.projDeviceFromBaked(pt)
		if !okP {
			*ok = false
			return
		}
		out.LineTo(d.X, d.Y)
		*last = d
	}
}

// withProjectedPathScope runs fn with the (already device-space) path in place
// and identity matrices — the projected path feeds the historic pipelines
// unchanged; both matrix layers are neutralized because the mapping already
// produced device coordinates. State is restored on return.
func (c *Context) withProjectedPathScope(devPath *Path, fn func() error) error {
	savedPath, savedMatrix, savedDevice := c.path, c.matrix, c.deviceMatrix
	c.path = devPath
	c.matrix = Identity()
	c.deviceMatrix = Identity()
	err := fn()
	c.path, c.matrix, c.deviceMatrix = savedPath, savedMatrix, savedDevice
	return err
}

// projTextBlocked reports whether the current draw must skip text. 拍板 B 之下
// 文字链还没接投影映射器（字形落位走 verb 期烘焙的用户矩阵，20 处烘焙缝不在本批
// 收），所以**只要投影层非恒等就整笔不画 + 具名计数**（fail-closed：宁可少画，
// 不画「半条链吃投影、半条链不吃」的错位画面）。后续专项：文字族接 baked 映射器
// （提升态可精确跟上）+ 透视文字 mesh warp（非仿射）。
func (c *Context) projTextBlocked() bool {
	if !c.projActive() {
		return false
	}
	c.projTextSkips++
	return true
}

// projTotalMatrix4 returns the full user→device chain as a 4×4 —
// deviceMatrix ∘ projection ∘ matrix（拍板 B 的链序）— for callers that need a
// true projective inverse (ClipBounds). 恒等投影时原样返回 totalMatrix() 的 4×4
// 提升（逐位同旧，不做两次多余乘法）。ok=false 只在矩阵奇异时。
func (c *Context) projTotalMatrix4() (Matrix4, bool) {
	if !c.projectionActive {
		return Matrix4FromMatrix(c.totalMatrix()), true
	}
	return Matrix4FromMatrix(c.deviceMatrix).Mul(c.projection).Mul(Matrix4FromMatrix(c.matrix)), true
}

// ProjectionStats 是投影层非仿射表态的具名计数快照（P3.7 批 1d）。
// 口径：计数只在「投影层装了非恒等矩阵且该消费点无法用 2×3 精确表达」时增长，
// 恒等投影下恒为 0 —— 降级/跳过一律可观测，禁止静默压扁。
type ProjectionStats struct {
	ClipDegrades     int // C 组：矩形/RRect 裁剪改走投影 quad/展平路径裁剪
	PathExpansions   int // D 组：路径展平+逐点投影映射后走设备空间管线
	TextSkips        int // E 组：投影层非恒等下文字整笔不画（fail-closed，含描字）
	GPUTextureAABBs  int // GPU 纹理/视频三口：队列口无 quad 形参 ⇒ 投影四角 AABB
	BrushBakes       int // F 组：设备空间刷烘焙无法表达 ⇒ 跳过烘焙（相位近似）
	ShadowApprox     int // F 组：阴影抬升/模糊比例按 2×3 层算（剪影本身仍走投影映射）
	PatternApprox    int // F 组：非 Normal 混合的图像走图样路（几何按投影、采样核按 2×3）
	InterpDowngrades int // 采样核：透视槽无 bicubic 变体 ⇒ 降 bilinear（quad/图集 GPU/图集 CPU 三口）
}

// ProjectionStats returns the projection-layer census since the last reset.
// 读口是累计值（不随帧清零），复位走 ResetProjectionStats。
func (c *Context) ProjectionStats() ProjectionStats {
	if c == nil {
		return ProjectionStats{}
	}
	return ProjectionStats{
		ClipDegrades:     c.projClipDegrades,
		PathExpansions:   c.projPathExpands,
		TextSkips:        c.projTextSkips,
		GPUTextureAABBs:  c.projGPUTexDegrades,
		BrushBakes:       c.projBrushDegrades,
		ShadowApprox:     c.projShadowDegrades,
		PatternApprox:    c.projPatternDegrades,
		InterpDowngrades: c.projInterpDegrades,
	}
}

// ResetProjectionStats zeroes the projection census and returns the value it
// replaced (window/HUD 逐段计量的习惯口）。
func (c *Context) ResetProjectionStats() ProjectionStats {
	if c == nil {
		return ProjectionStats{}
	}
	prev := c.ProjectionStats()
	c.projClipDegrades, c.projPathExpands, c.projTextSkips = 0, 0, 0
	c.projGPUTexDegrades, c.projBrushDegrades, c.projInterpDegrades = 0, 0, 0
	c.projShadowDegrades, c.projPatternDegrades = 0, 0
	return prev
}
