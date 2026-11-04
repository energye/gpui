//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/text"
)

// P3.7 批 1d · 旁路表态钉（12 枚）。
//
// 主链钉（p37_projection_test.go / _chain_test.go）咬的是 quad/图集/填充/矩形裁剪/文字跳过/录制；
// 这组把批 1d 实际改过、但主链没碰的旁路逐个咬住：ClipBounds 的 4×4 精确逆、描边路、RRect 裁剪与
// 两条快路门、老 DrawAtlas 升格、网腿 GPU 臂与索引 CPU 臂、GPU 纹理/视频三口 AABB、文字七个入口、
// DrawShadow——外加两枚此前从未被任何断言读过的计数（GPUTextureAABBs / Misc）。
//
// 链序=拍板 B：设备 = deviceMatrix ∘ 投影 ∘ 用户矩阵。离屏上下文 deviceMatrix=恒等，故
// 解析式 = ((100+x+dx)/w, (65+y+dy)/w)，w = 1 − 0.003·(y+dy)，与 projHomography() 同一份公式。

// stanceDevice 按链序 B 手算用户点 (ux,uy) 在「用户矩阵=Translate(dx,dy) + 地面单应性」下的设备点。
func stanceDevice(dx, dy, ux, uy float64) (render.Point, float64) {
	bx, by := ux+dx, uy+dy
	w := 1 - 0.003*by
	return render.Pt((100+bx)/w, (65+by)/w), w
}

// stanceQuad 手算用户矩形的投影四角（TL/TR/BR/BL）。
func stanceQuad(dx, dy float64, x, y, w0, h0 float64) [4]render.Point {
	corners := [][2]float64{{x, y}, {x + w0, y}, {x + w0, y + h0}, {x, y + h0}}
	var out [4]render.Point
	for i, c := range corners {
		p, _ := stanceDevice(dx, dy, c[0], c[1])
		out[i] = p
	}
	return out
}

// stanceInkNear 报告设备点 (x,y) 的 r 邻域内是否有墨（探针用容差，声明见各钉）。
func stanceInkNear(img image.Image, x, y, r int) bool {
	b := img.Bounds()
	for yy := y - r; yy <= y+r; yy++ {
		for xx := x - r; xx <= x+r; xx++ {
			if xx < b.Min.X || yy < b.Min.Y || xx >= b.Max.X || yy >= b.Max.Y {
				continue
			}
			if probeInk(img, xx, yy) {
				return true
			}
		}
	}
	return false
}

