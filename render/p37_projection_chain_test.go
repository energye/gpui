//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// probeInk 报告该设备格是否被画上非白内容。
func probeInk(img image.Image, x, y int) bool {
	r, g, b, a := img.At(x, y).RGBA()
	return a != 0 && (r != 0xffff || g != 0xffff || b != 0xffff)
}

func probeInkBox(img image.Image) (int, int, int, int) {
	b := img.Bounds()
	x0, y0, x1, y1 := b.Dx(), b.Dy(), -1, -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if probeInk(img, x, y) {
				if x < x0 {
					x0 = x
				}
				if y < y0 {
					y0 = y
				}
				if x > x1 {
					x1 = x
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	return x0, y0, x1, y1
}

func probeBlackPlate(t *testing.T, n int) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(n, n, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if err := img.SetRGBA(x, y, 0, 0, 0, 255); err != nil {
				t.Fatal(err)
			}
		}
	}
	img.NotifyPixelsChanged()
	return img
}

// P3.7 批 1d · 链序与映射次数钉（拍板 B：设备 = deviceMatrix ∘ 投影 ∘ 用户矩阵）。
//
// 这四枚都是先实测红、修完转绿的交付钉，各管一条不容分叉的规则：
//   1) CPU 网腿的顶点只许被投影映射一次（双投影指纹实测差 10992 像素）；
//   2) 眼后图形整块不画（CPU 兜底画出未投影画面=1762 像素，控制路 1024 证明画得出来）；
//   3) 纯仿射提升的投影层必须等价于把同一份仿射乘进 CTM（图像族/路径族同判）；
//   4) 同一几何在图像链与路径链下落点必须一致（链序分叉实测差 3371 像素）。

func probeNonWhite(img image.Image) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, a := img.At(x, y).RGBA()
			if a != 0 && (r != 0xffff || g != 0xffff || bb != 0xffff) {
				n++
			}
		}
	}
	return n
}

func probeDiff(a, b image.Image) int {
	ch := 0
	ba, bb := a.Bounds(), b.Bounds()
	if ba != bb {
		return -1
	}
	for y := ba.Min.Y; y < ba.Max.Y; y++ {
		for x := ba.Min.X; x < ba.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				ch++
			}
		}
	}
	return ch
}

// 钉 1：CPU 网腿在投影层下是否被映射了两次（drawDeviceTriangle 设备三角再进 doFill）。
func TestP37ProjectionMeshCPUMapsOnce(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	world := []render.Point{{X: 10, Y: 10}, {X: 130, Y: 10}, {X: 10, Y: 100}}
	h := projHomography()

	// A：被测路——投影层 + DrawVertices（顶点已投影映射进设备空间）。
	a := render.NewContext(320, 220)
	defer func() { _ = a.Close() }()
	a.ClearWithColor(render.White)
	a.SetRGBA(0, 0, 0, 1)
	a.SetProjection(h)
	a.DrawVertices(world, nil, render.VertexModeTriangles)
	ia := a.Image()

	// B：期望路——手算一次投影角，直接按设备三角填充（不设投影层）。
	b := render.NewContext(320, 220)
	defer func() { _ = b.Close() }()
	b.ClearWithColor(render.White)
	b.SetRGBA(0, 0, 0, 1)
	b.NewSubPath()
	for i, p := range world {
		d := projMapUserPointXY(p.X, p.Y)
		if i == 0 {
			b.MoveTo(d.X, d.Y)
		} else {
			b.LineTo(d.X, d.Y)
		}
	}
	b.ClosePath()
	if err := b.Fill(); err != nil {
		t.Fatal(err)
	}
	ib := b.Image()

	// C：双投影指纹——同一三角被投影映射两次。
	c := render.NewContext(320, 220)
	defer func() { _ = c.Close() }()
	c.ClearWithColor(render.White)
	c.SetRGBA(0, 0, 0, 1)
	c.NewSubPath()
	for i, p := range world {
		d1 := projMapUserPointXY(p.X, p.Y)
		d2 := projMapUserPointXY(d1.X, d1.Y)
		if i == 0 {
			c.MoveTo(d2.X, d2.Y)
		} else {
			c.LineTo(d2.X, d2.Y)
		}
	}
	c.ClosePath()
	if err := c.Fill(); err != nil {
		t.Fatal(err)
	}

	t.Logf("钉1 非白像素：A(被测)=%d B(一次映射期望)=%d C(两次映射指纹)=%d", probeNonWhite(ia), probeNonWhite(ib), probeNonWhite(c.Image()))
	t.Logf("钉1 逐像素差异：A vs B=%d  A vs C=%d", probeDiff(ia, ib), probeDiff(ia, c.Image()))
	if probeDiff(ia, ib) != 0 {
		t.Errorf("CPU 网腿投影映射次数不对（A vs B 差 %d 像素）", probeDiff(ia, ib))
	}
}

