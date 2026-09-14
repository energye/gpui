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

const epsProject = 1e-9

type projectEntry struct {
	World  [2]float64 `json:"world"`
	Depth  float64    `json:"depth"`
	Screen [2]float64 `json:"screen"`
	Scale  float64    `json:"scale"`
}

type scaleEntry struct {
	Depth float64 `json:"depth"`
	Scale float64 `json:"scale"`
}

type quadBlock struct {
	Corners [4][2]float64 `json:"corners"`
	Depths  [4]float64    `json:"depths"`
	Screens [4][2]float64 `json:"screens"`
	Scales  [4]float64    `json:"scales"`
}

type unprojectEntry struct {
	Screen [2]float64 `json:"screen"`
	Depth  float64    `json:"depth"`
	World  [2]float64 `json:"world"`
}

type projectFile struct {
	Focal        float64          `json:"focal"`
	Center       [2]float64       `json:"center"`
	Camera       [2]float64       `json:"camera"`
	Project      []projectEntry   `json:"project"`
	Scales       []scaleEntry     `json:"scales"`
	RejectDepths []float64        `json:"reject_depths"`
	Quad         quadBlock        `json:"quad"`
	Unproject    []unprojectEntry `json:"unproject"`
}

type roadSection struct {
	Left  [2]float64 `json:"left"`
	Right [2]float64 `json:"right"`
	Depth float64    `json:"depth"`
}

type roadFile struct {
	Focal    float64       `json:"focal"`
	Center   [2]float64    `json:"center"`
	Camera   [2]float64    `json:"camera"`
	Sections []roadSection `json:"sections"`
}

type ringFile struct {
	Focal  float64      `json:"focal"`
	Center [2]float64   `json:"center"`
	Camera [2]float64   `json:"camera"`
	Depth  float64      `json:"depth"`
	Points [][2]float64 `json:"points"`
}

func newProjectorFor(t *testing.T, focal float64, center, camera [2]float64) Projector {
	t.Helper()
	proj, err := NewProjector(focal, core.V2(center[0], center[1]), core.V2(camera[0], camera[1]))
	if err != nil {
		t.Fatalf("NewProjector: %v", err)
	}
	return proj
}

