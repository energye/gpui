package input

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type bufferPushDef struct {
	Action string `json:"action"`
	AtMs   int64  `json:"at_ms"`
}

type bufferStepDef struct {
	Op      string `json:"op"`
	Action  string `json:"action"`
	NowMs   int64  `json:"now_ms"`
	WantHit bool   `json:"want_hit"`
}

type bufferSeqDef struct {
	Name      string          `json:"name"`
	WindowMs  int64           `json:"window_ms"`
	Pushes    []bufferPushDef `json:"pushes"`
	Steps     []bufferStepDef `json:"steps"`
	CheckAtMs int64           `json:"check_at_ms"`
	WantLive  int             `json:"want_live"`
	WantLen   int             `json:"want_len"`
}

type touchOpDef struct {
	Op       string  `json:"op"`
	ID       int     `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	AtMs     int64   `json:"at_ms"`
	WantCode string  `json:"want_code"`
}

type touchWantDef struct {
	ID   int     `json:"id"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	AtMs int64   `json:"at_ms"`
}

type touchSeqDef struct {
	Name        string         `json:"name"`
	Ops         []touchOpDef   `json:"ops"`
	WantActive  []int          `json:"want_active"`
	WantTouches []touchWantDef `json:"want_touches"`
}

type bufferFile struct {
	Limits struct {
		MaxBuffered     int   `json:"max_buffered"`
		MaxTouches      int   `json:"max_touches"`
		DefaultWindowMs int64 `json:"default_window_ms"`
	} `json:"limits"`
	BufferSequences []bufferSeqDef `json:"buffer_sequences"`
	TouchSequences  []touchSeqDef  `json:"touch_sequences"`
	Perf            struct {
		BufferPushes int `json:"buffer_pushes"`
		Reps         int `json:"reps"`
		TouchReps    int `json:"touch_reps"`
	} `json:"perf"`
	Longrun struct {
		Reps int `json:"reps"`
	} `json:"longrun"`
}

func loadBufferCases(t *testing.T) bufferFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "buffer_cases.json"))
	if err != nil {
		t.Fatalf("read buffer_cases.json: %v", err)
	}
	var f bufferFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode buffer_cases.json: %v", err)
	}
	if len(f.BufferSequences) == 0 || len(f.TouchSequences) == 0 {
		t.Fatal("buffer_cases.json has no sequences")
	}
	return f
}

