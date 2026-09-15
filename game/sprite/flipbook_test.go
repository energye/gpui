package sprite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type flipClipDef struct {
	Name    string `json:"name"`
	Frames  []int  `json:"frames"`
	FrameMs int64  `json:"frame_ms"`
	Loop    string `json:"loop"`
}

type flipStepDef struct {
	DtMs      int64  `json:"dt_ms"`
	Play      string `json:"play"`
	WantFrame int    `json:"want_frame"`
	WantIndex int    `json:"want_index"`
	WantKind  string `json:"want_kind"`
}

type flipSeqDef struct {
	Name  string        `json:"name"`
	Clip  string        `json:"clip"`
	Steps []flipStepDef `json:"steps"`
}

type flipFile struct {
	Clips     []flipClipDef `json:"clips"`
	Sequences []flipSeqDef  `json:"sequences"`
}

func loadFlipbookCases(t *testing.T) flipFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "flipbook_cases.json"))
	if err != nil {
		t.Fatalf("read flipbook_cases.json: %v", err)
	}
	var f flipFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode flipbook_cases.json: %v", err)
	}
	if len(f.Clips) == 0 || len(f.Sequences) == 0 {
		t.Fatal("flipbook_cases.json has no clips or sequences")
	}
	return f
}

func parseLoop(t *testing.T, s string) LoopMode {
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

func parseKind(t *testing.T, s string) EventKind {
	t.Helper()
	switch s {
	case "none":
		return EventNone
	case "frame":
		return EventFrame
	case "loop":
		return EventLoop
	case "finished":
		return EventFinished
	}
	t.Fatalf("unknown kind %q", s)
	return EventNone
}

func buildFlipbook(t *testing.T, f flipFile) *Flipbook {
	t.Helper()
	fb := NewFlipbook()
	for _, c := range f.Clips {
		// No copy here: AddClip already copies the frame list.
		err := fb.AddClip(Clip{
			Name:     c.Name,
			Frames:   c.Frames,
			FrameDur: core.Milliseconds(c.FrameMs),
			Loop:     parseLoop(t, c.Loop),
		})
		if err != nil {
			t.Fatalf("AddClip %q: %v", c.Name, err)
		}
	}
	if got := fb.Count(); got != len(f.Clips) {
		t.Fatalf("Count = %d, want %d", got, len(f.Clips))
	}
	return fb
}

func mustFindClip(t *testing.T, f flipFile, name string) flipClipDef {
	t.Helper()
	for _, c := range f.Clips {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("flipbook_cases.json has no clip %q", name)
	return flipClipDef{}
}

func mustFindSeq(t *testing.T, f flipFile, name string) flipSeqDef {
	t.Helper()
	for _, s := range f.Sequences {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("flipbook_cases.json has no sequence %q", name)
	return flipSeqDef{}
}

// A: frame number, duration, loop, callback, and multi-action switches
// all land on the frozen sequences.
func TestFlipbookSequencesFromCases(t *testing.T) {
	f := loadFlipbookCases(t)
	for _, seq := range f.Sequences {
		fb := buildFlipbook(t, f)
		if err := fb.Play(seq.Clip); err != nil {
			t.Errorf("%s: Play %q: %v", seq.Name, seq.Clip, err)
			continue
		}
		var gotEvents []Event
		fb.OnEvent(func(ev Event) { gotEvents = append(gotEvents, ev) })
		wantClip := seq.Clip
		for i, st := range seq.Steps {
			gotEvents = nil
			if st.Play != "" {
				if err := fb.Play(st.Play); err != nil {
					t.Errorf("%s step %d: Play %q: %v", seq.Name, i, st.Play, err)
					continue
				}
				wantClip = st.Play
				// Play rewinds silently: no crossing callback.
				if len(gotEvents) != 0 {
					t.Errorf("%s step %d: Play emitted %d events, want 0", seq.Name, i, len(gotEvents))
				}
				clip, idx, frame, ok := fb.Current()
				if !ok || clip != st.Play || idx != st.WantIndex || frame != st.WantFrame {
					t.Errorf("%s step %d: after Play current = %q/%d/%d ok=%v, want %q/%d/%d",
						seq.Name, i, clip, idx, frame, ok, st.Play, st.WantIndex, st.WantFrame)
				}
				continue
			}
			wantKind := parseKind(t, st.WantKind)
			frame, ev, ok := fb.Update(core.Milliseconds(st.DtMs))
			if !ok {
				t.Errorf("%s step %d: Update ok=false, want true", seq.Name, i)
				continue
			}
			if frame != st.WantFrame || ev.Frame != st.WantFrame ||
				ev.Index != st.WantIndex || ev.Kind != wantKind {
				t.Errorf("%s step %d: Update = frame %d ev %+v, want frame %d index %d kind %s",
					seq.Name, i, frame, ev, st.WantFrame, st.WantIndex, st.WantKind)
			}
			if ev.Clip != wantClip {
				t.Errorf("%s step %d: event clip = %q, want %q", seq.Name, i, ev.Clip, wantClip)
			}
			clip, idx, curFrame, ok := fb.Current()
			if !ok || clip != wantClip || idx != st.WantIndex || curFrame != st.WantFrame {
				t.Errorf("%s step %d: Current = %q/%d/%d ok=%v, want %q index %d frame %d",
					seq.Name, i, clip, idx, curFrame, ok, wantClip, st.WantIndex, st.WantFrame)
			}
			// Callback mirrors the returned event exactly once.
			if wantKind == EventNone {
				if len(gotEvents) != 0 {
					t.Errorf("%s step %d: kind none but callback fired %+v", seq.Name, i, gotEvents)
				}
			} else {
				if len(gotEvents) != 1 {
					t.Errorf("%s step %d: kind %s but callback fired %d times", seq.Name, i, st.WantKind, len(gotEvents))
				} else if gotEvents[0] != ev {
					t.Errorf("%s step %d: callback %+v != returned %+v", seq.Name, i, gotEvents[0], ev)
				}
			}
		}
	}
}

// B: empty, zero, oversized, and corrupt inputs error without crashing.
func TestFlipbookEdgesNoCrash(t *testing.T) {
	f := loadFlipbookCases(t)
	fb := buildFlipbook(t, f)

	// Bad clips store nothing and report invalid-arg.
	bad := []Clip{
		{Name: "", Frames: []int{0}, FrameDur: core.Milliseconds(100), Loop: LoopLoop},
		{Name: "empty", Frames: nil, FrameDur: core.Milliseconds(100), Loop: LoopLoop},
		{Name: "negframe", Frames: []int{0, -1}, FrameDur: core.Milliseconds(100), Loop: LoopLoop},
		{Name: "zerodur", Frames: []int{0}, FrameDur: 0, Loop: LoopLoop},
		{Name: "negdur", Frames: []int{0}, FrameDur: core.Milliseconds(-5), Loop: LoopLoop},
		{Name: "badloop", Frames: []int{0}, FrameDur: core.Milliseconds(100), Loop: LoopMode(99)},
		{Name: "idle", Frames: []int{9}, FrameDur: core.Milliseconds(100), Loop: LoopLoop},
	}
	before := fb.Count()
	for i, c := range bad {
		if err := fb.AddClip(c); err == nil {
			t.Errorf("bad clip[%d] %+v: want error", i, c)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad clip[%d]: code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	if fb.Count() != before {
		t.Errorf("bad AddClip changed count to %d, want %d", fb.Count(), before)
	}
	if err := (*Flipbook)(nil).AddClip(Clip{Name: "x", Frames: []int{0}, FrameDur: core.Milliseconds(1), Loop: LoopLoop}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil AddClip err = %v, want invalid-arg", err)
	}

	// Unknown and empty plays keep the current action untouched.
	if err := fb.Play("idle"); err != nil {
		t.Fatalf("Play idle: %v", err)
	}
	if _, ev, _ := fb.Update(core.Milliseconds(200)); ev.Kind == EventNone {
		t.Fatalf("setup advance produced no event, test invalid")
	}
	clip, idx, frame, _ := fb.Current()
	playing := fb.IsPlaying()
	if err := fb.Play("no-such-action"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown Play err = %v, want not-found", err)
	}
	if err := fb.Play(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Play err = %v, want invalid-arg", err)
	}
	if c2, i2, f2, _ := fb.Current(); c2 != clip || i2 != idx || f2 != frame {
		t.Errorf("bad Play moved current to %q/%d/%d, want %q/%d/%d", c2, i2, f2, clip, idx, frame)
	}
	if fb.IsPlaying() != playing {
		t.Error("bad Play changed playing state")
	}
	if err := (*Flipbook)(nil).Play("idle"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Play err = %v, want invalid-arg", err)
	}

	// No current action: Update reports ok=false, never a frame.
	fresh := buildFlipbook(t, f)
	if _, _, ok := fresh.Update(core.Milliseconds(16)); ok {
		t.Error("Update before Play ok=true, want false")
	}
	if _, _, _, ok := fresh.Current(); ok {
		t.Error("Current before Play ok=true, want false")
	}
	if fresh.IsPlaying() {
		t.Error("IsPlaying before Play = true, want false")
	}

	// Zero and negative dt advance nothing and emit nothing.
	var calls int
	fb.OnEvent(func(Event) { calls++ })
	clip, idx, frame, _ = fb.Current()
	for _, dt := range []core.Duration{0, core.Milliseconds(-16)} {
		calls = 0
		got, ev, ok := fb.Update(dt)
		if !ok || got != frame || ev.Kind != EventNone || ev.Index != idx {
			t.Errorf("dt %d: Update = %d %+v ok=%v, want frame %d index %d none",
				dt.Milliseconds(), got, ev, ok, frame, idx)
		}
		if calls != 0 {
			t.Errorf("dt %d: callback fired %d times, want 0", dt.Milliseconds(), calls)
		}
	}
	// Nil handler clears the callback without breaking playback.
	fb.OnEvent(nil)
	if _, ev, ok := fb.Update(core.Milliseconds(100)); !ok || ev.Kind == EventNone {
		t.Errorf("nil-handler Update = %+v ok=%v, want a crossing", ev, ok)
	}

	// Huge dt coalesces to one event and stays in range.
	huge, ev, ok := fb.Update(core.Milliseconds(3600000))
	if !ok {
		t.Fatal("huge dt ok=false, want true")
	}
	if ev.Kind != EventLoop && ev.Kind != EventFinished {
		t.Errorf("huge dt kind = %s, want loop or finished", ev.Kind)
	}
	if _, idx, frame, ok := fb.Current(); !ok || idx != ev.Index || frame != huge {
		t.Errorf("huge dt current diverges from returned event")
	}

	// Nil receivers never panic.
	var nilFB *Flipbook
	if nilFB.Has("idle") || nilFB.Count() != 0 || nilFB.IsPlaying() {
		t.Error("nil accessors returned live values")
	}
	if _, _, _, ok := nilFB.Current(); ok {
		t.Error("nil Current ok=true, want false")
	}
	if _, _, ok := nilFB.Update(core.Milliseconds(16)); ok {
		t.Error("nil Update ok=true, want false")
	}
	nilFB.OnEvent(func(Event) {})
}

// C does not need pixels (pure math, draws nothing): the library copy
// must be lossless and replays bitwise identical instead.
func TestFlipbookBoundaryIdentical(t *testing.T) {
	f := loadFlipbookCases(t)

	// AddClip copies the frame list: later writes cannot change the library.
	src := []int{40, 41, 42}
	fb := NewFlipbook()
	if err := fb.AddClip(Clip{Name: "copy", Frames: src, FrameDur: core.Milliseconds(50), Loop: LoopLoop}); err != nil {
		t.Fatalf("AddClip copy: %v", err)
	}
	src[0] = 999
	if err := fb.Play("copy"); err != nil {
		t.Fatalf("Play copy: %v", err)
	}
	if _, _, frame, _ := fb.Current(); frame != 40 {
		t.Errorf("library aliased caller slice: first frame = %d, want 40", frame)
	}

	// Same dt stream replays frame-for-frame and event-for-event.
	replay := func() ([]int, []Event) {
		fb := buildFlipbook(t, f)
		if err := fb.Play("ping"); err != nil {
			t.Fatalf("Play ping: %v", err)
		}
		var frames []int
		var evs []Event
		for i := 0; i < 200; i++ {
			fr, ev, ok := fb.Update(core.Milliseconds(16))
			if !ok {
				t.Fatalf("replay step %d ok=false", i)
			}
			frames = append(frames, fr)
			evs = append(evs, ev)
		}
		return frames, evs
	}
	aFrames, aEvs := replay()
	bFrames, bEvs := replay()
	for i := range aFrames {
		if aFrames[i] != bFrames[i] || aEvs[i] != bEvs[i] {
			t.Fatalf("replay diverged at step %d: %d/%v vs %d/%v",
				i, aFrames[i], aEvs[i], bFrames[i], bEvs[i])
		}
	}
}

// D: many concurrent flipbooks advance with a measured cost.
func TestFlipbookPerfMulti(t *testing.T) {
	f := loadFlipbookCases(t)
	idle := mustFindClip(t, f, "idle")
	// Synthetic load only (no golden): golden order stays in
	// flipbook_cases.json. One frozen clip shared by N players.
	const n = 1000
	players := make([]*Flipbook, n)
	for i := range players {
		players[i] = NewFlipbook()
		if err := players[i].AddClip(Clip{
			Name:     idle.Name,
			Frames:   idle.Frames,
			FrameDur: core.Milliseconds(idle.FrameMs),
			Loop:     parseLoop(t, idle.Loop),
		}); err != nil {
			t.Fatalf("player %d AddClip: %v", i, err)
		}
		if err := players[i].Play(idle.Name); err != nil {
			t.Fatalf("player %d Play: %v", i, err)
		}
	}
	const reps = 200
	start := time.Now()
	for r := 0; r < reps; r++ {
		for _, p := range players {
			if _, _, ok := p.Update(core.Milliseconds(16)); !ok {
				t.Fatal("perf Update ok=false")
			}
		}
	}
	el := time.Since(start)
	total := int64(n * reps)
	t.Logf("flipbook-%d: %d players x %d updates (%d total) in %v (%.1f ns/op)",
		n, n, reps, total, el, float64(el.Nanoseconds())/float64(total))
}

// E: long runs neither drift nor diverge between replays.
func TestFlipbookLongRunNoDrift(t *testing.T) {
	f := loadFlipbookCases(t)
	mk := func(clip string) *Flipbook {
		fb := buildFlipbook(t, f)
		if err := fb.Play(clip); err != nil {
			t.Fatalf("Play %q: %v", clip, err)
		}
		return fb
	}
	// Integer-ms ledger: 100k x 16ms = 1.6M ms = 16000 idle frames,
	// a whole number of 4-frame loops, so idle must sit back on frame 0
	// with no remainder and both replays bitwise identical.
	const steps = 100000
	const dt = int64(16)
	a, b := mk("idle"), mk("idle")
	for i := 0; i < steps; i++ {
		fa, ea, oka := a.Update(core.Milliseconds(dt))
		fb, eb, okb := b.Update(core.Milliseconds(dt))
		if !oka || !okb || fa != fb || ea != eb {
			t.Fatalf("idle rep %d diverged: %d/%v vs %d/%v", i, fa, ea, fb, eb)
		}
	}
	if _, idx, frame, _ := a.Current(); idx != 0 || frame != 0 {
		t.Errorf("idle 100k x 16ms = %d/%d, want 0/0", idx, frame)
	}
	// Pingpong replays identically over the same long run.
	pa, pb := mk("ping"), mk("ping")
	for i := 0; i < steps; i++ {
		fa, ea, _ := pa.Update(core.Milliseconds(dt))
		fb, eb, _ := pb.Update(core.Milliseconds(dt))
		if fa != fb || ea != eb {
			t.Fatalf("ping rep %d diverged: %d/%v vs %d/%v", i, fa, ea, fb, eb)
		}
	}
	if _, _, fa, _ := pa.Current(); fa < 30 || fa > 33 {
		t.Errorf("ping long-run frame = %d, want within [30,33]", fa)
	}
	// A finished once clip holds its last frame forever.
	j := mk("jump")
	for i := 0; i < 10000; i++ {
		if _, _, ok := j.Update(core.Milliseconds(dt)); !ok {
			t.Fatalf("jump rep %d ok=false", i)
		}
	}
	if _, idx, frame, _ := j.Current(); idx != 2 || frame != 22 || j.IsPlaying() {
		t.Errorf("jump long-run = idx %d frame %d playing %v, want 2/22/false",
			idx, frame, j.IsPlaying())
	}
}

// F: offscreen golden stands in for the window (window-exempt pure math).
// The frozen frames in flipbook_cases.json are the evidence both backends
// share; shape assertions below pin the meaning, not just the numbers.
func TestFlipbookOffscreenGolden(t *testing.T) {
	f := loadFlipbookCases(t)
	idle := mustFindSeq(t, f, "idle_loop")
	jump := mustFindSeq(t, f, "jump_once")
	ping := mustFindSeq(t, f, "ping_bounce")
	sw := mustFindSeq(t, f, "switch_idle_run")
	// Loop wraps back to the first frame with a loop event.
	if last := idle.Steps[4]; last.WantFrame != 0 || last.WantKind != "loop" {
		t.Errorf("idle wrap = %+v, want frame 0 loop", last)
	}
	// Once holds the last frame and reports finished exactly once.
	finished := 0
	for _, st := range jump.Steps {
		if st.WantKind == "finished" {
			finished++
			if st.WantFrame != 22 || st.WantIndex != 2 {
				t.Errorf("jump finish = %+v, want frame 22 index 2", st)
			}
		}
	}
	if finished != 1 {
		t.Errorf("jump finished %d times, want exactly 1", finished)
	}
	if last := jump.Steps[len(jump.Steps)-1]; last.WantFrame != 22 || last.WantKind != "none" {
		t.Errorf("jump tail = %+v, want held frame 22 none", last)
	}
	// Pingpong hits both ends and leaves them with plain frames.
	if ping.Steps[2].WantFrame != 33 || ping.Steps[2].WantKind != "loop" {
		t.Errorf("ping far end = %+v, want frame 33 loop", ping.Steps[2])
	}
	if ping.Steps[5].WantFrame != 30 || ping.Steps[5].WantKind != "loop" {
		t.Errorf("ping near end = %+v, want frame 30 loop", ping.Steps[5])
	}
	if ping.Steps[3].WantFrame != 32 || ping.Steps[3].WantKind != "frame" {
		t.Errorf("ping leave end = %+v, want frame 32 frame", ping.Steps[3])
	}
	// Switching actions rewinds to the first frame.
	if sw.Steps[1].Play != "run" || sw.Steps[1].WantFrame != 10 {
		t.Errorf("switch to run = %+v, want frame 10", sw.Steps[1])
	}
	if sw.Steps[3].Play != "idle" || sw.Steps[3].WantFrame != 0 {
		t.Errorf("switch back = %+v, want frame 0", sw.Steps[3])
	}
	// Golden frame ids always belong to their clip.
	framesOf := map[string]map[int]bool{}
	for _, c := range f.Clips {
		m := map[int]bool{}
		for _, fr := range c.Frames {
			m[fr] = true
		}
		framesOf[c.Name] = m
	}
	for _, seq := range []flipSeqDef{idle, jump, ping} {
		clip := seq.Clip
		for i, st := range seq.Steps {
			if st.Play != "" {
				clip = st.Play
				continue
			}
			if !framesOf[clip][st.WantFrame] {
				t.Errorf("%s step %d: frame %d not in clip %q", seq.Name, i, st.WantFrame, clip)
			}
		}
	}
}
