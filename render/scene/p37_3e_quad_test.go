//go:build !nogpu

package scene_test

// P3.7 批 3e-2：scene 的「算好再喂」quad+权重通道（2.5D 正路，Godot 形态）。
//
// 判据正本 `evidence/P3.7/batch3e-design.md` §3/§4：
//  1. 四角 + 权重 + 透视位 + 采样 + 不透明度编码—解码逐值回环；
//  2. 回放 ≡ 直接调 `render.Context.DrawImageQuad`（scene 不再算第二套透视）；
//     反向鉴别：同一张图关掉透视必须与梯形 quad 画面不同（否则权重是空壳）；
//  3. 角点在录制期由场景变换烘焙、绘制时把回放 CTM 复位（与 TagImage 同约定）；
//  4. 合并位移只挪角点，**不**挪权重与不透明度（挪了就是把 1/w 当坐标平移）；
//     跨注册表合并要抬图像索引（不抬就指错图）。
//
// CPU 瓦片腿的表态（平行四边形折仿射画、真透视拒画 + 计数）在包内测
// `p37_3e_cpu_test.go`——Renderer.Render 一有加速器就走 GPU 优先腿，外部够不到瓦片执行器。

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

const quadSize = 8

// quadImage 是一张 2×2 格的红绿棋盘：UV 落点一挪，颜色就跟着挪。
// 纯色图当不了透视的鉴别物——仿射和透视采到同一个颜色，两腿天然一模一样。
// （scene.NewImage 只给尺寸不给像素，数据得自己铺。）
func quadImage() *scene.Image {
	img := scene.NewImage(quadSize, quadSize)
	img.Data = make([]byte, quadSize*quadSize*4)
	for y := 0; y < quadSize; y++ {
		for x := 0; x < quadSize; x++ {
			i := (y*quadSize + x) * 4
			img.Data[i+3] = 255
			if ((x/2)+(y/2))%2 == 0 {
				img.Data[i+0] = 255 // 红格
			} else {
				img.Data[i+1] = 255 // 绿格
			}
		}
	}
	return img
}

// quadBuf 与回放侧 sceneImageBuf 同一份铺法：那个函数在包内，外部测试够不到，
// 但「直接画」那一腿要的正是同一张图、同一换算。
func quadBuf(t *testing.T, img *scene.Image) *render.ImageBuf {
	t.Helper()
	return render.ImageBufFromImage(&image.RGBA{
		Pix:    img.Data,
		Stride: img.Width * 4,
		Rect:   image.Rect(0, 0, img.Width, img.Height),
	})
}

// trapezoidCorners 是「上窄下宽」的投影梯形：只有带权重插值才画得对。
func trapezoidCorners(ox, oy float64) [4]render.Point {
	return [4]render.Point{
		{X: ox + 12, Y: oy + 0},  // TL
		{X: ox + 28, Y: oy + 0},  // TR
		{X: ox + 36, Y: oy + 40}, // BR
		{X: ox + 4, Y: oy + 40},  // BL
	}
}

// parallelogramCorners 是平行四边形：仿射可精确表达，CPU 瓦片腿能吃下。
func parallelogramCorners(ox, oy float64) [4]render.Point {
	return [4]render.Point{
		{X: ox + 6, Y: oy + 0},   // TL
		{X: ox + 30, Y: oy + 0},  // TR
		{X: ox + 24, Y: oy + 24}, // BR
		{X: ox + 0, Y: oy + 24},  // BL
	}
}

func quadWeights() [4]float64 { return [4]float64{1, 1, 0.5, 0.5} }

type quadRecord struct {
	imageIndex  uint32
	corners     [4]render.Point
	weights     [4]float64
	perspective bool
	interp      render.InterpolationMode
	opacity     float64
	count       int
	all         []quadRecord
}

// readQuads 回读流里每一枚 quad（all 按流序），count = 总枚数。
func readQuads(t *testing.T, enc *scene.Encoding) quadRecord {
	t.Helper()
	dec := scene.NewDecoder(enc)
	if dec == nil {
		t.Fatal("nil decoder")
	}
	var got quadRecord
	for dec.Next() {
		if dec.Tag() != scene.TagImageQuad {
			continue
		}
		idx, corners, weights, persp, interp, opacity := dec.ImageQuad()
		got.count++
		got.imageIndex, got.corners, got.weights, got.perspective, got.interp, got.opacity = idx, corners, weights, persp, interp, opacity
		got.all = append(got.all, quadRecord{
			imageIndex: idx, corners: corners, weights: weights,
			perspective: persp, interp: interp, opacity: opacity,
		})
	}
	return got
}