func loadProjectCases(t *testing.T) (Projector, projectFile) {
	t.Helper()
	var cases projectFile
	raw, err := os.ReadFile(filepath.Join("testdata", "project_cases.json"))
	if err != nil {
		t.Fatalf("read project_cases.json: %v", err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decode project_cases.json: %v", err)
	}
	return newProjectorFor(t, cases.Focal, cases.Center, cases.Camera), cases
}

func loadRoad(t *testing.T) (Projector, roadFile) {
	t.Helper()
	var road roadFile
	raw, err := os.ReadFile(filepath.Join("testdata", "road_56.json"))
	if err != nil {
		t.Fatalf("read road_56.json: %v", err)
	}
	if err := json.Unmarshal(raw, &road); err != nil {
		t.Fatalf("decode road_56.json: %v", err)
	}
	if len(road.Sections) != 57 {
		t.Fatalf("sections = %d, want 57 (56 segments)", len(road.Sections))
	}
	return newProjectorFor(t, road.Focal, road.Center, road.Camera), road
}

func loadRing(t *testing.T) (Projector, ringFile) {
	t.Helper()
	var ring ringFile
	raw, err := os.ReadFile(filepath.Join("testdata", "ring_closed.json"))
	if err != nil {
		t.Fatalf("read ring_closed.json: %v", err)
	}
	if err := json.Unmarshal(raw, &ring); err != nil {
		t.Fatalf("decode ring_closed.json: %v", err)
	}
	return newProjectorFor(t, ring.Focal, ring.Center, ring.Camera), ring
}

func quadInputs(q quadBlock) ([4]core.Vec2, [4]float64) {
	var corners [4]core.Vec2
	var depths [4]float64
	for i := 0; i < 4; i++ {
		corners[i] = core.V2(q.Corners[i][0], q.Corners[i][1])
		depths[i] = q.Depths[i]
	}
	return corners, depths
}

func closeScreen(got core.Vec2, want [2]float64) bool {
	return math.Abs(got.X-want[0]) < epsProject && math.Abs(got.Y-want[1]) < epsProject
}

// A: normal inputs land on the frozen screen numbers.
func TestProjectFromCases(t *testing.T) {
	proj, cases := loadProjectCases(t)
	for i, k := range cases.Project {
		got, scale, ok := proj.Project(core.V2(k.World[0], k.World[1]), k.Depth)
		if !ok {
			t.Errorf("project[%d] ok=false, want true", i)
			continue
		}
		if !closeScreen(got, k.Screen) {
			t.Errorf("project[%d] screen = %v, want %v", i, got, k.Screen)
		}
		if math.Abs(scale-k.Scale) >= epsProject {
			t.Errorf("project[%d] scale = %v, want %v", i, scale, k.Scale)
		}
	}
	for i, k := range cases.Scales {
		got, ok := proj.DepthToScale(k.Depth)
		if !ok {
			t.Errorf("scale[%d] ok=false, want true", i)
			continue
		}
		if math.Abs(got-k.Scale) >= epsProject {
			t.Errorf("scale[%d] = %v, want %v", i, got, k.Scale)
		}
	}
	corners, depths := quadInputs(cases.Quad)
	screens, scales, ok := proj.ProjectQuad(corners, depths)
	if !ok {
		t.Fatal("quad ok=false, want true")
	}
	for i := 0; i < 4; i++ {
		if !closeScreen(screens[i], cases.Quad.Screens[i]) {
			t.Errorf("quad screen[%d] = %v, want %v", i, screens[i], cases.Quad.Screens[i])
		}
		if math.Abs(scales[i]-cases.Quad.Scales[i]) >= epsProject {
			t.Errorf("quad scale[%d] = %v, want %v", i, scales[i], cases.Quad.Scales[i])
		}
	}
	for i, k := range cases.Unproject {
		got, ok := proj.Unproject(core.V2(k.Screen[0], k.Screen[1]), k.Depth)
		if !ok {
			t.Errorf("unproject[%d] ok=false, want true", i)
			continue
		}
		if !closeScreen(got, k.World) {
			t.Errorf("unproject[%d] = %v, want %v", i, got, k.World)
		}
	}
}

// B: zero/negative/behind/NaN/Inf never panic, NaN, or hang.
func TestProjectEdgesNoCrash(t *testing.T) {
	proj, cases := loadProjectCases(t)
	// Depth zero stays on the screen.
	if _, scale, ok := proj.Project(core.V2(10, 20), 0); !ok || scale != 1 {
		t.Errorf("depth zero = scale %v ok %v, want 1 true", scale, ok)
	}
	// Valid negative depth (closer) enlarges, does not crash.
	if _, scale, ok := proj.Project(core.V2(10, 20), -250); !ok || scale != 2 {
		t.Errorf("depth -250 = scale %v ok %v, want 2 true", scale, ok)
	}
	// At/behind the eye: ok=false, zero outputs, no NaN.
	for i, d := range cases.RejectDepths {
		screen, scale, ok := proj.Project(core.V2(100, 50), d)
		if ok {
			t.Errorf("reject[%d] depth %v: ok=true, want false", i, d)
		}
		if screen != (core.Vec2{}) || scale != 0 {
			t.Errorf("reject[%d] depth %v: got %v/%v, want zero/0", i, d, screen, scale)
		}
		if math.IsNaN(screen.X) || math.IsNaN(screen.Y) || math.IsNaN(scale) {
			t.Errorf("reject[%d] depth %v produced NaN", i, d)
		}
		if _, ok := proj.DepthToScale(d); ok {
			t.Errorf("reject scale[%d] depth %v: ok=true, want false", i, d)
		}
	}
	badWorlds := []core.Vec2{
		{X: math.NaN(), Y: 0}, {X: 0, Y: math.NaN()},
		{X: math.Inf(1), Y: 0}, {X: 0, Y: math.Inf(-1)},
	}
	for i, w := range badWorlds {
		if _, _, ok := proj.Project(w, 0); ok {
			t.Errorf("bad world[%d] ok=true, want false", i)
		}
	}
	for _, d := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, _, ok := proj.Project(core.V2(1, 2), d); ok {
			t.Errorf("bad depth %v ok=true, want false", d)
		}
		if _, ok := proj.Unproject(core.V2(400, 300), d); ok {
			t.Errorf("unproject bad depth %v ok=true, want false", d)
		}
	}
	if _, _, ok := proj.ProjectQuad(
		[4]core.Vec2{{}, {}, {}, {}},
		[4]float64{0, 0, 0, -500},
	); ok {
		t.Error("quad with one behind-eye corner ok=true, want false")
	}
	// Bad constructors never guess.
	for _, focal := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := NewProjector(focal, core.V2(400, 300), core.V2(0, 0)); err == nil {
			t.Errorf("focal %v: want error", focal)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("focal %v code = %v, want invalid-arg", focal, core.CodeOf(err))
		}
	}
	if _, err := NewProjector(500, core.V2(math.NaN(), 0), core.V2(0, 0)); err == nil {
		t.Error("NaN center: want error")
	}
}

