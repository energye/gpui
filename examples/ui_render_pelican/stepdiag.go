// Command ui_render_pelican_stepdiag 是 pelican 示例的步距诊断变体：
// 不改示例本身，通过环境变量 PELICAN_STEPDIAG=1 开启步距记录——
// 每帧记录 (时刻, dist)，结束时输出步距直方图 + 最大停顿时长。
// 用于量化「公路/草地滚动卡顿」：理想 60Hz 下每帧 dist 步进应恒定
// ≈1.6px；若出现 0 步（重复）或 ≥2 倍步长（追帧），即用户看到的顿挫。
package main

import (
	"fmt"
	"os"
	"sort"
	"time"
)

type stepRec struct {
	at   time.Time
	dist float64
}

var stepLog []stepRec

var startedAt time.Time

func stepDiagEnabled() bool { return os.Getenv("PELICAN_STEPDIAG") == "1" }

func stepDiagRecord(dist float64) {
	if !stepDiagEnabled() {
		return
	}
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	stepLog = append(stepLog, stepRec{time.Now(), dist})
	if os.Getenv("PELICAN_STEPRAW") == "1" {
		if n := len(stepLog); n >= 2 {
			fmt.Fprintf(os.Stderr, "STEPRAW %.3f %.3f\n",
				stepLog[n-1].at.Sub(startedAt).Seconds(),
				stepLog[n-1].at.Sub(stepLog[n-2].at).Seconds()*1000)
		}
	}
}

// stepDiagReport 打印帧间 dist 步距分布与相邻帧间隔分布。
func stepDiagReport() {
	if !stepDiagEnabled() || len(stepLog) < 3 {
		return
	}
	type pair struct{ dtMs, dpx float64 }
	pairs := make([]pair, 0, len(stepLog)-1)
	for i := 1; i < len(stepLog); i++ {
		dt := stepLog[i].at.Sub(stepLog[i-1].at).Seconds()
		pairs = append(pairs, pair{dt * 1000, stepLog[i].dist - stepLog[i-1].dist})
	}
	// 直方图：dt 分桶
	hist := map[string]int{}
	var zeroFrames int
	for _, p := range pairs {
		switch b := p.dtMs; {
		case b < 4:
			hist["<4ms"]++
		case b < 12:
			hist["4-12"]++
		case b < 20:
			hist["12-20"]++
		case b < 34:
			hist["20-34"]++
		default:
			hist[">=34"]++
			zeroFrames++
		}
	}
	dts := make([]float64, len(pairs))
	for i, p := range pairs {
		dts[i] = p.dtMs
	}
	sort.Float64s(dts)
	pct := func(q float64) float64 {
		return dts[int(float64(len(dts)-1)*q)]
	}
	fmt.Fprintf(os.Stderr, "STEPDIAG frames=%d dt_ms: p50=%.2f p95=%.2f p99=%.2f max=%.2f hist=%v\n",
		len(pairs), pct(0.5), pct(0.95), pct(0.99), dts[len(dts)-1], hist)

	// px 步距：按速度归一后看异常（0 或 >2x 中位）
	med := dts[len(dts)/2]
	var zeroPx, bigPx int
	for _, p := range pairs {
		ratio := p.dpx / (1.605 * p.dtMs / med) // 归一化期望步长
		if ratio < 0.05 {
			zeroPx++
		} else if ratio > 1.9 {
			bigPx++
		}
	}
	fmt.Fprintf(os.Stderr, "STEPDIAG px-steps: zero=%d big(>1.9x)=%d of %d\n", zeroPx, bigPx, len(pairs))
}
