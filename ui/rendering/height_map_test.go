package rendering

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

func hmRefOffset(h []float64, i int) float64 {
	if i <= 0 {
		return 0
	}
	if i >= len(h) {
		i = len(h)
	}
	s := 0.0
	for k := 0; k < i; k++ {
		s += h[k]
	}
	return s
}

func hmRefIndex(h []float64, y float64) int {
	if len(h) == 0 || y <= 0 {
		return 0
	}
	s := 0.0
	for i, v := range h {
		if y < s+v {
			return i
		}
		s += v
	}
	return len(h) - 1
}

func TestHeightMap_MatchesReference(t *testing.T) {
	const n = 5000
	const est = 20.0
	hm := NewHeightMap(n, est, 100000)
	ref := make([]float64, n)
	for i := range ref {
		ref[i] = est
	}
	rng := rand.New(rand.NewSource(20261113))
	for op := 0; op < 4000; op++ {
		i := rng.Intn(n)
		var h float64
		if op%7 == 0 {
			h = est // remove override
		} else {
			h = 10 + float64(rng.Intn(31))
		}
		if !hm.Measure(i, h) {
			t.Fatalf("Measure(%d,%v) rejected", i, h)
		}
		ref[i] = h
		if op%97 == 0 {
			for k := 0; k < 8; k++ {
				j := rng.Intn(n + 1)
				if got, want := hm.OffsetOf(j), hmRefOffset(ref, j); got != want {
					t.Fatalf("op=%d OffsetOf(%d)=%v want %v", op, j, got, want)
				}
			}
			total := hmRefOffset(ref, n)
			if got := hm.TotalHeight(); got != total {
				t.Fatalf("op=%d total=%v want %v", op, got, total)
			}
			y := rng.Float64() * total
			if got, want := hm.IndexAtOffset(y), hmRefIndex(ref, y); got != want {
				t.Fatalf("op=%d IndexAtOffset(%v)=%d want %d", op, y, got, want)
			}
			mc := 0
			hm.EachMeasured(func(i int, h float64) bool {
				if h != ref[i] {
					t.Fatalf("EachMeasured(%d)=%v want %v", i, h, ref[i])
				}
				mc++
				return true
			})
			if mc != hm.MeasuredCount() {
				t.Fatalf("EachMeasured count=%d want %d", mc, hm.MeasuredCount())
			}
		}
	}
	if got, want := hm.MeasuredCount(), countNonEstimate(ref, est); got != want {
		t.Fatalf("measured=%d want %d", got, want)
	}
}

func countNonEstimate(ref []float64, est float64) int {
	n := 0
	for _, h := range ref {
		if h != est {
			n++
		}
	}
	return n
}

func TestHeightMap_CapEvicts(t *testing.T) {
	const n = 1000000
	const capN = 128
	hm := NewHeightMap(n, 20, capN)
	for i := 0; i < 1000; i++ {
		hm.Measure(int(int64(i)*7919%n), 10+float64(i%31))
	}
	if got := hm.MeasuredCount(); got > capN {
		t.Fatalf("measured=%d > cap=%d", got, capN)
	}
	ref := make([]float64, n)
	for i := range ref {
		ref[i] = 20
	}
	hm.EachMeasured(func(i int, h float64) bool {
		ref[i] = h
		return true
	})
	for _, i := range []int{0, 1, 999, 500000, n - 1} {
		if got, want := hm.OffsetOf(i), hmRefOffset(ref, i); got != want {
			t.Fatalf("OffsetOf(%d)=%v want %v", i, got, want)
		}
	}
	if got, want := hm.TotalHeight(), hmRefOffset(ref, n); got != want {
		t.Fatalf("total=%v want %v", got, want)
	}
}

func TestHeightMap_Memory3M(t *testing.T) {
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	hm := NewHeightMap(3000000, 20, 32768)
	for i := 0; i < 32768; i++ {
		line := int(int64(i) * 91 % 3000000)
		hm.Measure(line, 21+float64(i%17))
	}
	runtime.GC()
	runtime.ReadMemStats(&b)
	runtime.KeepAlive(hm)
	delta := int64(b.HeapAlloc) - int64(a.HeapAlloc)
	t.Logf("HeightMap 3M lines / 32768 measured heap delta=%d bytes (%.2fMB)", delta, float64(delta)/1e6)
	if delta > 3<<20 {
		t.Fatalf("3M 行高度图堆 %d bytes 超 3MB", delta)
	}
	if hm.MeasuredCount() != 32768 {
		t.Fatalf("measured=%d", hm.MeasuredCount())
	}
}

func TestHeightMap_QueryNoLinear(t *testing.T) {
	measure := func(n int) (float64, float64) {
		hm := NewHeightMap(n, 20, 32768)
		for i := 0; i < 32768; i++ {
			line := int(int64(i) * 91 % int64(n))
			hm.Measure(line, 21+float64(i%17))
		}
		rng := rand.New(rand.NewSource(7))
		start := time.Now()
		for i := 0; i < 500; i++ {
			j := rng.Intn(n)
			_ = hm.OffsetOf(j)
			_ = hm.IndexAtOffset(hm.TotalHeight() * rng.Float64())
		}
		return float64(time.Since(start)), 0
	}
	o1, _ := measure(10000)
	o2, _ := measure(3000000)
	ro := o2 / o1
	t.Logf("query wall ratio 3M/1e4: %.2f", ro)
	if ro > 8 {
		t.Fatalf("高度图查询疑似线性: %.1f", ro)
	}
}

func TestHeightMap_Edges(t *testing.T) {
	hm := NewHeightMap(10, 0, 0)
	if hm.Estimate() <= 0 || hm.MeasuredCap() != 32768 {
		t.Fatalf("defaults: est=%v cap=%d", hm.Estimate(), hm.MeasuredCap())
	}
	if hm.Measure(-1, 10) || hm.Measure(10, 10) {
		t.Fatalf("out-of-range Measure accepted")
	}
	hm.Measure(3, 100)
	hm.Measure(3, 20) // back to estimate removes override
	if _, ok := hm.Measured(3); ok || hm.MeasuredCount() != 0 {
		t.Fatalf("estimate reset did not remove override")
	}
	if got := hm.IndexAtOffset(-5); got != 0 {
		t.Fatalf("negative y index=%d", got)
	}
	if got := hm.IndexAtOffset(1e9); got != 9 {
		t.Fatalf("large y index=%d", got)
	}
}