func stanceNewCtx(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

// ClipBounds 必须用「2×3 ∘ 4×4」的精确 4×4 逆把投影后的设备裁剪映回用户空间。
// 只走 2×3 逆（旧形态）会按投影后的位置给界（y 界≈104..213，本钉上限 110）。
// 如实登记：映回的是「设备 AABB 的逆映像」，是用户矩形的保守超集（投影下裁剪是梯形，
// 其外接框逆映比原矩形宽），所以判据写「包住 + 不跑偏」，不假装精确。
func TestP37ProjectionClipBoundsUsesProjectiveInverse(t *testing.T) {
	dc := stanceNewCtx(320, 220)
	defer func() { _ = dc.Close() }()
	dc.SetProjection(projHomography())
	dc.Transform(render.Translate(50, 0))
	dc.ClipRect(20, 30, 100, 60)

	b := dc.ClipBounds()
	if b.Min.X > 20 || b.Min.Y > 30 || b.Max.X < 120 || b.Max.Y < 90 {
		t.Fatalf("ClipBounds 没包住用户裁剪矩形 (20,30)-(120,90)：%v", b)
	}
	if b.Max.Y > 110 {
		t.Fatalf("ClipBounds 仍按 2×3 逆算：y 界=%d（旧形态会给到 ≈213）：%v", b.Max.Y, b)
	}
	t.Logf("ClipBounds 投影下=%v（保守超集；2×3 逆旧形态会给 y≈104..213）", b)

	dc2 := stanceNewCtx(200, 200)
	defer func() { _ = dc2.Close() }()
	dc2.Transform(render.Scale(2, 2))
	dc2.ClipRect(10, 10, 30, 20)
	b2 := dc2.ClipBounds()
	if b2.Min.X < 4 || b2.Min.Y < 4 || b2.Max.X < 34 || b2.Max.Y < 24 || b2.Dx() > 46 {
		t.Fatalf("恒等投影下 ClipBounds 改了存量口径：%v，期望 ≈(10,10)-(40,30)", b2)
	}
	t.Log("ClipBounds：真透视走 4×4 精确逆（保守超集）、恒等逐位同旧")
}

// 描边路：投影下描边 ≡ 对同一三角形「手算投影后以恒等矩阵描边」（逐位）。
// 此前 doStroke 的投影 stance 零钉（填充有）。
func TestP37ProjectionStrokeMatchesProjectedPath(t *testing.T) {
	tri := [3][2]float64{{20, 30}, {120, 30}, {120, 90}}

	a := stanceNewCtx(320, 220)
	defer func() { _ = a.Close() }()
	a.SetProjection(projHomography())
	a.SetRGBA(0, 0, 0, 1)
	a.SetLineWidth(4)
	a.NewSubPath()
	a.MoveTo(tri[0][0], tri[0][1])
	a.LineTo(tri[1][0], tri[1][1])
	a.LineTo(tri[2][0], tri[2][1])
	a.ClosePath()
	if err := a.Stroke(); err != nil {
		t.Fatalf("投影下描边：%v", err)
	}

	b := stanceNewCtx(320, 220)
	defer func() { _ = b.Close() }()
	b.SetRGBA(0, 0, 0, 1)
	b.SetLineWidth(4)
	b.NewSubPath()
	for i, c := range tri {
		d, _ := stanceDevice(0, 0, c[0], c[1])
		if i == 0 {
			b.MoveTo(d.X, d.Y)
		} else {
			b.LineTo(d.X, d.Y)
		}
	}
	b.ClosePath()
	if err := b.Stroke(); err != nil {
		t.Fatal(err)
	}

	if changed := probeDiff(a.Image(), b.Image()); changed != 0 {
		t.Fatalf("投影描边 ≡ 投影多边形描边 未逐位一致（差异格=%d）", changed)
	}
	if st := a.ProjectionStats(); st.PathExpansions == 0 {
		t.Fatal("描边路径映射未被计数（D 组描边侧缺观测面）")
	}
	t.Log("描边路 ≡ 投影多边形描边（逐位）+ 计数在位")
}

//  3. ClipRoundRect：裁剪必须跟着投影走——用「用户空间内外探针 + 解析落点」判，
//     不依赖实现自己的展平结果（判据从投影公式反推）。容差：探针邻域 r=2。
func TestP37ProjectionClipRoundRectFollowsProjection(t *testing.T) {
	const dx, dy = 30.0, 0.0
	dc := stanceNewCtx(320, 220)
	defer func() { _ = dc.Close() }()
	dc.SetProjection(projHomography())
	dc.Transform(render.Translate(dx, dy))
	dc.ClipRoundRect(20, 30, 120, 60, 20)
	dc.SetRGBA(0, 0, 0, 1)
	dc.NewSubPath()
	dc.MoveTo(-40, -40)
	dc.LineTo(300, -40)
	dc.LineTo(300, 300)
	dc.LineTo(-40, 300)
	dc.ClosePath()
	if err := dc.Fill(); err != nil {
		t.Fatal(err)
	}
	img := dc.Image()

	// 用户空间：中心 (80,60) 必在圆角矩形内；(21,31) 在 AABB 内但在圆角外。
	in, _ := stanceDevice(dx, dy, 80, 60)
	out, _ := stanceDevice(dx, dy, 21, 31)
	if !stanceInkNear(img, int(in.X), int(in.Y), 2) {
		t.Fatalf("裁剪丢内容：用户 (80,60) 在圆角矩形内，投影落点 (%.0f,%.0f) 应被画", in.X, in.Y)
	}
	if stanceInkNear(img, int(out.X), int(out.Y), 2) {
		t.Fatalf("裁剪没跟投影：用户 (21,31) 在圆角外，投影落点 (%.0f,%.0f) 被画 ⇒ 裁剪仍是未按投影的形状", out.X, out.Y)
	}
	if st := dc.ProjectionStats(); st.ClipDegrades == 0 {
		t.Fatal("ClipRoundRect 的投影表态未被计数（C 组圆角侧缺观测面）")
	}
	t.Logf("ClipRoundRect：内点 (%.0f,%.0f) 画、外点 (%.0f,%.0f) 不画，降级计数=%d", in.X, in.Y, out.X, out.Y, dc.ProjectionStats().ClipDegrades)
}

// 提升态 RRect 裁剪 + 两条快路门：装了仿射提升（Scale 2×）后，均匀 RRect 裁剪必须跟着投影
// 放大到设备 (10..120,10..80)，而不是停在用户尺寸。对照=同一画法不设投影，落在原尺寸。
// 如实登记：提升态走「展平+逐点映射」的精确路径，与恒等态的 SDF 均匀快路**非逐位**
// （圆角按 0.25 容差展平），故本钉不比逐位，只比「裁没裁对地方」。
func TestP37ProjectionRRectLiftFollowsProjection(t *testing.T) {
	lift := render.Matrix4FromMatrix(render.Scale(2, 2))

	shoot := func(projection bool) image.Image {
		dc := stanceNewCtx(200, 200)
		defer func() { _ = dc.Close() }()
		if projection {
			dc.SetProjection(lift)
		}
		dc.ClipRRect(render.NewRRectXY(render.Rect{Min: render.Pt(5, 5), Max: render.Pt(60, 40)}, 8, 8))
		dc.SetRGBA(0, 0, 0, 1)
		dc.DrawRectangle(0, 0, 95, 95)
		if err := dc.Fill(); err != nil {
			t.Fatal(err)
		}
		return dc.Image()
	}

	a := shoot(true)
	b := shoot(false)

	// 设备 (100,60) = 用户 (50,30)：在圆角矩形内 ⇒ 提升态必须画；无投影态该点已出裁剪 ⇒ 不画。
	if !stanceInkNear(a, 100, 60, 2) {
		t.Fatal("提升态 RRect 裁剪没跟投影：设备 (100,60)（用户在裁剪内）没被画")
	}
	if stanceInkNear(b, 100, 60, 2) {
		t.Fatal("无投影对照也画到设备 (100,60)：判据失去鉴别力")
	}
	// 用户 (6,6) 在圆角外的 AABB 角 ⇒ 提升后设备 (12,12) 必须无墨（快路门若破了会按 AABB 放行）。
	if stanceInkNear(a, 12, 12, 2) {
		t.Fatal("提升态把圆角外的 AABB 角放进来了：快路门或展平路径破了")
	}

	// 旋转 + 提升：定向 RRect 快路（走 totalMatrix 的解析路）必须被拒，否则裁剪停在未放大的
	// 旋转位，探针点 (100,60) 会落空——这一子例专门给「定向快路门」做鉴别力（上一子例摘掉
	// 该门时十二枚旁路钉全绿，实测红钉 0 后补上）。
	rot := render.Rotate(0.35)
	lifted := render.Scale(2, 2).Multiply(rot)
	c := stanceNewCtx(200, 200)
	defer func() { _ = c.Close() }()
	c.SetProjection(lift)
	c.Transform(rot)
	c.ClipRRect(render.NewRRectXY(render.Rect{Min: render.Pt(5, 5), Max: render.Pt(60, 40)}, 8, 8))
	c.SetRGBA(0, 0, 0, 1)
	c.DrawRectangle(-40, -40, 160, 160)
	if err := c.Fill(); err != nil {
		t.Fatal(err)
	}
	d := stanceNewCtx(200, 200)
	defer func() { _ = d.Close() }()
	d.Transform(lifted)
	d.ClipRRect(render.NewRRectXY(render.Rect{Min: render.Pt(5, 5), Max: render.Pt(60, 40)}, 8, 8))
	d.SetRGBA(0, 0, 0, 1)
	d.DrawRectangle(-40, -40, 160, 160)
	if err := d.Fill(); err != nil {
		t.Fatal(err)
	}
	// 同一用户点在两条链下必须同色：提升态（投影层）≡ 把同一份仿射乘进 CTM。
	// 探针点离圆角边界留足余量（提升态走展平路径，与解析快路只在边界 ~0.5 用户单位的
	// 容差带内可能不同——那正是已登记的展平近似，不该拿它当不一致报）。
	for _, pt := range [2][2]float64{{30, 22}, {4, 4}} {
		e := lifted.TransformPoint(render.Pt(pt[0], pt[1]))
		inA := stanceInkNear(c.Image(), int(e.X), int(e.Y), 2)
		inB := stanceInkNear(d.Image(), int(e.X), int(e.Y), 2)
		if inA != inB {
			t.Fatalf("旋转+提升下裁剪与等价 CTM 不一致：用户 (%.0f,%.0f) → 设备 (%.0f,%.0f)，投影态=%v 等价态=%v",
				pt[0], pt[1], e.X, e.Y, inA, inB)
		}
	}
	t.Log("提升态 RRect 裁剪跟投影（均匀子例 + 旋转子例；定向/均匀两条快路门各有鉴别力）")
}

// 5) 老 DrawAtlas（批 1d 也给它加了升格分支）≡ 1c 批绘透视腿形态。
func TestP37ProjectionLegacyDrawAtlasUpgrades(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	h := projHomography()
	world := [4]render.Point{{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90}}
	screens, weights := projHomographyQuad(h, world)

	a := stanceNewCtx(320, 220)
	defer func() { _ = a.Close() }()
	a.SetProjection(h)
	a.DrawAtlas(img, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32,
		DstX: 0, DstY: 0, DstW: 120, DstH: 90, Opacity: 1,
	}})
	if err := a.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var bufA *render.ImageBuf
	if !a.ExportImageBuf(&bufA) || bufA == nil {
		t.Fatal("读回失败")
	}
	defer bufA.Dispose()

	b := stanceNewCtx(320, 220)
	defer func() { _ = b.Close() }()
	if _, err := b.DrawAtlasEx(img, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
		Perspective: true, Corners: screens, Weights: weights,
	}}, render.AtlasDrawOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := b.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var bufB *render.ImageBuf
	if !b.ExportImageBuf(&bufB) || bufB == nil {
		t.Fatal("读回失败")
	}
	defer bufB.Dispose()

	changed, maxD := diffStats(bufA, bufB)
	if changed != 0 || maxD != 0 {
		t.Fatalf("老 DrawAtlas 在投影下 ≡ 1c 批绘透视腿 未逐位一致（changed=%d max=%d）", changed, maxD)
	}
	// 空角钉：升格后 AABB 左下角在凸包外，不该有墨。
	if r, _, _, _ := bufA.GetRGBA(int(screens[3].X)-4, int(screens[2].Y)-2); r != 255 {
		t.Fatalf("老 DrawAtlas 没升格（AABB 空角被涂色 r=%d）", r)
	}
	t.Log("老 DrawAtlas 在投影层下升格 ≡ 1c 批绘透视腿（逐位 + 空角判别）")
}

