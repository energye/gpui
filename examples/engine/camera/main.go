// Command game_camera is the S55 camera window plus the S77 listener rider.
//
// Lens follow plus limits plus smoothing plus a parallax long road,
// all through the real engine/camera package (read-only from here).
// One audio listener glues itself to the camera each frame (S77) and
// the window shows the live center/right/far mixes through it.
//
// Modes:
//
//	go run ./examples/engine/camera -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/camera -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/engine/camera
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_camera. First run writes the golden
// baseline into testdata/; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/engine/audio"
	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "game-camera"
	scenario   = "game_camera"
	goldenPath = "examples/engine/camera/testdata/camera_follow_golden.png"
	frozenPath = "examples/engine/camera/testdata/camera_cases.json"

	// epsEngine is the bitwise-closeness bar for engine numbers.
	epsEngine = 1e-9
	// probePixelTol is the per-channel tolerance (0-255 steps) for the
	// offscreen pixel asserts. The golden mask compare stays at zero.
	probePixelTol = 4
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	skyR, skyG, skyB    = 0.10, 0.12, 0.18
	farR, farG, farB    = 0.20, 0.35, 0.55
	nearR, nearG, nearB = 0.25, 0.55, 0.35
	dotR, dotG, dotB    = 0.90, 0.15, 0.12
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	roadX, roadY, roadW, roadH = 16.0, 44.0, 600.0, 360.0
	infoX, infoY               = 632.0, 44.0
	noteY                      = 424.0

	offW, offH = 480, 270
)

// Live lens params mirror the frozen probe file (never the reverse).
const (
	liveSmoothing = 0.2
	liveDeadR     = 24.0
	waypointEvery = 2.0
)

func v2(p [2]float64) core.Vec2 { return core.V2(p[0], p[1]) }

func dist(a, b core.Vec2) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

type frozenCases struct {
	Follow struct {
		Start     [2]float64   `json:"start"`
		Smoothing float64      `json:"smoothing"`
		Target    [2]float64   `json:"target"`
		Steps     int          `json:"steps"`
		Want      [][2]float64 `json:"want"`
		MaxFrames int          `json:"max_frames"`
		MaxErr    float64      `json:"max_err"`
	} `json:"follow"`
	Limit struct {
		Rect   [4]float64 `json:"rect"`
		Target [2]float64 `json:"target"`
		Want   [2]float64 `json:"want"`
	} `json:"limit"`
	BadLimits []struct {
		Rect [4]float64 `json:"rect"`
	} `json:"bad_limits"`
	Seam struct {
		Factor [2]float64 `json:"factor"`
		Offset [2]float64 `json:"offset"`
		Mirror [2]float64 `json:"mirror"`
		Camera [2]float64 `json:"camera"`
		WorldA [2]float64 `json:"world_a"`
		WorldB [2]float64 `json:"world_b"`
		Tol    float64    `json:"tol"`
	} `json:"seam"`
	Parallax struct {
		World   [2]float64   `json:"world"`
		Camera  [2]float64   `json:"camera"`
		Depth   float64      `json:"depth"`
		Factors [][2]float64 `json:"factors"`
	} `json:"parallax"`
	Deadzone struct {
		Center  [2]float64 `json:"center"`
		Radius  float64    `json:"radius"`
		Inside  [2]float64 `json:"inside"`
		Outside [2]float64 `json:"outside"`
	} `json:"deadzone"`
	Dual struct {
		Target [2]float64 `json:"target"`
		Camera [2]float64 `json:"camera"`
	} `json:"dual"`
	Audio struct {
		Range          float64    `json:"range"`
		Attenuation    float64    `json:"attenuation"`
		PanStrength    float64    `json:"pan_strength"`
		CenterPos      [2]float64 `json:"center_pos"`
		RightPos       [2]float64 `json:"right_pos"`
		FarPos         [2]float64 `json:"far_pos"`
		WantCenterGain float64    `json:"want_center_gain"`
		WantCenterPan  float64    `json:"want_center_pan"`
		WantRightGain  float64    `json:"want_right_gain"`
		WantRightPan   float64    `json:"want_right_pan"`
		WantFarGain    float64    `json:"want_far_gain"`
		WantFarPan     float64    `json:"want_far_pan"`
		BusVolume      float64    `json:"bus_volume"`
		WantCombined   float64    `json:"want_combined"`
		MixerMax       int        `json:"mixer_max"`
		MixerGains     []float64  `json:"mixer_gains"`
		WantTotal      float64    `json:"want_total"`
		WantClipped    bool       `json:"want_clipped"`
		WantKept       int        `json:"want_kept"`
	} `json:"audio"`
}

func loadFrozen() (frozenCases, error) {
	var f frozenCases
	raw, err := os.ReadFile(frozenPath)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	return f, nil
}

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	FollowOK, LimitOK, SeamOK, ParOK bool
	DeadOK, DualOK, AudioOK, BusOK   bool
	ConvergeFrames                   int
	ConvergeErr                      float64
	PixOK                            bool
	PixDetail                        string
	GoldenOK                         bool
	GoldenChanged                    int
	GoldenWrote                      bool
	OK                               bool
	Detail                           string
}

// probeFollow replays the frozen follow table through the real engine:
// every step lands on the frozen number, and the lens sticks to the
// target within max_frames at max_err (0.5s at 60fps).
func probeFollow(f frozenCases) (bool, int, float64, string) {
	fl := f.Follow
	if fl.Steps <= 0 || len(fl.Want) < fl.Steps {
		return false, 0, 0, "frozen follow table short"
	}
	cam, err := camera.NewCamera(core.V2(800, 600))
	if err != nil {
		return false, 0, 0, "NewCamera: " + err.Error()
	}
	if err := cam.SetSmoothing(0); err != nil {
		return false, 0, 0, "snap: " + err.Error()
	}
	if err := cam.Follow(v2(fl.Start)); err != nil {
		return false, 0, 0, "start: " + err.Error()
	}
	if err := cam.SetSmoothing(fl.Smoothing); err != nil {
		return false, 0, 0, "smoothing: " + err.Error()
	}
	converged := 0
	var errLast float64
	tgt := v2(fl.Target)
	for i := 0; i < fl.Steps; i++ {
		if err := cam.Follow(tgt); err != nil {
			return false, 0, 0, fmt.Sprintf("step %d: %v", i, err)
		}
		got := cam.Pos()
		want := v2(fl.Want[i])
		if math.Abs(got.X-want.X) >= epsEngine || math.Abs(got.Y-want.Y) >= epsEngine {
			return false, 0, 0, fmt.Sprintf("step %d = %v, want %v", i, got, want)
		}
		errLast = dist(got, tgt)
		if converged == 0 && errLast <= fl.MaxErr {
			converged = i + 1
		}
	}
	if converged == 0 || converged > fl.MaxFrames {
		return false, converged, errLast,
			fmt.Sprintf("converge %d frames err %.4g, want <=%d/%.4g", converged, errLast, fl.MaxFrames, fl.MaxErr)
	}
	return true, converged, errLast,
		fmt.Sprintf("steps=%d converge=%d err=%.4g", fl.Steps, converged, errLast)
}

