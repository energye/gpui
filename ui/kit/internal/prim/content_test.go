package prim_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type contentFile struct {
	Estimate []estimateCase `json:"estimateCases"`
	Merge    []mergeCase    `json:"mergeCases"`
	Icon     []iconCase     `json:"iconCases"`
}

type estimateCase struct {
	Name      string  `json:"name"`
	Text      string  `json:"text"`
	FontSize  float64 `json:"fontSize"`
	ApproxW   float64 `json:"approxW"`
	LineMult  float64 `json:"lineMult"`
	MaxWidth  float64 `json:"maxWidth"`
	MaxLines  int     `json:"maxLines"`
	WantW     float64 `json:"wantW"`
	WantH     float64 `json:"wantH"`
	WantLines int     `json:"wantLines"`
}

type styleJSON struct {
	FontSize float64 `json:"fontSize"`
}

type mergeCase struct {
	Name     string    `json:"name"`
	Parent   styleJSON `json:"parent"`
	Child    styleJSON `json:"child"`
	WantSize float64   `json:"wantSize"`
}

type iconCase struct {
	Name     string  `json:"name"`
	Size     string  `json:"size"`
	Disabled bool    `json:"disabled"`
	WantPx   float64 `json:"wantPx"`
}

func loadContent(t *testing.T) contentFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "content_cases.json"))
	if err != nil {
		t.Fatalf("read content_cases.json: %v", err)
	}
	var f contentFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode content_cases.json: %v", err)
	}
	if len(f.Estimate) == 0 {
		t.Fatal("content_cases.json holds no estimate cases")
	}
	return f
}

// TestPrim_Content checks style merge, estimate sizes, ellipsis clamp,
// picture three-state flow and icon theme sizing.
func TestPrim_Content(t *testing.T) {
	f := loadContent(t)
	seed := theme.DefaultTokens()

	t.Run("estimate_sizes", func(t *testing.T) {
		for _, c := range f.Estimate {
			got := prim.EstimateSize(c.Text, c.FontSize, c.ApproxW, c.LineMult, c.MaxWidth, c.MaxLines, c.MaxLines > 0)
			if c.WantLines > 0 {
				lines := int(math.Round(got.Height / (c.FontSize * c.LineMult)))
				if lines != c.WantLines {
					t.Fatalf("%s lines = %d want %d", c.Name, lines, c.WantLines)
				}
				continue
			}
			if math.Abs(got.Width-c.WantW) > 1e-6 || math.Abs(got.Height-c.WantH) > 1e-6 {
				t.Fatalf("%s = %.3fx%.3f want %.3fx%.3f", c.Name, got.Width, got.Height, c.WantW, c.WantH)
			}
		}
	})

	t.Run("style_merge", func(t *testing.T) {
		for _, c := range f.Merge {
			parent := prim.TextStyle{FontSize: c.Parent.FontSize}
			child := prim.TextStyle{FontSize: c.Child.FontSize}
			if got := child.Merge(parent); got.FontSize != c.WantSize {
				t.Fatalf("%s = %.2f want %.2f", c.Name, got.FontSize, c.WantSize)
			}
		}
		red := theme.Hex("#ff4d4f")
		withColor := prim.TextStyle{Color: red, HasColor: true}
		if got := withColor.Merge(prim.TextStyle{FontSize: 12}); !got.HasColor || got.Color != red || got.FontSize != 12 {
			t.Fatalf("color merge keeps color and inherits size, got %+v", got)
		}
		base := prim.ResolveTextStyle(seed)
		if base.FontSize != seed.FontSize || base.Color != seed.ColorText {
			t.Fatalf("seed style must carry size+text color, got %+v", base)
		}
	})

	t.Run("ellipsis_flag", func(t *testing.T) {
		if !prim.Ellipsized(4, 2) {
			t.Fatal("4 lines over max 2 must ellipsize")
		}
		if prim.Ellipsized(2, 2) {
			t.Fatal("at-limit content must not ellipsize")
		}
	})

	t.Run("picture_three_states", func(t *testing.T) {
		spec := prim.ResolvePicture(200, 120, seed)
		if spec.Fill != seed.ColorFillSecondary {
			t.Fatal("placeholder fill must come from seed")
		}
		im := prim.NewPicture(spec)
		if got := prim.PicturePhase(im); got != prim.PicturePlaceholder {
			t.Fatalf("fresh picture = %v want placeholder", got)
		}
		im.SetLoading()
		if got := prim.PicturePhase(im); got != prim.PicturePlaceholder {
			t.Fatalf("loading picture = %v want placeholder", got)
		}
		im.SetError()
		if got := prim.PicturePhase(im); got != prim.PictureError {
			t.Fatalf("error picture = %v want error", got)
		}
	})

	t.Run("icon_follows_ctx", func(t *testing.T) {
		for _, c := range f.Icon {
			ctx := scope.DefaultCtx()
			switch c.Size {
			case "small":
				ctx = ctx.WithSize(scope.SizeSmall)
			case "large":
				ctx = ctx.WithSize(scope.SizeLarge)
			default:
				ctx = ctx.WithSize(scope.SizeMedium)
			}
			ctx = ctx.WithDisabled(c.Disabled)
			got := prim.ResolveIcon(ctx, seed)
			if got.Size != c.WantPx {
				t.Fatalf("%s size = %.1f want %.1f", c.Name, got.Size, c.WantPx)
			}
			if got.Color != seed.ColorTextSecondary {
				t.Fatalf("%s color must follow seed secondary", c.Name)
			}
			if sz := prim.IconBoxSize(got); sz.Width != c.WantPx || sz.Height != c.WantPx {
				t.Fatalf("%s box = %+v want square %.1f", c.Name, sz, c.WantPx)
			}
		}
		disabled := prim.ResolveIcon(scope.DefaultCtx().WithDisabled(true), seed)
		if disabled.Color != seed.ColorTextDisabled {
			t.Fatal("disabled icon must dim to the disabled token")
		}
	})

	t.Run("label_builds_nodes", func(t *testing.T) {
		lb := prim.NewLabel(prim.LabelProps{Text: "hi", Seed: seed, MaxWidth: 200})
		if lb == nil || lb.Text != "hi" {
			t.Fatal("label must carry text")
		}
		rich := prim.NewRichLabel(prim.RichProps{
			Spans: []prim.Span{{Text: "a"}, {Text: "b"}},
			Seed:  seed,
		})
		if rich == nil || len(rich.Runs) != 2 {
			t.Fatalf("rich label must hold 2 runs, got %+v", rich)
		}
		var _ rendering.RenderObject = lb
	})
}
