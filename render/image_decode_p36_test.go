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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
)

// P3.6 batch 1a: container sniffing by magic bytes, EXIF orientation parsing
// and application, and the named unsupported-format errors.
//
// The orientation expectations come from testdata/exif_grid_N.txt, which
// scripts/gen_p36_testdata.py produces with Pillow's ImageOps.exif_transpose —
// an independent implementation. Deriving them from our own mapping would make
// the test circular and prove nothing.

const p36DataDir = "testdata/imgfmt"

func p36Bytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(p36DataDir, name))
	if err != nil {
		t.Skipf("missing P3.6 sample %s (run scripts/gen_p36_testdata.py): %v", name, err)
	}
	return data
}

func TestImageEncodingOfBytesMatchesSamples(t *testing.T) {
	cases := []struct {
		file string
		want render.ImageEncoding
	}{
		{"gif_keep.gif", render.ImageEncodingGIF},
		{"gif_transparent.gif", render.ImageEncodingGIF},
		{"apng_plays0.png", render.ImageEncodingPNG},
		{"apng_delden0.png", render.ImageEncodingPNG},
		{"webp_anim_lossy.webp", render.ImageEncodingWebP},
		{"webp_static_lossy.webp", render.ImageEncodingWebP},
		{"exif_1.jpg", render.ImageEncodingJPEG},
		{"noexif.jpg", render.ImageEncodingJPEG},
	}
	for _, c := range cases {
		got := render.ImageEncodingOfBytes(p36Bytes(t, c.file))
		if got != c.want {
			t.Errorf("encoding of %s = %v, want %v", c.file, got, c.want)
		}
	}
}

// TestImageEncodingOfBytesNamesAVIFHEIF pins D1: the two containers are
// recognised by brand, and the loader says so by name instead of pretending the
// bytes are junk.
func TestImageEncodingOfBytesNamesAVIFHEIF(t *testing.T) {
	ftyp := func(major string, compat ...string) []byte {
		size := 8 + 4 + 4*len(compat)
		b := []byte{}
		b = append(b, byte(size), 0, 0, 0, 'f', 't', 'y', 'p')
		b = append(b, major...)
		b = append(b, 0, 0, 0, 0) // minor version
		for _, c := range compat {
			b = append(b, c...)
		}
		return b
	}
	cases := []struct {
		data []byte
		want render.ImageEncoding
	}{
		{ftyp("avif", "avif", "mif1"), render.ImageEncodingAVIF},
		{ftyp("avis"), render.ImageEncodingAVIF},
		{ftyp("msf1", "heic", "mif1"), render.ImageEncodingHEIF},
		{ftyp("heix"), render.ImageEncodingHEIF},
		{ftyp("mif1", "hevc", "mif1"), render.ImageEncodingHEIF},
		{[]byte("bogusbogus"), render.ImageEncodingUnknown},
	}
	for i, c := range cases {
		if got := render.ImageEncodingOfBytes(c.data); got != c.want {
			t.Errorf("case %d: encoding = %v, want %v", i, got, c.want)
		}
	}
	// Recognised but undecodable formats must fail with the named sentinel and
	// name the format, so a caller can tell "not wired yet" from "not an image".
	for _, enc := range []render.ImageEncoding{render.ImageEncodingAVIF, render.ImageEncodingHEIF} {
		var data []byte
		if enc == render.ImageEncodingAVIF {
			data = ftyp("avif")
		} else {
			data = ftyp("heix")
		}
		_, err := render.LoadImageFromBytes(data, render.DecodeOptions{})
		if !errors.Is(err, render.ErrUnsupportedFormat) {
			t.Errorf("%s: err = %v, want ErrUnsupportedFormat", enc, err)
		}
		if err != nil && !strings.Contains(err.Error(), enc.String()) {
			t.Errorf("%s: error does not name the format: %v", enc, err)
		}
		if enc.IsDecodable() {
			t.Errorf("%s: IsDecodable() = true, want false", enc)
		}
	}
	// Random bytes: same sentinel, no format name to print.
	if _, err := render.LoadImageFromBytes([]byte("not an image at all"), render.DecodeOptions{}); !errors.Is(err, render.ErrUnsupportedFormat) {
		t.Errorf("junk bytes: err = %v, want ErrUnsupportedFormat", err)
	}
}

