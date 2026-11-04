//go:build !nogpu

package render_test

import (
	"errors"
	"math"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// P3.7 批 1c 钉：图集批绘腿吃「四角+权重」——2.5D 精灵按深度投影的标准形态
// （对齐 Godot Sprite3D 的职责划分：引擎算好投影角，render 只收顶点级数据走透视腿）。
//
// 判据设计与批 1b 同风格：parity 防两腿分叉、判别器防「weights 被忽略退仿射」的
// 空转绿、零值钉防存量路被改。GPU 像素一律 ExportImageBuf+GetRGBA 读回。

// atlasPerspPlate 造一张左红右蓝 + 明暗横带的图集底图：u=0.5 处硬边色界，
// 仿射 UV 与透视 UV 在错切区必然跨色界分叉。
func atlasPerspPlate(t *testing.T) *render.ImageBuf {
	t.Helper()
	const w, h = 32, 32
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b uint8
			if x < w/2 {
				r, g, b = 200, 40, 40 // 左半红
			} else {
				r, g, b = 40, 60, 200 // 右半蓝
			}
			if (y/4)%2 == 0 { // 横带明暗：v 方向也有判别力
				r /= 2
				g /= 2
				b /= 2
			}
			if err := img.SetRGBA(x, y, r, g, b, 255); err != nil {
				t.Fatal(err)
			}
		}
	}
	img.NotifyPixelsChanged()
	return img
}

// atlasPerspQuad 用真实投影口（P3.5 批 3 Matrix4）从 world rect + 每角深度
// 算屏幕角与齐次权重——底边深 0（近、大）、顶边深 60（远、小）= 上窄下宽梯形，
// 相机放在世界矩形中心 ⇒ 左右两边对称外扩，AABB 两个上角在凸包外（空角钉用）。
// 「像素探针从画面所用同一份投影输出反推」，不从画面反推结论。
func atlasPerspQuad(focal float64) ([4]render.Point, [4]float64, bool) {
	m, ok := render.ProjectorMatrix4(focal, render.Pt(160, 110), render.Pt(60, 45))
	if !ok {
		return [4]render.Point{}, [4]float64{}, false
	}
	world := [4]render.Point{{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 90}, {X: 0, Y: 90}}
	depths := [4]float64{60, 60, 0, 0} // TL,TR,BR,BL：顶远底近
	return m.ProjectQuad(world, depths)
}

// drawAtlasPersp 用指定精灵画一张图集透视（或对照）帧并显存读回。
// cpu=true 走 GOGPU_RENDER_MODE=cpu 的 CPU 真采样腿。
func drawAtlasPersp(t *testing.T, img *render.ImageBuf, sp render.AtlasSprite, cpu bool) *render.ImageBuf {
	t.Helper()
	if cpu {
		t.Setenv("GOGPU_RENDER_MODE", "cpu")
	} else {
		os.Unsetenv("GOGPU_RENDER_MODE")
	}
	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	res, err := dc.DrawAtlasEx(img, []render.AtlasSprite{sp}, render.AtlasDrawOptions{})
	if err != nil {
		t.Fatalf("DrawAtlasEx: %v", err)
	}
	if res.Skipped || res.Drawn != 1 {
		t.Fatalf("图集未提交：res=%+v", res)
	}
	if !cpu {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("图集透视没走 GPU 或掉回 CPU：%s", st.LogLine())
		}
	} else {
		_ = dc.FlushGPU()
	}
	var out *render.ImageBuf
	if !dc.ExportImageBuf(&out) || out == nil {
		t.Fatal("显存读回失败")
	}
	return out
}

// sampleGridDiff 在指定采样格上取两图最大通道差与差异格数（GPU 读回缓冲对 CPU
// Image 或另一 GPU 缓冲）。透视 quad 的 AA 边缘不在采样格内（格点缩进边内）。
func sampleGridDiff(a *render.ImageBuf, b *render.ImageBuf, pts [][2]int) (worst int, changed int) {
	for _, p := range pts {
		r1, g1, b1, _ := a.GetRGBA(p[0], p[1])
		r2, g2, b2, _ := b.GetRGBA(p[0], p[1])
		for _, d := range [][2]uint8{{r1, r2}, {g1, g2}, {b1, b2}} {
			delta := int(d[0]) - int(d[1])
			if delta < 0 {
				delta = -delta
			}
			if delta > worst {
				worst = delta
			}
			if delta > 0 {
				changed++
			}
		}
	}
	return worst, changed
}

