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

const epsCamera = 1e-9

type pointCase struct {
	World  [2]float64 `json:"world"`
	Screen [2]float64 `json:"screen"`
}

type viewCase struct {
	Name        string      `json:"name"`
	Pos         [2]float64  `json:"pos"`
	Zoom        [2]float64  `json:"zoom"`
	Rot         float64     `json:"rot"`
	Shake       [2]float64  `json:"shake"`
	Limit       [4]float64  `json:"limit"`
	Anchor      [2]float64  `json:"anchor"`
	Viewport    [2]float64  `json:"viewport"`
	WantPos     [2]float64  `json:"want_pos"`
	WantView    [6]float64  `json:"want_view"`
	WantVisible [4]float64  `json:"want_visible"`
	Points      []pointCase `json:"points"`
}

type followCase struct {
	Start   [2]float64   `json:"start"`
	Smooth  float64      `json:"smoothing"`
	Limit   [4]float64   `json:"limit"`
	Targets [][2]float64 `json:"targets"`
	Want    [][2]float64 `json:"want"`
}

type decayCase struct {
	Start  [2]float64   `json:"start"`
	Factor float64      `json:"factor"`
	Steps  int          `json:"steps"`
	Want   [][2]float64 `json:"want"`
}

type cameraFile struct {
	Views      []viewCase `json:"views"`
	Follow     followCase `json:"follow"`
	FollowSnap followCase `json:"follow_snap"`
	Decay      decayCase  `json:"decay"`
}

func loadCameraCases(t *testing.T) cameraFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "camera_cases.json"))
	if err != nil {
		t.Fatalf("read camera_cases.json: %v", err)
	}
	var cases cameraFile
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decode camera_cases.json: %v", err)
	}
	if len(cases.Views) == 0 {
		t.Fatal("camera_cases.json has no views")
	}
	return cases
}

func mustSet(t *testing.T, name, op string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %s: %v", name, op, err)
	}
}

func cameraFromView(t *testing.T, v viewCase) Camera {
	t.Helper()
	cam, err := NewCamera(core.V2(v.Viewport[0], v.Viewport[1]))
	mustSet(t, v.Name, "NewCamera", err)
	mustSet(t, v.Name, "SetLimit", cam.SetLimit(core.NewRect(v.Limit[0], v.Limit[1], v.Limit[2], v.Limit[3])))
	mustSet(t, v.Name, "SetAnchor", cam.SetAnchor(core.V2(v.Anchor[0], v.Anchor[1])))
	mustSet(t, v.Name, "SetPos", cam.SetPos(core.V2(v.Pos[0], v.Pos[1])))
	mustSet(t, v.Name, "SetZoom", cam.SetZoom(core.V2(v.Zoom[0], v.Zoom[1])))
	mustSet(t, v.Name, "SetRotation", cam.SetRotation(v.Rot))
	mustSet(t, v.Name, "SetShake", cam.SetShake(core.V2(v.Shake[0], v.Shake[1])))
	return cam
}

func closeView(got core.Mat2D, want [6]float64) bool {
	g := [6]float64{got.A, got.B, got.C, got.D, got.E, got.F}
	for i := range g {
		if math.Abs(g[i]-want[i]) >= epsCamera {
			return false
		}
	}
	return true
}

func closeRect(got core.Rect, want [4]float64) bool {
	return math.Abs(got.X-want[0]) < epsCamera && math.Abs(got.Y-want[1]) < epsCamera &&
		math.Abs(got.W-want[2]) < epsCamera && math.Abs(got.H-want[3]) < epsCamera
}

