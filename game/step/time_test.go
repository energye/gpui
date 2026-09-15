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

type splitCase struct {
	FrameMs     int64   `json:"frame_ms"`
	Scale       float64 `json:"scale"`
	Paused      bool    `json:"paused"`
	WantWorldMs int64   `json:"want_world_ms"`
	WantUIMs    int64   `json:"want_ui_ms"`
}

type scaleCase struct {
	In   float64 `json:"in"`
	Want float64 `json:"want"`
}

type seqCase struct {
	FramesMs    []int64 `json:"frames_ms"`
	Scale       float64 `json:"scale"`
	Paused      []bool  `json:"paused"`
	WantWorldMs int64   `json:"want_world_ms"`
	WantUIMs    int64   `json:"want_ui_ms"`
}

type timePerf struct {
	FramesMs []int64 `json:"frames_ms"`
	Reps     int     `json:"reps"`
}

type timeLong struct {
	Frames []int64 `json:"frames"`
	Reps   int     `json:"reps"`
	Scale  float64 `json:"scale"`
}

type timeFile struct {
	MaxScale     float64     `json:"max_scale"`
	MaxFrameMs   int64       `json:"max_frame_ms"`
	DefaultScale float64     `json:"default_scale"`
	SplitCases   []splitCase `json:"split_cases"`
	ScaleCases   []scaleCase `json:"scale_cases"`
	SeqCases     []seqCase   `json:"seq_cases"`
	Perf         timePerf    `json:"perf"`
	Longrun      timeLong    `json:"longrun"`
}

func loadTimeCases(t *testing.T) timeFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "time_cases.json"))
	if err != nil {
		t.Fatalf("read time_cases.json: %v", err)
	}
	var f timeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode time_cases.json: %v", err)
	}
	if len(f.SplitCases) == 0 || len(f.ScaleCases) == 0 || len(f.SeqCases) == 0 {
		t.Fatal("time_cases.json has no cases")
	}
	return f
}

func mustClockWith(t *testing.T, scale float64, paused bool) Clock {
	t.Helper()
	c := NewClock()
	c.SetTimeScale(scale)
	c.SetPaused(paused)
	return c
}

func checkSplit(t *testing.T, tag string, c splitCase) {
	t.Helper()
	w, u := Split(core.Milliseconds(c.FrameMs), c.Scale, c.Paused)
	if w.Milliseconds() != c.WantWorldMs || u.Milliseconds() != c.WantUIMs {
		t.Errorf("%s: Split(%dms,x%v,paused=%v) = %d/%dms, want %d/%dms",
			tag, c.FrameMs, c.Scale, c.Paused, w.Milliseconds(), u.Milliseconds(),
			c.WantWorldMs, c.WantUIMs)
	}
	cc := mustClockWith(t, c.Scale, c.Paused)
	gw, gu := cc.Advance(core.Milliseconds(c.FrameMs))
	if gw != w || gu != u {
		t.Errorf("%s: Advance diverged from Split: %d/%dms vs %d/%dms",
			tag, gw.Milliseconds(), gu.Milliseconds(), w.Milliseconds(), u.Milliseconds())
	}
	if cc.WorldElapsed() != w || cc.UIElapsed() != u {
		t.Errorf("%s: ledger = %d/%dms, want single split %d/%dms",
			tag, cc.WorldElapsed().Milliseconds(), cc.UIElapsed().Milliseconds(),
			w.Milliseconds(), u.Milliseconds())
	}
}