// paintedCount 数与白底不同的像素。0 意味着那一腿什么都没画，
// 此时任何「两腿一致 / 两腿不同」的断言都不算证据。
func paintedCount(pm *render.Pixmap) int {
	if pm == nil {
		return 0
	}
	n := 0
	for y := 0; y < projProbeSize; y++ {
		for x := 0; x < projProbeSize; x++ {
			if pm.GetPixel(x, y) != render.White {
				n++
			}
		}
	}
	return n
}

// 1 + 4：逐值回环、合并位移只挪角点、跨注册表合并抬索引。
func TestP373ESceneQuadEncodingAndMerge(t *testing.T) {
	s := scene.NewScene()
	s.DrawImageQuad(quadImage(), trapezoidCorners(10, 10), render.QuadDrawOptions{
		Perspective: true, Weights: quadWeights(), Opacity: 0.75,
	})
	got := readQuads(t, s.Encoding())
	if got.count != 1 {
		t.Fatalf("应恰好一枚 TagImageQuad，实得 %d", got.count)
	}
	want := trapezoidCorners(10, 10)
	if got.corners != want {
		t.Fatalf("角点回环不齐：%v vs %v", got.corners, want)
	}
	if got.weights != quadWeights() || !got.perspective || got.opacity != 0.75 {
		t.Fatalf("权重/透视/不透明度回环不齐：%+v", got)
	}
	if got.imageIndex != 0 {
		t.Fatalf("首张图的索引应为 0，得 %d", got.imageIndex)
	}

	// 合并位移：角点整体平移，权重与不透明度一格不动；跨注册表时索引同样要抬。
	host := scene.NewScene()
	host.DrawImageQuad(quadImage(), parallelogramCorners(0, 0), render.QuadDrawOptions{})
	host.AppendWithTranslation(s, 6, -4)
	moved := readQuads(t, host.Encoding())
	if moved.count != 2 {
		t.Fatalf("合并后 quad 数 %d，want 2", moved.count)
	}
	if moved.imageIndex != 1 {
		t.Fatalf("位移合并没抬图像索引：%d want 1", moved.imageIndex)
	}
	for i := range moved.corners {
		if moved.corners[i].X != want[i].X+6 || moved.corners[i].Y != want[i].Y-4 {
			t.Fatalf("合并位移后角点不齐：第 %d 角 %v", i, moved.corners[i])
		}
	}
	if moved.weights != quadWeights() || moved.opacity != 0.75 {
		t.Fatalf("合并把权重/不透明度当坐标平移了：%+v", moved)
	}

	// 跨注册表合并：guest 的图必须指向 host 里的新位置，host 自己的那格不许被抬。
	host2 := scene.NewScene()
	host2.DrawImageQuad(quadImage(), trapezoidCorners(0, 0), render.QuadDrawOptions{})
	host2.Append(s)
	merged := readQuads(t, host2.Encoding())
	if merged.count != 2 || merged.imageIndex != 1 {
		t.Fatalf("合并后 quad=%d 枚、最后索引 %d，want 2 枚 / 1", merged.count, merged.imageIndex)
	}
	if len(merged.all) != 2 || merged.all[0].imageIndex != 0 {
		t.Fatalf("host 自己那枚的索引被抬了：%+v", merged.all)
	}
}

// 2：回放 ≡ 直接调 Context.DrawImageQuad；并与「关掉透视」区分开。
func TestP373ESceneQuadReplayMatchesContext(t *testing.T) {
	img := quadImage()
	corners := trapezoidCorners(20, 30)
	opts := render.QuadDrawOptions{Perspective: true, Weights: quadWeights(), Opacity: 1}

	direct := contextPaint(func(dc *render.Context) {
		dc.DrawImageQuad(quadBuf(t, img), corners, opts)
	})
	replay := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.DrawImageQuad(img, corners, opts)
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("RenderScene quad: %v", err)
		}
	})
	if paintedCount(direct) == 0 || paintedCount(replay) == 0 {
		t.Fatalf("两腿画出的像素 direct=%d replay=%d，有一腿根本没画", paintedCount(direct), paintedCount(replay))
	}
	if diff := paintDiff(direct, replay); diff != 0 {
		t.Fatalf("scene quad 回放与直接 DrawImageQuad 差 %d 像素：透视在 scene 侧被另算了一遍", diff)
	}

	flat := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.DrawImageQuad(img, corners, render.QuadDrawOptions{Weights: quadWeights(), Opacity: 1})
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("RenderScene non-persp: %v", err)
		}
	})
	if paintedCount(flat) == 0 {
		t.Fatal("关透视那一腿什么都没画")
	}
	// 阈值而非「!=0」：梯形 + 棋盘下权重一挪就是成片颜色搬家（实测 784 像素），
	// 差一两像素只能算噪声，不足以证明权重真进了采样。
	if d := paintDiff(flat, replay); d < 100 {
		t.Fatalf("透视开/关只差 %d 像素（want ≥100）：quad 的权重没进采样", d)
	}
	t.Logf("像素 direct=%d replay=%d flat=%d；flat↔persp 差 %d 像素",
		paintedCount(direct), paintedCount(replay), paintedCount(flat), paintDiff(flat, replay))
}

