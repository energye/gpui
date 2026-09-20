package prim_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/rendering"
)

type layoutFile struct {
	Cases []layoutCase `json:"cases"`
}

type layoutCase struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Horizontal bool        `json:"horizontal"`
	MainMax    float64     `json:"mainMax"`
	CrossMax   float64     `json:"crossMax"`
	Spacing    float64     `json:"spacing"`
	Dir        string      `json:"dir"`
	Cross      string      `json:"cross"`
	Items      []flexJSON  `json:"items"`
	WantOuter  sizeJSON    `json:"wantOuter"`
	WantBoxes  []boxJSON   `json:"wantBoxes"`
	ParentMaxW float64     `json:"parentMaxW"`
	ParentMaxH float64     `json:"parentMaxH"`
	Layers     []layerJSON `json:"layers"`
	RunSpacing float64     `json:"runSpacing"`
	Children   []sizeJSON  `json:"children"`
	Ratio      float64     `json:"ratio"`
	ParentMinW float64     `json:"parentMinW"`
	ParentMinH float64     `json:"parentMinH"`
	MinW       float64     `json:"minW"`
	MaxW       float64     `json:"maxW"`
	MinH       float64     `json:"minH"`
	MaxH       float64     `json:"maxH"`
	ChildW     float64     `json:"childW"`
	ChildH     float64     `json:"childH"`
	PadL       float64     `json:"l"`
	PadT       float64     `json:"t"`
	PadR       float64     `json:"r"`
	PadB       float64     `json:"b"`
}

type flexJSON struct {
	FixedW float64 `json:"fixedW"`
	FixedH float64 `json:"fixedH"`
	Flex   int     `json:"flex"`
	Tight  bool    `json:"tight"`
}

type sizeJSON struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type boxJSON struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type layerJSON struct {
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	Positioned bool    `json:"positioned"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	PW         float64 `json:"pw"`
	PH         float64 `json:"ph"`
}

func loadLayout(t *testing.T) layoutFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "layout_cases.json"))
	if err != nil {
		t.Fatalf("read layout_cases.json: %v", err)
	}
	var f layoutFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode layout_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("layout_cases.json holds no cases")
	}
	return f
}

func crossOf(s string) prim.CrossAlign {
	if s == "center" {
		return prim.CrossCenter
	}
	if s == "stretch" {
		return prim.CrossStretch
	}
	return prim.CrossStart
}

// TestPrim_LayoutMatrix checks Row/Column/Stack/Wrap/Aspect/Constrained/Pad
// sizes and positions under tight, loose and unbounded constraints.
func TestPrim_LayoutMatrix(t *testing.T) {
	f := loadLayout(t)
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			switch c.Kind {
			case "flex":
				items := make([]prim.FlexSpec, len(c.Items))
				for i, it := range c.Items {
					if it.Flex > 0 {
						if it.Tight {
							items[i] = prim.ExpandedSpec(it.Flex)
						} else {
							items[i] = prim.FlexibleSpec(it.Flex)
						}
						continue
					}
					items[i] = prim.FixedSpec(it.FixedW, it.FixedH)
				}
				dir := prim.DirLTR
				if c.Dir == "rtl" {
					dir = prim.DirRTL
				}
				got := prim.FlexLayout(c.Horizontal, c.MainMax, c.CrossMax, c.Spacing, dir, crossOf(c.Cross), items)
				checkSize(t, got.Outer, c.WantOuter)
				checkBoxes(t, got.Boxes, c.WantBoxes)
			case "stack":
				parent := rendering.Constraints{MaxWidth: c.ParentMaxW, MaxHeight: c.ParentMaxH}
				layers := make([]prim.StackSpec, len(c.Layers))
				for i, l := range c.Layers {
					if l.Positioned {
						layers[i] = prim.PositionedSpec(l.W, l.H, l.X, l.Y, l.PW, l.PH)
					} else {
						layers[i] = prim.StackedSpec(l.W, l.H)
					}
				}
				got := prim.StackLayout(parent, layers)
				checkSize(t, got.Outer, c.WantOuter)
				checkBoxes(t, stackBoxes(got.Boxes), c.WantBoxes)
			case "wrap":
				parent := rendering.Constraints{MaxWidth: c.ParentMaxW, MaxHeight: c.ParentMaxH}
				kids := make([]rendering.Size, len(c.Children))
				for i, s := range c.Children {
					kids[i] = rendering.Size{Width: s.W, Height: s.H}
				}
				got := prim.WrapLayout(parent, c.Spacing, c.RunSpacing, crossOf(c.Cross), kids)
				checkSize(t, got.Outer, c.WantOuter)
				checkBoxes(t, stackBoxes(got.Boxes), c.WantBoxes)
			case "aspect":
				parent := rendering.Constraints{MaxWidth: c.ParentMaxW, MaxHeight: c.ParentMaxH}
				got := prim.AspectFit(parent, c.Ratio)
				checkSize(t, got, c.WantOuter)
			case "constrained":
				parent := rendering.Constraints{MinWidth: c.ParentMinW, MaxWidth: c.ParentMaxW, MinHeight: c.ParentMinH, MaxHeight: c.ParentMaxH}
				lim := prim.Limits{MinW: c.MinW, MaxW: c.MaxW, MinH: c.MinH, MaxH: c.MaxH}
				got := prim.ConstrainSize(lim, parent, rendering.Size{Width: c.ChildW, Height: c.ChildH})
				checkSize(t, got, c.WantOuter)
			case "pad":
				got := prim.PadOuter(rendering.Size{Width: c.ChildW, Height: c.ChildH}, c.PadL, c.PadT, c.PadR, c.PadB)
				checkSize(t, got, c.WantOuter)
			default:
				t.Fatalf("unknown kind %q", c.Kind)
			}
		})
	}
}

func checkSize(t *testing.T, got rendering.Size, want sizeJSON) {
	t.Helper()
	if math.Abs(got.Width-want.W) > 1e-3 || math.Abs(got.Height-want.H) > 1e-3 {
		t.Fatalf("outer = %.4fx%.4f want %.4fx%.4f", got.Width, got.Height, want.W, want.H)
	}
}

func checkBoxes(t *testing.T, got []prim.FlexPlacement, want []boxJSON) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("boxes len = %d want %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i].W-want[i].W) > 1e-3 || math.Abs(got[i].H-want[i].H) > 1e-3 ||
			math.Abs(got[i].X-want[i].X) > 1e-3 || math.Abs(got[i].Y-want[i].Y) > 1e-3 {
			t.Fatalf("box %d = %.4f,%.4f @ %.4f,%.4f want %.4f,%.4f @ %.4f,%.4f",
				i, got[i].W, got[i].H, got[i].X, got[i].Y,
				want[i].W, want[i].H, want[i].X, want[i].Y)
		}
	}
}

func stackBoxes(in []prim.FlexPlacement) []prim.FlexPlacement { return in }
