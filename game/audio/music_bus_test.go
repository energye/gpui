package audio

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type xformGolden struct {
	Name         string   `json:"name"`
	Op           string   `json:"op"`
	StartStack   []string `json:"start_stack"`
	Target       string   `json:"target"`
	FadeMs       int64    `json:"fade_ms"`
	AtMs         int64    `json:"at_ms"`
	WantFrom     string   `json:"want_from"`
	WantFromGain float64  `json:"want_from_gain"`
	WantTo       string   `json:"want_to"`
	WantToGain   float64  `json:"want_to_gain"`
	WantFading   bool     `json:"want_fading"`
	WantTop      string   `json:"want_top"`
	WantDepth    int      `json:"want_depth"`
	WantProgress float64  `json:"want_progress"`
}

type busGolden struct {
	Name     string  `json:"name"`
	Volume   float64 `json:"volume"`
	Mute     bool    `json:"mute"`
	WantGain float64 `json:"want_gain"`
}

type duckGolden struct {
	Name       string  `json:"name"`
	Depth      float64 `json:"depth"`
	HoldMs     int64   `json:"hold_ms"`
	ReleaseMs  int64   `json:"release_ms"`
	DoTrigger  bool    `json:"do_trigger"`
	Strength   float64 `json:"strength"`
	AtMs       int64   `json:"at_ms"`
	WantGain   float64 `json:"want_gain"`
	WantActive bool    `json:"want_active"`
}

type voiceGolden struct {
	Name        string    `json:"name"`
	MaxVoices   int       `json:"max_voices"`
	Gains       []float64 `json:"gains"`
	WantTotal   float64   `json:"want_total"`
	WantClipped bool      `json:"want_clipped"`
	WantAudible int       `json:"want_audible"`
	WantKept    int       `json:"want_kept"`
}

type combineGolden struct {
	Name  string  `json:"name"`
	Music float64 `json:"music"`
	Bus   float64 `json:"bus"`
	Duck  float64 `json:"duck"`
	Want  float64 `json:"want"`
}

type musicBusFile struct {
	Xforms   []xformGolden   `json:"xforms"`
	Buses    []busGolden     `json:"buses"`
	Ducks    []duckGolden    `json:"ducks"`
	Mixes    []voiceGolden   `json:"mixes"`
	Combines []combineGolden `json:"combines"`
}

func loadMusicBusCases(t *testing.T) musicBusFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "music_bus_cases.json"))
	if err != nil {
		t.Fatalf("read music_bus_cases.json: %v", err)
	}
	var f musicBusFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode music_bus_cases.json: %v", err)
	}
	if len(f.Xforms) == 0 || len(f.Buses) == 0 || len(f.Ducks) == 0 || len(f.Mixes) == 0 || len(f.Combines) == 0 {
		t.Fatal("music_bus_cases.json has no cases")
	}
	return f
}

func buildStack(t *testing.T, stack []string) *MusicPlayer {
	t.Helper()
	p := NewMusicPlayer()
	for i, s := range stack {
		id := core.AssetID(s)
		var err error
		if i == 0 {
			err = p.Play(id)
		} else {
			err = p.Push(id, 0)
		}
		if err != nil {
			t.Fatalf("build stack %v[%d]: %v", stack, i, err)
		}
	}
	return p
}

func applyXform(t *testing.T, p *MusicPlayer, c xformGolden) {
	t.Helper()
	fade := core.Duration(c.FadeMs)
	var err error
	switch c.Op {
	case "xfade":
		err = p.CrossfadeTo(core.AssetID(c.Target), fade)
	case "push":
		err = p.Push(core.AssetID(c.Target), fade)
	case "pop":
		err = p.Pop(fade)
	case "stop":
		err = p.Stop(fade)
	default:
		t.Fatalf("%s: unknown op %q", c.Name, c.Op)
	}
	if err != nil {
		t.Fatalf("%s: op %s: %v", c.Name, c.Op, err)
	}
	p.Update(core.Duration(c.AtMs))
}