func mustBufferSeq(t *testing.T, f bufferFile, name string) bufferSeqDef {
	t.Helper()
	for _, s := range f.BufferSequences {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("buffer_cases.json has no buffer sequence %q", name)
	return bufferSeqDef{}
}

func mustTouchSeq(t *testing.T, f bufferFile, name string) touchSeqDef {
	t.Helper()
	for _, s := range f.TouchSequences {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("buffer_cases.json has no touch sequence %q", name)
	return touchSeqDef{}
}

func applyBufferPushes(t *testing.T, b *Buffer, pushes []bufferPushDef) {
	t.Helper()
	for i, p := range pushes {
		if err := b.Push(p.Action, core.Milliseconds(p.AtMs)); err != nil {
			t.Fatalf("push[%d] %+v: %v", i, p, err)
		}
	}
}

func checkBufferSteps(t *testing.T, b *Buffer, seq bufferSeqDef) {
	t.Helper()
	for i, s := range seq.Steps {
		switch s.Op {
		case "consume":
			got, err := b.Consume(s.Action, core.Milliseconds(s.NowMs))
			if err != nil {
				t.Fatalf("%s step[%d]: Consume: %v", seq.Name, i, err)
			}
			if got != s.WantHit {
				t.Errorf("%s step[%d] consume %q@%d = %v, want %v", seq.Name, i, s.Action, s.NowMs, got, s.WantHit)
			}
		case "peek":
			got, err := b.Peek(s.Action, core.Milliseconds(s.NowMs))
			if err != nil {
				t.Fatalf("%s step[%d]: Peek: %v", seq.Name, i, err)
			}
			if got != s.WantHit {
				t.Errorf("%s step[%d] peek %q@%d = %v, want %v", seq.Name, i, s.Action, s.NowMs, got, s.WantHit)
			}
		default:
			t.Fatalf("%s step[%d]: unknown op %q", seq.Name, i, s.Op)
		}
	}
}

func checkTouchLive(t *testing.T, tr *TouchTracker, seq touchSeqDef) {
	t.Helper()
	if got := tr.ActiveCount(); got != len(seq.WantActive) {
		t.Fatalf("%s: active = %d, want %d", seq.Name, got, len(seq.WantActive))
	}
	gotIDs := tr.ActiveIDs()
	if len(gotIDs) != len(seq.WantActive) {
		t.Fatalf("%s: ids = %v, want %v", seq.Name, gotIDs, seq.WantActive)
	}
	for i := range gotIDs {
		if gotIDs[i] != seq.WantActive[i] {
			t.Fatalf("%s: ids = %v, want %v", seq.Name, gotIDs, seq.WantActive)
		}
	}
	gotT := tr.Touches()
	if len(gotT) != len(seq.WantTouches) {
		t.Fatalf("%s: touches = %v, want %d entries", seq.Name, gotT, len(seq.WantTouches))
	}
	for i, w := range seq.WantTouches {
		if gotT[i].ID != w.ID || gotT[i].Pos.X != w.X || gotT[i].Pos.Y != w.Y || gotT[i].At != core.Milliseconds(w.AtMs) {
			t.Errorf("%s touch[%d] = %+v, want %+v", seq.Name, i, gotT[i], w)
		}
		if pos, ok := tr.Position(w.ID); !ok || pos.X != w.X || pos.Y != w.Y {
			t.Errorf("%s Position(%d) = %v/%v, want (%v,%v)/true", seq.Name, w.ID, pos, ok, w.X, w.Y)
		}
	}
}

func applyTouchOps(t *testing.T, tr *TouchTracker, ops []touchOpDef) {
	t.Helper()
	for i, o := range ops {
		var err error
		switch o.Op {
		case "begin":
			err = tr.Begin(o.ID, core.V2(o.X, o.Y), core.Milliseconds(o.AtMs))
		case "move":
			err = tr.Move(o.ID, core.V2(o.X, o.Y), core.Milliseconds(o.AtMs))
		case "end":
			err = tr.End(o.ID, core.Milliseconds(o.AtMs))
		default:
			t.Fatalf("op[%d]: unknown %q", i, o.Op)
		}
		if o.WantCode == "" && err != nil {
			t.Fatalf("op[%d] %+v: unexpected %v", i, o, err)
		}
		if o.WantCode != "" {
			if err == nil {
				t.Fatalf("op[%d] %+v: want %s error", i, o, o.WantCode)
			}
			if string(core.CodeOf(err).String()) != o.WantCode {
				t.Fatalf("op[%d] %+v code = %v, want %s", i, o, core.CodeOf(err), o.WantCode)
			}
		}
	}
}

// A: combo cache hits in order, multi-touch tracks without mixing ids.
func TestBufferComboFromCases(t *testing.T) {
	f := loadBufferCases(t)
	for _, seq := range f.BufferSequences {
		b, err := NewBuffer(core.Milliseconds(seq.WindowMs))
		if err != nil {
			t.Fatalf("%s: NewBuffer: %v", seq.Name, err)
		}
		if b.Window() != core.Milliseconds(seq.WindowMs) {
			t.Fatalf("%s: window = %v, want %v", seq.Name, b.Window(), seq.WindowMs)
		}
		applyBufferPushes(t, b, seq.Pushes)
		if b.Len() != len(seq.Pushes) {
			t.Fatalf("%s: len = %d, want %d", seq.Name, b.Len(), len(seq.Pushes))
		}
		checkBufferSteps(t, b, seq)
		if got := b.LiveCount(core.Milliseconds(seq.CheckAtMs)); got != seq.WantLive {
			t.Errorf("%s: live@%d = %d, want %d", seq.Name, seq.CheckAtMs, got, seq.WantLive)
		}
		if got := b.Len(); got != seq.WantLen {
			t.Errorf("%s: len = %d, want %d", seq.Name, got, seq.WantLen)
		}
	}
	for _, name := range []string{"multi_track", "swap_track"} {
		seq := mustTouchSeq(t, f, name)
		tr := NewTouchTracker()
		applyTouchOps(t, tr, seq.Ops)
		checkTouchLive(t, tr, seq)
	}
	// Presses copy cannot change the buffer; ActiveIDs copy cannot
	// change the tracker.
	seq := mustBufferSeq(t, f, "mixed_order")
	b, _ := NewBuffer(core.Milliseconds(seq.WindowMs))
	applyBufferPushes(t, b, seq.Pushes)
	cp := b.Presses()
	if len(cp) != len(seq.Pushes) {
		t.Fatalf("Presses len = %d, want %d", len(cp), len(seq.Pushes))
	}
	cp[0] = BufferedPress{}
	if again := b.Presses(); again[0] == (BufferedPress{}) {
		t.Error("Presses aliases the buffer, want a copy")
	}
	ts := mustTouchSeq(t, f, "multi_track")
	tr := NewTouchTracker()
	applyTouchOps(t, tr, ts.Ops)
	ids := tr.ActiveIDs()
	if len(ids) == 0 {
		t.Fatal("ActiveIDs empty, want live touch")
	}
	ids[0] = -999
	if again := tr.ActiveIDs(); again[0] == -999 {
		t.Error("ActiveIDs aliases the tracker, want a copy")
	}
	ts2 := tr.Touches()
	ts2[0].Pos = core.V2(999, 999)
	if pos, _ := tr.Position(ts.WantTouches[0].ID); pos == (core.Vec2{X: 999, Y: 999}) {
		t.Error("Touches aliases the tracker, want a copy")
	}
}

// B: broken touches and bad presses error without crashing.
func TestBufferEdgesNoCrash(t *testing.T) {
	f := loadBufferCases(t)
	// Bad windows never build.
	for _, w := range []int64{0, -1, -150} {
		if b, err := NewBuffer(core.Milliseconds(w)); err == nil || b != nil {
			t.Errorf("NewBuffer(%d) = %v/%v, want nil + invalid-arg", w, b, err)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("NewBuffer(%d) code = %v, want invalid-arg", w, core.CodeOf(err))
		}
	}
	b, err := NewBuffer(core.Milliseconds(mustBufferSeq(t, f, "combo_hit").WindowMs))
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	// Bad pushes store nothing.
	before := b.Len()
	for _, p := range []struct {
		action string
		at     int64
	}{
		{"", 0}, {"", 10}, {"jump", -1}, {"attack", -100},
	} {
		if err := b.Push(p.action, core.Milliseconds(p.at)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Push(%q,%d) err = %v, want invalid-arg", p.action, p.at, err)
		}
	}
	if b.Len() != before {
		t.Error("bad Push changed len, want untouched")
	}
	if err := b.Push("jump", core.Milliseconds(0)); err != nil {
		t.Fatalf("Push: %v", err)
	}
	// Bad consume/peek report invalid-arg and change nothing.
	for _, now := range []int64{-1, -50} {
		if _, err := b.Consume("jump", core.Milliseconds(now)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Consume now %d err = %v, want invalid-arg", now, err)
		}
		if _, err := b.Peek("jump", core.Milliseconds(now)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Peek now %d err = %v, want invalid-arg", now, err)
		}
	}
	if _, err := b.Consume("", core.Milliseconds(10)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Consume empty err = %v, want invalid-arg", err)
	}
	if _, err := b.Peek("", core.Milliseconds(10)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Peek empty err = %v, want invalid-arg", err)
	}
	// Bad window retunes keep the old window.
	oldWin := b.Window()
	for _, w := range []int64{0, -10} {
		if err := b.SetWindow(core.Milliseconds(w)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("SetWindow(%d) err = %v, want invalid-arg", w, err)
		}
	}
	if b.Window() != oldWin {
		t.Error("bad SetWindow changed window, want untouched")
	}
	// Prune with negative time and Clear keep working.
	b.Prune(core.Milliseconds(-5))
	if b.Len() == 0 {
		t.Error("negative Prune dropped presses, want untouched")
	}
	// Broken touch ids from the file plus hardcoded bad shapes.
	broken := mustTouchSeq(t, f, "broken_touches")
	tr := NewTouchTracker()
	applyTouchOps(t, tr, broken.Ops)
	checkTouchLive(t, tr, broken)
	if tr.ActiveCount() != 0 {
		t.Errorf("broken active = %d, want 0", tr.ActiveCount())
	}
	tr2 := NewTouchTracker()
	for _, id := range []int{-1, -5} {
		if err := tr2.Begin(id, core.V2(1, 1), 0); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Begin(%d) err = %v, want invalid-arg", id, err)
		}
		if err := tr2.Move(id, core.V2(1, 1), 0); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Move(%d) err = %v, want invalid-arg", id, err)
		}
		if err := tr2.End(id, 0); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("End(%d) err = %v, want invalid-arg", id, err)
		}
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := tr2.Begin(1, core.V2(v, 0), 0); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Begin(NaN/Inf x) err = %v, want invalid-arg", err)
		}
		if err := tr2.Begin(1, core.V2(0, v), 0); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Begin(NaN/Inf y) err = %v, want invalid-arg", err)
		}
		if err := tr2.Move(0, core.V2(v, 0), 0); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Move(NaN/Inf) err = %v, want invalid-arg", err)
		}
	}
	if err := tr2.Begin(0, core.V2(1, 1), core.Milliseconds(-1)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Begin negative at err = %v, want invalid-arg", err)
	}
	if tr2.ActiveCount() != 0 {
		t.Error("bad touch Begin changed live set, want untouched")
	}
	if err := tr2.Begin(5, core.V2(1, 1), 0); err != nil {
		t.Fatalf("Begin 5: %v", err)
	}
	if err := tr2.Begin(5, core.V2(2, 2), 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("duplicate Begin err = %v, want invalid-arg", err)
	}
	if pos, ok := tr2.Position(5); !ok || pos != (core.Vec2{X: 1, Y: 1}) {
		t.Errorf("duplicate Begin moved touch to %v, want (1,1)", pos)
	}
	if err := tr2.Move(99, core.V2(1, 1), 0); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("Move unknown err = %v, want not-found", err)
	}
	if err := tr2.End(99, 0); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("End unknown err = %v, want not-found", err)
	}
	// Full tracker refuses one more with out-of-memory, keeps the set.
	full := NewTouchTracker()
	for id := 0; id < MaxTouches; id++ {
		if err := full.Begin(id, core.V2(float64(id), 0), core.Milliseconds(int64(id))); err != nil {
			t.Fatalf("fill Begin(%d): %v", id, err)
		}
	}
	if full.ActiveCount() != MaxTouches {
		t.Fatalf("fill active = %d, want %d", full.ActiveCount(), MaxTouches)
	}
	if err := full.Begin(MaxTouches, core.V2(0, 0), 0); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("overfill err = %v, want out-of-memory", err)
	}
	if full.ActiveCount() != MaxTouches {
		t.Error("overfill changed live set, want untouched")
	}
	// Full buffer drops the oldest and stays bounded, never crashes.
	// Synthetic load only (no golden): golden presses stay in the file.
	big, _ := NewBuffer(DefaultBufferWindow)
	for i := 0; i < MaxBuffered+5; i++ {
		if err := big.Push("spam", core.Milliseconds(int64(i))); err != nil {
			t.Fatalf("overflow push %d: %v", i, err)
		}
	}
	if big.Len() != MaxBuffered {
		t.Errorf("overflow len = %d, want %d", big.Len(), MaxBuffered)
	}
	if got := big.Presses()[0].At; got != core.Milliseconds(5) {
		t.Errorf("overflow oldest = %v, want 5ms (drop oldest)", got)
	}
	// Huge-but-finite times store and consume without NaN.
	huge, _ := NewBuffer(core.Milliseconds(150))
	if err := huge.Push("jump", core.Milliseconds(1000000000000)); err != nil {
		t.Errorf("huge push: %v", err)
	}
	if hit, _ := huge.Consume("jump", core.Milliseconds(1000000000100)); !hit {
		t.Error("huge consume = false, want true within window")
	}
	// Nil receivers never panic.
	var nilBuf *Buffer
	if nilBuf.Window() != 0 || nilBuf.Len() != 0 || nilBuf.LiveCount(10) != 0 {
		t.Error("nil buffer accessors returned live values")
	}
	if nilBuf.Presses() != nil {
		t.Error("nil Presses != nil")
	}
	for _, err := range []error{
		nilBuf.Push("x", 0), nilBuf.SetWindow(10),
		func() error { _, e := nilBuf.Consume("x", 0); return e }(),
		func() error { _, e := nilBuf.Peek("x", 0); return e }(),
	} {
		if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("nil buffer mutator err = %v, want invalid-arg", err)
		}
	}
	nilBuf.Prune(10)
	nilBuf.Clear()
	var nilTr *TouchTracker
	if nilTr.ActiveCount() != 0 || nilTr.ActiveIDs() != nil || nilTr.Touches() != nil {
		t.Error("nil tracker accessors returned live values")
	}
	if _, ok := nilTr.Position(0); ok {
		t.Error("nil Position ok=true, want false")
	}
	for _, err := range []error{
		nilTr.Begin(0, core.V2(0, 0), 0),
		nilTr.Move(0, core.V2(0, 0), 0),
		nilTr.End(0, 0),
	} {
		if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("nil tracker mutator err = %v, want invalid-arg", err)
		}
	}
	nilTr.Clear()
	if _, ok := tr2.Position(9999); ok {
		t.Error("Position unknown ok=true, want false")
	}
}