// probeLimit checks the clamp box pins the center, bad boxes never crash,
// and NaN boxes report invalid-arg without moving state.
func probeLimit(f frozenCases) (bool, string) {
	cam, err := camera.NewCamera(core.V2(800, 600))
	if err != nil {
		return false, "NewCamera: " + err.Error()
	}
	lim := core.NewRect(f.Limit.Rect[0], f.Limit.Rect[1], f.Limit.Rect[2], f.Limit.Rect[3])
	if err := cam.SetLimit(lim); err != nil {
		return false, "SetLimit: " + err.Error()
	}
	if err := cam.Follow(v2(f.Limit.Target)); err != nil {
		return false, "Follow: " + err.Error()
	}
	if got, want := cam.Pos(), v2(f.Limit.Want); got != want {
		return false, fmt.Sprintf("clamped = %v, want %v", got, want)
	}
	// Bad boxes stay empty (disabled): far positions stick, no crash.
	for i, b := range f.BadLimits {
		r := core.NewRect(b.Rect[0], b.Rect[1], b.Rect[2], b.Rect[3])
		if !r.IsEmpty() {
			return false, fmt.Sprintf("bad_limits[%d] = %v, want empty/disabled", i, r)
		}
		if err := cam.SetLimit(r); err != nil {
			return false, fmt.Sprintf("bad_limits[%d] SetLimit: %v", i, err)
		}
		far := core.V2(1e6, -1e6)
		if err := cam.SetSmoothing(0); err != nil {
			return false, fmt.Sprintf("bad_limits[%d] snap: %v", i, err)
		}
		if err := cam.Follow(far); err != nil {
			return false, fmt.Sprintf("bad_limits[%d] Follow: %v", i, err)
		}
		if cam.Pos() != far {
			return false, fmt.Sprintf("bad_limits[%d] pos = %v, want it kept", i, cam.Pos())
		}
	}
	// NaN limit is invalid-arg and moves nothing.
	before := cam.Pos()
	if err := cam.SetLimit(core.NewRect(math.NaN(), 0, 10, 10)); err == nil {
		return false, "NaN limit accepted"
	}
	if cam.Pos() != before {
		return false, "NaN limit moved state"
	}
	// Restore the good box for the verdict.
	if err := cam.SetLimit(lim); err != nil {
		return false, "restore: " + err.Error()
	}
	return true, fmt.Sprintf("clamp=%v bad=%d nan_kept=true", v2(f.Limit.Want), len(f.BadLimits))
}

// probeSeam checks the 10km mirror joint: two worlds exactly one period
// apart shift and project to the same screen within tol.
func probeSeam(f frozenCases) (bool, string) {
	s := f.Seam
	layer, err := camera.NewLayer(v2(s.Factor), v2(s.Offset), v2(s.Mirror))
	if err != nil {
		return false, "NewLayer: " + err.Error()
	}
	camPos, wa, wb := v2(s.Camera), v2(s.WorldA), v2(s.WorldB)
	sa, ok := layer.Shift(wa, camPos)
	if !ok {
		return false, "shift a ok=false"
	}
	sb, ok := layer.Shift(wb, camPos)
	if !ok {
		return false, "shift b ok=false"
	}
	if math.Abs(sa.X-sb.X) >= s.Tol || math.Abs(sa.Y-sb.Y) >= s.Tol {
		return false, fmt.Sprintf("seam shift %v vs %v, tol %v", sa, sb, s.Tol)
	}
	proj, err := camera.NewProjector(500, core.V2(400, 300), camPos)
	if err != nil {
		return false, "NewProjector: " + err.Error()
	}
	pa, _, ok := layer.Screen(wa, 0, camPos, proj)
	if !ok {
		return false, "screen a ok=false"
	}
	pb, _, ok := layer.Screen(wb, 0, camPos, proj)
	if !ok {
		return false, "screen b ok=false"
	}
	if math.Abs(pa.X-pb.X) >= s.Tol || math.Abs(pa.Y-pb.Y) >= s.Tol {
		return false, fmt.Sprintf("seam screen %v vs %v, tol %v", pa, pb, s.Tol)
	}
	return true, fmt.Sprintf("shift=%v screen=%v tol=%v", sa, pa, s.Tol)
}

// probeParallax checks far-slow-near-fast ratios: the half-speed band
// lands exactly halfway between world-locked and screen-pinned.
func probeParallax(f frozenCases) (bool, string) {
	p := f.Parallax
	if len(p.Factors) < 3 {
		return false, "frozen parallax factors short"
	}
	proj, err := camera.NewProjector(500, core.V2(400, 300), v2(p.Camera))
	if err != nil {
		return false, "NewProjector: " + err.Error()
	}
	screens := make([]core.Vec2, len(p.Factors))
	for i, fp := range p.Factors {
		layer, err := camera.NewLayer(v2(fp), core.Vec2{}, core.Vec2{})
		if err != nil {
			return false, fmt.Sprintf("NewLayer[%d]: %v", i, err)
		}
		s, _, ok := layer.Screen(v2(p.World), p.Depth, v2(p.Camera), proj)
		if !ok {
			return false, fmt.Sprintf("screen[%d] ok=false", i)
		}
		screens[i] = s
	}
	// factors[0]=1 locks to world (screen == center), factors[2]=0 pins
	// further out; factors[1]=0.5 must sit exactly halfway.
	mid := core.V2((screens[0].X+screens[2].X)/2, (screens[0].Y+screens[2].Y)/2)
	if math.Abs(screens[1].X-mid.X) >= epsEngine || math.Abs(screens[1].Y-mid.Y) >= epsEngine {
		return false, fmt.Sprintf("half band %v, want halfway %v", screens[1], mid)
	}
	return true, fmt.Sprintf("near=%v half=%v sky=%v", screens[0], screens[1], screens[2])
}

// probeDeadDual checks the engine dead zone plus the dual query split:
// inside the radius the lens holds (target and picture differ), outside
// it moves.
func probeDeadDual(f frozenCases) (bool, bool, string) {
	cam, err := camera.NewCamera(core.V2(800, 600))
	if err != nil {
		return false, false, "NewCamera: " + err.Error()
	}
	if err := cam.SetSmoothing(liveSmoothing); err != nil {
		return false, false, "smoothing: " + err.Error()
	}
	center := v2(f.Deadzone.Center)
	if err := cam.SetSmoothing(0); err != nil {
		return false, false, "snap: " + err.Error()
	}
	if err := cam.Follow(center); err != nil {
		return false, false, "center: " + err.Error()
	}
	if err := cam.SetSmoothing(liveSmoothing); err != nil {
		return false, false, "restore: " + err.Error()
	}
	movedIn, err := cam.FollowDeadzone(v2(f.Deadzone.Inside), f.Deadzone.Radius)
	if err != nil {
		return false, false, "inside: " + err.Error()
	}
	deadOK := !movedIn && cam.DualPicture() == center
	// Dual query: the target sits inside while the picture holds center.
	tgt, pic, ok := cam.DualTarget(v2(f.Dual.Target))
	if !ok {
		return false, false, "dual: ok=false"
	}
	dualOK := tgt != pic && pic == center
	movedOut, err := cam.FollowDeadzone(v2(f.Deadzone.Outside), f.Deadzone.Radius)
	if err != nil {
		return false, false, "outside: " + err.Error()
	}
	deadOK = deadOK && movedOut && cam.DualPicture() != center
	return deadOK, dualOK, fmt.Sprintf("inside_hold=%v outside_moved=%v target=%v picture=%v",
		!movedIn, movedOut, tgt, pic)
}