// 钉 2：CPU 兜底在眼后 quad 上是拒绝还是照旧画未投影画面。
func TestP37ProjectionBehindEyeRefusedCPU(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img := probeBlackPlate(t, 32)
	defer img.Dispose()

	// 链序=拍板 B（用户矩阵先、投影后）：把矩形放到用户 y=660..692，过 CTM
	// 上移 320 后落在 y=340..372 ⇒ w=1-0.003y ≤ -0.02（眼后）；而「未投影画面」
	// 会落在设备 y≈13..45 的可见区，能区分拒绝与误画。
	a := render.NewContext(320, 220)
	defer func() { _ = a.Close() }()
	a.ClearWithColor(render.White)
	a.SetProjection(projHomography())
	a.Transform(render.Translate(0, -320))
	a.DrawImageEx(img, render.DrawImageOptions{X: 0, Y: 660, DstWidth: 32, DstHeight: 32})
	ia := a.Image()

	b := render.NewContext(320, 220)
	defer func() { _ = b.Close() }()
	b.ClearWithColor(render.White)
	// 对照正例：不设投影时同一矩形本该画得出来（平移量按无投影链取，保证落在可见区）。
	b.Transform(render.Translate(0, -620))
	b.DrawImageEx(img, render.DrawImageOptions{X: 0, Y: 660, DstWidth: 32, DstHeight: 32})

	t.Logf("钉2 非白像素：A(投影+眼后)=%d B(无投影对照)=%d", probeNonWhite(ia), probeNonWhite(b.Image()))
	if probeNonWhite(b.Image()) == 0 {
		t.Fatalf("对照路没画出东西，判据失去鉴别力")
	}
	if probeNonWhite(ia) != 0 {
		t.Errorf("A 应整块拒绝（眼后），实测画出 %d 像素 = 未投影画面", probeNonWhite(ia))
	}
}

// 钉 3：仿射提升的投影层——图像链吃、路径链不吃（同一画布两套语义）。
func TestP37ProjectionAffineLiftEqualsCTM(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	lift := render.Matrix4FromMatrix(render.Scale(2, 2))
	img := probeBlackPlate(t, 16)
	defer img.Dispose()

	// 路径族：A=投影层仿射提升，B=等价 CTM 缩放。
	a := render.NewContext(200, 200)
	defer func() { _ = a.Close() }()
	a.ClearWithColor(render.White)
	a.SetRGBA(0, 0, 0, 1)
	a.SetProjection(lift)
	a.NewSubPath()
	a.MoveTo(10, 10)
	a.LineTo(40, 10)
	a.LineTo(40, 30)
	a.ClosePath()
	if err := a.Fill(); err != nil {
		t.Fatal(err)
	}

	b := render.NewContext(200, 200)
	defer func() { _ = b.Close() }()
	b.ClearWithColor(render.White)
	b.SetRGBA(0, 0, 0, 1)
	b.Transform(render.Scale(2, 2))
	b.NewSubPath()
	b.MoveTo(10, 10)
	b.LineTo(40, 10)
	b.LineTo(40, 30)
	b.ClosePath()
	if err := b.Fill(); err != nil {
		t.Fatal(err)
	}

	// 图像族：C=投影层仿射提升，D=等价 CTM 缩放。
	corners := [4]render.Point{{X: 10, Y: 10}, {X: 40, Y: 10}, {X: 40, Y: 30}, {X: 10, Y: 30}}
	c := render.NewContext(200, 200)
	defer func() { _ = c.Close() }()
	c.ClearWithColor(render.White)
	c.SetProjection(lift)
	if err := c.DrawImageQuad(img, corners, render.QuadDrawOptions{}); err != nil {
		t.Fatal(err)
	}
	d := render.NewContext(200, 200)
	defer func() { _ = d.Close() }()
	d.ClearWithColor(render.White)
	d.Transform(render.Scale(2, 2))
	if err := d.DrawImageQuad(img, corners, render.QuadDrawOptions{}); err != nil {
		t.Fatal(err)
	}

	t.Logf("钉3 非白像素：路径(投影提升)=%d 路径(CTM缩放)=%d 图像(投影提升)=%d 图像(CTM缩放)=%d",
		probeNonWhite(a.Image()), probeNonWhite(b.Image()), probeNonWhite(c.Image()), probeNonWhite(d.Image()))
	t.Logf("钉3 逐像素差异：路径 A vs B=%d  图像 C vs D=%d", probeDiff(a.Image(), b.Image()), probeDiff(c.Image(), d.Image()))
	if probeDiff(a.Image(), b.Image()) != 0 {
		t.Errorf("仿射提升的投影层对路径族不生效（A vs B 差 %d 像素），但对图像族生效 ⇒ 同一画布两套语义", probeDiff(a.Image(), b.Image()))
	}
}

