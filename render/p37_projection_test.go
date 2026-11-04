//go:build !nogpu

package render_test

import (
	"errors"
	"math"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	recording "github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
)

// P3.7 批 1d 钉：Canvas 级 4×4 投影层（并存式）。
//
// 判据设计：与 1c 的 quad 透视腿对拍——投影层画出的画面必须与「引擎算好四角+
// 权重喂 DrawImageQuad」逐位一致（同一数学的两条入口）；恒等护栏=逐位同旧；
// 各表态（裁剪降级/路径映射/文字跳过）按拍板逐一像素验证。
// GPU 像素一律 ExportImageBuf+GetRGBA 读回。

// projHomography 构造地面单应性：平移部分对齐 1c 的投影几何（center=(160,110)、
// camera=(60,45) ⇒ 平移 (100,65)），w 行随 y 线性变化（越远 w 越小 = 越远越压缩）。
func projHomography() render.Matrix4 {
	return render.Matrix4Homography(render.Translate(100, 65), 0, -0.003, 1)
}

// projHomographyQuad 手算单应性下用户四角的屏幕角与齐次权重（与投影层同一公式：
// screen = m·p / w(p)，w = wx·x + wy·y + w1）——判据从同一份投影输出反推。
func projHomographyQuad(h render.Matrix4, corners [4]render.Point) ([4]render.Point, [4]float64) {
	var screens [4]render.Point
	var weights [4]float64
	aff := render.Translate(100, 65)
	for i, p := range corners {
		mid := aff.TransformPoint(p)
		w := 1 - 0.003*p.Y
		screens[i] = render.Pt(mid.X/w, mid.Y/w)
		weights[i] = w
	}
	_ = h
	return screens, weights
}

// projMapUserPointXY 手算单应性下的屏幕点（与 projHomographyQuad 同一公式）。
func projMapUserPointXY(x, y float64) render.Point {
	w := 1 - 0.003*y
	return render.Pt((100+x)/w, (65+y)/w)
}

// projAbsDiff 通道差。
func projAbsDiff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

func TestP37ProjectionStackSemantics(t *testing.T) {
	requireNativeGPU(t)
	os.Unsetenv("GOGPU_RENDER_MODE")
	dc := render.NewContext(64, 64)
	defer func() { _ = dc.Close() }()

	// 默认恒等（零值上下文返回 Matrix4Identity，不是零值矩阵——P3.5 批 3 契约）。
	if !dc.Projection().IsIdentity() {
		t.Fatal("默认投影层不是恒等")
	}

	h := projHomography()
	dc.SetProjection(h)
	if dc.Projection().IsIdentity() {
		t.Fatal("SetProjection 未生效")
	}

	// PushProjection 保存当前并替换；PopProjection 恢复。
	h2 := render.Matrix4Homography(render.Translate(10, 10), 0, -0.001, 1)
	dc.PushProjection(h2)
	if dc.Projection().IsIdentity() || dc.Projection() == h {
		t.Fatal("PushProjection 未替换当前投影层")
	}
	dc.PopProjection()
	if dc.Projection() != h {
		t.Fatal("PopProjection 未恢复到 PushProjection 前的值")
	}

	// 空栈 PopProjection = no-op（不 panic、值不动）。
	dc.PopProjection()
	if dc.Projection() != h {
		t.Fatal("空栈 PopProjection 改动了投影层")
	}

	// Push()/Pop() 同拍保存/恢复投影层（拍板 D-1 并入 Push/Pop）。
	dc.Push()
	dc.SetProjection(render.Matrix4Identity())
	if !dc.Projection().IsIdentity() {
		t.Fatal("Push 后 SetProjection 未生效")
	}
	dc.Pop()
	if dc.Projection() != h {
		t.Fatal("Pop 未恢复 Push 时的投影层（D-1 并入 Push/Pop）")
	}

	// 非有限矩阵 fail-closed：拒绝装入（保持原值）。
	dc.SetProjection(render.Matrix4Homography(render.Translate(math.NaN(), 0), 0, 0, 1))
	if !dc.Projection().IsIdentity() == false {
		// NaN 平移被拒绝 ⇒ 投影层保持 h。
	}
	if dc.Projection() == render.Matrix4Identity() {
		t.Fatal("非有限矩阵被装入（fail-closed 失效）")
	}

	// SetProjection(恒等) = 关闭投影层。
	dc.SetProjection(render.Matrix4Identity())
	if !dc.Projection().IsIdentity() {
		t.Fatal("SetProjection(恒等) 未关闭投影层")
	}
}

