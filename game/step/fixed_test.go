package step

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsFixed = 1e-9

type accumCase struct {
	FramesMs []int64 `json:"frames_ms"`
	WantStep []int   `json:"want_steps"`
	Steps    int     `json:"steps"`
	AccumMs  int64   `json:"accum_ms"`
	Elapsed  int64   `json:"elapsed_ms"`
	Alpha    float64 `json:"alpha"`
}

type interpCase struct {
	Prev  float64 `json:"prev"`
	Curr  float64 `json:"curr"`
	Alpha float64 `json:"alpha"`
	Want  float64 `json:"want"`
}

type interpVecCase struct {
	Prev  [2]float64 `json:"prev"`
	Curr  [2]float64 `json:"curr"`
	Alpha float64    `json:"alpha"`
	Want  [2]float64 `json:"want"`
}

type fixedPerf struct {
	FramesMs []int64 `json:"frames_ms"`
	Reps     int     `json:"reps"`
}

type fixedLong struct {
	Frames []int64 `json:"frames"`
	Reps   int     `json:"reps"`
}

type fixedFile struct {
	DtMs        int64           `json:"dt_ms"`
	MaxFrameMs  int64           `json:"max_frame_ms"`
	AccumCases  []accumCase     `json:"accum_cases"`
	InterpCases []interpCase    `json:"interp_cases"`
	VecCases    []interpVecCase `json:"interp_vec_cases"`
	Perf        fixedPerf       `json:"perf"`
	Longrun     fixedLong       `json:"longrun"`
}

func loadFixedCases(t *testing.T) fixedFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "fixed_cases.json"))
	if err != nil {
		t.Fatalf("read fixed_cases.json: %v", err)
	}
	var f fixedFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode fixed_cases.json: %v", err)
	}
	if len(f.AccumCases) == 0 || len(f.InterpCases) == 0 {
		t.Fatal("fixed_cases.json has no cases")
	}
	return f
}

func mustFixed(t *testing.T, dt core.Duration) *Fixed {
	t.Helper()
	f, err := NewFixed(dt)
	if err != nil {
		t.Fatalf("NewFixed(%d): %v", dt.Milliseconds(), err)
	}
	return &f
}

// driveFrames feeds ms frames into fx and returns the per-frame ticks.
func driveFrames(fx *Fixed, frames []int64) []int {
	got := make([]int, 0, len(frames))
	for _, ms := range frames {
		got = append(got, fx.Advance(core.Milliseconds(ms)))
	}
	return got
}

// checkLedger pins the integer books: accum, elapsed, alpha, frozen dt,
// plus elapsed == steps*dt so float drift can never hide.
func checkLedger(t *testing.T, tag string, fx *Fixed, dt core.Duration, accumMs, elapsedMs int64, alpha float64) {
	t.Helper()
	if fx.Accum().Milliseconds() != accumMs {
		t.Errorf("%s: accum = %dms, want %dms", tag, fx.Accum().Milliseconds(), accumMs)
	}
	if fx.Elapsed().Milliseconds() != elapsedMs {
		t.Errorf("%s: elapsed = %dms, want %dms", tag, fx.Elapsed().Milliseconds(), elapsedMs)
	}
	if math.Abs(fx.Alpha()-alpha) >= epsFixed {
		t.Errorf("%s: alpha = %.17g, want %.17g", tag, fx.Alpha(), alpha)
	}
	if fx.Dt() != dt {
		t.Errorf("%s: dt moved, want frozen %dms", tag, dt.Milliseconds())
	}
	if fx.Elapsed() != core.Duration(fx.Steps())*dt {
		t.Errorf("%s: elapsed %dms != steps*dt", tag, fx.Elapsed().Milliseconds())
	}
}

// checkInterpVec pins one frozen vector blend.
func checkInterpVec(t *testing.T, i int, c interpVecCase) {
	t.Helper()
	got := InterpVec(core.V2(c.Prev[0], c.Prev[1]), core.V2(c.Curr[0], c.Curr[1]), c.Alpha)
	if math.Abs(got.X-c.Want[0]) >= epsFixed || math.Abs(got.Y-c.Want[1]) >= epsFixed {
		t.Errorf("vec[%d] = %v, want %v", i, got, c.Want)
	}
}

func expectFixedCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

