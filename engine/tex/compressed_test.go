package tex

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
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

// F: offscreen golden stands in for the window.
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

// S64 frozen cases: ASTC 4x4 is the second kind (ETC2 stays reserved).
type s64FileDef struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Blocks int    `json:"blocks"`
	Upload int    `json:"upload"`
	Pixels int    `json:"pixels"`
}

type s64CasesFile struct {
	Format       string            `json:"format"`
	VkFormat     uint32            `json:"vk_format"`
	BlockW       int               `json:"block_w"`
	BlockH       int               `json:"block_h"`
	BlockBytes   int               `json:"block_bytes"`
	Tolerance    int               `json:"tolerance"`
	MaxDimension int               `json:"max_dimension"`
	MaxPixels    int               `json:"max_pixels"`
	Files        []s64FileDef      `json:"files"`
	ProxyRGBA    [4]int            `json:"proxy_rgba"`
	PinkBlockHex string            `json:"pink_block_hex"`
	SchemeReason map[string]string `json:"scheme_reasons"`
}

func loadS64Cases(t *testing.T) s64CasesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "s64_cases.json"))
	if err != nil {
		t.Fatalf("read s64_cases.json: %v", err)
	}
	var c s64CasesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode s64_cases.json: %v", err)
	}
	if len(c.Files) == 0 {
		t.Fatal("s64_cases.json has no files")
	}
	return c
}

// fakeBC1Red packs a solid-red BC1 payload for w*h (driver stand-in).
func fakeBC1Red(w, h int) []byte {
	nb := (w / blockW) * (h / blockH)
	out := make([]byte, nb*blockBytesBC1)
	for i := range nb {
		binary.LittleEndian.PutUint16(out[i*8:], 0xF800)
		binary.LittleEndian.PutUint16(out[i*8+2:], 0xF800)
	}
	return out
}

