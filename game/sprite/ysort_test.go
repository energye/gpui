package sprite

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type ysortItemDef struct {
	Name  string  `json:"name"`
	Layer int     `json:"layer"`
	FeetY float64 `json:"feet_y"`
}

type ysortCase struct {
	Name  string         `json:"name"`
	Items []ysortItemDef `json:"items"`
	Want  []string       `json:"want"`
}

type ysortFile struct {
	Layers map[string]int `json:"layers"`
	Cases  []ysortCase    `json:"cases"`
}

func loadYSortCases(t *testing.T) ysortFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "ysort_cases.json"))
	if err != nil {
		t.Fatalf("read ysort_cases.json: %v", err)
	}
	var cases ysortFile
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decode ysort_cases.json: %v", err)
	}
	if len(cases.Cases) == 0 {
		t.Fatal("ysort_cases.json has no cases")
	}
	return cases
}

func mustFindCase(t *testing.T, cases ysortFile, name string) ysortCase {
	t.Helper()
	for _, c := range cases.Cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("ysort_cases.json has no %s", name)
	return ysortCase{}
}

func ysortItems(t *testing.T, defs []ysortItemDef) []Item {
	t.Helper()
	out := make([]Item, len(defs))
	for i, d := range defs {
		it, err := NewItem(d.Name, Layer(d.Layer), d.FeetY)
		if err != nil {
			t.Fatalf("NewItem %q: %v", d.Name, err)
		}
		out[i] = it
	}
	return out
}

func ysortNames(got []Item) []string {
	names := make([]string, len(got))
	for i, it := range got {
		names[i] = it.Name
	}
	return names
}

func equalNames(got []Item, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Name != want[i] {
			return false
		}
	}
	return true
}

// A: feet Y plus layer lands on the frozen draw order, UI never covered.
func TestYSortOrderFromCases(t *testing.T) {
	cases := loadYSortCases(t)
	if cases.Layers["world"] != int(LayerWorld) ||
		cases.Layers["fx"] != int(LayerFX) ||
		cases.Layers["ui"] != int(LayerUI) {
		t.Fatalf("layer bands = %v, want world=%d fx=%d ui=%d",
			cases.Layers, int(LayerWorld), int(LayerFX), int(LayerUI))
	}
	if !(LayerWorld < LayerFX && LayerFX < LayerUI) {
		t.Fatal("layer order broken: want world < fx < ui")
	}
	for _, c := range cases.Cases {
		items := ysortItems(t, c.Items)
		got, err := Sort(items)
		if err != nil {
			t.Errorf("%s: Sort: %v", c.Name, err)
			continue
		}
		if !equalNames(got, c.Want) {
			t.Errorf("%s: order = %v, want %v", c.Name, ysortNames(got), c.Want)
		}
		if !IsSorted(got) {
			t.Errorf("%s: sorted result reports not sorted", c.Name)
		}
	}
}

