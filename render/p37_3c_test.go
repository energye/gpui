package render_test

// P3.7 批 3c 钉：绑定与预算契约扩展（D4 加重档）。判据源：batch3c-design.md
// §1/§3。前置探针（gles 真编译 binding 3+ + 272B uniform）已过。

import (
	"image"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

func p37CChecker(w, h int, seed int) *render.ImageBuf {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := y*m.Stride + x*4
			m.Pix[o] = uint8(40 + ((x*7 + y*13 + seed*31) % 200))
			m.Pix[o+1] = uint8(30 + ((x*11 + y*5 + seed*17) % 220))
			m.Pix[o+2] = uint8(20 + ((x*3 + y*17 + seed*7) % 230))
			m.Pix[o+3] = 255
		}
	}
	return render.ImageBufFromImage(m)
}

const p37CMixWGSL = `
struct Uniforms {
    mix_level: f32,
    _pad0: vec3<f32>,
}
@group(0) @binding(0) var<uniform> u: Uniforms;
@group(0) @binding(1) var p37c_tex0: texture_2d<f32>;
@group(0) @binding(2) var p37c_samp0: sampler;
@group(0) @binding(3) var p37c_tex1: texture_2d<f32>;
@group(0) @binding(4) var p37c_samp1: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = textureSample(p37c_tex0, p37c_samp0, uv);
    let b = textureSample(p37c_tex1, p37c_samp1, uv);
    return mix(a, b, u.mix_level);
}`

const p37CSplitWGSL = `
struct BigUniforms {
    split: f32,
    _pad0: vec3<f32>,
    coeffs: array<vec4<f32>, 16>, // 共 272B：超旧 256B 预算
}
@group(0) @binding(0) var<uniform> u: BigUniforms;
@group(0) @binding(1) var p37c_tex0: texture_2d<f32>;
@group(0) @binding(2) var p37c_samp0: sampler;
@group(0) @binding(3) var p37c_tex1: texture_2d<f32>;
@group(0) @binding(4) var p37c_samp1: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = textureSample(p37c_tex0, p37c_samp0, uv);
    let b = textureSample(p37c_tex1, p37c_samp1, uv);
    if uv.x < u.split {
        return a;
    }
    return b;
}`

// TestP37CContractPairs 契约面：三对纹理放行（ShaderInterface 读数）、对不齐/
// 孤绑定/超对拒绝、272B uniform 放行、>65536 拒绝。
func TestP37CContractPairs(t *testing.T) {
	threePairs := `
struct U { v: f32, _p: vec3<f32>, }
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var t0: texture_2d<f32>;
@group(0) @binding(2) var s0: sampler;
@group(0) @binding(3) var t1: texture_2d<f32>;
@group(0) @binding(4) var s1: sampler;
@group(0) @binding(5) var t2: texture_2d<f32>;
@group(0) @binding(6) var s2: sampler;

@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return (textureSample(t0, s0, uv) + textureSample(t1, s1, uv) + textureSample(t2, s2, uv)) / 3.0;
}`
	e, err := render.NewShaderEffect(threePairs)
	if err != nil {
		t.Fatalf("three pairs rejected: %v", err)
	}
	defer e.Close()
	bind := e.ShaderBinding()
	if !bind.UsesImage || bind.TexturePairs != 3 {
		t.Fatalf("ShaderInterface.TexturePairs=%d want 3", bind.TexturePairs)
	}
	// WGSL uniform 布局：{f32, vec3} 内容 28B 按结构体对齐 16 取整 ⇒ 32B。
	if bind.UniformBytes != 32 {
		t.Fatalf("ShaderInterface.UniformBytes=%d want 32 (WGSL 对齐取整)", bind.UniformBytes)
	}

	loneSampler := `
@group(0) @binding(2) var s0: sampler;
@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    _ = s0; return vec4<f32>(0.0);
}`
	if _, err := render.NewShaderEffect(loneSampler); err == nil {
		t.Fatal("sampler without texture must be rejected")
	}
	loneTex := `
@group(0) @binding(1) var t0: texture_2d<f32>;
@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    _ = t0; return vec4<f32>(0.0);
}`
	if _, err := render.NewShaderEffect(loneTex); err == nil {
		t.Fatal("texture without sampler must be rejected")
	}
	overCap := strings.Replace(threePairs, "@binding(5) var t2", "@binding(17) var t2", 1)
	overCap = strings.Replace(overCap, "@binding(6) var s2", "@binding(18) var s2", 1)
	if _, err := render.NewShaderEffect(overCap); err == nil {
		t.Fatal("bindings beyond the pair cap must be rejected")
	}
	bigOK := strings.Replace(p37CSplitWGSL, "@group(0)", "@group(0)", 1)
	e2, err := render.NewShaderEffect(bigOK)
	if err != nil {
		t.Fatalf("272B uniform rejected: %v", err)
	}
	defer e2.Close()
	// WGSL uniform 布局：vec3 之后数组按 16 对齐 ⇒ 偏移 32，总 32+256=288。
	if got := e2.ShaderBinding().UniformBytes; got != 288 {
		t.Fatalf("UniformBytes=%d want 288", got)
	}
	tooBig := strings.Replace(p37CSplitWGSL, "array<vec4<f32>, 16>", "array<vec4<f32>, 5000>", 1)
	if _, err := render.NewShaderEffect(tooBig); err == nil {
		t.Fatal("uniform beyond ShaderUniformBudgetMax must be rejected")
	}
}

