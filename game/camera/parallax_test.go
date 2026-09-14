package camera

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsParallax = 1e-9

type parallaxLayerDef struct {
	Name   string     `json:"name"`
	Factor [2]float64 `json:"factor"`
	Offset [2]float64 `json:"offset"`
	Mirror [2]float64 `json:"mirror"`
}

type parallaxShiftCase struct {
	Layer  string     `json:"layer"`
	World  [2]float64 `json:"world"`
	Camera [2]float64 `json:"camera"`
	Want   [2]float64 `json:"want"`
}

type parallaxScreenCase struct {
	Layer  string     `json:"layer"`
	World  [2]float64 `json:"world"`
	Depth  float64    `json:"depth"`
	Camera [2]float64 `json:"camera"`
	Screen [2]float64 `json:"screen"`
	Scale  float64    `json:"scale"`
}

type parallaxQuadCase struct {
	Layer   string        `json:"layer"`
	Corners [4][2]float64 `json:"corners"`
	Depths  [4]float64    `json:"depths"`
	Camera  [2]float64    `json:"camera"`
	Shifts  [4][2]float64 `json:"shifts"`
	Screens [4][2]float64 `json:"screens"`
	Scales  [4]float64    `json:"scales"`
}

type parallaxSeamCase struct {
	Layer  string     `json:"layer"`
	WorldA [2]float64 `json:"world_a"`
	WorldB [2]float64 `json:"world_b"`
	Camera [2]float64 `json:"camera"`
	Depth  float64    `json:"depth"`
}

type parallaxFile struct {
	Projector struct {
		Focal  float64    `json:"focal"`
		Center [2]float64 `json:"center"`
		Camera [2]float64 `json:"camera"`
	} `json:"projector"`
	Layers     []parallaxLayerDef   `json:"layers"`
	Shifts     []parallaxShiftCase  `json:"shifts"`
	Screens    []parallaxScreenCase `json:"screens"`
	Quad       parallaxQuadCase     `json:"quad"`
	Seamless   []parallaxSeamCase   `json:"seamless"`
	FollowPath [][2]float64         `json:"follow_path"`
	BadLayers  []parallaxLayerDef   `json:"bad_layers"`
}

