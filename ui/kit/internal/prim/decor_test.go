package prim_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

func mustTight(t *testing.T, w, h float64) rendering.Constraints {
	t.Helper()
	return rendering.Tight(w, h)
}

func mustPoint(x, y float64) rendering.Point { return rendering.Point{X: x, Y: y} }

type decorFile struct {
	Blend    []blendCase    `json:"blendCases"`
	RRect    []hitCase      `json:"rrectCases"`
	Oval     []hitCase      `json:"ovalCases"`
	Gradient []gradientCase `json:"gradientCases"`
}

type blendCase struct {
	Name    string     `json:"name"`
	Dst     [3]float64 `json:"dst"`
	Src     [3]float64 `json:"src"`
	Opacity float64    `json:"opacity"`
	Want    [3]float64 `json:"want"`
}

type hitCase struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	R    float64 `json:"r"`
	Want bool    `json:"want"`
}

type gradientCase struct {
	Name     string  `json:"name"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	MidT     float64 `json:"midT"`
	WantFrom string  `json:"wantFrom"`
	WantTo   string  `json:"wantTo"`
}

func loadDecor(t *testing.T) decorFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "decor_cases.json"))
	if err != nil {
		t.Fatalf("read decor_cases.json: %v", err)
	}
	var f decorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode decor_cases.json: %v", err)
	}
	return f
}

// TestPrim_Decor checks group blend math (容差≤12/通道), rounded hit
// (Hit ≡ 绘), shadow exclusion and gradient endpoints plus monotonicity.
func TestPrim_Decor(t *testing.T) {
	f := loadDecor(t)

	t.Run("blend_group_formula", func(t *testing.T) {
		for _, c := range f.Blend {
			got := prim.BlendSrcOver(c.Dst, c.Src, c.Opacity)
			for k := 0; k < 3; k++ {
				if math.Abs(got[k]-c.Want[k]) > 12.0/255 {
					t.Fatalf("%s channel %d = %.4f want %.4f", c.Name, k, got[k], c.Want[k])
				}
			}
		}
		if got := prim.BlendSrcOver([3]float64{0.5, 0.5, 0.5}, [3]float64{1, 1, 1}, 0.65); math.Abs(got[0]-0.825) > 1e-9 {
			t.Fatalf("0.65 white over grey = %.4f want 0.825", got[0])
		}
	})

	t.Run("rrect_hit_matches_paint", func(t *testing.T) {
		for _, c := range f.RRect {
			if got := prim.RRectContains(c.X, c.Y, c.W, c.H, c.R); got != c.Want {
				t.Fatalf("%s = %v want %v", c.Name, got, c.Want)
			}
		}
		// Live node must agree with the pure math (Hit ≡ 绘).
		inner := prim.Colored(100, 60, theme.Hex("#1677ff"))
		clip := prim.NewClipRRect(12, inner)
		clip.Layout(mustTight(t, 100, 60))
		if hit := clip.HitTest(mustPoint(1, 1)); hit != nil {
			t.Fatal("live clip corner hole must miss")
		}
		if hit := clip.HitTest(mustPoint(50, 30)); hit == nil {
			t.Fatal("live clip center must hit")
		}
	})

	t.Run("oval_hit", func(t *testing.T) {
		for _, c := range f.Oval {
			if got := prim.OvalContains(c.X, c.Y, c.W, c.H); got != c.Want {
				t.Fatalf("%s = %v want %v", c.Name, got, c.Want)
			}
		}
	})

	t.Run("shadow_never_expands_hit", func(t *testing.T) {
		// A point outside the rounded box is not a hit even when a
		// shadow is painted with an offset.
		if prim.RRectContains(101, 30, 100, 60, 12) {
			t.Fatal("outside point must miss the box")
		}
		if !prim.ShadowHitExcluded(101, 30, 100, 60, 12, 0, 4) {
			t.Fatal("shadow ring point must stay excluded from hit")
		}
		spec := prim.DecorSpec{Radius: 12, ShadowDX: 0, ShadowDY: 4, ShadowColor: theme.RGBA(0, 0, 0, 0.3)}
		node := prim.NewDecoratedBox(100, 60, spec, nil)
		node.Layout(mustTight(t, 100, 60))
		if hit := node.HitTest(mustPoint(101, 30)); hit != nil {
			t.Fatal("decorated shadow must not expand hit")
		}
	})

	t.Run("gradient_endpoints_monotonic", func(t *testing.T) {
		for _, c := range f.Gradient {
			from, to := theme.Hex(c.From), theme.Hex(c.To)
			if got := prim.GradientSample(from, to, 0); got != theme.Hex(c.WantFrom) {
				t.Fatalf("%s t=0 = %+v want %s", c.Name, got, c.WantFrom)
			}
			if got := prim.GradientSample(from, to, 1); got != theme.Hex(c.WantTo) {
				t.Fatalf("%s t=1 = %+v want %s", c.Name, got, c.WantTo)
			}
			mid := prim.GradientSample(from, to, c.MidT)
			if !prim.GradientMonotonic(from, mid, to) {
				t.Fatalf("%s midpoint %+v breaks monotonicity", c.Name, mid)
			}
		}
	})

	t.Run("resolve_prefers_props_over_seed", func(t *testing.T) {
		seed := theme.DefaultTokens()
		custom := theme.Hex("#722ed1")
		got := prim.ResolveDecor(prim.DecorProps{Bg: &custom}, nil, seed)
		if got.Bg != custom {
			t.Fatal("explicit bg must win")
		}
		def := prim.ResolveDecor(prim.DecorProps{}, nil, seed)
		if def.Bg != seed.ColorBgContainer {
			t.Fatal("empty props must fall back to seed")
		}
		if def.BorderWidth != seed.LineWidth || def.Radius != seed.Radius {
			t.Fatal("border and radius must come from seed")
		}
	})
}