// probeAudio checks the S77 rider through the real package: the listener
// follows the camera, center sits centered, right leans right, far is
// faint but placed, and bus plus mixer cover the attenuation chain.
func probeAudio(f frozenCases) (bool, bool, string) {
	a := f.Audio
	lis, err := audio.NewListener2D(core.V2(0, 0))
	if err != nil {
		return false, false, "NewListener2D: " + err.Error()
	}
	// Listener glues to the camera center each frame.
	if err := lis.FollowCamera(core.V2(0, 0)); err != nil {
		return false, false, "FollowCamera: " + err.Error()
	}
	lis.MakeCurrent()
	if !lis.IsCurrent() {
		return false, false, "not current after MakeCurrent"
	}
	cur, ok := audio.CurrentListener()
	if !ok || cur.Pos() != lis.Pos() {
		return false, false, "CurrentListener mismatch"
	}
	mk := func(p [2]float64) (audio.PosSound, error) {
		return audio.NewPosSound(v2(p), a.Range, a.Attenuation, a.PanStrength)
	}
	cs, err := mk(a.CenterPos)
	if err != nil {
		return false, false, "center build: " + err.Error()
	}
	rs, err := mk(a.RightPos)
	if err != nil {
		return false, false, "right build: " + err.Error()
	}
	fs, err := mk(a.FarPos)
	if err != nil {
		return false, false, "far build: " + err.Error()
	}
	cm, ok := lis.MixSource(cs)
	if !ok || !cm.Audible || cm.Gain != a.WantCenterGain || cm.Pan != a.WantCenterPan {
		return false, false, fmt.Sprintf("center = %+v, want gain %v pan %v", cm, a.WantCenterGain, a.WantCenterPan)
	}
	rm, ok := lis.MixSource(rs)
	if !ok || !rm.Audible {
		return false, false, fmt.Sprintf("right = %+v, want audible", rm)
	}
	if math.Abs(rm.Gain-a.WantRightGain) >= epsEngine || math.Abs(rm.Pan-a.WantRightPan) >= epsEngine {
		return false, false, fmt.Sprintf("right = %+v, want gain %v pan %v", rm, a.WantRightGain, a.WantRightPan)
	}
	fm, ok := lis.MixSource(fs)
	if !ok || !fm.Audible {
		return false, false, fmt.Sprintf("far = %+v, want faint audible", fm)
	}
	if math.Abs(fm.Gain-a.WantFarGain) >= epsEngine || math.Abs(fm.Pan-a.WantFarPan) >= epsEngine {
		return false, false, fmt.Sprintf("far = %+v, want gain %v pan %v", fm, a.WantFarGain, a.WantFarPan)
	}
	if !(cm.Gain > rm.Gain && rm.Gain > fm.Gain && rm.Pan > 0 && fm.Pan > rm.Pan) {
		return false, false, "gain/pan order wrong"
	}
	l, r := audio.Stereo(0.5, rm)
	if !(r > l && l > 0) {
		return false, false, fmt.Sprintf("stereo right l=%v r=%v, want r>l>0", l, r)
	}
	lc, rc := audio.Stereo(0.5, cm)
	if lc != rc {
		return false, false, "stereo center not centered"
	}
	// Singleton steal plus clear.
	other, err := audio.NewListener2D(core.V2(500, 0))
	if err != nil {
		return false, false, "other: " + err.Error()
	}
	other.MakeCurrent()
	if lis.IsCurrent() || !other.IsCurrent() {
		return false, false, "steal failed"
	}
	other.ClearCurrent()
	if _, ok := audio.CurrentListener(); ok {
		return false, false, "clear failed"
	}
	lis.MakeCurrent()
	all, ok := audio.MixCurrent([]audio.PosSound{cs, rs, fs})
	if !ok || len(all) != 3 || !all[0].Audible || !all[2].Audible {
		return false, false, fmt.Sprintf("MixCurrent = %v/%v", all, ok)
	}
	// Bus plus mixer cover the attenuation chain.
	bus, err := audio.NewBus("master", a.BusVolume)
	if err != nil {
		return false, false, "NewBus: " + err.Error()
	}
	if math.Abs(bus.Gain()-a.BusVolume) >= epsEngine {
		return false, false, fmt.Sprintf("bus gain = %v, want %v", bus.Gain(), a.BusVolume)
	}
	if got := audio.CombineGains(0.5, bus.Gain(), 1); math.Abs(got-a.WantCombined) >= epsEngine {
		return false, false, fmt.Sprintf("combined = %v, want %v", got, a.WantCombined)
	}
	mixer, err := audio.NewMixer(a.MixerMax)
	if err != nil {
		return false, false, "NewMixer: " + err.Error()
	}
	res := mixer.Mix(a.MixerGains)
	busOK := res.Total == a.WantTotal && res.Clipped == a.WantClipped &&
		res.Kept == a.WantKept && res.Audible == len(a.MixerGains)
	if !busOK {
		return false, false, fmt.Sprintf("mixer = %+v", res)
	}
	return true, true, fmt.Sprintf("center=%.2f/%.2f right=%.2f/%.2f far=%.2f/%.2f bus=%.2f",
		cm.Gain, cm.Pan, rm.Gain, rm.Pan, fm.Gain, fm.Pan, bus.Gain())
}

func sample8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func closeEnough(got, want uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= probePixelTol
}

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

// paintProbeFrame draws the deterministic probe scene: sky, a far band,
// a near band, and the target dot pinned inside the near band.
func paintProbeFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: skyR, G: skyG, B: skyB, A: 1})
	dc.SetRGB(farR, farG, farB)
	dc.DrawRectangle(20, 60, 440, 70)
	_ = dc.Fill()
	dc.SetRGB(nearR, nearG, nearB)
	dc.DrawRectangle(20, 140, 440, 70)
	_ = dc.Fill()
	dc.SetRGB(dotR, dotG, dotB)
	dc.DrawRectangle(300, 160, 24, 24)
	_ = dc.Fill()
}

// probePixels asserts sky/far/near/target colors offscreen.
func probePixels() (bool, string) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintProbeFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	sr, sg, sb := sample8(img, 10, 10)
	fr, fg, fb := sample8(img, 240, 95)
	nr, ng, nb := sample8(img, 100, 175)
	tr, tg, tb := sample8(img, 312, 172)
	ok := closeEnough(sr, want8(skyR)) && closeEnough(sg, want8(skyG)) && closeEnough(sb, want8(skyB)) &&
		closeEnough(fr, want8(farR)) && closeEnough(fg, want8(farG)) && closeEnough(fb, want8(farB)) &&
		closeEnough(nr, want8(nearR)) && closeEnough(ng, want8(nearG)) && closeEnough(nb, want8(nearB)) &&
		closeEnough(tr, want8(dotR)) && closeEnough(tg, want8(dotG)) && closeEnough(tb, want8(dotB))
	detail := fmt.Sprintf("sky=(%d,%d,%d) far=(%d,%d,%d) near=(%d,%d,%d) dot=(%d,%d,%d) tol=%d",
		sr, sg, sb, fr, fg, fb, nr, ng, nb, tr, tg, tb, probePixelTol)
	return ok, detail
}

// probeGolden compares the deterministic frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
func probeGolden() (ok bool, changed int, wrote bool) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintProbeFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/camera/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenPath)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		return encErr == nil, 0, encErr == nil
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	if !img.Bounds().Eq(want.Bounds()) {
		return false, 1, false
	}
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			ar, ag, ab, aa := img.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	return changed == 0, changed, false
}

// runProbes collects the three evidences: logic, pixels, golden mask.
func runProbes() probeResult {
	var p probeResult
	f, err := loadFrozen()
	if err != nil {
		p.Detail = "frozen read: " + err.Error()
		return p
	}
	var detail string
	p.FollowOK, p.ConvergeFrames, p.ConvergeErr, detail = probeFollow(f)
	_ = detail
	p.LimitOK, detail = probeLimit(f)
	_ = detail
	p.SeamOK, detail = probeSeam(f)
	_ = detail
	p.ParOK, detail = probeParallax(f)
	_ = detail
	p.DeadOK, p.DualOK, detail = probeDeadDual(f)
	_ = detail
	p.AudioOK, p.BusOK, detail = probeAudio(f)
	_ = detail
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.FollowOK && p.LimitOK && p.SeamOK && p.ParOK && p.DeadOK &&
		p.DualOK && p.AudioOK && p.BusOK && p.PixOK && p.GoldenOK
	p.Detail = fmt.Sprintf("follow=%v(%d/%.3g) limit=%v seam=%v par=%v dead=%v dual=%v audio=%v bus=%v",
		p.FollowOK, p.ConvergeFrames, p.ConvergeErr, p.LimitOK, p.SeamOK,
		p.ParOK, p.DeadOK, p.DualOK, p.AudioOK, p.BusOK)
	return p
}

