package main

import "testing"

func TestStageFitFor(t *testing.T) {
	for _, c := range []struct{ w, h float64 }{
		{1200, 800}, {800, 600}, {1600, 900}, {640, 480}, {2400, 1000},
	} {
		f := stageFitFor(c.w, c.h)
		s := c.w / 960
		if hs := c.h / 600; hs < s {
			s = hs
		}
		if f.s != s || f.ox != (c.w-960*s)/2 || f.oy != (c.h-600*s)/2 {
			t.Fatalf("stageFitFor(%v,%v) = %+v, want s=%v", c.w, c.h, f, s)
		}
		// 舞台右下角必须落在窗口内（等比缩放 + 居中）
		if rx, ry := f.ox+960*f.s, f.oy+600*f.s; rx > c.w+0.01 || ry > c.h+0.01 {
			t.Fatalf("stage overflows window (%v,%v): right=%v bottom=%v", c.w, c.h, rx, ry)
		}
	}
}