//  6. 网腿 GPU 臂：投影下 DrawVertices ≡ 直接喂「手算投影后的顶点」（逐位）。
//     此前只有 CPU 臂（钉 1）被测，GPU 臂零钉。
func TestP37ProjectionMeshGPULegMatchesProjectedVerts(t *testing.T) {
	requireNativeGPU(t)
	user := []render.Point{{X: 20, Y: 20}, {X: 140, Y: 20}, {X: 60, Y: 110}}
	projVerts := make([]render.Point, len(user))
	for i, p := range user {
		d, _ := stanceDevice(0, 0, p.X, p.Y)
		projVerts[i] = d
	}

	a := stanceNewCtx(320, 220)
	defer func() { _ = a.Close() }()
	a.SetProjection(projHomography())
	a.SetRGBA(0, 0, 0, 1)
	a.DrawVertices(user, nil, render.VertexModeTriangles)
	if err := a.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var bufA *render.ImageBuf
	if !a.ExportImageBuf(&bufA) || bufA == nil {
		t.Fatal("读回失败")
	}
	defer bufA.Dispose()

	b := stanceNewCtx(320, 220)
	defer func() { _ = b.Close() }()
	b.SetRGBA(0, 0, 0, 1)
	b.DrawVertices(projVerts, nil, render.VertexModeTriangles)
	if err := b.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var bufB *render.ImageBuf
	if !b.ExportImageBuf(&bufB) || bufB == nil {
		t.Fatal("读回失败")
	}
	defer bufB.Dispose()

	changed, maxD := diffStats(bufA, bufB)
	if changed != 0 || maxD != 0 {
		t.Fatalf("网腿 GPU 臂 ≡ 预投影顶点 未逐位一致（changed=%d max=%d）", changed, maxD)
	}
	t.Log("网腿 GPU 臂与预投影顶点逐位一致（顶点映射一次，几何精确）")
}

