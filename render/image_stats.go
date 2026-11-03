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

import "sync"

// ImageCacheStats 是图像纹理缓存的对外统计快照（P3.6 批 3 D20 转发口）。
//
// 字段逐一映射内部 ImageCache 的统计（render/internal/gpu/image_cache.go 的
// Stats 形状）；语义与批 1c 的「图号=内容版本」消费链对应：
//   - Entries：缓存条目数。同一 ImageBuf 原地换帧（换代际）时键变但旧条目被
//     监听器精确逐出，连播期间应恒为条目基线（一 buf 一条）。
//   - Creates：原生纹理创建次数。纹理随换代即归还复用，连播期间应平坦——
//     上升即纹理 churn（资源泄漏的信号）。
//   - Uploads：纹理上传次数。逐帧重传（换帧即换号即重传），随帧单调涨。
//   - Hits/Misses/Evictions：命中、未命中、按源号精确逐出。
//   - UsedBytes：当前占用字节。
//   - RetiredGens/Watched：已退役的内容代际数 / 仍在监听的源数——两者合看即
//     「动图归还自己的纹理而不是泄漏号」（批 1c 监听器逐出）。
//   - MipEntries/MipLevelUploads：多级条目数 / 深层上传次数——mip 档真建了链
//     并逐层填过的可见证据（批 2）。
type ImageCacheStats struct {
	Entries         int
	UsedBytes       int64
	Creates         uint64
	Hits            uint64
	Misses          uint64
	Uploads         uint64
	Evictions       uint64
	RetiredGens     uint64
	Watched         int
	MipEntries      int
	MipLevelUploads uint64
}

// imageCacheStatsCollector 由 GPU 装配层（render/gpu 的 init）注册：内部实现
// 把默认会话的 imageCache 统计映射成公开形状。ok=false 表示当前没有可读的
// 默认会话（GPU 未初始化、会话未建或已关闭）——调用方按无数据呈现，不猜值。
type imageCacheStatsCollector func() (ImageCacheStats, bool)

var (
	imageCacheStatsMu sync.Mutex
	imageCacheStatsFn imageCacheStatsCollector
)

// RegisterImageCacheStatsCollector 安装统计转发器（幂等覆盖）。
//
// fn: 采集函数；返回 ok=false 表示当前无可读会话。
//
// 调用时机：GPU 装配层建好 accelerator 后（render/gpu init）；非 GPU 装配
// （纯 CPU 测试环境）不注册，查询方按 ok=false 处理。
func RegisterImageCacheStatsCollector(fn func() (ImageCacheStats, bool)) {
	if fn == nil {
		return
	}
	imageCacheStatsMu.Lock()
	imageCacheStatsFn = fn
	imageCacheStatsMu.Unlock()
}

// ImageCacheSnapshot 返回图像纹理缓存的当前统计快照。
//
// 返回 ok=false 表示没有注册转发器或当前无可读会话（零值不可当数据解读）。
// 调用时机：任意线程（采集函数自身决定线程纪律；GPU 侧实现走内部锁）。
func ImageCacheSnapshot() (ImageCacheStats, bool) {
	imageCacheStatsMu.Lock()
	fn := imageCacheStatsFn
	imageCacheStatsMu.Unlock()
	if fn == nil {
		return ImageCacheStats{}, false
	}
	return fn()
}