// camSim is the live window state: a real camera chases waypoints with a
// dead zone, parallax bands drift with real layers, and one listener
// rides the camera while three fixed sounds mix through it. Full-window
// content: road fills the window, one overlay line floats at top-left.
type camSim struct {
	cam       camera.Camera
	target    core.Vec2
	waypoints []core.Vec2
	wpIndex   int
	wpClock   float64
	clock     float64
	layers    []camera.Layer
	listener  audio.Listener2D
	sounds    []audio.PosSound
	bus       *audio.Bus
	mixer     *audio.Mixer
	app       *embedder.PipelineApp
	root      *rendering.AbsoluteBox
	phase     *wrkit.PhaseClock
	road      *rendering.RenderBox
	car       *rendering.RenderColorBox
	// Thick content: foreground occluder posts + parallax far mountain.
	occlX     []float64
	frames    int
	overlay   *rendering.RenderText
	camTravel float64
	moves     int
	holds     int
	lastPos   core.Vec2
}

// TargetPos is where the target is (dual query, picture side excluded).
func (s *camSim) TargetPos() core.Vec2 { return s.target }

// CameraPos is where the picture is (dual query, target side excluded).
func (s *camSim) CameraPos() core.Vec2 {
	if s == nil {
		return core.Vec2{}
	}
	return s.cam.DualPicture()
}

type ticker struct{ s *camSim }

