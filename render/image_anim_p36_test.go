//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render_test

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/io"
)

// P3.6 batch 1b: animated containers (GIF / APNG / animated WebP) and the
// frame compositor.
//
// Every composite is compared against render/testdata/imgfmt/oracle/*.frameN.png,
// produced by scripts/p36_anim_oracles.py from an independent decoder
// (Pillow: its own GIF/APNG parsers, libwebp demux for WebP; ffmpeg agreed on
// the same pixels for the GIF/APNG cases). Comparing against our own metadata
// would prove nothing.

const p36OracleDir = "testdata/imgfmt/oracle"

type animCase struct {
	file       string
	frames     int
	repetition int
	durations  []time.Duration
	exact      bool // false = lossy pixels, compare with tolerance
}

var animCases = []animCase{
	{"gif_keep.gif", 3, -1, ms(100, 200, 300), true},
	{"gif_restore_bg.gif", 3, 2, ms(100, 0, 300), true},
	{"gif_restore_prev.gif", 3, 1, ms(100, 200, 300), true},
	{"gif_once.gif", 2, 0, ms(150, 250), true},
	{"gif_transparent.gif", 2, -1, ms(100, 200), true},
	{"apng_keep.png", 3, -1, ms(100, 200, 300), true},
	{"apng_restorebg.png", 3, -1, ms(100, 200, 300), true},
	{"apng_restoreprev.png", 3, -1, ms(100, 200, 300), true},
	{"apng_keep_erase.png", 3, -1, ms(100, 200, 300), true},
	{"apng_alphaover.png", 2, -1, ms(100, 200), true},
	{"apng_subset.png", 3, -1, ms(90, 90, 90), true},
	{"apng_plays3.png", 2, 2, ms(120, 240), true},
	// 畸形分母：上游口径 den==0 → 100，所以 120/100 秒 = 1200ms。
	{"apng_delden0.png", 2, 2, ms(1200, 240), true},
	{"webp_anim_lossy.webp", 3, 1, ms(100, 0, 250), false},
	{"webp_anim_lossless.webp", 2, -1, ms(120, 120), true},
}

func ms(vals ...int) []time.Duration {
	out := make([]time.Duration, len(vals))
	for i, v := range vals {
		out[i] = time.Duration(v) * time.Millisecond
	}
	return out
}

func openAnim(t *testing.T, file string) *render.AnimatedImage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata/imgfmt", file))
	if err != nil {
		t.Fatalf("missing sample %s (run scripts/gen_p36_testdata.py): %v", file, err)
	}
	a, err := render.LoadAnimatedImageFromBytes(data, render.DecodeOptions{})
	if err != nil {
		t.Fatalf("LoadAnimatedImageFromBytes(%s): %v", file, err)
	}
	t.Cleanup(func() { a.Dispose() })
	return a
}

func oracleFrame(t *testing.T, file string, i int) []byte {
	t.Helper()
	path := filepath.Join(p36OracleDir, fmt.Sprintf("%s.frame%d.png", file, i))
	raw, err := os.ReadFile(path)
	if err != nil {
		// 参照帧是提交进仓库的数据，缺了就是仓库缺陷，必须红而不是 Skip。
		t.Fatalf("missing oracle %s (run scripts/p36_anim_oracles.py): %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode oracle %s: %v", path, err)
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("oracle %s is %T, want *image.NRGBA", path, img)
	}
	w, h := nrgba.Bounds().Dx(), nrgba.Bounds().Dy()
	out := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		copy(out[y*w*4:], nrgba.Pix[y*nrgba.Stride:y*nrgba.Stride+w*4])
	}
	return out
}

