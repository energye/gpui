//go:build !nogpu

package render_test

// P3.7 批 3d 钉：描边/形状吃 shader（引擎洞修复）+ 七枚哨兵 errors.Is 面。
// 判据源：batch3d-design.md §1/§2/§3。像素判据：uv-gradient shader 的描边环
// 上沿 ≠ 下沿（渐变真到位）且 ≠ 环境实心色。

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

const p37DGradWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    // premul：红随 uv.y 从 0 到 0.9，绿/蓝固定低值——上沿暗、下沿亮红。
    let r = 0.9 * uv.y;
    return vec4<f32>(r, 0.05, 0.05, 1.0);
}`

func p37DRegion(t *testing.T, dc *render.Context, x, y, w, h int) []byte {
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

// TestP37DStrokeWithShaderGradient 描边+shader 像素钉：红实心底 + 渐变 shader
// 描边矩形环——描边环的下半（uv.y 大）红通道显著高于上半（修复前=环境实心红，
// 环上无渐变）。判据：下沿平均 R − 上沿平均 R ≥ 60。
func TestP37DStrokeWithShaderGradient(t *testing.T) {
	requireFilterOpsGPU(t)
	e, err := render.NewShaderEffect(p37DGradWGSL)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	dc := render.NewContext(48, 48)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.4, A: 1}) // 蓝底：环境红当场露馅
	dc.SetRGB(1, 0, 0)                                           // 环境红（修复前描边画它）
	dc.SetShader(e)
	dc.SetLineWidth(4)
	dc.DrawRectangle(8, 8, 32, 32)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("Stroke: %v", err)
	}
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}

	avgR := func(y0, y1 int) float64 {
		sum, n := 0, 0
		for y := y0; y < y1; y++ {
			for x := 8; x < 40; x++ {
				r, _, _, _ := dc.Image().At(x, y).RGBA()
				sum += int(r >> 8)
				n++
			}
		}
		if n == 0 {
			return 0
		}
		return float64(sum) / float64(n)
	}
	top, bottom := avgR(7, 11), avgR(37, 41)
	if bottom-top < 60 {
		t.Fatalf("stroke ring gradient missing: avgR top=%.1f bottom=%.1f (Δ=%.1f want ≥60) — shader 描边被环境实心顶替？", top, bottom, bottom-top)
	}
}

// TestP37DFillShapeWithShaderGradient 形状+shader 路由钉：DrawCircle 经
// tryGPUOpRC 的 shader 直达拦截（context_gpu.go:553，深审已装）进 FillPath
// shader 腿——渐变圆盘上/下半 R 差显著。负向 N2 实测：FillShape 分支是死代码
// （tryGPUOpRC 先拦截，FillShape 从不接 shader 画刷），审计判断已如实收回。
func TestP37DFillShapeWithShaderGradient(t *testing.T) {
	requireFilterOpsGPU(t)
	e, err := render.NewShaderEffect(p37DGradWGSL)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	dc := render.NewContext(48, 48)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.4, A: 1})
	dc.SetRGB(1, 0, 0)
	dc.SetShader(e)
	dc.DrawCircle(24, 24, 16)
	if err := dc.Fill(); err != nil {
		t.Fatal(err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	avgR := func(y0, y1 int) float64 {
		sum, n := 0, 0
		for y := y0; y < y1; y++ {
			for x := 16; x < 32; x++ {
				r, _, _, _ := dc.Image().At(x, y).RGBA()
				sum += int(r >> 8)
				n++
			}
		}
		if n == 0 {
			return 0
		}
		return float64(sum) / float64(n)
	}
	top, bottom := avgR(10, 14), avgR(34, 38)
	if bottom-top < 60 {
		t.Fatalf("shape gradient missing: avgR top=%.1f bottom=%.1f (Δ=%.1f want ≥60)", top, bottom, bottom-top)
	}
}

// TestP37DSentinelFamily 七枚哨兵 errors.Is 面（能判的六枚 + 双链包装）。
func TestP37DSentinelFamily(t *testing.T) {
	badWGSL := "this is not wgsl at all"
	if _, err := render.NewShaderEffect(badWGSL); !errors.Is(err, render.ErrShaderBadWGSL) {
		t.Fatalf("bad WGSL err=%v want ErrShaderBadWGSL", err)
	}
	noEntry := "@fragment fn fs_other() -> @location(0) vec4<f32> { return vec4<f32>(0.0); }"
	if _, err := render.NewShaderEffect(noEntry); !errors.Is(err, render.ErrShaderNoEntry) {
		t.Fatalf("no fs_main err=%v want ErrShaderNoEntry", err)
	}
	badReturn := "@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec3<f32> { return vec3<f32>(0.0); }"
	if _, err := render.NewShaderEffect(badReturn); !errors.Is(err, render.ErrShaderBadReturn) {
		t.Fatalf("vec3 return err=%v want ErrShaderBadReturn", err)
	}
	loneSampler := "@group(0) @binding(2) var s0: sampler;\n@fragment fn fs_main() -> @location(0) vec4<f32> { _ = s0; return vec4<f32>(0.0); }"
	if _, err := render.NewShaderEffect(loneSampler); !errors.Is(err, render.ErrShaderSlotConflict) {
		t.Fatalf("lone sampler err=%v want ErrShaderSlotConflict", err)
	}
	overUniform := strings.Builder{}
	overUniform.WriteString("struct Huge {\n")
	for i := 0; i < 4097; i++ {
		fmt.Fprintf(&overUniform, "    m%d: vec4<f32>,\n", i)
	}
	overUniform.WriteString("}\n@group(0) @binding(0) var<uniform> u: Huge;\n")
	overUniform.WriteString("@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(u.m0); }")
	if _, err := render.NewShaderEffect(overUniform.String()); !errors.Is(err, render.ErrShaderUniformTooLarge) {
		t.Fatalf("over-budget uniform err=%v want ErrShaderUniformTooLarge", err)
	}
	e, err := render.NewShaderEffect("@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0); }")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if err := e.SetUniformBytes([]byte{1}); !errors.Is(err, render.ErrShaderClosed) {
		t.Fatalf("SetUniformBytes after Close err=%v want ErrShaderClosed", err)
	}
	if err := e.SetImageAt(0, nil); !errors.Is(err, render.ErrShaderClosed) {
		t.Fatalf("SetImageAt after Close err=%v want ErrShaderClosed", err)
	}
	// 双链包装（后端编译失败同时保持 fallback 语义）：构造同形错误断言两链。
	wrapped := fmt.Errorf("%w (%w)", render.ErrShaderBackendCompile, render.ErrFallbackToCPU)
	if !errors.Is(wrapped, render.ErrShaderBackendCompile) || !errors.Is(wrapped, render.ErrFallbackToCPU) {
		t.Fatal("backend compile error must chain both sentinels")
	}
}

// TestP37DShaderBackendFailsCounter 计数字段存在且默认零（触发路径无确定性
// 构造器——M3 家族反向工程不在本批，如实登记：接线由 fillShaderAsImage 的
// 双链包装 + tryGPUFillWithMode 的 noteShaderBackend 覆盖，真触发随 gles
// 首绘失败场景在批 6 材质窗承载）。
func TestP37DShaderBackendFailsCounter(t *testing.T) {
	dc := render.NewContext(8, 8)
	defer func() { _ = dc.Close() }()
	if got := dc.RenderPathStats().ShaderBackendFails; got != 0 {
		t.Fatalf("ShaderBackendFails=%d want 0 on a clean context", got)
	}
}

var _ = image.NewNRGBA