// 7) 索引网腿 CPU 臂：投影下 DrawMesh ≡ 预投影顶点 + 同索引（逐位）。
func TestP37ProjectionMeshCPULegIndexedMatchesProjection(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	pos := []render.Point{{X: 20, Y: 20}, {X: 140, Y: 20}, {X: 60, Y: 110}, {X: 150, Y: 120}}
	idx := []uint16{0, 1, 2, 0, 2, 3}
	proj := make([]render.Point, len(pos))
	for i, p := range pos {
		proj[i], _ = stanceDevice(0, 0, p.X, p.Y)
	}

	a := stanceNewCtx(320, 220)
	defer func() { _ = a.Close() }()
	a.SetRGBA(0, 0, 0, 1)
	a.SetProjection(projHomography())
	a.DrawMesh(render.Mesh{Positions: pos, Indices: idx})

	b := stanceNewCtx(320, 220)
	defer func() { _ = b.Close() }()
	b.SetRGBA(0, 0, 0, 1)
	b.DrawMesh(render.Mesh{Positions: proj, Indices: idx})

	changed := probeDiff(a.Image(), b.Image())
	if changed != 0 {
		t.Fatalf("索引网腿 CPU 臂 ≡ 预投影网 未逐位一致（差异格=%d）", changed)
	}
	t.Log("索引网腿 CPU 臂与预投影网逐位一致（含 drawDeviceTriangle 三层中性化）")
}