// A: position, zoom, angle, shake, limit, smoothing, anchor all land on
// the frozen numbers.
func TestCameraViewFromCases(t *testing.T) {
	cases := loadCameraCases(t)
	for _, v := range cases.Views {
		cam := cameraFromView(t, v)
		if got := cam.Pos(); !got.ApproxEqual(core.V2(v.WantPos[0], v.WantPos[1]), epsCamera) {
			t.Errorf("%s: pos = %v, want %v", v.Name, got, v.WantPos)
		}
		if got := cam.View(); !closeView(got, v.WantView) {
			t.Errorf("%s: view = %+v, want %v", v.Name, got, v.WantView)
		}
		for i, p := range v.Points {
			got, ok := cam.WorldToScreen(core.V2(p.World[0], p.World[1]))
			if !ok {
				t.Errorf("%s: point[%d] ok=false, want true", v.Name, i)
				continue
			}
			if math.Abs(got.X-p.Screen[0]) >= epsCamera || math.Abs(got.Y-p.Screen[1]) >= epsCamera {
				t.Errorf("%s: point[%d] = %v, want %v", v.Name, i, got, p.Screen)
			}
			back, ok := cam.ScreenToWorld(core.V2(p.Screen[0], p.Screen[1]))
			if !ok {
				t.Errorf("%s: back[%d] ok=false, want true", v.Name, i)
				continue
			}
			if !back.ApproxEqual(core.V2(p.World[0], p.World[1]), epsCamera) {
				t.Errorf("%s: back[%d] = %v, want %v", v.Name, i, back, p.World)
			}
		}
		if got := cam.VisibleWorldRect(); !closeRect(got, v.WantVisible) {
			t.Errorf("%s: visible = %v, want %v", v.Name, got, v.WantVisible)
		}
		// The effective center always lands on the anchor pixel.
		eff, ok := cam.WorldToScreen(cam.EffectivePos())
		if !ok {
			t.Errorf("%s: effective pos not mappable", v.Name)
			continue
		}
		wantAnchor := core.V2(v.Anchor[0]*v.Viewport[0], v.Anchor[1]*v.Viewport[1])
		if !eff.ApproxEqual(wantAnchor, epsCamera) {
			t.Errorf("%s: center maps to %v, want anchor %v", v.Name, eff, wantAnchor)
		}
	}
}

// A: follow smoothing and shake decay follow the frozen sequences.
func TestCameraFollowAndDecay(t *testing.T) {
	cases := loadCameraCases(t)
	runFollow := func(name string, f followCase) {
		t.Helper()
		cam, err := NewCamera(core.V2(800, 600))
		mustSet(t, name, "NewCamera", err)
		mustSet(t, name, "SetLimit", cam.SetLimit(core.NewRect(f.Limit[0], f.Limit[1], f.Limit[2], f.Limit[3])))
		mustSet(t, name, "SetSmoothing", cam.SetSmoothing(f.Smooth))
		// Snap the start into place regardless of smoothing.
		mustSet(t, name, "snap", cam.SetSmoothing(0))
		mustSet(t, name, "start", cam.Follow(core.V2(f.Start[0], f.Start[1])))
		mustSet(t, name, "restore smoothing", cam.SetSmoothing(f.Smooth))
		for i, tgt := range f.Targets {
			if err := cam.Follow(core.V2(tgt[0], tgt[1])); err != nil {
				t.Fatalf("%s: follow[%d]: %v", name, i, err)
			}
			if got := cam.Pos(); !got.ApproxEqual(core.V2(f.Want[i][0], f.Want[i][1]), epsCamera) {
				t.Errorf("%s: follow[%d] = %v, want %v", name, i, got, f.Want[i])
			}
		}
	}
	runFollow("follow", cases.Follow)
	runFollow("follow_snap", cases.FollowSnap)

	cam, err := NewCamera(core.V2(800, 600))
	if err != nil {
		t.Fatalf("decay: NewCamera: %v", err)
	}
	if err := cam.SetShake(core.V2(cases.Decay.Start[0], cases.Decay.Start[1])); err != nil {
		t.Fatalf("decay: SetShake: %v", err)
	}
	for i := 0; i < cases.Decay.Steps; i++ {
		if err := cam.DecayShake(cases.Decay.Factor); err != nil {
			t.Fatalf("decay step %d: %v", i, err)
		}
		if got := cam.Shake(); !got.ApproxEqual(core.V2(cases.Decay.Want[i][0], cases.Decay.Want[i][1]), epsCamera) {
			t.Errorf("decay[%d] = %v, want %v", i, got, cases.Decay.Want[i])
		}
	}
}

