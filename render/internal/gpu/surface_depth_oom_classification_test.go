//go:build !nogpu

package gpu

// K6 档 2 尾注立项（2026-11-11 用户拍板「分类修正按标准做」）：深度/解析纹理
// 创建错误必须按 wgpu 语义分类——只有真 OOM（render.IsGPUOutOfMemory）可以走
// OOM 恢复路（1x1 降级 / 终态锁存）；`resource already released` 等
// validation error 一律显式报错（fail-closed，对齐 M6「未知格式 fail-closed」
// 先例）。此前任何深度创建错误都被包成 surfaceDepthOOMError → 静默降 1x1
// 深度 → stencil 退化 → 88 测试连锁画空（evidence/render_full_run_reds.md
// 「定性结论」段）。

import (
	"errors"
	"strings"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// failingTextureDevice wraps a real hal.Device, failing CreateTexture for one
// format with a given error. failFullOnly restricts the failure to non-1x1
// sizes (full-size probe), letting the degraded 1x1 attempt succeed.
type failingTextureDevice struct {
	hal.Device
	failFormat   types.TextureFormat
	err          error
	failFullOnly bool
}

func (d *failingTextureDevice) CreateTexture(desc *hal.TextureDescriptor) (hal.Texture, error) {
	if d.err != nil && desc.Format == d.failFormat && (!d.failFullOnly || desc.Size.Width > 1) {
		return nil, d.err
	}
	return d.Device.CreateTexture(desc)
}

func TestSurfaceDepth_NonOOMFailsClosed(t *testing.T) {
	dev, _, _ := newSurfaceCacheForTest(t, 0, 0)
	wrapped := &failingTextureDevice{
		Device:     dev,
		failFormat: types.TextureFormatDepth24PlusStencil8,
		err:        errors.New("wgpu: resource already released"),
	}
	trio, err := createSurfaceTrio(wrapped, surfaceKey(64, 32, 1, false, false), "t_nonoom")
	if err == nil {
		t.Fatal("non-OOM depth creation error must surface explicitly (K6 fail-closed)")
	}
	if trio != nil {
		t.Fatal("no trio may be returned on failure")
	}
	var oom *surfaceDepthOOMError
	if errors.As(err, &oom) {
		t.Fatalf("validation error must NOT be classified as depth OOM (would silently degrade to 1x1): %v", err)
	}
	if !strings.Contains(err.Error(), "resource already released") {
		t.Fatalf("error must carry the original cause, got: %v", err)
	}
}

func TestSurfaceDepth_OOMStillDegrades(t *testing.T) {
	dev, _, _ := newSurfaceCacheForTest(t, 0, 0)
	wrapped := &failingTextureDevice{
		Device:     dev,
		failFormat: types.TextureFormatDepth24PlusStencil8,
		err:        errors.New("wgpu: device out of memory"),
	}
	_, err := createSurfaceTrio(wrapped, surfaceKey(64, 32, 1, false, false), "t_oom")
	var oom *surfaceDepthOOMError
	if !errors.As(err, &oom) {
		t.Fatalf("genuine OOM must still take the 1x1 degradation path, got: %v", err)
	}
}

func TestSurfaceDepth_NonOOMTerminal1x1FailsClosed(t *testing.T) {
	dev, _, _ := newSurfaceCacheForTest(t, 0, 0)
	wrapped := &failingTextureDevice{
		Device:     dev,
		failFormat: types.TextureFormatDepth24PlusStencil8,
		err:        errors.New("wgpu: resource already released"),
	}
	_, err := createSurfaceTrio(wrapped, surfaceKey(64, 32, 1, false, true), "t_deg_nonoom")
	var term *surfaceTerminalOOMError
	if errors.As(err, &term) {
		t.Fatalf("non-OOM 1x1 failure must not latch an OOM-flavored terminal: %v", err)
	}
	if err == nil {
		t.Fatal("1x1 failure must surface an error")
	}
}

func TestSurfaceDepth_OOM1x1StillTerminal(t *testing.T) {
	dev, _, _ := newSurfaceCacheForTest(t, 0, 0)
	wrapped := &failingTextureDevice{
		Device:     dev,
		failFormat: types.TextureFormatDepth24PlusStencil8,
		err:        errors.New("wgpu: out of memory"),
	}
	_, err := createSurfaceTrio(wrapped, surfaceKey(64, 32, 1, false, true), "t_deg_oom")
	var term *surfaceTerminalOOMError
	if !errors.As(err, &term) {
		t.Fatalf("OOM on the 1x1 fallback must still latch terminal OOM, got: %v", err)
	}
}

func TestSurfaceResolve_NonOOMFailsClosed(t *testing.T) {
	dev, _, _ := newSurfaceCacheForTest(t, 0, 0)
	wrapped := &failingTextureDevice{
		Device:     dev,
		failFormat: types.TextureFormatBGRA8Unorm,
		err:        errors.New("wgpu: resource already released"),
	}
	_, err := createSurfaceTrio(wrapped, surfaceKey(64, 32, 1, true, false), "t_resolve_nonoom")
	var term *surfaceTerminalOOMError
	if errors.As(err, &term) {
		t.Fatalf("non-OOM resolve failure must not latch terminal OOM: %v", err)
	}
	if err == nil {
		t.Fatal("resolve failure must surface an error")
	}
}

// TestTextureSet_EnsureDegradLatchOnlyOnOOM pins the owner-level contract: a
// non-OOM depth failure never enters the degraded 1x1 window; a genuine OOM
// does (the P1.6/OOM recovery semantics stay intact).
func TestTextureSet_EnsureDegradLatchOnlyOnOOM(t *testing.T) {
	dev, _, _ := newSurfaceCacheForTest(t, 0, 0)

	oomDev := &failingTextureDevice{
		Device:       dev,
		failFormat:   types.TextureFormatDepth24PlusStencil8,
		err:          errors.New("wgpu: device out of memory"),
		failFullOnly: true,
	}
	ts := &textureSet{}
	if err := ts.ensure(oomDev, 64, 32, "t_ensure_oom", false, 1); err != nil {
		t.Fatalf("OOM full-size depth must degrade to 1x1 and succeed: %v", err)
	}
	if !ts.degradedTrio {
		t.Fatal("genuine OOM must latch the degraded 1x1 window")
	}
	ts.destroyTextures()

	nonOOMDev := &failingTextureDevice{
		Device:       dev,
		failFormat:   types.TextureFormatDepth24PlusStencil8,
		err:          errors.New("wgpu: resource already released"),
		failFullOnly: true,
	}
	ts2 := &textureSet{}
	if err := ts2.ensure(nonOOMDev, 64, 32, "t_ensure_nonoom", false, 1); err == nil {
		t.Fatal("non-OOM depth failure must surface explicitly")
	}
	if ts2.degradedTrio {
		t.Fatal("non-OOM failure must not enter the degraded 1x1 window")
	}
}