// 钉 4：投影与用户矩阵的作用序——图像链与路径链在同一几何、同一用户矩阵下，
// 按点判据核对落点（不用整幅 diff：图像双线性边缘与实心 AA 边缘本就差几像素）。
func TestP37ProjectionChainOrderUniform(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img := probeBlackPlate(t, 8)
	defer img.Dispose()
	h := projHomography()
	rect := [4]render.Point{{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 30}, {X: 0, Y: 30}}
	dx, dy := 50.0, 0.0

	// 图像链
	a := render.NewContext(320, 220)
	defer func() { _ = a.Close() }()
	a.ClearWithColor(render.White)
	a.SetProjection(h)
	a.Transform(render.Translate(dx, dy))
	if err := a.DrawImageQuad(img, rect, render.QuadDrawOptions{}); err != nil {
		t.Fatal(err)
	}
	// 路径链
	b := render.NewContext(320, 220)
	defer func() { _ = b.Close() }()
	b.ClearWithColor(render.White)
	b.SetRGBA(0, 0, 0, 1)
	b.SetProjection(h)
	b.Transform(render.Translate(dx, dy))
	b.NewSubPath()
	for i, p := range rect {
		if i == 0 {
			b.MoveTo(p.X, p.Y)
		} else {
			b.LineTo(p.X, p.Y)
		}
	}
	b.ClosePath()
	if err := b.Fill(); err != nil {
		t.Fatal(err)
	}

	// 图集链（仿射精灵在同一矩形上）：喂投影口的必须是用户角，非恒等 CTM 下
	// 把设备角再喂进去会把用户矩阵乘两遍（恒等 CTM 看不见这个洞，故单独一臂）。
	c2 := render.NewContext(320, 220)
	defer func() { _ = c2.Close() }()
	c2.ClearWithColor(render.White)
	c2.SetProjection(h)
	c2.Transform(render.Translate(dx, dy))
	if _, err := c2.DrawAtlasEx(img, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
		DstX: 0, DstY: 0, DstW: 40, DstH: 30, Opacity: 1,
	}}, render.AtlasDrawOptions{}); err != nil {
		t.Fatal(err)
	}

	// 判据：矩形内部的格点，按拍板 B 的链序（用户矩阵→投影→设备缩放）手算落点，
	// 两条链都必须把该点画黑。
	misImg, misPath, misAtlas := 0, 0, 0
	var checked int
	for ux := 5.0; ux <= 35.0; ux += 10 {
		for uy := 5.0; uy <= 25.0; uy += 5 {
			bx, by := ux+dx, uy+dy // 用户矩阵（平移）
			w := 1 - 0.003*by
			// 投影 = 单应性 h：先平移 (100,65)，再除 w（与 projMapUserPointXY 同式）
			px, py := (100+bx)/w, (65+by)/w
			if w <= 0 {
				continue
			}
			checked++
			if !probeInk(a.Image(), int(px), int(py)) {
				misImg++
			}
			if !probeInk(b.Image(), int(px), int(py)) {
				misPath++
			}
			if !probeInk(c2.Image(), int(px), int(py)) {
				misAtlas++
			}
		}
	}
	t.Logf("钉4 非白像素：图像链=%d 路径链=%d", probeNonWhite(a.Image()), probeNonWhite(b.Image()))
	ia0, ia1, ia2, ia3 := probeInkBox(a.Image())
	ip0, ip1, ip2, ip3 := probeInkBox(b.Image())
	t.Logf("钉4 墨迹包围盒：图像链=(x %d..%d, y %d..%d) 路径链=(x %d..%d, y %d..%d)", ia0, ia2, ia1, ia3, ip0, ip2, ip1, ip3)
	t.Logf("钉4 按点判据：查 %d 格，落点错=图像 %d / 路径 %d / 图集 %d", checked, misImg, misPath, misAtlas)
	if misImg != 0 || misPath != 0 || misAtlas != 0 {
		t.Errorf("三条链在同一链序下落点不一致（图像 %d / 路径 %d / 图集 %d）", misImg, misPath, misAtlas)
	}
}

