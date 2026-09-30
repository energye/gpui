//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build windows && !(js && wasm)

package gles

import (
	"testing"
	"time"
)

// 本机（Linux）只验编译；本文件不含 Submit 用例（无卡时 Lock 会进
// WGL 初始化路径，留 Linux 侧锁增量逻辑，两边 Submit 主体一致）。
// 以下用例真机跑时不碰驱动：CreateFence 只取函数表（GL() 可能为 nil，
// NewFence nil-safe），Signal/Wait/Status/Reset 全走记数回退。

func TestH4B3_GlesWindowsFenceWrappers(t *testing.T) {
	d := &Device{ctx: NewAdapterContext(0)}

	f, err := d.CreateFence()
	if err != nil {
		t.Fatalf("CreateFence() = %v, want nil", err)
	}
	gf, ok := f.(*Fence)
	if !ok {
		t.Fatalf("CreateFence type = %T, want *gles.Fence", f)
	}
	if err := gf.Signal(2); err != nil {
		t.Fatalf("Signal(2) = %v, want nil", err)
	}
	ok, err = d.WaitForFence(f, 2, time.Second)
	if err != nil || !ok {
		t.Fatalf("WaitForFence(2) = (%v, %v), want (true, nil)", ok, err)
	}
	signaled, err := d.GetFenceStatus(f)
	if err != nil || !signaled {
		t.Fatalf("GetFenceStatus() = (%v, %v), want (true, nil)", signaled, err)
	}
	if err := d.ResetFence(f); err != nil {
		t.Fatalf("ResetFence() = %v, want nil", err)
	}
	if signaled, _ := d.GetFenceStatus(f); signaled {
		t.Fatal("GetFenceStatus() after Reset = true, want false")
	}
	if err := d.ResetFence(nil); err == nil {
		t.Fatal("ResetFence(nil) = nil, want error")
	}
	if _, err := d.GetFenceStatus(nil); err == nil {
		t.Fatal("GetFenceStatus(nil) = nil, want error")
	}
}