// A:暂停慢放加速对:分层世界停界面动,慢放减半加速加倍.
func TestClockSplitFromCases(t *testing.T) {
	f := loadTimeCases(t)
	if MaxScale != f.MaxScale {
		t.Fatalf("MaxScale = %v, want frozen %v", MaxScale, f.MaxScale)
	}
	if MaxFrame.Milliseconds() != f.MaxFrameMs {
		t.Fatalf("MaxFrame = %dms, want frozen %dms", MaxFrame.Milliseconds(), f.MaxFrameMs)
	}
	if got := NewClock(); got.TimeScale() != f.DefaultScale || got.Paused() {
		t.Fatalf("NewClock = x%v paused=%v, want x%v unpaused",
			got.TimeScale(), got.Paused(), f.DefaultScale)
	}
	for _, c := range f.ScaleCases {
		cc := NewClock()
		cc.SetTimeScale(c.In)
		if cc.TimeScale() != c.Want {
			t.Errorf("SetTimeScale(%v) = %v, want %v", c.In, cc.TimeScale(), c.Want)
		}
	}
	for i, c := range f.SplitCases {
		checkSplit(t, "split", c)
		_ = i
	}
	for i, s := range f.SeqCases {
		if len(s.Paused) != len(s.FramesMs) {
			t.Fatalf("seq[%d]: paused len = %d, want %d", i, len(s.Paused), len(s.FramesMs))
		}
		cc := mustClockWith(t, s.Scale, false)
		for j, ms := range s.FramesMs {
			cc.SetPaused(s.Paused[j])
			cc.Advance(core.Milliseconds(ms))
		}
		if cc.WorldElapsed().Milliseconds() != s.WantWorldMs {
			t.Errorf("seq[%d]: world = %dms, want %dms", i, cc.WorldElapsed().Milliseconds(), s.WantWorldMs)
		}
		if cc.UIElapsed().Milliseconds() != s.WantUIMs {
			t.Errorf("seq[%d]: ui = %dms, want %dms", i, cc.UIElapsed().Milliseconds(), s.WantUIMs)
		}
	}
}

// B:倍率零负钳住,坏输入不崩不卡死,空时钟不崩.
func TestClockEdgesNoCrash(t *testing.T) {
	f := loadTimeCases(t)
	// Zero and negative park at 0; over-cap parks at MaxScale.
	for _, bad := range []float64{0, -1, -0.5, -1e308} {
		cc := NewClock()
		cc.SetTimeScale(bad)
		if cc.TimeScale() != 0 {
			t.Errorf("SetTimeScale(%v) = %v, want 0 (park)", bad, cc.TimeScale())
		}
		if w, _ := Split(core.Milliseconds(16), bad, false); w != 0 {
			t.Errorf("Split(16ms,x%v) world = %dms, want 0", bad, w.Milliseconds())
		}
	}
	// Non-finite never poisons the ledger.
	cc := NewClock()
	cc.SetTimeScale(math.NaN())
	if cc.TimeScale() != 0 {
		t.Errorf("SetTimeScale(NaN) = %v, want 0", cc.TimeScale())
	}
	cc.SetTimeScale(math.Inf(1))
	if cc.TimeScale() != MaxScale {
		t.Errorf("SetTimeScale(+Inf) = %v, want %v", cc.TimeScale(), MaxScale)
	}
	cc.SetTimeScale(math.Inf(-1))
	if cc.TimeScale() != 0 {
		t.Errorf("SetTimeScale(-Inf) = %v, want 0", cc.TimeScale())
	}
	if w, u := Split(core.Milliseconds(16), math.NaN(), false); w != 0 || u != core.Milliseconds(16) {
		t.Errorf("Split NaN = %d/%dms, want 0/16ms", w.Milliseconds(), u.Milliseconds())
	}
	if w, _ := Split(core.Milliseconds(16), math.Inf(1), false); w != core.Milliseconds(int64(f.MaxScale*16)) {
		t.Errorf("Split +Inf world = %dms, want %dms", w.Milliseconds(), int64(f.MaxScale*16))
	}
	// Zero frame parks, negative parks, huge clamps to MaxFrame.
	park := mustClockWith(t, 1, false)
	if w, u := park.Advance(0); w != 0 || u != 0 {
		t.Errorf("Advance(0) = %d/%dms, want 0/0", w.Milliseconds(), u.Milliseconds())
	}
	snap := park
	if w, u := park.Advance(core.Milliseconds(-500)); w != 0 || u != 0 {
		t.Errorf("Advance(-500ms) = %d/%dms, want 0/0", w.Milliseconds(), u.Milliseconds())
	}
	if park != snap {
		t.Error("negative frame moved the clock, want parked")
	}
	big := mustClockWith(t, 2, false)
	w, u := big.Advance(core.Milliseconds(10 * 3600 * 1000))
	if u != MaxFrame {
		t.Errorf("huge ui = %dms, want MaxFrame %dms", u.Milliseconds(), MaxFrame.Milliseconds())
	}
	if w != MaxFrame.Scale(2) {
		t.Errorf("huge world = %dms, want MaxFrame x2 %dms", w.Milliseconds(), MaxFrame.Scale(2).Milliseconds())
	}
	// Paused huge frame still moves the UI only.
	paused := mustClockWith(t, 2, true)
	if w, u := paused.Advance(core.Milliseconds(10000)); w != 0 || u != MaxFrame {
		t.Errorf("paused huge = %d/%dms, want 0/%dms", w.Milliseconds(), u.Milliseconds(), MaxFrame.Milliseconds())
	}
	// Reset keeps scale and pause but clears both totals.
	cc = mustClockWith(t, 0.5, true)
	cc.Advance(core.Milliseconds(100))
	cc.Reset()
	if cc.WorldElapsed() != 0 || cc.UIElapsed() != 0 {
		t.Errorf("after Reset = %d/%dms, want 0/0", cc.WorldElapsed().Milliseconds(), cc.UIElapsed().Milliseconds())
	}
	if cc.TimeScale() != 0.5 || !cc.Paused() {
		t.Errorf("Reset moved scale/paused: x%v paused=%v, want x0.5 paused=true", cc.TimeScale(), cc.Paused())
	}
	// Nil clock never panics.
	var nilClock *Clock
	if nilClock.TimeScale() != 0 || nilClock.Paused() {
		t.Error("nil getters moved off zero/false, want parked")
	}
	if nilClock.WorldElapsed() != 0 || nilClock.UIElapsed() != 0 {
		t.Error("nil ledger moved off zero, want parked")
	}
	if w, u := nilClock.Advance(core.Milliseconds(16)); w != 0 || u != 0 {
		t.Errorf("nil Advance = %d/%dms, want 0/0", w.Milliseconds(), u.Milliseconds())
	}
	nilClock.SetTimeScale(2)
	nilClock.SetPaused(true)
	nilClock.Pause()
	nilClock.Resume()
	nilClock.Reset()
	if nilClock.TimeScale() != 0 || nilClock.Paused() {
		t.Error("nil setter moved state, want parked")
	}
}

