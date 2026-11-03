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

import "testing"

// D20 转发口的钉：无装配时按「无数据」呈现（不猜值）；装配后读回采集值；
// 重复注册是幂等覆盖。真实 GPU 会话上的字段语义（Entries 恒 1、Creates 平坦、
// Uploads 随帧涨、RetiredGens 归还）由批 3 animplay 真窗的对账卡实测承担。
func TestImageCacheSnapshotForwarding(t *testing.T) {
	// 注册口是进程级单值（无 Unregister），-count=2 迭代共享状态：开头先注册
	// 「无会话」标准态，各段在其上覆盖；「无装配报 ok=false」的语义已由下面的
	// 透传段覆盖（collector 报无会话 → 快照 ok=false）。
	RegisterImageCacheStatsCollector(func() (ImageCacheStats, bool) { return ImageCacheStats{}, false })

	want := ImageCacheStats{Entries: 3, UsedBytes: 4096, Creates: 2, Uploads: 7, RetiredGens: 5}
	RegisterImageCacheStatsCollector(func() (ImageCacheStats, bool) { return want, true })
	got, ok := ImageCacheSnapshot()
	if !ok || got != want {
		t.Fatalf("snapshot after register = %+v,%v want %+v,true", got, ok, want)
	}

	// 覆盖注册：后装的生效（GPU 装配层重建时替换转发器）。
	next := ImageCacheStats{Entries: 1, Uploads: 2}
	RegisterImageCacheStatsCollector(func() (ImageCacheStats, bool) { return next, true })
	if got, _ := ImageCacheSnapshot(); got != next {
		t.Fatalf("re-register must overwrite: got %+v want %+v", got, next)
	}

	// 采集方报无会话时透传 ok=false（GPU 未初始化/会话已关，调用方不猜值）。
	RegisterImageCacheStatsCollector(func() (ImageCacheStats, bool) { return ImageCacheStats{}, false })
	if _, ok := ImageCacheSnapshot(); ok {
		t.Fatal("collector reporting no session must forward ok=false")
	}
	// nil 注册是无操作：保留上一个转发器（用哨兵值证明不是被清空）。
	keep := ImageCacheStats{Entries: 9}
	RegisterImageCacheStatsCollector(func() (ImageCacheStats, bool) { return keep, true })
	RegisterImageCacheStatsCollector(nil)
	if got, ok := ImageCacheSnapshot(); !ok || got != keep {
		t.Fatalf("nil register must keep the previous collector: got %+v,%v", got, ok)
	}
}