//  8. GPU 纹理口：队列口没有 quad 形参 ⇒ 投影下按「投影四角 AABB」降级 + 具名计数（拍板 D-3），
//     眼后 ⇒ 不画。判据 AABB 由投影公式反推，容差 ±3 像素。
func TestP37ProjectionGPUTextureDegradesToProjectedAABB(t *testing.T) {
	requireNativeGPU(t)
	src := render.NewContext(32, 32)
	defer func() { _ = src.Close() }()
	src.ClearWithColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})
	src.ApplyInvert()
	view, _, _, ok := src.GPUFilterTexture()
	if !ok || view.IsNil() {
		t.Skipf("离屏拿不到滤镜结果视图（GPUFilterTexture ok=%v），本钉需 GPU 发布路", ok)
	}

	dst := stanceNewCtx(320, 220)
	defer func() { _ = dst.Close() }()
	dst.SetProjection(projHomography())
	dst.DrawGPUTexture(view, 20, 30, 60, 40)
	if err := dst.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	st := dst.ProjectionStats()
	if st.GPUTextureAABBs == 0 {
		t.Fatal("GPU 纹理口的 AABB 降级没被计数（D-3 表态缺观测面）")
	}
	q := stanceQuad(0, 0, 20, 30, 60, 40)
	minX, minY, maxX, maxY := q[0].X, q[0].Y, q[0].X, q[0].Y
	for _, p := range q {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	x0, y0, x1, y1 := probeInkBox(dst.Image())
	if y0 < int(minY)-4 || y1 > int(maxY)+4 || x0 < int(minX)-4 || x1 > int(maxX)+4 {
		t.Fatalf("AABB 不是投影四角的外接框：墨迹=(%d..%d,%d..%d)，期望 ⊂ (%.0f..%.0f, %.0f..%.0f)±4",
			x0, x1, y0, y1, minX, maxX, minY, maxY)
	}
	t.Logf("GPU 纹理口 AABB=(%.0f..%.0f,%.0f..%.0f) 墨迹=(%d..%d,%d..%d) 计数=%d",
		minX, maxX, minY, maxY, x0, x1, y0, y1, st.GPUTextureAABBs)

	// 眼后 ⇒ 不画（prepareGPUTextureDraw 拒绝后调用方不得兜底画未投影画面）。
	behind := stanceNewCtx(320, 220)
	defer func() { _ = behind.Close() }()
	behind.SetProjection(projHomography())
	behind.Transform(render.Translate(0, -320))
	behind.DrawGPUTexture(view, 0, 660, 32, 32)
	if err := behind.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	if n := probeNonWhite(behind.Image()); n != 0 {
		t.Fatalf("眼后 GPU 纹理被画出未投影画面：%d 像素", n)
	}
	t.Log("GPU 纹理口：AABB 降级可计数、眼后整块不画")
}

