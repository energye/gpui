//go:build !nogpu

package scene

// P3.7 批 3e-1（包内测）：CPU 瓦片腿对投影的显式表态。
//
// Renderer.Render 一有加速器就先走 GPU 腿，所以「CPU 腿遇真投影拒画 + 计数」这条
// 判据只能直接驱动瓦片执行器（照 paint_correctness_test.go 的先例）。

import (
	"bytes"
	"testing"

	"github.com/energye/gpui/render"
)

// cpuTileScene 装一份投影再录一块红矩形（与外部钉同一形态：投影只管它之后的内容）。
func cpuTileScene(m render.Matrix4) *Scene {
	s := NewScene()
	s.SetProjection(m)
	s.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{R: 1, A: 1}),
		NewRectShape(4, 4, 24, 16))
	return s
}

// renderAllTiles 用 CPU 瓦片腿把编码画进整张网格，返回是否有任一像素非零。
func renderAllTiles(t *testing.T, r *Renderer, s *Scene) (painted bool) {
	t.Helper()
	enc := s.Encoding()
	images := s.Images()
	for ty := 0; ty*64 <= r.height; ty++ {
		for tx := 0; tx*64 <= r.width; tx++ {
			tile := r.tileGrid.TileAt(tx, ty)
			if tile == nil || tile.Width <= 0 || tile.Height <= 0 {
				continue
			}
			r.renderTile(tile, enc, nil, images)
			for _, b := range tile.Data[:tile.Width*tile.Height*4] {
				if b != 0 {
					painted = true
				}
			}
		}
	}
	return painted
}

func projectiveProbe() render.Matrix4 {
	return render.Matrix4FromElements([16]float64{
		1, 0, 0.125, 0,
		0, 1, 0.25, 0,
		0, 0, 1, 0,
		0, 0.03125, 0, 1,
	})
}

// 仿射可表达的投影：折进烘焙链，必须画得出来，且不计数。
func TestCPUTileAffineLiftProjectionDraws(t *testing.T) {
	r := NewRenderer(64, 64, WithWorkers(1))
	lift := render.Matrix4FromMatrix(render.Matrix{A: 1, B: 0, C: 8, D: 0, E: 1, F: 6})
	if !renderAllTiles(t, r, cpuTileScene(lift)) {
		t.Fatal("仿射提升的投影下 CPU 腿什么都没画")
	}
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("仿射可表达不该计数：%d", got)
	}
}

// 真投影：CPU 腿没有透视采样 ⇒ 整笔拒画 + 具名计数（绝不静默压扁成仿射）。
func TestCPUTileProjectiveProjectionRefusesAndCounts(t *testing.T) {
	r := NewRenderer(64, 64, WithWorkers(1))
	if renderAllTiles(t, r, cpuTileScene(projectiveProbe())) {
		t.Fatal("真投影下 CPU 腿画出了像素：要么静默压扁，要么拒画没生效")
	}
	if got := r.ProjectionDegrades(); got == 0 {
		t.Fatal("拒画没计数：表态丢了（AGENTS 禁止静默降级）")
	} else {
		t.Logf("projective refusals = %d", got)
	}
	r.ResetProjectionDegrades()
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("ResetProjectionDegrades 没清零：%d", got)
	}
	// 恒等投影：既不拒画也不计数（存量路径逐位不变）。
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("恒等不该计数：%d", got)
	}
	if !renderAllTiles(t, r, cpuTileScene(render.Matrix4Identity())) {
		t.Fatal("恒等投影下 CPU 腿应当照常画出内容")
	}
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("恒等投影被计数了：%d", got)
	}
}

