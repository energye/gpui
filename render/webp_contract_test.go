package render

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"testing"
)

// webpTestImage converts a decoded ImageBuf to the standard library image used
// by the channel-level assertions below.
func webpTestImage(t *testing.T, buf *ImageBuf) image.Image {
	t.Helper()
	if buf == nil {
		t.Fatal("nil ImageBuf")
	}
	std := buf.ToStdImage()
	if std == nil {
		t.Fatal("ToStdImage returned nil")
	}
	return std
}

// webpChannelDiff returns the maximum per-channel absolute difference and the
// number of differing channel samples. Alpha is included.
func webpChannelDiff(a, b image.Image) (maxDelta, diffSamples int) {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := ab.Dx(), bb.Dx()
	if ab.Dy() < h {
		h = ab.Dy()
	}
	if bb.Dy() < h {
		h = bb.Dy()
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, abl, aa := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			br, bg, bbl, ba := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			for _, d := range []int{
				int(ar>>8) - int(br>>8),
				int(ag>>8) - int(bg>>8),
				int(abl>>8) - int(bbl>>8),
				int(aa>>8) - int(ba>>8),
			} {
				if d < 0 {
					d = -d
				}
				if d > maxDelta {
					maxDelta = d
				}
				if d != 0 {
					diffSamples++
				}
			}
		}
	}
	return maxDelta, diffSamples
}

func webpAlphaHistogram(img image.Image) (zero, partial, opaque int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			switch a >> 8 {
			case 0:
				zero++
			case 255:
				opaque++
			default:
				partial++
			}
		}
	}
	return zero, partial, opaque
}

// TestLoadWebP_LosslessPixels pins the public WebP loader to a PNG reference
// produced from the same RGBA source. Lossless WebP must round-trip all four
// channels bit-exactly, including transparent and partial-alpha pixels.
func TestLoadWebP_LosslessPixels(t *testing.T) {
	webpBuf, err := LoadWebP("testdata/webp/lossless.webp")
	if err != nil {
		t.Fatalf("LoadWebP(lossless.webp): %v", err)
	}
	if webpBuf.Width() != 96 || webpBuf.Height() != 64 {
		t.Fatalf("dimensions = %dx%d, want 96x64", webpBuf.Width(), webpBuf.Height())
	}
	if webpBuf.Format() != FormatRGBA8 {
		t.Fatalf("format = %v, want RGBA8", webpBuf.Format())
	}
	refBuf, err := LoadImage("testdata/webp/lossless_ref.png")
	if err != nil {
		t.Fatalf("LoadImage(lossless_ref.png): %v", err)
	}
	got, want := webpTestImage(t, webpBuf), webpTestImage(t, refBuf)
	maxDelta, diffSamples := webpChannelDiff(got, want)
	if maxDelta != 0 || diffSamples != 0 {
		t.Fatalf("lossless WebP vs PNG: max channel delta=%d, differing samples=%d; want 0/0", maxDelta, diffSamples)
	}
	zero, partial, opaque := webpAlphaHistogram(got)
	if zero == 0 || partial == 0 || opaque == 0 {
		t.Fatalf("alpha histogram = zero:%d partial:%d opaque:%d; want all three classes present", zero, partial, opaque)
	}
	if zero != 379 || partial != 64 || opaque != 5701 {
		t.Fatalf("alpha histogram = zero:%d partial:%d opaque:%d; want 379/64/5701", zero, partial, opaque)
	}
}

// TestLoadImage_WebPByExtensionAndContent covers the two public LoadImage
// routes: the .webp extension and content sniffing on an unknown extension.
func TestLoadImage_WebPByExtensionAndContent(t *testing.T) {
	direct, err := LoadWebP("testdata/webp/lossless.webp")
	if err != nil {
		t.Fatalf("LoadWebP: %v", err)
	}
	byExt, err := LoadImage("testdata/webp/lossless.webp")
	if err != nil {
		t.Fatalf("LoadImage(.webp): %v", err)
	}
	byContent, err := LoadImage("testdata/webp/lossless.bin")
	if err != nil {
		t.Fatalf("LoadImage(.bin content sniff): %v", err)
	}
	want := webpTestImage(t, direct)
	for name, buf := range map[string]*ImageBuf{".webp": byExt, ".bin": byContent} {
		if buf.Width() != 96 || buf.Height() != 64 {
			t.Fatalf("%s dimensions = %dx%d, want 96x64", name, buf.Width(), buf.Height())
		}
		if maxDelta, diffSamples := webpChannelDiff(webpTestImage(t, buf), want); maxDelta != 0 || diffSamples != 0 {
			t.Fatalf("%s vs LoadWebP: max channel delta=%d, differing samples=%d; want 0/0", name, maxDelta, diffSamples)
		}
	}
}