func checkXform(t *testing.T, c xformGolden) {
	t.Helper()
	p := buildStack(t, c.StartStack)
	applyXform(t, p, c)
	from, fromGain, to, toGain := p.Blend()
	if string(from) != c.WantFrom {
		t.Errorf("%s: from = %q, want %q", c.Name, from, c.WantFrom)
	}
	if !closeFloat(fromGain, c.WantFromGain) {
		t.Errorf("%s: fromGain = %.17g, want %.17g", c.Name, fromGain, c.WantFromGain)
	}
	if string(to) != c.WantTo {
		t.Errorf("%s: to = %q, want %q", c.Name, to, c.WantTo)
	}
	if !closeFloat(toGain, c.WantToGain) {
		t.Errorf("%s: toGain = %.17g, want %.17g", c.Name, toGain, c.WantToGain)
	}
	if p.IsFading() != c.WantFading {
		t.Errorf("%s: fading = %v, want %v", c.Name, p.IsFading(), c.WantFading)
	}
	if string(p.Top()) != c.WantTop {
		t.Errorf("%s: top = %q, want %q", c.Name, p.Top(), c.WantTop)
	}
	if p.Depth() != c.WantDepth {
		t.Errorf("%s: depth = %d, want %d", c.Name, p.Depth(), c.WantDepth)
	}
	if !closeFloat(p.Progress(), c.WantProgress) {
		t.Errorf("%s: progress = %.17g, want %.17g", c.Name, p.Progress(), c.WantProgress)
	}
	if !finite(fromGain) || !finite(toGain) {
		t.Errorf("%s: non-finite blend %v/%v", c.Name, fromGain, toGain)
	}
	if fromGain < 0 || fromGain > 1 || toGain < 0 || toGain > 1 {
		t.Errorf("%s: out of range blend %v/%v", c.Name, fromGain, toGain)
	}
}