// C does not apply (pure math, draws nothing): the split path must be
// lossless and replays bitwise identical instead.
func TestClockBoundaryIdentical(t *testing.T) {
	f := loadTimeCases(t)
	// Same frame stream replays world-for-world and ui-for-ui.
	replay := func() (Clock, []core.Duration, []core.Duration) {
		cc := mustClockWith(t, f.Longrun.Scale, false)
		ws := make([]core.Duration, 0, len(f.Perf.FramesMs))
		us := make([]core.Duration, 0, len(f.Perf.FramesMs))
		for _, ms := range f.Perf.FramesMs {
			w, u := cc.Advance(core.Milliseconds(ms))
			ws = append(ws, w)
			us = append(us, u)
		}
		return cc, ws, us
	}
	a, aW, aU := replay()
	b, _, _ := replay()
	for i := range aW {
		w, u := Split(core.Milliseconds(f.Perf.FramesMs[i]), f.Longrun.Scale, false)
		if aW[i] != w || aU[i] != u {
			t.Fatalf("replay frame %d diverged from Split", i)
		}
	}
	if a.WorldElapsed() != b.WorldElapsed() || a.UIElapsed() != b.UIElapsed() {
		t.Fatal("replay totals diverged")
	}
	// Advance wires Split directly, never a second formula.
	cc := mustClockWith(t, 2, false)
	w, u := cc.Advance(core.Milliseconds(16))
	if sw, su := Split(core.Milliseconds(16), 2, false); w != sw || u != su {
		t.Errorf("Advance wiring diverged: %d/%dms vs Split %d/%dms",
			w.Milliseconds(), u.Milliseconds(), sw.Milliseconds(), su.Milliseconds())
	}
	// Ledger adds exactly: totals equal the per-frame sum.
	sumW, sumU := core.Duration(0), core.Duration(0)
	cc2 := mustClockWith(t, 0.5, false)
	for _, ms := range f.Perf.FramesMs {
		w, u := cc2.Advance(core.Milliseconds(ms))
		sumW += w
		sumU += u
	}
	if cc2.WorldElapsed() != sumW || cc2.UIElapsed() != sumU {
		t.Error("ledger != per-frame sum, want exact adds")
	}
	// Paused replay keeps the UI while the world stays zero.
	p1 := mustClockWith(t, 2, true)
	p2 := mustClockWith(t, 2, true)
	for _, ms := range f.Perf.FramesMs {
		w1, u1 := p1.Advance(core.Milliseconds(ms))
		w2, u2 := p2.Advance(core.Milliseconds(ms))
		if w1 != 0 || w2 != 0 || u1 != u2 {
			t.Fatalf("paused replay diverged: %d/%dms vs %d/%dms", w1.Milliseconds(), u1.Milliseconds(), w2.Milliseconds(), u2.Milliseconds())
		}
	}
	if p1.WorldElapsed() != 0 || p1.WorldElapsed() != p2.WorldElapsed() || p1.UIElapsed() != p2.UIElapsed() {
		t.Error("paused totals diverged, want identical")
	}
}

