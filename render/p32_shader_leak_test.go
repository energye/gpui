//go:build !nogpu

package render_test

import (
	"runtime"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// 深审 L2-P0-1 钉：ShaderEffect 的 GPU 管线缓存必须随 Close/上下文销毁退干净。
// 旧状 userShaderCache.entries 只插不删（唯二清理点：设备切换/无），
// Close() 只置标志；多轮 建→画→关 显存与句柄线性增长。
const shaderWGSL = `
@fragment fn fs_main() -> @location(0) vec4<f32> {
	return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`

func runOneShaderEffectRound(t *testing.T) {
	t.Helper()
	dc := render.NewContext(64, 64)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	e, err := render.NewShaderEffect(shaderWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	if err := e.SetFloatUniforms([]float32{1, 2, 3, 4}); err != nil {
		t.Fatalf("uniforms: %v", err)
	}
	dc.SetShader(e)
	dc.SetRGB(0, 0, 0)
	dc.DrawRectangle(4, 4, 32, 32)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU after fill: %v", err)
	}
	if err := e.Close(); err != nil {
		t.Fatalf("effect Close: %v", err)
	}
	if err := dc.Close(); err != nil {
		t.Fatalf("context close: %v", err)
	}
	runtime.GC()
}

func TestP32ShaderEffectNoLeakAcrossRounds(t *testing.T) {
	requireFilterOpsGPU(t)
	const rounds = 6

	// 建立基线：一轮预热（管线/共享资源首建不计入增长）。
	runOneShaderEffectRound(t)
	baseCount := hal.VramLiveCount()
	baseBytes := hal.VramLiveBytes()

	for i := 1; i < rounds; i++ {
		runOneShaderEffectRound(t)
		if c, b := hal.VramLiveCount(), hal.VramLiveBytes(); c > baseCount+2 || b > baseBytes+64*1024 {
			t.Fatalf("round %d leaks: live handles %d->%d, bytes %d->%d（ShaderEffect 的 GPU 管线缓存未随 Close/上下文销毁驱逐）",
				i, baseCount, c, baseBytes, b)
		}
	}
	c, b := hal.VramLiveCount(), hal.VramLiveBytes()
	t.Logf("steady after %d rounds: handles=%d (base %d), bytes=%d (base %d)", rounds, c, baseCount, b, baseBytes)
}

// Close 之后：访问器拒用、再画不回灌缓存（驱逐必须粘住）。
func TestP32ShaderEffectClosedRefusal(t *testing.T) {
	requireFilterOpsGPU(t)
	e, err := render.NewShaderEffect(shaderWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	if err := e.SetUniformBytes([]byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("uniform before close: %v", err)
	}
	dc := render.NewContext(32, 32)
	dc.SetShader(e)
	dc.SetRGB(0, 0, 0)
	dc.DrawRectangle(2, 2, 16, 16)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	if err := e.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !e.Closed() {
		t.Fatal("Closed() must report true after Close")
	}
	if err := e.Close(); err != nil {
		t.Fatalf("Close must be idempotent: %v", err)
	}
	if err := e.SetUniformBytes([]byte{9}); err == nil {
		t.Fatal("SetUniformBytes after Close must be refused")
	}
	e.SetImage(nil) // 不应 panic；closed 下静默拒
	// 已关 effect 再画：管线不回灌，句柄/字节不涨。
	before := hal.VramLiveCount()
	dc.SetShader(e)
	dc.DrawRectangle(2, 2, 16, 16)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	runtime.GC()
	if after := hal.VramLiveCount(); after > before {
		t.Fatalf("closed effect re-populated GPU caches: handles %d -> %d", before, after)
	}
	if err := dc.Close(); err != nil {
		t.Fatalf("ctx close: %v", err)
	}
}

// 进程内第一个 GPU op 就是 shader 绘制（-run 单跑时成立）：lazy 设备初始化
// 必须接住它（L2-P0-2：旧状 device==nil → 落 CPU 画环境色）。
func TestP32ShaderLazyFirstGPUOp(t *testing.T) {
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0) // ambient 红：画漏必现红
	e, err := render.NewShaderEffect(`
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0, 1.0, 0.0, 1.0); }
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer e.Close()
	dc.SetShader(e)
	dc.DrawRectangle(8, 8, 32, 32)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	st := dc.RenderPathStats()
	if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("first GPU op was not the shader draw: %s", st.LogLine())
	}
}

// 深审 D4 GPU 腿钉：GPU 档 DrawPaint advanced+Linear 必须线性域合成
// （layer→queue→dual-tex 链旧状两处硬写 false；CPU 档由 TestDrawPaintLinearModeSet 守）。
// 覆盖注记：headless 读回经 pending 层的 CPU resolve 腿（负向摘 pl.linear 实测红 76）；
// 真 GPU dual-tex 腿由真窗 ui_wr_p31_linear_blend 的 present-View 路径守（窗口轮复核）。
func TestP31DrawPaintLinearGPUDomain(t *testing.T) {
	requireFilterOpsGPU(t)
	dc := render.NewContext(16, 16)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 0.8, G: 0.8, B: 0.8, A: 1})
	dc.SetRGB(0.5, 0.5, 0.5)
	dc.SetBlendMode(render.BlendDifference)
	dc.SetBlendSpace(render.BlendSpaceLinear)
	dc.DrawPaint()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	r, _, _, _ := dc.Image().At(8, 8).RGBA()
	got := int(r >> 8)
	if d := got - 168; absInt(d) > 3 {
		t.Fatalf("GPU DrawPaint Difference+linear R=%d want 168±3（layer 链退回 sRGB 时为 76）", got)
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
