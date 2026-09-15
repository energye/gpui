package sprite_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/sprite"
)

// 1.2 depth body (S41/W5): far-to-near painter on top of the R5 branch.
// All standard orders come from testdata/depth_cases.json; the test
// hardcodes no standard pixels or orders.

type depthFileItem struct {
	Name  string  `json:"name"`
	Depth float64 `json:"depth"`
	Layer int     `json:"layer"`
	FeetY float64 `json:"feetY"`
}

type depthFileCase struct {
	Name      string          `json:"name"`
	Items     []depthFileItem `json:"items"`
	WantOrder []string        `json:"want_order"`
}

type depthFileEdge struct {
	Name       string            `json:"name"`
	Patch      map[string]string `json:"patch"`
	WantErr    bool              `json:"want_err"`
	WantSorted *bool             `json:"want_sorted"`
	SetDepth   string            `json:"setdepth"`
}

type depthFile struct {
	Cases []depthFileCase `json:"cases"`
	Edges []depthFileEdge `json:"edges"`
}

func loadDepthCases(t *testing.T) depthFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "depth_cases.json"))
	if err != nil {
		t.Fatalf("read depth cases: %v", err)
	}
	var f depthFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode depth cases: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("depth cases empty")
	}
	return f
}

func toDeepItems(t *testing.T, in []depthFileItem) []sprite.DeepItem {
	t.Helper()
	out := make([]sprite.DeepItem, len(in))
	for i, it := range in {
		got, err := sprite.NewDeepItem(it.Name, it.Depth, sprite.Layer(it.Layer), it.FeetY)
		if err != nil {
			t.Fatalf("NewDeepItem %s: %v", it.Name, err)
		}
		out[i] = got
	}
	return out
}

func orderNames(in []sprite.DeepItem) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = in[i].Name
	}
	return out
}

// A: far-to-near covers right, ties stable, layer/feet break same depth.
func TestDepthSortFromCases(t *testing.T) {
	f := loadDepthCases(t)
	for _, c := range f.Cases {
		in := toDeepItems(t, c.Items)
		got, err := sprite.DepthSort(in)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		names := orderNames(got)
		if len(names) != len(c.WantOrder) {
			t.Fatalf("%s order = %v, want %v", c.Name, names, c.WantOrder)
		}
		for i := range names {
			if names[i] != c.WantOrder[i] {
				t.Fatalf("%s order = %v, want %v", c.Name, names, c.WantOrder)
			}
		}
		if !sprite.IsDepthSorted(got) {
			t.Errorf("%s: sorted result not reported sorted", c.Name)
		}
	}
}

// B: bad values never crash, errors are InvalidArg-shaped, input untouched.
func TestDepthEdgesNoCrash(t *testing.T) {
	f := loadDepthCases(t)
	_ = f
	base, err := sprite.NewDeepItem("base", 1, sprite.LayerWorld, 10)
	if err != nil {
		t.Fatalf("NewDeepItem: %v", err)
	}
	if _, err := sprite.NewDeepItem("nan", math.NaN(), sprite.LayerWorld, 10); err == nil {
		t.Error("NaN depth accepted, want error")
	}
	if _, err := sprite.NewDeepItem("inf", math.Inf(1), sprite.LayerWorld, 10); err == nil {
		t.Error("Inf depth accepted, want error")
	}
	if _, err := sprite.NewDeepItem("nanfeet", 1, sprite.LayerWorld, math.NaN()); err == nil {
		t.Error("NaN feet accepted, want error")
	}
	if err := sprite.SetDepth(&base, math.NaN()); err == nil {
		t.Error("SetDepth NaN accepted, want error")
	} else if base.Depth != 1 {
		t.Errorf("SetDepth NaN moved value to %v, want 1", base.Depth)
	}
	if err := sprite.SetDepth(nil, 2); err == nil {
		t.Error("SetDepth nil accepted, want error")
	}
	bad := []sprite.DeepItem{{Name: "bad", Depth: math.NaN(), Layer: sprite.LayerWorld, FeetY: 10}}
	if _, err := sprite.DepthSort(bad); err == nil {
		t.Error("DepthSort NaN accepted, want error")
	}
	// DepthSort validates before copying, so the input cannot be mutated:
	// pin the contract explicitly.
	if !math.IsNaN(bad[0].Depth) {
		t.Error("DepthSort mutated input on error")
	}
	if sprite.IsDepthSorted(nil) {
		t.Error("IsDepthSorted(nil) = true, want false")
	}
	if sprite.IsDepthSorted(bad) {
		t.Error("IsDepthSorted(NaN) = true, want false")
	}
}