// C does not need pixels (pure math, draws nothing): the press path must
// be lossless and replay bitwise identical instead.
func TestBufferBoundaryIdentical(t *testing.T) {
	f := loadBufferCases(t)
	for _, seq := range f.BufferSequences {
		a, _ := NewBuffer(core.Milliseconds(seq.WindowMs))
		b, _ := NewBuffer(core.Milliseconds(seq.WindowMs))
		applyBufferPushes(t, a, seq.Pushes)
		applyBufferPushes(t, b, seq.Pushes)
		pa, pb := a.Presses(), b.Presses()
		if len(pa) != len(pb) {
			t.Fatalf("%s: presses len %d vs %d", seq.Name, len(pa), len(pb))
		}
		for i := range pa {
			if pa[i] != pb[i] {
				t.Fatalf("%s: press[%d] diverged: %+v vs %+v", seq.Name, i, pa[i], pb[i])
			}
		}
		for i, s := range seq.Steps {
			var ha, hb bool
			var err error
			if s.Op == "consume" {
				ha, err = a.Consume(s.Action, core.Milliseconds(s.NowMs))
				if err != nil {
					t.Fatalf("%s step[%d]: %v", seq.Name, i, err)
				}
				hb, err = b.Consume(s.Action, core.Milliseconds(s.NowMs))
				if err != nil {
					t.Fatalf("%s step[%d]: %v", seq.Name, i, err)
				}
			} else {
				ha, err = a.Peek(s.Action, core.Milliseconds(s.NowMs))
				if err != nil {
					t.Fatalf("%s step[%d]: %v", seq.Name, i, err)
				}
				hb, err = b.Peek(s.Action, core.Milliseconds(s.NowMs))
				if err != nil {
					t.Fatalf("%s step[%d]: %v", seq.Name, i, err)
				}
			}
			if ha != hb || ha != s.WantHit {
				t.Fatalf("%s step[%d] replay %v/%v, want %v", seq.Name, i, ha, hb, s.WantHit)
			}
		}
		if a.LiveCount(core.Milliseconds(seq.CheckAtMs)) != b.LiveCount(core.Milliseconds(seq.CheckAtMs)) {
			t.Fatalf("%s: live replay diverged", seq.Name)
		}
		if a.Len() != b.Len() {
			t.Fatalf("%s: len replay diverged: %d vs %d", seq.Name, a.Len(), b.Len())
		}
	}
	for _, seq := range f.TouchSequences {
		a, b := NewTouchTracker(), NewTouchTracker()
		applyTouchOps(t, a, seq.Ops)
		applyTouchOps(t, b, seq.Ops)
		aid, bid := a.ActiveIDs(), b.ActiveIDs()
		if len(aid) != len(bid) {
			t.Fatalf("%s: ids len %d vs %d", seq.Name, len(aid), len(bid))
		}
		for i := range aid {
			if aid[i] != bid[i] {
				t.Fatalf("%s: id[%d] diverged", seq.Name, i)
			}
		}
		at, bt := a.Touches(), b.Touches()
		if len(at) != len(bt) {
			t.Fatalf("%s: touches len %d vs %d", seq.Name, len(at), len(bt))
		}
		for i := range at {
			if at[i] != bt[i] {
				t.Fatalf("%s: touch[%d] diverged: %+v vs %+v", seq.Name, i, at[i], bt[i])
			}
		}
	}
	// Window retunes round-trip without touching presses.
	b, _ := NewBuffer(core.Milliseconds(100))
	if err := b.Push("jump", 0); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if err := b.SetWindow(core.Milliseconds(200)); err != nil {
		t.Fatalf("SetWindow: %v", err)
	}
	if b.Window() != core.Milliseconds(200) {
		t.Errorf("window = %v, want 200ms", b.Window())
	}
	if hit, _ := b.Peek("jump", core.Milliseconds(150)); !hit {
		t.Error("retuned peek = false, want true within wider window")
	}
}