// 钉 5：具名计数器的对外读口（ProjectionStats/ResetProjectionStats）与采样核
// 降级计数——四处 bicubic→bilinear 必须逐处可观测（quad / 图集 GPU / 图集 CPU /
// DrawImageEx CPU 兜底），恒等投影下一律为 0。
func TestP37ProjectionStatsCensus(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	h := projHomography()
	corners := [4]render.Point{{X: 20, Y: 20}, {X: 120, Y: 20}, {X: 120, Y: 100}, {X: 20, Y: 100}}

	// 恒等护栏：不设投影 ⇒ 全零。
	base := render.NewContext(320, 220)
	defer func() { _ = base.Close() }()
	base.ClearWithColor(render.White)
	if err := base.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: render.InterpBicubic}); err != nil {
		t.Fatal(err)
	}
	if got := base.ProjectionStats(); got != (render.ProjectionStats{}) {
		t.Fatalf("恒等投影下计数应为零：%+v", got)
	}

	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetProjection(h)

	// 1) quad 口 bicubic ⇒ 降 bilinear + 计数。
	if err := dc.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: render.InterpBicubic}); err != nil {
		t.Fatal(err)
	}
	afterQuad := dc.ProjectionStats().InterpDowngrades
	if afterQuad != 1 {
		t.Fatalf("quad 口采样核降级未计数：%d want 1", afterQuad)
	}

	// 2) 图集 GPU 腿（仿射精灵 + bicubic）⇒ 升格透视槽同时计数。
	if _, err := dc.DrawAtlasEx(img, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32,
		DstX: 140, DstY: 20, DstW: 100, DstH: 80, Opacity: 1,
		Filter: render.InterpBicubic,
	}}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("DrawAtlasEx GPU: %v", err)
	}
	afterAtlasGPU := dc.ProjectionStats().InterpDowngrades
	if afterAtlasGPU != afterQuad+1 {
		t.Fatalf("图集 GPU 腿采样核降级未计数：%d want %d", afterAtlasGPU, afterQuad+1)
	}

	// 3) 图集 CPU 腿 + 4) DrawImageEx CPU 兜底。
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	if _, err := dc.DrawAtlasEx(img, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32,
		DstX: 20, DstY: 130, DstW: 100, DstH: 60, Opacity: 1,
		Filter: render.InterpBicubic,
	}}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("DrawAtlasEx CPU: %v", err)
	}
	afterAtlasCPU := dc.ProjectionStats().InterpDowngrades
	if afterAtlasCPU != afterAtlasGPU+1 {
		t.Fatalf("图集 CPU 腿采样核降级未计数：%d want %d", afterAtlasCPU, afterAtlasGPU+1)
	}
	dc.DrawImageEx(img, render.DrawImageOptions{X: 140, Y: 130, DstWidth: 80, DstHeight: 60,
		Interpolation: render.InterpBicubic})
	afterImageCPU := dc.ProjectionStats().InterpDowngrades
	if afterImageCPU != afterAtlasCPU+1 {
		t.Fatalf("DrawImageEx CPU 兜底采样核降级未计数：%d want %d", afterImageCPU, afterAtlasCPU+1)
	}

	// 其它具名表态同口可见（文字跳过 + 路径映射 + 裁剪降级）。
	dc.DrawString(" census", 20, 210)
	st := dc.ProjectionStats()
	if st.TextSkips == 0 || st.PathExpansions == 0 || st.ClipDegrades == 0 {
		dc.ClipRect(0, 0, 40, 40)
		dc.NewSubPath()
		dc.MoveTo(10, 200)
		dc.LineTo(60, 200)
		dc.LineTo(60, 218)
		dc.ClosePath()
		_ = dc.Fill()
		st = dc.ProjectionStats()
	}
	if st.TextSkips == 0 {
		t.Fatal("文字跳过不可见（E 组表态缺对外观测面）")
	}
	if st.PathExpansions == 0 {
		t.Fatal("路径映射不可见（D 组表态缺对外观测面）")
	}
	if st.ClipDegrades == 0 {
		t.Fatal("裁剪降级不可见（C 组表态缺对外观测面）")
	}

	// 复位口：交回旧值并清零。
	prev := dc.ResetProjectionStats()
	if prev.InterpDowngrades == 0 || prev.TextSkips == 0 {
		t.Fatalf("ResetProjectionStats 没交回旧值：%+v", prev)
	}
	if got := dc.ProjectionStats(); got != (render.ProjectionStats{}) {
		t.Fatalf("复位后仍非零：%+v", got)
	}
	t.Logf("P37 对外计数读口在位：降级 %d 次 / 跳过 %d 次 / 映射 %d 次（复位前快照 %+v）",
		prev.InterpDowngrades, prev.TextSkips, prev.PathExpansions, prev)
}