// A:快慢机动作一致:同逻辑总量不管帧怎么切,步数余数blend全落在冻结数上.
func TestFixedStepsFromCases(t *testing.T) {
	f := loadFixedCases(t)
	dt := core.Milliseconds(f.DtMs)
	if MaxFrame.Milliseconds() != f.MaxFrameMs {
		t.Fatalf("MaxFrame = %dms, want frozen %dms", MaxFrame.Milliseconds(), f.MaxFrameMs)
	}
	for i, c := range f.AccumCases {
		fx := mustFixed(t, dt)
		got := driveFrames(fx, c.FramesMs)
		if len(c.WantStep) > 0 {
			if len(got) != len(c.WantStep) {
				t.Fatalf("case[%d]: steps len = %d, want %d", i, len(got), len(c.WantStep))
			}
			for j := range got {
				if got[j] != c.WantStep[j] {
					t.Errorf("case[%d] frame[%d] = %d ticks, want %d", i, j, got[j], c.WantStep[j])
				}
			}
		}
		if uint64(len(got)) > 0 && fx.Steps() != uint64(c.Steps) {
			t.Errorf("case[%d]: total steps = %d, want %d", i, fx.Steps(), c.Steps)
		}
		checkLedger(t, "case", fx, dt, c.AccumMs, c.Elapsed, c.Alpha)
	}
	// Same logic total however the frames slice it: 48ms as 3x16 or
	// 2x24 lands on the same 3 ticks with the same remainder.
	a := mustFixed(t, dt)
	for _, ms := range []int64{16, 16, 16} {
		a.Advance(core.Milliseconds(ms))
	}
	b := mustFixed(t, dt)
	for _, ms := range []int64{24, 24} {
		b.Advance(core.Milliseconds(ms))
	}
	if a.Steps() != b.Steps() || a.Accum() != b.Accum() || a.Elapsed() != b.Elapsed() {
		t.Errorf("48ms sliced 3x16 vs 2x24 diverged: %+v vs %+v", a, b)
	}
	// Interp lands on the frozen blends.
	for i, c := range f.InterpCases {
		if got := Interp(c.Prev, c.Curr, c.Alpha); math.Abs(got-c.Want) >= epsFixed {
			t.Errorf("interp[%d] = %.17g, want %.17g", i, got, c.Want)
		}
		if got := InterpFloat(c.Prev, c.Curr, c.Alpha); math.Abs(got-c.Want) >= epsFixed {
			t.Errorf("interpFloat[%d] = %.17g, want %.17g", i, got, c.Want)
		}
	}
	for i, c := range f.VecCases {
		checkInterpVec(t, i, c)
	}
}

// B:空零超大大步坏输入全不崩不卡死,大步钳住,错码分得清.
func TestFixedEdgesNoCrash(t *testing.T) {
	f := loadFixedCases(t)
	dt := core.Milliseconds(f.DtMs)
	// Zero and negative dt never build.
	for _, bad := range []core.Duration{0, core.Milliseconds(-16)} {
		if _, err := NewFixed(bad); err == nil {
			t.Errorf("NewFixed(%d) want error", bad.Milliseconds())
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("NewFixed(%d) code = %v, want invalid-arg", bad.Milliseconds(), core.CodeOf(err))
		}
	}
	fx := mustFixed(t, dt)
	// Zero frame parks, negative parks, huge clamps to MaxFrame.
	if n := fx.Advance(0); n != 0 {
		t.Errorf("Advance(0) = %d ticks, want 0", n)
	}
	snap := *fx
	if n := fx.Advance(core.Milliseconds(-500)); n != 0 {
		t.Errorf("Advance(-500ms) = %d ticks, want 0", n)
	}
	if *fx != snap {
		t.Error("negative frame moved the loop, want parked")
	}
	big := mustFixed(t, dt)
	capped := mustFixed(t, dt)
	if n := big.Advance(core.Milliseconds(10 * 3600 * 1000)); n > int(MaxFrame/dt)+1 {
		t.Errorf("huge frame ran %d ticks, want clamped near %d", n, int(MaxFrame/dt)+1)
	}
	capped.Advance(MaxFrame)
	if big.Steps() != capped.Steps() || big.Accum() != capped.Accum() {
		t.Error("huge frame not clamped to MaxFrame")
	}
	// Alpha never leaves [0,1): ends pass through, NaN parks at prev.
	if got := Interp(10, 20, math.NaN()); got != 10 {
		t.Errorf("Interp NaN = %v, want 10 (park at prev)", got)
	}
	if got := Interp(10, 20, math.Inf(1)); got != 20 {
		t.Errorf("Interp +Inf = %v, want 20", got)
	}
	if got := Interp(10, 20, math.Inf(-1)); got != 10 {
		t.Errorf("Interp -Inf = %v, want 10", got)
	}
	if got := InterpVec(core.V2(1, 2), core.V2(9, 9), math.NaN()); got != core.V2(1, 2) {
		t.Errorf("InterpVec NaN = %v, want prev", got)
	}
	// Reset keeps Dt but clears the remainder and totals.
	fx.Advance(core.Milliseconds(100))
	fx.Reset()
	if fx.Steps() != 0 || fx.Elapsed() != 0 || fx.Accum() != 0 || fx.Alpha() != 0 {
		t.Errorf("after Reset = steps %d elapsed %d accum %d alpha %v, want zeros",
			fx.Steps(), fx.Elapsed().Milliseconds(), fx.Accum().Milliseconds(), fx.Alpha())
	}
	if fx.Dt() != dt {
		t.Error("Reset moved Dt, want frozen")
	}
	// Nil loop never panics: getters park, Advance reports 0.
	var nilFx *Fixed
	if nilFx.Dt() != 0 || nilFx.Steps() != 0 || nilFx.Elapsed() != 0 || nilFx.Accum() != 0 {
		t.Error("nil getters moved off zero, want parked")
	}
	if nilFx.Alpha() != 0 {
		t.Error("nil Alpha != 0, want park")
	}
	if n := nilFx.Advance(core.Milliseconds(16)); n != 0 {
		t.Errorf("nil Advance = %d, want 0", n)
	}
	nilFx.Reset()
	expectFixedCode(t, "NewFixed(0) code", func() error { _, err := NewFixed(0); return err }(), core.CodeInvalidArg)
}

