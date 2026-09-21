package behavior_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/internal/behavior"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
)

type triggerFile struct {
	Placements []struct {
		Name        string   `json:"name"`
		Anchor      rectJSON `json:"anchor"`
		OW          float64  `json:"ow"`
		OH          float64  `json:"oh"`
		Want        string   `json:"want"`
		WantX       float64  `json:"wantX"`
		WantY       float64  `json:"wantY"`
		WantFlipped bool     `json:"wantFlipped"`
		Options     struct {
			Gap       float64 `json:"gap"`
			Flip      bool    `json:"flip"`
			Shift     bool    `json:"shift"`
			ViewportW float64 `json:"viewportW"`
			ViewportH float64 `json:"viewportH"`
		} `json:"options"`
	} `json:"placements"`
	Outside []struct {
		Name       string    `json:"name"`
		Point      pointJSON `json:"point"`
		WantOpen   bool      `json:"wantOpen"`
		WantClosed bool      `json:"wantClosed"`
	} `json:"outsideCases"`
	Esc []struct {
		Name       string `json:"name"`
		PressEsc   bool   `json:"pressEsc"`
		WantOpen   bool   `json:"wantOpen"`
		WantClosed bool   `json:"wantClosed"`
	} `json:"escCases"`
}

type rectJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type pointJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func loadTrigger(t *testing.T) triggerFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "overlay_trigger_cases.json"))
	if err != nil {
		t.Fatalf("read overlay_trigger_cases.json: %v", err)
	}
	var f triggerFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode overlay_trigger_cases.json: %v", err)
	}
	if len(f.Placements) == 0 {
		t.Fatal("overlay_trigger_cases.json holds no placements")
	}
	return f
}

func TestTrigger_Placement(t *testing.T) {
	f := loadTrigger(t)
	for _, c := range f.Placements {
		anchor := rendering.NewRect(c.Anchor.X, c.Anchor.Y, c.Anchor.W, c.Anchor.H)
		opt := &overlay.ResolveOptions{
			Gap:       c.Options.Gap,
			Flip:      c.Options.Flip,
			Shift:     c.Options.Shift,
			ViewportW: c.Options.ViewportW,
			ViewportH: c.Options.ViewportH,
		}
		got := behavior.ResolveFollower(anchor, c.OW, c.OH, placementFor(c.Name, overlay.Placement(c.Want)), opt)
		if string(got.Actual) != c.Want {
			t.Fatalf("%s: follower actual=%s want %s", c.Name, got.Actual, c.Want)
		}
		// Resolve through the trigger so the D-class entry is covered too.
		tr := behavior.NewTrigger(behavior.TriggerConfig{
			Want:    placementFor(c.Name, overlay.Placement(c.Want)),
			Options: opt,
		}, nil, nil)
		res := tr.Resolve(anchor, c.OW, c.OH)
		if string(res.Actual) != c.Want {
			t.Fatalf("%s: actual=%s want %s", c.Name, res.Actual, c.Want)
		}
		if res.Flipped != c.WantFlipped {
			t.Fatalf("%s: flipped=%v want %v", c.Name, res.Flipped, c.WantFlipped)
		}
		if math.Abs(res.X-c.WantX) > 1e-9 || math.Abs(res.Y-c.WantY) > 1e-9 {
			t.Fatalf("%s: origin=(%.2f,%.2f) want (%.2f,%.2f)", c.Name, res.X, res.Y, c.WantX, c.WantY)
		}
	}
}

func placementFor(name string, want overlay.Placement) overlay.Placement {
	// top_flips_to_bottom asks for top; the resolver flips to bottom.
	if name == "top_flips_to_bottom" {
		return overlay.Top
	}
	return want
}

func TestTrigger_OutsideAndEsc(t *testing.T) {
	f := loadTrigger(t)
	anchor := rendering.NewRect(80, 120, 120, 40)
	for _, c := range f.Outside {
		mgr := focus.NewManager()
		st := overlay.New()
		tr := behavior.NewTrigger(behavior.TriggerConfig{
			Want:      overlay.Bottom,
			Barrier:   true,
			FocusTrap: true,
			EscCloses: true,
		}, mgr, st)
		tr.Open(anchor, 200, 120)
		st.Layout(1200, 800)
		closed := tr.HandleOutside(rendering.Point{X: c.Point.X, Y: c.Point.Y})
		if closed != c.WantClosed {
			t.Fatalf("%s: closed=%v want %v", c.Name, closed, c.WantClosed)
		}
		if tr.IsOpen() != c.WantOpen {
			t.Fatalf("%s: open=%v want %v", c.Name, tr.IsOpen(), c.WantOpen)
		}
		tr.Close()
	}
	for _, c := range f.Esc {
		mgr := focus.NewManager()
		st := overlay.New()
		tr := behavior.NewTrigger(behavior.TriggerConfig{
			Want:      overlay.Bottom,
			Barrier:   true,
			FocusTrap: true,
			EscCloses: true,
		}, mgr, st)
		tr.Open(anchor, 200, 120)
		if tr.FocusNode() == nil || !tr.IsFocusInside(tr.FocusNode()) {
			t.Fatalf("%s: focus must be trapped", c.Name)
		}
		var consumed bool
		if c.PressEsc {
			consumed = tr.HandleKey(focus.KeyEvent{KeyCode: 0xFF1B, Pressed: true})
		}
		if consumed != c.WantClosed {
			t.Fatalf("%s: esc consumed=%v want %v", c.Name, consumed, c.WantClosed)
		}
		if tr.IsOpen() != c.WantOpen {
			t.Fatalf("%s: open=%v want %v", c.Name, tr.IsOpen(), c.WantOpen)
		}
	}
}
