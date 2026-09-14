package masonry_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/masonry"
	"github.com/energye/gpui/ui/rendering"
)

// MAS-11: style-class.tsx structure mounts; class strings are hooks only.
func TestMasonry_PRD_MAS11_SemanticStyleClass(t *testing.T) {
	fx := loadMasonry(t)
	masonry.ResetGlobalConfig()
	defer masonry.ResetGlobalConfig()
	items := []masonry.MasonryItem{
		{Key: "k1", Column: -1, Height: fx.Heights[0], Content: rendering.NewRenderColorBox(10, fx.Heights[0], 0.2, 0.4, 0.9, 1)},
		{Key: "k2", Column: -1, Height: fx.Heights[1], Content: rendering.NewRenderColorBox(10, fx.Heights[1], 0.2, 0.4, 0.9, 1)},
		{Key: "k3", Column: -1, Height: fx.Heights[2], Content: rendering.NewRenderColorBox(10, fx.Heights[2], 0.2, 0.4, 0.9, 1)},
	}
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(fx.Gutter, fx.Gutter)
	m.SetItems(items)
	before := m.Layout(rendering.Loose(fx.ContainerW, 2000))
	beforeBox := m.ItemBox("k1")
	if m.Node() == nil || len(m.Node().Children()) != len(items) {
		t.Fatalf("semantic structure must mount %d children", len(items))
	}
	m.SetClassName(masonry.SemanticRoot, "show-root")
	m.SetClassName(masonry.SemanticItem, "show-item")
	m.SetSemanticStyle(masonry.SemanticRoot, masonry.Style{Props: map[string]string{"backgroundColor": "rgba(250,250,250,0.5)"}})
	m.SetSemanticStyle(masonry.SemanticItem, masonry.Style{Props: map[string]string{"border": "1px solid #ccc"}})
	if m.ClassName(masonry.SemanticRoot) != "show-root" || m.ClassName(masonry.SemanticItem) != "show-item" {
		t.Fatalf("classNames root=%q item=%q", m.ClassName(masonry.SemanticRoot), m.ClassName(masonry.SemanticItem))
	}
	if _, ok := m.SemanticStyle(masonry.SemanticRoot); !ok {
		t.Fatal("root style must stick")
	}
	after := m.Layout(rendering.Loose(fx.ContainerW, 2000))
	if after != before {
		t.Fatalf("semantic hooks moved layout %v -> %v", before, after)
	}
	afterBox := m.ItemBox("k1")
	if afterBox != beforeBox {
		t.Fatalf("semantic hooks moved item box %v -> %v", beforeBox, afterBox)
	}
	m.SetClassNames(map[masonry.SemanticKey]string{masonry.SemanticRoot: "r2", masonry.SemanticItem: "i2"})
	if m.ClassName(masonry.SemanticRoot) != "r2" {
		t.Fatalf("replace root=%q", m.ClassName(masonry.SemanticRoot))
	}
	m.ClearSemanticStyles()
	if _, ok := m.SemanticStyle(masonry.SemanticRoot); ok {
		t.Fatal("ClearSemanticStyles must clear")
	}
	m.SetClassNames(nil)
	if m.ClassName(masonry.SemanticRoot) != "" {
		t.Fatal("nil classNames must clear")
	}
}

// MAS-12: _semantic.tsx nodes are exactly root/item; function form is caller-side.
func TestMasonry_PRD_MAS12_SemanticNodes(t *testing.T) {
	fx := loadMasonry(t)
	if masonry.SemanticRoot != "root" || masonry.SemanticItem != "item" {
		t.Fatalf("nodes=%q/%q want root/item", masonry.SemanticRoot, masonry.SemanticItem)
	}
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(0, 0)
	m.SetItems(autoItems("s12", []float64{fx.Heights[0], fx.Heights[1]}))
	sz0 := m.Layout(rendering.Loose(fx.ContainerW, 2000))
	// Function form (info:{props})=>Record maps to: read EffectiveColumnCount
	// then SetClassNames. Columns>2 picks the blue border per style-class.tsx.
	cols := m.EffectiveColumnCount()
	border := "#52c41a"
	if cols > 2 {
		border = "#1890ff"
	}
	m.SetClassNames(map[masonry.SemanticKey]string{masonry.SemanticRoot: "fn-root"})
	m.SetSemanticStyle(masonry.SemanticRoot, masonry.Style{Props: map[string]string{"border": border}})
	if m.ClassName(masonry.SemanticRoot) != "fn-root" {
		t.Fatal("function-form root must stick")
	}
	st, _ := m.SemanticStyle(masonry.SemanticRoot)
	if st.Props["border"] != border {
		t.Fatalf("function-form border=%q want %q", st.Props["border"], border)
	}
	if sz1 := m.Layout(rendering.Loose(fx.ContainerW, 2000)); sz1 != sz0 {
		t.Fatalf("semantic fn moved layout %v -> %v", sz0, sz1)
	}
}