// B: empty/zero/tie/bad inputs never panic, flicker, or mutate.
func TestYSortEdgesNoCrash(t *testing.T) {
	// Empty and single are already sorted.
	if !IsSorted(nil) {
		t.Error("nil reports not sorted, want true")
	}
	solo, err := NewItem("solo", LayerFX, 42)
	if err != nil {
		t.Fatalf("NewItem solo: %v", err)
	}
	if !IsSorted([]Item{solo}) {
		t.Error("single reports not sorted, want true")
	}
	// Zero and negative feet Y are legal positions.
	for _, y := range []float64{0, -100, -0.5} {
		if _, err := NewItem("p", LayerWorld, y); err != nil {
			t.Errorf("feetY %v: unexpected error %v", y, err)
		}
	}
	// Custom bands order like the frozen ones: bigger covers smaller.
	lo, err := NewItem("lo", Layer(5), 0)
	if err != nil {
		t.Fatalf("custom layer: %v", err)
	}
	hi, err := NewItem("hi", Layer(6), -1000)
	if err != nil {
		t.Fatalf("custom layer: %v", err)
	}
	got, err := Sort([]Item{hi, lo})
	if err != nil {
		t.Fatalf("custom Sort: %v", err)
	}
	if len(got) != 2 || got[0].Name != "lo" || got[1].Name != "hi" {
		t.Errorf("custom layer order = %v, want [lo hi]", ysortNames(got))
	}
	// Non-finite feet Y is rejected, never guessed.
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := NewItem("bad", LayerWorld, bad); err == nil {
			t.Errorf("NewItem feetY %v: want error", bad)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("NewItem feetY %v code = %v, want invalid-arg", bad, core.CodeOf(err))
		}
		badItems := []Item{{Name: "ok", Layer: LayerWorld, FeetY: 1}, {Name: "bad", Layer: LayerWorld, FeetY: bad}}
		snapName, snapLayer := badItems[1].Name, badItems[1].Layer
		if out, err := Sort(badItems); err == nil || out != nil {
			t.Errorf("Sort bad feetY %v: out=%v err=%v, want nil + error", bad, out, err)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Sort bad feetY %v code = %v, want invalid-arg", bad, core.CodeOf(err))
		}
		// NaN never equals itself, so the untouched check compares only
		// the name and layer slots here; value replay lives in TestYSortBoundaryIdentical.
		if badItems[1].Name != snapName || badItems[1].Layer != snapLayer {
			t.Errorf("Sort bad feetY %v mutated input", bad)
		}
		if IsSorted(badItems) {
			t.Errorf("IsSorted bad feetY %v = true, want false", bad)
		}
	}
	// Same layer and same Y keeps the input order across repeated sorts.
	tie := mustFindCase(t, loadYSortCases(t), "stable_tie")
	items := ysortItems(t, tie.Items)
	first, err := Sort(items)
	if err != nil {
		t.Fatalf("tie Sort: %v", err)
	}
	for i := 0; i < 100; i++ {
		again, err := Sort(items)
		if err != nil {
			t.Fatalf("tie rep %d: %v", i, err)
		}
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("tie rep %d flickered at %d: %v vs %v", i, j, ysortNames(again), ysortNames(first))
			}
		}
	}
	// Huge-but-finite heights sort without NaN or panic.
	huge := []Item{
		{Name: "a", Layer: LayerWorld, FeetY: 1e308},
		{Name: "b", Layer: LayerWorld, FeetY: -1e308},
	}
	got, err = Sort(huge)
	if err != nil {
		t.Fatalf("huge Sort: %v", err)
	}
	if len(got) != 2 || got[0].Name != "b" || got[1].Name != "a" {
		t.Errorf("huge order = %v, want [b a]", ysortNames(got))
	}
}

// C does not need pixels (pure math, draws nothing): the number path must
// be lossless and replays bitwise identical instead.
func TestYSortBoundaryIdentical(t *testing.T) {
	cases := loadYSortCases(t)
	for _, c := range cases.Cases {
		items := ysortItems(t, c.Items)
		snapshot := append([]Item(nil), items...)
		a, err := Sort(items)
		if err != nil {
			t.Errorf("%s: Sort: %v", c.Name, err)
			continue
		}
		// Input untouched: sorting never mutates the caller's slice.
		for i := range items {
			if items[i] != snapshot[i] {
				t.Errorf("%s: input mutated at %d", c.Name, i)
				break
			}
		}
		// Result is a fresh slice: writing it cannot alias the input.
		if len(a) > 0 {
			probe := append([]Item(nil), a...)
			a[0].Name = "mutated-probe"
			if items[0].Name == "mutated-probe" {
				t.Errorf("%s: result aliases input", c.Name)
			}
			a = probe
		}
		b, err := Sort(items)
		if err != nil {
			t.Errorf("%s: second Sort: %v", c.Name, err)
			continue
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("%s: replay diverged at %d: %+v vs %+v", c.Name, i, a[i], b[i])
				break
			}
		}
		if !IsSorted(b) {
			t.Errorf("%s: sorted result reports not sorted", c.Name)
		}
	}
}

