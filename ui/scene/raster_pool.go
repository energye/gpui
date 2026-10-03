//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scene

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// B1 并行光栅工人池：只做只读扫描（hasVisibleOps），重放仍串行。
// 对齐 Chromium/Flutter：前面分路看，后面合拢交。
//
// 线程规矩：
//   - 常驻工人，上限 min(NumCPU-1, 8)；两核以下不分（直串行）。
//   - 工人 5 秒没活自己退，不留后台线程；CloseRasterPool 显式收（关窗/测试）。
//   - 任务 <2 个不分；投递与关池/闲退互斥，不撞关池、不抛 panic。
//   - 每路只读自己那层的 Ops（包封印后只读，见 T1），不写共享状态。

// workerIdleTimeout 是工人没活多久自己退。
const workerIdleTimeout = 5 * time.Second

// maxRasterWorkers 上限：再多核也只开 8 路，防排队比干活贵。
const maxRasterWorkers = 8

var rasterPoolMu sync.RWMutex
var rasterPoolLive int
var rasterPoolTasks chan func()
var rasterPoolOpen bool

// rasterPoolGen 开池代次：关池即换代，旧工人只退不记数，不污染新池的人数。
var rasterPoolGen uint64

// RasterWorkers 返回本机该开几路。GPUI_RASTER_WORKERS 可盖（0/1=强制串行，测试用）。
func RasterWorkers() int {
	if v := os.Getenv("GPUI_RASTER_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	n := runtime.NumCPU() - 1
	if n < 1 {
		return 1
	}
	if n > maxRasterWorkers {
		return maxRasterWorkers
	}
	return n
}

// ensureRasterPool 把活工人补到 n 个（幂等）。工人闲置超时自退，这里只补差额。
// 通道和代次当参数交工人：工人代入后不再读全局变量，与开关池不撞车。
func ensureRasterPool(n int) {
	rasterPoolMu.Lock()
	defer rasterPoolMu.Unlock()
	if !rasterPoolOpen {
		rasterPoolTasks = make(chan func())
		rasterPoolOpen = true
		rasterPoolGen++
	}
	ch, gen := rasterPoolTasks, rasterPoolGen
	for rasterPoolLive < n {
		rasterPoolLive++
		go rasterWorker(ch, gen)
	}
}

// rasterWorker 常驻到闲置超时：有活干活，没活退，不留后台线程。
// 只碰自己代入的通道和代次，不读全局池变量（防与 Close/ensure 撞车）。
func rasterWorker(tasks chan func(), gen uint64) {
	idle := time.NewTimer(workerIdleTimeout)
	defer idle.Stop()
	for {
		select {
		case fn, ok := <-tasks:
			if !ok {
				return
			}
			resetIdle(idle)
			fn()
			idle.Reset(workerIdleTimeout)
		case <-idle.C:
			// 退之前再看一眼有没有活：select 两边同好时会随机走这边，
			// 直接退会把活留在通道里没人干、投递方永远等。
			select {
			case fn, ok := <-tasks:
				if !ok {
					return
				}
				resetIdle(idle)
				fn()
				idle.Reset(workerIdleTimeout)
				continue
			default:
			}
			// 旧代次的工人只退不记数：关池换代后人数已清零，别污染新池。
			rasterPoolMu.Lock()
			if gen == rasterPoolGen {
				rasterPoolLive--
			}
			rasterPoolMu.Unlock()
			return
		}
	}
}

// resetIdle 停表并排空 fired 信号（标准 timer 复用套路）。
func resetIdle(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

// CloseRasterPool 收池（关窗/测试显式收；工人闲置也会自退，双保险）。
func CloseRasterPool() {
	rasterPoolMu.Lock()
	defer rasterPoolMu.Unlock()
	if !rasterPoolOpen {
		return
	}
	close(rasterPoolTasks)
	rasterPoolTasks = nil
	rasterPoolOpen = false
	rasterPoolLive = 0
}

// runParallel 把 n 个任务分路跑完再回。要么全跑完，要么没起池——调用方无部分完成可处理。
// 投递全程持读锁，关池持写锁：不会撞上关池，不抛 panic。
// fn 按索引各写各的结果，无共享写；WaitGroup 计数零申请。
func runParallel(n int, fn func(i int)) {
	rasterPoolMu.RLock()
	defer rasterPoolMu.RUnlock()
	if !rasterPoolOpen || rasterPoolLive == 0 || rasterPoolTasks == nil {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	tasks := rasterPoolTasks
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		tasks <- func() {
			defer wg.Done()
			fn(i)
		}
	}
	wg.Wait()
}

// ParallelScanVisible 分路扫脏层看不看得见（只读，不碰 dc、不写层旗）。
// 路数不够或层数 <2 直接串行，结果一样。返回与入参同序的可见性。
func ParallelScanVisible(layers []*PictureLayer) []bool {
	out := make([]bool, len(layers))
	if len(layers) < 2 || RasterWorkers() <= 1 {
		for i, l := range layers {
			out[i] = layerVisible(l)
		}
		return out
	}
	ensureRasterPool(RasterWorkers())
	runParallel(len(layers), func(i int) { out[i] = layerVisible(layers[i]) })
	return out
}

// layerVisible 扫一层看不看得见。只读 Ops（封印后只读），无共享写。
func layerVisible(l *PictureLayer) bool {
	if l == nil {
		return false
	}
	return l.Picture.hasVisibleOps()
}