// B: zero/negative/bad inputs clamp or error, never panic or NaN.
func TestCameraEdgesNoCrash(t *testing.T) {
	cam, err := NewCamera(core.V2(800, 600))
	if err != nil {
		t.Fatalf("NewCamera: %v", err)
	}
	// Zero and negative zoom clamp to MinZoom, no error.
	for i, z := range []core.Vec2{core.V2(0, 0), core.V2(-2, 3), core.V2(1, -1)} {
		if err := cam.SetZoom(z); err != nil {
			t.Errorf("zoom[%d] %v: unexpected error %v", i, z, err)
		}
		got := cam.Zoom()
		if got.X < MinZoom || got.Y < MinZoom {
			t.Errorf("zoom[%d] %v: got %v, want each >= %v", i, z, got, MinZoom)
		}
	}
	// Negative viewport clamps to 0; anchor clamps to [0,1].
	if err := cam.SetViewport(core.V2(-800, 100)); err != nil {
		t.Errorf("negative viewport: unexpected error %v", err)
	}
	if got := cam.Viewport(); got.X != 0 || got.Y != 100 {
		t.Errorf("negative viewport = %v, want (0,100)", got)
	}
	if err := cam.SetAnchor(core.V2(2, -1)); err != nil {
		t.Errorf("anchor out of range: unexpected error %v", err)
	}
	if got := cam.Anchor(); got != core.V2(1, 0) {
		t.Errorf("anchor = %v, want (1,0)", got)
	}
	if err := cam.SetSmoothing(5); err != nil {
		t.Errorf("smoothing out of range: unexpected error %v", err)
	}
	if cam.Smoothing() != 1 {
		t.Errorf("smoothing = %v, want 1", cam.Smoothing())
	}
	// Empty limit disables clamping: far positions stick.
	if err := cam.SetLimit(core.Rect{}); err != nil {
		t.Errorf("empty limit: unexpected error %v", err)
	}
	if err := cam.SetPos(core.V2(1e6, -1e6)); err != nil {
		t.Errorf("far pos: unexpected error %v", err)
	}
	if cam.Pos() != core.V2(1e6, -1e6) {
		t.Errorf("far pos = %v, want it kept", cam.Pos())
	}
	// Rotation normalizes into [-pi, pi].
	if err := cam.SetRotation(4 * math.Pi); err != nil {
		t.Errorf("big rotation: unexpected error %v", err)
	}
	if r := cam.Rotation(); r < -math.Pi-1e-12 || r > math.Pi+1e-12 {
		t.Errorf("rotation = %v, want within [-pi,pi]", r)
	}
	// Decay factor clamps: negative keeps, huge clears.
	if err := cam.SetShake(core.V2(8, -6)); err != nil {
		t.Fatalf("SetShake: %v", err)
	}
	if err := cam.DecayShake(-3); err != nil {
		t.Errorf("negative decay: unexpected error %v", err)
	}
	if cam.Shake() != core.V2(8, -6) {
		t.Errorf("negative decay shake = %v, want (8,-6) kept", cam.Shake())
	}
	if err := cam.DecayShake(5); err != nil {
		t.Errorf("big decay: unexpected error %v", err)
	}
	if cam.Shake() != (core.Vec2{}) {
		t.Errorf("big decay shake = %v, want zero", cam.Shake())
	}
	// NaN/Inf never change state and report invalid-arg.
	before := cam
	badVecs := []core.Vec2{
		{X: math.NaN(), Y: 0}, {X: 0, Y: math.NaN()},
		{X: math.Inf(1), Y: 0}, {X: 0, Y: math.Inf(-1)},
	}
	for i, v := range badVecs {
		if err := cam.SetPos(v); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad pos[%d]: err = %v, want invalid-arg", i, err)
		}
		if err := cam.SetZoom(v); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad zoom[%d]: err = %v, want invalid-arg", i, err)
		}
		if err := cam.SetShake(v); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad shake[%d]: err = %v, want invalid-arg", i, err)
		}
		if err := cam.SetAnchor(v); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad anchor[%d]: err = %v, want invalid-arg", i, err)
		}
		if err := cam.SetViewport(v); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad viewport[%d]: err = %v, want invalid-arg", i, err)
		}
		if err := cam.Follow(v); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad follow[%d]: err = %v, want invalid-arg", i, err)
		}
	}
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := cam.SetRotation(f); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad rotation %v: err = %v, want invalid-arg", f, err)
		}
		if err := cam.SetSmoothing(f); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad smoothing %v: err = %v, want invalid-arg", f, err)
		}
		if err := cam.DecayShake(f); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad decay %v: err = %v, want invalid-arg", f, err)
		}
	}
	if cam != before {
		t.Error("bad inputs changed camera state, want unchanged")
	}
	if _, err := NewCamera(core.V2(math.NaN(), 0)); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("NaN viewport constructor err = %v, want invalid-arg", err)
	}
	// Bad map inputs report ok=false with zero output, never NaN.
	for i, v := range badVecs {
		if s, ok := cam.WorldToScreen(v); ok || s != (core.Vec2{}) {
			t.Errorf("bad world[%d] = %v/%v, want zero/false", i, s, ok)
		}
		if w, ok := cam.ScreenToWorld(v); ok || w != (core.Vec2{}) {
			t.Errorf("bad screen[%d] = %v/%v, want zero/false", i, w, ok)
		}
	}
	// Huge-but-finite inputs never produce NaN: either a finite screen
	// or ok=false with a zero point.
	for i, w := range []core.Vec2{core.V2(1e308, 0), core.V2(0, -1e308)} {
		s, ok := cam.WorldToScreen(w)
		if math.IsNaN(s.X) || math.IsNaN(s.Y) {
			t.Errorf("huge world[%d] produced NaN", i)
		}
		if !ok && s != (core.Vec2{}) {
			t.Errorf("huge world[%d] = %v, want zero on ok=false", i, s)
		}
	}
}