func loadParallaxCases(t *testing.T) (Projector, map[string]Layer, parallaxFile) {
	t.Helper()
	var cases parallaxFile
	raw, err := os.ReadFile(filepath.Join("testdata", "parallax_cases.json"))
	if err != nil {
		t.Fatalf("read parallax_cases.json: %v", err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decode parallax_cases.json: %v", err)
	}
	proj, err := NewProjector(cases.Projector.Focal,
		pt(cases.Projector.Center),
		pt(cases.Projector.Camera))
	if err != nil {
		t.Fatalf("NewProjector: %v", err)
	}
	layers := map[string]Layer{}
	for _, d := range cases.Layers {
		l, err := NewLayer(pt(d.Factor),
			pt(d.Offset),
			pt(d.Mirror))
		if err != nil {
			t.Fatalf("NewLayer %s: %v", d.Name, err)
		}
		layers[d.Name] = l
	}
	return proj, layers, cases
}

func closeParallax(got core.Vec2, want [2]float64) bool {
	return math.Abs(got.X-want[0]) < epsParallax && math.Abs(got.Y-want[1]) < epsParallax
}

// pt converts a frozen [x, y] pair from testdata to game units.
func pt(a [2]float64) core.Vec2 { return core.V2(a[0], a[1]) }

// seamOK reports whether two mirror-twin screens agree. Twins take
// different float paths to the same math point, so the gate is 1e-9
// closeness, not bitwise identity (bitwise replay of identical inputs
// is pinned separately by the replay tests).
func seamOK(a, b core.Vec2) bool { return a.ApproxEqual(b, epsParallax) }

// A: drifted positions land on the frozen numbers, mirrors wrap.
func TestParallaxShiftFromCases(t *testing.T) {
	_, layers, cases := loadParallaxCases(t)
	for i, k := range cases.Shifts {
		l, ok := layers[k.Layer]
		if !ok {
			t.Fatalf("shift[%d] unknown layer %q", i, k.Layer)
		}
		got, ok := l.Shift(pt(k.World), pt(k.Camera))
		if !ok {
			t.Errorf("shift[%d] %s ok=false, want true", i, k.Layer)
			continue
		}
		if !closeParallax(got, k.Want) {
			t.Errorf("shift[%d] %s = %v, want %v", i, k.Layer, got, k.Want)
		}
	}
}

// A: screens land on the frozen numbers; factor-1 matches a direct
// Project and a screen-pinned layer stays put across frames.
func TestParallaxScreenFromCases(t *testing.T) {
	proj, layers, cases := loadParallaxCases(t)
	for i, k := range cases.Screens {
		l, ok := layers[k.Layer]
		if !ok {
			t.Fatalf("screen[%d] unknown layer %q", i, k.Layer)
		}
		got, scale, ok := l.Screen(pt(k.World), k.Depth,
			pt(k.Camera), proj)
		if !ok {
			t.Errorf("screen[%d] %s ok=false, want true", i, k.Layer)
			continue
		}
		if !closeParallax(got, k.Screen) {
			t.Errorf("screen[%d] %s = %v, want %v", i, k.Layer, got, k.Screen)
		}
		if math.Abs(scale-k.Scale) >= epsParallax {
			t.Errorf("screen[%d] %s scale = %v, want %v", i, k.Layer, scale, k.Scale)
		}
	}
	// Factor 1 with no offset and no mirror is world-locked: the parallax
	// camera cancels out, so Screen equals a direct Project.
	near := layers["near"]
	for i, k := range cases.Screens {
		world := pt(k.World)
		want, _, wantOK := proj.Project(world, k.Depth)
		got, _, gotOK := near.Screen(world, k.Depth, pt(k.Camera), proj)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Errorf("world-lock[%d] = %v/%v, want %v/%v", i, got, gotOK, want, wantOK)
		}
	}
	// Screen-pinned layer in real per-frame use (projector rebuilt with
	// the same camera): the same world point lands on the same pixel no
	// matter where the camera sits.
	sky := layers["sky00"]
	world := core.V2(100, 50)
	var pinned core.Vec2
	for i, c := range cases.FollowPath {
		p, err := NewProjector(proj.Focal(), proj.Center(), pt(c))
		if err != nil {
			t.Fatalf("frame projector: %v", err)
		}
		got, _, ok := sky.Screen(world, 0, pt(c), p)
		if !ok {
			t.Fatalf("pinned frame[%d] ok=false", i)
		}
		if i == 0 {
			pinned = got
		} else if got != pinned {
			t.Fatalf("pinned frame[%d] = %v, want %v (camera %v)", i, got, pinned, c)
		}
	}
}