// 3：角点在录制期由场景变换烘焙（与 TagImage 同约定），回放 CTM 不重复施加。
func TestP373ESceneQuadBakesSceneTransform(t *testing.T) {
	img := quadImage()
	corners := trapezoidCorners(0, 0)
	opts := render.QuadDrawOptions{Perspective: true, Weights: quadWeights(), Opacity: 1}

	viaTransform := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.PushTransform(scene.TranslateAffine(20, 30))
		s.DrawImageQuad(img, corners, opts)
		s.PopTransform()
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("RenderScene baked: %v", err)
		}
	})
	viaLiteral := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.DrawImageQuad(img, shiftQuad(corners, 20, 30), opts)
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("RenderScene literal: %v", err)
		}
	})
	if paintedCount(viaLiteral) == 0 {
		t.Fatal("两条腿都没画东西，下面那条「一致」不算证据")
	}
	if diff := paintDiff(viaTransform, viaLiteral); diff != 0 {
		t.Fatalf("场景变换烘焙与等价手写角点差 %d 像素（变换被重复施加或丢了）", diff)
	}
}

// 不调 quad 动词的存量场景：流里一枚 TagImageQuad 都没有（新增 tag 不改旧画面）。
func TestP373ESceneQuadAbsentFromLegacyStreams(t *testing.T) {
	legacy := scene.NewScene()
	legacy.Fill(scene.FillNonZero, scene.TranslateAffine(4, 4),
		scene.SolidBrush(render.RGBA{R: 1, A: 1}), scene.NewRectShape(0, 0, 20, 20))
	legacy.DrawImage(quadImage(), scene.TranslateAffine(2, 2))
	if got := readQuads(t, legacy.Encoding()).count; got != 0 {
		t.Fatalf("存量场景（只 Fill + 仿射 DrawImage）流里冒出 %d 枚 TagImageQuad", got)
	}
	legacy.DrawImageQuad(quadImage(), parallelogramCorners(60, 60), render.QuadDrawOptions{})
	if got := readQuads(t, legacy.Encoding()).count; got != 1 {
		t.Fatalf("补画一次 quad 后回读出 %d 枚，want 1", got)
	}
}

// 自交（蝴蝶形）quad：render.Context 会以哨兵错拒画，scene 的 GPU 腿不中断整帧，
// 但必须「一格不画」——静默丢掉的是错数据，不能是错画面。
// （GPU 腿这条目前没有具名计数，登记为批 3 观测缺口，见 batch3e-design.md §6。）
func TestP373ESceneQuadSelfIntersectingDrawsNothing(t *testing.T) {
	img := quadImage()
	bowtie := [4]render.Point{
		{X: 20, Y: 20}, {X: 80, Y: 20}, {X: 20, Y: 80}, {X: 80, Y: 80}, // TL,TR,BR,BL 交叉
	}
	if kind := render.ClassifyQuad(bowtie); kind == render.QuadOK {
		t.Fatalf("蝴蝶形被判成合法 quad（kind=%v）：这条判据失去鉴别力", kind)
	}
	got := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.DrawImageQuad(img, bowtie, render.QuadDrawOptions{Opacity: 1})
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("RenderScene bowtie: %v", err)
		}
	})
	if n := paintedCount(got); n != 0 {
		t.Fatalf("自交 quad 画出 %d 像素：哨兵错被吞掉后还画了别的东西", n)
	}
}

func shiftQuad(q [4]render.Point, dx, dy float64) [4]render.Point {
	out := q
	for i := range out {
		out[i].X += dx
		out[i].Y += dy
	}
	return out
}
