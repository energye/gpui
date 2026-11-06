//go:build !nogpu

package render_test

// P3.7 批 3b 钉：三公开入口挂材质（DrawImageEx / DrawImageQuad / DrawAtlasEx）
// + CPU skip 口径 + 拒绝面。判据源：batch3b-design.md §4。
// 像素判据主干：恒等材质 ≡ 无材质逐位（A 档组合保持引擎乘链不变，批 3a/探针
// P3 实证）；区分钉用 uv-gradient 材质（防纯色假绿）。

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

const p37BIdentityWGSL = `
@group(0) @binding(1) var p37b_tex: texture_2d<f32>;
@group(0) @binding(2) var p37b_samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(p37b_tex, p37b_samp, uv);
}`

const p37BGradWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = 0.3 + 0.6 * uv.x;
    return vec4<f32>(0.8 * a, 0.35 * a, 0.1 * a, a);
}`

func p37BChecker(w, h int) *render.ImageBuf {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := y*m.Stride + x*4
			m.Pix[o] = uint8(40 + ((x*7 + y*13) % 200))
			m.Pix[o+1] = uint8(30 + ((x*11 + y*5) % 220))
			m.Pix[o+2] = uint8(20 + ((x*3 + y*17) % 230))
			m.Pix[o+3] = 255
		}
	}
	return render.ImageBufFromImage(m)
}

func p37BRegion(t *testing.T, dc *render.Context, x, y, w, h int) []byte {
	t.Helper()
	out := make([]byte, 0, w*h*4)
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			r, g, b, a := dc.Image().At(px, py).RGBA()
			out = append(out, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
		}
	}
	return out
}

func p37BDiff(a, b []byte) (n int, worst int) {
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		if d > 0 {
			n++
			if d > worst {
				worst = d
			}
		}
	}
	return n, worst
}

// TestP37BIdentityMaterialEqualsPlain 三入口恒等材质 ≡ 无材质逐位：同一上下文
// 两个区域各画一次，读回逐位相同（DrawImageEx / DrawImageQuad 含 tint /
// DrawAtlasEx 同子矩形）。
func TestP37BIdentityMaterialEqualsPlain(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(16, 16)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()

	dc := render.NewContext(64, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)

	// DrawImageEx：nil @ (4,8) / 材质 @ (36,8)
	dc.DrawImageEx(img, render.DrawImageOptions{X: 4, Y: 8})
	dc.DrawImageEx(img, render.DrawImageOptions{X: 36, Y: 8, Material: e})

	// DrawImageQuad：nil+tint @ (4,20 下方另行) —— 用两上下文避免重叠
	dc2 := render.NewContext(64, 32)
	defer func() { _ = dc2.Close() }()
	dc2.ClearWithColor(render.White)
	dc2.DrawImageQuad(img, [4]render.Point{{X: 4, Y: 8}, {X: 20, Y: 8}, {X: 20, Y: 24}, {X: 4, Y: 24}},
		render.QuadDrawOptions{Tint: render.RGBA{R: 1, G: 0.5, B: 0.25, A: 1}})
	dc2.DrawImageQuad(img, [4]render.Point{{X: 36, Y: 8}, {X: 52, Y: 8}, {X: 52, Y: 24}, {X: 36, Y: 24}},
		render.QuadDrawOptions{Tint: render.RGBA{R: 1, G: 0.5, B: 0.25, A: 1}, Material: e})

	// DrawAtlasEx：nil 精灵 @ (4,8) / 材质精灵 @ (36,8)（同图集同子矩形）
	dc3 := render.NewContext(64, 32)
	defer func() { _ = dc3.Close() }()
	dc3.ClearWithColor(render.White)
	dc3.DrawAtlasEx(img, []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16, DstX: 4, DstY: 8, DstW: 16, DstH: 16},
		{SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16, DstX: 36, DstY: 8, DstW: 16, DstH: 16, Material: e},
	}, render.AtlasDrawOptions{})

	for _, c := range []*render.Context{dc, dc2, dc3} {
		if err := c.Fill(); err != nil {
			t.Fatalf("Fill: %v", err)
		}
		if err := c.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
	}
	a := p37BRegion(t, dc, 4, 8, 16, 16)
	b := p37BRegion(t, dc, 36, 8, 16, 16)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("DrawImageEx identity material vs plain: %d differing channels, worst %d — 组合腿乘链偏离标准腿", n, worst)
	}
	a = p37BRegion(t, dc2, 4, 8, 16, 16)
	b = p37BRegion(t, dc2, 36, 8, 16, 16)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("DrawImageQuad identity material (tint) vs plain: %d differing channels, worst %d", n, worst)
	}
	a = p37BRegion(t, dc3, 4, 8, 16, 16)
	b = p37BRegion(t, dc3, 36, 8, 16, 16)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("DrawAtlasEx identity material vs plain: %d differing channels, worst %d", n, worst)
	}
}

// TestP37BDrawAtlasLegacyMaterialParity 老口 DrawAtlas（GPU 分支）材质路由钉：
// 批 3b 审计补——老口此前只有 CPU skip 钉，GPU 分支的材质队列（材质+仿射 ⇒
// QueueImageDrawMaterial）无像素咬合。恒等材质 ≡ 无材质逐位（同批两精灵）。
func TestP37BDrawAtlasLegacyMaterialParity(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(16, 16)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()

	dc := render.NewContext(64, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.DrawAtlas(img, []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16, DstX: 4, DstY: 8, DstW: 16, DstH: 16},
		{SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16, DstX: 36, DstY: 8, DstW: 16, DstH: 16, Material: e},
	})
	if err := dc.Fill(); err != nil {
		t.Fatal(err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	a := p37BRegion(t, dc, 4, 8, 16, 16)
	b := p37BRegion(t, dc, 36, 8, 16, 16)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("DrawAtlas (legacy GPU branch) identity material vs plain: %d differing channels, worst %d", n, worst)
	}
}

// TestP37BMaterialLegEngages 区分钉（防假绿）：uv-gradient 材质腿必须真接上——
// 材质画面 ≠ 无材质画面（成片差异），且 ≡ 同 shader 走填充路（同矩形同 uv 映射，
// 两条生产腿互证；两腿量化链不同 ⇒ ±1 容差，探针 P3 实测 0 差）。
func TestP37BMaterialLegEngages(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(16, 16)
	e, err := render.NewShaderEffect(p37BGradWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()

	const rx, ry, rw, rh = 4, 4, 24, 24
	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.DrawImageEx(img, render.DrawImageOptions{X: rx, Y: ry, DstWidth: rw, DstHeight: rh, Material: e})
	if err := dc.Fill(); err != nil {
		t.Fatal(err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}

	dc2 := render.NewContext(32, 32)
	defer func() { _ = dc2.Close() }()
	dc2.ClearWithColor(render.White)
	dc2.DrawImageEx(img, render.DrawImageOptions{X: rx, Y: ry, DstWidth: rw, DstHeight: rh})
	if err := dc2.Fill(); err != nil {
		t.Fatal(err)
	}
	if err := dc2.FlushGPU(); err != nil {
		t.Fatal(err)
	}

	// 填充路同 shader 同矩形（premul 契约修复后两腿同语义）。
	dc3 := render.NewContext(32, 32)
	defer func() { _ = dc3.Close() }()
	dc3.ClearWithColor(render.White)
	dc3.SetShader(e)
	dc3.DrawRectangle(rx, ry, rw, rh)
	if err := dc3.Fill(); err != nil {
		t.Fatal(err)
	}
	if err := dc3.FlushGPU(); err != nil {
		t.Fatal(err)
	}

	mat := p37BRegion(t, dc, rx, ry, rw, rh)
	plain := p37BRegion(t, dc2, rx, ry, rw, rh)
	fill := p37BRegion(t, dc3, rx, ry, rw, rh)

	if n, _ := p37BDiff(mat, plain); n < 100 {
		t.Fatalf("material leg did not engage: material vs plain only %d differing channels (阈值 ≥100，防纯色假绿)", n)
	}
	if n, worst := p37BDiff(mat, fill); n > 0 && worst > 1 {
		t.Fatalf("material leg vs fill leg (same shader, same rect): %d differing channels, worst %d — 两生产腿语义分叉（premul 契约回归？）", n, worst)
	}
}

// TestP37BCpuModeSkipsMaterial CPU 模式：材质绘制跳过 + MaterialSkips 计数 +
// CPUFallbackOps==0（skip ≠ fallback 口径），像素不动。
func TestP37BCpuModeSkipsMaterial(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img := p37BChecker(16, 16)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	dc := render.NewContext(64, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	before := dc.RenderPathStats()
	dc.DrawImageEx(img, render.DrawImageOptions{X: 4, Y: 8, Material: e})
	dc.DrawImageQuad(img, [4]render.Point{{X: 36, Y: 8}, {X: 52, Y: 8}, {X: 52, Y: 24}, {X: 36, Y: 24}},
		render.QuadDrawOptions{Material: e})
	afterTwo := dc.RenderPathStats()
	if got := afterTwo.MaterialSkips - before.MaterialSkips; got != 2 {
		t.Fatalf("MaterialSkips=%d want 2（DrawImageEx/Quad 各一跳）", got)
	}
	if fb := afterTwo.CPUFallbackOps - before.CPUFallbackOps; fb != 0 {
		t.Fatalf("DrawImageEx/Quad material skips moved CPUFallbackOps (%d) — skip 冒充 fallback（违反 P3.2 口径）", fb)
	}
	dc.DrawAtlas(img, []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16, DstX: 4, DstY: 20, DstW: 16, DstH: 16, Material: e},
	})
	after := dc.RenderPathStats()
	if got := after.MaterialSkips - afterTwo.MaterialSkips; got != 1 {
		t.Fatalf("atlas material skip missing (got %d)", got)
	}
	// 图集整批在 CPU 模式下本就走 CPU 路并记整批 fallback（既有语义，与材质
	// 无关——材质精灵在批内被跳过），这里只要求不因材质引入额外 fallback。
	if fb := after.CPUFallbackOps - afterTwo.CPUFallbackOps; fb > 1 {
		t.Fatalf("atlas batch fallback moved %d (≤1 expected, 既有整批语义)", fb)
	}
	// 像素不动（跳过 = 什么都不画）。
	if n, _ := p37BDiff(p37BRegion(t, dc, 4, 8, 16, 16), whiteRegion(t, 16, 16)); n != 0 {
		t.Fatalf("CPU mode material draw painted %d channels — 必须整笔跳过", n)
	}
}

func whiteRegion(t *testing.T, w, h int) []byte {
	t.Helper()
	out := make([]byte, 0, w*h*4)
	for i := 0; i < w*h; i++ {
		out = append(out, 255, 255, 255, 255)
	}
	return out
}

// TestP37BMaterialRejections 拒绝面：材质 + Perspective / bicubic ⇒
// ErrQuadMaterialUnsupported；DrawImageEx 材质 + blend≠Normal ⇒ CPU skip（无错误通道）。
func TestP37BMaterialRejections(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(8, 8)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)

	// 材质×透视专项（2026-11-07）：材质+Perspective 放行（权重透传 36B 材质
	// 缓冲），组合正确性由 TestP37MatPersp* 五钉承管（identity≡plain 逐位、
	// 解析分界、投影层、精灵路），此处不再断言拒绝。
	// 材质+bicubic 纹素契约（2026-11-07）已放行——引擎喂纹素（echo≡标准采样
	// 逐位由 TestP37MatTexelEcho* 钉住），此处不再断言拒绝。
	_ = img
	// blend≠Normal 在 blend 校验处先行拒绝（与材质无关的既有口径）。
	errBl := dc.DrawImageQuad(img, [4]render.Point{{X: 4, Y: 4}, {X: 12, Y: 4}, {X: 12, Y: 12}, {X: 4, Y: 12}},
		render.QuadDrawOptions{Material: e, BlendMode: render.BlendMultiply})
	if errBl == nil {
		t.Fatal("material+non-Normal blend must be rejected at the blend check")
	}
	// DrawImageEx 材质 + blend≠Normal：无错误通道 ⇒ CPU skip + 计数。
	before := dc.RenderPathStats().MaterialSkips
	dc.DrawImageEx(img, render.DrawImageOptions{X: 4, Y: 4, BlendMode: render.BlendMultiply, Material: e})
	if got := dc.RenderPathStats().MaterialSkips - before; got != 1 {
		t.Fatalf("DrawImageEx material+blend must skip+count (got %d)", got)
	}
	// 图集：材质精灵 + 自带透视 ⇒ 放行（材质×透视专项 2026-11-07，权重透传
	// 36B 材质缓冲），不再 skip+计数；组合画面由 TestP37MatPerspSpriteRoadDrawn
	// 钉住（identity≡plain 逐位）。
	before = dc.RenderPathStats().MaterialSkips
	res, err := dc.DrawAtlasEx(img, []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8,
			Material: e, Perspective: true, Corners: [4]render.Point{{X: 4, Y: 4}, {X: 12, Y: 4}, {X: 12, Y: 12}, {X: 4, Y: 12}},
			Weights: [4]float64{1, 1, 1, 1}},
	}, render.AtlasDrawOptions{})
	if err != nil {
		t.Fatalf("DrawAtlasEx material+persp: %v", err)
	}
	if res.Drawn != 1 {
		t.Fatalf("material+persp sprite must be drawn (Drawn=%d)", res.Drawn)
	}
	if got := dc.RenderPathStats().MaterialSkips - before; got != 0 {
		t.Fatalf("atlas material+persp sprite must not count a skip (got %d)", got)
	}
}