// cpuProjectionState 是本文件两条判据的纯函数面：三态划分不许含糊。
func TestCPUTileProjectionStateThreeCases(t *testing.T) {
	if aff, projective := cpuProjectionState(render.Matrix4{}); projective || !aff.IsIdentity() {
		t.Fatalf("零值应按恒等：%+v projective=%v", aff, projective)
	}
	if aff, projective := cpuProjectionState(render.Matrix4Identity()); projective || !aff.IsIdentity() {
		t.Fatalf("恒等不该 projective：%+v projective=%v", aff, projective)
	}
	lift := render.Matrix4FromMatrix(render.Matrix{A: 2, B: 0, C: 3, D: 0, E: 2, F: 5})
	aff, projective := cpuProjectionState(lift)
	if projective || aff.A != 2 || aff.C != 3 || aff.F != 5 {
		t.Fatalf("仿射提升丢了：%+v projective=%v", aff, projective)
	}
	if _, projective := cpuProjectionState(projectiveProbe()); !projective {
		t.Fatal("真透视被当成可仿射表达（会静默压扁）")
	}
}

// ---------------------------------------------------------------------------
// 批 3e-2：quad 通道的 CPU 瓦片腿表态
// ---------------------------------------------------------------------------

// cpuQuadSize / cpuQuadImage 是一张 4×4 格的checkerboard图：位置或 UV 一挪颜色就跟着挪。
const cpuQuadSize = 8

func cpuQuadImage() *Image {
	img := NewImage(cpuQuadSize, cpuQuadSize)
	img.Data = make([]byte, cpuQuadSize*cpuQuadSize*4)
	for y := 0; y < cpuQuadSize; y++ {
		for x := 0; x < cpuQuadSize; x++ {
			i := (y*cpuQuadSize + x) * 4
			img.Data[i+3] = 255
			if ((x/4)+(y/4))%2 == 0 {
				img.Data[i+0] = 255
			} else {
				img.Data[i+1] = 255
			}
		}
	}
	return img
}

// cpuCanvasInto 用 CPU 瓦片腿把一份编码逐瓦片铺成整张画布缓冲。
func cpuCanvasInto(t *testing.T, r *Renderer, s *Scene) []byte {
	t.Helper()
	enc := s.Encoding()
	images := s.Images()
	out := make([]byte, r.width*r.height*4)
	for ty := 0; ty*64 <= r.height; ty++ {
		for tx := 0; tx*64 <= r.width; tx++ {
			tile := r.tileGrid.TileAt(tx, ty)
			if tile == nil || tile.Width <= 0 || tile.Height <= 0 {
				continue
			}
			r.renderTile(tile, enc, nil, images)
			x0, y0, tw, th := tile.Bounds()
			for row := 0; row < th; row++ {
				base := ((y0+row)*r.width + x0) * 4
				src := row * tw * 4
				if x0+tw <= r.width {
					copy(out[base:base+tw*4], tile.Data[src:src+tw*4])
					continue
				}
				for col := 0; col < tw && x0+col < r.width; col++ {
					copy(out[base+col*4:base+col*4+4], tile.Data[src+col*4:src+col*4+4])
				}
			}
		}
	}
	return out
}

func cpuCanvasPainted(buf []byte) int {
	n := 0
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] != 0 || buf[i+1] != 0 || buf[i+2] != 0 || buf[i+3] != 0 {
			n++
		}
	}
	return n
}

// 平行四边形 quad ≡ 同一张图走等价仿射 DrawImage：CPU 腿能精确表达的场合不许走样。
func TestCPUTileQuadParallelogramEqualsAffineDraw(t *testing.T) {
	img := cpuQuadImage()
	// 8×8 图 → TL(10,8)、TR(34,8)、BL(4,32)：u=(TR−TL)/8=(3,0)，v=(BL−TL)/8=(−0.75,3)。
	corners := [4]render.Point{
		{X: 10, Y: 8}, {X: 34, Y: 8}, {X: 28, Y: 32}, {X: 4, Y: 32},
	}
	aff := NewAffine(3, -0.75, 10, 0, 3, 8)
	if got, ok := quadAffine(corners, cpuQuadSize, cpuQuadSize); !ok || got != aff {
		t.Fatalf("quadAffine 折出来的仿射不对：%+v ok=%v want %+v", got, ok, aff)
	}

	quadScene := NewScene()
	quadScene.DrawImageQuad(img, corners, render.QuadDrawOptions{Opacity: 1})
	affScene := NewScene()
	affScene.DrawImage(img, aff)

	rq := NewRenderer(64, 64, WithWorkers(1))
	ra := NewRenderer(64, 64, WithWorkers(1))
	qBuf, aBuf := cpuCanvasInto(t, rq, quadScene), cpuCanvasInto(t, ra, affScene)
	if cpuCanvasPainted(qBuf) == 0 {
		t.Fatal("平行四边形 quad 在 CPU 腿上一格没画")
	}
	if !bytes.Equal(qBuf, aBuf) {
		t.Fatalf("quad 折仿射与等价 DrawImage 画得不一样：%d 字节差", cpuCanvasDiff(qBuf, aBuf))
	}
	if got := rq.ProjectionDegrades(); got != 0 {
		t.Fatalf("可仿射表达的 quad 不该计数：%d", got)
	}
}