// TestAnimatedMetadataMatchesAuthoredValues pins the container layer: frame
// counts, loop semantics (negative = forever), per-frame durations, and the
// disposal/blend/rect metadata the compositor is built on.
func TestAnimatedMetadataMatchesAuthoredValues(t *testing.T) {
	for _, c := range animCases {
		a := openAnim(t, c.file)
		if got := a.FrameCount(); got != c.frames {
			t.Errorf("%s: FrameCount = %d, want %d", c.file, got, c.frames)
		}
		if got := a.RepetitionCount(); got != c.repetition {
			t.Errorf("%s: RepetitionCount = %d, want %d", c.file, got, c.repetition)
		}
		for i, want := range c.durations {
			info, err := a.FrameInfo(i)
			if err != nil {
				t.Fatalf("%s frame %d: %v", c.file, i, err)
			}
			if info.Duration != want {
				t.Errorf("%s frame %d: Duration = %v, want %v", c.file, i, info.Duration, want)
			}
			if info.DisposalRect.Dx() <= 0 || info.DisposalRect.Dy() <= 0 {
				t.Errorf("%s frame %d: empty DisposalRect %v", c.file, i, info.DisposalRect)
			}
			if want := info.RequiredFrame; want != render.NoRequiredFrame && want != i-1 {
				t.Errorf("%s frame %d: RequiredFrame = %d, want %d or NoRequiredFrame", c.file, i, want, i-1)
			}
		}
		if _, err := a.FrameInfo(c.frames); err == nil {
			t.Errorf("%s: FrameInfo(%d) out of range returned no error", c.file, c.frames)
		}
	}
}

// TestAnimatedCompositingMatchesOracleFrames is the pixel gate: every frame of
// every sample must composite to exactly what the independent decoder produced.
// exact=true cases compare byte-for-byte (all containers there are lossless);
// the lossy WebP case compares within a declared per-channel budget because the
// two VP8 decoders disagree on rounding, not because the compositor is fuzzy.
func TestAnimatedCompositingMatchesOracleFrames(t *testing.T) {
	const lossyMaxChannelDiff = 40
	const lossyMaxDiffPixelsPct = 15
	for _, c := range animCases {
		a := openAnim(t, c.file)
		w, h := a.Canvas()
		if w != 40 || h != 30 {
			t.Errorf("%s: canvas = %dx%d, want 40x30", c.file, w, h)
		}
		for i := 0; i < c.frames; i++ {
			if err := a.DecodeFrame(i); err != nil {
				t.Fatalf("%s frame %d: %v", c.file, i, err)
			}
			buf := a.Image()
			got := append([]byte(nil), buf.Data()[:w*h*4]...)
			want := oracleFrame(t, c.file, i)
			if c.exact {
				if !bytes.Equal(got, want) {
					dx, dy, count := firstDiff(got, want, w, h)
					t.Errorf("%s frame %d: %d/%d pixels differ, first at (%d,%d) got %v want %v",
						c.file, i, count, w*h, dx, dy,
						got[(dy*w+dx)*4:(dy*w+dx)*4+4], want[(dy*w+dx)*4:(dy*w+dx)*4+4])
				}
				continue
			}
			bad, worst := 0, 0
			for p := 0; p < w*h; p++ {
				for ch := 0; ch < 4; ch++ {
					d := p36AbsDiff(got[p*4+ch], want[p*4+ch])
					if d > worst {
						worst = d
					}
				}
				if channelMaxDiff(got[p*4:p*4+4], want[p*4:p*4+4]) > lossyMaxChannelDiff {
					bad++
				}
			}
			if pct := 100 * bad / (w * h); pct > lossyMaxDiffPixelsPct {
				t.Errorf("%s frame %d: %d/%d pixels off by >%d (worst channel diff %d), want <= %d%%",
					c.file, i, bad, w*h, lossyMaxChannelDiff, worst, lossyMaxDiffPixelsPct)
			}
			t.Logf("%s frame %d: %d/%d pixels beyond tolerance %d (worst channel diff %d)",
				c.file, i, bad, w*h, lossyMaxChannelDiff, worst)
		}
	}
}

func channelMaxDiff(a, b []byte) int {
	m := 0
	for ch := 0; ch < 4; ch++ {
		if d := p36AbsDiff(a[ch], b[ch]); d > m {
			m = d
		}
	}
	return m
}

func countDiff(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

func firstDiff(a, b []byte, w, h int) (int, int, int) {
	count := 0
	fx, fy := 0, 0
	found := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := (y*w + x) * 4
			if a[p] != b[p] || a[p+1] != b[p+1] || a[p+2] != b[p+2] || a[p+3] != b[p+3] {
				count++
				if !found {
					fx, fy, found = x, y, true
				}
			}
		}
	}
	return fx, fy, count
}

