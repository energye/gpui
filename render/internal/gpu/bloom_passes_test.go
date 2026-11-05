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

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
)

// TestP375BloomPassCount 是 design §5「本批只加计数与探针、不做性能门」的计数侧：
// 一枚 bloom 节点在 GPU 图上必须正好吃 5 个 pass（原图暂存 + 亮部提取 + 可分离高斯
// H + V + 加法合成），链式节点相加。pass 数取 `lastPassUniformSlots`（一次 run 真正
// 占用的槽数 = 真正编码下来的 pass 数）。
func TestP375BloomPassCount(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	device, queue := shared.device, shared.queue
	cache := &shared.filterGPU

	const w, h = 32, 32
	src := make([]byte, w*h*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0], src[i+1], src[i+2], src[i+3] = 200, 200, 200, 255
	}
	bloom := render.ImageFilterNode{Kind: render.ImageFilterBloom, Bloom: render.BloomParams{
		Threshold: 0.5, Sigma: 1, Intensity: 1,
	}}
	blur := render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 2}

	slotsOf := func(tag string, nodes ...render.ImageFilterNode) int {
		t.Helper()
		if _, err := runGPUFilterGraph(device, queue, cache, src, w, h, nodes); err != nil {
			t.Fatalf("%s graph: %v", tag, err)
		}
		cache.mu.Lock()
		slots := cache.lastPassUniformSlots
		cache.mu.Unlock()
		t.Logf("%s pass 槽 = %d", tag, slots)
		return slots
	}

	if got := slotsOf("bloom", bloom); got < 5 {
		t.Errorf("一枚 bloom 吃了 %d 个 pass，want ≥5（原图暂存 + 提取 + H + V + 加法）", got)
	}
	base := slotsOf("blur", blur)
	if base < 2 {
		t.Errorf("一枚 blur 吃了 %d 个 pass，want ≥2", base)
	}
	chain := slotsOf("bloom+blur", bloom, blur)
	if chain < base+5 {
		t.Errorf("链式 bloom+blur 吃了 %d 个 pass，want ≥%d（两节点各自该付的都付）", chain, base+5)
	}
}
