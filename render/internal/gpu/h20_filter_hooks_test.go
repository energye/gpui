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
	"testing"

	"github.com/energye/gpui/render"
)

// TestP376FilterHooksFollowSharedLifecycle — H20 根修钉：滤镜图钩子随注册它的
// shared 生死。registerFilterGraphIfNeeded 的三个钩子闭包捕获那一份 GPUShared，
// 而包级标志从不复位——shared 被 CloseAccelerator 关掉后钩子还攥着它，下一个
// 滤镜 op 复活它并在上面装新设备，发布视图随后在读回腿喂给了当前 shared 的设备
// ⇒ wgpu 非 unwind panic（H20）。钉断三段：注册归属、Close 注销、活 shared 重挂。
//
// 负向（本轮实测）：摘掉 Close 里的 UnregisterGPUFilterGraphs/复位块 ⇒ 本钉红
// （钩子跨过 Close 存活）；此时整包钉（TestP376H20DeviceResetRunsToCompletion）
// 仍绿——closed 守卫自己能挡住那次复活，两条是纵深不是重复。
func TestP376FilterHooksFollowSharedLifecycle(t *testing.T) {
	// 清场：不依赖其他测试留下的钩子状态（顺序无关，钉自己立世界）。
	render.UnregisterGPUFilterGraphs()
	gpuFilterGraphRegistered = false

	s := NewGPUShared()
	s.registerFilterGraphIfNeeded()
	if !s.filterGraphHooksRegistered {
		t.Fatal("registerFilterGraphIfNeeded 没把 shared 标成钩子归属者")
	}
	if !render.GPUFilterGraphRegistered() {
		t.Fatal("滤镜图钩子没注册上")
	}

	// Close 必须把钩子一并注销（与 Unregister 三件套同纪律）。
	s.Close()
	if s.filterGraphHooksRegistered {
		t.Fatal("Close 之后 filterGraphHooksRegistered 还挂着")
	}
	if render.GPUFilterGraphRegistered() {
		t.Fatal("滤镜图钩子跨过 GPUShared.Close 存活（H20 根修没生效）")
	}
	if gpuFilterGraphRegistered {
		t.Fatal("内部注册标志跨过 GPUShared.Close 存活")
	}

	// 已关闭的 shared 不许再注册钩子（closed 守卫族的一员）。
	s.registerFilterGraphIfNeeded()
	if render.GPUFilterGraphRegistered() {
		t.Fatal("已关闭的 shared 又把钩子注册回去了")
	}

	// 一个新 shared 能重新注册（活 shared 的 ensureGPU 路径天然重挂）。
	s2 := NewGPUShared()
	s2.registerFilterGraphIfNeeded()
	if !render.GPUFilterGraphRegistered() {
		t.Fatal("活 shared 重新注册滤镜图钩子失败")
	}
	if !s2.filterGraphHooksRegistered {
		t.Fatal("新 shared 没把自己标成钩子归属者")
	}
	s2.Close()
	if render.GPUFilterGraphRegistered() {
		t.Fatal("第二个 shared 关闭后钩子仍存活")
	}
}

// TestP376ClosedSharedStaysClosed — H20 纵深钉：已 Close 的 GPUShared 禁止复活。
// ensureGPU/initGPU 在 closed shared 上必须返回错误且不装设备——stale RC/闭包
// 就算还在调，也拿不到设备，上层滤镜 op 收 err 优雅退 CPU。
//
// 负向（本轮实测）：摘掉三处 closed 守卫 ⇒ 本钉红（ensureGPU 在关闭的 shared 上
// 装出了设备）；此时整包钉仍绿——Close 的钩子注销自己能挡住那次复活的来路，
// 两条是纵深不是重复。
func TestP376ClosedSharedStaysClosed(t *testing.T) {
	s := NewGPUShared()
	s.Close()

	if err := s.ensureGPU(); err == nil {
		t.Fatal("ensureGPU 在已关闭的 shared 上必须返回错误（H20 复活守卫）")
	}
	if s.device != nil {
		t.Fatal("已关闭的 shared 被装上了设备（ensureGPU 路）")
	}
	if s.gpuReady {
		t.Fatal("已关闭的 shared 的 gpuReady 被重新置位")
	}

	if err := s.initGPU(); err == nil {
		t.Fatal("initGPU 在已关闭的 shared 上必须返回错误（H20 复活守卫）")
	}
	if s.device != nil {
		t.Fatal("已关闭的 shared 被装上了设备（initGPU 路）")
	}

	// closed shared 不许成为钩子归属者（复活三部曲的第三条路）。
	s.registerFilterGraphIfNeeded()
	if s.filterGraphHooksRegistered {
		t.Fatal("已关闭的 shared 注册了滤镜图钩子")
	}
}