// S64-A: ASTC files land on the frozen numbers; CPU shows the pink proxy.
func TestS64ASTCDecodeFromCases(t *testing.T) {
	c := loadS64Cases(t)
	if c.Tolerance != maxDiffTolerance {
		t.Fatalf("tolerance = %d, want frozen %d", c.Tolerance, maxDiffTolerance)
	}
	if FormatASTCRGBA4x4.String() != c.Format {
		t.Fatalf("format = %q, want %q", FormatASTCRGBA4x4.String(), c.Format)
	}
	if FormatASTCRGBA4x4.VkFormat() != c.VkFormat {
		t.Fatalf("vk = %d, want %d", FormatASTCRGBA4x4.VkFormat(), c.VkFormat)
	}
	if bw, bh := FormatASTCRGBA4x4.BlockExtent(); bw != c.BlockW || bh != c.BlockH {
		t.Fatalf("extent = %dx%d, want %dx%d", bw, bh, c.BlockW, c.BlockH)
	}
	if FormatASTCRGBA4x4.BlockBytes() != c.BlockBytes {
		t.Fatalf("block bytes = %d, want %d", FormatASTCRGBA4x4.BlockBytes(), c.BlockBytes)
	}
	for _, f := range c.Files {
		im, err := LoadKTX2(filepath.Join("testdata", f.Name))
		if err != nil {
			t.Errorf("%s: LoadKTX2: %v", f.Name, err)
			continue
		}
		if im.Format() != FormatASTCRGBA4x4 {
			t.Errorf("%s: format = %v, want ASTC", f.Name, im.Format())
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
		if im.Ratio() != 4 {
			t.Errorf("%s: ratio = %v, want 4", f.Name, im.Ratio())
		}
		px := im.Pixels()
		for y := 0; y < f.Height; y++ {
			for x := 0; x < f.Width; x++ {
				if got := rgbaAt(px, f.Width, x, y); got != c.ProxyRGBA {
					t.Fatalf("%s: pixel (%d,%d) = %v, want pink proxy %v", f.Name, x, y, got, c.ProxyRGBA)
				}
			}
		}
		if !IsPink(im) {
			t.Errorf("%s: IsPink=false, want true for the ASTC CPU proxy", f.Name)
		}
	}
}

// S64-B: bad magic is BadData; bad scheme is Unsupported with the reason.
func TestS64RejectMagicAndScheme(t *testing.T) {
	c := loadS64Cases(t)
	valid := loadValid(t, "astc_red_4x4.ktx2")
	_, err := ParseKTX2(patchU32(valid, 0, 0xdeadbeef))
	expectCode(t, "astc bad-magic", err, core.CodeBadData)
	if err != nil && !strings.Contains(err.Error(), "identifier") {
		t.Errorf("bad-magic reason = %q, want identifier named", err.Error())
	}
	schemes := []struct {
		name string
		v    uint32
	}{
		{"basis-lz", SchemeBasisLZ}, {"zstd", SchemeZstd}, {"zlib", SchemeZlib},
	}
	for _, s := range schemes {
		_, err := ParseKTX2(patchU32(valid, 44, s.v))
		expectCode(t, "astc scheme "+s.name, err, core.CodeUnsupported)
		if err == nil {
			continue
		}
		if !strings.Contains(err.Error(), s.name) {
			t.Errorf("scheme %s reason = %q, want scheme named", s.name, err.Error())
		}
		if want, ok := c.SchemeReason[s.name]; ok && !strings.Contains(err.Error(), want) {
			t.Errorf("scheme %s reason = %q, want %q", s.name, err.Error(), want)
		}
	}
	// BC1 names its scheme the same way.
	_, err = ParseKTX2(patchU32(loadValid(t, "solid_red_4x4.ktx2"), 44, SchemeZstd))
	expectCode(t, "bc1 scheme zstd", err, core.CodeUnsupported)
	if err != nil && !strings.Contains(err.Error(), "zstd") {
		t.Errorf("bc1 scheme reason = %q, want zstd named", err.Error())
	}
}

// S64-C: same picture is 1/4 (ASTC) to 1/8 (BC1) of RGBA8.
func TestS64VramRatioQuarterToEighth(t *testing.T) {
	bc1, err := LoadKTX2(filepath.Join("testdata", "solid_256x256.ktx2"))
	if err != nil {
		t.Fatalf("bc1: %v", err)
	}
	astc, err := LoadKTX2(filepath.Join("testdata", "astc_red_256x256.ktx2"))
	if err != nil {
		t.Fatalf("astc: %v", err)
	}
	if bc1.PixelSize() != astc.PixelSize() {
		t.Fatalf("decoded = %d vs %d, want same picture", bc1.PixelSize(), astc.PixelSize())
	}
	for name, im := range map[string]*Image{"bc1": bc1, "astc": astc} {
		frac := float64(im.UploadSize()) / float64(im.PixelSize())
		if frac < 1.0/8 || frac > 1.0/4 {
			t.Errorf("%s: vram fraction = %v, want within 1/8..1/4", name, frac)
		}
	}
	if bc1.Ratio() != 8 || astc.Ratio() != 4 {
		t.Fatalf("ratios = %v/%v, want 8/4", bc1.Ratio(), astc.Ratio())
	}
	t.Logf("tex-s64-vram: bc1 %dB astc %dB decoded %dB (1/8 and 1/4)",
		bc1.UploadSize(), astc.UploadSize(), bc1.PixelSize())
}

// S64-D: normals refuse the lowest tier; colors pass; unknown stays loud.
func TestS64NormalLowestTierBlocked(t *testing.T) {
	if err := CheckUsage(FormatBC1RGBAUnorm, UsageNormal); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bc1 normal code = %v, want invalid-arg", core.CodeOf(err))
	} else if !strings.Contains(err.Error(), "lowest") {
		t.Errorf("bc1 normal reason = %q, want lowest tier named", err.Error())
	}
	if err := CheckUsage(FormatASTCRGBA4x4, UsageNormal); err != nil {
		t.Errorf("astc normal: %v, want nil", err)
	}
	if err := CheckUsage(FormatBC1RGBAUnorm, UsageColor); err != nil {
		t.Errorf("bc1 color: %v, want nil", err)
	}
	if err := CheckUsage(FormatASTCRGBA4x4, UsageColor); err != nil {
		t.Errorf("astc color: %v, want nil", err)
	}
	if err := CheckUsage(FormatUnknown, UsageColor); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("unknown code = %v, want unsupported", core.CodeOf(err))
	}
}

