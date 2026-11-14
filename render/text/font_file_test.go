//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// 同一路径的字体文件字节必须跨 (ttcIndex, parser) 缓存键共享一份映射：
// Skia SkData::MakeFromFileName / Flutter fml::FileMapping 同款语义。
// 本仓实测缺陷：同一路径不同 collection index 各存一份完整文件字节
// （CJK TTC 18.6MB 一份），多角色/多字重链叠加后每进程白占 20-40MB。
func TestFontFileSharedAcrossCacheKeys(t *testing.T) {
	path := filepath.Join("testdata", "wqy-microhei.ttf")
	st, err := os.Stat(path)
	if err != nil {
		t.Skipf("testdata 字体缺失: %v", err)
	}

	fontFileMu.Lock()
	_, existed := fontFileTable[filepath.Clean(path)]
	fontFileMu.Unlock()
	if existed {
		t.Skip("字体文件已被其他测试载入，跳过共享计数断言")
	}

	a, err := fontFileAcquire(path)
	if err != nil {
		t.Fatalf("acquire #1: %v", err)
	}
	b, err := fontFileAcquire(path)
	if err != nil {
		t.Fatalf("acquire #2: %v", err)
	}

	if a != b {
		t.Fatalf("同路径两次 acquire 必须返回同一共享条目")
	}
	if a.refs != 2 {
		t.Fatalf("refs = %d, want 2", a.refs)
	}
	if int64(len(a.data)) != st.Size() {
		t.Fatalf("映射字节数 %d != 文件大小 %d", len(a.data), st.Size())
	}
	fontFileMu.Lock()
	n := len(fontFileTable)
	fontFileMu.Unlock()
	if n != 1 {
		t.Fatalf("同路径两次 acquire 后注册表条目数 = %d, want 1", n)
	}

	a.release()
	fontFileMu.Lock()
	still := fontFileTable[filepath.Clean(path)]
	fontFileMu.Unlock()
	if still == nil {
		t.Fatalf("仍有引用时映射不得释放")
	}

	b.release()
	fontFileMu.Lock()
	gone := fontFileTable[filepath.Clean(path)] == nil
	fontFileMu.Unlock()
	if !gone {
		t.Fatalf("引用归零后映射必须释放（munmap）")
	}
}

// PurgeFontSourceCache 之后：缓存键清空，但 purge 前发出去的 FontSource 仍被
// 持有——它的映射必须存活（契约：handed-out sources stay valid，懒解析随时
// 会读字节）。源对象不可达后（GC），cleanup 释放映射（O9 熔断链的回收出口）。
func TestPurgeFontSourceCacheKeepsHandedOutMappings(t *testing.T) {
	path := filepath.Join("testdata", "wqy-microhei.ttf")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("testdata 字体缺失: %v", err)
	}
	PurgeFontSourceCache()
	src, err := NewFontSourceFromFile(path)
	if err != nil {
		t.Fatalf("载入字体: %v", err)
	}
	fontFileMu.Lock()
	n := len(fontFileTable)
	fontFileMu.Unlock()
	if n == 0 {
		t.Fatalf("载入后注册表应有映射条目")
	}
	PurgeFontSourceCache()
	// purge 后 handed-out source 还活着：映射必须保留（懒解析随时会读）。
	fontFileMu.Lock()
	n = len(fontFileTable)
	fontFileMu.Unlock()
	if n == 0 {
		t.Fatalf("purge 后 handed-out source 的映射不得立即释放")
	}
	// 懒解析在 purge 后仍必须能读映射字节（曾经 munmap 导致 SIGSEGV 的回归钉）。
	face := src.Face(14)
	if !face.HasGlyph('A') {
		t.Fatalf("purge 后 source 懒解析失败")
	}
	// 源不可达后 GC，cleanup 释放映射。
	runtime.GC()
	runtime.GC()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fontFileTableSize() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}
	t.Fatalf("源不可达后映射未被 cleanup 释放")
}
