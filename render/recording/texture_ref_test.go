//go:build !nogpu

// P4.3 治根 · 通用修法先红后绿（2026-11-10）。
//
// 根因（见 evidence/P4.3/root-cause.md）：render.Canvas 没有「设备自有纹理
// 绘制」的一等能力，Canvas 命令实现内部旁路的 GPU 纹理绘制不进录制流，
// live/present 帧静默丢内容。修法对齐 Skia GrSurfaceProxy / Flutter ui.Image：
// 引入**可录制的逻辑引用** render.TextureRef（不含设备指针），Canvas 一等
// 命令 DrawTextureRef 记录引用，回放侧按引用解析成当前设备的真实视图。
//
// 本测试先红（API 不存在→编译红），实现后要求：
//  1. 直画与「录制→回放」逐位一致，且都真的画出纹理（不是白底）；
//  2. 命令能穿过 Playback 折叠进外层录制（对齐既有 GPUTexture 命令契约）；
//  3. 引用释放后回放安全跳过（负向，不假装支持）。
package recording_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/recording"
)

const trW, trH = 16, 16

// trBluePlate 造一张纯蓝不透明离屏纹理（GPU 内容 + 读回校验口径同
// p43_upload_plate_probe_test.go）。src 生命周期交调用者持有。
func trBluePlate(t *testing.T) (*render.Context, render.TextureView, func()) {
	t.Helper()
	src := render.NewContext(trW, trH)
	src.ClearWithColor(render.RGBA{B: 1, A: 1})
	src.SetRGB(0, 0, 1)
	src.DrawRectangle(0, 0, trW, trH)
	_ = src.Fill()
	view, release := src.CreateOffscreenTexture(trW, trH)
	if view.IsNil() || release == nil {
		src.Close()
		t.Skip("CreateOffscreenTexture unavailable")
	}
	if err := src.FlushGPUWithView(view, uint32(trW), uint32(trH)); err != nil {
		release()
		src.Close()
		t.Skipf("FlushGPUWithView unavailable: %v", err)
	}
	return src, view, release
}

func trPixel(img image.Image, x, y int) [4]uint8 {
	r, g, b, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

// TestTextureRefReplayParity：同一 TextureRef 在「直画」与「录制→回放」两条
// 路径上必须逐位一致，且都真的呈现纹理（缺绘制=本批要修的洞）。
func TestTextureRefReplayParity(t *testing.T) {
	src, view, release := trBluePlate(t)
	defer src.Close()
	defer release()

	ref := render.RegisterTextureRef(func() render.TextureView { return view })
	defer render.ReleaseTextureRef(ref)

	direct := render.NewContext(trW, trH)
	defer direct.Close()
	direct.ClearWithColor(render.White)
	direct.DrawTextureRef(ref, 0, 0, trW, trH)
	if err := direct.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}

	rec := recording.NewRecorder(trW, trH)
	rec.ClearWithColor(render.White)
	rec.DrawTextureRef(ref, 0, 0, trW, trH)
	disp := rec.FinishRecording()
	replay := render.NewContext(trW, trH)
	defer replay.Close()
	if err := disp.Playback(recording.NewCanvasBackend(replay)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	if err := replay.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}

	di, ri := direct.Image(), replay.Image()
	// 容差显式声明：直画与回放走同一条 GPU 纹理 quad，边缘 bilinear 采样在
	// 两次独立 Context/Submit 下有 ±1/255 的固有残差（P4.1 批 4 补正 3 的
	// replay parity 同样用逐通道容差 2）。这里收紧到 1，且逐像素全幅检查
	// （不用 mean），任何 >1 的结构性差异立即红。
	const channelTol = 1
	maxDiff := 0
	for y := 0; y < trH; y++ {
		for x := 0; x < trW; x++ {
			a, b := trPixel(di, x, y), trPixel(ri, x, y)
			for i := 0; i < 4; i++ {
				d := int(a[i]) - int(b[i])
				if d < 0 {
					d = -d
				}
				if d > maxDiff {
					maxDiff = d
				}
			}
		}
	}
	t.Logf("max channel diff = %d (tolerance %d)", maxDiff, channelTol)
	if maxDiff > channelTol {
		t.Fatalf("replay parity: max channel diff %d > %d", maxDiff, channelTol)
	}
	// 两条路径都必须真的画出蓝色纹理（缺绘制 = 白底，不是「差一点」）。
	for _, img := range []image.Image{di, ri} {
		got := trPixel(img, 8, 8)
		if got[2] < 250 || got[0] > 8 || got[1] > 8 {
			t.Fatalf("texture not drawn at center: got %v want blue", got)
		}
	}
}

// TestTextureRefCommandSurvivesPlayback：引用是不透明逻辑 ID，命令必须能
// 穿过 Playback 折叠进外层录制（对齐 DrawGPUTextureCommand 既有契约）。
func TestTextureRefCommandSurvivesPlayback(t *testing.T) {
	ref := render.RegisterTextureRef(func() render.TextureView { return render.TextureView{} })
	defer render.ReleaseTextureRef(ref)

	inner := recording.NewRecorder(100, 100)
	inner.DrawTextureRef(ref, 4, 5, 16, 12)
	i := inner.FinishRecording()
	if got := i.PaintOpCount(); got != 1 {
		t.Fatalf("inner ops = %d, want 1", got)
	}
	if got, want := i.Bounds(), image.Rect(4, 5, 20, 17); !got.Eq(want) {
		t.Errorf("inner bounds = %v, want %v", got, want)
	}
	if i.NeedsCPUPassScratch() {
		t.Error("a texture-ref draw is GPU-only and must not request CPU scratch")
	}
	if !i.HasVisiblePaint() {
		t.Error("a valid texture ref must report visible paint")
	}

	outer := recording.NewRecorder(100, 100)
	if err := i.Playback(recording.NewCanvasBackend(outer)); err != nil {
		t.Fatalf("playback into recorder: %v", err)
	}
	o := outer.FinishRecording()
	if got := o.PaintOpCount(); got != 1 {
		t.Fatalf("outer ops = %d, want the folded texture-ref draw", got)
	}
	var found bool
	for _, cmd := range o.Commands() {
		if c, ok := cmd.(recording.DrawTextureRefCommand); ok && c.Ref == ref {
			found = true
		}
	}
	if !found {
		t.Error("folded recording lost the texture ref")
	}
}

// TestTextureRefReleasedSkipsReplay：引用释放后回放必须安全跳过（不 panic、
// 不画东西），不许静默假装支持。
func TestTextureRefReleasedSkipsReplay(t *testing.T) {
	ref := render.RegisterTextureRef(func() render.TextureView { return render.TextureView{} })
	// 先让命令真正进过录制流，再释放引用，回放才必须安全跳过。
	rec := recording.NewRecorder(16, 16)
	rec.ClearWithColor(render.White)
	rec.DrawTextureRef(ref, 0, 0, 16, 16)
	disp := rec.FinishRecording()
	render.ReleaseTextureRef(ref)
	if ref.Valid() {
		t.Fatal("released ref must be invalid")
	}
	out := render.NewContext(16, 16)
	defer out.Close()
	if err := disp.Playback(recording.NewCanvasBackend(out)); err != nil {
		t.Fatalf("playback with released ref: %v", err)
	}
	if err := out.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}
	if got := trPixel(out.Image(), 8, 8); got != [4]uint8{255, 255, 255, 255} {
		t.Fatalf("released ref drew something: got %v", got)
	}
}
