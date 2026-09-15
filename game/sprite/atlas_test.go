package sprite

import (
	"encoding/json"
	"errors"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// S36 only keeps the offscreen proof. Window game_sprite--case=rot lands
// with P2; this file reads game/sprite/testdata/atlas_cases.json and never
// hardcodes standard pixels.

type atlasQuadDef struct {
	Name string `json:"name"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	RGBA [4]int `json:"rgba"`
}

type atlasSpriteDef struct {
	Image   string    `json:"image"`
	Src     [4]float64 `json:"src"`
	Dst     [4]float64 `json:"dst"`
	Opacity float64   `json:"opacity"`
	Rot     float64   `json:"rot"`
	FlipX   bool      `json:"flipx"`
	FlipY   bool      `json:"flipy"`
	Pivot   [2]float64 `json:"pivot"`
	Tint    [4]float64 `json:"tint"`
	Filter  string    `json:"filter"`
	Tag     string    `json:"tag"`
}

type atlasProbeDef struct {
	X    int      `json:"x"`
	Y    int      `json:"y"`
	Want [4]uint8 `json:"want"`
	Tol  uint8    `json:"tol"`
	Desc string   `json:"desc"`
}

type atlasCaseDef struct {
	Note    string           `json:"note"`
	Sprites []atlasSpriteDef `json:"sprites"`
	Probes  []atlasProbeDef  `json:"probes"`
}

type atlasFile struct {
	Canvas [2]int `json:"canvas"`
	Atlas  struct {
		W     int            `json:"w"`
		H     int            `json:"h"`
		Quads []atlasQuadDef `json:"quads"`
	} `json:"atlas"`
	WindowIntent string                  `json:"window_intent"`
	Cases        map[string]atlasCaseDef `json:"cases"`
}

func loadAtlasCases(t *testing.T) atlasFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "atlas_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/atlas_cases.json: %v", err)
	}
	var f atlasFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode atlas_cases.json: %v", err)
	}
	if len(f.Cases) == 0 || len(f.Atlas.Quads) == 0 {
		t.Fatal("atlas_cases.json missing cases or quads")
	}
	if f.WindowIntent == "" {
		t.Fatal("atlas_cases.json missing window_intent")
	}
	return f
}

func mustFindAtlasCase(t *testing.T, f atlasFile, name string) atlasCaseDef {
	t.Helper()
	c, ok := f.Cases[name]
	if !ok {
		t.Fatalf("atlas_cases.json has no case %q", name)
	}
	if len(c.Sprites) == 0 && name != "hundred" {
		t.Fatalf("case %q has no sprites", name)
	}
	return c
}

func atlasRect(t *testing.T, v [4]float64, what string) core.Rect {
	t.Helper()
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			t.Fatalf("%s has non-finite %v", what, v)
		}
	}
	return core.NewRect(v[0], v[1], v[2], v[3])
}

func buildAtlasSprites(t *testing.T, defs []atlasSpriteDef) []AtlasSprite {
	t.Helper()
	out := make([]AtlasSprite, len(defs))
	for i, d := range defs {
		filt, err := ParseAtlasFilter(d.Filter)
		if err != nil {
			t.Fatalf("sprite %q: ParseAtlasFilter %q: %v", d.Tag, d.Filter, err)
		}
		s, err := NewAtlasSprite(core.AssetID(d.Image),
			atlasRect(t, d.Src, d.Tag+"/src"), atlasRect(t, d.Dst, d.Tag+"/dst"),
			d.Opacity, d.Rot, core.V2(d.Pivot[0], d.Pivot[1]),
			core.RGBA(d.Tint[0], d.Tint[1], d.Tint[2], d.Tint[3]),
			filt, d.FlipX, d.FlipY)
		if err != nil {
			t.Fatalf("sprite %q: NewAtlasSprite: %v", d.Tag, err)
		}
		s.Tag = d.Tag
		out[i] = s
	}
	return out
}

func atlasClose(got, want [4]uint8, tol uint8) bool {
	for i := 0; i < 4; i++ {
		d := int(got[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > int(tol) {
			return false
		}
	}
	return true
}

func buildAtlasImage(t *testing.T, f atlasFile) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(f.Atlas.W, f.Atlas.H, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf atlas: %v", err)
	}
	for _, q := range f.Atlas.Quads {
		for y := q.Y; y < q.Y+q.H; y++ {
			for x := q.X; x < q.X+q.W; x++ {
				if err := img.SetRGBA(x, y, uint8(q.RGBA[0]), uint8(q.RGBA[1]), uint8(q.RGBA[2]), uint8(q.RGBA[3])); err != nil {
					t.Fatalf("SetRGBA atlas %d,%d: %v", x, y, err)
				}
			}
		}
	}
	return img
}

func withAtlasCPU(t *testing.T) {
	t.Helper()
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	})
}

func withAtlasGPU(t *testing.T) {
	t.Helper()
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Unsetenv("GOGPU_RENDER_MODE")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	})
}

func newAtlasWhite(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

func atlasSample(t *testing.T, dc *render.Context, x, y int) [4]uint8 {
	t.Helper()
	img := dc.Image()
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		t.Fatalf("probe %d,%d outside bounds %v", x, y, b)
	}
	r, g, b2, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b2 >> 8), uint8(a >> 8)}
}

func atlasCheckProbes(t *testing.T, dc *render.Context, probes []atlasProbeDef) {
	t.Helper()
	for i, p := range probes {
		got := atlasSample(t, dc, p.X, p.Y)
		if !atlasClose(got, p.Want, p.Tol) {
			t.Errorf("probe[%d] %s (%d,%d) = %v, want %v tol %d", i, p.Desc, p.X, p.Y, got, p.Want, p.Tol)
		}
	}
}

// A: angle, tint, flip, pivot, and per-sprite filter land field for field.
func TestAtlasAnglesFiltersFromCases(t *testing.T) {
	f := loadAtlasCases(t)
	names := []string{"identity", "rot90_center", "rot180_center", "rot180_origin", "flipx_center", "flipy_center", "tint_gray", "filter_nearest_scaled", "filter_bilinear_scaled", "filter_bicubic_identity", "feet_center", "feet_rot360"}
	for _, name := range names {
		def := mustFindAtlasCase(t, f, name)
		got := buildAtlasSprites(t, def.Sprites)
		if len(got) != len(def.Sprites) {
			t.Fatalf("%s: built %d, want %d", name, len(got), len(def.Sprites))
		}
		for i, d := range def.Sprites {
			s := got[i]
			if string(s.Image) != d.Image {
				t.Errorf("%s[%d]: image = %q, want %q", name, i, s.Image, d.Image)
			}
			if s.Src != core.NewRect(d.Src[0], d.Src[1], d.Src[2], d.Src[3]) {
				t.Errorf("%s[%d]: src = %v, want %v", name, i, s.Src, d.Src)
			}
			if s.Dst != core.NewRect(d.Dst[0], d.Dst[1], d.Dst[2], d.Dst[3]) {
				t.Errorf("%s[%d]: dst = %v, want %v", name, i, s.Dst, d.Dst)
			}
			if s.Opacity != d.Opacity || s.Rot != d.Rot {
				t.Errorf("%s[%d]: opacity/rot = %v/%v, want %v/%v", name, i, s.Opacity, s.Rot, d.Opacity, d.Rot)
			}
			if s.FlipX != d.FlipX || s.FlipY != d.FlipY {
				t.Errorf("%s[%d]: flip = %v/%v, want %v/%v", name, i, s.FlipX, s.FlipY, d.FlipX, d.FlipY)
			}
			if s.Pivot != core.V2(d.Pivot[0], d.Pivot[1]) {
				t.Errorf("%s[%d]: pivot = %v, want %v", name, i, s.Pivot, d.Pivot)
			}
			if s.Tint != core.RGBA(d.Tint[0], d.Tint[1], d.Tint[2], d.Tint[3]) {
				t.Errorf("%s[%d]: tint = %v, want %v", name, i, s.Tint, d.Tint)
			}
			wantFilt, err := ParseAtlasFilter(d.Filter)
			if err != nil {
				t.Fatalf("%s[%d]: filter %q: %v", name, i, d.Filter, err)
			}
			if s.Filter != wantFilt {
				t.Errorf("%s[%d]: filter = %v, want %v", name, i, s.Filter, wantFilt)
			}
			if s.Tag != d.Tag {
				t.Errorf("%s[%d]: tag = %q, want %q", name, i, s.Tag, d.Tag)
			}
			if s.Skippable() {
				t.Errorf("%s[%d]: Skippable = true, want false", name, i)
			}
			if err := s.Validate(); err != nil {
				t.Errorf("%s[%d]: Validate: %v", name, i, err)
			}
			// Boundary mapping is exact, no rounding at the game edge.
			rs, err := s.ToRender()
			if err != nil {
				t.Errorf("%s[%d]: ToRender: %v", name, i, err)
				continue
			}
			if rs.SrcX != d.Src[0] || rs.SrcY != d.Src[1] || rs.SrcW != d.Src[2] || rs.SrcH != d.Src[3] {
				t.Errorf("%s[%d]: render src = %v %v %v %v, want %v", name, i, rs.SrcX, rs.SrcY, rs.SrcW, rs.SrcH, d.Src)
			}
			if rs.DstX != d.Dst[0] || rs.DstY != d.Dst[1] || rs.DstW != d.Dst[2] || rs.DstH != d.Dst[3] {
				t.Errorf("%s[%d]: render dst = %v %v %v %v, want %v", name, i, rs.DstX, rs.DstY, rs.DstW, rs.DstH, d.Dst)
			}
			if rs.Opacity != d.Opacity || rs.Rot != d.Rot || rs.FlipX != d.FlipX || rs.FlipY != d.FlipY {
				t.Errorf("%s[%d]: render rot/flip/opacity diverged", name, i)
			}
			if rs.PivotX != d.Pivot[0] || rs.PivotY != d.Pivot[1] {
				t.Errorf("%s[%d]: render pivot = %v,%v, want %v", name, i, rs.PivotX, rs.PivotY, d.Pivot)
			}
			if rs.Tint != core.RGBA(d.Tint[0], d.Tint[1], d.Tint[2], d.Tint[3]).ToRender() {
				t.Errorf("%s[%d]: render tint = %v, want %v", name, i, rs.Tint, d.Tint)
			}
			wantMode, err := wantFilt.ToRender()
			if err != nil {
				t.Fatalf("%s[%d]: filter ToRender: %v", name, i, err)
			}
			if rs.Filter != wantMode {
				t.Errorf("%s[%d]: render filter = %v, want %v", name, i, rs.Filter, wantMode)
			}
			// Tint presence and pivot meaning, not just bits.
			wantTint := d.Tint != [4]float64{0, 0, 0, 0}
			if s.HasTint() != wantTint {
				t.Errorf("%s[%d]: HasTint = %v, want %v", name, i, s.HasTint(), wantTint)
			}
			if pp := s.PivotPoint(); pp != core.V2(d.Dst[0]+d.Pivot[0], d.Dst[1]+d.Pivot[1]) {
				t.Errorf("%s[%d]: PivotPoint = %v, want dst+pivot", name, i, pp)
			}
		}
	}
	// Feet pivot is bottom-center by construction, so a turn keeps the feet.
	feetDef := mustFindAtlasCase(t, f, "feet_center")
	feet := buildAtlasSprites(t, feetDef.Sprites)[0]
	if want := FeetPivot(feet.Dst); feet.Pivot != want {
		t.Errorf("feet_center pivot = %v, want FeetPivot %v", feet.Pivot, want)
	}
	// Filter names round-trip; unknown stays loud.
	for _, tc := range []struct {
		in   string
		want AtlasFilter
	}{{ "", AtlasFilterDefault}, {"default", AtlasFilterDefault}, {"nearest", AtlasFilterNearest}, {"bilinear", AtlasFilterBilinear}, {"bicubic", AtlasFilterBicubic}} {
		gotF, err := ParseAtlasFilter(tc.in)
		if err != nil || gotF != tc.want {
			t.Errorf("ParseAtlasFilter(%q) = %v,%v, want %v,nil", tc.in, gotF, err, tc.want)
		}
		if gotF.String() != tc.in && !(tc.in == "" && gotF.String() == "default") {
			t.Errorf("Filter %v String = %q, want %q", gotF, gotF.String(), tc.in)
		}
	}
}

// B: empty, zero-size, and corrupt inputs never crash; bad paths report codes.
func TestAtlasEdgesNoCrash(t *testing.T) {
	// Empty conversion is a quiet no-op.
	if out, err := AtlasToRender(nil); err != nil || out != nil {
		t.Errorf("nil AtlasToRender = %v,%v, want nil,nil", out, err)
	}
	if out, err := AtlasToRender([]AtlasSprite{}); err != nil || len(out) != 0 {
		t.Errorf("empty AtlasToRender = %v,%v, want empty,nil", out, err)
	}
	// Zero-size sprites are valid but skippable, mirroring the R4 skip.
	mk := func(src, dst core.Rect) AtlasSprite {
		s, err := NewAtlasSprite("tex/atlas", src, dst, 1, 0, core.V2(0, 0), core.Color{}, AtlasFilterDefault, false, false)
		if err != nil {
			t.Fatalf("NewAtlasSprite zero probe: %v", err)
		}
		return s
	}
	zeroSrc := mk(core.NewRect(0, 0, 0, 8), core.NewRect(0, 0, 8, 8))
	if !zeroSrc.Skippable() {
		t.Error("zero-src Skippable = false, want true")
	}
	zeroDst := mk(core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 0, 8))
	if !zeroDst.Skippable() {
		t.Error("zero-dst Skippable = false, want true")
	}
	if _, err := zeroSrc.ToRender(); err != nil {
		t.Errorf("skippable ToRender err = %v, want nil (skip happens at draw)", err)
	}
	// Negative destination is kept (mirrored draw at the render edge).
	mir := mk(core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, -8, 8))
	if mir.Skippable() {
		t.Error("mirrored Dst Skippable = true, want false")
	}
	// Bad constructions store nothing and report invalid-arg.
	badFilter := AtlasFilter(99)
	bads := []func() error{
		func() error {
			_, err := NewAtlasSprite("", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), 1, 0, core.V2(0, 0), core.Color{}, AtlasFilterDefault, false, false)
			return err
		},
		func() error {
			_, err := NewAtlasSprite("tex/a", core.NewRect(math.NaN(), 0, 8, 8), core.NewRect(0, 0, 8, 8), 1, 0, core.V2(0, 0), core.Color{}, AtlasFilterDefault, false, false)
			return err
		},
		func() error {
			_, err := NewAtlasSprite("tex/a", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, math.Inf(1)), 1, 0, core.V2(0, 0), core.Color{}, AtlasFilterDefault, false, false)
			return err
		},
		func() error {
			_, err := NewAtlasSprite("tex/a", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), math.NaN(), 0, core.V2(0, 0), core.Color{}, AtlasFilterDefault, false, false)
			return err
		},
		func() error {
			_, err := NewAtlasSprite("tex/a", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), 1, math.Inf(1), core.V2(0, 0), core.Color{}, AtlasFilterDefault, false, false)
			return err
		},
		func() error {
			_, err := NewAtlasSprite("tex/a", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), 1, 0, core.V2(math.NaN(), 0), core.Color{}, AtlasFilterDefault, false, false)
			return err
		},
		func() error {
			_, err := NewAtlasSprite("tex/a", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), 1, 0, core.V2(0, 0), core.RGBA(math.NaN(), 0, 0, 1), AtlasFilterDefault, false, false)
			return err
		},
		func() error {
			_, err := NewAtlasSprite("tex/a", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), 1, 0, core.V2(0, 0), core.Color{}, badFilter, false, false)
			return err
		},
	}
	for i, fn := range bads {
		if err := fn(); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad NewAtlasSprite[%d] code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	if _, err := ParseAtlasFilter("sharp"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("ParseAtlasFilter sharp err = %v, want invalid-arg", err)
	}
	if _, err := badFilter.ToRender(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad filter ToRender err = %v, want invalid-arg", err)
	}
	// Struct-literal bad paths fail closed at Validate/ToRender.
	literal := AtlasSprite{Image: "tex/a", Src: core.NewRect(0, 0, 8, 8), Dst: core.NewRect(0, 0, 8, 8), Opacity: 1, Filter: badFilter}
	if err := literal.Validate(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("literal Validate err = %v, want invalid-arg", err)
	}
	if _, err := literal.ToRender(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("literal ToRender err = %v, want invalid-arg", err)
	}
	var zero AtlasSprite
	if err := zero.Validate(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero Validate err = %v, want invalid-arg", err)
	}
	// First bad sprite aborts the batch with no partial draw.
	okDef := mustFindAtlasCase(t, loadAtlasCases(t), "identity")
	ok := buildAtlasSprites(t, okDef.Sprites)
	mixed := append(append([]AtlasSprite{}, ok...), literal)
	if out, err := AtlasToRender(mixed); err == nil || out != nil {
		t.Errorf("mixed AtlasToRender = %v,%v, want nil,error", out, err)
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("mixed AtlasToRender code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Opacity mapping reuses the frozen edge: <=0 defaults, >1 clamps.
	for _, tc := range []struct {
		in, want float64
	}{{0, 1}, {-2, 1}, {0.5, 0.5}, {1, 1}, {2, 1}} {
		if got := EffectiveOpacity(tc.in); got != tc.want {
			t.Errorf("EffectiveOpacity(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// C: conversion is lossless and replays identical; GPU vs CPU only sampling rounding.
func TestAtlasBoundaryIdentical(t *testing.T) {
	f := loadAtlasCases(t)
	for _, name := range []string{"identity", "rot90_center", "rot180_center", "flipx_center", "flipy_center", "tint_gray", "filter_bilinear_scaled", "feet_center"} {
		def := mustFindAtlasCase(t, f, name)
		build := func() []AtlasSprite { return buildAtlasSprites(t, def.Sprites) }
		a, b := build(), build()
		ra, err := AtlasToRender(a)
		if err != nil {
			t.Fatalf("%s: AtlasToRender: %v", name, err)
		}
		rb, err := AtlasToRender(b)
		if err != nil {
			t.Fatalf("%s: second AtlasToRender: %v", name, err)
		}
		if len(ra) != len(rb) || len(ra) != len(def.Sprites) {
			t.Fatalf("%s: render counts %d vs %d, want %d", name, len(ra), len(rb), len(def.Sprites))
		}
		for i := range ra {
			if ra[i] != rb[i] {
				t.Fatalf("%s: replay diverged at %d: %+v vs %+v", name, i, ra[i], rb[i])
			}
		}
		// Input untouched: conversion never mutates the caller's slice.
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("%s: input mutated at %d", name, i)
			}
		}
		// Result is fresh: writing it cannot alias a later conversion.
		if len(ra) > 0 {
			ra[0].Opacity = -999
			again, err := AtlasToRender(build())
			if err != nil {
				t.Fatalf("%s: third AtlasToRender: %v", name, err)
			}
			if again[0].Opacity == -999 {
				t.Fatalf("%s: emitted slice aliases later conversion", name)
			}
		}
	}
	// GPU vs CPU parity needs a native GPU; otherwise skip with reason, never fake green.
	if render.Accelerator() == nil {
		t.Skip("no GPU accelerator, parity needs native GPU (offscreen CPU proof stays green)")
	}
	probe := newAtlasWhite(8, 8)
	probe.ClearWithColor(render.White)
	probe.SetRGB(1, 0, 0)
	probe.DrawRectangle(0, 0, 8, 8)
	_ = probe.Fill()
	if err := probe.FlushGPU(); err != nil {
		probe.Close()
		t.Skipf("GPU flush unavailable: %v", err)
	}
	isGPU := probe.RenderPathStats().GPUOps > 0
	probe.Close()
	if !isGPU {
		t.Skip("no GPU ops on probe, parity needs native GPU")
	}
	img := buildAtlasImage(t, f)
	w, h := f.Canvas[0], f.Canvas[1]
	for _, name := range []string{"identity", "rot90_center"} {
		def := mustFindAtlasCase(t, f, name)
		sprites, err := AtlasToRender(buildAtlasSprites(t, def.Sprites))
		if err != nil {
			t.Fatalf("%s: AtlasToRender: %v", name, err)
		}
		withAtlasCPU(t)
		dcCPU := newAtlasWhite(w, h)
		if _, err := dcCPU.DrawAtlasEx(img, sprites, render.AtlasDrawOptions{}); err != nil {
			dcCPU.Close()
			t.Fatalf("%s CPU Ex: %v", name, err)
		}
		cpuImg := dcCPU.Image()
		dcCPU.Close()
		withAtlasGPU(t)
		dcGPU := newAtlasWhite(w, h)
		if _, err := dcGPU.DrawAtlasEx(img, sprites, render.AtlasDrawOptions{}); err != nil {
			dcGPU.Close()
			t.Fatalf("%s GPU Ex: %v", name, err)
		}
		if err := dcGPU.FlushGPU(); err != nil {
			dcGPU.Close()
			if errors.Is(err, render.ErrFallbackToCPU) {
				t.Skipf("flush fell back to CPU (no native GPU in this env): %v", err)
			}
			t.Fatalf("%s flush: %v", name, err)
		}
		if dcGPU.RenderPathStats().GPUOps == 0 {
			dcGPU.Close()
			t.Skipf("no GPUOps for %s (no native GPU in this env)", name)
		}
		gpuImg := dcGPU.Image()
		dcGPU.Close()
		changed, total, mean, max := diffImages(cpuImg, gpuImg)
		changedPct := 100 * float64(changed) / float64(total)
		t.Logf("%s parity changed=%d/%d (%.3f%%) mean=%.3f max=%d DISPLAY=%s canvas=%dx%d", name, changed, total, changedPct, mean, max, os.Getenv("DISPLAY"), w, h)
		if changedPct > 1.0 {
			t.Fatalf("%s parity changed %.3f%% > 1.0%%", name, changedPct)
		}
		if mean > 1.0 {
			t.Fatalf("%s parity mean %.3f > 1.0", name, mean)
		}
	}
}

func diffImages(a, b image.Image) (changed, total int, mean float64, max int) {
	ab := a.Bounds()
	bb := b.Bounds()
	if !ab.Eq(bb) {
		return -1, 0, 0, 0
	}
	total = ab.Dx() * ab.Dy()
	var sumAbs float64
	var maxDelta int
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ar, ag, ab2, _ := a.At(x, y).RGBA()
			br, bg, bb2, _ := b.At(x, y).RGBA()
			ds := [3]int{
				absInt(int(ar>>8) - int(br>>8)),
				absInt(int(ag>>8) - int(bg>>8)),
				absInt(int(ab2>>8) - int(bb2>>8)),
			}
			pixelChanged := false
			for _, d := range ds {
				if d > 2 {
					pixelChanged = true
				}
				if d > maxDelta {
					maxDelta = d
				}
				sumAbs += float64(d)
			}
			if pixelChanged {
				changed++
			}
		}
	}
	mean = sumAbs / float64(total*3)
	return changed, total, mean, maxDelta
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// D: hundred sprites convert and draw with a measured cost.
func TestAtlasPerfHundred(t *testing.T) {
	f := loadAtlasCases(t)
	def := mustFindAtlasCase(t, f, "hundred")
	if len(def.Sprites) != 100 {
		t.Fatalf("hundred sprites = %d, want 100", len(def.Sprites))
	}
	items := buildAtlasSprites(t, def.Sprites)
	for i, s := range items {
		if s.Skippable() {
			t.Fatalf("hundred[%d] skippable, want drawable", i)
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("hundred[%d]: Validate: %v", i, err)
		}
	}
	const reps = 2000
	start := time.Now()
	var n int
	for i := 0; i < reps; i++ {
		out, err := AtlasToRender(items)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		n = len(out)
	}
	el := time.Since(start)
	if n != 100 {
		t.Fatalf("converted = %d, want 100", n)
	}
	t.Logf("atlas-100-convert: %d reps x100 sprites in %v (%.1f us/rep)", reps, el, float64(el.Microseconds())/float64(reps))
	// Same-image hundred draws once per Ex call on the CPU road.
	withAtlasCPU(t)
	img := buildAtlasImage(t, f)
	rs, err := AtlasToRender(items)
	if err != nil {
		t.Fatalf("AtlasToRender hundred: %v", err)
	}
	dc := newAtlasWhite(64, 64)
	defer dc.Close()
	const draws = 50
	dstart := time.Now()
	drawn := 0
	for i := 0; i < draws; i++ {
		dc.ClearWithColor(render.White)
		res, err := dc.DrawAtlasEx(img, rs, render.AtlasDrawOptions{})
		if err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
		drawn = res.Drawn
	}
	del := time.Since(dstart)
	t.Logf("atlas-100-draw: %d draws x100 sprites in %v (%.2f ms/draw drawn=%d)", draws, del, float64(del.Milliseconds())/float64(draws), drawn)
	if drawn != 100 {
		t.Errorf("hundred drawn = %d, want 100", drawn)
	}
}

// E: long runs neither drift nor leak between replays.
func TestAtlasLongRunStable(t *testing.T) {
	f := loadAtlasCases(t)
	idDef := mustFindAtlasCase(t, f, "identity")
	rotDef := mustFindAtlasCase(t, f, "rot90_center")
	mk := func() []AtlasSprite { return buildAtlasSprites(t, idDef.Sprites) }
	first, err := AtlasToRender(mk())
	if err != nil {
		t.Fatalf("AtlasToRender: %v", err)
	}
	for i := 0; i < 5000; i++ {
		got, err := AtlasToRender(mk())
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if len(got) != len(first) || got[0] != first[0] {
			t.Fatalf("rep %d diverged", i)
		}
	}
	// Alternating draws end on the rotated frame: every file probe for the
	// rotated case still passes, so no drift accumulated.
	withAtlasCPU(t)
	img := buildAtlasImage(t, f)
	w, h := f.Canvas[0], f.Canvas[1]
	idSp, err := AtlasToRender(buildAtlasSprites(t, idDef.Sprites))
	if err != nil {
		t.Fatalf("id AtlasToRender: %v", err)
	}
	rotSp, err := AtlasToRender(buildAtlasSprites(t, rotDef.Sprites))
	if err != nil {
		t.Fatalf("rot AtlasToRender: %v", err)
	}
	dc := newAtlasWhite(w, h)
	defer dc.Close()
	const n = 500
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		sp := idSp
		if i%2 == 1 {
			sp = rotSp
		}
		if _, err := dc.DrawAtlasEx(img, sp, render.AtlasDrawOptions{}); err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
	}
	atlasCheckProbes(t, dc, rotDef.Probes)
	t.Logf("atlas-longrun: 5000 conversions identical + %d alternating draws end on rotated probes", n)
}

// F: offscreen golden stands in for game_sprite--case=rot (window lands with P2).
func TestAtlasOffscreenGolden(t *testing.T) {
	withAtlasCPU(t)
	f := loadAtlasCases(t)
	t.Logf("window-intent: %s", f.WindowIntent)
	if f.WindowIntent == "" {
		t.Fatal("missing window_intent")
	}
	img := buildAtlasImage(t, f)
	w, h := f.Canvas[0], f.Canvas[1]
	names := make([]string, 0, len(f.Cases))
	for name := range f.Cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if name == "hundred" {
			continue
		}
		def := f.Cases[name]
		sprites, err := AtlasToRender(buildAtlasSprites(t, def.Sprites))
		if err != nil {
			t.Fatalf("%s: AtlasToRender: %v", name, err)
		}
		dc := newAtlasWhite(w, h)
		res, err := dc.DrawAtlasEx(img, sprites, render.AtlasDrawOptions{})
		if err != nil {
			dc.Close()
			t.Fatalf("%s: DrawAtlasEx: %v", name, err)
		}
		if res.Skipped || res.Drawn != len(def.Sprites) {
			t.Errorf("%s Ex = %+v, want Drawn=%d", name, res, len(def.Sprites))
		}
		atlasCheckProbes(t, dc, def.Probes)
		dc.Close()
	}
	// Shape: pivot at the feet keeps a full turn glued (feet never lift).
	feet := buildAtlasSprites(t, mustFindAtlasCase(t, f, "feet_center").Sprites)[0]
	if want := FeetPivot(feet.Dst); feet.Pivot != want {
		t.Errorf("feet pivot = %v, want FeetPivot %v", feet.Pivot, want)
	}
	if pp := feet.PivotPoint(); pp != core.V2(feet.Dst.X+feet.Dst.W/2, feet.Dst.Y+feet.Dst.H) {
		t.Errorf("feet PivotPoint = %v, want bottom-center", pp)
	}
	// Shape: flips are set per axis, tint marks the gray case only.
	flipx := buildAtlasSprites(t, mustFindAtlasCase(t, f, "flipx_center").Sprites)[0]
	if !flipx.FlipX || flipx.FlipY {
		t.Errorf("flipx flags = %v/%v, want true/false", flipx.FlipX, flipx.FlipY)
	}
	flipy := buildAtlasSprites(t, mustFindAtlasCase(t, f, "flipy_center").Sprites)[0]
	if flipy.FlipX || !flipy.FlipY {
		t.Errorf("flipy flags = %v/%v, want false/true", flipy.FlipX, flipy.FlipY)
	}
	gray := buildAtlasSprites(t, mustFindAtlasCase(t, f, "tint_gray").Sprites)[0]
	plain := buildAtlasSprites(t, mustFindAtlasCase(t, f, "identity").Sprites)[0]
	if !gray.HasTint() || plain.HasTint() {
		t.Errorf("HasTint gray/plain = %v/%v, want true/false", gray.HasTint(), plain.HasTint())
	}
	// Shape: hundred stays one Ex call with 100 draws (window intent:
	// game_sprite --case=rot draws the turn plus a hundred-block field;
	// the window itself lands with P2, this golden is the offscreen proof).
	hund := mustFindAtlasCase(t, f, "hundred")
	if len(hund.Sprites) != 100 {
		t.Errorf("hundred sprites = %d, want 100", len(hund.Sprites))
	}
	t.Logf("atlas-offscreen: env DISPLAY=%s canvas=%dx%d GOGPU_RENDER_MODE=cpu debug-build (release window run with P2)", os.Getenv("DISPLAY"), w, h)
}
