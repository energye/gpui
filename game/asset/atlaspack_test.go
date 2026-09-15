package asset

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

// Tolerance is frozen at 0: shelf copies bytes verbatim, no smoothing.
const atlasDiffTolerance = 0

type spriteCase struct {
	Name   string  `json:"name"`
	W      int     `json:"w"`
	H      int     `json:"h"`
	Color  [4]int  `json:"color"`
	PivotX float64 `json:"pivotX"`
	PivotY float64 `json:"pivotY"`
	Nine   [4]int  `json:"nine"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
}

type atlasInfo struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	File   string `json:"file"`
}

type atlasBadCase struct {
	File     string `json:"file"`
	WantCode string `json:"want_code"`
}

type atlasCasesFile struct {
	Version          string         `json:"version"`
	MaxSprites       int            `json:"max_sprites"`
	MaxSize          int            `json:"max_size"`
	MaxNameLen       int            `json:"max_name_len"`
	Pad              int            `json:"pad"`
	MaxW             int            `json:"max_w"`
	MaxH             int            `json:"max_h"`
	Tolerance        int            `json:"tolerance"`
	Atlas            atlasInfo      `json:"atlas"`
	Sprites          []spriteCase   `json:"sprites"`
	BadFiles         []atlasBadCase `json:"bad_files"`
	UnsupportedFiles []atlasBadCase `json:"unsupported_files"`
}

func loadAtlasCases(t *testing.T) atlasCasesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "atlas_cases.json"))
	if err != nil {
		t.Fatalf("read atlas_cases.json: %v", err)
	}
	var c atlasCasesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode atlas_cases.json: %v", err)
	}
	if len(c.Sprites) == 0 {
		t.Fatal("atlas_cases.json has no sprites")
	}
	if c.Atlas.File == "" || len(c.BadFiles) == 0 || len(c.UnsupportedFiles) == 0 {
		t.Fatal("atlas_cases.json has no atlas/bad/unsupported files")
	}
	return c
}

func mustFindSprite(t *testing.T, c atlasCasesFile, name string) spriteCase {
	t.Helper()
	for _, s := range c.Sprites {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("atlas_cases.json has no sprite %q", name)
	return spriteCase{}
}

// fillSolid expands one frozen color to W*H RGBA8 bytes. The color stays
// in atlas_cases.json; this only constructs the Pack input, never the
// golden placement.
func fillSolid(w, h int, c [4]int) []byte {
	px := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		px[i*4] = byte(c[0])
		px[i*4+1] = byte(c[1])
		px[i*4+2] = byte(c[2])
		px[i*4+3] = byte(c[3])
	}
	return px
}

func buildAtlasInputs(c atlasCasesFile) []Input {
	ins := make([]Input, len(c.Sprites))
	for i, s := range c.Sprites {
		ins[i] = Input{
			ID: core.AssetID(s.Name), W: s.W, H: s.H,
			Pixels: fillSolid(s.W, s.H, s.Color),
			PivotX: s.PivotX, PivotY: s.PivotY, Nine: s.Nine,
		}
	}
	return ins
}

func mustPackCases(t *testing.T, c atlasCasesFile) *Atlas {
	t.Helper()
	a, err := Pack(buildAtlasInputs(c), c.MaxW, c.MaxH)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	return a
}

func expectAtlasCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

func atlasCodeFromName(s string) core.Code {
	switch s {
	case "not-found":
		return core.CodeNotFound
	case "bad-data":
		return core.CodeBadData
	case "out-of-memory":
		return core.CodeOutOfMemory
	case "unsupported":
		return core.CodeUnsupported
	case "invalid-arg":
		return core.CodeInvalidArg
	case "version-mismatch":
		return core.CodeVersionMismatch
	default:
		return core.CodeUnknown
	}
}

func diffByte(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		d = -d
	}
	return d
}

// A:小图拼大图号对,位置轴心九宫格落在冻结数上.
func TestAtlasPackFromCases(t *testing.T) {
	c := loadAtlasCases(t)
	if CurrentAtlasVersion.String() != c.Version {
		t.Fatalf("version = %v, want %v", CurrentAtlasVersion, c.Version)
	}
	if MaxSprites != c.MaxSprites || MaxAtlasSize != c.MaxSize ||
		MaxAtlasNameLen != c.MaxNameLen || AtlasPad != c.Pad {
		t.Fatalf("limits diverge from atlas_cases.json")
	}
	if c.Tolerance != atlasDiffTolerance {
		t.Fatalf("tolerance = %d, want frozen %d", c.Tolerance, atlasDiffTolerance)
	}
	a := mustPackCases(t, c)
	if a.Version() != CurrentAtlasVersion {
		t.Errorf("atlas version = %v, want %v", a.Version(), CurrentAtlasVersion)
	}
	if a.Width() != c.Atlas.Width || a.Height() != c.Atlas.Height {
		t.Errorf("atlas size = %dx%d, want %dx%d", a.Width(), a.Height(), c.Atlas.Width, c.Atlas.Height)
	}
	if a.Count() != len(c.Sprites) {
		t.Fatalf("count = %d, want %d", a.Count(), len(c.Sprites))
	}
	for _, want := range c.Sprites {
		got, ok := a.Find(core.AssetID(want.Name))
		if !ok {
			t.Errorf("%s: Find ok=false", want.Name)
			continue
		}
		if got.X() != want.X || got.Y() != want.Y || got.W() != want.W || got.H() != want.H {
			t.Errorf("%s: rect = %d,%d %dx%d, want %d,%d %dx%d",
				want.Name, got.X(), got.Y(), got.W(), got.H(), want.X, want.Y, want.W, want.H)
		}
		if got.PivotX() != want.PivotX || got.PivotY() != want.PivotY {
			t.Errorf("%s: pivot = %v,%v, want %v,%v", want.Name, got.PivotX(), got.PivotY(), want.PivotX, want.PivotY)
		}
		if got.Nine() != want.Nine {
			t.Errorf("%s: nine = %v, want %v", want.Name, got.Nine(), want.Nine)
		}
		if got.Name() != core.AssetID(want.Name) {
			t.Errorf("%s: name = %q", want.Name, got.Name())
		}
	}
	// Rects stay inside the sheet with a pad border, never overlapping.
	for i := 0; i < a.Count(); i++ {
		ei, _ := a.Entry(i)
		if ei.X() < AtlasPad || ei.Y() < AtlasPad ||
			ei.X()+ei.W()+AtlasPad > a.Width() || ei.Y()+ei.H()+AtlasPad > a.Height() {
			t.Errorf("%s: rect %d,%d %dx%d breaks pad border in %dx%d",
				ei.Name(), ei.X(), ei.Y(), ei.W(), ei.H(), a.Width(), a.Height())
		}
		for j := i + 1; j < a.Count(); j++ {
			ej, _ := a.Entry(j)
			if ei.X() < ej.X()+ej.W() && ej.X() < ei.X()+ei.W() &&
				ei.Y() < ej.Y()+ej.H() && ej.Y() < ei.Y()+ei.H() {
				t.Errorf("%s overlaps %s", ei.Name(), ej.Name())
			}
		}
	}
	// Encode round-trips through Parse to the same placements.
	raw, err := a.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(Encode): %v", err)
	}
	if back.Width() != a.Width() || back.Height() != a.Height() || back.Count() != a.Count() {
		t.Fatalf("round-trip size/count = %dx%d/%d, want %dx%d/%d",
			back.Width(), back.Height(), back.Count(), a.Width(), a.Height(), a.Count())
	}
	for _, want := range c.Sprites {
		g, _ := a.Find(core.AssetID(want.Name))
		b, ok := back.Find(core.AssetID(want.Name))
		if !ok {
			t.Fatalf("%s: round-trip missing", want.Name)
		}
		if b.X() != g.X() || b.Y() != g.Y() || b.W() != g.W() || b.H() != g.H() ||
			b.PivotX() != g.PivotX() || b.PivotY() != g.PivotY() || b.Nine() != g.Nine() {
			t.Errorf("%s: round-trip drifted", want.Name)
		}
	}
	// Save to TempDir then Load back: same placements, no aliasing.
	dir := t.TempDir()
	path := filepath.Join(dir, "atlas.json")
	if err := a.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, want := range c.Sprites {
		g, _ := a.Find(core.AssetID(want.Name))
		r, ok := reloaded.Find(core.AssetID(want.Name))
		if !ok {
			t.Fatalf("%s: file round-trip missing", want.Name)
		}
		if r != g {
			t.Errorf("%s: file round-trip drifted", want.Name)
		}
	}
}

// B:空零超大坏数据全不崩不卡死,超大图报错,未冻先报占位.
func TestAtlasPackEdgesNoCrash(t *testing.T) {
	c := loadAtlasCases(t)
	base := buildAtlasInputs(c)
	if _, err := Pack(nil, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil inputs code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Pack([]Input{}, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty inputs code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Pack(base, 0, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero maxW code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Pack(base, c.MaxW, MaxAtlasSize+1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("huge maxH code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Duplicate names never pack.
	dup := append(append([]Input(nil), base...), base[0])
	if _, err := Pack(dup, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("duplicate code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Empty and overlong names fail closed.
	empty := append([]Input(nil), base...)
	empty[0].ID = ""
	if _, err := Pack(empty, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty name code = %v, want invalid-arg", core.CodeOf(err))
	}
	long := append([]Input(nil), base...)
	long[0].ID = core.AssetID(strings.Repeat("x", MaxAtlasNameLen+1))
	if _, err := Pack(long, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long name code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Zero size, pixel mismatch, bad pivot, bad nine fail closed.
	zero := append([]Input(nil), base...)
	zero[0].W = 0
	zero[0].Pixels = fillSolid(1, zero[0].H, [4]int{1, 2, 3, 255})
	if _, err := Pack(zero, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero w code = %v, want invalid-arg", core.CodeOf(err))
	}
	mismatch := append([]Input(nil), base...)
	mismatch[0].Pixels = []byte{1, 2, 3}
	if _, err := Pack(mismatch, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("pixel mismatch code = %v, want invalid-arg", core.CodeOf(err))
	}
	nanPivot := append([]Input(nil), base...)
	nanPivot[0].PivotX = math.NaN()
	if _, err := Pack(nanPivot, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nan pivot code = %v, want invalid-arg", core.CodeOf(err))
	}
	outPivot := append([]Input(nil), base...)
	outPivot[0].PivotX = 2
	if _, err := Pack(outPivot, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("pivot range code = %v, want invalid-arg", core.CodeOf(err))
	}
	badNine := append([]Input(nil), base...)
	badNine[0].Nine = [4]int{9, 9, 9, 9}
	if _, err := Pack(badNine, c.MaxW, c.MaxH); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad nine code = %v, want invalid-arg", core.CodeOf(err))
	}
	// One picture larger than the sheet reports OutOfMemory, never a guess.
	huge := []Input{{
		ID: "huge/pic", W: c.MaxW + 1, H: 8,
		Pixels: fillSolid(c.MaxW+1, 8, [4]int{9, 9, 9, 255}),
		PivotX: 0.5, PivotY: 0.5,
	}}
	expectAtlasCode(t, "oversize sprite", mustPackErr(huge, c.MaxW, c.MaxH), core.CodeOutOfMemory)
	// Shelf overflow reports OutOfMemory too: two 16x16 cannot share 32x32
	// once the pad row breaks (see A placements).
	overflow := []Input{
		{ID: "big/a", W: 16, H: 16, Pixels: fillSolid(16, 16, [4]int{1, 0, 0, 255}), PivotX: 0.5, PivotY: 0.5},
		{ID: "big/b", W: 16, H: 16, Pixels: fillSolid(16, 16, [4]int{0, 1, 0, 255}), PivotX: 0.5, PivotY: 0.5},
		{ID: "big/c", W: 16, H: 16, Pixels: fillSolid(16, 16, [4]int{0, 0, 1, 255}), PivotX: 0.5, PivotY: 0.5},
	}
	expectAtlasCode(t, "shelf overflow", mustPackErr(overflow, c.MaxW, c.MaxH), core.CodeOutOfMemory)
	// Frozen bad files fail with their frozen codes and stay queryable.
	for _, b := range c.BadFiles {
		raw, err := os.ReadFile(filepath.Join("testdata", b.File))
		if err != nil {
			t.Fatalf("read %s: %v", b.File, err)
		}
		_, perr := Parse(raw)
		expectAtlasCode(t, b.File, perr, atlasCodeFromName(b.WantCode))
	}
	for _, u := range c.UnsupportedFiles {
		raw, err := os.ReadFile(filepath.Join("testdata", u.File))
		if err != nil {
			t.Fatalf("read %s: %v", u.File, err)
		}
		_, perr := Parse(raw)
		expectAtlasCode(t, u.File, perr, atlasCodeFromName(u.WantCode))
	}
	// Future versions never load: VersionMismatch, never a guessed sheet.
	rawSmall, err := os.ReadFile(filepath.Join("testdata", c.Atlas.File))
	if err != nil {
		t.Fatalf("read %s: %v", c.Atlas.File, err)
	}
	future := bytes.Replace(rawSmall, []byte(`"1.0"`), []byte(`"2.0"`), 1)
	_, ferr := Parse(future)
	expectAtlasCode(t, "future version", ferr, core.CodeVersionMismatch)
	if _, err := Parse(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil parse code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Load(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty load code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Load(filepath.Join("testdata", "no_such_atlas.json")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing load code = %v, want not-found", core.CodeOf(err))
	}
	// Nil atlas never panics.
	var nilA *Atlas
	if _, err := nilA.Encode(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Encode code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilA.Save(filepath.Join(t.TempDir(), "x.json")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Save code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, ok := nilA.Find("x"); ok {
		t.Error("nil Find ok=true, want false")
	}
	if _, ok := nilA.Entry(0); ok {
		t.Error("nil Entry ok=true, want false")
	}
	if _, _, _, _, ok := nilA.At(0, 0); ok {
		t.Error("nil At ok=true, want false")
	}
	if !nilA.Equal(nil) {
		t.Error("nil Equal(nil) = false, want true")
	}
	good := mustPackCases(t, c)
	if nilA.Equal(good) || !good.Equal(good) {
		t.Error("nil/good Equal wrong")
	}
	packed := mustPackCases(t, c)
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {packed.Width(), 0}, {0, packed.Height()}, {999, 999}} {
		if _, _, _, _, ok := packed.At(p[0], p[1]); ok {
			t.Errorf("At%v ok=true, want false", p)
		}
	}
}

func mustPackErr(ins []Input, maxW, maxH int) error {
	_, err := Pack(ins, maxW, maxH)
	return err
}

// C:画出来差在容差内,两边同数,留白透明.
func TestAtlasPackPixelsWithinTolerance(t *testing.T) {
	c := loadAtlasCases(t)
	if c.Tolerance != atlasDiffTolerance {
		t.Fatalf("tolerance = %d, want frozen %d", c.Tolerance, atlasDiffTolerance)
	}
	a := mustPackCases(t, c)
	byName := map[string]spriteCase{}
	for _, s := range c.Sprites {
		byName[s.Name] = s
	}
	maxDiff := 0
	bad := 0
	total := 0
	for i := 0; i < a.Count(); i++ {
		e, _ := a.Entry(i)
		want := byName[string(e.Name())]
		for dy := 0; dy < e.H(); dy++ {
			for dx := 0; dx < e.W(); dx++ {
				ax, ay := e.X()+dx, e.Y()+dy
				r, g, b, al, ok := a.At(ax, ay)
				if !ok {
					t.Fatalf("%s (%d,%d): At ok=false", e.Name(), ax, ay)
				}
				d := 0
				for k, v := range []uint8{r, g, b, al} {
					if dd := diffByte(v, byte(want.Color[k])); dd > d {
						d = dd
					}
				}
				if d > maxDiff {
					maxDiff = d
				}
				if d > c.Tolerance {
					bad++
				}
				total++
				// Pixels slice agrees with At on every composed pixel.
				off := (ay*a.Width() + ax) * 4
				px := a.Pixels()
				if px[off] != r || px[off+1] != g || px[off+2] != b || px[off+3] != al {
					t.Fatalf("%s (%d,%d): At/slice diverge", e.Name(), ax, ay)
				}
			}
		}
	}
	t.Logf("atlas-tolerance: maxDiff=%d bad=%d/%d tolerance=%d sheet=%dx%d",
		maxDiff, bad, total, c.Tolerance, a.Width(), a.Height())
	if maxDiff > atlasDiffTolerance || bad > 0 {
		t.Fatalf("sheet drift: maxDiff=%d bad=%d, want <= %d and 0", maxDiff, bad, atlasDiffTolerance)
	}
	// Gutter stays transparent: the pad column between the two top sprites
	// plus the outer border prove the留白.
	for _, p := range [][2]int{{0, 0}, {17, 1}, {17, 16}, {0, 26}} {
		r, g, b, al, ok := a.At(p[0], p[1])
		if !ok {
			t.Fatalf("gutter (%d,%d): At ok=false", p[0], p[1])
		}
		if r != 0 || g != 0 || b != 0 || al != 0 {
			t.Errorf("gutter (%d,%d) = %d,%d,%d,%d, want transparent", p[0], p[1], r, g, b, al)
		}
	}
}

// D:打包跑得动,数量耗时有数.
func TestAtlasPackPerfPack(t *testing.T) {
	// Synthetic load only (no golden): golden stays in atlas_cases.json.
	const n = 64
	base := make([]Input, n)
	for i := 0; i < n; i++ {
		name := core.AssetID("perf/sprite-" + itoaAtlas(i))
		base[i] = Input{
			ID: name, W: 16, H: 16,
			Pixels: fillSolid(16, 16, [4]int{i % 256, (i * 3) % 256, (i * 7) % 256, 255}),
			PivotX: 0.5, PivotY: 0.5,
		}
	}
	const maxSide = 256
	const reps = 50
	start := time.Now()
	var first *Atlas
	for i := 0; i < reps; i++ {
		a, err := Pack(base, maxSide, maxSide)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if i == 0 {
			first = a
		}
	}
	el := time.Since(start)
	t.Logf("atlas-pack: %d reps %d sprites 16x16 into %dx%d in %v (%.1f us/pack)",
		reps, n, first.Width(), first.Height(), el, float64(el.Microseconds())/reps)
	if first.Count() != n {
		t.Fatalf("count = %d, want %d", first.Count(), n)
	}
	// Encode joins the cost once.
	encStart := time.Now()
	raw, err := first.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	t.Logf("atlas-encode: %d sprites %dB in %v", n, len(raw), time.Since(encStart))
	if len(raw) == 0 || len(raw) > MaxAtlasJSONBytes {
		t.Fatalf("encode %dB out of 1..%d", len(raw), MaxAtlasJSONBytes)
	}
}

func itoaAtlas(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "000"
	}
	var out [16]byte
	p := len(out)
	n := i
	for n > 0 {
		p--
		out[p] = digits[n%10]
		n /= 10
	}
	for p > len(out)-3 {
		p--
		out[p] = '0'
	}
	return string(out[p:])
}

// E:重复打包稳定,字节逐位一致,坏输入不粘.
func TestAtlasPackLongRunStable(t *testing.T) {
	c := loadAtlasCases(t)
	ins := buildAtlasInputs(c)
	first, err := Pack(ins, c.MaxW, c.MaxH)
	if err != nil {
		t.Fatalf("first Pack: %v", err)
	}
	firstRaw, err := first.Encode()
	if err != nil {
		t.Fatalf("first Encode: %v", err)
	}
	for i := 0; i < 500; i++ {
		got, err := Pack(buildAtlasInputs(c), c.MaxW, c.MaxH)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if !got.Equal(first) {
			t.Fatalf("rep %d diverged", i)
		}
		raw, err := got.Encode()
		if err != nil {
			t.Fatalf("rep %d Encode: %v", i, err)
		}
		if !bytes.Equal(raw, firstRaw) {
			t.Fatalf("rep %d bytes drifted", i)
		}
	}
	// Copies never alias: mutating a return cannot corrupt the replay.
	probe := first.Pixels()
	if len(probe) == 0 {
		t.Fatal("sheet pixels empty, golden invalid")
	}
	probe[0] ^= 0xFF
	if first.Pixels()[0] == probe[0] {
		t.Fatal("Pixels aliases the atlas")
	}
	ins[0].Pixels[0] ^= 0xFF
	again, err := Pack(buildAtlasInputs(c), c.MaxW, c.MaxH)
	if err != nil {
		t.Fatalf("after input probe: %v", err)
	}
	if !again.Equal(first) {
		t.Fatal("input alias corrupted the atlas")
	}
	if !first.Equal(again) {
		t.Fatal("probe corrupted the atlas")
	}
	// Bad inputs never poison the next good pack.
	huge := []Input{{
		ID: "stable/huge", W: c.MaxW + 1, H: 8,
		Pixels: fillSolid(c.MaxW+1, 8, [4]int{1, 1, 1, 255}),
		PivotX: 0.5, PivotY: 0.5,
	}}
	if _, err := Pack(huge, c.MaxW, c.MaxH); err == nil {
		t.Error("huge want error")
	}
	if back, err := Pack(buildAtlasInputs(c), c.MaxW, c.MaxH); err != nil {
		t.Fatalf("good after bad: %v", err)
	} else if !back.Equal(first) {
		t.Fatal("good after bad diverged")
	}
	// File round-trips stay byte-stable through TempDir.
	dir := t.TempDir()
	path := filepath.Join(dir, "stable.json")
	if err := first.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	size := fi.Size()
	for i := 0; i < 100; i++ {
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("load rep %d: %v", i, err)
		}
		raw, err := loaded.Encode()
		if err != nil {
			t.Fatalf("encode rep %d: %v", i, err)
		}
		parsed, err := Parse(raw)
		if err != nil {
			t.Fatalf("parse rep %d: %v", i, err)
		}
		if parsed.Width() != first.Width() || parsed.Height() != first.Height() || parsed.Count() != first.Count() {
			t.Fatalf("file rep %d diverged", i)
		}
		if nfi, _ := os.Stat(path); nfi.Size() != size {
			t.Fatalf("file rep %d size %d, want %d", i, nfi.Size(), size)
		}
	}
}

// F:离屏金对照窗(窗免,纯打包):冻结数加形状断言,账即证据.
func TestAtlasPackOffscreenGolden(t *testing.T) {
	c := loadAtlasCases(t)
	rawSmall, err := os.ReadFile(filepath.Join("testdata", c.Atlas.File))
	if err != nil {
		t.Fatalf("read %s: %v", c.Atlas.File, err)
	}
	gold, err := Parse(rawSmall)
	if err != nil {
		t.Fatalf("Parse %s: %v", c.Atlas.File, err)
	}
	// Golden pins the anchors through files, not code.
	if gold.Version().String() != c.Version {
		t.Errorf("golden version = %v, want %v", gold.Version(), c.Version)
	}
	if gold.Width() != c.Atlas.Width || gold.Height() != c.Atlas.Height {
		t.Errorf("golden size = %dx%d, want %dx%d", gold.Width(), gold.Height(), c.Atlas.Width, c.Atlas.Height)
	}
	for _, want := range c.Sprites {
		got, ok := gold.Find(core.AssetID(want.Name))
		if !ok {
			t.Errorf("%s: golden missing", want.Name)
			continue
		}
		if got.X() != want.X || got.Y() != want.Y || got.W() != want.W || got.H() != want.H {
			t.Errorf("%s: golden rect = %d,%d %dx%d, want %d,%d %dx%d",
				want.Name, got.X(), got.Y(), got.W(), got.H(), want.X, want.Y, want.W, want.H)
		}
		if got.PivotX() != want.PivotX || got.PivotY() != want.PivotY {
			t.Errorf("%s: golden pivot = %v,%v", want.Name, got.PivotX(), got.PivotY())
		}
		if got.Nine() != want.Nine {
			t.Errorf("%s: golden nine = %v, want %v", want.Name, got.Nine(), want.Nine)
		}
	}
	// Pack lands on the same golden without re-spelling it.
	packed := mustPackCases(t, c)
	for _, want := range c.Sprites {
		g, _ := packed.Find(core.AssetID(want.Name))
		b, ok := gold.Find(core.AssetID(want.Name))
		if !ok {
			continue
		}
		if g.X() != b.X() || g.Y() != b.Y() || g.W() != b.W() || g.H() != b.H() ||
			g.PivotX() != b.PivotX() || g.PivotY() != b.PivotY() || g.Nine() != b.Nine() {
			t.Errorf("%s: pack diverges from golden", want.Name)
		}
	}
	// Shape: names distinct, pivots in range, nine inside, rects inside.
	seen := map[string]bool{}
	var area int
	for _, want := range c.Sprites {
		if seen[want.Name] {
			t.Errorf("name %q collides", want.Name)
		}
		seen[want.Name] = true
		if want.PivotX < 0 || want.PivotX > 1 || want.PivotY < 0 || want.PivotY > 1 {
			t.Errorf("%s: pivot %v,%v out of 0..1", want.Name, want.PivotX, want.PivotY)
		}
		if want.Nine[0]+want.Nine[2] > want.W || want.Nine[1]+want.Nine[3] > want.H {
			t.Errorf("%s: nine %v overflows %dx%d", want.Name, want.Nine, want.W, want.H)
		}
		if want.X < AtlasPad || want.Y < AtlasPad ||
			want.X+want.W+AtlasPad > c.Atlas.Width || want.Y+want.H+AtlasPad > c.Atlas.Height {
			t.Errorf("%s: rect breaks pad border", want.Name)
		}
		if want.W < 1 || want.W > MaxAtlasSize || want.H < 1 || want.H > MaxAtlasSize {
			t.Errorf("%s: size %dx%d out of budget", want.Name, want.W, want.H)
		}
		area += want.W * want.H
	}
	sheet := c.Atlas.Width * c.Atlas.Height
	t.Logf("atlas-golden: %d sprites area=%d sheet=%dx%d=%d util=%.1f%% pad=%d",
		len(c.Sprites), area, c.Atlas.Width, c.Atlas.Height, sheet, 100*float64(area)/float64(sheet), AtlasPad)
	if sheet > MaxAtlasSize*MaxAtlasSize {
		t.Errorf("sheet %d exceeds budget", sheet)
	}
	// Shape: every stored number stays inside its frozen budget.
	if len(c.Sprites) > MaxSprites {
		t.Errorf("sprites %d exceeds MaxSprites", len(c.Sprites))
	}
	// Shape: bad and unfrozen stay out of the Ready set.
	for _, b := range c.BadFiles {
		raw, _ := os.ReadFile(filepath.Join("testdata", b.File))
		if _, err := Parse(raw); err == nil {
			t.Errorf("%s want error", b.File)
		} else if atlasCodeFromName(b.WantCode) != core.CodeOf(err) {
			t.Errorf("%s code = %v, want %v", b.File, core.CodeOf(err), b.WantCode)
		}
	}
	for _, u := range c.UnsupportedFiles {
		raw, _ := os.ReadFile(filepath.Join("testdata", u.File))
		if _, err := Parse(raw); core.CodeOf(err) != core.CodeUnsupported {
			t.Errorf("%s code = %v, want unsupported", u.File, core.CodeOf(err))
		}
	}
}