// D: 100 sprites sort with a measured cost.
func TestYSortPerfHundred(t *testing.T) {
	// Synthetic load only (no golden): golden order stays in
	// ysort_cases.json. Seeded rand keeps the load replayable.
	r := core.NewRand(20260915)
	const n = 100
	items := make([]Item, n)
	for i := 0; i < n; i++ {
		layer := LayerWorld
		switch i % 3 {
		case 1:
			layer = LayerFX
		case 2:
			layer = LayerUI
		}
		y := r.RangeFloat(-500, 1500)
		it, err := NewItem("p", layer, y)
		if err != nil {
			t.Fatalf("item %d: %v", i, err)
		}
		// Perf needs no unique debug keys: order depends only on layer and Y.
		items[i] = it
	}
	const reps = 2000
	start := time.Now()
	for i := 0; i < reps; i++ {
		got, err := Sort(items)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if len(got) != n || !IsSorted(got) {
			t.Fatalf("rep %d: not sorted", i)
		}
	}
	el := time.Since(start)
	t.Logf("ysort-100: %d sorts x %d items in %v (%.1f us/sort)", reps, n, el, float64(el.Microseconds())/reps)
}

// E: long runs keep the same order with no drift or input damage.
func TestYSortLongRunStable(t *testing.T) {
	split := mustFindCase(t, loadYSortCases(t), "layer_split")
	items := ysortItems(t, split.Items)
	snapshot := append([]Item(nil), items...)
	first, err := Sort(items)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	for i := 0; i < 10000; i++ {
		got, err := Sort(items)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		for j := range first {
			if got[j] != first[j] {
				t.Fatalf("rep %d drifted at %d: %v vs %v", i, j, ysortNames(got), ysortNames(first))
			}
		}
	}
	for i := range items {
		if items[i] != snapshot[i] {
			t.Fatal("10k sorts mutated the input")
		}
	}
	// A walking hero crosses a tree and comes back: the order flips and
	// returns, never sticking in the wrong half.
	tree, err := NewItem("tree", LayerWorld, 100)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	for _, y := range []float64{50, 150, 50} {
		hero, err := NewItem("hero", LayerWorld, y)
		if err != nil {
			t.Fatalf("hero %v: %v", y, err)
		}
		got, err := Sort([]Item{hero, tree})
		if err != nil {
			t.Fatalf("walk %v: %v", y, err)
		}
		wantFirst := "hero"
		if y > 100 {
			wantFirst = "tree"
		}
		if got[0].Name != wantFirst {
			t.Errorf("walk y=%v first = %q, want %q", y, got[0].Name, wantFirst)
		}
	}
}

// F: offscreen golden stands in for the window (window-exempt pure math).
// The frozen order in ysort_cases.json is the evidence both backends share;
// shape assertions below pin the meaning, not just the names.
func TestYSortOffscreenGolden(t *testing.T) {
	split := mustFindCase(t, loadYSortCases(t), "layer_split")
	got, err := Sort(ysortItems(t, split.Items))
	if err != nil {
		t.Fatalf("layer_split Sort: %v", err)
	}
	if !equalNames(got, split.Want) {
		t.Fatalf("golden = %v, want %v", ysortNames(got), split.Want)
	}
	// Shape: UI draws last so effects never cover it; world Y rises in order.
	if got[len(got)-1].Layer != LayerUI {
		t.Errorf("last = %+v, want UI layer on top", got[len(got)-1])
	}
	var lastWorldY float64
	worldSeen := false
	for _, it := range got {
		if it.Layer != LayerWorld {
			continue
		}
		if worldSeen && it.FeetY < lastWorldY {
			t.Errorf("world Y falls: %v after %v", it.FeetY, lastWorldY)
		}
		lastWorldY, worldSeen = it.FeetY, true
	}
	// Band order: fx sits between world and UI, never after UI.
	seenFX, seenUI := false, false
	for _, it := range got {
		switch it.Layer {
		case LayerFX:
			seenFX = true
			if seenUI {
				t.Error("fx draws after UI, want fx below UI")
			}
		case LayerUI:
			seenUI = true
		}
	}
	if !seenFX || !seenUI {
		t.Errorf("golden bands incomplete: fx=%v ui=%v", seenFX, seenUI)
	}
}