// A:切换淡入淡出总线闪避落在冻结数上，混音上限与叠乘对。
func TestMusicXformFromCases(t *testing.T) {
	if DefaultBusVolume != 1 || DefaultMaxVoices != 32 || DefaultDuckDepth != 0.5 {
		t.Fatalf("defaults = %v/%v/%v, want 1/32/0.5", DefaultBusVolume, DefaultMaxVoices, DefaultDuckDepth)
	}
	if MaxMixerVoices != 256 || MaxMusicStack != 8 {
		t.Fatalf("budgets = %v/%v, want 256/8", MaxMixerVoices, MaxMusicStack)
	}
	if DefaultFade != core.Second {
		t.Fatalf("default fade = %v, want 1s", DefaultFade)
	}
	f := loadMusicBusCases(t)
	seen := map[string]bool{}
	for _, c := range f.Xforms {
		if seen[c.Name] {
			t.Errorf("duplicate xform %q", c.Name)
		}
		seen[c.Name] = true
		checkXform(t, c)
	}
	for _, c := range f.Buses {
		b, err := NewBus("bus_"+c.Name, c.Volume)
		if err != nil {
			t.Fatalf("bus %s: %v", c.Name, err)
		}
		if err := b.SetMute(c.Mute); err != nil {
			t.Fatalf("bus %s mute: %v", c.Name, err)
		}
		if got := b.Gain(); !closeFloat(got, c.WantGain) {
			t.Errorf("bus %s: gain = %.17g, want %.17g", c.Name, got, c.WantGain)
		}
		if got := b.Volume(); !closeFloat(got, c.Volume) {
			t.Errorf("bus %s: volume = %.17g, want %.17g", c.Name, got, c.Volume)
		}
		if b.Muted() != c.Mute {
			t.Errorf("bus %s: muted = %v, want %v", c.Name, b.Muted(), c.Mute)
		}
	}
	for _, c := range f.Ducks {
		d, err := NewDucker(c.Depth, core.Duration(c.HoldMs), core.Duration(c.ReleaseMs))
		if err != nil {
			t.Fatalf("duck %s: %v", c.Name, err)
		}
		if c.DoTrigger {
			if err := d.Trigger(c.Strength); err != nil {
				t.Fatalf("duck %s trigger: %v", c.Name, err)
			}
		}
		d.Update(core.Duration(c.AtMs))
		if got := d.Gain(); !closeFloat(got, c.WantGain) {
			t.Errorf("duck %s: gain = %.17g, want %.17g", c.Name, got, c.WantGain)
		}
		if d.Active() != c.WantActive {
			t.Errorf("duck %s: active = %v, want %v", c.Name, d.Active(), c.WantActive)
		}
	}
	for _, c := range f.Mixes {
		m, err := NewMixer(c.MaxVoices)
		if err != nil {
			t.Fatalf("mix %s: %v", c.Name, err)
		}
		got := m.Mix(c.Gains)
		if !closeFloat(got.Total, c.WantTotal) {
			t.Errorf("mix %s: total = %.17g, want %.17g", c.Name, got.Total, c.WantTotal)
		}
		if got.Clipped != c.WantClipped {
			t.Errorf("mix %s: clipped = %v, want %v", c.Name, got.Clipped, c.WantClipped)
		}
		if got.Audible != c.WantAudible {
			t.Errorf("mix %s: audible = %d, want %d", c.Name, got.Audible, c.WantAudible)
		}
		if got.Kept != c.WantKept {
			t.Errorf("mix %s: kept = %d, want %d", c.Name, got.Kept, c.WantKept)
		}
		if math.IsNaN(got.Total) || math.IsInf(got.Total, 0) || got.Total < 0 || got.Total > 1 {
			t.Errorf("mix %s: bad total %v", c.Name, got.Total)
		}
	}
	for _, c := range f.Combines {
		if got := CombineGains(c.Music, c.Bus, c.Duck); !closeFloat(got, c.Want) {
			t.Errorf("combine %s: got %.17g, want %.17g", c.Name, got, c.Want)
		}
	}
	// Wiring: frozen tables agree through CombineGains. xfade_500 music
	// 0.5 times typical bus 0.8 times hold_0 duck 0.5 lands on ducked 0.2.
	xf := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "xfade_500")
	bu := mustFindBy(t, f.Buses, func(c busGolden) string { return c.Name }, "bus", "typical")
	du := mustFindBy(t, f.Ducks, func(c duckGolden) string { return c.Name }, "duck", "hold_0")
	co := mustFindBy(t, f.Combines, func(c combineGolden) string { return c.Name }, "combine", "ducked")
	got := CombineGains(xf.WantToGain, bu.WantGain, du.WantGain)
	if !closeFloat(got, co.Want) {
		t.Fatalf("wiring xfade/bus/duck = %.17g, want combine %.17g", got, co.Want)
	}
}