// MAS-19: P1 global defaults + reduced-motion (explicit wins over global).
func TestMasonry_PRD_MAS19_P1GlobalReducedMotion(t *testing.T) {
	fx := loadMasonry(t)
	masonry.ResetGlobalConfig()
	defer masonry.ResetGlobalConfig()
	masonry.SetGlobalConfig(masonry.GlobalConfig{
		Columns:          fx.Columns,
		HasColumns:       true,
		GutterH:          fx.Gutter,
		GutterV:          fx.Gutter,
		HasGutter:        true,
		Fresh:            true,
		HasFresh:         true,
		ReducedMotion:    true,
		HasReducedMotion: true,
	})
	g := masonry.NewMasonry()
	if g.EffectiveColumnCount() != fx.Columns {
		t.Fatalf("global columns=%d want %d", g.EffectiveColumnCount(), fx.Columns)
	}
	gh, gv := g.EffectiveGutter()
	near(t, gh, fx.Gutter, fx.Tolerance, "global gutterH")
	near(t, gv, fx.Gutter, fx.Tolerance, "global gutterV")
	if !g.Fresh() {
		t.Fatal("global fresh must apply")
	}
	if !g.ReducedMotion() || g.MotionEnabled() {
		t.Fatal("global reduced-motion must disable motion")
	}
	g.SetColumns(fx.SingleColumn)
	if g.EffectiveColumnCount() != fx.SingleColumn {
		t.Fatalf("explicit columns=%d must win", g.EffectiveColumnCount())
	}
	g.SetGutter(0, 0)
	// Explicit zero gutter wins over the global non-zero one.
	if h, v := g.EffectiveGutter(); h != 0 || v != 0 {
		t.Fatalf("explicit gutter=%v/%v must win", h, v)
	}
	g.SetFresh(false)
	if g.Fresh() {
		t.Fatal("explicit fresh(false) must win")
	}
	g.SetReducedMotion(false)
	if g.ReducedMotion() || !g.MotionEnabled() {
		t.Fatal("explicit reduced(false) must win")
	}
	masonry.ResetGlobalConfig()
	plain := masonry.NewMasonry()
	if plain.EffectiveColumnCount() != masonry.DefaultColumns {
		t.Fatalf("cleared columns=%d want %d", plain.EffectiveColumnCount(), masonry.DefaultColumns)
	}
	if plain.ReducedMotion() || !plain.MotionEnabled() {
		t.Fatal("cleared motion must default enabled")
	}
}

// MAS-18 L4 needs a reviewer side-by-side: documented Skip.
func TestMasonry_PRD_MAS18_HumanEyeNA(t *testing.T) {
	t.Skip("L4 MAS-18 needs human side-by-side sign-off against ant.design; no automated assertion")
}

// P1 pixel-level motion is staged; P0 is instant and respects reduced-motion.
func TestMasonry_P1_AnimationPixelStaged(t *testing.T) {
	t.Skip("P1 animation pixel motion (fade/left-top transition) is staged; P0 instant relayout already covers fresh/responsive and SetReducedMotion gates the transition")
}

// Complex virtual list is staged; small galleries lay out fully.
func TestMasonry_P1_VirtualListStaged(t *testing.T) {
	t.Skip("P1 complex virtual list is staged per §6.8; desktop small item counts use full shortest-column layout")
}

// Masonry has no own lazy loader; image laziness belongs to the image/business layer.
func TestMasonry_P1_LazyLoadingNA(t *testing.T) {
	t.Skip("P1 lazy loading N/A to masonry itself: no lazy prop in §3 API; image async decode/culling belongs to the image component or business ItemRender, masonry only re-lays out on measured heights")
}

// Browser-only APIs have no desktop equivalent.
func TestMasonry_P1_BrowserOnlyNA(t *testing.T) {
	t.Skip("P1 browser-only N/A: DOM refs/CSS class string depth and ResizeObserver wiring have no desktop equivalent beyond SetClassName hooks and SetFresh remeasure, which are already covered")
}

// Official debug demo + site pixel hash are explicitly out of scope.
func TestMasonry_P1_DebugHashNA(t *testing.T) {
	t.Skip("P1 debug/site-hash N/A: fresh.tsx is an official debug demo excluded from P0, and §6.1 L4 excludes ant.design逐像素哈希")
}
