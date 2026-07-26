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

func (c *catalogCtx) registerSelect() {
	// Select — docs/antd/select.md §6.8 P0
	// https://ant.design/components/select
	// demos: basic / search / search-filter-option / search-multi-field /
	//        multiple / size / option-render / search-sort
	//
	// P1 not shown: tags full, optgroup, coordinate, label-in-value,
	// virtual big-data, semantic classNames/styles, maxCount, responsive maxTagCount.

	face, th := c.face, c.theme
	status := c.status
	vp := core.Size{Width: 1280, Height: 800}

	track := func(s *kit.Select) *kit.Select {
		if s == nil {
			return nil
		}
		s.SetFace(face)
		if th != nil {
			s.SetTheme(th)
		}
		s.Viewport = vp
		*c.tickers = append(*c.tickers, s)
		return s
	}

	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}

	// ---------- basic.tsx ----------
	basicOpts := []kit.SelectOption{
		{Value: "jack", Label: "Jack"},
		{Value: "lucy", Label: "Lucy"},
		{Value: "Yiminghe", Label: "yiminghe"},
		{Value: "disabled", Label: "Disabled", Disabled: true},
	}
	basic := track(kit.NewSelect("", basicOpts...))
	basic.SetDefaultValue("lucy")
	basic.SetFixedWidth(120)
	basic.SetOnChange(func(v string) {
		if status != nil {
			*status = "select basic → " + v
		}
	})

	basicDis := track(kit.NewSelect("", kit.SelectOption{Value: "lucy", Label: "Lucy"}))
	basicDis.SetDefaultValue("lucy")
	basicDis.SetFixedWidth(120)
	basicDis.SetDisabled(true)

	basicLoad := track(kit.NewSelect("", kit.SelectOption{Value: "lucy", Label: "Lucy"}))
	basicLoad.SetDefaultValue("lucy")
	basicLoad.SetFixedWidth(120)
	basicLoad.SetLoading(true)

	basicClear := track(kit.NewSelect("select it", kit.SelectOption{Value: "lucy", Label: "Lucy"}))
	basicClear.SetDefaultValue("lucy")
	basicClear.SetFixedWidth(120)
	basicClear.SetAllowClear(true)
	basicClear.SetOnChange(func(v string) {
		if status != nil {
			*status = "select clear → " + v
		}
	})

	secBasic := demoSection(face, th, "基本使用",
		"基本使用。默认值、禁用、加载中、可清除。",
		spaceWrap(12, basic.Node(), basicDis.Node(), basicLoad.Node(), basicClear.Node()))

	// ---------- search.tsx ----------
	search := track(kit.NewSelect("Select a person",
		kit.SelectOption{Value: "jack", Label: "Jack"},
		kit.SelectOption{Value: "lucy", Label: "Lucy"},
		kit.SelectOption{Value: "tom", Label: "Tom"},
	))
	search.SetShowSearch(true)
	search.SetOptionFilterProp("label")
	search.SetFixedWidth(200)
	search.SetOnSearch(func(v string) {
		if status != nil {
			*status = "select search → " + v
		}
	})
	search.SetOnChange(func(v string) {
		if status != nil {
			*status = "select search change → " + v
		}
	})
	secSearch := demoSection(face, th, "带搜索框",
		"展开后可对选项 label 过滤；onSearch 回调搜索词。",
		search.Node())

	// ---------- search-filter-option.tsx ----------
	filter := track(kit.NewSelect("Select a person",
		kit.SelectOption{Value: "1", Label: "Jack"},
		kit.SelectOption{Value: "2", Label: "Lucy"},
		kit.SelectOption{Value: "3", Label: "Tom"},
	))
	filter.SetShowSearch(true)
	filter.SetFixedWidth(200)
	filter.SetFilterOptionFunc(func(input string, option kit.SelectOption) bool {
		return strings.Contains(strings.ToLower(option.Label), strings.ToLower(input))
	})
	secFilter := demoSection(face, th, "自定义搜索",
		"filterOption 自定义匹配逻辑（按 label 忽略大小写 contains）。",
		filter.Node())

	// ---------- search-multi-field.tsx ----------
	multiField := track(kit.NewSelect("Select an option",
		kit.SelectOption{Value: "a11", Label: "a11", Fields: map[string]string{"otherField": "c11"}},
		kit.SelectOption{Value: "b22", Label: "b22", Fields: map[string]string{"otherField": "b11"}},
		kit.SelectOption{Value: "c33", Label: "c33", Fields: map[string]string{"otherField": "b33"}},
		kit.SelectOption{Value: "d44", Label: "d44", Fields: map[string]string{"otherField": "d44"}},
	))
	multiField.SetShowSearch(true)
	multiField.SetOptionFilterProp("label", "otherField")
	multiField.SetFixedWidth(220)
	secMultiField := demoSection(face, th, "多字段搜索",
		"optionFilterProp 同时匹配 label 与 otherField。",
		multiField.Node())

	// ---------- multiple.tsx ----------
	var multiOpts []kit.SelectOption
	for i := 10; i < 36; i++ {
		v := string(rune('a'+i-10)) + fmt.Sprintf("%d", i)
		multiOpts = append(multiOpts, kit.SelectOption{Value: v, Label: v})
	}
	multi := track(kit.NewSelect("Please select", multiOpts...))
	multi.SetMode(kit.SelectMultiple)
	multi.SetAllowClear(true)
	multi.SetDefaultValues([]string{"a10", "c12"})
	multi.SetFixedWidth(360)
	multi.SetOnChangeMulti(func(v []string) {
		if status != nil {
			*status = fmt.Sprintf("select multiple → %v", v)
		}
	})
	multiDis := track(kit.NewSelect("Please select", multiOpts...))
	multiDis.SetMode(kit.SelectMultiple)
	multiDis.SetDisabled(true)
	multiDis.SetDefaultValues([]string{"a10", "c12"})
	multiDis.SetFixedWidth(360)
	secMultiple := demoSection(face, th, "多选",
		"mode=multiple；默认选中两项；可清除；禁用态。",
		col(multi.Node(), multiDis.Node()))

	// ---------- size.tsx ----------
	sizeOpts := multiOpts
	mkSize := func(sz kit.InputSize) core.Node {
		s1 := track(kit.NewSelect("", sizeOpts...))
		s1.SetSize(sz)
		s1.SetDefaultValue("a10")
		s1.SetFixedWidth(200)
		s2 := track(kit.NewSelect("Please select", sizeOpts...))
		s2.SetMode(kit.SelectMultiple)
		s2.SetSize(sz)
		s2.SetDefaultValues([]string{"a10", "c12"})
		s2.SetFixedWidth(360)
		s3 := track(kit.NewSelect("Please select", sizeOpts...))
		s3.SetMode(kit.SelectTags)
		s3.SetSize(sz)
		s3.SetDefaultValues([]string{"a10", "c12"})
		s3.SetFixedWidth(360)
		return col(s1.Node(), s2.Node(), s3.Node())
	}
	secSize := demoSection(face, th, "三种大小",
		"large / middle / small；单选、多选、tags 同步 size。",
		col(
			demoDesc(face, "Large"), mkSize(kit.InputLarge),
			demoDesc(face, "Middle"), mkSize(kit.InputMiddle),
			demoDesc(face, "Small"), mkSize(kit.InputSmall),
		))

	// ---------- option-render.tsx ----------
	moodOpts := []kit.SelectOption{
		{Label: "Happy", Value: "happy", Emoji: "😄", Desc: "Feeling Good"},
		{Label: "Sad", Value: "sad", Emoji: "😢", Desc: "Feeling Blue"},
		{Label: "Angry", Value: "angry", Emoji: "😡", Desc: "Furious"},
		{Label: "Cool", Value: "cool", Emoji: "😎", Desc: "Chilling"},
		{Label: "Sleepy", Value: "sleepy", Emoji: "😴", Desc: "Need Sleep"},
	}
	optRender := track(kit.NewSelect("Please select your current mood.", moodOpts...))
	optRender.SetMode(kit.SelectMultiple)
	optRender.SetDefaultValues([]string{"happy"})
	optRender.SetFixedWidth(360)
	optRender.SetOptionRender(func(opt kit.SelectOption) core.Node {
		lab := primitive.NewText(fmt.Sprintf("%s %s (%s)", opt.Emoji, opt.Label, opt.Desc))
		lab.Face = face
		lab.FontSize = 14
		return lab
	})
	secOptRender := demoSection(face, th, "自定义下拉选项",
		"optionRender 自定义 option 行内容（emoji + label + desc）。",
		optRender.Node())

	// ---------- search-sort.tsx ----------
	sortSel := track(kit.NewSelect("Search to Select",
		kit.SelectOption{Value: "1", Label: "Not Identified"},
		kit.SelectOption{Value: "2", Label: "Closed"},
		kit.SelectOption{Value: "3", Label: "Communicated"},
		kit.SelectOption{Value: "4", Label: "Identified"},
		kit.SelectOption{Value: "5", Label: "Resolved"},
		kit.SelectOption{Value: "6", Label: "Cancelled"},
	))
	sortSel.SetShowSearch(true)
	sortSel.SetOptionFilterProp("label")
	sortSel.SetFixedWidth(200)
	sortSel.SetFilterSort(func(a, b kit.SelectOption) int {
		al := strings.ToLower(a.Label)
		bl := strings.ToLower(b.Label)
		if al < bl {
			return -1
		}
		if al > bl {
			return 1
		}
		return 0
	})
	secSort := demoSection(face, th, "带排序的搜索",
		"filterSort 按 label 字典序排序过滤结果。",
		sortSel.Node())

	// Lifecycle (#9)
	life := track(kit.NewSelect("Lifecycle", basicOpts...))
	life.SetDefaultValue("jack")
	life.SetSize(kit.InputLarge)
	life.SetStatus(kit.InputStatusWarning)
	life.SetFixedWidth(200)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Size) then chromeChange (Status)；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeSelect, func(pc *core.PaintContext, n core.Node) {
		d, ok := n.(*primitive.Decorated)
		if !ok || d == nil {
			return
		}
		if d.Base().Key == "select-skin-demo" {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
		}
		if p := baseSkin.Painter(kit.TypeSelect); p != nil {
			p(pc, d)
			return
		}
		primitive.PaintDecorated(pc, d)
	})
	skinSel := track(kit.NewSelect("skin override", basicOpts...))
	skinSel.SetDefaultValue("lucy")
	skinSel.SetFixedWidth(200)
	if dec, ok := skinSel.ChromeNode().(*primitive.Decorated); ok {
		dec.Base().Key = "select-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"decor.SkinType=kit.Select。Key=select-skin-demo → 蓝色 2px 边框 Override。",
		skinSel.Node())

	c.addPage("select", "Select", demoPage(face,
		"Select",
		"下拉选择器。对齐 Ant Design Select 主路径（docs/antd/select.md §6 P0）。",
		secBasic, secSearch, secFilter, secMultiField, secMultiple, secSize, secOptRender, secSort, secLife, secSkin,
	))
}
