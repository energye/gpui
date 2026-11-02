package image

import (
	"testing"
)

// refDownsample is the ORIGINAL per-pixel implementation (GetRGBA 4 taps →
// average → SetRGBA), kept here as the independent definition of what a level
// must contain. The production downsample reads and writes whole rows; this
// test pins that it is byte-for-byte the same picture, not a re-interpretation.
func refDownsample(t *testing.T, src *ImageBuf) *ImageBuf {
	t.Helper()
	srcW, srcH := src.Bounds()
	dstW, dstH := max(1, srcW/2), max(1, srcH/2)
	dst, err := NewImageBuf(dstW, dstH, src.Format())
	if err != nil {
		t.Fatalf("reference dst: %v", err)
	}
	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			sx := dx * 2
			sy := dy * 2
			var r0, g0, b0, a0 uint8
			var sr, sg, sb, sa uint16
			for _, p := range [][2]int{
				{min(sx, srcW-1), min(sy, srcH-1)},
				{min(sx+1, srcW-1), min(sy, srcH-1)},
				{min(sx, srcW-1), min(sy+1, srcH-1)},
				{min(sx+1, srcW-1), min(sy+1, srcH-1)},
			} {
				r0, g0, b0, a0 = src.GetRGBA(p[0], p[1])
				sr += uint16(r0)
				sg += uint16(g0)
				sb += uint16(b0)
				sa += uint16(a0)
			}
			if err := dst.SetRGBA(dx, dy, byte(sr/4), byte(sg/4), byte(sb/4), byte(sa/4)); err != nil {
				t.Fatalf("reference write: %v", err)
			}
		}
	}
	return dst
}

func TestDownsampleByteIdentical(t *testing.T) {
	formats := []Format{FormatRGBA8, FormatRGBAPremul, FormatBGRA8, FormatBGRAPremul, FormatRGB8, FormatGray8, FormatGray16}
	sizes := [][2]int{{8, 8}, {7, 5}, {1, 4}, {4, 1}, {1, 1}, {13, 9}, {16, 3}}
	for _, f := range formats {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			src, err := NewImageBuf(w, h, f)
			if err != nil {
				t.Fatalf("%v %dx%d: %v", f, w, h, err)
			}
			// Deterministic per-texel pattern (not a standard dataset: it only
			// needs to make channel and edge handling differ pixel by pixel).
			bpe := f.BytesPerPixel()
			data := src.Data()
			st := src.Stride()
			i := uint32(1)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					i = i*1103515245 + 12345
					for c := 0; c < bpe; c++ {
						data[y*st+x*bpe+c] = byte(i>>((c+1)*4)) ^ byte(x*17+y*7+c)
					}
				}
			}
			src.InvalidatePremulCache()

			got := downsample(src)
			want := refDownsample(t, src)
			if got == nil {
				t.Fatalf("%v %dx%d: downsample returned nil", f, w, h)
			}
			gw, gh := got.Bounds()
			ww, wh := want.Bounds()
			if gw != ww || gh != wh {
				t.Fatalf("%v %dx%d: size %dx%d want %dx%d", f, w, h, gw, gh, ww, wh)
			}
			gstride, wstride := got.Stride(), want.Stride()
			gdata, wdata := got.Data(), want.Data()
			for y := 0; y < gh; y++ {
				for x := 0; x < gw*bpe; x++ {
					if x >= bpe*gw {
						continue
					}
					if gdata[y*gstride+x] != wdata[y*wstride+x] {
						t.Fatalf("%v %dx%d level 字节差 @(%d,%d byte %d) got=%d want=%d",
							f, w, h, x/bpe, y, x%bpe, gdata[y*gstride+x], wdata[y*wstride+x])
					}
				}
			}
			src.Dispose()
			got.Dispose()
			want.Dispose()
		}
	}
}