// C does not apply (pure math, draws nothing): the render boundary must
// be lossless and replays bitwise identical instead.
func TestCameraBoundaryIdentical(t *testing.T) {
	cases := loadCameraCases(t)
	for _, v := range cases.Views {
		cam := cameraFromView(t, v)
		back := core.Mat2DFromRenderMatrix(cam.View().ToRenderMatrix())
		if back != cam.View() {
			t.Errorf("%s: matrix boundary round trip diverged", v.Name)
		}
		a, _ := cam.WorldToScreen(core.V2(100, 50))
		b, _ := cam.WorldToScreen(core.V2(100, 50))
		if a != b {
			t.Errorf("%s: replay diverged: %v vs %v", v.Name, a, b)
		}
	}
}

// D: 10k view + map ops complete with a measured cost.
func TestCameraPerf10k(t *testing.T) {
	cam, err := NewCamera(core.V2(800, 600))
	if err != nil {
		t.Fatalf("NewCamera: %v", err)
	}
	if err := cam.SetZoom(core.V2(2, 2)); err != nil {
		t.Fatalf("SetZoom: %v", err)
	}
	if err := cam.SetRotation(0.3); err != nil {
		t.Fatalf("SetRotation: %v", err)
	}
	const n = 10000
	var acc core.Vec2
	start := time.Now()
	for i := 0; i < n; i++ {
		w := core.V2(float64(i%800), float64(i%600))
		s, ok := cam.WorldToScreen(w)
		if !ok {
			t.Fatalf("op %d ok=false", i)
		}
		_ = cam.View()
		acc = acc.Add(s)
	}
	el := time.Since(start)
	t.Logf("camera-10k: %d view+map ops in %v (%.1f ns/op)", n, el, float64(el.Nanoseconds())/n)
	if acc.IsZero() {
		t.Error("perf loop folded to zero, benchmark invalid")
	}
}