// 视频帧口：DrawVideoFrame 实测**不是** AABB 降级——它转调 DrawImageEx，因此吃到透视 quad 腿
// （比 D-3 更好）。本钉把这件事钉死：落点=解析投影四角，而不是未投影矩形。
// 另两口（DrawVideoSlot/DrawVideoPlanes 的 AABB 降级）需真设备 slot，离屏造不出来，
// 登记为覆盖缺口（批 6 视频窗承载），此处不假装已验。
func TestP37ProjectionVideoFrameFollowsPerspQuad(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	frame, err := render.NewImageBuf(32, 32, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Dispose()
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if err := frame.SetRGBA(x, y, 0, 200, 0, 255); err != nil {
				t.Fatal(err)
			}
		}
	}
	frame.NotifyPixelsChanged()

	dc := stanceNewCtx(320, 220)
	defer func() { _ = dc.Close() }()
	dc.SetProjection(projHomography())
	if !dc.DrawVideoFrame(frame, render.VideoDrawOptions{X: 20, Y: 30, DstWidth: 80, DstHeight: 50}) {
		t.Skipf("DrawVideoFrame 本机返回 false（无直传路），登记缺口")
	}
	q := stanceQuad(0, 0, 20, 30, 80, 50)
	tl, br := q[0], q[2]
	if !stanceInkNear(dc.Image(), int(tl.X)+3, int(tl.Y)+3, 3) {
		t.Fatalf("视频帧没走透视 quad 腿：投影左上角 (%.0f,%.0f) 邻域无墨", tl.X, tl.Y)
	}
	// 未投影形态的右下角 (100,80) 必须在墨迹之外（两形态分开，判据才有鉴别力）。
	if probeInk(dc.Image(), 100, 80) {
		t.Fatal("视频帧在未投影位置也有墨 ⇒ 投影没生效")
	}
	t.Logf("DrawVideoFrame 走透视 quad 腿：投影四角=(%.0f,%.0f)-(%.0f,%.0f)，未投影右下角 (100,80) 无墨",
		tl.X, tl.Y, br.X, br.Y)
}