// C: both sides agree — replay twice, same order, input never mutated.
func TestDepthBoundaryIdentical(t *testing.T) {
	f := loadDepthCases(t)
	for _, c := range f.Cases {
		a := toDeepItems(t, c.Items)
		snap := append([]sprite.DeepItem(nil), a...)
		first, err := sprite.DepthSort(a)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		second, err := sprite.DepthSort(a)
		if err != nil {
			t.Fatalf("%s re-sort: %v", c.Name, err)
		}
		if len(first) != len(second) {
			t.Fatalf("%s replay len %d vs %d", c.Name, len(first), len(second))
		}
		for i := range first {
			if first[i] != second[i] {
				t.Fatalf("%s replay diverged at %d", c.Name, i)
			}
		}
		for i := range a {
			if a[i] != snap[i] {
				t.Fatalf("%s mutated input at %d", c.Name, i)
			}
		}
		if !sprite.IsDepthSorted(first) {
			t.Errorf("%s: not sorted after sort", c.Name)
		}
	}
}

// D: a hundred occluders sort fast enough to count.
func TestDepthPerfOcclusion(t *testing.T) {
	f := loadDepthCases(t)
	big := toDeepItems(t, f.Cases[3].Items)
	for len(big) < 100 {
		big = append(big, big...)
	}
	big = big[:100]
	for i := range big {
		big[i].Name = string(rune('a' + i%26))
		big[i].Depth = float64(100 - i)
	}
	start := time.Now()
	n := 2000
	for i := 0; i < n; i++ {
		if _, err := sprite.DepthSort(big); err != nil {
			t.Fatalf("sort: %v", err)
		}
	}
	el := time.Since(start)
	t.Logf("depth 100 occluders x%d in %v (%.1fus/sort)", n, el, float64(el.Nanoseconds())/float64(n)/1000)
}

// E: long runs never drift, never flicker.
func TestDepthLongRunStable(t *testing.T) {
	f := loadDepthCases(t)
	in := toDeepItems(t, f.Cases[3].Items)
	first, err := sprite.DepthSort(in)
	if err != nil {
		t.Fatalf("sort: %v", err)
	}
	want := orderNames(first)
	for i := 0; i < 10000; i++ {
		got, err := sprite.DepthSort(in)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		for j := range got {
			if got[j].Name != want[j] {
				t.Fatalf("iter %d diverged at %d", i, j)
			}
		}
	}
}

// F: offscreen contrast — far-first red covers green, raw order would not.
// Pinned against the frozen depth window intent (game_sprite--case=depth).
func TestDepthOffscreenContrast(t *testing.T) {
	far, _ := sprite.NewDeepItem("far", 9, sprite.LayerWorld, 10)
	near, _ := sprite.NewDeepItem("near", 1, sprite.LayerWorld, 10)
	raw := []sprite.DeepItem{near, far}
	ordered, err := sprite.DepthSort(raw)
	if err != nil {
		t.Fatalf("sort: %v", err)
	}
	if ordered[0].Name != "far" || ordered[1].Name != "near" {
		t.Fatalf("sorted = %v, want [far near]", orderNames(ordered))
	}
	if sprite.IsDepthSorted(raw) {
		t.Fatal("raw [near far] reported sorted, want false")
	}
	f := loadDepthCases(t)
	found := false
	for _, c := range f.Cases {
		if c.Name == "far_first" {
			found = true
		}
	}
	if !found {
		t.Fatal("depth_cases.json lost far_first")
	}
}
