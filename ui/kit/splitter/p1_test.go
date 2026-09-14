package splitter_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/splitter"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// P1 semantic hooks: classNames/styles are stored paint-only hooks
// (customize.tsx / style-class.tsx / _semantic.tsx staging). Layout stays
// stable and the bar override recolors paint (SPL-23).
func TestSplitter_PRD_SPL23_SemanticHooks(t *testing.T) {
	spec := loadSplitterSpec(t)
	show := loadShowcaseSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	before := s.Layout(rendering.Tight(spec.ContainerMain, spec.ContainerCross))

	s.SetClassName(splitter.SemanticRoot, "my-splitter")
	s.SetClassNames(map[splitter.SemanticKey]string{
		splitter.SemanticRoot: "my-splitter",
		splitter.SemanticBar:  "my-bar",
	})
	if s.ClassName(splitter.SemanticRoot) != "my-splitter" {
		t.Fatalf("class root=%q", s.ClassName(splitter.SemanticRoot))
	}
	if got := s.ClassNames(); got[splitter.SemanticBar] != "my-bar" {
		t.Fatalf("classes=%v", got)
	}
	barHex := theme.Hex(show.CustomBar)
	barRGBA := render.RGBA{R: barHex.R, G: barHex.G, B: barHex.B, A: 1}
	s.SetSemanticStyle(splitter.SemanticBar, splitter.Style{Bg: barRGBA, UseBg: true})
	st, ok := s.SemanticStyle(splitter.SemanticBar)
	if !ok || !st.UseBg || st.Bg != barRGBA {
		t.Fatalf("semantic bar=%+v ok=%v", st, ok)
	}
	if got := s.EffectiveBarColor(); got != barRGBA {
		t.Fatalf("override bar=%+v want %+v", got, barRGBA)
	}
	after := layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if before != after {
		t.Fatalf("semantic must not move layout: %v vs %v", before, after)
	}
	paintSplitter(s, 120, 60)
	s.ClearSemanticStyles()
	if _, ok := s.SemanticStyle(splitter.SemanticBar); ok {
		t.Fatal("clear must drop styles")
	}
	s.SetClassNames(nil)
	if s.ClassName(splitter.SemanticRoot) != "" {
		t.Fatal("clear must drop classes")
	}
}

// P1 custom style (customize.tsx): bar + handle overrides repaint only.
func TestSplitter_PRD_SPL23_CustomStyle(t *testing.T) {
	spec := loadSplitterSpec(t)
	show := loadShowcaseSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	barHex := theme.Hex(show.CustomBar)
	handleHex := theme.Hex(show.CustomHandle)
	barRGBA := render.RGBA{R: barHex.R, G: barHex.G, B: barHex.B, A: 1}
	handleRGBA := render.RGBA{R: handleHex.R, G: handleHex.G, B: handleHex.B, A: 1}
	s.SetSemanticStyle(splitter.SemanticBar, splitter.Style{Bg: barRGBA, UseBg: true})
	s.SetSemanticStyle(splitter.SemanticHandle, splitter.Style{Bg: handleRGBA, UseBg: true})
	if got := s.EffectiveBarColor(); got != barRGBA {
		t.Fatalf("bar=%+v", got)
	}
	if got := s.EffectiveHandleColor(); got != handleRGBA {
		t.Fatalf("handle=%+v", got)
	}
	sz := layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	paintSplitter(s, 120, 60)
}