// 10) 文字七个入口逐个 fail-closed：每个都必须 ① 不画 ② TextSkips 递增 ③ 恒等投影对照能画出（鉴别力）。
func TestP37ProjectionAllTextEntriesFailClosed(t *testing.T) {
	src, err := text.NewFontSourceFromFile("text/testdata/source-sans/OTF/SourceSans3-Regular.otf")
	if err != nil {
		t.Skipf("字体缺失：%v", err)
	}
	face := src.Face(28)

	entries := map[string]func(*render.Context){
		"DrawString":            func(dc *render.Context) { dc.DrawString("Affine", 30, 60) },
		"DrawStringAnchored":    func(dc *render.Context) { dc.DrawStringAnchored("Affine", 60, 60, 0.5, 0.5) },
		"DrawStringWrapped":     func(dc *render.Context) { dc.DrawStringWrapped("Affine grid", 30, 60, 0, 0, 120, 30, render.AlignLeft) },
		"StrokeString":          func(dc *render.Context) { dc.StrokeString("Affine", 30, 60) },
		"StrokeStringAnchored":  func(dc *render.Context) { dc.StrokeStringAnchored("Affine", 60, 60, 0.5, 0.5) },
		"DrawParagraph":         func(dc *render.Context) { dc.DrawParagraph(text.LayoutTextSimple("Affine", face), 30, 60) },
		"DrawParagraphStyled":   func(dc *render.Context) { dc.DrawParagraphStyled(stanceParagraph(t, face), 30, 60) },
		"DrawShapedGlyphs":      func(dc *render.Context) { dc.DrawShapedGlyphs(text.Shape("Affine", face), face, 30, 60) },
		"DrawShapedColorGlyphs": func(dc *render.Context) { dc.DrawShapedColorGlyphs(text.Shape("Affine", face), face, 30, 60) },
	}

	for name, draw := range entries {
		if draw == nil {
			continue
		}
		// 投影下：必须不画 + 计数 +1
		dc := stanceNewCtx(320, 220)
		dc.SetFont(face)
		dc.SetProjection(projHomography())
		before := dc.ProjectionStats().TextSkips
		draw(dc)
		st := dc.ProjectionStats()
		n := probeNonWhite(dc.Image())
		_ = dc.Close()
		if n != 0 {
			t.Errorf("%s 在投影层下画出了东西（%d 像素）：fail-closed 破了", name, n)
		}
		if st.TextSkips != before+1 {
			t.Errorf("%s 跳过却没计数：TextSkips %d→%d", name, before, st.TextSkips)
		}

		// 恒等对照：同一入口必须画得出东西（否则上面的「不画」没有鉴别力）
		ref := stanceNewCtx(320, 220)
		ref.SetFont(face)
		draw(ref)
		m := probeNonWhite(ref.Image())
		_ = ref.Close()
		if m == 0 {
			// 该入口在测试字体上本就画不出（如无彩色字形时 DrawShapedColorGlyphs）——
			// 如实记录，不把它算成 fail-closed 的证据，也不算假绿。
			t.Logf("%s：投影下不画且计数 +1；恒等对照也是 0 像素（该字体无彩色字形，鉴别力不足，登记）", name)
			continue
		}
		t.Logf("%s：投影下 0 像素 + 计数 +1；恒等对照 %d 像素", name, m)
	}
}

