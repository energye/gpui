package tex

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

// Tolerance is frozen at 0: solid BC1 blocks decode bit-exact, no
// smoothing allowed. compressed_cases.json carries the same 0; the test
// fails if the file ever drifts.
const maxDiffTolerance = 0

type fileDef struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Blocks int    `json:"blocks"`
	Upload int    `json:"upload"`
	Pixels int    `json:"pixels"`
}

type spotDef struct {
	File string `json:"file"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	RGBA [4]int `json:"rgba"`
}

// quadrantDef holds one frozen checker: TL/TR/BL/BR split at half width/height.
type quadrantDef struct {
	TL [4]int `json:"tl"`
	TR [4]int `json:"tr"`
	BL [4]int `json:"bl"`
	BR [4]int `json:"br"`
}

type casesFile struct {
	Format       string                 `json:"format"`
	VkFormat     uint32                 `json:"vk_format"`
	BlockW       int                    `json:"block_w"`
	BlockH       int                    `json:"block_h"`
	BlockBytes   int                    `json:"block_bytes"`
	Tolerance    int                    `json:"tolerance"`
	MaxDimension int                    `json:"max_dimension"`
	MaxPixels    int                    `json:"max_pixels"`
	Files        []fileDef              `json:"files"`
	Solids       map[string][4]int      `json:"solids"`
	Quadrants    map[string]quadrantDef `json:"quadrants"`
	Spots        []spotDef              `json:"spots"`
}

func loadCases(t *testing.T) casesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "compressed_cases.json"))
	if err != nil {
		t.Fatalf("read compressed_cases.json: %v", err)
	}
	var c casesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode compressed_cases.json: %v", err)
	}
	if len(c.Files) == 0 {
		t.Fatal("compressed_cases.json has no files")
	}
	return c
}

func findFile(t *testing.T, c casesFile, name string) fileDef {
	t.Helper()
	for _, f := range c.Files {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("compressed_cases.json has no %s", name)
	return fileDef{}
}

func rgbaAt(px []byte, w, x, y int) [4]int {
	off := (y*w + x) * 4
	return [4]int{int(px[off]), int(px[off+1]), int(px[off+2]), int(px[off+3])}
}

func diffRGBA(a, b [4]int) int {
	m := 0
	for i := 0; i < 4; i++ {
		d := a[i] - b[i]
		if d < 0 {
			d = -d
		}
		if d > m {
			m = d
		}
	}
	return m
}

func loadValid(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}

// cloned copies b so patch helpers never alias the valid golden bytes.
func cloned(b []byte) []byte { return append([]byte(nil), b...) }

func patchU32(b []byte, off int, v uint32) []byte {
	out := cloned(b)
	binary.LittleEndian.PutUint32(out[off:off+4], v)
	return out
}

func patchU64(b []byte, off int, v uint64) []byte {
	out := cloned(b)
	binary.LittleEndian.PutUint64(out[off:off+8], v)
	return out
}

// quadrantWant picks the frozen color for (x, y) without re-spelling the split.
func quadrantWant(q quadrantDef, w, h, x, y int) [4]int {
	switch {
	case x >= w/2 && y < h/2:
		return q.TR
	case x < w/2 && y >= h/2:
		return q.BL
	case x >= w/2 && y >= h/2:
		return q.BR
	default:
		return q.TL
	}
}

// wantColor converts one At pixel to the comparable [4]int form.
func wantColor(c core.Color) [4]int {
	r, g, b, a := c.ToBytes()
	return [4]int{int(r), int(g), int(b), int(a)}
}

// expectCode fails unless err carries want; nil always fails.
func expectCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

// A: valid headers land on the frozen numbers, blocks upload as-is.
func TestCompressedDecodeFromCases(t *testing.T) {
	c := loadCases(t)
	if c.Tolerance != maxDiffTolerance {
		t.Fatalf("tolerance = %d, want frozen %d", c.Tolerance, maxDiffTolerance)
	}
	if FormatBC1RGBAUnorm.String() != c.Format {
		t.Fatalf("format = %q, want %q", FormatBC1RGBAUnorm.String(), c.Format)
	}
	if FormatBC1RGBAUnorm.VkFormat() != c.VkFormat {
		t.Fatalf("vk = %d, want %d", FormatBC1RGBAUnorm.VkFormat(), c.VkFormat)
	}
	if bw, bh := FormatBC1RGBAUnorm.BlockExtent(); bw != c.BlockW || bh != c.BlockH {
		t.Fatalf("extent = %dx%d, want %dx%d", bw, bh, c.BlockW, c.BlockH)
	}
	if FormatBC1RGBAUnorm.BlockBytes() != c.BlockBytes {
		t.Fatalf("block bytes = %d, want %d", FormatBC1RGBAUnorm.BlockBytes(), c.BlockBytes)
	}
	for _, f := range c.Files {
		im, err := LoadKTX2(filepath.Join("testdata", f.Name))
		if err != nil {
			t.Errorf("%s: LoadKTX2: %v", f.Name, err)
			continue
		}
		if im.Format() != FormatBC1RGBAUnorm {
			t.Errorf("%s: format = %v, want BC1", f.Name, im.Format())
		}
		if im.Width() != f.Width || im.Height() != f.Height {
			t.Errorf("%s: size = %dx%d, want %dx%d", f.Name, im.Width(), im.Height(), f.Width, f.Height)
		}
		if im.BlockCount() != f.Blocks {
			t.Errorf("%s: blocks = %d, want %d", f.Name, im.BlockCount(), f.Blocks)
		}
		if im.UploadSize() != f.Upload {
			t.Errorf("%s: upload = %d, want %d", f.Name, im.UploadSize(), f.Upload)
		}
		if im.PixelSize() != f.Pixels {
			t.Errorf("%s: pixels = %d, want %d", f.Name, im.PixelSize(), f.Pixels)
		}
		if im.Ratio() != 8 {
			t.Errorf("%s: ratio = %v, want 8", f.Name, im.Ratio())
		}
	}
	// Solids decode bit-exact across every pixel.
	for name, want := range c.Solids {
		f := findFile(t, c, name)
		im, err := LoadKTX2(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		px := im.Pixels()
		for y := 0; y < f.Height; y++ {
			for x := 0; x < f.Width; x++ {
				if got := rgbaAt(px, f.Width, x, y); got != want {
					t.Fatalf("%s: pixel (%d,%d) = %v, want %v", name, x, y, got, want)
				}
			}
		}
	}
	// Checker quadrants land on their frozen colors.
	for name, q := range c.Quadrants {
		f := findFile(t, c, name)
		im, err := LoadKTX2(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		px := im.Pixels()
		for y := 0; y < f.Height; y++ {
			for x := 0; x < f.Width; x++ {
				want := quadrantWant(q, f.Width, f.Height, x, y)
				if got := rgbaAt(px, f.Width, x, y); got != want {
					t.Fatalf("%s: pixel (%d,%d) = %v, want %v", name, x, y, got, want)
				}
			}
		}
	}
	// Spots pin exact positions through At (core.Color boundary).
	for _, s := range c.Spots {
		im, err := LoadKTX2(filepath.Join("testdata", s.File))
		if err != nil {
			t.Fatalf("%s: %v", s.File, err)
		}
		got, ok := im.At(s.X, s.Y)
		if !ok {
			t.Fatalf("%s (%d,%d): At ok=false", s.File, s.X, s.Y)
		}
		if wantColor(got) != s.RGBA {
			t.Errorf("%s (%d,%d) = %v, want %v", s.File, s.X, s.Y, wantColor(got), s.RGBA)
		}
	}
}

// B: empty/zero/huge/corrupt inputs never panic, always a core code.
func TestCompressedEdgesNoCrash(t *testing.T) {
	_, err := ParseKTX2(nil)
	expectCode(t, "nil data", err, core.CodeInvalidArg)
	_, err = ParseKTX2([]byte{})
	expectCode(t, "empty data", err, core.CodeInvalidArg)
	_, err = LoadKTX2("")
	expectCode(t, "empty path", err, core.CodeInvalidArg)
	_, err = LoadKTX2(filepath.Join("testdata", "no_such.ktx2"))
	expectCode(t, "missing file", err, core.CodeNotFound)
	valid := loadValid(t, "solid_red_4x4.ktx2")
	badData := []struct {
		name string
		data []byte
	}{
		{"truncated", valid[:40]},
		{"bad-identifier", patchU32(valid, 0, 0xdeadbeef)},
		{"zero-dims", patchU32(patchU32(valid, 20, 0), 24, 0)},
		{"odd-width", patchU32(valid, 20, 6)},
		{"odd-height", patchU32(valid, 24, 6)},
		{"bad-typeSize", patchU32(valid, 16, 4)},
		{"level-mismatch", patchU64(valid, 88, 7)},
		{"level-overflow", patchU64(valid, 80, uint64(len(valid)+100))},
		{"dfd-overflow", patchU32(patchU32(valid, 48, uint32(len(valid)+10)), 52, 8)},
	}
	for _, k := range badData {
		_, err := ParseKTX2(k.data)
		expectCode(t, k.name, err, core.CodeBadData)
	}
	unsupported := []struct {
		name string
		data []byte
	}{
		{"vk-uncompressed", patchU32(valid, 12, 37)},
		{"vk-etc2", patchU32(valid, 12, 147)},
		{"vk-astc", patchU32(valid, 12, 74)},
		{"super-basis", patchU32(valid, 44, 1)},
		{"super-zstd", patchU32(valid, 44, 2)},
		{"depth-3d", patchU32(valid, 28, 1)},
		{"array", patchU32(valid, 32, 1)},
		{"cubemap", patchU32(valid, 36, 6)},
		{"mipmaps", patchU32(valid, 40, 4)},
	}
	for _, k := range unsupported {
		_, err := ParseKTX2(k.data)
		expectCode(t, k.name, err, core.CodeUnsupported)
	}
	huge := patchU32(patchU32(valid, 20, 16384), 24, 16384)
	_, err = ParseKTX2(huge)
	expectCode(t, "huge", err, core.CodeOutOfMemory)
	// Basis stays reserved: present files report unsupported, missing stays not-found.
	present := filepath.Join("testdata", "solid_red_4x4.ktx2")
	_, err = ParseBasis(valid)
	expectCode(t, "ParseBasis", err, core.CodeUnsupported)
	_, err = ParseBasis(nil)
	expectCode(t, "ParseBasis empty", err, core.CodeInvalidArg)
	_, err = LoadBasis(present)
	expectCode(t, "LoadBasis present", err, core.CodeUnsupported)
	_, err = LoadBasis("")
	expectCode(t, "LoadBasis empty", err, core.CodeInvalidArg)
	_, err = LoadBasis(filepath.Join("testdata", "no_such.ktx2"))
	expectCode(t, "LoadBasis missing", err, core.CodeNotFound)
	// At never panics: nil, negative, and past-edge all fail closed.
	var nilIm *Image
	if _, ok := nilIm.At(0, 0); ok {
		t.Error("nil At ok=true, want false")
	}
	im, err := LoadKTX2(filepath.Join("testdata", "solid_red_4x4.ktx2"))
	if err != nil {
		t.Fatalf("load for At: %v", err)
	}
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 4}, {100, 100}} {
		if _, ok := im.At(p[0], p[1]); ok {
			t.Errorf("At%v ok=true, want false", p)
		}
	}
	if im.Format() == FormatUnknown || FormatUnknown.String() == "" {
		t.Error("format names missing")
	}
}

// C: decoded bytes stay within tolerance of the frozen reference (0 = exact).
// Both backends share these bytes, so this is the two-sides evidence.
func TestCompressedPixelsWithinTolerance(t *testing.T) {
	c := loadCases(t)
	maxDiff := 0
	bad := 0
	total := 0
	for _, f := range c.Files {
		im, err := LoadKTX2(filepath.Join("testdata", f.Name))
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		px := im.Pixels()
		for y := 0; y < f.Height; y++ {
			for x := 0; x < f.Width; x++ {
				want := c.Solids[f.Name]
				if q, ok := c.Quadrants[f.Name]; ok {
					want = quadrantWant(q, f.Width, f.Height, x, y)
				}
				got := rgbaAt(px, f.Width, x, y)
				if d := diffRGBA(got, want); d > maxDiff {
					maxDiff = d
				}
				if diffRGBA(got, want) > c.Tolerance {
					bad++
				}
				total++
				// At agrees with the byte slice on every pixel.
				col, ok := im.At(x, y)
				if !ok {
					t.Fatalf("%s (%d,%d): At ok=false", f.Name, x, y)
				}
				if wantColor(col) != got {
					t.Fatalf("%s (%d,%d): At=%v bytes=%v", f.Name, x, y, wantColor(col), got)
				}
			}
		}
	}
	t.Logf("tex-tolerance: maxDiff=%d bad=%d/%d tolerance=%d", maxDiff, bad, total, c.Tolerance)
	if maxDiff > maxDiffTolerance || bad > 0 {
		t.Fatalf("pixels drift: maxDiff=%d bad=%d, want <= %d and 0", maxDiff, bad, maxDiffTolerance)
	}
}

// D: a big picture decodes with measured cost and honest VRAM numbers.
func TestCompressedPerfLarge(t *testing.T) {
	c := loadCases(t)
	f := findFile(t, c, "solid_256x256.ktx2")
	raw := loadValid(t, f.Name)
	const reps = 20
	start := time.Now()
	for i := 0; i < reps; i++ {
		im, err := ParseKTX2(raw)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if im.UploadSize() != f.Upload || im.PixelSize() != f.Pixels {
			t.Fatalf("rep %d: upload=%d pixels=%d, want %d/%d", i, im.UploadSize(), im.PixelSize(), f.Upload, f.Pixels)
		}
	}
	el := time.Since(start)
	t.Logf("tex-256: %d parses %dx%d (upload %dB decoded %dB ratio %.0f) in %v (%.1f ms/parse)", reps, f.Width, f.Height, f.Upload, f.Pixels, float64(f.Pixels)/float64(f.Upload), el, float64(el.Milliseconds())/reps)
}

// E: long runs replay bitwise identical with no growth; bad files never stick.
func TestCompressedLongRunStable(t *testing.T) {
	raw := loadValid(t, "solid_red_4x4.ktx2")
	first, err := ParseKTX2(raw)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	for i := 0; i < 1000; i++ {
		got, err := ParseKTX2(raw)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if !got.Equal(first) {
			t.Fatalf("rep %d diverged", i)
		}
	}
	again, err := LoadKTX2(filepath.Join("testdata", "solid_red_4x4.ktx2"))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !again.Equal(first) {
		t.Fatal("file reload diverged from bytes parse")
	}
	// Copies never alias: mutating a return cannot corrupt the replay.
	probe := first.Blocks()
	probe[0] ^= 0xFF
	if first.Blocks()[0] == probe[0] {
		t.Fatal("Blocks aliases the image")
	}
	px := first.Pixels()
	px[0] ^= 0xFF
	if first.Pixels()[0] == px[0] {
		t.Fatal("Pixels aliases the image")
	}
	if !first.Equal(again) {
		t.Fatal("copy probe corrupted the image")
	}
	var nilIm *Image
	if !nilIm.Equal(nil) || nilIm.Equal(first) {
		t.Error("nil Equal wrong")
	}
}

// F: offscreen golden stands in for the window (window-exempt in W1).
// The frozen spots plus sharp quadrant edges are the evidence both
// backends share; no game_* window is built for 3.1.
func TestCompressedOffscreenGolden(t *testing.T) {
	c := loadCases(t)
	for _, s := range c.Spots {
		im, err := LoadKTX2(filepath.Join("testdata", s.File))
		if err != nil {
			t.Fatalf("%s: %v", s.File, err)
		}
		got, ok := im.At(s.X, s.Y)
		if !ok {
			t.Fatalf("%s (%d,%d): At ok=false", s.File, s.X, s.Y)
		}
		if wantColor(got) != s.RGBA {
			t.Fatalf("%s (%d,%d) = %v, want %v", s.File, s.X, s.Y, wantColor(got), s.RGBA)
		}
	}
	// Shape: checker edges are sharp at the block seam, no bleed.
	im, err := LoadKTX2(filepath.Join("testdata", "checker_8x8.ktx2"))
	if err != nil {
		t.Fatalf("checker: %v", err)
	}
	edgePairs := [][2][2]int{{{3, 3}, {4, 3}}, {{3, 4}, {4, 4}}, {{3, 0}, {4, 0}}, {{0, 3}, {0, 4}}}
	for i, p := range edgePairs {
		a, _ := im.At(p[0][0], p[0][1])
		b, _ := im.At(p[1][0], p[1][1])
		if a == b {
			t.Errorf("edge[%d] %v vs %v share %v, want a sharp seam", i, p[0], p[1], a)
		}
	}
	// Grid: every size is a multiple of the frozen 4x4 block.
	for _, f := range c.Files {
		if f.Width%c.BlockW != 0 || f.Height%c.BlockH != 0 {
			t.Errorf("%s: %dx%d not a multiple of %dx%d", f.Name, f.Width, f.Height, c.BlockW, c.BlockH)
		}
		if f.Blocks != (f.Width/c.BlockW)*(f.Height/c.BlockH) {
			t.Errorf("%s: blocks=%d, want grid %d", f.Name, f.Blocks, (f.Width/c.BlockW)*(f.Height/c.BlockH))
		}
		if f.Upload != f.Blocks*c.BlockBytes || f.Pixels != f.Width*f.Height*4 {
			t.Errorf("%s: upload/pixels mismatch", f.Name)
		}
	}
}
