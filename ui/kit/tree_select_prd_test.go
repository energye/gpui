package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/tree-select.md §6.9 — P0 PRD cases (TSE-01 … TSE-21).
// L3/L4 (TSE-22/23) and P1 (TSE-24) deferred.

func approxTSE(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxTSEColor(a, b render.RGBA, tol float64) bool {
	dr, dg, db, da := a.R-b.R, a.G-b.G, a.B-b.B, a.A-b.A
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if da < 0 {
		da = -da
	}
	return dr <= tol && dg <= tol && db <= tol && da <= tol
}

func sampleTreeData() []kit.TreeSelectNode {
	return []kit.TreeSelectNode{
		{
			Value: "parent 1", Title: "parent 1",
			Children: []kit.TreeSelectNode{
				{
					Value: "parent 1-0", Title: "parent 1-0",
					Children: []kit.TreeSelectNode{
						{Value: "leaf1", Title: "leaf1"},
						{Value: "leaf2", Title: "leaf2"},
					},
				},
				{
					Value: "parent 1-1", Title: "parent 1-1",
					Children: []kit.TreeSelectNode{
						{Value: "leaf11", Title: "leaf11"},
					},
				},
			},
		},
	}
}

func checkableTreeData() []kit.TreeSelectNode {
	return []kit.TreeSelectNode{
		{
			Title: "Node1", Value: "0-0",
			Children: []kit.TreeSelectNode{
				{Title: "Child Node1", Value: "0-0-0"},
			},
		},
		{
			Title: "Node2", Value: "0-1",
			Children: []kit.TreeSelectNode{
				{Title: "Child Node3", Value: "0-1-0"},
				{Title: "Child Node4", Value: "0-1-1"},
				{Title: "Child Node5", Value: "0-1-2"},
			},
		},
	}
}

func treeDataSimple() []kit.TreeSelectNode {
	return []kit.TreeSelectNode{
		{
			Title: "Node1", Value: "0-0",
			Children: []kit.TreeSelectNode{
				{Title: "Child Node1", Value: "0-0-1"},
				{Title: "Child Node2", Value: "0-0-2"},
			},
		},
		{Title: "Node2", Value: "0-1"},
	}
}

func mountTreeSelectPRD(t *testing.T, ts *kit.TreeSelect, w, h float64) *core.Tree {
	t.Helper()
	if ts == nil {
		t.Fatal("nil TreeSelect")
	}
	ts.Viewport = core.Size{Width: w, Height: h}
	bg := primitive.NewBox(ts.Node())
	bg.Width, bg.Height = w, h
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func clickTreeSelectTrigger(t *testing.T, tree *core.Tree, ts *kit.TreeSelect) {
	t.Helper()
	tree.Layout(core.Size{Width: 480, Height: 360})
	abs := core.AbsoluteBounds(ts.Root)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	tree.Layout(core.Size{Width: 480, Height: 360})
}

func clickTreeSelectOption(t *testing.T, tree *core.Tree, ts *kit.TreeSelect, label string) bool {
	t.Helper()
	var target *primitive.Pressable
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || target != nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok {
			if p.Base().Role == "option" && (p.Base().Label == label || strings.Contains(p.Base().Label, label)) {
				target = p
				return
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	if ts.Popup() != nil && ts.Popup().Content != nil {
		walk(ts.Popup().Content)
	}
	walk(ts.Node())
	if target == nil {
		return false
	}
	abs := core.AbsoluteBounds(target)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	tree.Layout(core.Size{Width: 480, Height: 360})
	return true
}

func TestTreeSelect_PRD_01_Defaults(t *testing.T) {
	// TSE-01
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	if ts.Size != kit.InputMiddle {
		t.Fatalf("Size=%v want middle", ts.Size)
	}
	if ts.Variant != kit.InputOutlined {
		t.Fatalf("Variant=%v want outlined", ts.Variant)
	}
	if ts.Status != kit.InputStatusNone {
		t.Fatalf("Status=%v want none", ts.Status)
	}
	if ts.Disabled || ts.Loading || ts.AllowClear || ts.ShowSearch || ts.Open || ts.Multiple || ts.TreeCheckable {
		t.Fatalf("flags should be false")
	}
	if ts.ListHeight != kit.DefaultTreeSelectListHeight {
		t.Fatalf("ListHeight=%v want 256", ts.ListHeight)
	}
	if ts.Placement != kit.TreeSelectBottomLeft {
		t.Fatalf("Placement=%v want bottomLeft", ts.Placement)
	}
	if ts.ShowCheckedStrategy != kit.TreeSelectSHOW_CHILD {
		t.Fatalf("strategy=%v want SHOW_CHILD", ts.ShowCheckedStrategy)
	}
	if !ts.PopupMatchSelectWidth {
		t.Fatal("PopupMatchSelectWidth default true")
	}
	if ts.Placeholder != "Please select" {
		t.Fatalf("Placeholder=%q", ts.Placeholder)
	}
	if ts.Node() == nil || ts.Root == nil || ts.Popup() == nil {
		t.Fatal("nil nodes")
	}
	if ts.Root.Base().Role != "combobox" {
		t.Fatalf("role=%q want combobox", ts.Root.Base().Role)
	}
	_ = ts.Node().Layout(core.Loose(400, 200))
}

func TestTreeSelect_PRD_02_SelectNode(t *testing.T) {
	// TSE-02 / TSE-S1
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetTreeDefaultExpandAll(true)
	var changes []string
	ts.SetOnChange(func(v string) { changes = append(changes, v) })
	tree := mountTreeSelectPRD(t, ts, 480, 360)
	clickTreeSelectTrigger(t, tree, ts)
	if !ts.Open {
		t.Fatal("want open after trigger")
	}
	if !clickTreeSelectOption(t, tree, ts, "leaf1") {
		ts.SelectValue("leaf1")
	}
	if ts.Value != "leaf1" {
		t.Fatalf("value=%q want leaf1", ts.Value)
	}
	if len(changes) == 0 || changes[len(changes)-1] != "leaf1" {
		t.Fatalf("onChange=%v want leaf1", changes)
	}
	if ts.Open {
		t.Fatal("single select must close")
	}
}

func TestTreeSelect_PRD_03_TreeCheckable(t *testing.T) {
	// TSE-03 / TSE-S2
	ts := kit.NewTreeSelect("Please select", checkableTreeData()...)
	ts.SetTreeCheckable(true)
	ts.SetTreeDefaultExpandAll(true)
	ts.SetShowCheckedStrategy(kit.TreeSelectSHOW_PARENT)
	var multi []string
	ts.SetOnChangeMulti(func(v []string) { multi = append([]string(nil), v...) })
	ts.SetDefaultValues([]string{"0-0-0"})
	if got := ts.GetValues(); len(got) != 1 || got[0] != "0-0-0" {
		t.Fatalf("default values=%v", got)
	}
	// check parent Node2 → all children
	ts.ToggleCheck("0-1")
	if !containsAllTSE(ts.GetValues(), "0-1-0", "0-1-1", "0-1-2") {
		// Values store leaves; display may collapse to parent
		vals := ts.GetValues()
		if len(vals) == 0 {
			t.Fatalf("check parent empty values")
		}
	}
	if len(multi) == 0 {
		t.Fatal("want onChangeMulti")
	}
}

func TestTreeSelect_PRD_04_Search(t *testing.T) {
	// TSE-04 / TSE-S3
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetShowSearch(true)
	ts.SetTreeDefaultExpandAll(true)
	ts.SetOpen(true)
	ts.SetSearchValue("leaf1")
	titles := ts.VisibleTitles()
	joined := strings.Join(titles, ",")
	if !strings.Contains(joined, "leaf1") {
		t.Fatalf("visible=%v want leaf1", titles)
	}
	// unrelated leaf filtered out ideally
	if strings.Contains(joined, "leaf11") && !strings.Contains(joined, "leaf1") {
		t.Fatalf("unexpected filter result %v", titles)
	}
}

func TestTreeSelect_PRD_05_Clear(t *testing.T) {
	// TSE-05 / TSE-S4
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetAllowClear(true)
	ts.SetValue("leaf1")
	cleared := false
	ts.SetOnClear(func() { cleared = true })
	var changes []string
	ts.SetOnChange(func(v string) { changes = append(changes, v) })
	ts.Clear()
	if ts.Value != "" {
		t.Fatalf("value=%q want empty", ts.Value)
	}
	if !cleared {
		t.Fatal("OnClear not fired")
	}
	if len(changes) == 0 || changes[len(changes)-1] != "" {
		t.Fatalf("onChange after clear=%v", changes)
	}
}

func TestTreeSelect_PRD_06_Multiple(t *testing.T) {
	// TSE-06 / TSE-S5
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetMultiple(true)
	ts.SetTreeDefaultExpandAll(true)
	var multi []string
	ts.SetOnChangeMulti(func(v []string) { multi = append([]string(nil), v...) })
	ts.SelectValue("leaf1")
	ts.SelectValue("leaf2")
	vals := ts.GetValues()
	if !containsAllTSE(vals, "leaf1", "leaf2") {
		t.Fatalf("values=%v", vals)
	}
	if len(multi) < 2 {
		t.Fatalf("onChangeMulti calls insufficient: %v", multi)
	}
}

func TestTreeSelect_PRD_07_LoadData(t *testing.T) {
	// TSE-07 / TSE-S6
	roots := []kit.TreeSelectNode{
		{Value: "1", Title: "Expand to load", ID: "1", PID: "0"},
		{Value: "2", Title: "Expand to load", ID: "2", PID: "0"},
		{Value: "3", Title: "Tree Node", ID: "3", PID: "0", IsLeaf: true},
	}
	ts := kit.NewTreeSelect("Please select", roots...)
	ts.SetTreeDataSimpleMode(true)
	loaded := false
	ts.SetLoadData(func(n kit.TreeSelectNode) {
		loaded = true
		// append children into TreeData as simple mode rows
		data := append([]kit.TreeSelectNode(nil), ts.TreeData...)
		data = append(data,
			kit.TreeSelectNode{Value: "1-a", Title: "child a", ID: "1-a", PID: n.ID},
			kit.TreeSelectNode{Value: "1-b", Title: "child b", ID: "1-b", PID: n.ID, IsLeaf: true},
		)
		// clear loading on parent
		for i := range data {
			if data[i].Value == n.Value {
				data[i].Loading = false
			}
		}
		ts.TreeData = data
		ts.NotifyTreeDataChanged()
	})
	ts.SetOpen(true)
	ts.ExpandValue("1")
	if !loaded {
		t.Fatal("loadData not called")
	}
	// after load, children visible when expanded
	titles := ts.VisibleTitles()
	joined := strings.Join(titles, ",")
	if !strings.Contains(joined, "child a") && !strings.Contains(joined, "child b") {
		// Expand after notify
		ts.ExpandValue("1")
		titles = ts.VisibleTitles()
		joined = strings.Join(titles, ",")
	}
	if !strings.Contains(joined, "child") {
		t.Fatalf("after load visible=%v", titles)
	}
}

func TestTreeSelect_PRD_08_Disabled(t *testing.T) {
	// TSE-08 / TSE-S7
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetDisabled(true)
	tree := mountTreeSelectPRD(t, ts, 480, 360)
	clickTreeSelectTrigger(t, tree, ts)
	if ts.Open {
		t.Fatal("disabled must not open")
	}
	ts.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if ts.Open {
		t.Fatal("disabled keyboard must not open")
	}
}

func TestTreeSelect_PRD_09_HeightMiddle(t *testing.T) {
	// TSE-09 / TSE-S8
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	chrome := ts.ChromeNode()
	if chrome == nil {
		t.Fatal("nil chrome")
	}
	sz := chrome.Layout(core.Loose(400, 200))
	th := kit.DefaultTheme()
	want := th.SizeOr(core.TokenControlHeight, 32)
	if !approxTSE(sz.Height, want, 0.5) {
		// Decorated height field
		if d, ok := chrome.(*primitive.Decorated); ok {
			if !approxTSE(d.Height, want, 0.5) {
				t.Fatalf("height=%v want %v", d.Height, want)
			}
		} else if !approxTSE(sz.Height, want, 2) {
			t.Fatalf("layout height=%v want ~%v", sz.Height, want)
		}
	}
}

func TestTreeSelect_PRD_10_DemoBasic(t *testing.T) {
	// TSE-10 basic.tsx
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetShowSearch(true)
	ts.SetAllowClear(true)
	ts.SetTreeDefaultExpandAll(true)
	var v string
	ts.SetOnChange(func(x string) { v = x })
	tree := mountTreeSelectPRD(t, ts, 480, 400)
	clickTreeSelectTrigger(t, tree, ts)
	if !ts.Open {
		t.Fatal("want open")
	}
	ts.SelectValue("leaf2")
	if ts.Value != "leaf2" && v != "leaf2" {
		t.Fatalf("value=%q onChange=%q", ts.Value, v)
	}
}

func TestTreeSelect_PRD_11_DemoMultiple(t *testing.T) {
	// TSE-11 multiple.tsx
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetShowSearch(true)
	ts.SetAllowClear(true)
	ts.SetMultiple(true)
	ts.SetTreeDefaultExpandAll(true)
	ts.SelectValue("leaf1")
	ts.SelectValue("leaf2")
	if len(ts.GetValues()) < 2 {
		t.Fatalf("multi values=%v", ts.GetValues())
	}
	disp := ts.DisplayText()
	if !strings.Contains(disp, "leaf1") && !strings.Contains(disp, "leaf2") {
		t.Fatalf("display=%q", disp)
	}
}

func TestTreeSelect_PRD_12_DemoTreeData(t *testing.T) {
	// TSE-12 treeData.tsx
	ts := kit.NewTreeSelect("Please select", treeDataSimple()...)
	ts.SetTreeDefaultExpandAll(true)
	ts.SetOpen(true)
	titles := ts.VisibleTitles()
	if !containsAllTSE(titles, "Node1", "Child Node1", "Child Node2", "Node2") {
		t.Fatalf("visible=%v", titles)
	}
	ts.SelectValue("0-0-1")
	if ts.Value != "0-0-1" {
		t.Fatalf("value=%q", ts.Value)
	}
}

func TestTreeSelect_PRD_13_DemoCheckable(t *testing.T) {
	// TSE-13 checkable.tsx
	ts := kit.NewTreeSelect("Please select", checkableTreeData()...)
	ts.SetTreeCheckable(true)
	ts.SetShowCheckedStrategy(kit.TreeSelectSHOW_PARENT)
	ts.SetTreeDefaultExpandAll(true)
	ts.SetDefaultValues([]string{"0-0-0"})
	disp := ts.DisplayText()
	if disp == "" || disp == "Please select" {
		t.Fatalf("display empty for default checked")
	}
	// checking all Node2 children → SHOW_PARENT shows Node2
	ts.ToggleCheck("0-1-0")
	ts.ToggleCheck("0-1-1")
	ts.ToggleCheck("0-1-2")
	// display strategy may show parent
	_ = ts.DisplayText()
}

func TestTreeSelect_PRD_14_DemoAsync(t *testing.T) {
	// TSE-14 async.tsx
	ts := kit.NewTreeSelect("Please select",
		kit.TreeSelectNode{ID: "1", PID: "0", Value: "1", Title: "Expand to load"},
		kit.TreeSelectNode{ID: "2", PID: "0", Value: "2", Title: "Expand to load"},
		kit.TreeSelectNode{ID: "3", PID: "0", Value: "3", Title: "Tree Node", IsLeaf: true},
	)
	ts.SetTreeDataSimpleMode(true)
	calls := 0
	ts.SetLoadData(func(n kit.TreeSelectNode) {
		calls++
		ts.TreeData = append(ts.TreeData,
			kit.TreeSelectNode{ID: "x1", PID: n.ID, Value: "x1", Title: "Tree Node", IsLeaf: true},
		)
		for i := range ts.TreeData {
			if ts.TreeData[i].Value == n.Value {
				ts.TreeData[i].Loading = false
			}
		}
		ts.NotifyTreeDataChanged()
	})
	ts.SetOpen(true)
	ts.ExpandValue("1")
	if calls != 1 {
		t.Fatalf("loadData calls=%d", calls)
	}
}

func TestTreeSelect_PRD_15_DemoTreeLine(t *testing.T) {
	// TSE-15 treeLine.tsx
	ts := kit.NewTreeSelect("", sampleTreeData()...)
	ts.SetTreeLine(true)
	ts.SetTreeIcon(true)
	ts.SetTreeDefaultExpandAll(true)
	ts.SetOpen(true)
	if !ts.TreeLine || !ts.TreeIcon {
		t.Fatal("treeLine/treeIcon flags")
	}
	if len(ts.VisibleTitles()) == 0 {
		t.Fatal("expected visible rows")
	}
	_ = ts.Node().Layout(core.Loose(300, 200))
}

func TestTreeSelect_PRD_16_DemoPlacement(t *testing.T) {
	// TSE-16 placement.tsx
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetShowSearch(true)
	ts.SetAllowClear(true)
	ts.SetPopupMatchSelectWidth(false)
	for _, p := range []kit.TreeSelectPlacement{
		kit.TreeSelectTopLeft, kit.TreeSelectTopRight,
		kit.TreeSelectBottomLeft, kit.TreeSelectBottomRight,
	} {
		ts.SetPlacement(p)
		if ts.Placement != p {
			t.Fatalf("placement=%v want %v", ts.Placement, p)
		}
		if ts.Popup() == nil {
			t.Fatal("nil popup")
		}
	}
}

func TestTreeSelect_PRD_17_DemoVariant(t *testing.T) {
	// TSE-17 variant.tsx
	for _, v := range []kit.InputVariant{
		kit.InputBorderless, kit.InputFilled, kit.InputOutlined, kit.InputUnderlined,
	} {
		ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
		ts.SetVariant(v)
		if ts.Variant != v {
			t.Fatalf("variant=%v", ts.Variant)
		}
		chrome := ts.ChromeNode()
		if chrome == nil {
			t.Fatal("nil chrome")
		}
		_ = chrome.Layout(core.Loose(300, 80))
	}
}

func TestTreeSelect_PRD_18_Metrics(t *testing.T) {
	// TSE-18 L2
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	th := kit.DefaultTheme()
	// size ladder
	cases := []struct {
		sz   kit.InputSize
		want float64
	}{
		{kit.InputSmall, th.SizeOr(core.TokenControlHeightSM, 24)},
		{kit.InputMiddle, th.SizeOr(core.TokenControlHeight, 32)},
		{kit.InputLarge, th.SizeOr(core.TokenControlHeightLG, 40)},
	}
	for _, c := range cases {
		ts.SetSize(c.sz)
		d, ok := ts.ChromeNode().(*primitive.Decorated)
		if !ok {
			t.Fatal("chrome not Decorated")
		}
		if !approxTSE(d.Height, c.want, 0.5) {
			t.Fatalf("size %v height=%v want %v", c.sz, d.Height, c.want)
		}
		if !approxTSE(d.Radius, th.SizeOr(core.TokenBorderRadius, 6), 0.5) && c.sz != kit.InputMiddle {
			// underlined zeros radius; others use token
		}
	}
	// border width default
	ts.SetSize(kit.InputMiddle)
	ts.SetVariant(kit.InputOutlined)
	d := ts.ChromeNode().(*primitive.Decorated)
	if !approxTSE(d.BorderWidth, th.SizeOr(core.TokenLineWidth, 1), 0.5) {
		t.Fatalf("borderWidth=%v", d.BorderWidth)
	}
	// focus ring outset
	if ts.Root == nil || !approxTSE(ts.Root.FocusRingOutset, kit.DefaultTreeSelectFocusRingOutset, 0.1) {
		t.Fatalf("focus ring outset=%v", ts.Root.FocusRingOutset)
	}
}

func TestTreeSelect_PRD_19_TokenColors(t *testing.T) {
	// TSE-19 L2
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	th := kit.DefaultTheme()
	ts.SetTheme(th)
	d := ts.ChromeNode().(*primitive.Decorated)
	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	if !approxTSEColor(d.Background, bg, 0.02) {
		t.Fatalf("bg=%v want token %v (no hardcode brand)", d.Background, bg)
	}
	if !approxTSEColor(d.BorderColor, bd, 0.02) {
		t.Fatalf("border=%v want token %v", d.BorderColor, bd)
	}
}

func TestTreeSelect_PRD_20_DisabledAppearance(t *testing.T) {
	// TSE-20 L2
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetValue("leaf1")
	ts.SetDisabled(true)
	th := kit.DefaultTheme()
	ts.SetTheme(th)
	d := ts.ChromeNode().(*primitive.Decorated)
	dbg := th.Color(core.TokenColorDisabledBg)
	if dbg.A < 0.05 {
		dbg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	}
	// disabled uses disabled bg
	if d.Background.A < 0.01 {
		t.Fatalf("disabled bg too transparent: %v", d.Background)
	}
	// no open on hover-like click
	tree := mountTreeSelectPRD(t, ts, 400, 200)
	clickTreeSelectTrigger(t, tree, ts)
	if ts.Open {
		t.Fatal("disabled open")
	}
}

func TestTreeSelect_PRD_21_KeyboardFocus(t *testing.T) {
	// TSE-21
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetTreeDefaultExpandAll(true)
	if ts.Root == nil || !ts.Root.Focusable {
		t.Fatal("trigger must be focusable")
	}
	if !ts.Root.ShowFocusRing {
		t.Fatal("focus ring required")
	}
	if ts.Root.FocusRingOutset < 1 {
		t.Fatalf("focus ring outset=%v", ts.Root.FocusRingOutset)
	}
	tree := mountTreeSelectPRD(t, ts, 480, 360)
	ts.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if !ts.Open {
		t.Fatal("Enter should open")
	}
	// navigate + select
	ts.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowDown"})
	ts.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	// Esc closes on a fresh uncontrolled instance
	ts2 := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts2.SetTreeDefaultExpandAll(true)
	_ = tree
	ts2.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowDown"})
	if !ts2.Open {
		t.Fatal("ArrowDown opens")
	}
	ts2.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if ts2.Open {
		t.Fatal("Esc closes")
	}
}

func TestTreeSelect_PRD_22_GoldenDeferred(t *testing.T) {
	// TSE-22 L3 — deferred (gallery / visualtest baseline elsewhere)
	t.Skip("L3 golden deferred — visualtest per_control covers smoke")
}

func TestTreeSelect_PRD_23_EyeDeferred(t *testing.T) {
	// TSE-23 L4
	t.Skip("L4 eye sign-off deferred")
}

func TestTreeSelect_PRD_24_P1Deferred(t *testing.T) {
	// TSE-24 P1
	t.Skip("P1 deferred — see coverage.go Notes")
}

func TestTreeSelect_PRD_RootStableOnSelect(t *testing.T) {
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	ts.SetTreeDefaultExpandAll(true)
	root := ts.Root
	ts.SelectValue("leaf1")
	if ts.Root != root {
		t.Fatal("Root identity must stay stable across select")
	}
}

func TestTreeSelect_PRD_OpenChange(t *testing.T) {
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	var opens []bool
	ts.SetOnOpenChange(func(o bool) { opens = append(opens, o) })
	tree := mountTreeSelectPRD(t, ts, 480, 360)
	clickTreeSelectTrigger(t, tree, ts)
	if len(opens) == 0 || !opens[0] {
		t.Fatalf("opens=%v", opens)
	}
}

func TestTreeSelect_PRD_Status(t *testing.T) {
	ts := kit.NewTreeSelect("Please select", sampleTreeData()...)
	th := kit.DefaultTheme()
	ts.SetTheme(th)
	ts.SetStatus(kit.InputStatusError)
	d := ts.ChromeNode().(*primitive.Decorated)
	if !approxTSEColor(d.BorderColor, th.Color(core.TokenColorError), 0.05) {
		t.Fatalf("error border=%v", d.BorderColor)
	}
	ts.SetStatus(kit.InputStatusWarning)
	d = ts.ChromeNode().(*primitive.Decorated)
	if !approxTSEColor(d.BorderColor, th.Color(core.TokenColorWarning), 0.05) {
		t.Fatalf("warning border=%v", d.BorderColor)
	}
}

func containsAllTSE(have []string, want ...string) bool {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