// B:空缺超限坏数据不崩不卡死，占位加报错。
func TestMusicBusEdgesNoCrash(t *testing.T) {
	p := NewMusicPlayer()
	expectCode(t, "play-empty", 0, p.Play(""))
	expectCode(t, "xfade-empty", 0, p.CrossfadeTo("", 100))
	expectCode(t, "push-empty", 0, p.Push("", 100))
	expectCode(t, "xfade-neg", 0, p.CrossfadeTo("music/a", -1))
	expectCode(t, "push-neg", 0, p.Push("music/a", -1))
	expectCode(t, "pop-neg", 0, p.Pop(-1))
	expectCode(t, "stop-neg", 0, p.Stop(-1))
	if err := p.Pop(0); err == nil {
		t.Error("pop empty: want error")
	} else if core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("pop empty code = %v, want not-found", core.CodeOf(err))
	}
	if err := p.Play("music/a"); err != nil {
		t.Fatalf("play a: %v", err)
	}
	depthBefore := p.Depth()
	expectCode(t, "play-empty-keeps", 0, p.Play(""))
	if p.Depth() != depthBefore || p.Top() != "music/a" {
		t.Errorf("bad play changed stack to %v/%v", p.Top(), p.Depth())
	}
	// Stack cap: fill to MaxMusicStack, one more is OutOfMemory.
	full := NewMusicPlayer()
	if err := full.Play("music/0"); err != nil {
		t.Fatalf("fill play: %v", err)
	}
	for i := 1; i < MaxMusicStack; i++ {
		if err := full.Push(core.AssetID("music/"+strconv.Itoa(i)), 0); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if full.Depth() != MaxMusicStack {
		t.Fatalf("fill depth = %d, want %d", full.Depth(), MaxMusicStack)
	}
	if err := full.Push("music/over", 0); err == nil {
		t.Error("push over cap: want error")
	} else if core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("push over cap code = %v, want out-of-memory", core.CodeOf(err))
	}
	if full.Depth() != MaxMusicStack {
		t.Errorf("over-push changed depth to %d", full.Depth())
	}
	// Nil players never panic.
	var nilP *MusicPlayer
	expectCode(t, "nil-play", 0, nilP.Play("music/a"))
	expectCode(t, "nil-push", 0, nilP.Push("music/a", 0))
	expectCode(t, "nil-pop", 0, nilP.Pop(0))
	expectCode(t, "nil-xfade", 0, nilP.CrossfadeTo("music/a", 0))
	expectCode(t, "nil-stop", 0, nilP.Stop(0))
	nilP.Update(100)
	if nilP.Depth() != 0 || nilP.Top() != "" || nilP.IsFading() || nilP.Progress() != 1 {
		t.Error("nil player reports non-silence")
	}
	if from, fg, to, tg := nilP.Blend(); from != "" || to != "" || fg != 0 || tg != 0 {
		t.Errorf("nil blend = %v/%v %v/%v, want silence", from, fg, to, tg)
	}
	// Non-positive dt never advances.
	q := buildStack(t, []string{"music/a"})
	if err := q.CrossfadeTo("music/b", 1000); err != nil {
		t.Fatalf("q xfade: %v", err)
	}
	q.Update(0)
	q.Update(-5)
	if _, fg, _, tg := q.Blend(); !closeFloat(fg, 1) || !closeFloat(tg, 0) {
		t.Errorf("non-positive dt advanced to %v/%v", fg, tg)
	}
	// Buses: bad volumes keep the old value.
	if _, err := NewBus("", 0.5); err == nil {
		t.Error("bus empty name: want error")
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bus empty name code = %v, want invalid-arg", core.CodeOf(err))
	}
	for _, v := range []float64{-0.1, 1.1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := NewBus("b", v); err == nil {
			t.Errorf("bus volume %v: want error", v)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bus volume %v code = %v, want invalid-arg", v, core.CodeOf(err))
		}
	}
	b, err := NewBus("music", 0.8)
	if err != nil {
		t.Fatalf("good bus: %v", err)
	}
	for _, v := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		before := b.Volume()
		expectCode(t, "bus-set", v, b.SetVolume(v))
		if b.Volume() != before {
			t.Errorf("bad SetVolume %v changed to %v", v, b.Volume())
		}
	}
	var nilB *Bus
	expectCode(t, "nil-bus-vol", 0, nilB.SetVolume(0.5))
	expectCode(t, "nil-bus-mute", 0, nilB.SetMute(true))
	if nilB.Gain() != 0 || nilB.Volume() != 0 || nilB.Muted() || nilB.Name() != "" {
		t.Error("nil bus reports non-silence")
	}
	// Mixers: bad caps keep the old cap; bad voices never blast.
	for _, n := range []int{0, -1, MaxMixerVoices + 1} {
		if _, err := NewMixer(n); err == nil {
			t.Errorf("mixer cap %d: want error", n)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("mixer cap %d code = %v, want invalid-arg", n, core.CodeOf(err))
		}
	}
	m, err := NewMixer(2)
	if err != nil {
		t.Fatalf("good mixer: %v", err)
	}
	beforeCap := m.MaxVoices()
	for _, n := range []int{0, -3, MaxMixerVoices + 9} {
		expectCode(t, "mixer-set", float64(n), m.SetMaxVoices(n))
		if m.MaxVoices() != beforeCap {
			t.Errorf("bad SetMaxVoices %d changed cap", n)
		}
	}
	badVoices := []float64{math.NaN(), math.Inf(1), math.Inf(-1), -0.5, 0}
	got := m.Mix(badVoices)
	if got.Total != 0 || got.Audible != 0 || got.Kept != 0 || got.Clipped {
		t.Errorf("bad voices mix = %+v, want silence", got)
	}
	snap := append([]float64(nil), badVoices...)
	m.Mix(badVoices)
	for i := range badVoices {
		if badVoices[i] != snap[i] && !(math.IsNaN(badVoices[i]) && math.IsNaN(snap[i])) {
			t.Fatal("Mix mutated the input")
		}
	}
	var nilM *Mixer
	if got := nilM.Mix([]float64{0.5}); got.Total != 0 || got.Audible != 0 {
		t.Errorf("nil mixer = %+v, want zero", got)
	}
	expectCode(t, "nil-mixer-set", 0, nilM.SetMaxVoices(4))
	// Duckers: bad depth/strength keep the old state; nil never panics.
	for _, depth := range []float64{-0.1, 1.1, math.NaN(), math.Inf(1)} {
		if _, err := NewDucker(depth, 100, 100); err == nil {
			t.Errorf("duck depth %v: want error", depth)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("duck depth %v code = %v, want invalid-arg", depth, core.CodeOf(err))
		}
	}
	if _, err := NewDucker(0.5, -1, 0); err == nil {
		t.Error("duck neg hold: want error")
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("duck neg hold code = %v, want invalid-arg", core.CodeOf(err))
	}
	d, err := NewDucker(0.5, 500, 500)
	if err != nil {
		t.Fatalf("good ducker: %v", err)
	}
	for _, s := range []float64{-0.1, 1.1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		wasActive := d.Active()
		expectCode(t, "duck-trigger", s, d.Trigger(s))
		if d.Active() != wasActive {
			t.Errorf("bad trigger %v flipped active", s)
		}
	}
	var nilD *Ducker
	expectCode(t, "nil-duck", 0, nilD.Trigger(1))
	nilD.Update(100)
	if nilD.Active() || nilD.Gain() != 1 || nilD.Depth() != 0 {
		t.Error("nil ducker reports non-rest")
	}
	// Combine fails closed at 0, never NaN and never above 1.
	for _, bad := range [][3]float64{
		{math.NaN(), 0.5, 0.5}, {0.5, math.Inf(1), 0.5}, {0.5, 0.5, math.Inf(-1)},
	} {
		if got := CombineGains(bad[0], bad[1], bad[2]); got != 0 {
			t.Errorf("combine %v = %v, want 0", bad, got)
		}
	}
	if got := CombineGains(2, 2, 2); got != 1 {
		t.Errorf("combine blast = %v, want 1", got)
	}
}

