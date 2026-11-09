//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import "testing"

// TestGlesDeviceLostTracksReleased 钉住 J5 campaign 的 Linux 语义：IsLost 报告
// 「无 GL 上下文 / 已置 lost」，让 GPUShared.ensureGPU 重建设备，而不是把已
// 释放的 device 继续交给会话（见 device_linux.go 的 IsLost 与 Release）。
//
// H4-b3 的共享钉测（behavior_shared_test.go）曾假定 IsLost 恒 false，那是
// J5 之前的桩语义；J5 之后 Linux 与 Windows 出现分歧，故把平台特有值从共享
// 测试移到此处锁住。Windows 端 IsLost 仍恒 false（device.go），本机不可验，
// 按跨平台标记纪律在共享测试注释中登记。
func TestGlesDeviceLostTracksReleased(t *testing.T) {
	// 零值 Device 没有 GL 上下文 → 不可用 → lost。
	d := &Device{}
	if !d.IsLost() {
		t.Fatal("zero-value Device (nil ctx) must report lost")
	}

	// 具备 GL 上下文且未置 lost → 存活。IsLost 只比较 ctx 是否为 nil，
	// 不解引用，故零值 AdapterContext 足以表达「上下文存在」。
	live := &Device{ctx: &AdapterContext{}}
	if live.IsLost() {
		t.Fatal("live Device (ctx set, lost=false) must not report lost")
	}

	// 显式置 lost（Release 走的就是这条路径）→ 即便 ctx 仍在也报 lost。
	live.lost.Store(true)
	if !live.IsLost() {
		t.Fatal("Device with lost flag set must report lost")
	}
}
