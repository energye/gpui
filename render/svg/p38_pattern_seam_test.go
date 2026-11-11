//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image"
	"os"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

// TestP38PatternTileSeamGPU 是 tilePatt 格边竖缝的隔离红测（P3.8 批②专项）。
//
// 现象：最小 pattern（patternUnits=userSpaceOnUse tile 48×48 +
// patternContentUnits=objectBoundingBox 四象限）在被填 rect 上平铺时，
// GPU 会话活跃的进程里绿带每格首列出现一条暗竖缝（设备 scale≈0.557 时
// 周期≈26.7px；scale=1 时周期=48px）。纯 CPU 进程不可见——tile 离屏
// Context 未带 WithCPUOnly，窗口里被 GPU 光栅化，左边缘 AA 覆盖不足
// （实测列 0 alpha=236/255），平铺采样到该列即变暗。
//
// 本测需要真实 GPU 设备（WGPU_NATIVE_PATH），与 render/conformance_test.go
// 同口径：无库/无设备时 skip，不静默假绿。
func TestP38PatternTileSeamGPU(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH unset: native wgpu library path required")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	inst, err := webgpu.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		t.Skipf("webgpu CreateInstance unavailable: %v", err)
	}
	ad, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference: gputypes.PowerPreferenceHighPerformance,
	})
	if err != nil {
		inst.Release()
		t.Skipf("webgpu RequestAdapter unavailable: %v", err)
	}
	dev, err := ad.RequestDevice(render.DeviceDescriptor("svg-pattern-seam"))
	if err != nil {
		ad.Release()
		inst.Release()
		t.Skipf("webgpu RequestDevice unavailable: %v", err)
	}
	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: dev, Adpt: ad, Format: gputypes.TextureFormatBGRA8Unorm,
	}); err != nil {
		dev.Release()
		ad.Release()
		inst.Release()
		t.Skipf("SetDeviceProvider: %v", err)
	}
	defer func() {
		_ = rendgpu.ResetAccelerator()
		_ = dev.WaitIdle()
		dev.Release()
		ad.Release()
		inst.Release()
	}()

	doc := mustParseFile(t, "pattern_tile_seam.svg")
	cases := []struct {
		name           string
		w, h           int
		x0, x1, y0, y1 int
	}{
		// 570×357：scale≈0.5566（showcase 面板显示分辨率），绿带设备行 313–319。
		{"scale0.557", 570, 357, 385, 551, 313, 319},
		// 1024×640：scale=1 对照，绿带设备行 561–573（每 48px 一格）。
		{"scale1", 1024, 640, 685, 995, 561, 573},
	}
	for _, c := range cases {
		img := doc.Render(c.w, c.h)
		if cols := patternSeamColumns(img, c.x0, c.x1, c.y0, c.y1); len(cols) > 0 {
			t.Errorf("%s %dx%d: periodic vertical seams at device x=%v (want none); "+
				"tile offscreen must be CPU-rasterized like Document.Render",
				c.name, c.w, c.h, cols)
		}
	}
}

// patternSeamColumns reports columns whose whole band run is darker than both
// immediate neighbours (sum of RGB over rows, mean delta > 20 per row).
// A tile-boundary seam is exactly this shape; real content edges are excluded
// by the caller's x range.
func patternSeamColumns(img *image.RGBA, x0, x1, y0, y1 int) []int {
	colSum := func(x int) int {
		s := 0
		for y := y0; y <= y1; y++ {
			c := img.RGBAAt(x, y)
			s += int(c.R) + int(c.G) + int(c.B)
		}
		return s
	}
	rows := y1 - y0 + 1
	var cols []int
	for x := x0 + 1; x <= x1-1; x++ {
		c := colSum(x)
		l, r := colSum(x-1), colSum(x+1)
		if l-c > 20*rows && r-c > 20*rows {
			cols = append(cols, x)
		}
	}
	return cols
}
