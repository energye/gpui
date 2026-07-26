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

func (c *catalogCtx) registerCascader() {
	// Cascader — antd demos §6.8 P0:
	// 基本 / 默认值 / 可以自定义显示 / 移入展开 / 禁用选项 /
	// 选择即改变 / 多选 / 自定义回填方式
	// https://ant.design/components/cascader
	//
	// P1 not shown: size full matrix, custom tagRender, advanced search object form,
	// full lazy load visual demo.

	face, th := c.face, c.theme
	status := c.status
	vp := core.Size{Width: 1280, Height: 800}

	sample := []kit.CascaderOption{
		{
			Value: "zhejiang", Label: "Zhejiang",
			Children: []kit.CascaderOption{
				{
					Value: "hangzhou", Label: "Hangzhou",
					Children: []kit.CascaderOption{
						{Value: "xihu", Label: "West Lake"},
					},
				},
			},
		},
		{
			Value: "jiangsu", Label: "Jiangsu",
			Children: []kit.CascaderOption{
				{
					Value: "nanjing", Label: "Nanjing",
					Children: []kit.CascaderOption{
						{Value: "zhonghuamen", Label: "Zhong Hua Men"},
					},
				},
			},
		},
	}

	multiOpts := []kit.CascaderOption{
		{
			Label: "Light", Value: "light",
			Children: []kit.CascaderOption{
				{Label: "Number 0", Value: "0"},
				{Label: "Number 1", Value: "1"},
				{Label: "Number 2", Value: "2"},
			},
		},
		{
			Label: "Bamboo", Value: "bamboo",
			Children: []kit.CascaderOption{
				{
					Label: "Little", Value: "little",
					Children: []kit.CascaderOption{
						{Label: "Toy Fish", Value: "fish"},
						{Label: "Toy Cards", Value: "cards"},
						{Label: "Toy Bird", Value: "bird"},
					},
				},
			},
		},
	}

	track := func(cas *kit.Cascader) *kit.Cascader {
		if cas == nil {
			return nil
		}
		cas.SetFace(face)
		if th != nil {
			cas.SetTheme(th)
		}
		cas.Viewport = vp
		return cas
	}

	// ── 基本 ──────────────────────────────────────────────
	basic := track(kit.NewCascader("Please select", sample...))
	basic.SetOnChange(func(v []string, _ []kit.CascaderOption) {
		if status != nil {
			*status = fmt.Sprintf("Cascader basic: %v", v)
		}
	})
	secBasic := demoSection(face, th, "基本",
		"级联选择框。",
		basic.Node())

	// ── 默认值 ────────────────────────────────────────────
	defv := track(kit.NewCascader("Please select", sample...))
	defv.SetDefaultValue([]string{"zhejiang", "hangzhou", "xihu"})
	secDefault := demoSection(face, th, "默认值",
		"默认值通过 defaultValue 设置。",
		defv.Node())

	// ── 可以自定义显示（custom trigger） ──────────────────
	customText := kit.NewText("Unselect")
	customText.SetFace(face)
	casCustom := track(kit.NewCascader("", sample...))
	link := kit.NewText("Change city")
	link.SetFace(face)
	casCustom.SetTriggerNode(link.Node())
	casCustom.SetOnChange(func(_ []string, selected []kit.CascaderOption) {
		labels := make([]string, 0, len(selected))
		for _, o := range selected {
			labels = append(labels, o.DisplayLabel())
		}
		customText.SetValue(strings.Join(labels, ", "))
		if status != nil {
			*status = "Cascader custom trigger: " + strings.Join(labels, ", ")
		}
	})
	customRow := primitive.Row(customText.Node(), casCustom.Node())
	customRow.Gap = 8
	customRow.CrossAlign = core.CrossCenter
	secCustom := demoSection(face, th, "可以自定义显示",
		"切换按钮和结果分开。",
		customRow)

	// ── 移入展开 ──────────────────────────────────────────
	hover := track(kit.NewCascader("Please select", sample...))
	hover.SetExpandTrigger(kit.CascaderExpandHover)
	hover.SetDisplayRender(func(labels []string, _ []kit.CascaderOption) string {
		if len(labels) == 0 {
			return ""
		}
		return labels[len(labels)-1]
	})
	hover.SetOnChange(func(v []string, _ []kit.CascaderOption) {
		if status != nil {
			*status = fmt.Sprintf("Cascader hover: %v", v)
		}
	})
	secHover := demoSection(face, th, "移入展开",
		"移入展开二级菜单，点击选中选项。",
		hover.Node())

	// ── 禁用选项 ──────────────────────────────────────────
	disOpts := make([]kit.CascaderOption, len(sample))
	copy(disOpts, sample)
	disOpts[1].Disabled = true
	dis := track(kit.NewCascader("Please select", disOpts...))
	secDisabled := demoSection(face, th, "禁用选项",
		"通过 disabled 禁用选项。",
		dis.Node())

	// ── 选择即改变 ────────────────────────────────────────
	cos := track(kit.NewCascader("Please select", sample...))
	cos.SetChangeOnSelect(true)
	cos.SetOnChange(func(v []string, _ []kit.CascaderOption) {
		if status != nil {
			*status = fmt.Sprintf("Cascader changeOnSelect: %v", v)
		}
	})
	secCOS := demoSection(face, th, "选择即改变",
		"这种交互允许只选中父级选项。",
		cos.Node())

	// ── 多选 ──────────────────────────────────────────────
	multi := track(kit.NewCascader("Please select", multiOpts...))
	multi.SetMultiple(true)
	multi.FixedWidth = 320
	multi.SetOnChangeMulti(func(v [][]string, _ [][]kit.CascaderOption) {
		if status != nil {
			*status = fmt.Sprintf("Cascader multiple: %v", v)
		}
	})
	multi.SetFace(face)
	secMulti := demoSection(face, th, "多选",
		"一次性选择多个选项。",
		multi.Node())

	// ── 自定义回填方式 ────────────────────────────────────
	child := track(kit.NewCascader("Please select", multiOpts...))
	child.SetMultiple(true)
	child.SetShowCheckedStrategy(kit.CascaderSHOW_CHILD)
	child.FixedWidth = 320
	child.SetDefaultMultiValue([][]string{
		{"bamboo", "little", "fish"},
		{"bamboo", "little", "cards"},
		{"bamboo", "little", "bird"},
	})
	child.SetFace(face)

	parent := track(kit.NewCascader("Please select", multiOpts...))
	parent.SetMultiple(true)
	parent.SetShowCheckedStrategy(kit.CascaderSHOW_PARENT)
	parent.FixedWidth = 320
	parent.SetDefaultMultiValue([][]string{{"bamboo"}})
	parent.SetFace(face)

	strategyCol := primitive.Column(child.Node(), parent.Node())
	strategyCol.Gap = 16
	secStrategy := demoSection(face, th, "自定义回填方式",
		"SHOW_CHILD 只显示子节点；SHOW_PARENT 在子节点全选时显示父节点。",
		strategyCol)

	// ── 搜索（P0 showSearch 能力冒烟） ────────────────────
	search := track(kit.NewCascader("Please select", sample...))
	search.SetShowSearch(true)
	search.SetOnSearch(func(q string) {
		if status != nil {
			*status = "Cascader search: " + q
		}
	})
	searchIn := kit.NewInput("Type to filter paths…")
	searchIn.SetFace(face)
	searchIn.SetOnChange(func(v string) {
		search.SetSearchValue(v)
	})
	searchCol := primitive.Column(searchIn.Node(), search.Node())
	searchCol.Gap = 8
	secSearch := demoSection(face, th, "搜索（P0 冒烟）",
		"showSearch 路径过滤。对象形态 filter/limit 等为 P1。",
		searchCol)

	// Lifecycle (#9)
	life := track(kit.NewCascader("Lifecycle", sample...))
	life.SetDefaultValue([]string{"zhejiang", "hangzhou", "xihu"})
	life.SetSize(kit.InputLarge)
	life.SetStatus(kit.InputStatusWarning)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Size) then chromeChange (Status)；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeCascader, func(pc *core.PaintContext, n core.Node) {
		d, ok := n.(*primitive.Decorated)
		if !ok || d == nil {
			return
		}
		if d.Base().Key == "cascader-skin-demo" {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
		}
		if p := baseSkin.Painter(kit.TypeCascader); p != nil {
			p(pc, d)
			return
		}
		primitive.PaintDecorated(pc, d)
	})
	skinC := track(kit.NewCascader("skin override", sample...))
	skinNode := skinC.Node()
	if shell := skinC.TriggerShell(); shell != nil {
		if kids := shell.Children(); len(kids) > 0 {
			if dec, ok := kids[0].(*primitive.Decorated); ok {
				dec.Base().Key = "cascader-skin-demo"
			}
		}
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"decor.SkinType=kit.Cascader。Key=cascader-skin-demo → 蓝色 2px 边框 Override。",
		skinNode)

	page := demoPage(face,
		"Cascader 级联选择",
		"级联选择框。P0 对齐 docs/antd/cascader.md §6；P1 见 coverage Notes。",
		secBasic, secDefault, secCustom, secHover, secDisabled, secCOS, secMulti, secStrategy, secSearch, secLife, secSkin,
	)
	c.addPage("cascader", "Cascader", page)
}