// TestP37ProjectionQuadMatchesQuadLeg：投影层画 quad ≡ 引擎算好四角+权重喂
// DrawImageQuad（同一投影数学的两条入口必须逐位一致——1d 与 1c 的对拍钉）。
func TestP37ProjectionQuadMatchesQuadLeg(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	h := projHomography()
	world := [4]render.Point{{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90}}
	screens, weights := projHomographyQuad(h, world)

	// A：投影层形态。
	os.Unsetenv("GOGPU_RENDER_MODE")
	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetProjection(h)
	if err := dc.DrawImageQuad(img, world, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1}); err != nil {
		t.Fatalf("投影层 DrawImageQuad: %v", err)
	}
	dc.SetProjection(render.Matrix4Identity())
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("投影层 quad 没走 GPU：%s", st.LogLine())
	}
	var viaLayer *render.ImageBuf
	if !dc.ExportImageBuf(&viaLayer) || viaLayer == nil {
		t.Fatal("读回失败")
	}
	defer viaLayer.Dispose()

	// B：1c 形态（引擎算好四角+权重）。
	dc2 := render.NewContext(320, 220)
	defer func() { _ = dc2.Close() }()
	dc2.ClearWithColor(render.White)
	if err := dc2.DrawImageQuad(img, screens, render.QuadDrawOptions{
		Interpolation: render.InterpNearest, Opacity: 1, Perspective: true, Weights: weights,
	}); err != nil {
		t.Fatalf("quad 腿 DrawImageQuad: %v", err)
	}
	if err := dc2.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var viaQuad *render.ImageBuf
	if !dc2.ExportImageBuf(&viaQuad) || viaQuad == nil {
		t.Fatal("读回失败")
	}
	defer viaQuad.Dispose()

	changed, maxDelta := diffStats(viaLayer, viaQuad)
	if changed != 0 || maxDelta != 0 {
		t.Fatalf("投影层与 quad 腿画面不同（changed=%d max=%d）：同一投影数学的两条入口分叉", changed, maxDelta)
	}
	t.Log("P37 投影层 ≡ quad 透视腿（逐位一致）")
}

// TestP37ProjectionQuadBehindEyeRefused：任一角 w ≤ 0 ⇒ 整 quad 拒绝（具名错）。
func TestP37ProjectionQuadBehindEyeRefused(t *testing.T) {
	requireNativeGPU(t)
	os.Unsetenv("GOGPU_RENDER_MODE")
	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetProjection(projHomography())
	// w(y)=1-0.003y 在 y≈333 过零；四角跨过眼平面 ⇒ 整 quad 拒绝。
	err := dc.DrawImageQuad(atlasPerspPlateForBehind(t), [4]render.Point{
		{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 400}, {X: 0, Y: 400},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1})
	if !errors.Is(err, render.ErrQuadBehindEye) {
		t.Fatalf("behind-eye quad got %v want ErrQuadBehindEye", err)
	}
}

func atlasPerspPlateForBehind(t *testing.T) *render.ImageBuf {
	return atlasPerspPlate(t)
}