// C不适用（纯算数不画画）：输入不改加逐位重放即两边同数。
func TestMusicBusBoundaryIdentical(t *testing.T) {
	f := loadMusicBusCases(t)
	for _, c := range f.Xforms {
		mkBlend := func() (core.AssetID, float64, core.AssetID, float64, float64) {
			p := buildStack(t, c.StartStack)
			applyXform(t, p, c)
			from, fg, to, tg := p.Blend()
			return from, fg, to, tg, p.Progress()
		}
		aFrom, aFG, aTo, aTG, aProg := mkBlend()
		bFrom, bFG, bTo, bTG, bProg := mkBlend()
		if aFrom != bFrom || aTo != bTo || aFG != bFG || aTG != bTG || aProg != bProg {
			t.Errorf("%s: blend replay diverged", c.Name)
		}
	}
	for _, c := range f.Mixes {
		m, err := NewMixer(c.MaxVoices)
		if err != nil {
			t.Fatalf("mix %s: %v", c.Name, err)
		}
		snap := append([]float64(nil), c.Gains...)
		a := m.Mix(c.Gains)
		b := m.Mix(c.Gains)
		if a != b {
			t.Errorf("mix %s: replay diverged %+v vs %+v", c.Name, a, b)
		}
		for i := range c.Gains {
			if c.Gains[i] != snap[i] {
				t.Fatalf("mix %s mutated input at %d", c.Name, i)
			}
		}
	}
	for _, c := range f.Ducks {
		mk := func() *Ducker {
			d, err := NewDucker(c.Depth, core.Duration(c.HoldMs), core.Duration(c.ReleaseMs))
			if err != nil {
				t.Fatalf("duck %s: %v", c.Name, err)
			}
			if c.DoTrigger {
				if err := d.Trigger(c.Strength); err != nil {
					t.Fatalf("duck %s trigger: %v", c.Name, err)
				}
			}
			d.Update(core.Duration(c.AtMs))
			return d
		}
		if mk().Gain() != mk().Gain() || mk().Active() != mk().Active() {
			t.Errorf("duck %s: replay diverged", c.Name)
		}
	}
	for _, c := range f.Combines {
		if CombineGains(c.Music, c.Bus, c.Duck) != CombineGains(c.Music, c.Bus, c.Duck) {
			t.Errorf("combine %s: replay diverged", c.Name)
		}
	}
	// Stack copy isolation: pushing never aliases the old slice.
	p := buildStack(t, []string{"music/a"})
	before := p.Depth()
	if err := p.Push("music/b", 0); err != nil {
		t.Fatalf("push: %v", err)
	}
	if before != 1 || p.Depth() != 2 {
		t.Errorf("stack depths %d->%d, want 1->2", before, p.Depth())
	}
}

