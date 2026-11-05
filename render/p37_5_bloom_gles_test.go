//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP375BloomRunsOnGLBackend 把「一份 WGSL 双后端通用」从口号变成证据：
// 在纯 Go GL 设备（naga→GLSL 真翻译 + GL 真编译 + 真跑四 pass）上执行泛光，
// 要求①走的是 GPU 图而不是回落 CPU，②与 CPU 8-bit 那条腿在同亮度差 ≤3/255 内。
//
// 容差来源（写死）：GL 端逐抽头 exp() 与累加次序与 CPU 不同、采样器为 ClampToEdge
// 的硬件过滤，P3.4 那批 GPU vs CPU 的模糊对拍用的就是 3/255 这把尺子。
// 无 DISPLAY / GL 设备起不来 → t.Skipf（照 p1 探针口径，不造假绿）；
// Metal 本机不可验，按跨平台标记纪律在台账写明「待验」，不在这里冒充。
func TestP375BloomRunsOnGLBackend(t *testing.T) {
	dev, ad := p1GLDevice(t)
	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("GL provider 绑定失败: %v", err)
	}
	defer render.AbandonAcceleratorDevice()

	// 滤镜图注册在 GPUShared 初始化里（gpu_shared.go:1280），而后者是第一次真 GPU
	// 会话才起——先跑一笔普通绘制把它带起来，再谈图。
	warm := render.NewContext(8, 8)
	warm.SetRGBA(1, 0, 0, 1)
	warm.DrawRectangle(0, 0, 8, 8)
	_ = warm.Fill()
	_ = warm.FlushGPU()
	warm.Close()
	if !render.GPUFilterGraphRegistered() {
		t.Fatal("GL 设备下 GPU 滤镜图没注册：泛光在 GL 后端只能走 CPU，双后端一致性无从谈起")
	}

	const w, h = 24, 24
	params := render.BloomParams{Threshold: 0.4, Sigma: 1.5, Intensity: 2}

	gl := render.NewContext(w, h)
	defer gl.Close()
	bloomSpotScene(gl, w, h, 12, 12)
	gl.ResetRenderPathStats()
	gl.ApplyBloom(params)
	stats := gl.RenderPathStats()
	if stats.GPUOps == 0 || stats.CPUFallbackOps != 0 {
		t.Fatalf("GL 后端没在 GPU 图上跑泛光：%s", stats.LogLine())
	}
	glImg := gl.Image()

	cpu := render.NewContext(w, h)
	defer cpu.Close()
	bloomSpotScene(cpu, w, h, 12, 12)
	restore := render.DisableGPUFilterGraphForTest()
	cpu.ApplyBloom(params)
	restore()
	if d := filterMaxDiff(glImg, cpu.Image()); d > 3 {
		t.Fatalf("GL 与 CPU 8-bit 泛光最大通道差 %d，容差 3：一份 WGSL 两后端对不齐", d)
	}
}