// E: long runs do not drift.
func TestCameraLongRunNoDrift(t *testing.T) {
	mk := func() Camera {
		cam, err := NewCamera(core.V2(800, 600))
		if err != nil {
			t.Fatalf("NewCamera: %v", err)
		}
		if err := cam.SetSmoothing(0.1); err != nil {
			t.Fatalf("SetSmoothing: %v", err)
		}
		return cam
	}
	a, b := mk(), mk()
	target := core.V2(1000, 0)
	const steps = 100000
	for i := 0; i < steps; i++ {
		if err := a.Follow(target); err != nil {
			t.Fatalf("follow a step %d: %v", i, err)
		}
		if err := b.Follow(target); err != nil {
			t.Fatalf("follow b step %d: %v", i, err)
		}
	}
	if a.Pos() != b.Pos() {
		t.Fatalf("replay diverged: %v vs %v", a.Pos(), b.Pos())
	}
	if math.Abs(a.Pos().X-1000) > 1e-6 || a.Pos().Y != 0 {
		t.Errorf("converged pos = %v, want ~(1000,0)", a.Pos())
	}
	// Repeated maps stay bitwise identical; round trips close.
	first, _ := a.WorldToScreen(core.V2(123, 456))
	for i := 0; i < 10000; i++ {
		s, _ := a.WorldToScreen(core.V2(123, 456))
		if s != first {
			t.Fatalf("map drifted at rep %d: %v vs %v", i, s, first)
		}
	}
	back, ok := a.ScreenToWorld(first)
	if !ok {
		t.Fatal("round trip ok=false")
	}
	if !back.ApproxEqual(core.V2(123, 456), epsCamera) {
		t.Errorf("round trip = %v, want (123,456)", back)
	}
}

// F: offscreen golden stands in for the window (window-exempt pure math).
// The frozen screens in camera_cases.json are the evidence both backends
// share; shape assertions below pin the meaning, not just the numbers.
func TestCameraOffscreenGolden(t *testing.T) {
	cases := loadCameraCases(t)
	byName := map[string]viewCase{}
	for _, v := range cases.Views {
		byName[v.Name] = v
	}
	centered, zoomed, rotated := byName["centered"], byName["zoomed"], byName["rotated90"]
	centerCam := cameraFromView(t, centered)
	// Zooming in shrinks the visible world area.
	centerArea := centerCam.VisibleWorldRect().Area()
	zoomCam := cameraFromView(t, zoomed)
	zoomArea := zoomCam.VisibleWorldRect().Area()
	if !(zoomArea < centerArea) {
		t.Errorf("zoomed area %v not smaller than centered %v", zoomArea, centerArea)
	}
	// A 90-degree roll swaps visible width and height.
	rotCam := cameraFromView(t, rotated)
	rv := rotCam.VisibleWorldRect()
	cv := centerCam.VisibleWorldRect()
	if math.Abs(rv.W-cv.H) >= epsCamera ||
		math.Abs(rv.H-cv.W) >= epsCamera {
		t.Errorf("rotated visible = %v, want swapped centered %v", rv, cv)
	}
	// Golden numbers stay frozen.
	for _, v := range []viewCase{centered, zoomed, rotated} {
		cam := cameraFromView(t, v)
		if got := cam.View(); !closeView(got, v.WantView) {
			t.Errorf("%s golden view = %+v, want %v", v.Name, got, v.WantView)
		}
	}
}