// C does not apply (pure math, draws nothing): the integer ledger must be
// lossless and replays bitwise identical instead.
func TestFixedBoundaryIdentical(t *testing.T) {
	f := loadFixedCases(t)
	dt := core.Milliseconds(f.DtMs)
	// Same frame stream replays step-for-step and alpha-for-alpha.
	replay := func() (*Fixed, []int) {
		fx := mustFixed(t, dt)
		return fx, driveFrames(fx, f.Perf.FramesMs)
	}
	a, aNs := replay()
	b, bNs := replay()
	for i := range aNs {
		if aNs[i] != bNs[i] {
			t.Fatalf("replay diverged at frame %d: %d vs %d", i, aNs[i], bNs[i])
		}
	}
	if a.Steps() != b.Steps() || a.Accum() != b.Accum() || a.Elapsed() != b.Elapsed() || a.Alpha() != b.Alpha() {
		t.Fatal("replay totals diverged")
	}
	// Step counting matches core.Step: same index math, no drift.
	st := core.FirstStep(dt)
	for i := uint64(0); i < a.Steps(); i++ {
		st = st.Next()
	}
	if st.Index != a.Steps() {
		t.Errorf("core.Step index = %d, want %d", st.Index, a.Steps())
	}
	if st.Total() != a.Elapsed() {
		t.Errorf("core.Step total = %dms, want elapsed %dms", st.Total().Milliseconds(), a.Elapsed().Milliseconds())
	}
	// Interp ends pass through exactly; blends stay in range.
	for _, c := range f.InterpCases {
		if got := Interp(c.Prev, c.Curr, 0); got != c.Prev {
			t.Errorf("alpha 0 moved prev %v -> %v", c.Prev, got)
		}
		if got := Interp(c.Prev, c.Curr, 1); got != c.Curr {
			t.Errorf("alpha 1 moved curr %v -> %v", c.Curr, got)
		}
	}
}

// D:步数跑得动,耗时有数.
func TestFixedPerfSteps(t *testing.T) {
	// Synthetic load only (no golden): golden stays in fixed_cases.json.
	f := loadFixedCases(t)
	dt := core.Milliseconds(f.DtMs)
	if f.Perf.Reps <= 0 || len(f.Perf.FramesMs) == 0 {
		t.Fatal("perf params missing, want frozen frames and reps")
	}
	fx := mustFixed(t, dt)
	start := time.Now()
	total := 0
	for r := 0; r < f.Perf.Reps; r++ {
		for _, ms := range f.Perf.FramesMs {
			total += fx.Advance(core.Milliseconds(ms))
		}
	}
	el := time.Since(start)
	frames := int64(f.Perf.Reps * len(f.Perf.FramesMs))
	t.Logf("fixed-perf: %d frames -> %d steps in %v (%.1f ns/frame)", frames, total, el, float64(el.Nanoseconds())/float64(frames))
	if total <= 0 {
		t.Error("perf produced no steps, benchmark invalid")
	}
	// Interp costs with the blend on the hot path.
	var acc float64
	start = time.Now()
	for i := 0; i < 10000; i++ {
		acc += Interp(0, 100, float64(i%1000)/1000)
		acc += InterpVec(core.V2(0, 0), core.V2(100, 50), float64(i%1000)/1000).X
	}
	el = time.Since(start)
	t.Logf("fixed-interp-10k: blends in %v (%.1f ns/op)", el, float64(el.Nanoseconds())/10000)
	if math.IsNaN(acc) || math.IsInf(acc, 0) || acc == 0 {
		t.Error("interp perf accumulation invalid")
	}
}