func (t *ticker) Tick(dt float64) bool {
	s := t.s
	if s == nil {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	s.frames++
	s.clock += dt
	s.wpClock += dt
	if s.wpClock >= waypointEvery && len(s.waypoints) > 0 {
		s.wpClock = 0
		s.wpIndex = (s.wpIndex + 1) % len(s.waypoints)
		s.target = s.waypoints[s.wpIndex]
	}
	moved, _ := s.cam.FollowDeadzone(s.target, liveDeadR)
	if moved {
		s.moves++
	} else {
		s.holds++
	}
	_ = s.cam.DecayShake(0.05)
	pos := s.cam.Pos()
	s.camTravel += dist(pos, s.lastPos)
	s.lastPos = pos
	// S77: the hear point rides the effective center every frame.
	_ = s.listener.FollowCamera(s.cam.EffectivePos())
	if scr, ok := s.cam.WorldToScreen(s.target); ok {
		// Car is road-local at full-window size: road fills the window.
		s.car.MoveTo(scr.X-12, scr.Y-12)
	}
	// Thick content: foreground occluder posts drift with the lens.
	for i := range s.occlX {
		s.occlX[i] -= 60 * dt
		if s.occlX[i] < -40 {
			s.occlX[i] = float64(winW + 40)
		}
	}
	s.road.MarkNeedsPaint()
	// Single floating overlay line at top-left over the picture.
	mixes, _ := audio.MixCurrent(s.sounds)
	gains := make([]float64, 0, len(mixes))
	txt := "听 "
	for i, m := range mixes {
		if i > 0 {
			txt += " "
		}
		txt += fmt.Sprintf("%.2f/%.2f", m.Gain, m.Pan)
		gains = append(gains, audio.CombineGains(m.Gain, s.bus.Gain(), 1))
	}
	held := s.mixer.Mix(gains)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	if s.overlay != nil {
		s.overlay.SetText(fmt.Sprintf("fps %.0f tgt %.0f,%.0f cam %.0f,%.0f %s 合%.2f留%d mv%d",
			fps, s.target.X, s.target.Y, pos.X, pos.Y, txt, held.Total, held.Kept, s.moves))
	}
	_ = s.phase.Advance(dt)
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func failJSON(probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func main() {
	flag.String("case", "follow", "scenario case (follow lens road, or voices S89 large-voice reuse)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if flag.Lookup("case").Value.String() == "voices" {
		runVoicesCase(*autoOnly, *manualSeconds)
		return
	}
	if flag.Lookup("case").Value.String() != "follow" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want follow or voices (one ability per case, no combo)\n", flag.Lookup("case").Value.String())
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_camera: probes ok=%v %s pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.Detail, probe.PixOK, probe.GoldenOK,
		probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_camera: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(8)
		wrkit.RequireMinRun(secs, abilityID)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !*autoOnly

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	root.Background = &rendering.Color{R: skyR, G: skyG, B: skyB, A: 1}

	// Full-window lens: camera viewport matches the window.
	cam, err := camera.NewCamera(core.V2(float64(winW), float64(winH)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: NewCamera:", err)
		os.Exit(1)
	}
	_ = cam.SetSmoothing(liveSmoothing)
	_ = cam.SetLimit(core.NewRect(100, 100, 400, 300))
	_ = cam.SetSmoothing(0)
	_ = cam.Follow(core.V2(300, 250))
	_ = cam.SetSmoothing(liveSmoothing)

	factors := [][2]float64{{1, 1}, {0.5, 0.5}, {0.25, 0.4}, {0, 0}}
	mirrors := [][2]float64{{0, 0}, {0, 0}, {400, 300}, {0, 0}}
	layers := make([]camera.Layer, 0, len(factors))
	for i := range factors {
		ly, err := camera.NewLayer(v2(factors[i]), core.Vec2{}, v2(mirrors[i]))
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: NewLayer:", err)
			os.Exit(1)
		}
		layers = append(layers, ly)
	}
	lis, err := audio.NewListener2D(cam.EffectivePos())
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: NewListener2D:", err)
		os.Exit(1)
	}
	lis.MakeCurrent()
	sounds := []audio.PosSound{}
	for _, p := range [][2]float64{{300, 250}, {400, 250}, {490, 250}} {
		s, err := audio.NewPosSound(v2(p), 200, 1, 1)
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: NewPosSound:", err)
			os.Exit(1)
		}
		sounds = append(sounds, s)
	}
	bus, err := audio.NewBus("master", 0.8)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: NewBus:", err)
		os.Exit(1)
	}
	mixer, err := audio.NewMixer(32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: NewMixer:", err)
		os.Exit(1)
	}

	sim := &camSim{
		cam:       cam,
		target:    core.V2(300, 250),
		waypoints: []core.Vec2{core.V2(150, 150), core.V2(450, 150), core.V2(450, 300), core.V2(150, 300)},
		layers:    layers,
		listener:  lis,
		sounds:    sounds,
		bus:       bus,
		mixer:     mixer,
		root:      root,
		occlX:     []float64{200, 600, 1000},
		lastPos:   cam.Pos(),
	}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	// Full-window content: road fills the window (no partitions).
	// Thick: follow + shake + parallax far mountain + foreground occluders
	// + limit frame + audio mixes all in the same picture.
	rw, rh := float64(winW), float64(winH)
	sim.road = rendering.NewRenderBox()
	sim.road.FixedWidth, sim.road.FixedHeight = rw, rh
	sim.road.SetRepaintBoundary(true)
	sim.road.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGBA(skyR, skyG, skyB, 1)
		pc.DC.DrawRectangle(ax, ay, rw, rh)
		_ = pc.DC.Fill()
		// Parallax stripes: real Layer.Shift mapped through the projector
		// so far bands visibly lag the lens.
		proj, err := camera.NewProjector(500, core.V2(rw/2, rh/2), sim.cam.EffectivePos())
		if err != nil {
			return
		}
		// Far mountain ridge (slowest layer) across the full width.
		for k := 0; k < 8; k++ {
			w := core.V2(float64(k*220), 60)
			sh, ok := sim.layers[3].Shift(w, sim.cam.EffectivePos())
			if !ok {
				continue
			}
			sp, _, ok := proj.Project(sh, 0)
			if !ok {
				continue
			}
			pc.DC.SetRGBA(farR, farG, farB, 1)
			pc.DC.DrawRectangle(ax+sp.X-40, ay+sp.Y-30, 80, 60)
			_ = pc.DC.Fill()
		}
		bandCols := [][3]float64{{farR, farG, farB}, {nearR, nearG, nearB}}
		for bi := 0; bi < 2; bi++ {
			for k := 0; k < 6; k++ {
				w := core.V2(float64(k*220), 120+float64(bi*110))
				sh, ok := sim.layers[bi+1].Shift(w, sim.cam.EffectivePos())
				if !ok {
					continue
				}
				sp, _, ok := proj.Project(sh, 0)
				if !ok {
					continue
				}
				px, py := ax+sp.X-30, ay+sp.Y-14
				if px < ax-60 || px > ax+rw+60 || py < ay-40 || py > ay+rh+40 {
					continue
				}
				pc.DC.SetRGBA(bandCols[bi][0], bandCols[bi][1], bandCols[bi][2], 1)
				pc.DC.DrawRectangle(px, py, 60, 28)
				_ = pc.DC.Fill()
			}
		}
		// Limit frame corners through the real lens.
		lim := sim.cam.Limit()
		if !lim.IsEmpty() {
			corners := [4]core.Vec2{
				{X: lim.X, Y: lim.Y},
				{X: lim.X + lim.W, Y: lim.Y},
				{X: lim.X + lim.W, Y: lim.Y + lim.H},
				{X: lim.X, Y: lim.Y + lim.H},
			}
			pc.DC.SetRGBA(1, 0.9, 0.2, 1)
			pc.DC.SetLineWidth(2)
			var prevX, prevY float64
			var havePrev bool
			for i := 0; i < 5; i++ {
				c := corners[i%4]
				sp, ok := sim.cam.WorldToScreen(c)
				if !ok {
					havePrev = false
					continue
				}
				if havePrev {
					pc.DC.DrawLine(ax+prevX, ay+prevY, ax+sp.X, ay+sp.Y)
					_ = pc.DC.Stroke()
				}
				prevX, prevY, havePrev = sp.X, sp.Y, true
			}
		}
		// Foreground occluder posts: same height as the window, drawn over
		// everything to prove foreground occlusion in one picture.
		for _, ox := range sim.occlX {
			pc.DC.SetRGBA(0.05, 0.05, 0.07, 0.85)
			pc.DC.DrawRectangle(ax+ox, ay, 36, rh)
			_ = pc.DC.Fill()
		}
	}
	root.Place(sim.road, 0, 0)
	sim.car = rendering.NewRenderColorBox(24, 24, dotR, dotG, dotB, 1)
	if scr, ok := sim.cam.WorldToScreen(sim.target); ok {
		root.Place(sim.car, scr.X-12, scr.Y-12)
	} else {
		root.Place(sim.car, rw/2-12, rh/2-12)
	}

	// One-line floating overlay at top-left over the picture (no own band).
	sim.overlay = wrkit.Label("--", 13, 1, 1, 1)
	root.Place(sim.overlay, 12, 10)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_camera", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: skyR, ClearG: skyG, ClearB: skyB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_camera: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				// Drag the target toward the pressed point (full window).
				if w, ok := sim.cam.ScreenToWorld(core.V2(ev.X, ev.Y)); ok {
					sim.target = w
					sim.wpClock = 0
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_camera: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_camera events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					const step = 40.0
					switch ev.Rune {
					case 'a', 'A':
						sim.target.X -= step
						sim.wpClock = 0
					case 'd', 'D':
						sim.target.X += step
						sim.wpClock = 0
					case 'w', 'W':
						sim.target.Y -= step
						sim.wpClock = 0
					case 's', 'S':
						sim.target.Y += step
						sim.wpClock = 0
					case ' ':
						_ = sim.cam.SetShake(core.V2(6, 4))
					}
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_camera: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_camera events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsLayout()
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_camera: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_camera events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&ticker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	convOK := 0
	if probe.ConvergeFrames > 0 && probe.ConvergeFrames <= 30 {
		convOK = 1
	}
	extra := map[string]any{
		"case":            "follow",
		"probe_ok":        probeOK,
		"follow_ok":       probe.FollowOK,
		"converge_frames": probe.ConvergeFrames,
		"converge_err":    probe.ConvergeErr,
		"converge_ok":     convOK,
		"limit_ok":        probe.LimitOK,
		"seam_ok":         probe.SeamOK,
		"parallax_ok":     probe.ParOK,
		"dead_ok":         probe.DeadOK,
		"dual_ok":         probe.DualOK,
		"audio_ok":        probe.AudioOK,
		"bus_ok":          probe.BusOK,
		"cam_travel_px":   sim.camTravel,
		"moves":           sim.moves,
		"holds":           sim.holds,
		"frames":          sim.frames,
		"boundary_skip":   snap.BoundarySkip,
		"pixels":          probe.PixDetail,
		"golden":          probe.GoldenChanged,
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenario,
			Snap:          snap,
			PresentCount:  presents,
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 || !probe.OK || convOK != 1 || sim.camTravel <= 0 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v converge=%d(%.3g) travel=%.1f (want >=1, true, <=30, >0)\n",
				presents, probe.OK, probe.ConvergeFrames, probe.ConvergeErr, sim.camTravel)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_camera: OK presents=%d converge=%d err=%.3g travel=%.0f moves=%d holds=%d elapsed=%.1fs\n",
			presents, probe.ConvergeFrames, probe.ConvergeErr, sim.camTravel, sim.moves, sim.holds, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":        presents,
		"elapsed_sec":     elapsed,
		"cam_travel_px":   sim.camTravel,
		"converge_frames": probe.ConvergeFrames,
		"probe_ok":        probeOK,
		"timed":           summary.Timed,
		"note":            summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_camera: backend=%s presents=%d travel=%.0f converge=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.camTravel, probe.ConvergeFrames, elapsed)
}

// ---- --case=voices: S89 large-voice reuse (listener window reuse) ----
//
// Reads engine/audio/testdata/large_voices.json (engine-owned numbers).
// Offscreen paints the frozen mix map; the live window replays the same
// 128 asks through SelectVoices plus listener-follow-camera plus the
// explosion duck plus the beat clock. 2.5D摆法：满窗即内容，指标浮左上。

const (
	voicesAbilityID = "audio-voices"
	voicesScenario  = "game_camera--case=voices"
	voicesEngineDir = "engine/audio/testdata"
	voicesEngineFn  = "large_voices.json"

	voicesOffW, voicesOffH = 640, 360
	voicesPerfMaxUs        = 500 // 128 select+128 stereo under 0.5ms
	voicesBarX             = 40.0
	voicesBarY             = 60.0
	voicesBarW             = 560.0
	voicesBarH             = 18.0
)

type voicesReqJSON struct {
	Pos         [2]float64 `json:"pos"`
	Range       float64    `json:"range"`
	Attenuation float64    `json:"attenuation"`
	PanStrength float64    `json:"pan_strength"`
	Level       float64    `json:"level"`
	Explosive   bool       `json:"explosive"`
}