// D: pressure input keeps up with a measured cost and stays bounded.
func TestBufferPerfPressure(t *testing.T) {
	// Synthetic load only (no golden): golden presses stay in
	// buffer_cases.json. Frozen windows drive every evaluation.
	f := loadBufferCases(t)
	pushes, reps, touchReps := f.Perf.BufferPushes, f.Perf.Reps, f.Perf.TouchReps
	if pushes <= 0 || reps <= 0 || touchReps <= 0 {
		t.Fatalf("perf params = %d/%d/%d, want > 0", pushes, reps, touchReps)
	}
	b, _ := NewBuffer(DefaultBufferWindow)
	var hits int
	start := time.Now()
	for i := 0; i < reps; i++ {
		base := core.Milliseconds(int64(i * 10))
		for j := 0; j < pushes; j++ {
			_ = b.Push("jump", base+core.Duration(j))
		}
		b.Prune(base + core.Duration(pushes))
		if hit, _ := b.Peek("jump", base+core.Duration(pushes)); hit {
			hits++
		}
		b.Clear()
	}
	el := time.Since(start)
	total := int64(reps * (pushes + 1))
	t.Logf("buffer-pressure: %d reps x %d pushes (+peek) (%d ops) in %v (%.1f ns/op, hits %d)",
		reps, pushes, total, el, float64(el.Nanoseconds())/float64(total), hits)
	if hits != reps {
		t.Errorf("pressure hits = %d, want %d (every rep live)", hits, reps)
	}
	if b.Len() != 0 {
		t.Errorf("pressure len = %d, want 0 after Clear", b.Len())
	}
	tr := NewTouchTracker()
	start = time.Now()
	for i := 0; i < touchReps; i++ {
		id := i % MaxTouches
		_ = tr.Begin(id, core.V2(float64(i%100), float64(i%50)), core.Milliseconds(int64(i)))
		_ = tr.Move(id, core.V2(float64(i%100+1), float64(i%50+1)), core.Milliseconds(int64(i+1)))
		_ = tr.End(id, core.Milliseconds(int64(i+2)))
	}
	el = time.Since(start)
	t.Logf("touch-pressure: %d reps (begin/move/end) in %v (%.1f ns/op, active %d)",
		touchReps, el, float64(el.Nanoseconds())/float64(touchReps*3), tr.ActiveCount())
	if tr.ActiveCount() != 0 {
		t.Errorf("touch pressure active = %d, want 0", tr.ActiveCount())
	}
	if b.Len() > MaxBuffered || tr.ActiveCount() > MaxTouches {
		t.Error("pressure exceeded frozen budgets, want bounded")
	}
}