// B: zero/negative/bad inputs never panic, NaN, or hang.
func TestParallaxEdgesNoCrash(t *testing.T) {
	_, layers, cases := loadParallaxCases(t)
	// Negative repeat lengths are rejected, never silently wrapped.
	for i, d := range cases.BadLayers {
		if _, err := NewLayer(pt(d.Factor),
			pt(d.Offset),
			pt(d.Mirror)); err == nil {
			t.Errorf("bad layer[%d] %+v: want error", i, d)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad layer[%d] code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	badVecs := []core.Vec2{
		{X: math.NaN(), Y: 0}, {X: 0, Y: math.NaN()},
		{X: math.Inf(1), Y: 0}, {X: 0, Y: math.Inf(-1)},
	}
	for _, bad := range badVecs {
		for _, arg := range []string{"factor", "offset", "mirror"} {
			factor, offset, mirror := core.V2(1, 1), core.V2(0, 0), core.Vec2{}
			switch arg {
			case "factor":
				factor = bad
			case "offset":
				offset = bad
			case "mirror":
				mirror = bad
			}
			if _, err := NewLayer(factor, offset, mirror); err == nil {
				t.Errorf("NewLayer bad %s %v: want error", arg, bad)
			} else if core.CodeOf(err) != core.CodeInvalidArg {
				t.Errorf("NewLayer bad %s code = %v, want invalid-arg", arg, core.CodeOf(err))
			}
		}
	}
	// Bad world/camera inputs fail closed with zero outputs, no NaN.
	near, tile := layers["near"], layers["tile"]
	badShifts := []struct {
		world, camera core.Vec2
	}{
		{core.V2(math.NaN(), 0), core.V2(0, 0)},
		{core.V2(0, 0), core.V2(0, math.Inf(1))},
		{core.V2(math.Inf(-1), 0), core.V2(math.Inf(1), 0)},
	}
	for i, k := range badShifts {
		for _, l := range []Layer{near, tile} {
			if got, ok := l.Shift(k.world, k.camera); ok {
				t.Errorf("bad shift[%d] ok=true, want false", i)
			} else if got != (core.Vec2{}) {
				t.Errorf("bad shift[%d] = %v, want zero", i, got)
			}
			if _, _, ok := l.Screen(k.world, 0, k.camera, mustParallaxProj(t)); ok {
				t.Errorf("bad screen[%d] ok=true, want false", i)
			}
		}
	}
	// Zero mirror disables repeat: far positions pass through unwrapped.
	plain, err := NewLayer(core.V2(0.3, 0.3), core.V2(0, 0), core.Vec2{})
	if err != nil {
		t.Fatalf("plain layer: %v", err)
	}
	if got, ok := plain.Shift(core.V2(10000, 0), core.V2(10000, 0)); !ok || got != core.V2(17000, 0) {
		t.Errorf("zero mirror = %v/%v, want (17000,0)/true", got, ok)
	}
	// Negative raw positions wrap up into [0, mirror).
	if got, ok := tile.Shift(core.V2(-500, -20), core.V2(0, 0)); !ok || got.X < 0 || got.X >= 400 {
		t.Errorf("negative wrap = %v/%v, want x in [0,400)/true", got, ok)
	}
	// Behind-eye and NaN depths fail closed through the projector.
	if _, _, ok := near.Screen(core.V2(1, 2), -500, core.V2(0, 0), mustParallaxProj(t)); ok {
		t.Error("behind-eye depth ok=true, want false")
	}
	if _, _, ok := near.Screen(core.V2(1, 2), math.NaN(), core.V2(0, 0), mustParallaxProj(t)); ok {
		t.Error("NaN depth ok=true, want false")
	}
	if _, _, ok := near.ScreenQuad(
		[4]core.Vec2{{}, {}, {}, {}},
		[4]float64{0, 0, 0, -500},
		core.V2(0, 0), mustParallaxProj(t)); ok {
		t.Error("quad with one behind-eye corner ok=true, want false")
	}
}

func mustParallaxProj(t *testing.T) Projector {
	t.Helper()
	p, err := NewProjector(500, core.V2(400, 300), core.V2(0, 0))
	if err != nil {
		t.Fatalf("frame projector: %v", err)
	}
	return p
}

// C: game math draws nothing, so both backends share one number path;
// the boundary convert must be lossless and replay must be exact.
func TestParallaxBoundaryIdentical(t *testing.T) {
	proj, layers, cases := loadParallaxCases(t)
	for _, k := range cases.Screens {
		l := layers[k.Layer]
		world, cam := pt(k.World), pt(k.Camera)
		a, _, _ := l.Screen(world, k.Depth, cam, proj)
		b, _, _ := l.Screen(world, k.Depth, cam, proj)
		if a != b {
			t.Fatalf("replay diverged: %v vs %v", a, b)
		}
		if back := core.Vec2FromRenderPoint(a.ToRenderPoint()); back != a {
			t.Fatalf("boundary round trip = %v, want %v", back, a)
		}
		shifted, ok := l.Shift(world, cam)
		if !ok {
			t.Fatalf("shift ok=false for %v", k.World)
		}
		if again, _ := l.Shift(world, cam); again != shifted {
			t.Fatalf("shift replay diverged: %v vs %v", again, shifted)
		}
	}
}

// D: follow-path sweep completes with a measured cost.
func TestParallaxFollowPathPerf(t *testing.T) {
	proj, layers, cases := loadParallaxCases(t)
	worlds := make([]core.Vec2, len(cases.Shifts))
	for i, k := range cases.Shifts {
		worlds[i] = pt(k.World)
	}
	start := time.Now()
	n := 0
	for _, l := range layers {
		for _, c := range cases.FollowPath {
			cam := pt(c)
			for _, w := range worlds {
				if _, _, ok := l.Screen(w, 0, cam, proj); !ok {
					t.Fatalf("perf screen ok=false (layer %+v)", l)
				}
				n++
			}
		}
	}
	el := time.Since(start)
	t.Logf("parallax-follow: %d screens (%d layers x %d worlds x %d cams) in %v (%.1f ns/op)",
		n, len(layers), len(worlds), len(cases.FollowPath), el, float64(el.Nanoseconds())/float64(n))
}

// E: 10 km follow run stays seamless with no drift.
func TestParallaxLongRunSeamless(t *testing.T) {
	proj, layers, _ := loadParallaxCases(t)
	tile := layers["tile"]
	world := core.V2(100, 50)
	mirrorStep := core.V2(400, 0)
	for i := 0; i <= 100; i++ {
		cam := core.V2(float64(i*100), 0)
		shifted, ok := tile.Shift(world, cam)
		if !ok {
			t.Fatalf("step %d ok=false", i)
		}
		if shifted.X < 0 || shifted.X >= 400 {
			t.Fatalf("step %d shift %v escapes [0,400)", i, shifted)
		}
		if shifted.Y != 50 {
			t.Fatalf("step %d shift %v drifts off y=50", i, shifted)
		}
		a, _, okA := tile.Screen(world, 0, cam, proj)
		b, _, okB := tile.Screen(world.Add(mirrorStep), 0, cam, proj)
		if !okA || !okB {
			t.Fatalf("step %d seamless ok=%v/%v", i, okA, okB)
		}
		// Mirror twins share one screen: the long-road seam holds.
		if !seamOK(a, b) {
			t.Fatalf("step %d seam breaks: %v vs %v", i, a, b)
		}
	}
	// Same inputs replay bitwise identical after 1000 laps.
	first, _, _ := tile.Screen(world, 0, core.V2(2300, 720), proj)
	last := first
	for lap := 0; lap < 1000; lap++ {
		s, _, ok := tile.Screen(world, 0, core.V2(2300, 720), proj)
		if !ok {
			t.Fatalf("lap %d ok=false", lap)
		}
		last = s
	}
	if last != first {
		t.Fatalf("1000 laps drifted: %v vs %v", last, first)
	}
}

// F: offscreen golden stands in for the window.
// Capability 1.3b is window-exempt in W1 (pure math); the frozen quad in
// parallax_cases.json is the offscreen evidence both backends share, and
// the follow window (game_camera --case=follow) is built later.
func TestParallaxOffscreenGolden(t *testing.T) {
	proj, layers, cases := loadParallaxCases(t)
	l, ok := layers[cases.Quad.Layer]
	if !ok {
		t.Fatalf("unknown quad layer %q", cases.Quad.Layer)
	}
	var corners [4]core.Vec2
	var depths [4]float64
	for i := 0; i < 4; i++ {
		corners[i] = pt(cases.Quad.Corners[i])
		depths[i] = cases.Quad.Depths[i]
	}
	cam := pt(cases.Quad.Camera)
	for i := 0; i < 4; i++ {
		if got, ok := l.Shift(corners[i], cam); !ok || !closeParallax(got, cases.Quad.Shifts[i]) {
			t.Errorf("quad shift[%d] = %v/%v, want %v/true", i, got, ok, cases.Quad.Shifts[i])
		}
	}
	screens, scales, ok := l.ScreenQuad(corners, depths, cam, proj)
	if !ok {
		t.Fatal("quad ok=false, want true")
	}
	for i := 0; i < 4; i++ {
		if !closeParallax(screens[i], cases.Quad.Screens[i]) {
			t.Errorf("golden[%d] = %v, want %v", i, screens[i], cases.Quad.Screens[i])
		}
		if math.Abs(scales[i]-cases.Quad.Scales[i]) >= epsParallax {
			t.Errorf("golden scale[%d] = %v, want %v", i, scales[i], cases.Quad.Scales[i])
		}
	}
	// Trapezoid shape: near edge wider than far edge.
	near := math.Abs(screens[1].X - screens[0].X)
	far := math.Abs(screens[2].X - screens[3].X)
	if !(far < near) {
		t.Errorf("trapezoid far %v not narrower than near %v", far, near)
	}
	// Mirror-separated twins share one screen: the long-road seam holds.
	for i, k := range cases.Seamless {
		l := layers[k.Layer]
		a, _, okA := l.Screen(pt(k.WorldA), k.Depth, pt(k.Camera), proj)
		b, _, okB := l.Screen(pt(k.WorldB), k.Depth, pt(k.Camera), proj)
		if !okA || !okB {
			t.Errorf("seam[%d] ok=%v/%v, want true/true", i, okA, okB)
			continue
		}
		if !seamOK(a, b) {
			t.Errorf("seam[%d] = %v vs %v, want within %v", i, a, b, epsParallax)
		}
	}
}