// P1 double-click reset (reset.tsx): hook fires, then sizes restore.
func TestSplitter_PRD_SPL23_DoubleClickReset(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	fired := 0
	s.OnDraggerDoubleClick(func(i int) { fired++ })
	s.SetResetOnDoubleClick(true)
	if !s.ResetOnDoubleClick() {
		t.Fatal("reset flag")
	}
	if !s.DragBar(0, spec.DragDelta) {
		t.Fatal("drag")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	moved := append([]float64(nil), s.PanelSizes()...)
	s.DoubleClickBar(0)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	restored := s.PanelSizes()
	if fired != 1 {
		t.Fatalf("double-click=%d want 1", fired)
	}
	if math.Abs(restored[0]-spec.ContainerMain/2) > 0.5 {
		t.Fatalf("reset=%v moved=%v want 300", restored, moved)
	}
	// Without the flag only the hook fires (no auto reset).
	s2 := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s2, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	n := 0
	s2.OnDraggerDoubleClick(func(i int) { n++ })
	s2.DragBar(0, spec.DragDelta)
	layoutSplitter(t, s2, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	pre := append([]float64(nil), s2.PanelSizes()...)
	s2.DoubleClickBar(0)
	post := s2.PanelSizes()
	if n != 1 || math.Abs(post[0]-pre[0]) > 0.5 {
		t.Fatalf("no-reset hook=%d pre=%v post=%v", n, pre, post)
	}
}

// P0 destroyOnHidden (global + per-panel override): folded size 0 drops
// content, expand restores it.
func TestSplitter_PRD_SPL23_DestroyOnHidden(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetCollapsible(true)
	s := splitter.NewSplitter(lp, rp)
	s.SetDestroyOnHidden(true)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	s.CollapseAt(0, splitter.CollapseStart)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if !s.IsContentDestroyed(0) || s.ContentVisible(0) {
		t.Fatal("global destroy must drop folded content")
	}
	if s.IsContentDestroyed(1) {
		t.Fatal("open panel must keep content")
	}
	s.CollapseAt(0, splitter.CollapseStart)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if s.IsContentDestroyed(0) {
		t.Fatal("expand must restore content")
	}
	// Per-panel override wins over the global flag.
	lp2, _ := newColorPanel(0.9, 0.2, 0.2)
	rp2, _ := newColorPanel(0.2, 0.4, 0.9)
	lp2.SetCollapsible(true)
	lp2.SetDestroyOnHidden(false)
	s2 := splitter.NewSplitter(lp2, rp2)
	s2.SetDestroyOnHidden(true)
	s2.SetWidth(spec.ContainerMain)
	s2.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s2, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	s2.CollapseAt(0, splitter.CollapseStart)
	layoutSplitter(t, s2, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if s2.IsContentDestroyed(0) {
		t.Fatal("panel override false must keep content")
	}
}

// P0 orientation priority: orientation wins over the vertical sugar.
func TestSplitter_PRD_SPL23_OrientationPriority(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	s.SetOrientation(splitter.Horizontal)
	s.SetVertical(true)
	if s.EffectiveOrientation() != splitter.Horizontal || s.IsVertical() {
		t.Fatal("orientation horizontal must win over vertical=true")
	}
	s.SetOrientation(splitter.Vertical)
	s.SetVertical(false)
	if s.EffectiveOrientation() != splitter.Vertical || !s.IsVertical() {
		t.Fatal("orientation vertical must win over vertical=false")
	}
	sz := layoutSplitter(t, s, rendering.Tight(spec.ContainerCross, spec.ContainerMain))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

// P0 percent min/max: thresholds resolve against container length.
func TestSplitter_PRD_SPL23_PercentMinMax(t *testing.T) {
	spec := loadSplitterSpec(t)
	minPct := spec.MinPx / spec.ContainerMain * 100
	maxPct := spec.MaxPx / spec.ContainerMain * 100
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetMinPercent(minPct)
	lp.SetMaxPercent(maxPct)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	s.DragBar(0, 60-spec.ContainerMain/2)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if math.Abs(s.PanelSizes()[0]-spec.MinPx) > 0.5 {
		t.Fatalf("percent min=%v want %v", s.PanelSizes(), spec.MinPx)
	}
	s.DragBar(0, spec.ContainerMain)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if math.Abs(s.PanelSizes()[0]-spec.MaxPx) > 0.5 {
		t.Fatalf("percent max=%v want %v", s.PanelSizes(), spec.MaxPx)
	}
}

// P0 collapsible motion: P0 instant fold stays instant; the hook only
// records intent (pixel-level animation stays P1 staged).
func TestSplitter_PRD_SPL23_CollapsibleMotion(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetCollapsible(true)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	if s.CollapsibleMotion() {
		t.Fatal("motion default false (P0 instant)")
	}
	s.SetCollapsibleMotion(true)
	if !s.CollapsibleMotion() {
		t.Fatal("motion hook not stored")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	s.CollapseAt(0, splitter.CollapseStart)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if math.Abs(s.PanelSizes()[0]) > 0.5 {
		t.Fatalf("motion fold must stay instant %v", s.PanelSizes())
	}
}

// P0 dragger/collapsible custom icons (§6.7): stored nodes keep layout
// stable and paint without crashing (thick handle + square marks).
func TestSplitter_PRD_SPL23_CustomIcons(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetCollapsible(true)
	lp.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
	rp.SetCollapsible(true)
	rp.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	before := s.Layout(rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	dragger := rendering.NewRenderColorBox(4, 20, 0.2, 0.2, 0.2, 1)
	startIcon := rendering.NewRenderColorBox(6, 6, 0.2, 0.2, 0.2, 1)
	endIcon := rendering.NewRenderColorBox(6, 6, 0.3, 0.3, 0.3, 1)
	s.SetDraggerIcon(dragger)
	s.SetCollapsibleIcons(startIcon, endIcon)
	if s.DraggerIcon() == nil {
		t.Fatal("dragger icon missing")
	}
	if st, en := s.CollapsibleIcons(); st == nil || en == nil {
		t.Fatal("collapse icons missing")
	}
	after := layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if before != after {
		t.Fatalf("icons must not move layout: %v vs %v", before, after)
	}
	paintSplitter(s, 120, 60)
}

// P0 resize lifecycle: start/update/end payloads share one sizes snapshot.
func TestSplitter_PRD_SPL23_ResizeLifecycle(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	var starts, resizes, ends [][]float64
	s.OnResizeStart(func(v []float64) { starts = append(starts, append([]float64(nil), v...)) })
	s.OnResize(func(v []float64) { resizes = append(resizes, append([]float64(nil), v...)) })
	s.OnResizeEnd(func(v []float64) { ends = append(ends, append([]float64(nil), v...)) })
	if !s.BeginDrag(0) {
		t.Fatal("begin")
	}
	s.UpdateDrag(spec.DragDelta)
	s.EndDrag()
	if len(starts) != 1 || len(resizes) != 1 || len(ends) != 1 {
		t.Fatalf("lifecycle %d/%d/%d want 1/1/1", len(starts), len(resizes), len(ends))
	}
	for _, payload := range [][][]float64{starts, resizes, ends} {
		if math.Abs(payload[0][0]+payload[0][1]-spec.ContainerMain) > 0.5 {
			t.Fatalf("payload sum=%v want 600", payload[0])
		}
	}
}

// P1 ConfigProvider staging: per-instance provider already wires theme
// (SPL-18); the shared ConfigProvider kit is NotStarted so the package
// global layer stays staged here.
func TestSplitter_PRD_SPL23_ConfigProvider(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	base := theme.DefaultTokens()
	base.ColorPrimary = theme.RGBA(0, 200, 0, 1)
	p := theme.NewProvider(base)
	s.SetProvider(p)
	if got := s.EffectivePreviewColor(); got.G < 0.5 {
		t.Fatalf("provider preview=%+v want green", got)
	}
	s.SetProvider(nil)
	s.SetTheme(nil)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	paintSplitter(s, 120, 60)
}

// P1 Tabs nesting has no host to mount: Tabs kit is NotStarted, so the
// combination page is staged; group nesting (SPL-15) already proves a
// splitter survives inside another node.
func TestSplitter_PRD_SPL23_NestedTabsNA(t *testing.T) {
	t.Skip("P1 nested-in-tabs needs the Tabs host (W2 NotStarted); group nesting SPL-15 covers splitter-in-node, tabs combo staged")
}

// P1 pixel-level fold animation and complex virtual lists have no desktop
// mapping: P0 instant fold is kept, animation frames staged.
func TestSplitter_PRD_SPL23_VirtualMotionNA(t *testing.T) {
	t.Skip("P1 fold pixel animation / virtual-list panels are browser-scale motion with no gpui mapping; P0 instant fold covered, staged")
}

// P1 debug demos and ant.design pixel-hash parity are out of scope
// (§6.1 L4 / §6.8 P1): documented Skip.
func TestSplitter_PRD_SPL23_DebugPixelHashNA(t *testing.T) {
	t.Skip("P1 debug/size-mix demos and ant.design pixel-hash parity are not built;本库 golden is the L3 source")
}

// L4 human-eye side-by-side sign-off needs a reviewer looking at
// ant.design: documented Skip (not a code gap).
func TestSplitter_PRD_SPL22_HumanEyeNA(t *testing.T) {
	t.Skip("L4 SPL-22 needs human side-by-side sign-off against ant.design; no automated assertion, staged")
}
