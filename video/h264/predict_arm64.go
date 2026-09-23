//go:build arm64

package h264

// S1 arm64 predict entry: integer copy, then the shared S1 dispatch
// (qpelFast -> qpelBlock, NEON on arm64), then the scalar留守. Mirrors
// predictLumaBlock (inter.go) bit for bit; kept as the arm64 callers'
// entry so the NEON path is exercised through the same dispatch the
// gates pin on every arch.
func predictLumaBlockArm(ref *Picture, px, py, w, h int, mx, my int16, out []uint8) {
	if ref == nil {
		for i := range out {
			out[i] = 128
		}
		return
	}
	rw, rh := int(ref.Width), int(ref.Height)
	if predictStatsOn {
		notePredictClass(rw, rh, px, py, w, h, mx, my)
	}
	qx0 := px*4 + int(mx)
	qy0 := py*4 + int(my)
	ix0 := qx0 >> 2
	iy0 := qy0 >> 2
	fx0 := qx0 - (ix0 << 2)
	fy0 := qy0 - (iy0 << 2)
	if fx0 == 0 && fy0 == 0 {
		sx, sy := ix0, iy0
		if sx >= 0 && sy >= 0 && sx+w <= rw && sy+h <= rh {
			for dy := 0; dy < h; dy++ {
				copy(out[dy*w:(dy+1)*w], ref.Y[(sy+dy)*rw+sx:(sy+dy)*rw+sx+w])
			}
			return
		}
	}
	if qpelFast(ref.Y, rw, rh, px, py, w, h, mx, my, out) {
		return
	}
	predictLumaBlockScalar(ref.Y, rw, rh, px, py, w, h, mx, my, out)
}
