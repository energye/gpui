//go:build !nogpu

package gpu

import (
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// M5 P2（evidence/M5/assessment-design.md §3 G3）：float 离屏 RT 的池键、账本、
// 能力门与具名回退。
func TestP2OffscreenFormatKeyAndBytes(t *testing.T) {
	k8 := offscreenKey(64, 48, types.TextureFormatBGRA8Unorm)
	k16 := offscreenKey(64, 48, types.TextureFormatRGBA16Float)
	if k8 == k16 {
		t.Fatal("8-bit and 16F offscreen keys must differ")
	}
	if got, want := offscreenRTBytes(k16), uint64(64*48*8); got != want {
		t.Fatalf("RGBA16Float entry bytes = %d, want %d", got, want)
	}
	if got, want := offscreenRTBytes(k8), uint64(64*48*4); got != want {
		t.Fatalf("BGRA8 entry bytes = %d, want %d", got, want)
	}
	// Zero Format keeps the historical BGRA8 size (pre-P2 cache keys).
	if got, want := offscreenRTBytes(offscreenKey(8, 8, types.TextureFormatUndefined)), uint64(8*8*4); got != want {
		t.Fatalf("zero-format entry bytes = %d, want %d", got, want)
	}
}

func newP2Shared(t *testing.T) (*GPUShared, *GPURenderContext) {
	t.Helper()
	shared := NewGPUShared()
	shared.mu.Lock()
	err := shared.initGPU()
	shared.mu.Unlock()
	if err != nil {
		t.Skipf("GPU not available: %v", err)
	}
	t.Cleanup(shared.Close)
	rc := shared.NewRenderContext()
	t.Cleanup(rc.Close)
	return shared, rc
}

func TestP2OffscreenFloatTargetAccounting(t *testing.T) {
	t.Setenv("GPUI_NO_OFFSCREEN_POOL", "1")
	shared, rc := newP2Shared(t)

	before := hal.VramLiveBytes()
	view, release, err := rc.CreateOffscreenTextureFormat(64, 64, types.TextureFormatRGBA16Float)
	if err != nil {
		t.Fatalf("create RGBA16Float offscreen: %v", err)
	}
	if view.IsNil() {
		t.Fatal("RGBA16Float offscreen view is nil")
	}
	if got, want := hal.VramLiveBytes()-before, uint64(64*64*8); got != want {
		t.Fatalf("VRAM delta = %d, want %d (16F must be charged 8 B/px)", got, want)
	}
	if got := unpackView(view).Texture().Format(); got != types.TextureFormatRGBA16Float {
		t.Fatalf("view texture format = %v, want RGBA16Float", got)
	}

	// The returned view must be usable as a render attachment.
	device, queue := shared.Device(), shared.Queue()
	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p2_clear"})
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "p2_clear_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: unpackView(view), LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 2, G: 0.5, B: 0.25, A: 1},
		}},
	})
	if err != nil {
		t.Fatalf("begin 16F pass: %v", err)
	}
	if err := rp.End(); err != nil {
		t.Fatalf("end 16F pass: %v", err)
	}
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("submit: %v", err)
	}
	device.FreeCommandBuffer(cmd)

	release()
	if got := hal.VramLiveBytes(); got != before {
		t.Fatalf("VRAM after release = %d, want baseline %d", got, before)
	}
}

func TestP2OffscreenFormatFailClosedAndFallback(t *testing.T) {
	t.Setenv("GPUI_NO_OFFSCREEN_POOL", "1")
	_, rc := newP2Shared(t)

	before := hal.VramLiveBytes()
	if _, _, err := rc.CreateOffscreenTextureFormat(32, 32, types.TextureFormatDepth24PlusStencil8); !errors.Is(err, ErrOffscreenFormatUnsupported) {
		t.Fatalf("depth format error = %v, want ErrOffscreenFormatUnsupported", err)
	}
	if got := hal.VramLiveBytes(); got != before {
		t.Fatalf("rejected format allocated VRAM: %d != %d", got, before)
	}
	if got := rc.OffscreenFormatFallbacks(); got != 0 {
		t.Fatalf("fallback counter = %d before any fallback", got)
	}

	view, release, actual := rc.CreateOffscreenTextureFormatOrFallback(32, 32, types.TextureFormatDepth24PlusStencil8)
	if view.IsNil() || release == nil {
		t.Fatal("fallback did not return an 8-bit target")
	}
	if actual != types.TextureFormatBGRA8Unorm {
		t.Fatalf("fallback format = %v, want BGRA8", actual)
	}
	if got := rc.OffscreenFormatFallbacks(); got != 1 {
		t.Fatalf("fallback counter = %d, want 1", got)
	}
	release()
}

