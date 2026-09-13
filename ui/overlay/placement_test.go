package overlay_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
)

type placementFile struct {
	Defaults struct {
		Gap          float64 `json:"gap"`
		ArrowWidth   float64 `json:"arrowWidth"`
		ArrowOffsetH float64 `json:"arrowOffsetH"`
		ArrowOffsetV float64 `json:"arrowOffsetV"`
		ViewportW    float64 `json:"viewportW"`
		ViewportH    float64 `json:"viewportH"`
	} `json:"defaults"`
	Cases []struct {
		Name        string    `json:"name"`
		Anchor      []float64 `json:"anchor"`
		Overlay     []float64 `json:"overlay"`
		Want        string    `json:"want"`
		Expect      *struct {
			X       *float64 `json:"x"`
			Y       *float64 `json:"y"`
			Actual  string   `json:"actual"`
			Flipped *bool    `json:"flipped"`
		} `json:"expect"`
		ExpectArrow []float64 `json:"expectArrow"`
	} `json:"cases"`
}

func loadPlacements(t *testing.T) placementFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "placements.json"))
	if err != nil {
		t.Fatalf("read placements: %v", err)
	}
	var f placementFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse placements: %v", err)
	}
	return f
}

// TestPlacement_AntTwelveDrivesFromFile checks ideal origins, edge flips,
// viewport shift clamping and arrow pinning against testdata cases.
func TestPlacement_AntTwelveDrivesFromFile(t *testing.T) {
	f := loadPlacements(t)
	for _, c := range f.Cases {
		if len(c.Anchor) != 4 || len(c.Overlay) != 2 {
			t.Fatalf("%s: bad geometry", c.Name)
		}
		anchor := rendering.NewRect(c.Anchor[0], c.Anchor[1], c.Anchor[2], c.Anchor[3])
		got := overlay.Resolve(anchor, c.Overlay[0], c.Overlay[1], overlay.Placement(c.Want), &overlay.ResolveOptions{
			Gap:          f.Defaults.Gap,
			ArrowWidth:   f.Defaults.ArrowWidth,
			ArrowOffsetH: f.Defaults.ArrowOffsetH,
			ArrowOffsetV: f.Defaults.ArrowOffsetV,
			ViewportW:    f.Defaults.ViewportW,
			ViewportH:    f.Defaults.ViewportH,
			Flip:         true,
			Shift:        true,
		})
		if c.Expect != nil {
			if c.Expect.Actual != "" && string(got.Actual) != c.Expect.Actual {
				t.Fatalf("%s: actual=%s want %s", c.Name, got.Actual, c.Expect.Actual)
			}
			if c.Expect.Flipped != nil && got.Flipped != *c.Expect.Flipped {
				t.Fatalf("%s: flipped=%v want %v", c.Name, got.Flipped, *c.Expect.Flipped)
			}
			if c.Expect.X != nil && got.X != *c.Expect.X {
				t.Fatalf("%s: x=%v want %v", c.Name, got.X, *c.Expect.X)
			}
			if c.Expect.Y != nil && got.Y != *c.Expect.Y {
				t.Fatalf("%s: y=%v want %v", c.Name, got.Y, *c.Expect.Y)
			}
		}
		if len(c.ExpectArrow) == 2 {
			if got.ArrowX != c.ExpectArrow[0] || got.ArrowY != c.ExpectArrow[1] {
				t.Fatalf("%s: arrow=(%v,%v) want (%v,%v)", c.Name, got.ArrowX, got.ArrowY, c.ExpectArrow[0], c.ExpectArrow[1])
			}
		}
		// Overlay must stay inside the viewport after shift.
		if got.X < 0 || got.Y < 0 || got.X+c.Overlay[0] > f.Defaults.ViewportW+1e-9 || got.Y+c.Overlay[1] > f.Defaults.ViewportH+1e-9 {
			t.Fatalf("%s: out of viewport (%v,%v)", c.Name, got.X, got.Y)
		}
	}
}

func TestPlacement_FlipTable(t *testing.T) {
	pairs := map[overlay.Placement]overlay.Placement{
		overlay.Top: BottomOf(overlay.Top), overlay.Bottom: overlay.Top,
	}
	_ = pairs
	want := map[overlay.Placement]overlay.Placement{
		overlay.Top: overlay.Bottom, overlay.Bottom: overlay.Top,
		overlay.TopLeft: overlay.BottomLeft, overlay.BottomLeft: overlay.TopLeft,
		overlay.TopRight: overlay.BottomRight, overlay.BottomRight: overlay.TopRight,
		overlay.Left: overlay.Right, overlay.Right: overlay.Left,
		overlay.LeftTop: overlay.RightTop, overlay.RightTop: overlay.LeftTop,
		overlay.LeftBottom: overlay.RightBottom, overlay.RightBottom: overlay.LeftBottom,
	}
	for k, w := range want {
		if overlay.FlipPlacement(k) != w {
			t.Fatalf("flip(%s)=%s want %s", k, overlay.FlipPlacement(k), w)
		}
	}
	if len(overlay.AllPlacements) != 12 {
		t.Fatalf("placements=%d want 12", len(overlay.AllPlacements))
	}
}

func BottomOf(p overlay.Placement) overlay.Placement { return overlay.FlipPlacement(p) }

func TestInteract_OutsideAndFocusLock(t *testing.T) {
	st := overlay.New()
	if overlay.HitOutside(st, rendering.Point{X: 5, Y: 5}) {
		t.Fatal("empty state is not outside-close")
	}
	st.Insert(overlay.NewBarrierEntry(0, 0, 200, 200, nil))
	st.Layout(200, 200)
	if overlay.HitOutside(st, rendering.Point{X: 50, Y: 50}) {
		t.Fatal("barrier hit should not count as outside")
	}
	if !overlay.HitOutside(st, rendering.Point{X: 250, Y: 250}) {
		t.Fatal("miss should count as outside")
	}

	mgr := focus.NewManager()
	a := focus.NewFocusNode("overlay-ok")
	b := focus.NewFocusNode("overlay-cancel")
	out := focus.NewFocusNode("page")
	mgr.Register(a)
	mgr.Register(b)
	mgr.Register(out)
	mgr.RequestFocus(out)
	lock := overlay.NewFocusLock(mgr, a, b)
	lock.Acquire()
	if !lock.Held() || mgr.Primary() != a {
		t.Fatalf("acquire primary=%v", mgr.Primary())
	}
	if !lock.Contains(a) || lock.Contains(out) {
		t.Fatal("contains mismatch")
	}
	lock.Release()
	if lock.Held() || mgr.Primary() != out {
		t.Fatalf("release primary=%v", mgr.Primary())
	}
}