// TestLoadWebP_LossySanity covers the lossy VP8 path. Lossy decoding is not
// bit-identical across decoders, so this test checks dimensions, opacity, image
// diversity, and a bounded mean difference against a decoder-independent
// reference exported from the encoded file.
func TestLoadWebP_LossySanity(t *testing.T) {
	buf, err := LoadWebP("testdata/webp/lossy.webp")
	if err != nil {
		t.Fatalf("LoadWebP(lossy.webp): %v", err)
	}
	if buf.Width() != 160 || buf.Height() != 120 {
		t.Fatalf("dimensions = %dx%d, want 160x120", buf.Width(), buf.Height())
	}
	got := webpTestImage(t, buf)
	refBuf, err := LoadImage("testdata/webp/lossy_ref.png")
	if err != nil {
		t.Fatalf("LoadImage(lossy_ref.png): %v", err)
	}
	ref := webpTestImage(t, refBuf)
	gb, rb := got.Bounds(), ref.Bounds()
	if gb.Dx() != rb.Dx() || gb.Dy() != rb.Dy() {
		t.Fatalf("reference dimensions = %v, got %v", rb, gb)
	}
	var sum float64
	colors := map[[3]uint8]struct{}{}
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			gr, gg, gbl, ga := got.At(x, y).RGBA()
			rr, rg, rbl, _ := ref.At(x, y).RGBA()
			if ga>>8 != 255 {
				t.Fatalf("lossy pixel (%d,%d) alpha=%d, want 255", x, y, ga>>8)
			}
			for _, d := range []int{int(gr>>8) - int(rr>>8), int(gg>>8) - int(rg>>8), int(gbl>>8) - int(rbl>>8)} {
				if d < 0 {
					d = -d
				}
				sum += float64(d) / 255
			}
			colors[[3]uint8{uint8(gr >> 8), uint8(gg >> 8), uint8(gbl >> 8)}] = struct{}{}
		}
	}
	mean := sum / float64(gb.Dx()*gb.Dy()*3)
	if mean > 0.05 {
		t.Fatalf("lossy mean |delta| /255 = %.5f, want <= 0.05", mean)
	}
	if len(colors) < 1000 {
		t.Fatalf("lossy image has %d distinct colors, want >= 1000 (not a flat/failed decode)", len(colors))
	}
}

// TestLoadWebP_Errors pins the failure contract: missing files, non-WebP data,
// and empty data must return an error instead of a nil buffer or a panic.
func TestLoadWebP_Errors(t *testing.T) {
	if buf, err := LoadWebP("testdata/webp/does-not-exist.webp"); err == nil || buf != nil {
		t.Fatalf("missing file = (%v, %v), want (nil, error)", buf, err)
	}
	if buf, err := LoadWebP("testdata/webp/lossless_ref.png"); err == nil || buf != nil {
		t.Fatalf("PNG bytes through LoadWebP = (%v, %v), want (nil, error)", buf, err)
	}
	empty := filepath.Join(t.TempDir(), "empty.webp")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if buf, err := LoadWebP(empty); err == nil || buf != nil {
		t.Fatalf("empty file = (%v, %v), want (nil, error)", buf, err)
	}
}

// TestLoadWebP_Deterministic guards against state leaking between decodes.
func TestLoadWebP_Deterministic(t *testing.T) {
	a, err := LoadWebP("testdata/webp/lossless.webp")
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	b, err := LoadWebP("testdata/webp/lossless.webp")
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if !bytes.Equal(a.Data(), b.Data()) {
		t.Fatal("two loads of the same file differ")
	}
}
