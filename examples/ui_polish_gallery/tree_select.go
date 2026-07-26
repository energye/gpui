//go:build linux && !nogpu

package main

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTreeSelect() {
	// TreeSelect — docs/antd/tree-select.md §6.8 P0
	// https://ant.design/components/tree-select
	// demos: basic / multiple / treeData / checkable / async /
	//        treeLine / placement / variant
	//
	// P1 not shown: maxCount, suffix/prefix, status full page,
	// semantic classNames/styles, custom tree node render.

	face, th := c.face, c.theme
	status := c.status
	vp := core.Size{Width: 1280, Height: 800}

	basicData := []kit.TreeSelectNode{
		{
			Value: "parent 1", Title: "parent 1",
			Children: []kit.TreeSelectNode{
				{
					Value: "parent 1-0", Title: "parent 1-0",
					Children: []kit.TreeSelectNode{
						{Value: "leaf1", Title: "leaf1"},
						{Value: "leaf2", Title: "leaf2"},
						{Value: "leaf3", Title: "leaf3"},
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

	treeDataDemo := []kit.TreeSelectNode{
		{
			Title: "Node1", Value: "0-0",
			Children: []kit.TreeSelectNode{
				{Title: "Child Node1", Value: "0-0-1"},
				{Title: "Child Node2", Value: "0-0-2"},
			},
		},
		{Title: "Node2", Value: "0-1"},
	}

	checkData := []kit.TreeSelectNode{
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

	track := func(ts *kit.TreeSelect) *kit.TreeSelect {
		if ts == nil {
			return nil
		}
		ts.SetFace(face)
		if th != nil {
			ts.SetTheme(th)
		}
		ts.Viewport = vp
		*c.tickers = append(*c.tickers, ts)
		return ts
	}

	// ---------- basic.tsx ----------
	basic := track(kit.NewTreeSelect("Please select", basicData...))
	basic.SetShowSearch(true)
	basic.SetAllowClear(true)
	basic.SetTreeDefaultExpandAll(true)
	basic.SetFixedWidth(280)
	basic.SetOnChange(func(v string) {
		if status != nil {
			*status = "TreeSelect basic → " + v
		}
	})
	secBasic := demoSection(face, th, "基本",
		"树型选择控件。showSearch + allowClear + treeDefaultExpandAll。",
		basic.Node())

	// ---------- multiple.tsx ----------
	multi := track(kit.NewTreeSelect("Please select", basicData...))
	multi.SetShowSearch(true)
	multi.SetAllowClear(true)
	multi.SetMultiple(true)
	multi.SetTreeDefaultExpandAll(true)
	multi.SetFixedWidth(280)
	multi.SetOnChangeMulti(func(v []string) {
		if status != nil {
			*status = "TreeSelect multiple → " + strings.Join(v, ", ")
		}
	})
	secMulti := demoSection(face, th, "多选",
		"multiple 支持多选；点击节点切换选中集合。",
		multi.Node())

	// ---------- treeData.tsx ----------
	fromData := track(kit.NewTreeSelect("Please select", treeDataDemo...))
	fromData.SetTreeDefaultExpandAll(true)
	fromData.SetFixedWidth(280)
	fromData.SetOnChange(func(v string) {
		if status != nil {
			*status = "TreeSelect treeData → " + v
		}
	})
	secData := demoSection(face, th, "从数据直接生成",
		"使用 treeData 生成树节点，无需手写 TreeNode。",
		fromData.Node())

	// ---------- checkable.tsx ----------
	check := track(kit.NewTreeSelect("Please select", checkData...))
	check.SetTreeCheckable(true)
	check.SetShowCheckedStrategy(kit.TreeSelectSHOW_PARENT)
	check.SetTreeDefaultExpandAll(true)
	check.SetDefaultValues([]string{"0-0-0"})
	check.SetFixedWidth(280)
	check.SetOnChangeMulti(func(v []string) {
		if status != nil {
			*status = "TreeSelect checkable → " + strings.Join(v, ", ")
		}
	})
	secCheck := demoSection(face, th, "可勾选",
		"treeCheckable + showCheckedStrategy=SHOW_PARENT。",
		check.Node())

	// ---------- async.tsx ----------
	asyncData := []kit.TreeSelectNode{
		{ID: "1", PID: "0", Value: "1", Title: "Expand to load"},
		{ID: "2", PID: "0", Value: "2", Title: "Expand to load"},
		{ID: "3", PID: "0", Value: "3", Title: "Tree Node", IsLeaf: true},
	}
	asyncTS := track(kit.NewTreeSelect("Please select", asyncData...))
	asyncTS.SetTreeDataSimpleMode(true)
	asyncTS.SetFixedWidth(280)
	asyncTS.SetLoadData(func(n kit.TreeSelectNode) {
		// simulate async append under expanded node
		id := n.ID
		if id == "" {
			id = n.Value
		}
		next := append([]kit.TreeSelectNode(nil), asyncTS.TreeData...)
		next = append(next,
			kit.TreeSelectNode{ID: id + "-a", PID: id, Value: id + "-a", Title: "Tree Node", IsLeaf: true},
			kit.TreeSelectNode{ID: id + "-b", PID: id, Value: id + "-b", Title: "Tree Node", IsLeaf: true},
			kit.TreeSelectNode{ID: id + "-c", PID: id, Value: id + "-c", Title: "Expand to load"},
		)
		for i := range next {
			if next[i].Value == n.Value {
				next[i].Loading = false
			}
		}
		asyncTS.TreeData = next
		asyncTS.NotifyTreeDataChanged()
		if status != nil {
			*status = fmt.Sprintf("TreeSelect loadData under %s", n.Value)
		}
	})
	asyncTS.SetOnChange(func(v string) {
		if status != nil {
			*status = "TreeSelect async → " + v
		}
	})
	secAsync := demoSection(face, th, "异步加载",
		"treeDataSimpleMode + loadData：展开时异步挂载子节点。",
		asyncTS.Node())

	// ---------- treeLine.tsx ----------
	line := track(kit.NewTreeSelect("Please select", basicData...))
	line.SetTreeLine(true)
	line.SetTreeIcon(true)
	line.SetTreeDefaultExpandAll(true)
	line.SetFixedWidth(300)
	secLine := demoSection(face, th, "线性样式",
		"treeLine + treeIcon 展示树形引导线与前置图标槽。",
		line.Node())

	// ---------- placement.tsx ----------
	place := track(kit.NewTreeSelect("Please select", basicData...))
	place.SetShowSearch(true)
	place.SetAllowClear(true)
	place.SetTreeDefaultExpandAll(true)
	place.SetPopupMatchSelectWidth(false)
	place.SetPlacement(kit.TreeSelectTopLeft)
	place.SetFixedWidth(280)
	// radio-like placement switcher
	placeRadios := kit.NewRadioGroup()
	placeRadios.SetOptions(
		kit.RadioOption{Value: "topLeft", Label: "topLeft"},
		kit.RadioOption{Value: "topRight", Label: "topRight"},
		kit.RadioOption{Value: "bottomLeft", Label: "bottomLeft"},
		kit.RadioOption{Value: "bottomRight", Label: "bottomRight"},
	)
	placeRadios.SetValue("topLeft")
	placeRadios.SetOnChange(func(v string) {
		switch v {
		case "topLeft":
			place.SetPlacement(kit.TreeSelectTopLeft)
		case "topRight":
			place.SetPlacement(kit.TreeSelectTopRight)
		case "bottomLeft":
			place.SetPlacement(kit.TreeSelectBottomLeft)
		case "bottomRight":
			place.SetPlacement(kit.TreeSelectBottomRight)
		}
		if status != nil {
			*status = "TreeSelect placement → " + v
		}
	})
	secPlace := demoSection(face, th, "弹出位置",
		"placement：topLeft / topRight / bottomLeft / bottomRight。",
		spaceWrap(12, placeRadios.Node(), place.Node()))

	// ---------- variant.tsx ----------
	vBorderless := track(kit.NewTreeSelect("Please select", basicData...))
	vBorderless.SetVariant(kit.InputBorderless)
	vBorderless.SetFixedWidth(280)
	vFilled := track(kit.NewTreeSelect("Please select", basicData...))
	vFilled.SetVariant(kit.InputFilled)
	vFilled.SetFixedWidth(280)
	vOutlined := track(kit.NewTreeSelect("Please select", basicData...))
	vOutlined.SetVariant(kit.InputOutlined)
	vOutlined.SetFixedWidth(280)
	vUnderlined := track(kit.NewTreeSelect("Please select", basicData...))
	vUnderlined.SetVariant(kit.InputUnderlined)
	vUnderlined.SetFixedWidth(280)
	secVariant := demoSection(face, th, "形态变体",
		"variant：borderless / filled / outlined / underlined。",
		spaceWrap(12, vBorderless.Node(), vFilled.Node(), vOutlined.Node(), vUnderlined.Node()))

	// size ladder (metric demo, not a separate official demo but useful)
	szS := track(kit.NewTreeSelect("small", basicData...))
	szS.SetSize(kit.InputSmall)
	szS.SetFixedWidth(200)
	szM := track(kit.NewTreeSelect("middle", basicData...))
	szM.SetSize(kit.InputMiddle)
	szM.SetFixedWidth(200)
	szL := track(kit.NewTreeSelect("large", basicData...))
	szL.SetSize(kit.InputLarge)
	szL.SetFixedWidth(200)
	secSize := demoSection(face, th, "三种大小",
		"size：small(24) / middle(32) / large(40)。",
		spaceWrap(12, szS.Node(), szM.Node(), szL.Node()))

	// status
	stErr := track(kit.NewTreeSelect("error", basicData...))
	stErr.SetStatus(kit.InputStatusError)
	stErr.SetFixedWidth(200)
	stWarn := track(kit.NewTreeSelect("warning", basicData...))
	stWarn.SetStatus(kit.InputStatusWarning)
	stWarn.SetFixedWidth(200)
	secStatus := demoSection(face, th, "自定义状态",
		"status：error / warning 边框语义色。",
		spaceWrap(12, stErr.Node(), stWarn.Node()))

	// Lifecycle (#9)
	life := track(kit.NewTreeSelect("Lifecycle", basicData...))
	life.SetDefaultValue("leaf1")
	life.SetSize(kit.InputLarge)
	life.SetStatus(kit.InputStatusWarning)
	life.SetFixedWidth(240)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Size) then chromeChange (Status)；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeTreeSelect, func(pc *core.PaintContext, n core.Node) {
		d, ok := n.(*primitive.Decorated)
		if !ok || d == nil {
			return
		}
		if d.Base().Key == "tree-select-skin-demo" {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
		}
		if p := baseSkin.Painter(kit.TypeTreeSelect); p != nil {
			p(pc, d)
			return
		}
		primitive.PaintDecorated(pc, d)
	})
	skinTS := track(kit.NewTreeSelect("skin override", basicData...))
	skinTS.SetFixedWidth(240)
	if dec, ok := skinTS.ChromeNode().(*primitive.Decorated); ok {
		dec.Base().Key = "tree-select-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"decor.SkinType=kit.TreeSelect。Key=tree-select-skin-demo → 蓝色 2px 边框 Override。",
		skinTS.Node())

	page := primitive.Column(
		secBasic, secMulti, secData, secCheck,
		secAsync, secLine, secPlace, secVariant,
		secSize, secStatus, secLife, secSkin,
	)
	page.Gap = 24
	page.CrossAlign = core.CrossStart
	page.MainAlign = core.MainStart

	c.add("tree_select", "TreeSelect", "Data Entry · TreeSelect", page)
}