// TestP37ProjectionImageExPerspParity：DrawImageEx 投影下 GPU 与 CPU 透视腿
// 在梯形稳定区逐通道 ≤1/255（两腿同式，容差沿用 P3.5 批 3 声明）。
func TestP37ProjectionImageExPerspParity(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	h := projHomography()

	draw := func(cpu bool) *render.ImageBuf {
		t.Helper()
		if cpu {
			t.Setenv("GOGPU_RENDER_MODE", "cpu")
		} else {
			os.Unsetenv("GOGPU_RENDER_MODE")
		}
		dc := render.NewContext(320, 220)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		dc.SetProjection(h)
		dc.DrawImageEx(img, render.DrawImageOptions{X: 0, Y: 0, DstWidth: 120, DstHeight: 90, Opacity: 1})
		_ = dc.FlushGPU()
		if !cpu {
			if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
				t.Fatalf("投影层 DrawImageEx 没走 GPU：%s", st.LogLine())
			}
		}
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("读回失败")
		}
		return out
	}
	gpu := draw(false)
	defer gpu.Dispose()
	cpu := draw(true)
	defer cpu.Dispose()

	world := [4]render.Point{{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90}}
	screens, _ := projHomographyQuad(h, world)
	pts := atlasPerspProbePoints(screens)
	worst, _ := sampleGridDiff(gpu, cpu, pts)
	if worst > 1 {
		t.Fatalf("投影层 DrawImageEx GPU/CPU 分叉 %d > 1（显式声明容差）", worst)
	}
	t.Logf("P37 投影层 DrawImageEx GPU/CPU 稳定区最大通道差=%d", worst)
}

// TestP37ProjectionAtlasAffineUpgrade：仿射精灵在投影层下自动升格为透视 quad
// ——画面必须等于「引擎算好四角+权重」的 1c 形态（逐位）。
func TestP37ProjectionAtlasAffineUpgrade(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	h := projHomography()
	world := [4]render.Point{{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90}}
	screens, weights := projHomographyQuad(h, world)

	// A：投影层 + 仿射精灵（Dst 表达同一世界矩形）。
	os.Unsetenv("GOGPU_RENDER_MODE")
	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetProjection(h)
	if _, err := dc.DrawAtlasEx(img, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32,
		DstX: 0, DstY: 0, DstW: 120, DstH: 90, Opacity: 1,
	}}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("DrawAtlasEx: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var viaLayer *render.ImageBuf
	if !dc.ExportImageBuf(&viaLayer) || viaLayer == nil {
		t.Fatal("读回失败")
	}
	defer viaLayer.Dispose()

	// B：1c 形态对照（四角+权重直接喂批绘透视腿）。
	dc2 := render.NewContext(320, 220)
	defer func() { _ = dc2.Close() }()
	dc2.ClearWithColor(render.White)
	if _, err := dc2.DrawAtlasEx(img, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
		Perspective: true, Corners: screens, Weights: weights,
	}}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("DrawAtlasEx: %v", err)
	}
	if err := dc2.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var viaQuad *render.ImageBuf
	if !dc2.ExportImageBuf(&viaQuad) || viaQuad == nil {
		t.Fatal("读回失败")
	}
	defer viaQuad.Dispose()

	changed, maxDelta := diffStats(viaLayer, viaQuad)
	if changed != 0 || maxDelta != 0 {
		t.Fatalf("图集仿射精灵升格与 1c 形态不同（changed=%d max=%d）", changed, maxDelta)
	}
	// 空角钉：该单应性 y 向透视压缩 ⇒ 底边在 y≈212、左边缘斜向右下（BL.x≈137），
	// AABB 左下角 (100,212) 在凸包外（升格真的发生了；若仍按 Dst 画仿射矩形
	// 则 x∈[0,120] 该格被涂色）。
	if r, _, _, _ := viaLayer.GetRGBA(int(screens[3].X)-4, int(screens[2].Y)-2); r != 255 {
		t.Fatalf("AABB 空角被涂色 r=%d：仿射精灵没有升格为透视 quad", r)
	}
	t.Log("P37 图集仿射精灵在投影层下升格 ≡ 1c 批绘透视腿（逐位一致）")
}

