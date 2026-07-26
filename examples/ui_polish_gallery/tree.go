//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTree() {
	// Tree — docs/antd/tree.md §6.8 P0
	// https://ant.design/components/tree
	// demos: basic / basic-controlled / draggable / dynamic /
	//        search / line / customized-icon / directory
	//
	// P1 not shown: virtual-scroll, block-node standalone page,
	// switcher-icon full page, semantic classNames/styles, ConfigProvider.

	face, th := c.face, c.theme
	status := c.status

	wire := func(tr *kit.Tree, tag string) *kit.Tree {
		if tr == nil {
			return nil
		}
		tr.SetFace(face)
		if th != nil {
			tr.SetTheme(th)
		}
		tr.SetOnSelect(func(keys []string, n kit.TreeNode, selected bool) {
			if status != nil {
				*status = fmt.Sprintf("tree %s select=%v node=%s sel=%v", tag, keys, n.Key, selected)
			}
		})
		tr.SetOnExpand(func(keys []string, n kit.TreeNode, expanded bool) {
			if status != nil {
				*status = fmt.Sprintf("tree %s expand=%v node=%s open=%v", tag, keys, n.Key, expanded)
			}
		})
		tr.SetOnCheck(func(keys []string, n kit.TreeNode, checked bool) {
			if status != nil {
				*status = fmt.Sprintf("tree %s check=%v node=%s on=%v", tag, keys, n.Key, checked)
			}
		})
		*c.tickers = append(*c.tickers, tr)
		return tr
	}

	// ---------- basic.tsx ----------
	basicData := []kit.TreeNode{
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
					Children: []kit.TreeNode{
						{Title: "sss", Key: "0-0-1-0"},
					},
				},
			},
		},
	}
	basic := wire(kit.NewTree(basicData...), "basic")
	basic.SetCheckable(true)
	basic.SetDefaultExpandedKeys([]string{"0-0-0", "0-0-1"})
	basic.SetDefaultSelectedKeys([]string{"0-0-1"})
	basic.SetDefaultCheckedKeys([]string{"0-0-0", "0-0-1"})
	secBasic := demoSection(face, th, "基本",
		"basic.tsx：checkable + defaultExpanded/Selected/CheckedKeys。",
		basic.Node())

	// ---------- basic-controlled.tsx ----------
	ctrlData := []kit.TreeNode{
		{
			Title: "0-0", Key: "0-0",
			Children: []kit.TreeNode{
				{
					Title: "0-0-0", Key: "0-0-0",
					Children: []kit.TreeNode{
						{Title: "0-0-0-0", Key: "0-0-0-0"},
						{Title: "0-0-0-1", Key: "0-0-0-1"},
						{Title: "0-0-0-2", Key: "0-0-0-2"},
					},
				},
				{
					Title: "0-0-1", Key: "0-0-1",
					Children: []kit.TreeNode{
						{Title: "0-0-1-0", Key: "0-0-1-0"},
						{Title: "0-0-1-1", Key: "0-0-1-1"},
						{Title: "0-0-1-2", Key: "0-0-1-2"},
					},
				},
				{Title: "0-0-2", Key: "0-0-2"},
			},
		},
		{
			Title: "0-1", Key: "0-1",
			Children: []kit.TreeNode{
				{Title: "0-1-0-0", Key: "0-1-0-0"},
				{Title: "0-1-0-1", Key: "0-1-0-1"},
				{Title: "0-1-0-2", Key: "0-1-0-2"},
			},
		},
		{Title: "0-2", Key: "0-2"},
	}
	ctrl := wire(kit.NewTree(ctrlData...), "controlled")
	ctrl.SetCheckable(true)
	ctrl.SetControlledExpanded(true)
	ctrl.SetControlledSelected(true)
	ctrl.SetControlledChecked(true)
	ctrl.SetExpandedKeys([]string{"0-0-0", "0-0-1"})
	ctrl.SetCheckedKeys([]string{"0-0-0"})
	ctrl.SetSelectedKeys(nil)
	ctrl.SetOnExpand(func(keys []string, n kit.TreeNode, expanded bool) {
		ctrl.SetExpandedKeys(keys)
		if status != nil {
			*status = fmt.Sprintf("tree controlled expand=%v", keys)
		}
	})
	ctrl.SetOnCheck(func(keys []string, n kit.TreeNode, checked bool) {
		ctrl.SetCheckedKeys(keys)
		if status != nil {
			*status = fmt.Sprintf("tree controlled check=%v", keys)
		}
	})
	ctrl.SetOnSelect(func(keys []string, n kit.TreeNode, selected bool) {
		ctrl.SetSelectedKeys(keys)
		if status != nil {
			*status = fmt.Sprintf("tree controlled select=%v", keys)
		}
	})
	secCtrl := demoSection(face, th, "受控操作示例",
		"basic-controlled.tsx：expandedKeys / selectedKeys / checkedKeys 受控。",
		ctrl.Node())

	// ---------- draggable.tsx ----------
	genDrag := func() []kit.TreeNode {
		var roots []kit.TreeNode
		for i := 0; i < 3; i++ {
			k := fmt.Sprintf("0-%d", i)
			n := kit.TreeNode{Title: k, Key: k}
			if i < 2 {
				var kids []kit.TreeNode
				for j := 0; j < 3; j++ {
					ck := fmt.Sprintf("%s-%d", k, j)
					kids = append(kids, kit.TreeNode{Title: ck, Key: ck})
				}
				n.Children = kids
			}
			roots = append(roots, n)
		}
		return roots
	}
	drag := wire(kit.NewTree(genDrag()...), "drag")
	drag.SetDraggable(true)
	drag.SetBlockNode(true)
	drag.SetDefaultExpandedKeys([]string{"0-0", "0-0-0"})
	drag.SetOnDrop(func(info kit.TreeDropInfo) {
		if status != nil {
			*status = fmt.Sprintf("tree drag drop %s → %s pos=%d gap=%v",
				info.DragKey, info.DropKey, info.DropPosition, info.DropToGap)
		}
		fn := drag.OnDrop
		drag.OnDrop = nil
		drag.Drop(info.DragKey, info.DropKey, info.DropToGap, info.DropPosition)
		drag.OnDrop = fn
	})
	secDrag := demoSection(face, th, "拖动示例",
		"draggable.tsx：draggable + blockNode + onDrop 重排。",
		drag.Node())

	// ---------- dynamic.tsx (async loadData) ----------
	asyncData := []kit.TreeNode{
		{Title: "Expand to load", Key: "0"},
		{Title: "Expand to load", Key: "1"},
		{Title: "Tree Node", Key: "2", IsLeaf: true},
	}
	async := wire(kit.NewTree(asyncData...), "async")
	async.SetLoadData(func(node kit.TreeNode) {
		key := node.Key
		data := async.TreeDataCopy()
		var update func(nodes []kit.TreeNode) []kit.TreeNode
		update = func(nodes []kit.TreeNode) []kit.TreeNode {
			for i := range nodes {
				if nodes[i].Key == key {
					nodes[i].Children = []kit.TreeNode{
						{Title: "Child Node", Key: key + "-0"},
						{Title: "Child Node", Key: key + "-1"},
					}
					nodes[i].Loading = false
					return nodes
				}
				if len(nodes[i].Children) > 0 {
					nodes[i].Children = update(nodes[i].Children)
				}
			}
			return nodes
		}
		async.SetTreeData(update(data)...)
		async.MarkLoaded(key)
		if status != nil {
			*status = "tree async loaded " + key
		}
	})
	secAsync := demoSection(face, th, "异步数据加载",
		"dynamic.tsx：loadData 展开叶节点后出现子节点 + Loading Ticker。",
		async.Node())

	// ---------- search.tsx ----------
	searchRoots := []kit.TreeNode{
		{
			Title: "0-0", Key: "0-0",
			Children: []kit.TreeNode{
				{Title: "0-0-0", Key: "0-0-0", Children: []kit.TreeNode{
					{Title: "0-0-0-0", Key: "0-0-0-0"},
					{Title: "0-0-0-1", Key: "0-0-0-1"},
				}},
				{Title: "0-0-1", Key: "0-0-1", Children: []kit.TreeNode{
					{Title: "0-0-1-0", Key: "0-0-1-0"},
				}},
			},
		},
		{Title: "0-1", Key: "0-1", Children: []kit.TreeNode{
			{Title: "0-1-0", Key: "0-1-0"},
			{Title: "0-1-1", Key: "0-1-1"},
		}},
		{Title: "0-2", Key: "0-2"},
	}
	searchTree := wire(kit.NewTree(searchRoots...), "search")
	searchTree.SetAutoExpandParent(true)
	searchIn := kit.NewSearch("Search")
	searchIn.SetFace(face)
	if th != nil {
		searchIn.SetTheme(th)
	}
	searchIn.SetFixedSize(240, 32)
	searchIn.SetOnChange(func(v string) {
		searchTree.SetAutoExpandParent(true)
		searchTree.SetSearchValue(v)
		if status != nil {
			*status = "tree search=" + v
		}
	})
	searchCol := primitive.Column(searchIn.Node(), searchTree.Node())
	searchCol.Gap = 8
	searchCol.CrossAlign = core.CrossStart
	secSearch := demoSection(face, th, "可搜索",
		"search.tsx：外部 Search + searchValue 高亮 + autoExpandParent。",
		searchCol)

	// ---------- line.tsx ----------
	lineData := []kit.TreeNode{
		{
			Title: "parent 1", Key: "0-0", Icon: "folder",
			Children: []kit.TreeNode{
				{
					Title: "parent 1-0", Key: "0-0-0", Icon: "folder",
					Children: []kit.TreeNode{
						{Title: "leaf", Key: "0-0-0-0", Icon: "file"},
						{Title: "leaf", Key: "0-0-0-1", Icon: "file"},
						{Title: "leaf", Key: "0-0-0-2", Icon: "file"},
					},
				},
				{
					Title: "parent 1-1", Key: "0-0-1", Icon: "folder",
					Children: []kit.TreeNode{
						{Title: "leaf", Key: "0-0-1-0", Icon: "file"},
					},
				},
			},
		},
		{
			Title: "parent 2", Key: "0-1", Icon: "folder",
			Children: []kit.TreeNode{
				{Title: "parent 2-0", Key: "0-1-0", Icon: "folder",
					Children: []kit.TreeNode{
						{Title: "leaf", Key: "0-1-0-0", Icon: "file"},
						{Title: "leaf", Key: "0-1-0-1", Icon: "file"},
					}},
			},
		},
	}
	line := wire(kit.NewTree(lineData...), "line")
	line.SetShowLine(true)
	line.SetShowLeafIcon(true)
	line.SetDefaultExpandedKeys([]string{"0-0-0"})
	secLine := demoSection(face, th, "连接线",
		"line.tsx：showLine + showLeafIcon。",
		line.Node())

	// ---------- customized-icon.tsx ----------
	iconData := []kit.TreeNode{
		{
			Title: "parent 1", Key: "0-0", Icon: "smile",
			Children: []kit.TreeNode{
				{Title: "leaf", Key: "0-0-0", Icon: "meh"},
				{Title: "leaf", Key: "0-0-1", Icon: "frown"},
			},
		},
	}
	icons := wire(kit.NewTree(iconData...), "icon")
	icons.SetShowIcon(true)
	icons.SetDefaultExpandAll(true)
	icons.SetDefaultSelectedKeys([]string{"0-0-0"})
	icons.SetSwitcherIcon("chevron-down")
	secIcon := demoSection(face, th, "自定义图标",
		"customized-icon.tsx：showIcon + 节点 icon + switcherIcon。",
		icons.Node())

	// ---------- directory.tsx ----------
	dirData := []kit.TreeNode{
		{
			Title: "parent 0", Key: "0-0",
			Children: []kit.TreeNode{
				{Title: "leaf 0-0", Key: "0-0-0", IsLeaf: true},
				{Title: "leaf 0-1", Key: "0-0-1", IsLeaf: true},
			},
		},
		{
			Title: "parent 1", Key: "0-1",
			Children: []kit.TreeNode{
				{Title: "leaf 1-0", Key: "0-1-0", IsLeaf: true},
				{Title: "leaf 1-1", Key: "0-1-1", IsLeaf: true},
			},
		},
	}
	dir := wire(kit.NewDirectoryTree(dirData...), "directory")
	dir.SetMultiple(true)
	dir.SetDraggable(true)
	dir.SetDefaultExpandAll(true)
	secDir := demoSection(face, th, "目录",
		"directory.tsx：DirectoryTree（directory 选中主色底）+ multiple + draggable。",
		dir.Node())

	// Lifecycle (#9)
	lifeData := []kit.TreeNode{
		{Title: "parent", Key: "l-0", Children: []kit.TreeNode{
			{Title: "leaf a", Key: "l-0-0"},
			{Title: "leaf b", Key: "l-0-1"},
		}},
	}
	life := wire(kit.NewTree(lifeData...), "life")
	life.SetDefaultExpandAll(true)
	life.SetCheckable(true)
	life.SetShowLine(true)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：TreeData → Checkable → ShowLine；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeTree, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "tree-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypeTree); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeTree); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinData := []kit.TreeNode{
		{Title: "skin parent", Key: "s-0", Children: []kit.TreeNode{
			{Title: "skin leaf", Key: "s-0-0"},
		}},
	}
	skinTr := wire(kit.NewTree(skinData...), "skin")
	skinTr.SetDefaultExpandAll(true)
	skinNode := skinTr.Node()
	if d, ok := skinTr.ChromeNode().(*primitive.Decorated); ok && d != nil {
		d.Base().Key = "tree-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root.SkinType=kit.Tree 已注册。Key=tree-skin-demo → 蓝边框 Override。",
		skinNode)

	page := demoPage(face, "Tree 树形控件",
		"多层次的结构列表。P0 对齐 docs/antd/tree.md §6（官方 basic / controlled / draggable / dynamic / search / line / customized-icon / directory）。Also #9 lifecycle + #6 Skin.",
		secBasic, secCtrl, secDrag, secAsync, secSearch, secLine, secIcon, secDir, secLife, secSkin)
	c.addPage("tree", "Tree", page)
}
