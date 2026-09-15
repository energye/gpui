package anim

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsTimeline = 1e-9

type tlKeyDef struct {
	AtMs  int64   `json:"at_ms"`
	Value float64 `json:"value"`
	Ease  string  `json:"ease"`
}

type tlTrackDef struct {
	Name string     `json:"name"`
	Keys []tlKeyDef `json:"keys"`
}

type tlEventDef struct {
	AtMs int64  `json:"at_ms"`
	Name string `json:"name"`
}

type tlStepDef struct {
	DtMs       int64              `json:"dt_ms"`
	WantPos    int64              `json:"want_pos"`
	Want       map[string]float64 `json:"want"`
	WantEvents []string           `json:"want_events"`
}

type tlSeqDef struct {
	Name  string      `json:"name"`
	Loop  string      `json:"loop"`
	Steps []tlStepDef `json:"steps"`
}

type tlFile struct {
	DurationMs int64        `json:"duration_ms"`
	Tracks     []tlTrackDef `json:"tracks"`
	Events     []tlEventDef `json:"events"`
	Sequences  []tlSeqDef   `json:"sequences"`
}

func loadTimelineCases(t *testing.T) tlFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "timeline_cases.json"))
	if err != nil {
		t.Fatalf("read timeline_cases.json: %v", err)
	}
	var f tlFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode timeline_cases.json: %v", err)
	}
	if len(f.Tracks) == 0 || len(f.Events) == 0 || len(f.Sequences) == 0 {
		t.Fatal("timeline_cases.json has no tracks, events, or sequences")
	}
	return f
}

func mustTimelineLoop(t *testing.T, s string) LoopMode {
	t.Helper()
	switch s {
	case "once":
		return LoopOnce
	case "loop":
		return LoopLoop
	case "pingpong":
		return LoopPingPong
	}
	t.Fatalf("unknown loop %q", s)
	return LoopOnce
}

func buildTimeline(t *testing.T, f tlFile, loop LoopMode) *Timeline {
	t.Helper()
	tl, err := NewTimeline(core.Milliseconds(f.DurationMs))
	if err != nil {
		t.Fatalf("NewTimeline: %v", err)
	}
	if err := tl.SetLoop(loop); err != nil {
		t.Fatalf("SetLoop: %v", err)
	}
	for _, tr := range f.Tracks {
		for _, k := range tr.Keys {
			kind, err := Parse(k.Ease)
			if err != nil {
				t.Fatalf("Parse %q: %v", k.Ease, err)
			}
			if err := tl.AddKey(tr.Name, Key{
				At:    core.Milliseconds(k.AtMs),
				Value: k.Value,
				Ease:  kind,
			}); err != nil {
				t.Fatalf("AddKey %s/%d: %v", tr.Name, k.AtMs, err)
			}
		}
	}
	for _, e := range f.Events {
		if err := tl.AddEvent(core.Milliseconds(e.AtMs), e.Name); err != nil {
			t.Fatalf("AddEvent %q/%d: %v", e.Name, e.AtMs, err)
		}
	}
	return tl
}