// D:多声跑得动，耗时加声数有数。
func TestMusicBusPerfCapped(t *testing.T) {
	// Synthetic load only (no golden): golden stays in
	// music_bus_cases.json. Seeded rand keeps the load replayable.
	r := core.NewRand(20260915)
	const n = 256
	voices := make([]float64, n)
	for i := range voices {
		voices[i] = 0.05 + 0.9*r.Float64()
	}
	m, err := NewMixer(DefaultMaxVoices)
	if err != nil {
		t.Fatalf("mixer: %v", err)
	}
	const reps = 2000
	start := time.Now()
	audible := 0
	for i := 0; i < reps; i++ {
		got := m.Mix(voices)
		if got.Audible != n {
			t.Fatalf("rep %d: audible = %d, want %d", i, got.Audible, n)
		}
		if got.Kept != DefaultMaxVoices {
			t.Fatalf("rep %d: kept = %d, want %d", i, got.Kept, DefaultMaxVoices)
		}
		if got.Total < 0 || got.Total > 1 || math.IsNaN(got.Total) {
			t.Fatalf("rep %d: bad total %v", i, got.Total)
		}
		if !got.Clipped {
			t.Fatalf("rep %d: want clipped with 256 loud voices", i)
		}
		audible += got.Audible
		// Combine path joins the cost: one voice per rep.
		if out := CombineGains(got.Total, DefaultBusVolume, 0.5); out < 0 || out > 1 {
			t.Fatalf("rep %d: bad combine %v", i, out)
		}
	}
	el := time.Since(start)
	t.Logf("musicbus-256: %d reps x %d voices (%d mixes) in %v (%.1f us/rep)", reps, n, reps*n, el, float64(el.Microseconds())/reps)
	if audible == 0 {
		t.Error("perf load mixed nothing, benchmark invalid")
	}
}