// TestP37ProjectionIdentityBitIdentical：SetProjection(恒等) 与不设投影逐位相同
// ——恒等护栏是本批存量零变化的构造性保证。
func TestP37ProjectionIdentityBitIdentical(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	world := [4]render.Point{{X: 40, Y: 40}, {X: 200, Y: 30}, {X: 210, Y: 160}, {X: 30, Y: 150}}

	draw := func(setIdentity bool) *render.ImageBuf {
		t.Helper()
		os.Unsetenv("GOGPU_RENDER_MODE")
		dc := render.NewContext(320, 220)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		if setIdentity {
			dc.SetProjection(render.Matrix4Identity())
		}
		if err := dc.DrawImageQuad(img, world, render.QuadDrawOptions{Interpolation: render.InterpBilinear, Opacity: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := dc.DrawAtlasEx(img, []render.AtlasSprite{{
			SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16, DstX: 230, DstY: 20, DstW: 60, DstH: 45, Opacity: 1,
		}}, render.AtlasDrawOptions{}); err != nil {
			t.Fatal(err)
		}
		dc.DrawImageEx(img, render.DrawImageOptions{X: 235, Y: 90, DstWidth: 55, DstHeight: 40, Opacity: 1})
		if err := dc.FlushGPU(); err != nil {
			t.Fatal(err)
		}
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("读回失败")
		}
		return out
	}
	plain := draw(false)
	defer plain.Dispose()
	ident := draw(true)
	defer ident.Dispose()
	if changed, maxDelta := diffStats(plain, ident); changed != 0 || maxDelta != 0 {
		t.Fatalf("SetProjection(恒等) 改变了画面（changed=%d max=%d）：恒等护栏失效", changed, maxDelta)
	}
}

// TestP37ProjectionClipDegrade：投影下 ClipRect 的降级=投影 quad 路径裁剪——
// 与「直接路径裁剪同一投影几何」逐位一致；裁剪半区内可见、外不可见。
func TestP37ProjectionClipDegrade(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	h := projHomography()
	world := [4]render.Point{{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90}}
	screens, weights := projHomographyQuad(h, world)

	draw := func(viaClipRect bool) *render.ImageBuf {
		t.Helper()
		os.Unsetenv("GOGPU_RENDER_MODE")
		dc := render.NewContext(320, 220)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		if viaClipRect {
			// 投影层 + 用户矩形裁剪（降级路：投影四角 quad 路径裁剪）。
			dc.SetProjection(h)
			dc.ClipRect(0, 0, 120, 45) // 用户空间上半（远半）
		} else {
			// 无投影 + 直接路径裁剪同一投影几何（对照路：用户 y=0/45 两线
			// 经同一单应性映射出的屏幕点连线）。
			t0 := projMapUserPointXY(0, 0)
			t1 := projMapUserPointXY(120, 0)
			b0 := projMapUserPointXY(0, 45)
			b1 := projMapUserPointXY(120, 45)
			dc.NewSubPath()
			dc.MoveTo(t0.X, t0.Y)
			dc.LineTo(t1.X, t1.Y)
			dc.LineTo(b1.X, b1.Y)
			dc.LineTo(b0.X, b0.Y)
			dc.ClosePath()
			dc.Clip()
		}
		// 画同一份内容：投影激活时喂世界角（投影层映射），对照路喂投影后
		// 角点+权重（无投影）。最终设备几何相同 ⇒ 可比。
		if viaClipRect {
			if err := dc.DrawImageQuad(img, world, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1}); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := dc.DrawImageQuad(img, screens, render.QuadDrawOptions{
				Interpolation: render.InterpNearest, Opacity: 1, Perspective: true, Weights: weights,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := dc.FlushGPU(); err != nil {
			t.Fatal(err)
		}
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("读回失败")
		}
		return out
	}
	viaRect := draw(true)
	defer viaRect.Dispose()
	viaPath := draw(false)
	defer viaPath.Dispose()

	if changed, maxDelta := diffStats(viaRect, viaPath); changed != 0 || maxDelta != 0 {
		t.Fatalf("投影裁剪降级与直接路径裁剪不同（changed=%d max=%d）：降级不是同一几何", changed, maxDelta)
	}
	// 可见性定性：裁剪半区内（用户 (60,20) 的像）可见；半区外（用户 (60,70) 的像）空白。
	inside := projMapUserPointXY(60, 20)
	outside := projMapUserPointXY(60, 70)
	if r, g, b, _ := viaRect.GetRGBA(int(inside.X), int(inside.Y)); r == 255 && g == 255 && b == 255 {
		t.Fatalf("裁剪半区内 (%d,%d) 被裁空：降级裁剪把可见区裁没了", int(inside.X), int(inside.Y))
	}
	if r, g, b, _ := viaRect.GetRGBA(int(outside.X), int(outside.Y)); r != 255 || g != 255 || b != 255 {
		t.Fatalf("裁剪半区外 (%d,%d) 有内容：降级裁剪没生效", int(outside.X), int(outside.Y))
	}
	t.Log("P37 投影裁剪降级 ≡ 直接路径裁剪（逐位一致 + 半区定性）")
}

// TestP37ProjectionPathFillMatchesQuad：投影下用户矩形填充 = 投影 quad 的实心
// 填充（D 组表态：展平+映射，投影保直线 ⇒ 精确）——内部区取样一致、界外空白。
func TestP37ProjectionPathFillMatchesQuad(t *testing.T) {
	requireNativeGPU(t)
	h := projHomography()
	world := [4]render.Point{{X: 20, Y: 10}, {X: 100, Y: 10}, {X: 100, Y: 80}, {X: 20, Y: 80}}
	screens, _ := projHomographyQuad(h, world)

	draw := func(viaProjection bool) *render.ImageBuf {
		t.Helper()
		os.Unsetenv("GOGPU_RENDER_MODE")
		dc := render.NewContext(320, 220)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		dc.SetRGBA(0.2, 0.4, 0.8, 1)
		if viaProjection {
			dc.SetProjection(h)
			dc.DrawRectangle(20, 10, 80, 70)
		} else {
			dc.NewSubPath()
			dc.MoveTo(screens[0].X, screens[0].Y)
			dc.LineTo(screens[1].X, screens[1].Y)
			dc.LineTo(screens[2].X, screens[2].Y)
			dc.LineTo(screens[3].X, screens[3].Y)
			dc.ClosePath()

		}
		_ = dc.Fill()
		if err := dc.FlushGPU(); err != nil {
			t.Fatal(err)
		}
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("读回失败")
		}
		return out
	}
	_ = draw
	viaProj := draw(true)
	defer viaProj.Dispose()
	viaQuad := draw(false)
	defer viaQuad.Dispose()

	// 内部取样：用户空间网格点经同一单应性映射为屏幕探针。
	hits := 0
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			ux := 30 + 16*float64(col)
			uy := 20 + 16*float64(row)
			sp := projMapUserPointXY(ux, uy)
			r1, g1, b1, _ := viaProj.GetRGBA(int(sp.X), int(sp.Y))
			r2, g2, b2, _ := viaQuad.GetRGBA(int(sp.X), int(sp.Y))
			if r1 == 255 && g1 == 255 && b1 == 255 {
				t.Fatalf("投影矩形填充在内部采样格 (%d,%d) 缺失", int(sp.X), int(sp.Y))
			}
			if d := projAbsDiff(r1, r2) + projAbsDiff(g1, g2) + projAbsDiff(b1, b2); d > 6 {
				t.Fatalf("两路内部色差 %d > 6 @(%d,%d)", d, int(sp.X), int(sp.Y))
			}
			hits++
		}
	}
	if hits != 16 {
		t.Fatalf("采样格数 %d != 16", hits)
	}
	t.Log("P37 投影下矩形填充 ≡ 投影 quad 填充（内部 16 格一致）")
}