// S64-E: routing table plus pink fallback that never panics.
func TestS64TranscodeRouteAndPink(t *testing.T) {
	if got := TranscodeTargetFor(FormatASTCRGBA4x4, CapsDesktopBC1); got != TargetBC1 {
		t.Errorf("astc on desktop = %v, want bc1", got)
	}
	if got := TranscodeTargetFor(FormatASTCRGBA4x4, CapsMobileASTC); got != TargetNative {
		t.Errorf("astc on mobile = %v, want native", got)
	}
	if got := TranscodeTargetFor(FormatASTCRGBA4x4, CapsNone); got != TargetPink {
		t.Errorf("astc on none = %v, want pink", got)
	}
	if got := TranscodeTargetFor(FormatBC1RGBAUnorm, CapsDesktopBC1); got != TargetNative {
		t.Errorf("bc1 on desktop = %v, want native", got)
	}
	if got := TranscodeTargetFor(FormatBC1RGBAUnorm, CapsMobileASTC); got != TargetPink {
		t.Errorf("bc1 on mobile = %v, want pink", got)
	}
	if got := TranscodeTargetFor(FormatUnknown, CapsUniversal); got != TargetPink {
		t.Errorf("unknown = %v, want pink", got)
	}
	im, err := LoadKTX2(filepath.Join("testdata", "astc_red_4x4.ktx2"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Native returns the same picture untouched.
	if same, err := Transcode(im, CapsMobileASTC, nil); err != nil || same != im {
		t.Errorf("native = %p,%v, want same picture nil error", same, err)
	}
	// Driver hook re-packs to red BC1.
	red := func(blocks []byte, w, h int) ([]byte, error) { return fakeBC1Red(w, h), nil }
	out, err := Transcode(im, CapsDesktopBC1, red)
	if err != nil {
		t.Fatalf("transcode: %v", err)
	}
	if out.Format() != FormatBC1RGBAUnorm || out.Width() != 4 || out.Height() != 4 {
		t.Fatalf("transcoded = %v %dx%d, want BC1 4x4", out.Format(), out.Width(), out.Height())
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if got := rgbaAt(out.Pixels(), 4, x, y); got != [4]int{255, 0, 0, 255} {
				t.Fatalf("transcoded (%d,%d) = %v, want red", x, y, got)
			}
		}
	}
	if IsPink(out) {
		t.Error("transcoded shows pink, want red")
	}
	// Failing hook serves pink with the cause, never nil, never a panic.
	badFn := func(blocks []byte, w, h int) ([]byte, error) {
		return nil, errors.New("driver out of memory")
	}
	fallback, ferr := Transcode(im, CapsDesktopBC1, badFn)
	if ferr == nil {
		t.Error("failing transcode err=nil, want the driver cause")
	}
	if fallback == nil || fallback.Width() != 4 || fallback.Height() != 4 || !IsPink(fallback) {
		t.Error("failing transcode did not serve same-size pink")
	}
	// Missing hook and short payload also fall back to pink.
	for name, fn := range map[string]Transcoder{"nil-hook": nil,
		"short": func(blocks []byte, w, h int) ([]byte, error) { return []byte{1, 2, 3}, nil }} {
		got, gerr := Transcode(im, CapsDesktopBC1, fn)
		if gerr == nil {
			t.Errorf("%s: err=nil, want a reason", name)
		}
		if got == nil || !IsPink(got) {
			t.Errorf("%s: no pink fallback", name)
		}
	}
	// Neither card decodes: BC1 on mobile goes pink too.
	bc1, err := LoadKTX2(filepath.Join("testdata", "solid_red_4x4.ktx2"))
	if err != nil {
		t.Fatalf("bc1 load: %v", err)
	}
	got, gerr := Transcode(bc1, CapsMobileASTC, red)
	if gerr == nil || got == nil || !IsPink(got) {
		t.Errorf("bc1-on-mobile = %v,%v, want pink plus reason", got, gerr)
	}
	// Nil picture never panics: shared placeholder plus InvalidArg.
	got, gerr = Transcode(nil, CapsDesktopBC1, red)
	expectCode(t, "nil image", gerr, core.CodeInvalidArg)
	if got != PlaceholderImage() {
		t.Error("nil image did not serve the shared placeholder")
	}
}

// S64-F: big pictures transcode off-thread; whole ASTC picture scans pink.
func TestS64LargeAsyncOffMainThread(t *testing.T) {
	big, err := LoadKTX2(filepath.Join("testdata", "astc_red_256x256.ktx2"))
	if err != nil {
		t.Fatalf("big: %v", err)
	}
	small, err := LoadKTX2(filepath.Join("testdata", "astc_red_4x4.ktx2"))
	if err != nil {
		t.Fatalf("small: %v", err)
	}
	if !NeedsAsync(big) {
		t.Error("256x256 NeedsAsync=false, want true so it stays off the play thread")
	}
	if NeedsAsync(small) {
		t.Error("4x4 NeedsAsync=true, want false")
	}
	if NeedsAsync(nil) {
		t.Error("nil NeedsAsync=true, want false")
	}
	red := func(blocks []byte, w, h int) ([]byte, error) { return fakeBC1Red(w, h), nil }
	select {
	case r := <-TranscodeAsync(big, CapsDesktopBC1, red):
		if r.Err != nil || r.Fallback {
			t.Errorf("async ok: err=%v fallback=%v, want clean native-rate path", r.Err, r.Fallback)
		}
		if r.Image == nil || r.Image.Format() != FormatBC1RGBAUnorm {
			t.Fatalf("async image = %v, want transcoded BC1", r.Image)
		}
		if got := rgbaAt(r.Image.Pixels(), 256, 128, 64); got != [4]int{255, 0, 0, 255} {
			t.Errorf("async spot = %v, want red", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("async transcode timed out")
	}
	select {
	case r := <-TranscodeAsync(big, CapsNone, red):
		if r.Err == nil || !r.Fallback || r.Image == nil || !IsPink(r.Image) {
			t.Errorf("async fail: err=%v fallback=%v pink=%v, want pink plus cause",
				r.Err, r.Fallback, IsPink(r.Image))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("async fallback timed out")
	}
	// Offscreen whole picture: every ASTC pixel is the pink proxy and the
	// grid math holds for all three S64 files.
	c := loadS64Cases(t)
	for _, f := range c.Files {
		im, err := LoadKTX2(filepath.Join("testdata", f.Name))
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		px := im.Pixels()
		bad := 0
		for y := 0; y < f.Height; y++ {
			for x := 0; x < f.Width; x++ {
				if got := rgbaAt(px, f.Width, x, y); got != c.ProxyRGBA {
					bad++
				}
				col, ok := im.At(x, y)
				if !ok {
					t.Fatalf("%s (%d,%d): At ok=false", f.Name, x, y)
				}
				if wantColor(col) != c.ProxyRGBA {
					t.Fatalf("%s (%d,%d): At=%v, want pink proxy", f.Name, x, y, wantColor(col))
				}
			}
		}
		if bad > 0 {
			t.Errorf("%s: %d non-pink pixels, want all pink", f.Name, bad)
		}
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
	t.Logf("tex-s64-offscreen: 3 astc pictures all-pink proxy, grid 4x4 exact")
}