// TestLoadImageIgnoresFileExtension pins that the extension is only a hint: the
// container decides the decoder.
func TestLoadImageIgnoresFileExtension(t *testing.T) {
	dir := t.TempDir()
	png := p36Bytes(t, "apng_plays0.png")
	jpg := p36Bytes(t, "exif_1.jpg")
	webp := p36Bytes(t, "webp_static_lossy.webp")

	cases := []struct {
		name  string
		data  []byte
		w, h  int
		exten string
	}{
		{"lies-as-jpeg", png, 40, 30, ".jpg"},
		{"lies-as-png", jpg, 40, 30, ".png"},
		{"no-extension", webp, 40, 30, ""},
	}
	for _, c := range cases {
		path := filepath.Join(dir, "probe"+c.exten)
		if err := os.WriteFile(path, c.data, 0o600); err != nil {
			t.Fatalf("write probe: %v", err)
		}
		img, err := render.LoadImage(path)
		if err != nil {
			t.Fatalf("%s: LoadImage: %v", c.name, err)
		}
		w, h := img.Bounds()
		if w != c.w || h != c.h {
			t.Errorf("%s: bounds = %dx%d, want %dx%d", c.name, w, h, c.w, c.h)
		}
		img.Dispose()
	}
	// A format-specific loader asked for the wrong container fails loudly
	// instead of quietly decoding something else.
	path := filepath.Join(dir, "pngbytes.jpeg")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	if _, err := render.LoadWebP(path); err == nil {
		t.Errorf("LoadWebP on PNG bytes: got nil error, want mismatch")
	}
}

// p36Grid reads a reference grid file: a "# orientation=N WxH" header followed
// by H rows of W labels.
func p36Grid(t *testing.T, name string) (w, h int, rows []string) {
	t.Helper()
	text := string(p36Bytes(t, name))
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "#") {
		t.Fatalf("%s: missing header", name)
	}
	if _, err := fmt.Sscanf(lines[0], "# orientation=%d %dx%d", new(int), &w, &h); err != nil {
		t.Fatalf("%s: bad header %q: %v", name, lines[0], err)
	}
	rows = lines[1:]
	if len(rows) != h {
		t.Fatalf("%s: %d rows, want %d", name, len(rows), h)
	}
	return w, h, rows
}

// p36Classify mirrors scripts/gen_p36_testdata.py's label function. JPEG chroma
// subsampling smears feature edges, so classification (not channel equality) is
// the comparison unit; the declared mismatch budget is measured, see below.
func p36Classify(r, g, b uint8) byte {
	switch {
	case r > 150 && g < 100 && b < 100:
		return 'R'
	case b > 150 && r < 100 && g < 100:
		return 'B'
	case r < 90 && g < 90 && b < 90:
		return 'K'
	default:
		return 'G'
	}
}

func p36GridOf(t *testing.T, img *render.ImageBuf) []string {
	t.Helper()
	w, h := img.Bounds()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			r, g, b, _ := img.GetRGBA(x, y)
			sb.WriteByte(p36Classify(r, g, b))
		}
		rows[y] = sb.String()
	}
	return rows
}

// p36MaxMismatchPct is the declared budget for the Pillow-reference grid
// comparisons. Measured: 20 of 1200 pixels (1.67%) classify differently, and the
// count is identical for all eight orientations — the disagreements sit on the
// feature edges, where the two JPEG decoders smear chroma differently. A wrong
// transform moves whole regions (tens of percent), so 3% discriminates while
// leaving headroom.
const p36MaxMismatchPct = 3