func p36AbsDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// TestDisposalVariantsActuallyDiffer guards the sample set itself: if two
// containers with different disposal methods produced identical pixels, the
// oracle comparison above would be vacuous.
func TestDisposalVariantsActuallyDiffer(t *testing.T) {
	pixel := func(t *testing.T, file string, frame int, x, y int) [4]uint8 {
		a := openAnim(t, file)
		if err := a.DecodeFrame(frame); err != nil {
			t.Fatalf("%s frame %d: %v", file, frame, err)
		}
		r, g, b, al := a.Image().GetRGBA(x, y)
		return [4]uint8{r, g, b, al}
	}
	// 绿块中心：保留=绿，还原背景=清空；红块擦除处：保留=底色，还原前一帧=红回来。
	keep := pixel(t, "apng_keep.png", 2, 25, 12)
	bg := pixel(t, "apng_restorebg.png", 2, 25, 12)
	if _, err := os.Stat(filepath.Join("testdata/imgfmt", "apng_keep_erase.png")); err != nil {
		t.Skipf("missing apng_keep_erase.png (run scripts/gen_p36_testdata.py)")
	}
	prev := pixel(t, "apng_restoreprev.png", 2, 10, 10)
	erased := pixel(t, "apng_keep_erase.png", 2, 10, 10) // 同帧同矩形，只差 dispose 位
	if keep == bg {
		t.Errorf("apng keep vs restore-bg agree at (25,12): %v — sample cannot discriminate", keep)
	}
	if prev == erased {
		t.Errorf("restore-previous vs keep agree at (10,10): %v — sample cannot discriminate", prev)
	}
	if gifKeep := pixel(t, "gif_keep.gif", 2, 25, 12); gifKeep == pixel(t, "gif_restore_bg.gif", 2, 25, 12) {
		t.Errorf("GIF keep vs restore-bg agree — sample cannot discriminate")
	}
	// GIF 的还原背景回到逻辑屏底色，APNG 的清成透明：两个容器的差异必须是实测出来的。
	if bg[3] != 0 {
		t.Errorf("APNG restore-bg alpha = %d, want 0 (transparent)", bg[3])
	}
	if gifBg := pixel(t, "gif_restore_bg.gif", 2, 25, 12); gifBg[3] != 255 {
		t.Errorf("GIF restore-bg alpha = %d, want 255 (background colour index 0)", gifBg[3])
	}
}

// TestAnimatedFrameAdvanceKeepsOneBuffer pins the P3.6 batch 1c contract: one
// reused *ImageBuf across playback, and a NEW content generation per frame, with
// every retired generation reported to watchers so each consumer cache reclaims
// its own stale texture (Skia SkPixelRef::notifyPixelsChanged semantics).
func TestAnimatedFrameAdvanceKeepsOneBuffer(t *testing.T) {
	a := openAnim(t, "apng_keep.png")
	first := a.Image()
	seen := map[uint64]bool{}
	var retired []uint64
	remove := first.AddGenIDWatcher(func(old uint64) { retired = append(retired, old) })
	defer remove()
	for i := 0; i < a.FrameCount(); i++ {
		if err := a.DecodeFrame(i); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if got := a.Image(); got != first {
			t.Fatalf("frame %d: Image() returned a different buffer", i)
		}
		gen := a.Image().GenerationID()
		if gen == 0 {
			t.Fatalf("frame %d: zero generation id", i)
		}
		if seen[gen] {
			t.Fatalf("frame %d: generation id %d repeats (a repeated id would serve a stale texture)", i, gen)
		}
		seen[gen] = true
		if want := uint64(i); uint64(len(retired)) != want+1 {
			t.Fatalf("frame %d: retired ids reported = %d, want %d (caches would never learn the pixels moved)", i, len(retired), want+1)
		}
	}
	if len(seen) != a.FrameCount() {
		t.Fatalf("distinct generations = %d, want frame count %d", len(seen), a.FrameCount())
	}
	// 乱序回到更早的帧必须重放而不是画出半旧画布。
	if err := a.DecodeFrame(0); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	got := append([]byte(nil), a.Image().Data()[:40*30*4]...)
	want := oracleFrame(t, "apng_keep.png", 0)
	if !bytes.Equal(got, want) {
		t.Errorf("rewind to frame 0 did not reproduce the oracle pixels")
	}
}

