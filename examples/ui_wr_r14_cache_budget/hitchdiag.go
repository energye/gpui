package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// 帧管线分段计时器：定位 hitch 帧的时间花在哪一段。
// 挂在 PipelineApp 渲染路径的关键点上（UI build / raster submit / present）。
// 输出：每次间隔 >33.4ms 的帧，各段耗时明细 + 当时 GC/内存状态。

type frameSpan struct {
	frameID   int64
	tBuildEnd time.Time // UI 构建（layout+paint+packet）完成
	tSubmit   time.Time // raster job 提交完成
	tDone     time.Time // raster 执行完成（present 提交点）
	gcPauseNs uint64
	heapMB    float64
	numGC     uint32
}

var (
	spanMu       sync.Mutex
	spans        = map[int64]*frameSpan{}
	lastFrameEnd time.Time
	hitchLog     []string
)

func spanMark(frameID int64, stage string) {
	spanMu.Lock()
	defer spanMu.Unlock()
	f := spans[frameID]
	if f == nil {
		f = &frameSpan{frameID: frameID}
		spans[frameID] = f
	}
	now := time.Now()
	switch stage {
	case "submit":
		f.tSubmit = now
	case "build":
		f.tBuildEnd = now
	}
}

// frameDone 在 raster job 末尾调用：结算该帧的分段耗时，
// 与上一帧完成时刻比较，>33.4ms 记为 hitch 并输出分段归因。
func frameDone(frameID int64) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	spanMu.Lock()
	defer spanMu.Unlock()
	f := spans[frameID]
	if f == nil {
		return
	}
	f.tDone = time.Now()
	f.gcPauseNs = ms.PauseTotalNs
	f.heapMB = float64(ms.HeapAlloc) / 1e6
	f.numGC = ms.NumGC

	if !lastFrameEnd.IsZero() {
		gapMs := f.tDone.Sub(lastFrameEnd).Seconds() * 1000
		if gapMs > 33.4 {
			buildMs := 0.0
			if !f.tBuildEnd.IsZero() {
				buildMs = f.tBuildEnd.Sub(f.tSubmit).Seconds() * 1000
			}
			subToDone := 0.0
			if !f.tSubmit.IsZero() {
				subToDone = f.tDone.Sub(f.tSubmit).Seconds() * 1000
			}
			line := fmt.Sprintf("HITCH frame=%d gap=%.1fms (ui→submit=%.1f submit→done=%.1f) heap=%.0fMB numGC=%d",
				frameID, gapMs, buildMs, subToDone, f.heapMB, f.numGC)
			hitchLog = append(hitchLog, line)
			fmt.Fprintln(os.Stderr, line)
		}
	}
	lastFrameEnd = f.tDone

	// 防泄漏：只保留最近 64 帧
	if len(spans) > 128 {
		for id := range spans {
			if id < frameID-64 {
				delete(spans, id)
			}
		}
	}
}