// atlasPerspProbePoints 返回梯形内部稳定区采样格：从投影四角反推——取每条扫描线的
// 梯形横向内缩区间，缩进 2px 避开 AA 边缘；行范围 t=0.08..0.92 避开上下边。
// y 从底(近)向顶(远)走 5 行、每行 5 列。
func atlasPerspProbePoints(corners [4]render.Point) [][2]int {
	var pts [][2]int
	for row := 0; row < 5; row++ {
		t := 0.08 + 0.21*float64(row) // 0.08..0.92：远离上下边
		y := corners[3].Y + (corners[0].Y-corners[3].Y)*t
		// 梯形横向边界随 t 线性内缩（底宽顶窄）。
		left := corners[3].X + (corners[0].X-corners[3].X)*t
		right := corners[2].X + (corners[1].X-corners[2].X)*t
		for col := 0; col < 5; col++ {
			u := 0.12 + 0.19*float64(col)
			x := left + (right-left)*u
			pts = append(pts, [2]int{int(math.Round(x)), int(math.Round(y))})
		}
	}
	return pts
}

// TestP37AtlasPerspCPUGPUParity：图集透视 GPU 与 CPU 真采样在梯形内部稳定区
// 逐通道 ≤1/255（沿用 P3.5 批 3 声明：GPU f32 clip-w vs CPU f64）。
func TestP37AtlasPerspCPUGPUParity(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	corners, weights, ok := atlasPerspQuad(100)
	if !ok {
		t.Fatal("投影失败")
	}
	sp := render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32,
		Opacity:     1,
		Perspective: true,
		Corners:     corners,
		Weights:     weights,
	}
	gpu := drawAtlasPersp(t, img, sp, false)
	defer gpu.Dispose()
	cpu := drawAtlasPersp(t, img, sp, true)
	defer cpu.Dispose()
	worst, _ := sampleGridDiff(gpu, cpu, atlasPerspProbePoints(corners))
	if worst > 1 {
		t.Fatalf("图集透视 GPU/CPU 分叉 %d > 1（显式声明容差）", worst)
	}
	t.Logf("P37 图集透视 GPU/CPU 稳定区最大通道差=%d", worst)
}

// TestP37AtlasPerspTrapezoidUV：同梯形同图源，透视批绘与仿射 quad（DrawImageQuad
// 同角点、整图 UV）在错切区必不同色——防「weights 被忽略退仿射」的空转绿；
// 且梯形 AABB 的空角必须仍是底色（四角承载几何、Dst 被忽略）。
func TestP37AtlasPerspTrapezoidUV(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	corners, weights, ok := atlasPerspQuad(100)
	if !ok {
		t.Fatal("投影失败")
	}
	persp := drawAtlasPersp(t, img, render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
		Perspective: true, Corners: corners, Weights: weights,
	}, false)
	defer persp.Dispose()

	// 仿射对照：同角点同整图 UV，但不带权重（仿射线性插值 UV）。
	os.Unsetenv("GOGPU_RENDER_MODE")
	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	if err := dc.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Interpolation: render.InterpBilinear, Opacity: 1,
	}); err != nil {
		t.Fatalf("DrawImageQuad affine: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	var affine *render.ImageBuf
	if !dc.ExportImageBuf(&affine) || affine == nil {
		t.Fatal("显存读回失败")
	}
	defer affine.Dispose()

	worst, changed := sampleGridDiff(persp, affine, atlasPerspProbePoints(corners))
	if changed == 0 {
		t.Fatal("图集透视与仿射 quad 逐格相同：1/w 有理插值没接上（weights 被忽略退仿射的形态）")
	}
	t.Logf("P37 图集透视 vs 仿射：差异格=%d 最大通道差=%d（错切区必分叉）", changed, worst)

	// 空角钉：梯形上窄下宽 ⇒ AABB 左上/右上角在四角凸包外，必须仍是白底。
	// 探针取 AABB 角内侧 1px（x=BL.x+1 与 BR.x-1、y=TL.y+1）——该两格在
	// 上窄下宽的梯形外；若透视分支被绕过（按 Dst 画满 AABB），会被涂色。
	if r, _, _, _ := persp.GetRGBA(int(corners[3].X)+1, int(corners[0].Y)+1); r != 255 {
		t.Fatalf("AABB 左上空角被涂色 r=%d：四角没有承载几何（Dst 被误用）", r)
	}
	if r, _, _, _ := persp.GetRGBA(int(corners[2].X)-1, int(corners[0].Y)+1); r != 255 {
		t.Fatalf("AABB 右上空角被涂色 r=%d：四角没有承载几何（Dst 被误用）", r)
	}
}

// TestP37AtlasPerspTintDarkens：透视腿的 tint 通道（批 1b 形参）在图集精灵上生效：
// 同精灵 tint=0.5 灰必须把取样点压暗。
func TestP37AtlasPerspTintDarkens(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	corners, weights, ok := atlasPerspQuad(100)
	if !ok {
		t.Fatal("投影失败")
	}
	plain := drawAtlasPersp(t, img, render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
		Perspective: true, Corners: corners, Weights: weights,
	}, false)
	defer plain.Dispose()
	dyed := drawAtlasPersp(t, img, render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
		Perspective: true, Corners: corners, Weights: weights,
		Tint: render.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 1},
	}, false)
	defer dyed.Dispose()
	worst, changed := sampleGridDiff(plain, dyed, atlasPerspProbePoints(corners))
	if changed == 0 {
		t.Fatal("透视腿 tint 未生效（图集精灵走透视腿时被写死白）")
	}
	// 方向判定：染色帧的取样通道必须不高于素帧（压暗）。
	for _, p := range atlasPerspProbePoints(corners) {
		r1, g1, b1, _ := plain.GetRGBA(p[0], p[1])
		r2, g2, b2, _ := dyed.GetRGBA(p[0], p[1])
		if r2 > r1+1 || g2 > g1+1 || b2 > b1+1 {
			t.Fatalf("染色帧反而变亮 @%v：plain=%d,%d,%d dyed=%d,%d,%d", p, r1, g1, b1, r2, g2, b2)
		}
	}
	_ = worst
	t.Logf("P37 图集透视 tint 生效：差异格=%d 最大通道差=%d", changed, worst)
}