type voicesWantJSON struct {
	Channels       int       `json:"channels"`
	Audible        int       `json:"audible"`
	Total          float64   `json:"total"`
	Clipped        bool      `json:"clipped"`
	HasExplosion   bool      `json:"has_explosion"`
	AudibleIndices []int     `json:"audible_indices"`
	Gains          []float64 `json:"gains"`
	Pans           []float64 `json:"pans"`
	Audibles       []bool    `json:"audibles"`
}

type voicesBeatJSON struct {
	BPM           float64 `json:"bpm"`
	BeatsPerBar   int     `json:"beats_per_bar"`
	DtsMs         []int64 `json:"dts_ms"`
	WantBeats     int64   `json:"want_beats"`
	WantPosMs     int64   `json:"want_pos_ms"`
	WantTimeBeat4 int64   `json:"want_time_beat4_ms"`
}

type voicesFile struct {
	Listener [2]float64      `json:"listener"`
	Camera   [2]float64      `json:"camera"`
	Requests []voicesReqJSON `json:"requests"`
	Want     voicesWantJSON  `json:"want"`
	Beat     voicesBeatJSON  `json:"beat"`
}

func loadVoicesFile() (voicesFile, error) {
	var f voicesFile
	raw, err := os.ReadFile(voicesEngineDir + "/" + voicesEngineFn)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	if len(f.Requests) == 0 {
		return f, fmt.Errorf("voices file has no requests")
	}
	return f, nil
}

func buildVoiceReqs(f voicesFile) ([]audio.VoiceRequest, error) {
	out := make([]audio.VoiceRequest, len(f.Requests))
	for i, q := range f.Requests {
		s, err := audio.NewPosSound(v2(q.Pos), q.Range, q.Attenuation, q.PanStrength)
		if err != nil {
			return nil, fmt.Errorf("request %d sound: %w", i, err)
		}
		vr, err := audio.NewVoiceRequest(s, q.Level, q.Explosive)
		if err != nil {
			return nil, fmt.Errorf("request %d level: %w", i, err)
		}
		out[i] = vr
	}
	return out, nil
}

func closeVoice(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func voicesSelftest(f voicesFile) (map[string]any, bool) {
	extra := map[string]any{}
	ok := true
	fail := func(k string, v any) {
		extra[k] = v
		ok = false
	}
	reqs, err := buildVoiceReqs(f)
	if err != nil {
		fail("build_err", err.Error())
		return extra, false
	}
	lis := v2(f.Listener)
	picks, st, okSel := audio.SelectVoices(lis, reqs)
	if !okSel {
		fail("select_ok", false)
		return extra, false
	}
	extra["voices_channels"] = st.Channels
	extra["voices_audible"] = st.Audible
	extra["voices_total"] = st.Total
	extra["voices_clipped"] = st.Clipped
	extra["voices_boom"] = st.HasExplosion
	if st.Channels != f.Want.Channels || st.Audible != f.Want.Audible {
		fail("voices_counts", []int{st.Channels, st.Audible})
	} else {
		extra["voices_counts_ok"] = true
	}
	if st.Channels > audio.MaxVoiceChannels || st.Audible > audio.AudibleVoices {
		fail("voices_budget", []int{st.Channels, st.Audible})
	}
	if !closeVoice(st.Total, f.Want.Total) || st.Clipped != f.Want.Clipped || st.HasExplosion != f.Want.HasExplosion {
		fail("voices_mix", []any{st.Total, st.Clipped, st.HasExplosion})
	} else {
		extra["voices_mix_ok"] = true
	}
	if len(picks) != len(reqs) {
		fail("picks_len", len(picks))
		return extra, false
	}
	audSet := map[int]bool{}
	for _, idx := range f.Want.AudibleIndices {
		audSet[idx] = true
	}
	pickOK := true
	for i, p := range picks {
		if p.Index != i {
			pickOK = false
			break
		}
		wantAud := audSet[i]
		if p.Audible != wantAud || !closeVoice(p.Gain, f.Want.Gains[i]) ||
			!closeVoice(p.Pan, f.Want.Pans[i]) || p.Audible != f.Want.Audibles[i] {
			pickOK = false
			break
		}
	}
	extra["picks_ok"] = pickOK
	if !pickOK {
		ok = false
	}
	// Loudest-kept: every audible pick outranks every silent ranked one.
	loudOK := true
	minAud := math.MaxFloat64
	maxSil := 0.0
	for _, p := range picks {
		if p.Audible {
			if p.Loud < minAud {
				minAud = p.Loud
			}
		} else if p.Loud > maxSil {
			maxSil = p.Loud
		}
	}
	if minAud < maxSil {
		loudOK = false
	}
	extra["loudest_ok"] = loudOK
	if !loudOK {
		ok = false
	}
	// Replay twice bitwise.
	picks2, st2, _ := audio.SelectVoices(lis, reqs)
	replayOK := st2 == st && len(picks2) == len(picks)
	if replayOK {
		for i := range picks {
			if picks2[i] != picks[i] {
				replayOK = false
				break
			}
		}
	}
	extra["parity_ok"] = replayOK
	if !replayOK {
		ok = false
	}
	// Listener-follow-camera: same asks through the rider agree.
	l, err := audio.NewListener2D(v2(f.Listener))
	if err != nil {
		fail("listener_err", err.Error())
		return extra, false
	}
	rp, rst, rok := audio.FollowAndSelect(&l, v2(f.Camera), reqs)
	followOK := rok && rst == st && len(rp) == len(picks)
	if followOK {
		for i := range picks {
			if rp[i] != picks[i] {
				followOK = false
				break
			}
		}
	}
	extra["follow_ok"] = followOK
	if !followOK {
		ok = false
	}
	// Explosion ducks music: trigger only when audible boom present.
	duck, err := audio.NewDucker(audio.DefaultDuckDepth, 250*core.Millisecond, 250*core.Millisecond)
	if err != nil {
		fail("duck_err", err.Error())
		return extra, false
	}
	before := duck.Gain()
	did, err := audio.ApplyExplosionDuck(duck, st, 1)
	duckOK := err == nil && did == st.HasExplosion && (before == 1 || !did)
	if did {
		duckOK = duckOK && duck.Active() && duck.Gain() < 1
	}
	quiet := st
	quiet.HasExplosion = false
	didQ, errQ := audio.ApplyExplosionDuck(duck, quiet, 1)
	duckOK = duckOK && errQ == nil && !didQ
	extra["duck_ok"] = duckOK
	if !duckOK {
		ok = false
	}
	// Beat clock: frozen dts replay to the frozen beat/pos.
	bc, err := audio.NewBeatClock(f.Beat.BPM, f.Beat.BeatsPerBar)
	if err != nil {
		fail("beat_err", err.Error())
		return extra, false
	}
	var crossed int64
	for _, d := range f.Beat.DtsMs {
		crossed += int64(len(bc.Update(core.Duration(d))))
	}
	t4, err := bc.TimeOfBeat(4)
	beatOK := err == nil && bc.Beat() == f.Beat.WantBeats && bc.Pos().Milliseconds() == f.Beat.WantPosMs &&
		t4.Milliseconds() == f.Beat.WantTimeBeat4 && crossed == f.Beat.WantBeats
	extra["beat"] = bc.Beat()
	extra["beat_pos_ms"] = bc.Pos().Milliseconds()
	extra["beat_ok"] = beatOK
	if !beatOK {
		ok = false
	}
	// Perf: 128 select + 128 stereo under 0.5ms.
	t0 := time.Now()
	for k := 0; k < 20; k++ {
		ps, _, _ := audio.SelectVoices(lis, reqs)
		for _, p := range ps {
			_, _ = audio.Stereo(0.5, audio.Mix{Gain: p.Gain, Pan: p.Pan, Distance: 0, Audible: p.Audible})
		}
	}
	perUs := float64(time.Since(t0).Microseconds()) / 20
	extra["voices_batch_us"] = perUs
	extra["voices_perf_ok"] = perUs <= voicesPerfMaxUs
	if perUs > voicesPerfMaxUs {
		ok = false
	}
	// Waveform guard: full-scale stereo never clips per channel.
	waveOK := true
	for _, p := range picks {
		l, r := audio.Stereo(1, audio.Mix{Gain: p.Gain, Pan: p.Pan, Distance: 0, Audible: p.Audible})
		if math.Abs(l) > 1 || math.Abs(r) > 1 || math.IsNaN(l) || math.IsNaN(r) {
			waveOK = false
			break
		}
	}
	extra["wave_ok"] = waveOK
	if !waveOK {
		ok = false
	}
	extra["probe_ok"] = ok
	return extra, ok
}

func paintVoicesOffscreen(dc *render.Context, f voicesFile, picks []audio.VoicePick) {
	dc.ClearWithColor(render.RGBA{R: 0.08, G: 0.09, B: 0.11, A: 1})
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(20, 20, voicesOffW-40, voicesOffH-40)
	_ = dc.Fill()
	// Map [-600,600]x[-400,400] into the field. Paint order keeps probes
	// readable: silent dots first, audible green cells next, explosive
	// red booms last so the center boom is never buried.
	silent := func(q voicesReqJSON, p audio.VoicePick) bool { return !q.Explosive && !p.Audible }
	audible := func(q voicesReqJSON, p audio.VoicePick) bool { return !q.Explosive && p.Audible }
	boom := func(q voicesReqJSON, p audio.VoicePick) bool { return q.Explosive }
	_ = boom
	dot := func(i int) {
		q := f.Requests[i]
		p := picks[i]
		px := 20 + (q.Pos[0]+600)/1200*(voicesOffW-40)
		py := 20 + (q.Pos[1]+400)/800*(voicesOffH-40)
		if q.Explosive {
			dc.SetRGB(0.95, 0.3, 0.2)
			sz := 3.0 + 4*p.Gain
			dc.DrawRectangle(px-sz/2, py-sz/2, sz, sz)
			_ = dc.Fill()
			return
		}
		if p.Audible {
			dc.SetRGB(0.2, 0.65, 0.3)
			dc.DrawRectangle(px-4, py-4, 8, 8)
			_ = dc.Fill()
			return
		}
		dc.SetRGB(0.55, 0.58, 0.62)
		dc.DrawRectangle(px-1.5, py-1.5, 3, 3)
		_ = dc.Fill()
	}
	for i := range f.Requests {
		if silent(f.Requests[i], picks[i]) {
			dot(i)
		}
	}
	for i := range f.Requests {
		if audible(f.Requests[i], picks[i]) {
			dot(i)
		}
	}
	for i := range f.Requests {
		if f.Requests[i].Explosive {
			dot(i)
		}
	}
	// Listener cross at center.
	lx := 20 + (f.Listener[0]+600)/1200*(voicesOffW-40)
	ly := 20 + (f.Listener[1]+400)/800*(voicesOffH-40)
	dc.SetRGB(0.9, 0.15, 0.12)
	dc.DrawRectangle(lx-5, ly-1, 10, 2)
	_ = dc.Fill()
	dc.DrawRectangle(lx-1, ly-5, 2, 10)
	_ = dc.Fill()
}

func voicesRenderOffscreen(f voicesFile, picks []audio.VoicePick) image.Image {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}()
	dc := render.NewContext(voicesOffW, voicesOffH)
	defer dc.Close()
	paintVoicesOffscreen(dc, f, picks)
	raw := dc.Image()
	cp := image.NewRGBA(raw.Bounds())
	if rgba, isRGBA := raw.(*image.RGBA); isRGBA {
		copy(cp.Pix, rgba.Pix)
		return cp
	}
	for y := 0; y < voicesOffH; y++ {
		for x := 0; x < voicesOffW; x++ {
			cp.Set(x, y, raw.At(x, y))
		}
	}
	return cp
}