// E:长跑不漂:十万帧双重放逐位一致,整数台账对得上.
func TestFixedLongRunNoDrift(t *testing.T) {
	f := loadFixedCases(t)
	dt := core.Milliseconds(f.DtMs)
	if f.Longrun.Reps <= 0 || len(f.Longrun.Frames) == 0 {
		t.Fatal("longrun params missing, want frozen frames and reps")
	}
	run := func() *Fixed {
		fx := mustFixed(t, dt)
		for i := 0; i < f.Longrun.Reps; i++ {
			fx.Advance(core.Milliseconds(f.Longrun.Frames[i%len(f.Longrun.Frames)]))
		}
		return fx
	}
	a, b := run(), run()
	if a.Steps() != b.Steps() || a.Accum() != b.Accum() || a.Elapsed() != b.Elapsed() {
		t.Fatalf("long replay diverged: %+v vs %+v", a, b)
	}
	if a.Elapsed() != core.Duration(a.Steps())*dt {
		t.Error("long elapsed != steps*dt, ledger drifted")
	}
	// Alpha stays in [0,1) after the soak; a fresh Reset replays clean.
	if al := a.Alpha(); math.IsNaN(al) || al < 0 || al >= 1 {
		t.Errorf("long alpha = %v, want in [0,1)", al)
	}
	a.Reset()
	if a.Steps() != 0 || a.Elapsed() != 0 || a.Accum() != 0 {
		t.Error("post-soak Reset did not clear totals")
	}
}

// F:离屏金对照窗(W1窗免,纯算数):冻结数加形状断言.
func TestFixedOffscreenGolden(t *testing.T) {
	f := loadFixedCases(t)
	dt := core.Milliseconds(f.DtMs)
	// Golden numbers stay frozen.
	for i, c := range f.AccumCases {
		if len(c.WantStep) == 0 {
			continue
		}
		if got := driveFrames(mustFixed(t, dt), c.FramesMs); len(got) != len(c.WantStep) {
			t.Fatalf("golden case[%d] frames = %d, want %d", i, len(got), len(c.WantStep))
		} else {
			for j, n := range got {
				if n != c.WantStep[j] {
					t.Fatalf("golden case[%d] frame[%d] = %d, want %d", i, j, n, c.WantStep[j])
				}
			}
		}
	}
	// Shape: dt slices divide evenly (16ms x3 = 3 ticks, no remainder).
	even := mustFixed(t, dt)
	for _, ms := range []int64{16, 16, 16} {
		even.Advance(core.Milliseconds(ms))
	}
	if even.Steps() != 3 || even.Accum() != 0 || even.Alpha() != 0 {
		t.Errorf("even 48ms = steps %d accum %d alpha %v, want 3/0/0",
			even.Steps(), even.Accum().Milliseconds(), even.Alpha())
	}
	// Shape: partial remainder blends strictly inside (0,1).
	part := mustFixed(t, dt)
	part.Advance(core.Milliseconds(24))
	if al := part.Alpha(); al <= 0 || al >= 1 {
		t.Errorf("24ms alpha = %v, want inside (0,1)", al)
	}
	if got := Interp(0, 100, part.Alpha()); got != 50 {
		t.Errorf("half blend = %v, want 50", got)
	}
	// Shape: clamped hitch never exceeds one frame of debt plus remainder.
	hitch := mustFixed(t, dt)
	n := hitch.Advance(MaxFrame)
	if n != int(MaxFrame/dt) {
		t.Errorf("MaxFrame ticks = %d, want %d", n, int(MaxFrame/dt))
	}
	if hitch.Accum() != MaxFrame%dt {
		t.Errorf("MaxFrame remainder = %dms, want %dms", hitch.Accum().Milliseconds(), (MaxFrame % dt).Milliseconds())
	}
}