// D:切换跑得动,耗时有数.
func TestClockPerfSwitch(t *testing.T) {
	// Synthetic load only (no golden): golden stays in time_cases.json.
	f := loadTimeCases(t)
	if f.Perf.Reps <= 0 || len(f.Perf.FramesMs) == 0 {
		t.Fatal("perf params missing, want frozen frames and reps")
	}
	cc := mustClockWith(t, 1, false)
	scales := []float64{0.5, 1, 2, 0, 8}
	start := time.Now()
	var sumW, sumU core.Duration
	total := 0
	for r := 0; r < f.Perf.Reps; r++ {
		cc.SetTimeScale(scales[r%len(scales)])
		if r%2 == 0 {
			cc.Pause()
		} else {
			cc.Resume()
		}
		for _, ms := range f.Perf.FramesMs {
			w, u := cc.Advance(core.Milliseconds(ms))
			sumW += w
			sumU += u
			total++
		}
	}
	el := time.Since(start)
	t.Logf("time-perf: %d advances (switch scale/pause each rep) in %v (%.1f ns/op, world %dms ui %dms)",
		total, el, float64(el.Nanoseconds())/float64(total), sumW.Milliseconds(), sumU.Milliseconds())
	if total <= 0 || sumU <= 0 {
		t.Error("perf produced no advances, benchmark invalid")
	}
	// Stateless split costs on the hot path too.
	var acc core.Duration
	start = time.Now()
	for i := 0; i < 10000; i++ {
		w, u := Split(core.Milliseconds(int64(8+i%25)), scales[i%len(scales)], i%2 == 0)
		acc += w + u
	}
	el = time.Since(start)
	t.Logf("time-split-10k: splits in %v (%.1f ns/op)", el, float64(el.Nanoseconds())/10000)
	if acc <= 0 {
		t.Error("split perf accumulation invalid")
	}
}

// E:长跑不漂:十万帧双重放逐位一致,整数台账对得上.
func TestClockLongRunNoDrift(t *testing.T) {
	f := loadTimeCases(t)
	if f.Longrun.Reps <= 0 || len(f.Longrun.Frames) == 0 {
		t.Fatal("longrun params missing, want frozen frames and reps")
	}
	run := func() Clock {
		cc := mustClockWith(t, f.Longrun.Scale, false)
		for i := 0; i < f.Longrun.Reps; i++ {
			cc.Advance(core.Milliseconds(f.Longrun.Frames[i%len(f.Longrun.Frames)]))
		}
		return cc
	}
	a, b := run(), run()
	if a.WorldElapsed() != b.WorldElapsed() || a.UIElapsed() != b.UIElapsed() {
		t.Fatalf("long replay diverged: %d/%dms vs %d/%dms",
			a.WorldElapsed().Milliseconds(), a.UIElapsed().Milliseconds(),
			b.WorldElapsed().Milliseconds(), b.UIElapsed().Milliseconds())
	}
	// Totals equal the exact per-cycle sum scaled by full cycles.
	var cycleW, cycleU core.Duration
	for _, ms := range f.Longrun.Frames {
		w, u := Split(core.Milliseconds(ms), f.Longrun.Scale, false)
		cycleW += w
		cycleU += u
	}
	cycles := f.Longrun.Reps / len(f.Longrun.Frames)
	rest := f.Longrun.Reps % len(f.Longrun.Frames)
	wantW, wantU := cycleW*core.Duration(cycles), cycleU*core.Duration(cycles)
	for i := 0; i < rest; i++ {
		w, u := Split(core.Milliseconds(f.Longrun.Frames[i]), f.Longrun.Scale, false)
		wantW += w
		wantU += u
	}
	if a.WorldElapsed() != wantW || a.UIElapsed() != wantU {
		t.Errorf("long ledger = %d/%dms, want cycle sum %d/%dms",
			a.WorldElapsed().Milliseconds(), a.UIElapsed().Milliseconds(),
			wantW.Milliseconds(), wantU.Milliseconds())
	}
	// Paused soak keeps the world at zero while the UI still accrues.
	p := mustClockWith(t, f.Longrun.Scale, true)
	for i := 0; i < f.Longrun.Reps; i++ {
		p.Advance(core.Milliseconds(f.Longrun.Frames[i%len(f.Longrun.Frames)]))
	}
	if p.WorldElapsed() != 0 {
		t.Errorf("paused long world = %dms, want 0", p.WorldElapsed().Milliseconds())
	}
	q := mustClockWith(t, f.Longrun.Scale, true)
	for i := 0; i < f.Longrun.Reps; i++ {
		q.Advance(core.Milliseconds(f.Longrun.Frames[i%len(f.Longrun.Frames)]))
	}
	if p.UIElapsed() != q.UIElapsed() {
		t.Error("paused long UI replay diverged")
	}
	// A fresh Reset replays clean.
	a.Reset()
	if a.WorldElapsed() != 0 || a.UIElapsed() != 0 {
		t.Error("post-soak Reset did not clear totals")
	}
	if a.TimeScale() != f.Longrun.Scale || a.Paused() {
		t.Error("post-soak Reset moved scale/pause, want kept")
	}
}