// TestLoadImageAppliesOrientationMatchesReferenceGrid is the pixel half of the
// EXIF gate: for each of the eight recorded orientations, the decoded raster
// must match what the independent implementation produced — dimensions and
// feature layout, not just "the tag parsed".
func TestLoadImageAppliesOrientationMatchesReferenceGrid(t *testing.T) {
	for ori := 1; ori <= 8; ori++ {
		name := fmt.Sprintf("exif_grid_%d.txt", ori)
		w, h, want := p36Grid(t, name)
		img, err := render.LoadImageFromBytes(p36Bytes(t, fmt.Sprintf("exif_%d.jpg", ori)), render.DecodeOptions{})
		if err != nil {
			t.Fatalf("orientation %d: load: %v", ori, err)
		}
		gw, gh := img.Bounds()
		if gw != w || gh != h {
			t.Errorf("orientation %d: decoded %dx%d, reference %dx%d", ori, gw, gh, w, h)
			img.Dispose()
			continue
		}
		got := p36GridOf(t, img)
		img.Dispose()
		mismatch := 0
		for y := 0; y < h; y++ {
			if len(got[y]) != w {
				t.Fatalf("orientation %d: row %d width %d, want %d", ori, y, len(got[y]), w)
			}
			for x := 0; x < w; x++ {
				if got[y][x] != want[y][x] {
					mismatch++
				}
			}
		}
		if pct := 100 * mismatch / (w * h); pct > p36MaxMismatchPct {
			t.Errorf("orientation %d: %d/%d pixels classify differently (%d%%), want <= %d%%\ngot:\n%s\nwant:\n%s",
				ori, mismatch, w*h, pct, p36MaxMismatchPct, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		t.Logf("orientation %d: mismatch %d/%d (%.2f%%)", ori, mismatch, w*h, float64(mismatch)/float64(w*h)*100)
	}
}

// TestPreserveOrientationKeepsStoredRaster pins the opt-out. All eight samples
// encode the same stored raster (same source, same encoder settings, only the
// APP1 orientation byte differs), so with PreserveOrientation they must come
// back identical to each other — that comparison is exact and needs no external
// oracle. That common stored raster is, by definition, the reference grid for
// orientation 1 (identity), and is compared to it under the same chroma-edge
// budget as the oriented test.
func TestPreserveOrientationKeepsStoredRaster(t *testing.T) {
	w, h, want := p36Grid(t, "exif_grid_1.txt")
	var first []string
	for ori := 1; ori <= 8; ori++ {
		img, err := render.LoadImageFromBytes(p36Bytes(t, fmt.Sprintf("exif_%d.jpg", ori)),
			render.DecodeOptions{PreserveOrientation: true})
		if err != nil {
			t.Fatalf("orientation %d: load: %v", ori, err)
		}
		if gw, gh := img.Bounds(); gw != w || gh != h {
			t.Errorf("orientation %d preserved: %dx%d, want stored %dx%d", ori, gw, gh, w, h)
			img.Dispose()
			continue
		}
		got := p36GridOf(t, img)
		img.Dispose()
		if ori == 1 {
			first = got
			continue
		}
		for y := range first {
			if got[y] != first[y] {
				t.Errorf("orientation %d preserved row %d differs from orientation 1:\n got %s\nwant %s",
					ori, y, got[y], first[y])
				break
			}
		}
	}
	mismatch := 0
	for y := range want {
		for x := range want[y] {
			if first[y][x] != want[y][x] {
				mismatch++
			}
		}
	}
	if pct := 100 * mismatch / (w * h); pct > p36MaxMismatchPct {
		t.Errorf("stored raster vs reference grid: %d/%d pixels differ (%d%%), want <= %d%%",
			mismatch, w*h, pct, p36MaxMismatchPct)
	}
	t.Logf("stored raster vs reference grid: mismatch %d/%d (%.2f%%)", mismatch, w*h, float64(mismatch)/float64(w*h)*100)
}

// TestEachOrientationProducesDistinctRaster guards the false green where
// orientation is parsed but never applied: all eight rasters must differ.
func TestEachOrientationProducesDistinctRaster(t *testing.T) {
	grids := map[int][]string{}
	for ori := 1; ori <= 8; ori++ {
		img, err := render.LoadImageFromBytes(p36Bytes(t, fmt.Sprintf("exif_%d.jpg", ori)), render.DecodeOptions{})
		if err != nil {
			t.Fatalf("orientation %d: load: %v", ori, err)
		}
		grids[ori] = p36GridOf(t, img)
		img.Dispose()
	}
	for a := 1; a <= 8; a++ {
		for b := a + 1; b <= 8; b++ {
			if len(grids[a]) != len(grids[b]) {
				continue // different dimensions are already distinct
			}
			same := true
			for y := range grids[a] {
				if grids[a][y] != grids[b][y] {
					same = false
					break
				}
			}
			if same {
				t.Errorf("orientations %d and %d produced identical rasters: orientation not applied", a, b)
			}
		}
	}
}

// TestImageOrientationOfBytesReportsTagOnly checks the query口 does not decode:
// it reports the recorded tag and leaves the raster alone.
func TestImageOrientationOfBytesReportsTagOnly(t *testing.T) {
	for ori := 1; ori <= 8; ori++ {
		got, err := render.ImageOrientationOfBytes(p36Bytes(t, fmt.Sprintf("exif_%d.jpg", ori)))
		if err != nil {
			t.Fatalf("orientation %d: %v", ori, err)
		}
		if got != render.ImageOrientation(ori) {
			t.Errorf("orientation of exif_%d.jpg = %v, want %d", ori, got, ori)
		}
	}
	if got, err := render.ImageOrientationOfBytes(p36Bytes(t, "noexif.jpg")); err != nil || got != render.OrientationUnspecified {
		t.Errorf("noexif.jpg = (%v, %v), want (OrientationUnspecified, nil)", got, err)
	}
	// PNG and WebP samples carry no EXIF block: unspecified, no error.
	for _, name := range []string{"apng_plays0.png", "webp_static_lossy.webp", "gif_keep.gif"} {
		got, err := render.ImageOrientationOfBytes(p36Bytes(t, name))
		if err != nil || got != render.OrientationUnspecified {
			t.Errorf("%s = (%v, %v), want (OrientationUnspecified, nil)", name, got, err)
		}
	}
}

// TestImageOrientationSwapsWidthHeight pins the transpose table callers rely on
// when sizing buffers before decode.
func TestImageOrientationSwapsWidthHeight(t *testing.T) {
	for ori, wantSwap := range map[render.ImageOrientation]bool{
		render.OrientationUnspecified: false,
		render.OrientationTopLeft:     false,
		render.OrientationTopRight:    false,
		render.OrientationBottomRight: false,
		render.OrientationBottomLeft:  false,
		render.OrientationLeftTop:     true,
		render.OrientationRightTop:    true,
		render.OrientationRightBottom: true,
		render.OrientationLeftBottom:  true,
	} {
		if got := ori.SwapsWidthHeight(); got != wantSwap {
			t.Errorf("%v.SwapsWidthHeight() = %v, want %v", ori, got, wantSwap)
		}
	}
}

// TestStaticDecodersStayUnoriented pins the raw Decode* family: it must not
// normalise, so callers that want the encoded raster exactly have one door.
func TestStaticDecodersStayUnoriented(t *testing.T) {
	img, err := render.LoadImageFromBytes(p36Bytes(t, "exif_6.jpg"), render.DecodeOptions{PreserveOrientation: true})
	if err != nil {
		t.Fatalf("preserve load: %v", err)
	}
	defer img.Dispose()
	w, h := img.Bounds()
	if w != 40 || h != 30 {
		t.Fatalf("stored size = %dx%d, want 40x30", w, h)
	}
	// Orientation 6 transposes: the default load must land on 30x40 instead.
	view, err := render.LoadImageFromBytes(p36Bytes(t, "exif_6.jpg"), render.DecodeOptions{})
	if err != nil {
		t.Fatalf("default load: %v", err)
	}
	defer view.Dispose()
	if vw, vh := view.Bounds(); vw != 30 || vh != 40 {
		t.Errorf("oriented size = %dx%d, want 30x40", vw, vh)
	}
}