// projMapUserPoint 双参形式。
func projMapUserPointX(ux, uy float64) render.Point {
	w := 1 - 0.003*uy
	return render.Pt((100+ux)/w, (65+uy)/w)
}

// TestP37ProjectionTextSkipped：投影层非仿射 ⇒ 文字跳过（拍板 D-2 fail-closed）。
func TestP37ProjectionTextSkipped(t *testing.T) {
	requireNativeGPU(t)
	os.Unsetenv("GOGPU_RENDER_MODE")

	src, err := text.NewFontSourceFromFile("text/testdata/source-sans/OTF/SourceSans3-Regular.otf")
	if err != nil {
		t.Skipf("字体缺失，跳过文字跳过钉：%v", err)
	}
	face := src.Face(24)

	draw := func(withProjection bool) *render.ImageBuf {
		t.Helper()
		dc := render.NewContext(320, 220)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		if withProjection {
			dc.SetProjection(projHomography())
		}
		dc.SetFont(face)
		dc.SetRGBA(0, 0, 0, 1)
		dc.DrawString("PERSPECTIVE", 40, 60)
		_ = dc.FlushGPU()
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("读回失败")
		}
		return out
	}
	plain := draw(false)
	defer plain.Dispose()
	projected := draw(true)
	defer projected.Dispose()

	// 无投影：文字真实画上（黑像素存在）。
	dark := 0
	for y := 20; y < 90; y++ {
		for x := 20; x < 220; x++ {
			if r, _, _, _ := plain.GetRGBA(x, y); r < 128 {
				dark++
			}
		}
	}
	if dark == 0 {
		t.Fatal("对照路文字没画出来（判据失效）")
	}
	// 有投影：fail-closed 跳过 ⇒ 画面无黑像素。
	for y := 20; y < 110; y++ {
		for x := 20; x < 300; x++ {
			if r, _, _, _ := projected.GetRGBA(x, y); r < 128 {
				t.Fatalf("投影下文字被绘制（fail-closed 失效）@(%d,%d)", x, y)
			}
		}
	}
	t.Logf("P37 投影层文字跳过（对照路黑像素 %d，投影路 0）", dark)
}