// C: CPU and GPU receive identical screen corners.
// The projector draws nothing, so both backends share one number path;
// the boundary convert must be lossless, and replay must be exact.
func TestProjectBoundaryIdentical(t *testing.T) {
	proj, cases := loadProjectCases(t)
	corners, depths := quadInputs(cases.Quad)
	screens, _, ok := proj.ProjectQuad(corners, depths)
	if !ok {
		t.Fatal("quad ok=false")
	}
	for i, s := range screens {
		back := core.Vec2FromRenderPoint(s.ToRenderPoint())
		if back != s {
			t.Errorf("corner[%d] boundary round trip = %v, want %v", i, back, s)
		}
	}
	// Same input twice gives bitwise identical output: no backend drift.
	a, _, _ := proj.Project(core.V2(100, 50), 500)
	b, _, _ := proj.Project(core.V2(100, 50), 500)
	if a != b {
		t.Errorf("replay diverged: %v vs %v", a, b)
	}
}

// D: full 56-segment road reprojects with a measured cost.
func TestProjectRoad56Perf(t *testing.T) {
	proj, road := loadRoad(t)
	start := time.Now()
	widths := make([]float64, len(road.Sections))
	for i, s := range road.Sections {
		l, _, okL := proj.Project(core.V2(s.Left[0], s.Left[1]), s.Depth)
		r, _, okR := proj.Project(core.V2(s.Right[0], s.Right[1]), s.Depth)
		if !okL || !okR {
			t.Fatalf("road depth %v: ok %v/%v, want true/true", s.Depth, okL, okR)
		}
		widths[i] = r.X - l.X
	}
	el := time.Since(start)
	n := len(road.Sections) * 2
	t.Logf("project-road56: %d points in %v (%.1f ns/op)", n, el, float64(el.Nanoseconds())/float64(n))
	// Far side must be narrower: perspective shrinks with depth.
	if !(widths[len(widths)-1] < widths[0]) {
		t.Errorf("far width %v not narrower than near %v", widths[len(widths)-1], widths[0])
	}
	for i := 1; i < len(widths); i++ {
		if widths[i] > widths[i-1]+1e-9 {
			t.Errorf("width[%d] %v wider than width[%d] %v, want monotonic shrink", i, widths[i], i-1, widths[i-1])
		}
	}
}

// E: closed ring replays 1000 laps with no drift.
func TestProjectLoopClosed1000(t *testing.T) {
	proj, ring := loadRing(t)
	first := make([]core.Vec2, len(ring.Points))
	for i, p := range ring.Points {
		s, _, ok := proj.Project(core.V2(p[0], p[1]), ring.Depth)
		if !ok {
			t.Fatalf("ring point[%d] ok=false", i)
		}
		first[i] = s
	}
	// 1000 laps: same inputs must give bitwise identical screens.
	last := make([]core.Vec2, 0, len(ring.Points))
	for lap := 0; lap < 1000; lap++ {
		last = last[:0]
		for _, p := range ring.Points {
			s, _, ok := proj.Project(core.V2(p[0], p[1]), ring.Depth)
			if !ok {
				t.Fatalf("lap %d ok=false", lap)
			}
			last = append(last, s)
		}
	}
	for i := range first {
		if last[i] != first[i] {
			t.Fatalf("lap 1000 point[%d] drifted: %v vs %v", i, last[i], first[i])
		}
	}
	// Round trip closes within float tolerance.
	for i, p := range ring.Points {
		world := core.V2(p[0], p[1])
		screen, _, _ := proj.Project(world, ring.Depth)
		back, ok := proj.Unproject(screen, ring.Depth)
		if !ok {
			t.Fatalf("round trip[%d] ok=false", i)
		}
		if !back.ApproxEqual(world, 1e-9) {
			t.Errorf("round trip[%d] = %v, want %v", i, back, world)
		}
	}
}

// F: offscreen golden comparison stands in for the window.
// Capability 1.1 is window-exempt (pure math); the frozen screens in
// project_cases.json are the offscreen evidence both backends share.
func TestProjectOffscreenGolden(t *testing.T) {
	proj, cases := loadProjectCases(t)
	corners, depths := quadInputs(cases.Quad)
	screens, _, ok := proj.ProjectQuad(corners, depths)
	if !ok {
		t.Fatal("quad ok=false")
	}
	// Trapezoid shape: near edge wider than far edge.
	near := math.Abs(screens[1].X - screens[0].X)
	far := math.Abs(screens[2].X - screens[3].X)
	if !(far < near) {
		t.Errorf("trapezoid far %v not narrower than near %v", far, near)
	}
	for i := 0; i < 4; i++ {
		if !closeScreen(screens[i], cases.Quad.Screens[i]) {
			t.Errorf("golden[%d] = %v, want %v", i, screens[i], cases.Quad.Screens[i])
		}
	}
}