func mustFindSeq(t *testing.T, f tlFile, name string) tlSeqDef {
	t.Helper()
	for _, s := range f.Sequences {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("timeline_cases.json has no sequence %q", name)
	return tlSeqDef{}
}

func eventNames(evs []Event) []string {
	out := []string{}
	for _, e := range evs {
		out = append(out, e.Name)
	}
	return out
}

func sameNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A: interpolation, wrap, and cue order land on the frozen sequences.
func TestTimelineSamplesFromCases(t *testing.T) {
	f := loadTimelineCases(t)
	tracks := []string{"x", "alpha", "slash"}
	for _, seq := range f.Sequences {
		tl := buildTimeline(t, f, mustTimelineLoop(t, seq.Loop))
		var gotCalls []Event
		tl.OnEvent(func(ev Event) { gotCalls = append(gotCalls, ev) })
		for i, st := range seq.Steps {
			gotCalls = nil
			fired, ok := tl.Update(core.Milliseconds(st.DtMs))
			if !ok {
				t.Errorf("%s step %d: Update ok=false, want true", seq.Name, i)
				continue
			}
			if got := eventNames(fired); !sameNames(got, st.WantEvents) {
				t.Errorf("%s step %d: events = %q, want %q", seq.Name, i, got, st.WantEvents)
			}
			if pos := tl.Pos().Milliseconds(); pos != st.WantPos {
				t.Errorf("%s step %d: pos = %d, want %d", seq.Name, i, pos, st.WantPos)
			}
			for _, tr := range tracks {
				v, ok := tl.Sample(tr, tl.Pos())
				if !ok {
					t.Errorf("%s step %d: Sample %q ok=false", seq.Name, i, tr)
					continue
				}
				if math.Abs(v-st.Want[tr]) >= epsTimeline {
					t.Errorf("%s step %d: %s = %.17g, want %.17g",
						seq.Name, i, tr, v, st.Want[tr])
				}
			}
			// Callback mirrors the returned cues exactly.
			if len(fired) == 0 {
				if len(gotCalls) != 0 {
					t.Errorf("%s step %d: no cues but callback fired %+v", seq.Name, i, gotCalls)
				}
			} else {
				if len(gotCalls) != len(fired) {
					t.Errorf("%s step %d: callback fired %d, want %d",
						seq.Name, i, len(gotCalls), len(fired))
				} else {
					for j := range fired {
						if gotCalls[j] != fired[j] {
							t.Errorf("%s step %d: callback[%d] %+v != %+v",
								seq.Name, i, j, gotCalls[j], fired[j])
						}
					}
				}
			}
		}
	}
}

// B: empty, zero, oversized, and corrupt inputs error without crashing.
func TestTimelineEdgesNoCrash(t *testing.T) {
	f := loadTimelineCases(t)

	// Bad durations build nothing.
	for _, ms := range []int64{0, -100} {
		if tl, err := NewTimeline(core.Milliseconds(ms)); err == nil || tl != nil {
			t.Errorf("NewTimeline(%d) = %v/%v, want error", ms, tl, err)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("NewTimeline(%d) code = %v, want invalid-arg", ms, core.CodeOf(err))
		}
	}
	tl := buildTimeline(t, f, LoopOnce)
	dur := tl.Duration()
	trackKeys, eventCount := len(f.Tracks[0].Keys), len(f.Events)

	// Bad keys store nothing and report invalid-arg.
	badKeys := []struct {
		track string
		key   Key
	}{
		{"", Key{At: 0, Value: 1, Ease: Linear}},
		{"x", Key{At: core.Milliseconds(-1), Value: 1, Ease: Linear}},
		{"x", Key{At: dur + 1, Value: 1, Ease: Linear}},
		{"x", Key{At: 0, Value: 1, Ease: Linear}},
		{"x", Key{At: core.Milliseconds(100), Value: 1, Ease: Kind(-1)}},
		{"new", Key{At: core.Milliseconds(100), Value: 1, Ease: Kind(1e6)}},
	}
	for i, b := range badKeys {
		if err := tl.AddKey(b.track, b.key); err == nil {
			t.Errorf("bad key[%d] %+v: want error", i, b)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad key[%d] code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	if tl.KeyCount("x") != trackKeys || tl.KeyCount("new") != 0 {
		t.Errorf("bad AddKey changed keys: x=%d new=%d", tl.KeyCount("x"), tl.KeyCount("new"))
	}

	// Bad cues store nothing and report invalid-arg.
	for i, e := range []struct {
		at   core.Duration
		name string
	}{
		{0, ""}, {-1, "neg"}, {dur + 1, "past"},
	} {
		if err := tl.AddEvent(e.at, e.name); err == nil {
			t.Errorf("bad event[%d]: want error", i)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad event[%d] code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	if tl.EventCount() != eventCount {
		t.Errorf("bad AddEvent changed count to %d, want %d", tl.EventCount(), eventCount)
	}

	// Bad wrap keeps the old mode.
	if err := tl.SetLoop(LoopMode(99)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad SetLoop err = %v, want invalid-arg", err)
	}
	if tl.Loop() != LoopOnce {
		t.Errorf("bad SetLoop moved mode to %v", tl.Loop())
	}

	// Unknown tracks read ok=false, never a value.
	if v, ok := tl.Sample("no-track", 0); ok || v != 0 {
		t.Errorf("unknown track Sample = %v/%v, want 0/false", v, ok)
	}

	// Zero and negative dt advance nothing and fire nothing.
	var calls int
	tl.OnEvent(func(Event) { calls++ })
	pos := tl.Pos()
	for _, dt := range []core.Duration{0, core.Milliseconds(-16)} {
		calls = 0
		fired, ok := tl.Update(dt)
		if !ok || len(fired) != 0 || tl.Pos() != pos {
			t.Errorf("dt %d: Update moved head or fired", dt.Milliseconds())
		}
		if calls != 0 {
			t.Errorf("dt %d: callback fired %d times, want 0", dt.Milliseconds(), calls)
		}
	}
	// Nil handler clears the callback without breaking playback.
	tl.OnEvent(nil)
	if fired, ok := tl.Update(core.Milliseconds(250)); !ok || len(fired) != 1 {
		t.Errorf("nil-handler Update = %v/%v, want one cue", fired, ok)
	}

	// A finished once head holds the end and stays silent.
	end, err := NewTimeline(dur)
	if err != nil {
		t.Fatalf("NewTimeline: %v", err)
	}
	for _, tr := range f.Tracks {
		for _, k := range tr.Keys {
			kind, _ := Parse(k.Ease)
			if err := end.AddKey(tr.Name, Key{At: core.Milliseconds(k.AtMs), Value: k.Value, Ease: kind}); err != nil {
				t.Fatalf("AddKey: %v", err)
			}
		}
	}
	if _, ok := end.Update(dur); !ok || end.IsPlaying() {
		t.Fatalf("full-length Update still playing, want finished")
	}
	if pos := end.Pos(); pos != dur {
		t.Errorf("finished pos = %d, want %d", pos.Milliseconds(), dur.Milliseconds())
	}
	if fired, ok := end.Update(core.Milliseconds(100)); !ok || len(fired) != 0 {
		t.Errorf("finished Update = %v/%v, want silent hold", fired, ok)
	}
	// Seek clamps into range, faces forward, fires nothing.
	if err := end.Seek(dur + 5000); err != nil {
		t.Errorf("Seek past end: %v", err)
	}
	if pos := end.Pos(); pos != dur {
		t.Errorf("Seek past end pos = %d, want dur", pos.Milliseconds())
	}
	if err := end.Seek(core.Milliseconds(-5)); err != nil {
		t.Errorf("Seek before start: %v", err)
	}
	if pos := end.Pos(); pos != 0 {
		t.Errorf("Seek before start pos = %d, want 0", pos.Milliseconds())
	}

	// Huge dt stays bounded: once fires the rest once, loop fires each once.
	huge := buildTimeline(t, f, LoopOnce)
	fired, ok := huge.Update(core.Milliseconds(3600000))
	if !ok || len(fired) != eventCount-1 {
		t.Errorf("once huge dt fired %d, want %d (At=0 excluded)", len(fired), eventCount-1)
	}
	hugeLoop := buildTimeline(t, f, LoopLoop)
	fired, ok = hugeLoop.Update(core.Milliseconds(3600000))
	if !ok || len(fired) != eventCount {
		t.Errorf("loop huge dt fired %d, want %d (each once)", len(fired), eventCount)
	}

	// Nil receivers never panic.
	var nilTL *Timeline
	if _, err := NewTimeline(0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero NewTimeline err = %v, want invalid-arg", err)
	}
	if nilTL.Duration() != 0 || nilTL.Loop() != LoopOnce || nilTL.Pos() != 0 {
		t.Error("nil accessors returned live values")
	}
	if nilTL.IsPlaying() || len(nilTL.TrackNames()) != 0 {
		t.Error("nil accessors returned live values")
	}
	if nilTL.KeyCount("x") != 0 || nilTL.EventCount() != 0 {
		t.Error("nil counters returned live values")
	}
	if v, ok := nilTL.Sample("x", 0); ok || v != 0 {
		t.Error("nil Sample ok=true, want false")
	}
	if _, ok := nilTL.Update(16); ok {
		t.Error("nil Update ok=true, want false")
	}
	nilTL.OnEvent(func(Event) {})
	nilTL.Pause()
	nilTL.Resume()
	if err := nilTL.AddKey("x", Key{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil AddKey err = %v, want invalid-arg", err)
	}
	if err := nilTL.AddEvent(0, "x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil AddEvent err = %v, want invalid-arg", err)
	}
	if err := nilTL.SetLoop(LoopOnce); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetLoop err = %v, want invalid-arg", err)
	}
	if err := nilTL.Seek(0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Seek err = %v, want invalid-arg", err)
	}
}

// C does not need pixels (pure math, draws nothing): the number path must
// be lossless and replay bitwise identical instead.
func TestTimelineBoundaryIdentical(t *testing.T) {
	f := loadTimelineCases(t)

	// Same dt stream replays samples and cues bitwise identically.
	replay := func() ([]float64, []Event) {
		tl := buildTimeline(t, f, LoopLoop)
		var vals []float64
		var evs []Event
		for i := 0; i < 500; i++ {
			fired, ok := tl.Update(core.Milliseconds(16))
			if !ok {
				t.Fatalf("replay step %d ok=false", i)
			}
			evs = append(evs, fired...)
			evs = append(evs, Event{At: -1, Name: "|"})
			for _, tr := range []string{"x", "alpha", "slash"} {
				v, ok := tl.Sample(tr, tl.Pos())
				if !ok {
					t.Fatalf("replay step %d sample %q ok=false", i, tr)
				}
				vals = append(vals, v)
			}
		}
		return vals, evs
	}
	aVals, aEvs := replay()
	bVals, bEvs := replay()
	for i := range aVals {
		if aVals[i] != bVals[i] {
			t.Fatalf("replay value diverged at %d: %.17g vs %.17g", i, aVals[i], bVals[i])
		}
	}
	if len(aEvs) != len(bEvs) {
		t.Fatalf("replay cues len %d vs %d", len(aEvs), len(bEvs))
	}
	for i := range aEvs {
		if aEvs[i] != bEvs[i] {
			t.Fatalf("replay cue diverged at %d: %+v vs %+v", i, aEvs[i], bEvs[i])
		}
	}

	// Sample is pure: same args read bitwise equal and move nothing.
	tl := buildTimeline(t, f, LoopOnce)
	pos := tl.Pos()
	for _, at := range []int64{0, 100, 250, 999, 1000, 5000} {
		a, oka := tl.Sample("x", core.Milliseconds(at))
		b, okb := tl.Sample("x", core.Milliseconds(at))
		if !oka || !okb || a != b {
			t.Fatalf("Sample pure diverged at %d: %v/%v vs %v/%v", at, a, oka, b, okb)
		}
	}
	if tl.Pos() != pos {
		t.Error("Sample moved the head")
	}

	// Timelines own their data: adding to one never changes a sibling.
	sib := buildTimeline(t, f, LoopOnce)
	if err := tl.AddKey("x", Key{At: core.Milliseconds(100), Value: 7, Ease: Linear}); err != nil {
		t.Fatalf("AddKey: %v", err)
	}
	if sib.KeyCount("x") != len(f.Tracks[0].Keys) {
		t.Error("AddKey leaked into a sibling timeline")
	}
}

// D: many tracks sample with a measured cost.
func TestTimelinePerfMulti(t *testing.T) {
	// Synthetic load only (no golden): golden order stays in
	// timeline_cases.json. Eight tracks share one finished shape.
	tl, err := NewTimeline(core.Milliseconds(1000))
	if err != nil {
		t.Fatalf("NewTimeline: %v", err)
	}
	if err := tl.SetLoop(LoopLoop); err != nil {
		t.Fatalf("SetLoop: %v", err)
	}
	tracks := []string{"x", "y", "rot", "scale", "alpha", "slash", "glow", "shake"}
	for _, tr := range tracks {
		for _, k := range []struct {
			at   int64
			v    float64
			ease string
		}{{0, 0, "linear"}, {250, 10, "in_quad"}, {500, 20, "out_quad"}, {750, 15, "in_out_sine"}, {1000, 30, "linear"}} {
			kind, _ := Parse(k.ease)
			if err := tl.AddKey(tr, Key{At: core.Milliseconds(k.at), Value: k.v, Ease: kind}); err != nil {
				t.Fatalf("AddKey %s: %v", tr, err)
			}
		}
	}
	for _, e := range []struct {
		at   int64
		name string
	}{{250, "slash_show"}, {500, "sfx_swing"}, {750, "call_spawn"}, {900, "hide"}} {
		if err := tl.AddEvent(core.Milliseconds(e.at), e.name); err != nil {
			t.Fatalf("AddEvent: %v", err)
		}
	}
	const rounds = 25000
	var acc float64
	start := time.Now()
	for r := 0; r < rounds; r++ {
		at := core.Milliseconds(int64(r % 1200))
		for _, tr := range tracks {
			v, ok := tl.Sample(tr, at)
			if !ok {
				t.Fatal("perf Sample ok=false")
			}
			acc += v
		}
	}
	el := time.Since(start)
	total := int64(rounds * len(tracks))
	t.Logf("timeline-%d: %d tracks x %d rounds (%d samples) in %v (%.1f ns/op)",
		len(tracks), len(tracks), rounds, total, el, float64(el.Nanoseconds())/float64(total))
	if math.IsNaN(acc) || math.IsInf(acc, 0) {
		t.Fatal("perf accumulation went non-finite, benchmark invalid")
	}
	if acc == 0 {
		t.Error("perf loop folded to zero, benchmark invalid")
	}
	// Updates advance with a measured cost too (cues armed, no handler).
	start = time.Now()
	for r := 0; r < rounds; r++ {
		if _, ok := tl.Update(core.Milliseconds(16)); !ok {
			t.Fatal("perf Update ok=false")
		}
	}
	el = time.Since(start)
	t.Logf("timeline-update: %d x 16ms updates in %v (%.1f ns/op)",
		rounds, el, float64(el.Nanoseconds())/float64(rounds))
}

// E: long runs neither drift nor diverge between replays.
func TestTimelineLongRunNoDrift(t *testing.T) {
	f := loadTimelineCases(t)
	mk := func(loop LoopMode) *Timeline { return buildTimeline(t, f, loop) }
	// Integer-ms ledger: 100k x 16ms = 1.6M ms = 1600 whole loops, so a
	// loop head must sit back on 0 with both replays bitwise identical.
	const steps = 100000
	const dt = int64(16)
	a, b := mk(LoopLoop), mk(LoopLoop)
	for i := 0; i < steps; i++ {
		fa, oka := a.Update(core.Milliseconds(dt))
		fb, okb := b.Update(core.Milliseconds(dt))
		if !oka || !okb {
			t.Fatalf("loop rep %d ok=false", i)
		}
		if !sameNames(eventNames(fa), eventNames(fb)) {
			t.Fatalf("loop rep %d cues diverged: %q vs %q", i, eventNames(fa), eventNames(fb))
		}
		if a.Pos() != b.Pos() {
			t.Fatalf("loop rep %d pos diverged: %d vs %d", i, a.Pos(), b.Pos())
		}
		for _, tr := range []string{"x", "alpha", "slash"} {
			va, _ := a.Sample(tr, a.Pos())
			vb, _ := b.Sample(tr, b.Pos())
			if va != vb {
				t.Fatalf("loop rep %d %s diverged: %.17g vs %.17g", i, tr, va, vb)
			}
		}
	}
	if pos := a.Pos().Milliseconds(); pos != 0 {
		t.Errorf("loop 100k x 16ms pos = %d, want 0", pos)
	}
	// Pingpong replays identically over the same long run.
	pa, pb := mk(LoopPingPong), mk(LoopPingPong)
	for i := 0; i < steps; i++ {
		fa, _ := pa.Update(core.Milliseconds(dt))
		fb, _ := pb.Update(core.Milliseconds(dt))
		if !sameNames(eventNames(fa), eventNames(fb)) || pa.Pos() != pb.Pos() {
			t.Fatalf("pingpong rep %d diverged", i)
		}
	}
	// A finished once timeline holds its end forever.
	j := mk(LoopOnce)
	for i := 0; i < 10000; i++ {
		if fired, ok := j.Update(core.Milliseconds(dt)); !ok || (i > 1000 && len(fired) != 0) {
			t.Fatalf("once rep %d = %v/%v, want silent hold", i, fired, ok)
		}
	}
	if pos := j.Pos().Milliseconds(); pos != f.DurationMs || j.IsPlaying() {
		t.Errorf("once long-run pos = %d playing %v, want %d/false",
			pos, j.IsPlaying(), f.DurationMs)
	}
	if v, _ := j.Sample("x", j.Pos()); v != 30 {
		t.Errorf("once long-run x = %v, want 30", v)
	}
}

// F: offscreen golden stands in for the window (window-exempt pure math
// until the game_anim --case=tl window lands with P2). The frozen numbers
// in timeline_cases.json are the evidence both backends share; shape
// assertions below pin the meaning, not just the numbers.
func TestTimelineOffscreenGolden(t *testing.T) {
	f := loadTimelineCases(t)
	once := mustFindSeq(t, f, "once_attack")
	wrap := mustFindSeq(t, f, "loop_wrap")
	bounce := mustFindSeq(t, f, "pingpong_bounce")

	// Key hits land exactly: every step on a key boundary holds its value.
	if once.Steps[0].Want["x"] != 10 || once.Steps[1].Want["x"] != 20 ||
		once.Steps[2].Want["x"] != 15 || once.Steps[4].Want["x"] != 30 {
		t.Errorf("once key hits drifted: %+v", once.Steps)
	}
	// Hand-checkable linear midpoints pin the interpolation wiring.
	if once.Steps[0].Want["alpha"] != 0.5 || once.Steps[2].Want["alpha"] != 0.5 {
		t.Errorf("alpha midpoints drifted: %+v", once.Steps)
	}
	if bounce.Steps[1].Want["alpha"] != 0.2 || bounce.Steps[1].Want["slash"] != 0.36 ||
		bounce.Steps[1].Want["x"] != 16.8 {
		t.Errorf("bounce pos 600 drifted: %+v", bounce.Steps[1].Want)
	}
	// At=0 fires only on wrap, never on the first forward pass.
	if len(once.Steps[0].WantEvents) != 1 || once.Steps[0].WantEvents[0] != "slash_show" {
		t.Errorf("first pass fired At=0: %q", once.Steps[0].WantEvents)
	}
	if len(wrap.Steps[1].WantEvents) == 0 || wrap.Steps[1].WantEvents[0] != "exit" ||
		wrap.Steps[1].WantEvents[1] != "enter" {
		t.Errorf("wrap order drifted: %q", wrap.Steps[1].WantEvents)
	}
	// Once holds the end and reports nothing more.
	if last := once.Steps[len(once.Steps)-1]; last.WantPos != f.DurationMs || len(last.WantEvents) != 0 {
		t.Errorf("once tail drifted: %+v", last)
	}
	// Loop wraps back onto the readable start; pingpong mirrors it.
	tl := buildTimeline(t, f, LoopLoop)
	a, _ := tl.Sample("x", core.Milliseconds(1100))
	b, _ := tl.Sample("x", core.Milliseconds(100))
	if a != b {
		t.Errorf("loop wrap reads diverged: %.17g vs %.17g", a, b)
	}
	tl.SetLoop(LoopPingPong)
	a, _ = tl.Sample("x", core.Milliseconds(1100))
	b, _ = tl.Sample("x", core.Milliseconds(900))
	if a != b {
		t.Errorf("pingpong mirror reads diverged: %.17g vs %.17g", a, b)
	}
	// Golden cues arrive in time order on the forward pass.
	wantOrder := []string{"slash_show", "sfx_swing", "call_spawn", "hide", "exit"}
	var gotOrder []string
	for _, st := range once.Steps {
		gotOrder = append(gotOrder, st.WantEvents...)
	}
	if !sameNames(gotOrder, wantOrder) {
		t.Errorf("once cue order = %q, want %q", gotOrder, wantOrder)
	}
	// Golden values always sit between their neighbor keys.
	bounds := map[string][2]float64{"x": {0, 30}, "alpha": {0, 1}, "slash": {0, 1}}
	for _, seq := range f.Sequences {
		for i, st := range seq.Steps {
			for tr, bd := range bounds {
				if v := st.Want[tr]; v < bd[0]-epsTimeline || v > bd[1]+epsTimeline {
					t.Errorf("%s step %d: %s = %v outside [%v,%v]",
						seq.Name, i, tr, v, bd[0], bd[1])
				}
			}
		}
	}
}