// F:离屏金对照窗(W2先离屏,真窗game_step--case=pause后建):冻结数加形状断言.
func TestClockOffscreenGolden(t *testing.T) {
	f := loadTimeCases(t)
	// Golden numbers stay frozen.
	if MaxScale != f.MaxScale {
		t.Fatalf("MaxScale = %v, want frozen %v", MaxScale, f.MaxScale)
	}
	if MaxFrame.Milliseconds() != f.MaxFrameMs {
		t.Fatalf("MaxFrame = %dms, want frozen %dms", MaxFrame.Milliseconds(), f.MaxFrameMs)
	}
	for i, c := range f.SplitCases {
		if w, u := Split(core.Milliseconds(c.FrameMs), c.Scale, c.Paused); w.Milliseconds() != c.WantWorldMs || u.Milliseconds() != c.WantUIMs {
			t.Fatalf("golden split[%d] = %d/%dms, want %d/%dms",
				i, w.Milliseconds(), u.Milliseconds(), c.WantWorldMs, c.WantUIMs)
		}
	}
	for i, c := range f.ScaleCases {
		cc := NewClock()
		cc.SetTimeScale(c.In)
		if cc.TimeScale() != c.Want {
			t.Fatalf("golden scale[%d] x%v = x%v, want x%v", i, c.In, cc.TimeScale(), c.Want)
		}
	}
	for i, s := range f.SeqCases {
		cc := mustClockWith(t, s.Scale, false)
		for j, ms := range s.FramesMs {
			cc.SetPaused(s.Paused[j])
			cc.Advance(core.Milliseconds(ms))
		}
		if cc.WorldElapsed().Milliseconds() != s.WantWorldMs || cc.UIElapsed().Milliseconds() != s.WantUIMs {
			t.Fatalf("golden seq[%d] = %d/%dms, want %d/%dms",
				i, cc.WorldElapsed().Milliseconds(), cc.UIElapsed().Milliseconds(),
				s.WantWorldMs, s.WantUIMs)
		}
	}
	// Shape: pause stops the world only; the UI keeps the clamped frame.
	if w, u := Split(core.Milliseconds(16), 1, true); w != 0 || u != core.Milliseconds(16) {
		t.Errorf("paused 16ms = %d/%dms, want 0/16ms", w.Milliseconds(), u.Milliseconds())
	}
	// Shape: slow halves and accel doubles the same frame.
	if w, _ := Split(core.Milliseconds(16), 0.5, false); w != core.Milliseconds(8) {
		t.Errorf("slow 16ms = %dms, want 8ms", w.Milliseconds())
	}
	if w, _ := Split(core.Milliseconds(16), 2, false); w != core.Milliseconds(32) {
		t.Errorf("accel 16ms = %dms, want 32ms", w.Milliseconds())
	}
	// Shape: hitch parks the UI at MaxFrame while the world scales it.
	if _, u := Split(core.Milliseconds(10000), 1, false); u != MaxFrame {
		t.Errorf("hitch ui = %dms, want MaxFrame %dms", u.Milliseconds(), MaxFrame.Milliseconds())
	}
	if w, _ := Split(core.Milliseconds(500), 2, false); w != MaxFrame.Scale(2) {
		t.Errorf("hitch accel world = %dms, want %dms", w.Milliseconds(), MaxFrame.Scale(2).Milliseconds())
	}
	// Shape: over-cap parks at MaxScale, never beyond.
	if w, _ := Split(core.Milliseconds(16), 100, false); w != core.Milliseconds(int64(f.MaxScale*16)) {
		t.Errorf("over-cap world = %dms, want MaxScale x16 = %dms", w.Milliseconds(), int64(f.MaxScale*16))
	}
}