// 梯形（不是平行四边形）没有仿射描述 ⇒ 整笔拒画并计数，绝不压扁成近似。
func TestCPUTileQuadTrapezoidRefusesAndCounts(t *testing.T) {
	img := cpuQuadImage()
	trapezoid := [4]render.Point{
		{X: 12, Y: 4}, {X: 28, Y: 4}, {X: 36, Y: 44}, {X: 4, Y: 44},
	}
	if _, ok := quadAffine(trapezoid, cpuQuadSize, cpuQuadSize); ok {
		t.Fatal("梯形被判成了平行四边形：仿射折叠的门槛丢了")
	}
	s := NewScene()
	s.DrawImageQuad(img, trapezoid, render.QuadDrawOptions{Opacity: 1})
	r := NewRenderer(64, 64, WithWorkers(1))
	if buf := cpuCanvasInto(t, r, s); cpuCanvasPainted(buf) != 0 {
		t.Fatal("梯形 quad 在 CPU 腿上画出了东西：静默压扁（AGENTS 禁止）")
	}
	if got := r.ProjectionDegrades(); got == 0 {
		t.Fatal("拒画没计数：表态丢了")
	} else {
		t.Logf("trapezoid refusals = %d", got)
	}
}

// 平行四边形 + 打开透视：权重让 UV 不再仿射 ⇒ 同样拒画计数，且画面必须与不开透视不同。
func TestCPUTileQuadPerspectiveFlagRefusesAndCounts(t *testing.T) {
	img := cpuQuadImage()
	corners := [4]render.Point{
		{X: 10, Y: 8}, {X: 34, Y: 8}, {X: 28, Y: 32}, {X: 4, Y: 32},
	}
	plain := NewScene()
	plain.DrawImageQuad(img, corners, render.QuadDrawOptions{Opacity: 1})
	persp := NewScene()
	persp.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Perspective: true, Weights: [4]float64{1, 1, 0.5, 0.5}, Opacity: 1,
	})

	rp := NewRenderer(64, 64, WithWorkers(1))
	if buf := cpuCanvasInto(t, rp, persp); cpuCanvasPainted(buf) != 0 {
		t.Fatal("带权重的 quad 在 CPU 腿上画出了东西：CPU 没有透视采样，这就是压扁")
	}
	if got := rp.ProjectionDegrades(); got == 0 {
		t.Fatal("透视位拒画没计数")
	}
	rn := NewRenderer(64, 64, WithWorkers(1))
	if buf := cpuCanvasInto(t, rn, plain); cpuCanvasPainted(buf) == 0 {
		t.Fatal("对照组（不透视）没画：这条鉴别不成立")
	}
	if got := rn.ProjectionDegrades(); got != 0 {
		t.Fatalf("对照组不该计数：%d", got)
	}
}