// TestP37AtlasPerspZeroIsIdentity：Perspective=false 时新字段全部被忽略
// （塞垃圾值与不塞逐位相同）——零值=历史路，存量画面逐位不变。
func TestP37AtlasPerspZeroIsIdentity(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	base := render.AtlasSprite{
		SrcX: 4, SrcY: 8, SrcW: 16, SrcH: 16,
		DstX: 60, DstY: 60, DstW: 90, DstH: 70, Opacity: 1,
	}
	clean := drawAtlasPersp(t, img, base, false)
	defer clean.Dispose()
	// 垃圾值全塞：Perspective=false 时 Corners/Weights 不参与校验与绘制。
	dirty := base
	dirty.Corners = [4]render.Point{{X: math.NaN(), Y: math.NaN()}, {}, {}, {}}
	dirty.Weights = [4]float64{-1, 0, math.Inf(1), 3}
	garbage := drawAtlasPersp(t, img, dirty, false)
	defer garbage.Dispose()
	if worst, changed := sampleGridDiff(clean, garbage, perspGridPoints(60, 60, 90, 70)); changed != 0 || worst != 0 {
		t.Fatalf("Perspective=false 时新字段影响了画面（changed=%d worst=%d）：零值不是老路", changed, worst)
	}
}

// perspGridPoints 在矩形内部取采样格（零值恒等判定用）。
func perspGridPoints(x, y, w, h float64) [][2]int {
	var pts [][2]int
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			px := x + 4 + (w-8)*float64(col)/3
			py := y + 4 + (h-8)*float64(row)/3
			pts = append(pts, [2]int{int(math.Round(px)), int(math.Round(py))})
		}
	}
	return pts
}