// E: long runs neither leak nor diverge between replays.
func TestBufferLongRunStable(t *testing.T) {
	f := loadBufferCases(t)
	reps := f.Longrun.Reps
	if reps <= 0 {
		t.Fatalf("longrun reps = %d, want > 0", reps)
	}
	a, _ := NewBuffer(DefaultBufferWindow)
	b, _ := NewBuffer(DefaultBufferWindow)
	for i := 0; i < reps; i++ {
		at := core.Milliseconds(int64(i))
		now := core.Milliseconds(int64(i + 10))
		if err := a.Push("jump", at); err != nil {
			t.Fatalf("rep %d push a: %v", i, err)
		}
		if err := b.Push("jump", at); err != nil {
			t.Fatalf("rep %d push b: %v", i, err)
		}
		ha, _ := a.Consume("jump", now)
		hb, _ := b.Consume("jump", now)
		if ha != hb || !ha {
			t.Fatalf("rep %d consume %v/%v, want true/true", i, ha, hb)
		}
		if a.Len() != b.Len() || a.Len() != 0 {
			t.Fatalf("rep %d len %d/%d, want 0/0", i, a.Len(), b.Len())
		}
	}
	// Overflow soak parks at the cap with identical contents.
	c, d := mustSoakBuffer(t)
	if c.Len() != MaxBuffered || d.Len() != MaxBuffered {
		t.Fatalf("soak len = %d/%d, want %d", c.Len(), d.Len(), MaxBuffered)
	}
	pc, pd := c.Presses(), d.Presses()
	for i := range pc {
		if pc[i] != pd[i] {
			t.Fatalf("soak press[%d] diverged", i)
		}
	}
	// Touch soak: begin/move/end cycles replay identically with no growth.
	ta, tb := NewTouchTracker(), NewTouchTracker()
	for i := 0; i < reps; i++ {
		id := i % MaxTouches
		at := core.Milliseconds(int64(i))
		_ = ta.Begin(id, core.V2(float64(id), 0), at)
		_ = tb.Begin(id, core.V2(float64(id), 0), at)
		// Only one of each id is live at a time; cycle ends before reuse.
		_ = ta.Move(id, core.V2(float64(id+1), 1), at+1)
		_ = tb.Move(id, core.V2(float64(id+1), 1), at+1)
		_ = ta.End(id, at+2)
		_ = tb.End(id, at+2)
		if ta.ActiveCount() != tb.ActiveCount() {
			t.Fatalf("rep %d active diverged", i)
		}
	}
	if ta.ActiveCount() != 0 || tb.ActiveCount() != 0 {
		t.Errorf("touch soak active = %d/%d, want 0/0", ta.ActiveCount(), tb.ActiveCount())
	}
	// Clear drops presses but keeps the window.
	a.Clear()
	if a.Len() != 0 || a.Window() != DefaultBufferWindow {
		t.Errorf("after Clear len/window = %d/%v, want 0/%v", a.Len(), a.Window(), DefaultBufferWindow)
	}
	ta.Clear()
	if ta.ActiveCount() != 0 {
		t.Error("after Clear active != 0")
	}
}