// TestSingleFrameStaticPathForGIF pins the poster behaviour of the ordinary
// image door: a GIF must still load through LoadImage as its first frame, and a
// single-frame file must not pass the animated door.
func TestSingleFrameStaticPathForGIF(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata/imgfmt", "gif_keep.gif"))
	if err != nil {
		t.Skipf("missing sample: %v", err)
	}
	img, err := render.LoadImageFromBytes(data, render.DecodeOptions{})
	if err != nil {
		t.Fatalf("LoadImageFromBytes(GIF): %v", err)
	}
	defer img.Dispose()
	w, h := img.Bounds()
	if w != 40 || h != 30 {
		t.Errorf("GIF poster = %dx%d, want 40x30", w, h)
	}
	// 第一帧就是整幅内容，参照帧可直接对拍（GIF 调色板转真彩可能有 0/1 差）。
	want := oracleFrame(t, "gif_keep.gif", 0)
	if got := append([]byte(nil), img.Data()[:w*h*4]...); !bytes.Equal(got, want) {
		t.Errorf("GIF poster differs from oracle frame 0 in %d bytes", countDiff(got, want))
	}
	if _, err := render.LoadAnimatedImageFromBytes(mustEncodePNG(t, 40, 30), render.DecodeOptions{}); err == nil {
		t.Errorf("single-frame PNG passed the animated door, want error")
	}
}

func mustEncodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 7
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}

// TestUIIODecodeGoesThroughRenderSource pins the D5 convergence: the UI decode
// pool reports the same container set as the engine, decodes a GIF (a format it
// did not know before), and its Result.Format is the container the *bytes* are,
// not the first registered name.
func TestUIIODecodeGoesThroughRenderSource(t *testing.T) {
	got := io.SupportedDecoders()
	want := []string{"gif", "jpeg", "png", "webp"}
	if len(got) != len(want) {
		t.Fatalf("SupportedDecoders = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SupportedDecoders = %v, want %v", got, want)
		}
	}
	cases := []struct {
		file, wantFormat string
		w, h             int
	}{
		{"gif_keep.gif", "gif", 40, 30},
		{"apng_plays3.png", "png", 40, 30},
		{"webp_static_lossy.webp", "webp", 40, 30},
		{"noexif.jpg", "jpeg", 40, 30},
	}
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join("testdata/imgfmt", c.file))
		if err != nil {
			t.Skipf("missing sample %s: %v", c.file, err)
		}
		ch := make(chan io.Result, 1)
		io.DecodeBytes(data, func(r io.Result) { ch <- r })
		res := <-ch
		if res.Err != nil {
			t.Fatalf("%s: ui/io decode: %v", c.file, res.Err)
		}
		if res.Format != c.wantFormat {
			t.Errorf("%s: Result.Format = %q, want %q", c.file, res.Format, c.wantFormat)
		}
		if res.Width != c.w || res.Height != c.h {
			t.Errorf("%s: Result dims = %dx%d, want %dx%d", c.file, res.Width, res.Height, c.w, c.h)
		}
		res.Img.Dispose()
	}
	// 一个容器名都不认识的字节必须报错，不能被某个注册项顺手接走。
	ch := make(chan io.Result, 1)
	io.DecodeBytes([]byte("definitely not an image"), func(r io.Result) { ch <- r })
	if res := <-ch; res.Err == nil {
		t.Errorf("junk bytes decoded without error (format %q)", res.Format)
	}
}

// TestLoadAnimatedImageRejectsUnknownContainer pins the door's failure mode.
func TestLoadAnimatedImageRejectsUnknownContainer(t *testing.T) {
	if _, err := render.LoadAnimatedImageFromBytes([]byte("nope"), render.DecodeOptions{}); err == nil {
		t.Errorf("junk bytes passed the animated door")
	}
}
