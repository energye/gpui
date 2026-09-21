package behavior_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/kit/internal/behavior"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/scheduler"
)

type motionFile struct {
	Durations struct {
		Fast     string  `json:"fast"`
		Mid      string  `json:"mid"`
		Slow     string  `json:"slow"`
		WantFast float64 `json:"wantFast"`
		WantMid  float64 `json:"wantMid"`
		WantSlow float64 `json:"wantSlow"`
	} `json:"durations"`
	Curves []struct {
		Name string  `json:"name"`
		In   float64 `json:"in"`
		Want float64 `json:"want"`
	} `json:"curves"`
	Spinner struct {
		DurationSec     float64 `json:"durationSec"`
		WantAngleAtHalf float64 `json:"wantAngleAtHalf"`
	} `json:"spinner"`
	Wave struct {
		WantRadiusAtSpread float64 `json:"wantRadiusAtSpread"`
		WantAlphaAtStart   float64 `json:"wantAlphaAtStart"`
	} `json:"wave"`
	ReducedMotion struct {
		MotionOff bool `json:"motionOff"`
		Reduced   bool `json:"reduced"`
	} `json:"reducedMotion"`
}

func loadMotion(t *testing.T) motionFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "motion_cases.json"))
	if err != nil {
		t.Fatalf("read motion_cases.json: %v", err)
	}
	var f motionFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode motion_cases.json: %v", err)
	}
	return f
}

func TestMotion_DurationsAndCurves(t *testing.T) {
	f := loadMotion(t)
	if got := behavior.ParseDuration(f.Durations.Fast); math.Abs(got-f.Durations.WantFast) > 1e-9 {
		t.Fatalf("fast=%v want %v", got, f.Durations.WantFast)
	}
	if got := behavior.ParseDuration(f.Durations.Mid); math.Abs(got-f.Durations.WantMid) > 1e-9 {
		t.Fatalf("mid=%v want %v", got, f.Durations.WantMid)
	}
	if got := behavior.ParseDuration(f.Durations.Slow); math.Abs(got-f.Durations.WantSlow) > 1e-9 {
		t.Fatalf("slow=%v want %v", got, f.Durations.WantSlow)
	}
	for _, c := range f.Curves {
		curve := behavior.ResolveCurve(c.Name)
		if curve == nil {
			t.Fatalf("%s: nil curve", c.Name)
		}
		if got := curve.Transform(c.In); math.Abs(got-c.Want) > 1e-9 {
			t.Fatalf("%s(%v)=%v want %v", c.Name, c.In, got, c.Want)
		}
	}
	// Unknown curve falls back, never nil.
	if behavior.ResolveCurve("bounce-wobble") == nil {
		t.Fatal("unknown curve must fall back")
	}
}

func TestMotion_SpinnerTrueRotation(t *testing.T) {
	f := loadMotion(t)
	reg := &scheduler.TickerRegistry{}
	motion := scope.DefaultMotionConfig()
	sp := behavior.NewSpinner(f.Spinner.DurationSec, animation.CurveLinear)
	sp.Start(reg, motion)
	if !sp.IsSpinning() {
		t.Fatal("spinner must tick")
	}
	// Half duration advances half turn for linear repeat.
	sp.Controller().Tick(0.5)
	if got := sp.Angle(); math.Abs(got-f.Spinner.WantAngleAtHalf) > 1e-6 {
		t.Fatalf("angle=%v want %v", got, f.Spinner.WantAngleAtHalf)
	}
	sp.Stop()
	if sp.IsSpinning() {
		t.Fatal("stop must freeze")
	}
}

func TestMotion_ReducedMotionFreezes(t *testing.T) {
	f := loadMotion(t)
	_ = f
	off := scope.MotionConfig{Enabled: false, ReducedMotion: false}
	sp := behavior.NewSpinner(1, animation.CurveLinear)
	sp.Start(&scheduler.TickerRegistry{}, off)
	if sp.IsSpinning() {
		t.Fatal("motion-off must not tick")
	}
	reduced := scope.MotionConfig{Enabled: true, ReducedMotion: true}
	sp2 := behavior.NewSpinner(1, animation.CurveLinear)
	sp2.Start(&scheduler.TickerRegistry{}, reduced)
	if sp2.IsSpinning() {
		t.Fatal("reduced-motion must not tick")
	}
	w := behavior.NewWave()
	if w.Start(reduced, behavior.WaveConfig{Enabled: true}) {
		t.Fatal("reduced-motion must suppress wave")
	}
}

func TestMotion_WaveSpreadAndFade(t *testing.T) {
	f := loadMotion(t)
	motion := scope.DefaultMotionConfig()
	w := behavior.NewWave()
	if !w.Start(motion, behavior.WaveConfig{Enabled: true, HasBorder: true}) {
		t.Fatal("wave must start post-tap")
	}
	if got := w.Alpha(); math.Abs(got-f.Wave.WantAlphaAtStart) > 1e-9 {
		t.Fatalf("alpha at start=%v want %v", got, f.Wave.WantAlphaAtStart)
	}
	// Press-and-hold never starts: Start is only called post-tap, and
	// suppressed configs refuse.
	if w2 := behavior.NewWave(); w2.Start(motion, behavior.WaveConfig{Enabled: false}) {
		t.Fatal("disabled wave switch must refuse")
	}
	if w3 := behavior.NewWave(); w3.Start(motion, behavior.WaveConfig{Enabled: true, Disabled: true}) {
		t.Fatal("disabled widget must not ripple")
	}
	if w4 := behavior.NewWave(); w4.Start(motion, behavior.WaveConfig{Enabled: true, IsTextLink: true}) {
		t.Fatal("text/link must not ripple")
	}
	w.Tick(0.4)
	if got := w.Radius(); math.Abs(got-f.Wave.WantRadiusAtSpread) > 1e-9 {
		t.Fatalf("radius at spread=%v want %v", got, f.Wave.WantRadiusAtSpread)
	}
	w.Tick(1.6)
	if w.IsRunning() {
		t.Fatal("wave must complete by 2s")
	}
	if got := w.Alpha(); got != 0 {
		t.Fatalf("alpha after fade=%v want 0", got)
	}
}
