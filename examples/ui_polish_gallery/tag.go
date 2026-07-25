//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTag() {
	// Tag — docs/antd/tag.md §6.8 P0
	// https://ant.design/components/tag
	// demos: basic / colorful / control / checkable / animation / icon / status / draggable
	//
	// P1 not shown: semantic classNames/styles, href/target, motion 像素级, full dnd-kit,
	// debug customize/component-token/disabled pages, ConfigProvider global.

	face, th := c.face, c.theme
	status := c.status

	track := func(tg *kit.Tag) *kit.Tag {
		tg.SetFace(face)
		if th != nil {
			tg.SetTheme(th)
		}
		return tg
	}
	trackCT := func(ct *kit.CheckableTag) *kit.CheckableTag {
		ct.SetFace(face)
		if th != nil {
			ct.SetTheme(th)
		}
		return ct
	}
	trackG := func(g *kit.CheckableTagGroup) *kit.CheckableTagGroup {
		g.SetFace(face)
		if th != nil {
			g.SetTheme(th)
		}
		return g
	}

	// ---------- basic.tsx ----------
	b1 := track(kit.NewTag("Tag 1"))
	b2 := track(kit.NewTag("Link"))
	b2.SetOnClick(func() {
		if status != nil {
			*status = "tag link click"
		}
	})
	b3 := track(kit.NewTag("Prevent Default"))
	b3.SetClosable(true)
	b3.OnClose = func(e *kit.TagCloseEvent) {
		e.PreventDefault()
		if status != nil {
			*status = "tag close prevented"
		}
	}
	b4 := track(kit.NewTag("Tag 2"))
	b4.SetClosable(true)
	b4.SetOnClose(func() {
		if status != nil {
			*status = "tag 2 closed"
		}
	})
	secBasic := demoSection(face, th, "基本",
		"普通标签、可点、closable + PreventDefault。",
		spaceWrap(8, b1.Node(), b2.Node(), b3.Node(), b4.Node()))

	// ---------- colorful.tsx ----------
	presets := []string{"magenta", "red", "volcano", "orange", "gold", "lime", "green", "cyan", "blue", "geekblue", "purple"}
	customs := []string{"#f50", "#2db7f5", "#87d068", "#108ee9"}
	colorRows := primitive.Column()
	colorRows.Gap = 12
	colorRows.CrossAlign = core.CrossStart
	for _, v := range []struct {
		name string
		v    kit.TagVariant
	}{
		{"Presets (filled)", kit.TagFilled},
		{"Presets (solid)", kit.TagSolid},
		{"Presets (outlined)", kit.TagOutlined},
	} {
		kids := make([]core.Node, 0, len(presets))
		for _, p := range presets {
			tg := track(kit.NewTag(p))
			tg.SetColor(p)
			tg.SetVariant(v.v)
			kids = append(kids, tg.Node())
		}
		colorRows.AddChild(demoSection(face, th, v.name, "", spaceWrap(6, kids...)))
	}
	for _, v := range []struct {
		name string
		v    kit.TagVariant
	}{
		{"Custom (filled)", kit.TagFilled},
		{"Custom (solid)", kit.TagSolid},
		{"Custom (outlined)", kit.TagOutlined},
	} {
		kids := make([]core.Node, 0, len(customs))
		for _, p := range customs {
			tg := track(kit.NewTag(p))
			tg.SetColor(p)
			tg.SetVariant(v.v)
			kids = append(kids, tg.Node())
		}
		colorRows.AddChild(demoSection(face, th, v.name, "", spaceWrap(6, kids...)))
	}
	secColor := demoSection(face, th, "多彩标签",
		"预设色板 × filled/solid/outlined；自定义 #hex。",
		colorRows)

	// ---------- control.tsx ----------
	ctrlTags := []string{"Unremovable", "Tag 2", "Tag 3"}
	ctrlHost := primitive.Row()
	ctrlHost.Gap = 8
	ctrlHost.Wrap = true
	ctrlHost.CrossAlign = core.CrossCenter
	var rebuildCtrl func()
	rebuildCtrl = func() {
		ctrlHost.ClearChildren()
		for i, name := range ctrlTags {
			tg := track(kit.NewTag(name))
			if i != 0 {
				tg.SetClosable(true)
				n := name
				tg.SetOnClose(func() {
					next := make([]string, 0, len(ctrlTags))
					for _, s := range ctrlTags {
						if s != n {
							next = append(next, s)
						}
					}
					ctrlTags = next
					rebuildCtrl()
					if status != nil {
						*status = fmt.Sprintf("control tags=%v", ctrlTags)
					}
				})
			}
			ctrlHost.AddChild(tg.Node())
		}
		// + New Tag
		add := track(kit.NewTag("+ New Tag"))
		add.SetBordered(true) // outlined dashed-ish via outlined
		add.SetVariant(kit.TagOutlined)
		add.SetIcon("plus")
		add.SetOnClick(func() {
			n := fmt.Sprintf("Tag %d", len(ctrlTags)+1)
			ctrlTags = append(ctrlTags, n)
			rebuildCtrl()
			if status != nil {
				*status = "added " + n
			}
		})
		ctrlHost.AddChild(add.Node())
		ctrlHost.MarkNeedsLayout()
		ctrlHost.MarkNeedsPaint()
	}
	rebuildCtrl()
	secCtrl := demoSection(face, th, "动态添加和删除",
		"closable 删除；+ New Tag 追加（control.tsx）。",
		ctrlHost)

	// ---------- checkable.tsx ----------
	yes := trackCT(kit.NewCheckableTag("Yes"))
	yes.SetDefaultChecked(true)
	yes.SetOnChange(func(on bool) {
		if status != nil {
			*status = fmt.Sprintf("checkable Yes=%v", on)
		}
	})
	gSingle := trackG(kit.NewCheckableTagGroupStrings("Movies", "Books", "Music", "Sports"))
	gSingle.SetDefaultValue("Books")
	gSingle.SetOnChange(func(v string) {
		if status != nil {
			*status = "group single=" + v
		}
	})
	gMulti := trackG(kit.NewCheckableTagGroupStrings("Movies", "Books", "Music", "Sports"))
	gMulti.SetMultiple(true)
	gMulti.SetDefaultValues([]string{"Movies", "Music"})
	gMulti.SetOnChangeMulti(func(vs []string) {
		if status != nil {
			*status = fmt.Sprintf("group multi=%v", vs)
		}
	})
	chkCol := primitive.Column(
		spaceWrap(8, yes.Node()),
		gSingle.Node(),
		gMulti.Node(),
	)
	chkCol.Gap = 12
	secCheck := demoSection(face, th, "可选择标签",
		"CheckableTag + CheckableTagGroup 单选/多选。",
		chkCol)

	// ---------- animation.tsx (instant add/remove) ----------
	animTags := []string{"Tag 1", "Tag 2", "Tag 3"}
	animHost := primitive.Row()
	animHost.Gap = 8
	animHost.Wrap = true
	var rebuildAnim func()
	rebuildAnim = func() {
		animHost.ClearChildren()
		for _, name := range animTags {
			tg := track(kit.NewTag(name))
			tg.SetClosable(true)
			n := name
			tg.SetOnClose(func() {
				next := make([]string, 0, len(animTags))
				for _, s := range animTags {
					if s != n {
						next = append(next, s)
					}
				}
				animTags = next
				rebuildAnim()
			})
			animHost.AddChild(tg.Node())
		}
		animHost.MarkNeedsLayout()
		animHost.MarkNeedsPaint()
	}
	rebuildAnim()
	addAnim := track(kit.NewTag("+ New Tag"))
	addAnim.SetVariant(kit.TagOutlined)
	addAnim.SetOnClick(func() {
		animTags = append(animTags, fmt.Sprintf("Tag %d", len(animTags)+1))
		rebuildAnim()
	})
	secAnim := demoSection(face, th, "添加动画",
		"P0 瞬时增删（motion 像素级 P1）。",
		spaceWrap(8, animHost, addAnim.Node()))

	// ---------- icon.tsx ----------
	iconRow := spaceWrap(8,
		func() core.Node {
			tg := track(kit.NewTag("Star"))
			tg.SetIcon("star")
			tg.SetColor("#55acee")
			return tg.Node()
		}(),
		func() core.Node {
			tg := track(kit.NewTag("Heart"))
			tg.SetIcon("heart")
			tg.SetColor("#cd201f")
			return tg.Node()
		}(),
		func() core.Node {
			tg := track(kit.NewTag("User"))
			tg.SetIcon("user")
			tg.SetColor("#3b5999")
			return tg.Node()
		}(),
		func() core.Node {
			ct := trackCT(kit.NewCheckableTag("Info"))
			ct.SetIcon("info")
			ct.SetDefaultChecked(true)
			return ct.Node()
		}(),
	)
	secIcon := demoSection(face, th, "图标按钮",
		"Tag / CheckableTag 前导 icon。",
		iconRow)

	// ---------- status.tsx ----------
	statusKids := make([]core.Node, 0, 15)
	for _, v := range []kit.TagVariant{kit.TagFilled, kit.TagSolid, kit.TagOutlined} {
		for _, st := range []struct {
			name string
			icon string
			spin bool
		}{
			{"success", "check", false},
			{"processing", "sync", true},
			{"warning", "info", false},
			{"error", "close", false},
			{"default", "user", false},
		} {
			tg := track(kit.NewTag(st.name))
			tg.SetColor(st.name)
			tg.SetVariant(v)
			if st.icon != "" {
				tg.SetIcon(st.icon)
				if st.spin {
					tg.SetIconSpin(true)
				}
			}
			statusKids = append(statusKids, tg.Node())
		}
	}
	secStatus := demoSection(face, th, "预设状态的标签",
		"success / processing / warning / error / default × 三 variant。",
		spaceWrap(6, statusKids...))

	// ---------- draggable.tsx (composition reorder) ----------
	dragItems := []string{"Tag 1", "Tag 2", "Tag 3"}
	dragHost := primitive.Row()
	dragHost.Gap = 8
	dragHost.Wrap = true
	var rebuildDrag func()
	rebuildDrag = func() {
		dragHost.ClearChildren()
		for i, name := range dragItems {
			tg := track(kit.NewTag(name))
			idx := i
			tg.SetOnClick(func() {
				// click cycles item toward front (simple reorder demo)
				if idx <= 0 {
					return
				}
				dragItems[idx], dragItems[idx-1] = dragItems[idx-1], dragItems[idx]
				rebuildDrag()
				if status != nil {
					*status = fmt.Sprintf("drag order=%v", dragItems)
				}
			})
			dragHost.AddChild(tg.Node())
		}
		dragHost.MarkNeedsLayout()
		dragHost.MarkNeedsPaint()
	}
	rebuildDrag()
	secDrag := demoSection(face, th, "可拖拽标签",
		"组合示意：点击与左侧交换顺序（完整 dnd-kit P1）。",
		dragHost)

	// ---------- disabled (debug demo, still useful) ----------
	d1 := track(kit.NewTag("disabled"))
	d1.SetDisabled(true)
	d2 := track(kit.NewTag("closable disabled"))
	d2.SetClosable(true)
	d2.SetDisabled(true)
	secDis := demoSection(face, th, "禁用",
		"disabled 降对比；close 不可点。",
		spaceWrap(8, d1.Node(), d2.Node()))

	page := primitive.Column(
		secBasic, secColor, secCtrl, secCheck,
		secAnim, secIcon, secStatus, secDrag, secDis,
	)
	page.Gap = 16
	page.MainAlign = core.MainStart
	page.CrossAlign = core.CrossStretch
	page.Padding = primitive.All(4)

	c.addPage("tag", "Tag", panel(face, "Data Display · Tag（docs/antd/tag.md §6.8 P0）", page))
}