// 仿射可表达的投影层必须折进图像/quad 的烘焙链（与 TagTransform 同一口径），
// 不然 GPU 腿按投影画、CPU 腿把投影丢了，两腿分叉。
func TestCPUTileAffineLiftProjectionAppliesToImagesAndQuads(t *testing.T) {
	img := cpuQuadImage()
	// 内容矩阵带缩放，投影带平移：两者不可交换，顺序写反落点就差一倍（拍板 B：
	// device = deviceMatrix ∘ projection ∘ matrix ∘ user，投影在场景矩阵之外）。
	content := NewAffine(2, 0, 1, 0, 2, 3)
	lift := render.Matrix4FromMatrix(render.Matrix{A: 1, B: 0, C: 8, D: 0, E: 1, F: 6})

	projImg := NewScene()
	projImg.SetProjection(lift)
	projImg.DrawImage(img, content)
	// 正确序：p → 缩放2+平移(1,3) → +投影(8,6) ⇒ 2p+(9,9)；反序会落到 2p+(17,17)。
	plainImg := NewScene()
	plainImg.DrawImage(img, NewAffine(2, 0, 9, 0, 2, 9))

	rp, ra := NewRenderer(64, 64, WithWorkers(1)), NewRenderer(64, 64, WithWorkers(1))
	pBuf, aBuf := cpuCanvasInto(t, rp, projImg), cpuCanvasInto(t, ra, plainImg)
	if cpuCanvasPainted(pBuf) == 0 {
		t.Fatal("投影下的图像在 CPU 腿上一格没画")
	}
	if !bytes.Equal(pBuf, aBuf) {
		t.Fatalf("投影层没折进图像烘焙链：与等价手写仿射差 %d 字节", cpuCanvasDiff(pBuf, aBuf))
	}
	if got := rp.ProjectionDegrades(); got != 0 {
		t.Fatalf("仿射可表达不该计数：%d", got)
	}

	corners := [4]render.Point{
		{X: 10, Y: 8}, {X: 34, Y: 8}, {X: 28, Y: 32}, {X: 4, Y: 32},
	}
	projQuad := NewScene()
	projQuad.SetProjection(lift)
	projQuad.DrawImageQuad(img, corners, render.QuadDrawOptions{Opacity: 1})
	plainQuad := NewScene()
	plainQuad.DrawImageQuad(img, shiftForLift(corners, 8, 6), render.QuadDrawOptions{Opacity: 1})

	rpq := NewRenderer(64, 64, WithWorkers(1))
	if buf := cpuCanvasInto(t, rpq, projQuad); cpuCanvasPainted(buf) == 0 {
		t.Fatal("投影下的 quad 在 CPU 腿上一格没画")
	}
	if got := rpq.ProjectionDegrades(); got != 0 {
		t.Fatalf("仿射可表达的 quad 不该计数：%d", got)
	}
	rpl := NewRenderer(64, 64, WithWorkers(1))
	if qBuf, lBuf := cpuCanvasInto(t, rpq, projQuad), cpuCanvasInto(t, rpl, plainQuad); !bytes.Equal(qBuf, lBuf) {
		t.Fatalf("投影层没折进 quad 角点：与等价手写角点差 %d 字节", cpuCanvasDiff(qBuf, lBuf))
	}
}

// 真投影 + quad：拒画并计数（与图像/文字族同纪律）。
func TestCPUTileProjectiveProjectionRefusesQuad(t *testing.T) {
	s := NewScene()
	s.SetProjection(projectiveProbe())
	s.DrawImageQuad(cpuQuadImage(), [4]render.Point{
		{X: 10, Y: 8}, {X: 34, Y: 8}, {X: 28, Y: 32}, {X: 4, Y: 32},
	}, render.QuadDrawOptions{Opacity: 1})
	r := NewRenderer(64, 64, WithWorkers(1))
	if buf := cpuCanvasInto(t, r, s); cpuCanvasPainted(buf) != 0 {
		t.Fatal("真投影下 quad 画出了像素：静默压扁")
	}
	if got := r.ProjectionDegrades(); got == 0 {
		t.Fatal("真投影下的 quad 拒画没计数")
	}
}

func shiftForLift(q [4]render.Point, dx, dy float64) [4]render.Point {
	out := q
	for i := range out {
		out[i].X += dx
		out[i].Y += dy
	}
	return out
}