// TestP37AtlasPerspSentinels：透视图集精灵的校验哨兵逐个点名，错误后不画。
func TestP37AtlasPerspSentinels(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	corners, weights, ok := atlasPerspQuad(100)
	if !ok {
		t.Fatal("投影失败")
	}
	base := render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
		Perspective: true, Corners: corners, Weights: weights,
	}

	mk := func(mutate func(*render.AtlasSprite)) render.AtlasSprite {
		sp := base
		mutate(&sp)
		return sp
	}
	cases := []struct {
		name string
		sp   render.AtlasSprite
		want error
	}{
		{"非有限角", mk(func(sp *render.AtlasSprite) { sp.Corners[2] = render.Pt(math.NaN(), 5) }), render.ErrAtlasNonFinite},
		{"零面积", mk(func(sp *render.AtlasSprite) {
			sp.Corners = [4]render.Point{{X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}}
		}), render.ErrAtlasQuadDegenerate},
		{"自交", mk(func(sp *render.AtlasSprite) {
			sp.Corners = [4]render.Point{{X: 10, Y: 10}, {X: 50, Y: 50}, {X: 50, Y: 10}, {X: 10, Y: 50}}
		}), render.ErrAtlasQuadBowTie},
		{"权重 0", mk(func(sp *render.AtlasSprite) { sp.Weights[1] = 0 }), render.ErrAtlasQuadBadWeights},
		{"权重负", mk(func(sp *render.AtlasSprite) { sp.Weights[2] = -3 }), render.ErrAtlasQuadBadWeights},
		{"权重 NaN", mk(func(sp *render.AtlasSprite) { sp.Weights[0] = math.NaN() }), render.ErrAtlasQuadBadWeights},
		{"透视+bicubic", mk(func(sp *render.AtlasSprite) { sp.Filter = render.InterpBicubic }), render.ErrQuadPerspectiveBicubic},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv("GOGPU_RENDER_MODE")
			dc := render.NewContext(320, 220)
			defer func() { _ = dc.Close() }()
			dc.ClearWithColor(render.White)
			_, err := dc.DrawAtlasEx(img, []render.AtlasSprite{tc.sp}, render.AtlasDrawOptions{})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			// 错误后不画：整帧仍是白底。
			r, g, b, a := mustExport(t, dc).GetRGBA(160, 110)
			if r != 255 || g != 255 || b != 255 || a != 255 {
				t.Fatalf("校验失败后画面被污染：(%d,%d,%d,%d)", r, g, b, a)
			}
		})
	}

	// 深度口早拒绝：NewDepthSprite 对透视坏权重报同族哨兵。
	bad := base
	bad.Weights[1] = 0
	if _, err := render.NewDepthSprite("bad", bad, 5); !errors.Is(err, render.ErrAtlasQuadBadWeights) {
		t.Fatalf("NewDepthSprite 坏权重 got %v want ErrAtlasQuadBadWeights", err)
	}
}

func mustExport(t *testing.T, dc *render.Context) *render.ImageBuf {
	t.Helper()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	var out *render.ImageBuf
	if !dc.ExportImageBuf(&out) || out == nil {
		t.Fatal("显存读回失败")
	}
	t.Cleanup(func() { out.Dispose() })
	return out
}

// TestP37DepthSpritesCarryPersp：深度精灵的透视承载——DrawDepthSprites 按提交序画
// 不变 + 透视校正画面与直接 DrawAtlasEx 一致（像素钉）。
func TestP37DepthSpritesCarryPersp(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	corners, weights, ok := atlasPerspQuad(100)
	if !ok {
		t.Fatal("投影失败")
	}
	os.Unsetenv("GOGPU_RENDER_MODE")
	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	far := render.DepthSprite{Sprite: render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
		Perspective: true, Corners: corners, Weights: weights,
	}, Depth: 10, Name: "far-trapezoid"}
	near := render.DepthSprite{Sprite: render.AtlasSprite{
		SrcX: 0, SrcY: 16, SrcW: 16, SrcH: 16,
		DstX: 238, DstY: 52, DstW: 60, DstH: 50, Opacity: 1, // 摆在梯形 AABB 之外（x>220, y<82 区），采样区不重叠
	}, Depth: 2, Name: "near-rect"}
	res, err := dc.DrawDepthSprites(img, []render.DepthSprite{far, near})
	if err != nil {
		t.Fatalf("DrawDepthSprites: %v", err)
	}
	if res.Drawn != 2 {
		t.Fatalf("深度批形态 %+v，want Drawn=2", res)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("深度批没走 GPU 或掉回 CPU：%s", st.LogLine())
	}
	out := mustExport(t, dc)
	// 远精灵梯形内部取一格色带（左半红/暗带），近精灵矩形内取一格，
	// 与直接画（无深度口）逐格对拍。
	direct := drawAtlasPersp(t, img, far.Sprite, false)
	defer direct.Dispose()
	if worst, changed := sampleGridDiff(out, direct, atlasPerspProbePoints(corners)); changed != 0 || worst != 0 {
		t.Fatalf("深度口的透视精灵与直接 DrawAtlasEx 不一致（changed=%d worst=%d）", changed, worst)
	}
}