func runVoicesPixelProbes(img image.Image, f voicesFile, picks []audio.VoicePick) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	at := func(x, y float64) (uint8, uint8, uint8, bool) {
		px := 20 + (x+600)/1200*(voicesOffW-40)
		py := 20 + (y+400)/800*(voicesOffH-40)
		r32, g32, b32, _ := img.At(int(px+0.5), int(py+0.5)).RGBA()
		return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8), true
	}
	// Center explosive (request 0) reads red. Probe inside its own dot:
	// +4 world units right stays on the boom while leaving the listener
	// cross and the neighbor green cell behind.
	r, g, b, _ := at(f.Requests[0].Pos[0]+4, f.Requests[0].Pos[1])
	boomOK := r >= 180 && g <= 140 && b <= 140
	out["boom"] = []int{int(r), int(g), int(b)}
	out["boom_ok"] = boomOK
	if !boomOK {
		ok = false
	}
	// An audible non-explosive request reads green. Skip the cell that
	// collides with the center boom (request 0 sits 5px away and paints
	// last); the next audible cell probes clean.
	found := -1
	bx, by := f.Requests[0].Pos[0], f.Requests[0].Pos[1]
	for i, p := range picks {
		if !p.Audible || f.Requests[i].Explosive {
			continue
		}
		dx := f.Requests[i].Pos[0] - bx
		dy := f.Requests[i].Pos[1] - by
		if dx*dx+dy*dy < 400 {
			continue
		}
		found = i
		break
	}
	if found < 0 {
		out["aud_ok"] = false
		ok = false
	} else {
		r, g, b, _ := at(f.Requests[found].Pos[0], f.Requests[found].Pos[1])
		audOK := g >= 120 && r <= 120 && b <= 150
		out["aud"] = []int{int(r), int(g), int(b)}
		out["aud_ok"] = audOK
		if !audOK {
			ok = false
		}
	}
	// Listener cross reads red.
	r, g, b, _ = at(f.Listener[0], f.Listener[1])
	lisOK := r >= 180 && g <= 90 && b <= 90
	out["listener"] = []int{int(r), int(g), int(b)}
	out["listener_ok"] = lisOK
	if !lisOK {
		ok = false
	}
	out["pixel_ok"] = ok
	return out, ok
}

func checkVoicesGolden(cur image.Image) (float64, int64, bool, bool) {
	_ = os.MkdirAll("examples/engine/camera/testdata", 0o755)
	basePath := "examples/engine/camera/testdata/voices_golden.png"
	if _, err := os.Stat(basePath); err != nil {
		fo, err := os.Create(basePath)
		if err != nil {
			return 100, 0, false, false
		}
		_ = png.Encode(fo, cur)
		_ = fo.Close()
		return 0, 0, true, true
	}
	fo, err := os.Open(basePath)
	if err != nil {
		return 100, 0, false, false
	}
	want, err := png.Decode(fo)
	_ = fo.Close()
	if err != nil {
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		return 100, int64(cur.Bounds().Dx() * cur.Bounds().Dy()), false, false
	}
	var diff int64
	total := int64(cur.Bounds().Dx() * cur.Bounds().Dy())
	for y := 0; y < cur.Bounds().Dy(); y++ {
		for x := 0; x < cur.Bounds().Dx(); x++ {
			ar, ag, ab, aa := cur.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				diff++
			}
		}
	}
	var pct float64
	if total > 0 {
		pct = 100 * float64(diff) / float64(total)
	}
	return pct, total, false, diff == 0
}