// TestP37CSetImageAtSlots SetImageAt 回环 + 越槽拒绝 + SetImage 旧口 = slot 0。
func TestP37CSetImageAtSlots(t *testing.T) {
	e, err := render.NewShaderEffect(p37CMixWGSL)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	a := p37CChecker(4, 4, 1)
	b := p37CChecker(4, 4, 2)
	if err := e.SetImageAt(0, a); err != nil {
		t.Fatalf("SetImageAt(0): %v", err)
	}
	if err := e.SetImageAt(1, b); err != nil {
		t.Fatalf("SetImageAt(1): %v", err)
	}
	if e.ImageAt(0) != a || e.ImageAt(1) != b {
		t.Fatal("ImageAt round-trip broken")
	}
	if err := e.SetImageAt(2, b); err == nil {
		t.Fatal("slot beyond declared pairs must be a named rejection")
	}
	// 旧口 = slot 0 别价。
	c := p37CChecker(4, 4, 3)
	e.SetImage(c)
	if e.Image() != c || e.ImageAt(0) != c {
		t.Fatal("legacy SetImage/Image must alias slot 0")
	}
}

func p37CRegion(t *testing.T, dc *render.Context, x, y, w, h int) []byte {
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

// TestP37CMultiTextureMaterialPixel 多纹理材质像素钉：mix_level=1 ⇒ 画面 ≡
// SetImageAt(1) 的第二纹理（SetImage 槽位真绑定到位）；mix_level=0 ⇒ ≡ 精灵纹理。
func TestP37CMultiTextureMaterialPixel(t *testing.T) {
	requireFilterOpsGPU(t)
	imgA := p37CChecker(24, 24, 1)
	imgB := p37CChecker(24, 24, 2)

	draw := func(mixLevel float32) []byte {
		e, err := render.NewShaderEffect(p37CMixWGSL)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		if err := e.SetUniformBytes([]byte{0, 0, 128, 63, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil { // 1.0f32 LE
			t.Fatal(err)
		}
		_ = mixLevel
		if err := e.SetImageAt(1, imgB); err != nil {
			t.Fatal(err)
		}
		dc := render.NewContext(32, 32)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		dc.DrawImageEx(imgA, render.DrawImageOptions{X: 4, Y: 4, DstWidth: 24, DstHeight: 24, Material: e})
		if err := dc.Fill(); err != nil {
			t.Fatal(err)
		}
		if err := dc.FlushGPU(); err != nil {
			t.Fatal(err)
		}
		return p37CRegion(t, dc, 4, 4, 24, 24)
	}
	bTex := p37CRegionOfImage(t, imgB)
	got := draw(1.0)
	want := mixOverWhite(bTex, 255)
	if n, worst := p37BDiff(got, want); n > 0 && worst > 1 {
		t.Fatalf("mix_level=1 material != second texture: %d differing channels, worst %d — SetImage 槽位没绑到位", n, worst)
	}
}

func p37CRegionOfImage(t *testing.T, img *render.ImageBuf) []byte {
	t.Helper()
	pm := img.PremultipliedData()
	out := make([]byte, 0, 24*24*4)
	for py := 0; py < 24; py++ {
		for px := 0; px < 24; px++ {
			o := (py*24 + px) * 4
			out = append(out, pm[o], pm[o+1], pm[o+2], pm[o+3])
		}
	}
	return out
}

// mixOverWhite 把 premul 纹理素对白底合成一次（out = src + 255 - a，整数精确）。
func mixOverWhite(src []byte, _ uint8) []byte {
	out := make([]byte, len(src))
	for i := 0; i+3 < len(src); i += 4 {
		a := int(src[i+3])
		out[i] = uint8(int(src[i]) + 255 - a)
		out[i+1] = uint8(int(src[i+1]) + 255 - a)
		out[i+2] = uint8(int(src[i+2]) + 255 - a)
		out[i+3] = 255
	}
	return out
}

// TestP37CBigUniformMaterialPixel 大 uniform（272B）材质像素钉：u.split=0.5 ⇒
// 左半 ≡ 纹理 A、右半 ≡ 纹理 B（预算放开 + 大块布局真下发）。
func TestP37CBigUniformMaterialPixel(t *testing.T) {
	requireFilterOpsGPU(t)
	imgA := p37CChecker(24, 24, 1)
	imgB := p37CChecker(24, 24, 2)

	e, err := render.NewShaderEffect(p37CSplitWGSL)
	if err != nil {
		t.Fatalf("272B uniform effect: %v", err)
	}
	defer e.Close()
	uniform := make([]byte, 272)
	uniform[0] = 0
	uniform[1] = 0
	uniform[2] = 0
	uniform[3] = 63 // 0.5 f32 LE (0x3F000000)
	if err := e.SetUniformBytes(uniform); err != nil {
		t.Fatalf("SetUniformBytes 272B: %v", err)
	}
	if err := e.SetImageAt(1, imgB); err != nil {
		t.Fatal(err)
	}
	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.DrawImageEx(imgA, render.DrawImageOptions{X: 4, Y: 4, DstWidth: 24, DstHeight: 24, Material: e})
	if err := dc.Fill(); err != nil {
		t.Fatal(err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	left := p37CRegion(t, dc, 4, 4, 12, 24)
	right := p37CRegion(t, dc, 16, 4, 12, 24)
	aTex := p37CRegionOfImageHalf(t, imgA, 0, 12)
	bTex := p37CRegionOfImageHalf(t, imgB, 12, 24)
	if n, worst := p37BDiff(left, mixOverWhite(aTex, 255)); n > 0 && worst > 1 {
		t.Fatalf("left half != texture A: %d channels worst %d", n, worst)
	}
	if n, worst := p37BDiff(right, mixOverWhite(bTex, 255)); n > 0 && worst > 1 {
		t.Fatalf("right half != texture B: %d channels worst %d", n, worst)
	}
}

func p37CRegionOfImageHalf(t *testing.T, img *render.ImageBuf, x0, x1 int) []byte {
	t.Helper()
	pm := img.PremultipliedData()
	out := make([]byte, 0, 12*24*4)
	for py := 0; py < 24; py++ {
		for px := x0; px < x1; px++ {
			o := (py*24 + px) * 4
			out = append(out, pm[o], pm[o+1], pm[o+2], pm[o+3])
		}
	}
	return out
}

// TestP37CFillPathMultiTexture 填充路多纹理：同 shader 走 SetShader 填充路
// （SetImage=slot0 + SetImageAt(1)），mix_level=1 ⇒ 画面 ≡ 纹理 B。
func TestP37CFillPathMultiTexture(t *testing.T) {
	requireFilterOpsGPU(t)
	imgB := p37CChecker(24, 24, 2)

	e, err := render.NewShaderEffect(p37CMixWGSL)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.SetUniformBytes([]byte{0, 0, 128, 63, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := e.SetImageAt(1, imgB); err != nil {
		t.Fatal(err)
	}
	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetShader(e)
	dc.DrawRectangle(4, 4, 24, 24)
	if err := dc.Fill(); err != nil {
		t.Fatal(err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	got := p37CRegion(t, dc, 4, 4, 24, 24)
	want := mixOverWhite(p37CRegionOfImage(t, imgB), 255)
	if n, worst := p37BDiff(got, want); n > 0 && worst > 1 {
		t.Fatalf("fill path multi-texture: %d channels worst %d != texture B", n, worst)
	}
}