// E:长跑不爆不漂，坏数据不粘。
func TestMusicBusLongRunStable(t *testing.T) {
	f := loadMusicBusCases(t)
	mid := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "xfade_500")
	p := buildStack(t, mid.StartStack)
	applyXform(t, p, mid)
	from, fg, to, tg := p.Blend()
	for i := 0; i < 10000; i++ {
		qFrom, qFG, qTo, qTG := p.Blend()
		if qFrom != from || qTo != to || qFG != fg || qTG != tg {
			t.Fatalf("rep %d diverged", i)
		}
		if !finite(qFG) || !finite(qTG) || qFG < 0 || qFG > 1 || qTG < 0 || qTG > 1 {
			t.Fatalf("rep %d out of range %v/%v", i, qFG, qTG)
		}
	}
	// Walk a full fade in 100 steps: gains glide monotonically to settled.
	walk := NewMusicPlayer()
	if err := walk.Play("music/a"); err != nil {
		t.Fatalf("walk play: %v", err)
	}
	if err := walk.CrossfadeTo("music/b", 1000); err != nil {
		t.Fatalf("walk xfade: %v", err)
	}
	lastFrom := 2.0
	lastTo := -1.0
	for i := 0; i < 100; i++ {
		walk.Update(10)
		_, fg, _, tg := walk.Blend()
		if fg > lastFrom || tg < lastTo {
			t.Fatalf("step %d not monotonic %v/%v", i, fg, tg)
		}
		lastFrom, lastTo = fg, tg
	}
	if walk.IsFading() {
		t.Error("walk did not settle after 1000ms")
	}
	if _, fg, _, tg := walk.Blend(); fg != 0 || tg != 1 {
		t.Errorf("walk settled = %v/%v, want 0/1", fg, tg)
	}
	// Duck recovers fully and never blasts across 5000 full-scale frames.
	d, err := NewDucker(0.5, 500, 500)
	if err != nil {
		t.Fatalf("ducker: %v", err)
	}
	if err := d.Trigger(1); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	for i := 0; i < 1000; i++ {
		d.Update(1)
	}
	if d.Active() || d.Gain() != 1 {
		t.Errorf("duck did not recover: active=%v gain=%v", d.Active(), d.Gain())
	}
	for i := 0; i < 5000; i++ {
		if out := CombineGains(1, 1, d.Gain()); out < 0 || out > 1 {
			t.Fatalf("rep %d clips %v", i, out)
		}
	}
	// Bad data never poisons the next good mix.
	if _, err := NewBus("b", math.NaN()); err == nil {
		t.Error("bad bus volume: want error")
	}
	after, err := NewBus("b", 0.8)
	if err != nil || after.Gain() != 0.8 {
		t.Errorf("after bad: %v/%v, want 0.8/nil", after, err)
	}
}