// TestP37ImageMixedOrderDraws：透视与仿射图像命令任意先后序都必须都上屏
// （既有洞修复钉：RecordDraws 的顶点缓冲切换——透视绘制切到 36B 缓冲后，
// 仿射绘制必须切回 32B 缓冲；修复前「透视在前」的整帧因顶点越界整 pass
// 编码失效，两笔全丢。quad 路与图集路同钉）。
func TestP37ImageMixedOrderDraws(t *testing.T) {
	requireNativeGPU(t)
	img := atlasPerspPlate(t)
	defer img.Dispose()
	corners, weights, ok := atlasPerspQuad(100)
	if !ok {
		t.Fatal("投影失败")
	}
	os.Unsetenv("GOGPU_RENDER_MODE")

	// quad 路混批：DrawImageQuad(透视) 与 DrawImageEx(仿射) 两种序。
	drawQuadMix := func(perspFirst bool) *render.ImageBuf {
		t.Helper()
		dc := render.NewContext(320, 220)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		if perspFirst {
			_ = dc.DrawImageQuad(img, corners, render.QuadDrawOptions{
				Interpolation: render.InterpBilinear, Opacity: 1, Perspective: true, Weights: weights,
			})
			dc.DrawImageEx(img, render.DrawImageOptions{X: 238, Y: 52, DstWidth: 60, DstHeight: 50, Opacity: 1})
		} else {
			dc.DrawImageEx(img, render.DrawImageOptions{X: 238, Y: 52, DstWidth: 60, DstHeight: 50, Opacity: 1})
			_ = dc.DrawImageQuad(img, corners, render.QuadDrawOptions{
				Interpolation: render.InterpBilinear, Opacity: 1, Perspective: true, Weights: weights,
			})
		}
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("没走 GPU 或掉回 CPU：%s", st.LogLine())
		}
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("显存读回失败")
		}
		return out
	}
	for _, perspFirst := range []bool{true, false} {
		out := drawQuadMix(perspFirst)
		r, g, b, _ := out.GetRGBA(170, 120) // 梯形内部（左半红区）
		if r == 255 && g == 255 && b == 255 {
			t.Fatalf("quad 混批 perspFirst=%v：透视梯形丢失（整 pass 失效形态）", perspFirst)
		}
		if r, _, _, _ := out.GetRGBA(268, 77); r == 255 && g == 255 && b == 255 {
			t.Fatalf("quad 混批 perspFirst=%v：仿射矩形丢失", perspFirst)
		}
		out.Dispose()
	}

	// 图集路混批：DrawAtlasEx [透视,仿射] 与 [仿射,透视] 两种序。
	drawAtlasMix := func(perspFirst bool) *render.ImageBuf {
		t.Helper()
		farSp := render.AtlasSprite{
			SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, Opacity: 1,
			Perspective: true, Corners: corners, Weights: weights,
		}
		nearSp := render.AtlasSprite{
			SrcX: 0, SrcY: 16, SrcW: 16, SrcH: 16,
			DstX: 238, DstY: 52, DstW: 60, DstH: 50, Opacity: 1,
		}
		sprs := []render.AtlasSprite{farSp, nearSp}
		if !perspFirst {
			sprs = []render.AtlasSprite{nearSp, farSp}
		}
		dc := render.NewContext(320, 220)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		if _, err := dc.DrawAtlasEx(img, sprs, render.AtlasDrawOptions{}); err != nil {
			t.Fatalf("DrawAtlasEx: %v", err)
		}
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("显存读回失败")
		}
		return out
	}
	for _, perspFirst := range []bool{true, false} {
		out := drawAtlasMix(perspFirst)
		if r, g, b, _ := out.GetRGBA(170, 120); r == 255 && g == 255 && b == 255 {
			t.Fatalf("图集混批 perspFirst=%v：透视梯形丢失", perspFirst)
		}
		if r, g, b, _ := out.GetRGBA(268, 77); r == 255 && g == 255 && b == 255 {
			t.Fatalf("图集混批 perspFirst=%v：仿射精灵丢失", perspFirst)
		}
		out.Dispose()
	}
	t.Log("P37 混批四序全上屏：透视/仿射 × quad/图集")
}