func TestP2OffscreenPoolSeparatesFormats(t *testing.T) {
	_, rc := newP2Shared(t)
	cache, _ := rc.offscreenCacheFor()
	if cache == nil {
		t.Skip("no offscreen cache (bare context)")
	}

	v8, r8, err := rc.CreateOffscreenTextureFormat(48, 48, types.TextureFormatBGRA8Unorm)
	if err != nil || v8.IsNil() {
		t.Fatalf("8-bit offscreen: view=%v err=%v", v8, err)
	}
	v16, r16, err := rc.CreateOffscreenTextureFormat(48, 48, types.TextureFormatRGBA16Float)
	if err != nil || v16.IsNil() {
		t.Fatalf("16F offscreen: view=%v err=%v", v16, err)
	}
	if got := unpackView(v16).Texture().Format(); got != types.TextureFormatRGBA16Float {
		t.Fatalf("pooled 16F view format = %v", got)
	}
	if got := unpackView(v8).Texture().Format(); got != types.TextureFormatBGRA8Unorm {
		t.Fatalf("pooled 8-bit view format = %v", got)
	}
	if s := cache.Stats(); s.Misses < 2 {
		t.Fatalf("cache misses = %d, want >= 2 (separate keys per format)", s.Misses)
	}
	r8()
	r16()

	// Same format+size must hit the pool after release, and must hand back the
	// 16F texture (not the 8-bit one).
	hitsBefore := cache.Stats().Hits
	v16b, r16b, err := rc.CreateOffscreenTextureFormat(48, 48, types.TextureFormatRGBA16Float)
	if err != nil || v16b.IsNil() {
		t.Fatalf("re-acquire 16F: view=%v err=%v", v16b, err)
	}
	if got := unpackView(v16b).Texture().Format(); got != types.TextureFormatRGBA16Float {
		t.Fatalf("re-acquired view format = %v, want RGBA16Float", got)
	}
	if cache.Stats().Hits <= hitsBefore {
		t.Fatal("16F re-acquire did not hit the pool")
	}
	r16b()
}

// P5 全量门禁抓到的 H20 回归：能力探测若发生在 ensureGPU 之前，会把「设备还没建」
// 缓存成「格式不支持」，之后 CreateOffscreenTexture 永远返回空。本钉不预初始化设备。
func TestP5OffscreenBeforeDeviceInit(t *testing.T) {
	shared := NewGPUShared()
	t.Cleanup(shared.Close)
	rc := shared.NewRenderContext()
	t.Cleanup(rc.Close)
	view, release := rc.CreateOffscreenTexture(8, 8)
	if view.IsNil() || release == nil {
		t.Fatal("CreateOffscreenTexture before initGPU returned no target (probe poisoned the cache)")
	}
	release()
}

// The historical 8-bit entry keeps its contract (non-nil view, no error leak).
func TestP2OffscreenLegacyEntryStillWorks(t *testing.T) {
	_, rc := newP2Shared(t)
	view, release := rc.CreateOffscreenTexture(40, 40)
	if view.IsNil() || release == nil {
		t.Fatal("CreateOffscreenTexture returned no target")
	}
	if got := unpackView(view).Texture().Format(); got != types.TextureFormatBGRA8Unorm {
		t.Fatalf("legacy entry format = %v, want BGRA8", got)
	}
	release()
}