func mustSoakBuffer(t *testing.T) (*Buffer, *Buffer) {
	t.Helper()
	c, _ := NewBuffer(DefaultBufferWindow)
	d, _ := NewBuffer(DefaultBufferWindow)
	for i := 0; i < MaxBuffered+20; i++ {
		at := core.Milliseconds(int64(i))
		if err := c.Push("soak", at); err != nil {
			t.Fatalf("soak c %d: %v", i, err)
		}
		if err := d.Push("soak", at); err != nil {
			t.Fatalf("soak d %d: %v", i, err)
		}
	}
	return c, d
}

// F: offscreen golden stands in for game_input--case=combo (W2 intent).
// Frozen windows plus shape assertions pin the meaning, not just numbers.
func TestBufferOffscreenGolden(t *testing.T) {
	f := loadBufferCases(t)
	if MaxBuffered != f.Limits.MaxBuffered {
		t.Fatalf("MaxBuffered = %d, want frozen %d", MaxBuffered, f.Limits.MaxBuffered)
	}
	if MaxTouches != f.Limits.MaxTouches {
		t.Fatalf("MaxTouches = %d, want frozen %d", MaxTouches, f.Limits.MaxTouches)
	}
	if DefaultBufferWindow != core.Milliseconds(f.Limits.DefaultWindowMs) {
		t.Fatalf("DefaultBufferWindow = %v, want frozen %vms", DefaultBufferWindow, f.Limits.DefaultWindowMs)
	}
	if len(f.BufferSequences) != 6 {
		t.Fatalf("buffer sequences = %d, want 6 frozen", len(f.BufferSequences))
	}
	if len(f.TouchSequences) != 3 {
		t.Fatalf("touch sequences = %d, want 3 frozen", len(f.TouchSequences))
	}
	// Shape: hits land inside the window, expires land outside it.
	combo := mustBufferSeq(t, f, "combo_hit")
	for _, p := range combo.Pushes {
		if p.AtMs > 100 {
			t.Errorf("combo push %+v after first consume, want cached early", p)
		}
	}
	expire := mustBufferSeq(t, f, "expire_miss")
	if diff := expire.Steps[1].NowMs - expire.Pushes[0].AtMs; diff <= expire.WindowMs {
		t.Errorf("expire diff = %d, want > window %d", diff, expire.WindowMs)
	}
	future := mustBufferSeq(t, f, "future_skip")
	if future.Steps[0].NowMs >= future.Pushes[0].AtMs {
		t.Errorf("future first consume must predate the push")
	}
	if diff := future.Steps[2].NowMs - future.Pushes[0].AtMs; diff < 0 || diff > future.WindowMs {
		t.Errorf("future hit diff = %d, want inside (0,window]", diff)
	}
	prune := mustBufferSeq(t, f, "prune_window")
	if diff := prune.Steps[0].NowMs - prune.Pushes[0].AtMs; diff <= prune.WindowMs {
		t.Errorf("prune first push must be expired at first consume, diff %d", diff)
	}
	// Shape: every sequence drains fully (combos never stick).
	for _, seq := range f.BufferSequences {
		if seq.WantLive != 0 || seq.WantLen != 0 {
			t.Errorf("%s: want_live/len = %d/%d, want 0/0 drained", seq.Name, seq.WantLive, seq.WantLen)
		}
		if len(seq.Pushes) == 0 || len(seq.Steps) == 0 {
			t.Errorf("%s: empty pushes/steps, want frozen combo", seq.Name)
		}
	}
	// Shape: touch ids never mix, moves land on the survivor.
	multi := mustTouchSeq(t, f, "multi_track")
	if len(multi.WantActive) != 1 || multi.WantActive[0] != 1 {
		t.Errorf("multi active = %v, want [1] survivor", multi.WantActive)
	}
	if len(multi.WantTouches) != 1 || multi.WantTouches[0].X != 15 || multi.WantTouches[0].Y != 25 {
		t.Errorf("multi survivor = %+v, want moved (15,25)", multi.WantTouches)
	}
	broken := mustTouchSeq(t, f, "broken_touches")
	sawNotFound, sawInvalid := false, false
	for _, o := range broken.Ops {
		switch o.WantCode {
		case "not-found":
			sawNotFound = true
		case "invalid-arg":
			sawInvalid = true
		}
	}
	if !sawNotFound || !sawInvalid {
		t.Error("broken golden must pin both not-found (lift unknown) and invalid-arg (double begin)")
	}
	for _, seq := range f.TouchSequences {
		for i := 1; i < len(seq.WantActive); i++ {
			if seq.WantActive[i-1] >= seq.WantActive[i] {
				t.Errorf("%s: active %v not sorted", seq.Name, seq.WantActive)
			}
		}
		for i := 1; i < len(seq.WantTouches); i++ {
			if seq.WantTouches[i-1].ID >= seq.WantTouches[i].ID {
				t.Errorf("%s: touches not sorted by id", seq.Name)
			}
		}
		for _, w := range seq.WantTouches {
			if math.IsNaN(w.X) || math.IsInf(w.X, 0) || math.IsNaN(w.Y) || math.IsInf(w.Y, 0) {
				t.Errorf("%s: touch %+v non-finite, want finite", seq.Name, w)
			}
		}
	}
	// Recompute one golden end to end: file numbers must reproduce.
	seq := mustBufferSeq(t, f, "mixed_order")
	b, _ := NewBuffer(core.Milliseconds(seq.WindowMs))
	applyBufferPushes(t, b, seq.Pushes)
	checkBufferSteps(t, b, seq)
	if b.LiveCount(core.Milliseconds(seq.CheckAtMs)) != seq.WantLive || b.Len() != seq.WantLen {
		t.Error("mixed_order recompute diverged from golden")
	}
}