func runVoicesCase(autoOnly bool, manualSeconds int) {
	var secs int
	if autoOnly {
		secs = runSeconds(8)
		wrkit.RequireMinRun(secs, voicesAbilityID)
	} else if manualSeconds > 0 {
		secs = manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !autoOnly
	wrkit.EnsureUIFace()

	f, err := loadVoicesFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: load %s: %v\n", voicesEngineFn, err)
		os.Exit(1)
	}
	extra, ok := voicesSelftest(f)
	reqs, _ := buildVoiceReqs(f)
	picks, st, _ := audio.SelectVoices(v2(f.Listener), reqs)
	cpuImg := voicesRenderOffscreen(f, picks)
	if cpuImg == nil {
		extra["pixel_ok"] = false
		ok = false
	} else {
		pixMap, pixOK := runVoicesPixelProbes(cpuImg, f, picks)
		for k, v := range pixMap {
			extra[k] = v
		}
		if !pixOK {
			ok = false
		}
		_ = os.MkdirAll("examples/engine/camera/testdata", 0o755)
		if fh, err := os.Create("examples/engine/camera/testdata/voices_last.png"); err == nil {
			_ = png.Encode(fh, cpuImg)
			_ = fh.Close()
		}
		gd, total, first, gok := checkVoicesGolden(cpuImg)
		extra["golden_diff_pct"] = gd
		extra["golden_total_px"] = total
		if first {
			extra["golden_first_run"] = 1
		}
		extra["golden_ok"] = gok
		if !gok && !first {
			ok = false
		}
	}
	extra["case"] = "voices"
	extra["channels"] = st.Channels
	extra["audible"] = st.Audible
	if !ok {
		raw, _ := json.Marshal(map[string]any{"ability_id": voicesAbilityID, "scenario": voicesScenario, "extra": extra, "pass": false})
		fmt.Println(string(raw))
		fmt.Fprintln(os.Stderr, "game_camera: voices selftest FAIL, not opening window")
		os.Exit(1)
	}

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}
	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	root.Background = &rendering.Color{R: skyR, G: skyG, B: skyB, A: 1}
	lis, _ := audio.NewListener2D(v2(f.Camera))
	lis.MakeCurrent()
	duck, _ := audio.NewDucker(audio.DefaultDuckDepth, 250*core.Millisecond, 250*core.Millisecond)
	beat, _ := audio.NewBeatClock(f.Beat.BPM, f.Beat.BeatsPerBar)
	bus, _ := audio.NewBus("master", 0.8)
	mixer, _ := audio.NewMixer(audio.AudibleVoices)
	cam, _ := camera.NewCamera(core.V2(float64(winW), float64(winH)))
	_ = cam.SetSmoothing(liveSmoothing)
	_ = cam.Follow(v2(f.Camera))
	road := rendering.NewRenderBox()
	road.FixedWidth, road.FixedHeight = float64(winW), float64(winH)
	road.SetRepaintBoundary(true)
	elapsed := 0.0
	camX := f.Camera[0]
	road.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGBA(skyR, skyG, skyB, 1)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		// Mix bars: one row per audible pick (engine SelectVoices only).
		ps, rst, _ := audio.SelectVoices(core.V2(camX, f.Camera[1]), reqs)
		y := ay + 40.0
		n := 0
		for _, p := range ps {
			if !p.Audible {
				continue
			}
			w := voicesBarW * p.Gain
			px := ax + voicesBarX + float64((p.Pan+1)/2*40)
			if p.Gain > 0.7 {
				pc.DC.SetRGB(0.95, 0.3, 0.2)
			} else if p.Gain > 0.35 {
				pc.DC.SetRGB(0.95, 0.75, 0.2)
			} else {
				pc.DC.SetRGB(0.2, 0.65, 0.3)
			}
			pc.DC.DrawRectangle(px, y, w, voicesBarH)
			_ = pc.DC.Fill()
			y += voicesBarH + 4
			n++
			if y > ay+size.Height-40 {
				break
			}
		}
		_ = n
		_ = rst
	}
	root.Place(road, 0, 0)
	overlay := wrkit.Label("--", 13, 1, 1, 1)
	root.Place(overlay, 12, 10)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_camera", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()
	var summary manualSummary
	summary.Note = "case=voices"
	frames := 0
	beats := int64(0)
	setTitle := func() {
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("game_camera — voices ch=%d aud=%d beats=%d ptr=%d key=%d t=%.0fs",
			st.Channels, st.Audible, beats, summary.Pointer, summary.Key, elapsed))
	}
	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: skyR, ClearG: skyG, ClearB: skyB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_camera: voices close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				camX += 20
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_camera: voices pointer n=%d\n", summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_camera voices events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					camX -= 20
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_camera: voices key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
					}
				}
				setTitle()
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsLayout()
				}
				setTitle()
				return
			default:
				return
			}
		},
	})
	stC := st
	// Voices tick owns the frame: no follow-case camSim ticker here (its
	// car/road pointers are nil on this case and would nil-panic).
	voicesTick := tickFunc(func(dt float64) bool {
		if dt < 0 {
			dt = 0
		}
		if dt > 0.05 {
			dt = 0.05
		}
		frames++
		elapsed += dt
		camX = f.Camera[0] + 60*math.Sin(elapsed*0.3)
		_ = cam.Follow(core.V2(camX, f.Camera[1]))
		ps, rst, _ := audio.FollowAndSelect(&lis, cam.EffectivePos(), reqs)
		_, _ = audio.ApplyExplosionDuck(duck, rst, 1)
		duck.Update(core.SecondsFloat(dt))
		beats += int64(len(beat.Update(core.SecondsFloat(dt))))
		gains := make([]float64, 0, len(ps))
		for _, p := range ps {
			if p.Audible {
				gains = append(gains, audio.CombineGains(p.Gain, bus.Gain(), duck.Gain()))
			}
		}
		held := mixer.Mix(gains)
		snap := app.Metrics().Snapshot()
		fps := 0.0
		if snap.AvgFrameIntervalMs > 1e-6 {
			fps = 1000.0 / snap.AvgFrameIntervalMs
		}
		overlay.SetText(fmt.Sprintf("fps %.0f ch %d/%d aud %d beats %d duck %.2f 合%.2f",
			fps, rst.Channels, rst.Audible, stC.Audible, beats, duck.Gain(), held.Total))
		road.MarkNeedsPaint()
		app.ScheduleFrame()
		return true
	})
	app.Scheduler().Tickers().Add(&voicesTick)
	app.Scheduler().SetMode(scheduler.ModePersistent)
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	root.MarkNeedsPaint()
	app.ScheduleFrame()
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	extraOut := map[string]any{
		"case": "voices", "probe_ok": 1, "channels": st.Channels, "audible": st.Audible,
		"beats": beats, "frames": frames, "pixels": "boom/aud/listener probes",
		"golden": extra["golden_diff_pct"],
	}
	if autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID: voicesAbilityID, Scenario: voicesScenario, Snap: snap,
			PresentCount: presents, ElapsedSec: elapsedSec,
			SurfaceAreaPx: winW * winH, Warmup: true, Extra: extraOut,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d want >=1\n", presents)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_camera: OK voices presents=%d ch=%d aud=%d beats=%d elapsed=%.1fs\n",
			presents, st.Channels, st.Audible, beats, elapsedSec)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": voicesAbilityID, "scenario": voicesScenario,
		"backend":  win.Backend().String(),
		"events":   map[string]any{"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize},
		"presents": presents, "elapsed_sec": elapsedSec,
		"channels": st.Channels, "audible": st.Audible, "beats": beats,
		"timed": summary.Timed, "note": summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_camera: voices backend=%s presents=%d ch=%d aud=%d beats=%d elapsed=%.1fs\n",
		win.Backend(), presents, st.Channels, st.Audible, beats, elapsedSec)
}

type tickFunc func(dt float64) bool

func (f *tickFunc) Tick(dt float64) bool { return (*f)(dt) }
