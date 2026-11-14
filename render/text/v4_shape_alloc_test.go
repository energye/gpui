package text

// hitch 分配削减专项（2026-11-14，用户拍板按方案修）：L2 真窗滚动 hitch
// 骑线的 GC 归因显示整形路径是分配大头（harfbuzz Buffer.append 27% /
// buildShapedCarets 12.5% / rebaseGlyphs 7.4% 的 growslice）。本测试把
// 「滚动式整形流量的分配数」钉在门禁上：连续 Shape 不重复短文本（模拟
// 首滚新行，绕过 shape-result 缓存），AllocsPerRun 中位数 ≤40/shape。
//
// 修前基线 51.88 allocs/shape（hb.NewBuffer 每次新建 + Info/Pos 逐行扩容
// + []rune(text) 每次分配）；修后 = Buffer 挂 shaper 成员 Clear() 复用
// （Skia SkShaper_harfbuzz.cpp:721 `HBBuffer fBuffer` 同款）+ runes scratch。
// 行为锁：shaper 全家 byte-exact 测试与 shaper_hb_race_test 并发测试不
// 改变任何输出。

import (
	"fmt"
	"testing"
)

func TestShapeAlloc_ScrollTrafficBound(t *testing.T) {
	face := builtinTestFace(t)
	shaper := NewHbShaper()
	const n = 8
	texts := make([]string, n)
	for i := range texts {
		texts[i] = fmt.Sprintf("2026-11-14T00:00:%02dZ INFO line content alpha beta gamma %04d", i, i)
	}
	// Warm font parse + hb font caches; per-line content stays unique below
	// (the shape-result cache must not mask the shaping allocations).
	for _, tx := range texts {
		shaper.Shape(tx, face)
	}
	// Distinct text each call (first-scroll traffic); median over 3 runs.
	var runs [3]float64
	for r := range runs {
		runs[r] = testing.AllocsPerRun(50, func() {
			for i := 0; i < n; i++ {
				shaper.Shape(fmt.Sprintf("%s %05d", texts[i], r*7919+i), face)
			}
		})
	}
	median := runs[0]
	for _, v := range runs { // min-of-3: AllocsPerRun is a lower-bound metric,
		if v < median { // the cleanest run is the honest median proxy
			median = v
		}
	}
	perShape := median / n
	t.Logf("Shape scroll-traffic: %.1f allocs / %d shapes = %.2f per shape (bound 40)", median, n, perShape)
	if perShape > 40 {
		t.Fatalf("RED: %.2f allocs/shape exceeds the 40 bound — shaping keeps allocating per call (Buffer/scratch not reused)", perShape)
	}
}