// F:离屏金对照窗（窗免，纯算数加人工听）：冻结数加形状断言，终混数即听感。
func TestMusicBusOffscreenGolden(t *testing.T) {
	f := loadMusicBusCases(t)
	x0 := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "xfade_0")
	x500 := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "xfade_500")
	x1000 := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "xfade_1000")
	// Golden pins the crossfade anchors: full old, half-half, full new.
	if x0.WantFromGain != 1 || x0.WantToGain != 0 || !x0.WantFading {
		t.Errorf("xfade_0 = %+v, want 1/0 fading", x0)
	}
	if !closeFloat(x500.WantFromGain, 0.5) || !closeFloat(x500.WantToGain, 0.5) {
		t.Errorf("xfade_500 = %+v, want 0.5/0.5", x500)
	}
	if x1000.WantFromGain != 0 || x1000.WantToGain != 1 || x1000.WantFading {
		t.Errorf("xfade_1000 = %+v, want 0/1 settled", x1000)
	}
	// Shape: fades glide monotonically and sum to 1 while both sides play.
	names := []string{"xfade_0", "xfade_250", "xfade_500", "xfade_750", "xfade_1000"}
	lastFrom := 2.0
	lastTo := -1.0
	for _, name := range names {
		c := mustFindBy(t, f.Xforms, func(v xformGolden) string { return v.Name }, "xform", name)
		if c.WantFromGain > lastFrom || c.WantToGain < lastTo {
			t.Errorf("%s not monotonic in %+v", name, f.Xforms)
		}
		if name != "xfade_1000" && !closeFloat(c.WantFromGain+c.WantToGain, 1) {
			t.Errorf("%s sums to %v, want 1", name, c.WantFromGain+c.WantToGain)
		}
		lastFrom, lastTo = c.WantFromGain, c.WantToGain
	}
	// Shape: stop falls to silence; push grows depth; pop shrinks back.
	st500 := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "stop_500")
	if st500.WantDepth != 0 || st500.WantTop != "" {
		t.Errorf("stop_500 = %+v, want depth0 top empty", st500)
	}
	pu := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "push_500")
	if pu.WantDepth != 2 || pu.WantTop != "music/b" {
		t.Errorf("push_500 = %+v, want depth2 top b", pu)
	}
	po := mustFindBy(t, f.Xforms, func(c xformGolden) string { return c.Name }, "xform", "pop_500")
	if po.WantDepth != 1 || po.WantTop != "music/a" {
		t.Errorf("pop_500 = %+v, want depth1 top a", po)
	}
	// Shape: bus mute silences, duck holds flat then climbs, mixer caps
	// loudest and ceilings at 1, combine never boosts past its inputs.
	bu := mustFindBy(t, f.Buses, func(c busGolden) string { return c.Name }, "bus", "muted")
	if bu.WantGain != 0 {
		t.Errorf("muted bus = %+v, want 0", bu)
	}
	du0 := mustFindBy(t, f.Ducks, func(c duckGolden) string { return c.Name }, "duck", "hold_0")
	du250 := mustFindBy(t, f.Ducks, func(c duckGolden) string { return c.Name }, "duck", "hold_250")
	du750 := mustFindBy(t, f.Ducks, func(c duckGolden) string { return c.Name }, "duck", "release_750")
	du1000 := mustFindBy(t, f.Ducks, func(c duckGolden) string { return c.Name }, "duck", "done_1000")
	if du0.WantGain != du250.WantGain {
		t.Errorf("hold not flat: %v vs %v", du0.WantGain, du250.WantGain)
	}
	if !(du0.WantGain < du750.WantGain && du750.WantGain < du1000.WantGain) {
		t.Errorf("release does not climb: %v %v %v", du0.WantGain, du750.WantGain, du1000.WantGain)
	}
	for _, c := range f.Mixes {
		if c.WantTotal < 0 || c.WantTotal > 1 {
			t.Errorf("mix %s total %v out of [0,1]", c.Name, c.WantTotal)
		}
		if c.WantKept > c.WantAudible || c.WantKept > c.MaxVoices {
			t.Errorf("mix %s kept %d audible %d cap %d", c.Name, c.WantKept, c.WantAudible, c.MaxVoices)
		}
		if c.WantTotal == 1 && !c.WantClipped && c.WantAudible > 1 {
			// Single full voice is loud but not clipped; sums above 1 clip.
			t.Errorf("mix %s total1 without clipped flag", c.Name)
		}
	}
	for _, c := range f.Combines {
		top := c.Music
		if c.Bus < top {
			top = c.Bus
		}
		if c.Duck < top {
			top = c.Duck
		}
		if c.Want+1e-9 < 0 || c.Want > top+1e-9 {
			t.Errorf("combine %s = %v above inputs %v/%v/%v", c.Name, c.Want, c.Music, c.Bus, c.Duck)
		}
	}
	// Waveform guard: full-scale music through any frozen bus/duck never clips.
	for _, b := range f.Buses {
		for _, d := range f.Ducks {
			if out := CombineGains(1, b.WantGain, d.WantGain); out < 0 || out > 1 {
				t.Errorf("bus %s duck %s full-scale clips %v", b.Name, d.Name, out)
			}
		}
	}
	// Manual listening note: the frozen final gains are the audible
	// samples. Human check with headphones: play music/a, crossfade to
	// music/b over 1s (voice glides, no click); trigger an explosion at
	// full strength (music dips to half, voice stays intelligible, then
	// climbs back over 0.5s); mute the music bus (music gone, hits stay);
	// stack push/pop (layers add and fall back). Waveform stays in
	// [0,1] per voice (asserted above), so long runs never blast.
}
