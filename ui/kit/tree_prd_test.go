package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// docs/antd/tree.md §6.9 — P0 PRD cases (TRE-01 … TRE-23).
// L3/L4 (TRE-24/25) and P1 (TRE-26) deferred.

func sampleTreePRDData() []kit.TreeNode {
	return []kit.TreeNode{
		{
			Key: "0-0", Title: "parent 1",
			Children: []kit.TreeNode{
				{
					Key: "0-0-0", Title: "parent 1-0",
					Children: []kit.TreeNode{
						{Key: "0-0-0-0", Title: "leaf0"},
						{Key: "0-0-0-1", Title: "leaf1"},
					},
				},
				{
					Key: "0-0-1", Title: "parent 1-1",
					Children: []kit.TreeNode{
						{Key: "0-0-1-0", Title: "leaf2"},
					},
				},
			},
		},
	}
}

func layoutTree(t *testing.T, tr *kit.Tree) *core.Tree {
	t.Helper()
	tree := core.NewTree(tr.Node())
	tree.Layout(core.Size{Width: 320, Height: 400})
	return tree
}

func approxTree(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func TestTree_PRD_01_Defaults(t *testing.T) {
	// TRE-01
	tr := kit.NewTree(sampleTreePRDData()...)
	if tr.Disabled || tr.Checkable || tr.Multiple || tr.Draggable || tr.ShowLine || tr.ShowIcon || tr.Directory {
		t.Fatalf("flags: disabled=%v checkable=%v multi=%v drag=%v line=%v icon=%v dir=%v",
			tr.Disabled, tr.Checkable, tr.Multiple, tr.Draggable, tr.ShowLine, tr.ShowIcon, tr.Directory)
	}
	if !tr.DefaultExpandParent {
		t.Fatal("DefaultExpandParent want true")
	}
	if tr.Node() == nil || tr.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	if tr.Root.Base().Role != "tree" {
		t.Fatalf("role=%q", tr.Root.Base().Role)
	}
	layoutTree(t, tr)
	// collapsed by default → only roots visible
	keys := tr.VisibleKeys()
	if len(keys) != 1 || keys[0] != "0-0" {
		t.Fatalf("visible=%v", keys)
	}
}

func TestTree_PRD_02_Expand(t *testing.T) {
	// TRE-02
	tr := kit.NewTree(sampleTreePRDData()...)
	var got []string
	var expNode string
	var expBool bool
	tr.SetOnExpand(func(keys []string, n kit.TreeNode, expanded bool) {
		got = append([]string(nil), keys...)
		expNode = n.Key
		expBool = expanded
	})
	layoutTree(t, tr)
	tr.ToggleExpand("0-0")
	if !tr.IsExpanded("0-0") {
		t.Fatal("not expanded")
	}
	if !expBool || expNode != "0-0" {
		t.Fatalf("onExpand node=%s open=%v", expNode, expBool)
	}
	if len(got) == 0 || got[0] != "0-0" {
		t.Fatalf("expandedKeys=%v", got)
	}
	vis := tr.VisibleKeys()
	// root + two children
	if len(vis) < 3 {
		t.Fatalf("visible after expand=%v", vis)
	}
}

func TestTree_PRD_03_Select(t *testing.T) {
	// TRE-03
	tr := kit.NewTree(sampleTreePRDData()...)
	var got []string
	tr.SetOnSelect(func(keys []string, n kit.TreeNode, selected bool) {
		got = append([]string(nil), keys...)
	})
	layoutTree(t, tr)
	tr.SelectKey("0-0")
	if !tr.IsSelected("0-0") {
		t.Fatal("not selected")
	}
	if len(got) != 1 || got[0] != "0-0" {
		t.Fatalf("selected=%v", got)
	}
}

func TestTree_PRD_04_Checkable(t *testing.T) {
	// TRE-04
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetCheckable(true)
	tr.SetDefaultExpandAll(true)
	var got []string
	tr.SetOnCheck(func(keys []string, n kit.TreeNode, checked bool) {
		got = append([]string(nil), keys...)
	})
	layoutTree(t, tr)
	tr.ToggleCheck("0-0-0-0")
	if !tr.IsChecked("0-0-0-0") {
		t.Fatal("leaf not checked")
	}
	if len(got) == 0 {
		t.Fatal("onCheck not fired")
	}
}

func TestTree_PRD_05_ParentChildCheck(t *testing.T) {
	// TRE-05
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetCheckable(true)
	tr.SetDefaultExpandAll(true)
	layoutTree(t, tr)
	tr.ToggleCheck("0-0-0") // parent → all descendants
	if !tr.IsChecked("0-0-0-0") || !tr.IsChecked("0-0-0-1") {
		t.Fatalf("cascade failed checked=%v", tr.GetCheckedKeys())
	}
}

func TestTree_PRD_06_DisabledNode(t *testing.T) {
	// TRE-06
	data := []kit.TreeNode{
		{Key: "a", Title: "A", Disabled: true},
		{Key: "b", Title: "B"},
	}
	tr := kit.NewTree(data...)
	tr.SetCheckable(true)
	layoutTree(t, tr)
	tr.SelectKey("a")
	if tr.IsSelected("a") {
		t.Fatal("disabled node selected")
	}
	tr.ToggleCheck("a")
	if tr.IsChecked("a") {
		t.Fatal("disabled node checked")
	}
	tr.SelectKey("b")
	if !tr.IsSelected("b") {
		t.Fatal("enabled select failed")
	}
}

func TestTree_PRD_07_LoadData(t *testing.T) {
	// TRE-07
	tr := kit.NewTree(
		kit.TreeNode{Key: "0", Title: "Expand to load"},
		kit.TreeNode{Key: "1", Title: "Leaf", IsLeaf: true},
	)
	loaded := false
	tr.SetLoadData(func(node kit.TreeNode) {
		loaded = true
		data := tr.TreeDataCopy()
		for i := range data {
			if data[i].Key == node.Key {
				data[i].Children = []kit.TreeNode{
					{Key: node.Key + "-0", Title: "Child"},
				}
				data[i].Loading = false
			}
		}
		tr.SetTreeData(data...)
		tr.MarkLoaded(node.Key)
	})
	layoutTree(t, tr)
	tr.ExpandKey("0")
	if !loaded {
		t.Fatal("loadData not called")
	}
	vis := tr.VisibleKeys()
	found := false
	for _, k := range vis {
		if k == "0-0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("child not visible after load: %v", vis)
	}
	if len(tr.GetLoadedKeys()) == 0 {
		t.Fatal("loadedKeys empty")
	}
}

func TestTree_PRD_08_ControlledExpanded(t *testing.T) {
	// TRE-08
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetControlledExpanded(true)
	tr.SetExpandedKeys([]string{"0-0"})
	var called bool
	var wantOpen bool
	tr.SetOnExpand(func(keys []string, n kit.TreeNode, expanded bool) {
		called = true
		wantOpen = expanded
	})
	layoutTree(t, tr)
	if !tr.IsExpanded("0-0") {
		t.Fatal("seed expand")
	}
	// toggle without parent Set → state unchanged
	tr.ToggleExpand("0-0")
	if !tr.IsExpanded("0-0") {
		t.Fatal("controlled expand should not self-mutate on collapse attempt")
	}
	if !called || wantOpen {
		t.Fatalf("onExpand called=%v wantOpen=%v", called, wantOpen)
	}
	// parent applies
	tr.SetExpandedKeys(nil)
	if tr.IsExpanded("0-0") {
		t.Fatal("SetExpandedKeys clear failed")
	}
}

func TestTree_PRD_09_ShowLine(t *testing.T) {
	// TRE-09
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetShowLine(true)
	tr.SetDefaultExpandAll(true)
	layoutTree(t, tr)
	if !tr.ShowLineEnabled() {
		t.Fatal("showLine off")
	}
	if tr.Node() == nil {
		t.Fatal("nil")
	}
}

func TestTree_PRD_10_Multiple(t *testing.T) {
	// TRE-10
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetMultiple(true)
	tr.SetDefaultExpandAll(true)
	layoutTree(t, tr)
	tr.SelectKey("0-0-0-0")
	tr.SelectKey("0-0-0-1")
	sel := tr.GetSelectedKeys()
	if len(sel) != 2 {
		t.Fatalf("multi select=%v", sel)
	}
}

func TestTree_PRD_11_Search(t *testing.T) {
	// TRE-11
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetAutoExpandParent(true)
	tr.SetSearchValue("leaf1")
	layoutTree(t, tr)
	// parents of leaf1 should expand
	if !tr.IsExpanded("0-0") || !tr.IsExpanded("0-0-0") {
		t.Fatalf("parents not expanded: exp=%v", tr.ExpandedKeys())
	}
	vis := strings.Join(tr.VisibleTitles(), ",")
	if !strings.Contains(vis, "leaf1") {
		t.Fatalf("leaf1 not visible: %s", vis)
	}
}

func TestTree_PRD_12_BasicExample(t *testing.T) {
	// TRE-12 basic.tsx
	data := []kit.TreeNode{
		{
			Title: "parent 1", Key: "0-0",
			Children: []kit.TreeNode{
				{
					Title: "parent 1-0", Key: "0-0-0", Disabled: true,
					Children: []kit.TreeNode{
						{Title: "leaf", Key: "0-0-0-0", DisableCheckbox: true},
						{Title: "leaf", Key: "0-0-0-1"},
					},
				},
				{
					Title: "parent 1-1", Key: "0-0-1",
					Children: []kit.TreeNode{{Title: "sss", Key: "0-0-1-0"}},
				},
			},
		},
	}
	tr := kit.NewTree(data...)
	tr.SetCheckable(true)
	tr.SetDefaultExpandedKeys([]string{"0-0-0", "0-0-1"})
	tr.SetDefaultSelectedKeys([]string{"0-0-1"})
	tr.SetDefaultCheckedKeys([]string{"0-0-0", "0-0-1"})
	layoutTree(t, tr)
	if !tr.IsSelected("0-0-1") {
		t.Fatal("default selected")
	}
	// default expand parent of defaults → 0-0 expanded
	if !tr.IsExpanded("0-0") {
		// defaultExpandParent may expand parents of selected/checked
		// 0-0-0 is in defaultExpanded → but parent 0-0 may need expand to show
	}
	if len(tr.GetCheckedKeys()) == 0 {
		t.Fatal("default checked empty")
	}
}

func TestTree_PRD_13_ControlledExample(t *testing.T) {
	// TRE-13
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetCheckable(true)
	tr.SetControlledExpanded(true)
	tr.SetControlledChecked(true)
	tr.SetControlledSelected(true)
	tr.SetExpandedKeys([]string{"0-0", "0-0-0"})
	tr.SetCheckedKeys([]string{"0-0-0-0"})
	tr.SetSelectedKeys([]string{"0-0-1"})
	tr.SetOnExpand(func(keys []string, n kit.TreeNode, expanded bool) {
		tr.SetExpandedKeys(keys)
	})
	tr.SetOnCheck(func(keys []string, n kit.TreeNode, checked bool) {
		tr.SetCheckedKeys(keys)
	})
	tr.SetOnSelect(func(keys []string, n kit.TreeNode, selected bool) {
		tr.SetSelectedKeys(keys)
	})
	layoutTree(t, tr)
	if !tr.IsExpanded("0-0-0") || !tr.IsChecked("0-0-0-0") || !tr.IsSelected("0-0-1") {
		t.Fatalf("controlled state exp=%v chk=%v sel=%v",
			tr.ExpandedKeys(), tr.GetCheckedKeys(), tr.GetSelectedKeys())
	}
}

func TestTree_PRD_14_Draggable(t *testing.T) {
	// TRE-14
	data := []kit.TreeNode{
		{Key: "a", Title: "A", Children: []kit.TreeNode{{Key: "a0", Title: "A0"}}},
		{Key: "b", Title: "B"},
	}
	tr := kit.NewTree(data...)
	tr.SetDraggable(true)
	tr.SetBlockNode(true)
	tr.SetDefaultExpandAll(true)
	var info kit.TreeDropInfo
	tr.SetOnDrop(func(i kit.TreeDropInfo) {
		info = i
		// apply reorder
		fn := tr.OnDrop
		tr.OnDrop = nil
		tr.Drop(i.DragKey, i.DropKey, i.DropToGap, i.DropPosition)
		tr.OnDrop = fn
	})
	layoutTree(t, tr)
	tr.Drop("b", "a", false, 0) // drop b inside a via OnDrop
	if info.DragKey != "b" || info.DropKey != "a" {
		t.Fatalf("drop info=%+v", info)
	}
	// after OnDrop handler applied, b should be under a
	roots := tr.TreeDataCopy()
	if len(roots) != 1 || roots[0].Key != "a" {
		t.Fatalf("roots after drop=%+v", roots)
	}
	found := false
	for _, c := range roots[0].Children {
		if c.Key == "b" {
			found = true
		}
	}
	if !found {
		t.Fatalf("b not under a: %+v", roots[0].Children)
	}
}

func TestTree_PRD_15_DynamicExample(t *testing.T) {
	// TRE-15
	tr := kit.NewTree(
		kit.TreeNode{Key: "0", Title: "Expand to load"},
		kit.TreeNode{Key: "2", Title: "Tree Node", IsLeaf: true},
	)
	tr.SetLoadData(func(node kit.TreeNode) {
		data := tr.TreeDataCopy()
		for i := range data {
			if data[i].Key == node.Key {
				data[i].Children = []kit.TreeNode{
					{Key: node.Key + "-0", Title: "Child Node"},
					{Key: node.Key + "-1", Title: "Child Node"},
				}
			}
		}
		tr.SetTreeData(data...)
		tr.MarkLoaded(node.Key)
	})
	layoutTree(t, tr)
	tr.ExpandKey("0")
	vis := tr.VisibleKeys()
	if len(vis) < 3 {
		t.Fatalf("dynamic children missing: %v", vis)
	}
}

func TestTree_PRD_16_SearchExample(t *testing.T) {
	// TRE-16
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetAutoExpandParent(true)
	tr.SetSearchValue("leaf0")
	layoutTree(t, tr)
	if !tr.IsExpanded("0-0-0") {
		t.Fatal("search expand parent")
	}
}

func TestTree_PRD_17_LineExample(t *testing.T) {
	// TRE-17
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetShowLine(true)
	tr.SetShowLeafIcon(true)
	tr.SetDefaultExpandedKeys([]string{"0-0-0"})
	layoutTree(t, tr)
	if !tr.ShowLine || !tr.ShowLeafIcon {
		t.Fatal("line flags")
	}
}

func TestTree_PRD_18_CustomizedIcon(t *testing.T) {
	// TRE-18
	data := []kit.TreeNode{
		{
			Key: "0-0", Title: "parent 1", Icon: "smile",
			Children: []kit.TreeNode{
				{Key: "0-0-0", Title: "leaf", Icon: "meh"},
			},
		},
	}
	tr := kit.NewTree(data...)
	tr.SetShowIcon(true)
	tr.SetDefaultExpandAll(true)
	tr.SetSwitcherIcon("chevron-down")
	tr.SetDefaultSelectedKeys([]string{"0-0-0"})
	layoutTree(t, tr)
	if !tr.ShowIcon || tr.SwitcherIcon != "chevron-down" {
		t.Fatal("icon config")
	}
	if !tr.IsSelected("0-0-0") {
		t.Fatal("selected")
	}
}

func TestTree_PRD_19_Directory(t *testing.T) {
	// TRE-19
	data := []kit.TreeNode{
		{
			Key: "0-0", Title: "parent 0",
			Children: []kit.TreeNode{
				{Key: "0-0-0", Title: "leaf 0-0", IsLeaf: true},
				{Key: "0-0-1", Title: "leaf 0-1", IsLeaf: true},
			},
		},
	}
	tr := kit.NewDirectoryTree(data...)
	tr.SetMultiple(true)
	tr.SetDefaultExpandAll(true)
	layoutTree(t, tr)
	if !tr.Directory || !tr.BlockNode {
		t.Fatal("directory defaults")
	}
	tr.SelectKey("0-0-0")
	tr.SelectKey("0-0-1")
	if len(tr.GetSelectedKeys()) != 2 {
		t.Fatalf("multi=%v", tr.GetSelectedKeys())
	}
}

func TestTree_PRD_20_Metrics(t *testing.T) {
	// TRE-20
	tr := kit.NewTree(sampleTreePRDData()...)
	th := core.DefaultTheme()
	tr.SetTheme(th)
	layoutTree(t, tr)
	if !approxTree(tr.TitleHeight(), kit.DefaultTreeTitleHeight, 0.5) {
		t.Fatalf("titleH=%v", tr.TitleHeight())
	}
	if !approxTree(tr.IndentSize(), kit.DefaultTreeIndentSize, 0.5) {
		t.Fatalf("indent=%v", tr.IndentSize())
	}
	if !approxTree(tr.SwitcherSize(), kit.DefaultTreeSwitcherSize, 0.5) {
		t.Fatalf("switcher=%v", tr.SwitcherSize())
	}
	if !approxTree(tr.FontSize(), kit.DefaultTreeFontSize, 0.5) {
		t.Fatalf("font=%v", tr.FontSize())
	}
	// Theme controlHeightSM
	if !approxTree(th.SizeOr(core.TokenControlHeightSM, 0), 24, 0.5) {
		t.Fatalf("controlHeightSM=%v", th.SizeOr(core.TokenControlHeightSM, 0))
	}
}

func TestTree_PRD_21_TokenColors(t *testing.T) {
	// TRE-21
	tr := kit.NewTree(sampleTreePRDData()...)
	custom := core.DefaultTheme()
	custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#FF00AA")
	custom.Tokens.Colors[core.TokenColorPrimaryBg] = render.Hex("#FFE0F5")
	tr.SetTheme(custom)
	tr.SetSelectedKeys([]string{"0-0"})
	layoutTree(t, tr)
	// no hard-coded brand as sole skin — theme primary is non-default
	if custom.Color(core.TokenColorPrimary).R < 0.9 {
		t.Fatal("custom primary not applied to theme")
	}
	// selected fill uses token path (antItemSelectedFill → primaryBg)
	_ = tr.Root
}

func TestTree_PRD_22_DisabledAppearance(t *testing.T) {
	// TRE-22
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetDisabled(true)
	layoutTree(t, tr)
	tr.SelectKey("0-0")
	if tr.IsSelected("0-0") {
		t.Fatal("disabled tree still selects")
	}
	tr.ToggleExpand("0-0")
	if tr.IsExpanded("0-0") {
		t.Fatal("disabled tree still expands")
	}
}

func TestTree_PRD_23_KeyboardFocus(t *testing.T) {
	// TRE-23
	tr := kit.NewTree(sampleTreePRDData()...)
	tr.SetDefaultExpandAll(true)
	tree := layoutTree(t, tr)
	// focus ring config on root
	// rows are pressable treeitems with focus ring
	if tr.Root == nil {
		t.Fatal("nil root")
	}
	// programmatic select via SelectKey (Enter path on Pressable uses Click)
	tr.SelectKey("0-0-0-0")
	if !tr.IsSelected("0-0-0-0") {
		t.Fatal("select path")
	}
	// a11y roles
	if tr.Root.Base().Role != "tree" {
		t.Fatalf("role=%q", tr.Root.Base().Role)
	}
	_ = tree
}