// 路径几何的链序钉：投影在场景矩阵**之外**（拍板 B：device = projection ∘ matrix ∘ user）。
// 缩放 2 与平移 (8,6) 不可交换，两种先后落在不同位置，写反就看得见。
// 复合有两个现场：TagTransform 落地时重算、TagProjection 落地时重算（后者出现在
// 「变换没变、投影插在两笔之间」的去重场景里），一处一条子钉分别钉死。
func TestCPUTileProjectionAppliesOutsideSceneMatrix(t *testing.T) {
	white := SolidBrush(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	arms := []struct {
		name   string
		body   func(*Scene)
		probe  [][2]int // 正确序必盖到
		forbid [][2]int // 只有反序才会盖到
	}{
		{
			// 先投影后变换：TagTransform 落地时复合。
			name: "tag_transform_site",
			body: func(s *Scene) {
				s.SetProjection(cpuLift())
				s.PushTransform(ScaleAffine(2, 2))
				s.Fill(FillNonZero, IdentityAffine(), white, NewRectShape(0, 0, 4, 4))
				s.PopTransform()
			},
			// 正序 2p+(8,6) ⇒ (8,6)-(16,14)；反序 2p+(16,12) ⇒ (16,12)-(24,20)。
			probe:  [][2]int{{10, 8}, {14, 10}},
			forbid: [][2]int{{20, 16}, {18, 14}},
		},
		{
			// 两笔之间只改投影：第二笔不再重发 TagTransform（去重），
			// 落点完全由 TagProjection 那处的复合决定。
			name: "tag_projection_site",
			body: func(s *Scene) {
				s.PushTransform(ScaleAffine(2, 2))
				s.Fill(FillNonZero, IdentityAffine(), white, NewRectShape(0, 0, 4, 4))
				s.SetProjection(cpuLift())
				s.Fill(FillNonZero, IdentityAffine(), white, NewRectShape(20, 20, 4, 4))
				s.PopTransform()
			},
			// 第二块：正序 2p+(8,6) ⇒ (48,46)-(56,54)；反序 2p+(16,12) ⇒ (56,52)-(64,60)。
			probe:  [][2]int{{50, 48}, {54, 52}},
			forbid: [][2]int{{60, 56}, {58, 58}},
		},
	}
	for _, arm := range arms {
		t.Run(arm.name, func(t *testing.T) {
			s := NewScene()
			arm.body(s)

			const w = 64
			r := NewRenderer(w, w, WithWorkers(1))
			buf := cpuCanvasInto(t, r, s)
			if cpuCanvasPainted(buf) == 0 {
				t.Fatal("投影 + 缩放下 CPU 腿什么都没画")
			}
			for _, p := range arm.probe {
				if !cpuPixelPainted(buf, w, p[0], p[1]) {
					t.Fatalf("正确序应覆盖 (%d,%d)，实为空：投影被折进了矩阵内侧", p[0], p[1])
				}
			}
			for _, p := range arm.forbid {
				if cpuPixelPainted(buf, w, p[0], p[1]) {
					t.Fatalf("反序才会覆盖到 (%d,%d)：链序写反了", p[0], p[1])
				}
			}
			if got := r.ProjectionDegrades(); got != 0 {
				t.Fatalf("仿射可表达不该计数：%d", got)
			}
		})
	}
}

func cpuLift() render.Matrix4 {
	return render.Matrix4FromMatrix(render.Matrix{A: 1, B: 0, C: 8, D: 0, E: 1, F: 6})
}

func cpuPixelPainted(buf []byte, w, x, y int) bool {
	i := (y*w + x) * 4
	return buf[i] != 0 || buf[i+1] != 0 || buf[i+2] != 0 || buf[i+3] != 0
}

func cpuCanvasDiff(a, b []byte) int {
	n := 0
	for i := range a {
		if i >= len(b) {
			return n + 1
		}
		if a[i] != b[i] {
			n++
		}
	}
	return n
}
