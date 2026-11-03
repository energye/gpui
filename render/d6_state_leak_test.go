//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"testing"
)

// 深审 D6 钉（L1-P1-3 ∪ L2-P3-9）：便捷绘制入口不得把一次性合成状态漏给调用方。
// 旧状 DrawImageEx 注释声称「Save current state (paint…)」但 Push/Pop 不保存 paint
// （本仓 save/restore 的既定语义不含 paint），DrawShaderMask 手抄还原只回滚
// Brush/Blend，漏 BlendSpace 与 Shader——ambient shader 还会在掩码期劫持 mask fill。

func TestD6DrawImageExNoAmbientLeak(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(16, 16)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(White)
	dc.SetRGB(0, 0, 1)
	img, err := NewImageBuf(4, 4, FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer img.Dispose()
	dc.DrawImageEx(img, DrawImageOptions{
		X: 2, Y: 2, DstWidth: 8, DstHeight: 8,
		BlendMode: BlendDifference, BlendSpace: BlendSpaceLinear,
	})
	if got := dc.BlendMode(); got != BlendNormal {
		t.Fatalf("ambient BlendMode leaked: %d, want BlendNormal", got)
	}
	if got := dc.BlendSpace(); got != BlendSpaceSRGB {
		t.Fatalf("ambient BlendSpace leaked: %d, want SRGB", got)
	}
	if dc.paint.Pattern != nil || !dc.paint.isSolid {
		t.Fatalf("ambient fill source leaked: pattern=%v isSolid=%v", dc.paint.Pattern != nil, dc.paint.isSolid)
	}
	// 之后的正常绘制必须仍以蓝色 solid 进行。
	dc.DrawRectangle(0, 0, 2, 2)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	r, g, b, _ := pixelAt(dc.PixmapForTest(), 1, 1)
	if r > 40 || g > 40 || b < 200 {
		t.Fatalf("post-call fill color drifted: (%d,%d,%d), want blue", r, g, b)
	}
}

func TestD6DrawShaderMaskNoShaderOrSpaceLeak(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(16, 16)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(White)
	ambient, err := NewShaderEffect(`
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0, 0.0, 0.0, 1.0); }
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer ambient.Close()
	other, err := NewShaderEffect(`
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0, 0.0, 1.0, 1.0); }
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer other.Close()
	dc.SetShader(ambient)

	mask := SolidBrush{Color: RGBA{R: 1, G: 1, B: 1, A: 0.5}}
	DrawShaderMask(dc, 2, 2, 10, 10, mask, func() {
		dc.SetRGB(1, 0, 0)
		dc.SetBlendSpace(BlendSpaceLinear)
		dc.SetShader(other) // 回调改 shader：出掩码必须回到 ambient
		dc.DrawRectangle(3, 3, 6, 6)
		_ = dc.Fill()
	})
	if got := dc.Shader(); got != ambient {
		t.Fatalf("ambient Shader leaked: got %p want ambient %p", got, ambient)
	}
	if got := dc.BlendSpace(); got != BlendSpaceSRGB {
		t.Fatalf("ambient BlendSpace leaked: %d, want SRGB", got)
	}
}

func TestD6DrawShaderMaskAmbientShaderDoesNotHijack(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(16, 16)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(White)
	e, err := NewShaderEffect(`
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0, 1.0, 0.0, 1.0); }
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer e.Close()
	dc.SetShader(e)
	// 修复语义：掩码子层内 shader 临时摘除——回调的刷子绘制与 mask fill 正常发生，
	// 结果 = 半透明红进白底（R 显著 > 0）。旧状 shader 劫持两笔 fill → 全白。
	mask := SolidBrush{Color: RGBA{R: 1, G: 1, B: 1, A: 0.5}}
	DrawShaderMask(dc, 2, 2, 10, 10, mask, func() {
		dc.SetRGB(1, 0, 0)
		dc.DrawRectangle(3, 3, 6, 6)
		_ = dc.Fill()
	})
	r, _, b, _ := pixelAt(dc.PixmapForTest(), 6, 6)
	if r < 100 || r-b < 50 {
		t.Fatalf("inside-mask pixel (%d,_,%d), want reddish（ambient shader 劫持了掩码/回调绘制，白底假绿已排除）", r, b)
	}
}
