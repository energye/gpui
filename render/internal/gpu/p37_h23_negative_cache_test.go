//go:build !nogpu

package gpu

import (
	"errors"
	"testing"

	"github.com/energye/gpui/render"
)

const p37H23BadWGSL = `
struct BadAlign {
    vals: array<f32, 64>,
}
@group(0) @binding(0) var<uniform> u: BadAlign;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let v = u.vals[0];
    return vec4<f32>(uv.x, uv.y, 0.5, 1.0) + vec4<f32>(v, v, v, 0.0);
}`

const p37H23GoodWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x, uv.y, 0.5, 1.0);
}`

func TestP37H23NegativeCacheStopsDeviceRetries(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()
	clip, err := createClipBindGroupLayout(device, "p37h23_test_clip")
	if err != nil {
		t.Fatalf("clip layout: %v", err)
	}
	defer clip.Destroy()
	s.clipBindLayout = clip

	bad, err := render.NewShaderEffect(p37H23BadWGSL)
	if err != nil {
		t.Fatalf("前置事实：本机前端应放行这枚模块（后端才拒），got %v", err)
	}
	defer func() { _ = bad.Close() }()
	good, err := render.NewShaderEffect(p37H23GoodWGSL)
	if err != nil {
		t.Fatalf("good effect: %v", err)
	}
	defer func() { _ = good.Close() }()

	const frames = 3
	for i := 1; i <= frames; i++ {
		if _, err := s.ensureMaterialPipeline(bad, false); err == nil {
			t.Fatalf("第 %d 帧：被拒模块居然报可用", i)
		} else if !errors.Is(err, render.ErrShaderBackendCompile) {
			t.Fatalf("第 %d 帧：错误不具名：%v", i, err)
		}
	}
	if got := s.MaterialRejectAttempts(); got != 1 {
		t.Errorf("负面缓存失效：%d 帧打了 %d 次设备编译（want 1，重复帧应只读缓存）", frames, got)
	}
	// 对照：合法模块必须照常真建一次，不许被前面的拒绝连带误拒。
	if _, err := s.ensureMaterialPipeline(good, false); err != nil {
		t.Fatalf("合法模块被误拒：%v", err)
	}
	if got := s.MaterialRejectAttempts(); got != 2 {
		t.Errorf("合法模块应真建一次：attempts=%d want 2", got)
	}
}