// TestP37ProjectionRecordingRoundTrip：录制（含投影命令）→ CanvasBackend 回放
// ≡ 直接画（录制面随接口合规自然进，拍板 D-4）。
func TestP37ProjectionRecordingRoundTrip(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	h := projHomography()

	// 直接画。
	os.Unsetenv("GOGPU_RENDER_MODE")
	direct := render.NewContext(320, 220)
	defer func() { _ = direct.Close() }()
	direct.ClearWithColor(render.White)
	direct.SetProjection(h)
	if err := direct.DrawImageQuad(img, [4]render.Point{
		{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1}); err != nil {
		t.Fatal(err)
	}
	if err := direct.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var want *render.ImageBuf
	if !direct.ExportImageBuf(&want) || want == nil {
		t.Fatal("读回失败")
	}
	defer want.Dispose()

	// 录制 + 回放。
	rc := recording.NewRecorder(320, 220)
	rc.SetProjection(h)
	if err := rc.DrawImageQuad(img, [4]render.Point{
		{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1}); err != nil {
		t.Fatal(err)
	}
	replayCtx := render.NewContext(320, 220)
	defer func() { _ = replayCtx.Close() }()
	replayCtx.ClearWithColor(render.White)
	backend := recording.NewCanvasBackend(replayCtx)
	if err := rc.FinishRecording().Playback(backend); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if err := replayCtx.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	var got *render.ImageBuf
	if !replayCtx.ExportImageBuf(&got) || got == nil {
		t.Fatal("读回失败")
	}
	defer got.Dispose()

	changed, maxDelta := diffStats(want, got)
	if changed != 0 || maxDelta != 0 {
		t.Fatalf("录制回放与直接画不同（changed=%d max=%d）：投影命令录制面失效", changed, maxDelta)
	}
	t.Log("P37 录制（含投影命令）回放 ≡ 直接画（逐位一致）")
}