func stanceParagraph(t *testing.T, face text.Face) *render.Paragraph {
	t.Helper()
	b := render.NewParagraphBuilder(face, render.TextStyle{})
	b.AddText("Affine")
	p := b.Build(text.LayoutOptions{})
	if p == nil {
		t.Fatal("Build 段落返回 nil")
	}
	return p
}

// 描字（StrokeString / StrokeStringAnchored）在投影层下同族 fail-closed：
// 实测同一字形轮廓 Fill 正常（2052 像素）、描边却画空（展平后的绕行与「闭环空腔反向绕行」
// 相互抵消，只留下一次路径映射计数）。与其让内容静默消失，这里显式不画 + 记 TextSkips，
// 「描字接投影映射（保序展平）」登记为后续专项。判据含恒等对照（有鉴别力）。
func TestP37ProjectionStrokeTextFailClosed(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	src, err := text.NewFontSourceFromFile("text/testdata/source-sans/OTF/SourceSans3-Regular.otf")
	if err != nil {
		t.Skipf("字体缺失：%v", err)
	}
	face := src.Face(48)

	for name, draw := range map[string]func(*render.Context){
		"StrokeString":         func(dc *render.Context) { dc.StrokeString("Ag", 30, 100) },
		"StrokeStringAnchored": func(dc *render.Context) { dc.StrokeStringAnchored("Ag", 30, 100, 0.5, 0.5) },
	} {
		dc := stanceNewCtx(320, 220)
		dc.SetFont(face)
		dc.SetRGBA(0, 0, 0, 1)
		dc.SetLineWidth(2)
		dc.SetProjection(projHomography())
		before := dc.ProjectionStats().TextSkips
		draw(dc)
		st := dc.ProjectionStats()
		n := probeNonWhite(dc.Image())
		_ = dc.Close()
		if n != 0 {
			t.Errorf("%s 投影下画出 %d 像素：描字 fail-closed 破了（会画残缺画面）", name, n)
		}
		if st.TextSkips != before+1 {
			t.Errorf("%s 跳过却没记 TextSkips：%d→%d", name, before, st.TextSkips)
		}
		if st.PathExpansions != 0 {
			t.Errorf("%s 被跳过却仍记了路径映射（守卫顺序不对）：%d", name, st.PathExpansions)
		}

		ref := stanceNewCtx(320, 220)
		ref.SetFont(face)
		ref.SetRGBA(0, 0, 0, 1)
		ref.SetLineWidth(2)
		draw(ref)
		m := probeNonWhite(ref.Image())
		_ = ref.Close()
		if m == 0 {
			t.Errorf("%s 恒等对照也没画：判据失去鉴别力", name)
		}
		t.Logf("%s：投影下 0 像素 + TextSkips +1（不记路径映射）；恒等对照 %d 像素", name, m)
	}
}

// 12) DrawShadow：投影生效时抬升/模糊比例按 2×3 层算（登记近似）+ Misc 计数，剪影仍走路径映射绘制。
func TestP37ProjectionShadowCountsMiscAndStillDraws(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := stanceNewCtx(320, 220)
	defer func() { _ = dc.Close() }()
	dc.SetRGBA(0, 0, 0, 1)
	dc.SetProjection(projHomography())
	sil := render.NewPath()
	sil.MoveTo(40, 40)
	sil.LineTo(140, 40)
	sil.LineTo(140, 100)
	sil.Close()
	dc.DrawShadow(sil, render.RGBA{R: 0, G: 0, B: 0, A: 0.5}, 12, false)

	st := dc.ProjectionStats()
	if st.Misc == 0 {
		t.Fatal("DrawShadow 的 2×3 近似表态没被计数（Misc 此前从未被任何断言读过）")
	}
	if probeNonWhite(dc.Image()) == 0 {
		t.Fatal("投影下阴影整笔丢失（剪影应仍经路径映射绘制，只是抬升比例近似）")
	}
	t.Logf("DrawShadow：Misc 计数=%d，画面仍画（近似已登记）", st.Misc)
}
