//go:build linux && !nogpu

package main

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTransfer() {
	// Transfer — docs/antd/transfer.md §6.8 P0
	// https://ant.design/components/transfer
	// demos: basic / oneWay / search / advanced / custom-item / actions /
	//        large-data (pagination) / table-transfer
	//
	// P1 not shown: tree-transfer, style-class, selectionsIcon menu,
	// selectAllLabels, semantic classNames/styles, ConfigProvider.

	face, th := c.face, c.theme
	status := c.status

	wire := func(tr *kit.Transfer, tag string) *kit.Transfer {
		tr.SetFace(face)
		if th != nil {
			tr.SetTheme(th)
		}
		tr.SetOnChange(func(keys []string, dir kit.TransferDirection, move []string) {
			if status != nil {
				*status = fmt.Sprintf("transfer %s → %s move=%v target=%v", tag, dir, move, keys)
			}
			if tr.Controlled {
				tr.SetTargetKeys(keys)
			}
		})
		return tr
	}

	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}

	mockData := func(n int) []kit.TransferItem {
		out := make([]kit.TransferItem, n)
		for i := 0; i < n; i++ {
			out[i] = kit.TransferItem{
				Key:         fmt.Sprintf("%d", i),
				Title:       fmt.Sprintf("content%d", i+1),
				Description: fmt.Sprintf("description of content%d", i+1),
			}
		}
		return out
	}

	// ---------- basic.tsx ----------
	basicData := mockData(20)
	var basicTarget []string
	for _, it := range basicData {
		// keys > 10
		var k int
		fmt.Sscanf(it.Key, "%d", &k)
		if k > 10 {
			basicTarget = append(basicTarget, it.Key)
		}
	}
	basic := wire(kit.NewTransfer(), "basic")
	basic.SetDataSource(basicData)
	basic.SetTitles("Source", "Target")
	basic.SetControlled(true)
	basic.SetTargetKeys(basicTarget)
	basic.SetRender(func(it kit.TransferItem) string { return it.Title })
	secBasic := demoSection(face, th, "基本用法",
		"最基础的用法，展示了 dataSource、targetKeys、titles、onChange。",
		basic.Node())

	// ---------- oneWay.tsx ----------
	oneData := mockData(20)
	for i := range oneData {
		oneData[i].Disabled = i%3 < 1
	}
	var oneTarget []string
	for i, it := range oneData {
		if i%3 > 1 {
			oneTarget = append(oneTarget, it.Key)
		}
	}
	one := wire(kit.NewTransfer(), "oneWay")
	one.SetDataSource(oneData)
	one.SetTitles("Source", "Target")
	one.SetOneWay(true)
	one.SetControlled(true)
	one.SetTargetKeys(oneTarget)
	one.SetRender(func(it kit.TransferItem) string { return it.Title })
	secOne := demoSection(face, th, "单向样式",
		"oneWay：仅右移操作钮；右栏项可单条移除。部分项 disabled。",
		one.Node())

	// ---------- search.tsx ----------
	searchData := mockData(20)
	var searchTarget []string
	for i, it := range searchData {
		if i%2 == 0 {
			searchTarget = append(searchTarget, it.Key)
		}
	}
	search := wire(kit.NewTransfer(), "search")
	search.SetDataSource(searchData)
	search.SetShowSearch(true)
	search.SetControlled(true)
	search.SetTargetKeys(searchTarget)
	search.SetFilterOption(func(input string, option kit.TransferItem, _ kit.TransferDirection) bool {
		return strings.Contains(option.Description, input)
	})
	search.SetRender(func(it kit.TransferItem) string { return it.Title })
	secSearch := demoSection(face, th, "带搜索框",
		"showSearch + filterOption（按 description 包含匹配）。",
		search.Node())

	// ---------- advanced.tsx ----------
	advData := mockData(20)
	var advTarget []string
	for i, it := range advData {
		if i%2 == 0 {
			advTarget = append(advTarget, it.Key)
		}
	}
	adv := wire(kit.NewTransfer(), "advanced")
	adv.SetDataSource(advData)
	adv.SetShowSearch(true)
	adv.SetActions("to right", "to left")
	adv.SetListWidth(250)
	adv.SetListHeight(300)
	adv.SetControlled(true)
	adv.SetTargetKeys(advTarget)
	adv.SetRender(func(it kit.TransferItem) string {
		return it.Title + "-" + it.Description
	})
	adv.SetFooter(func(dir kit.TransferDirection) core.Node {
		lab := "Left button reload"
		if dir == kit.TransferRight {
			lab = "Right button reload"
		}
		btn := kit.NewButton(lab)
		btn.SetSize(kit.ButtonSmall)
		btn.SetFace(face)
		btn.SetOnClick(func() {
			// reload mock: re-apply data
			adv.SetDataSource(mockData(20))
			if status != nil {
				*status = "transfer advanced reload " + string(dir)
			}
		})
		box := primitive.NewBox(btn.Node())
		box.Padding = primitive.All(8)
		return box
	})
	secAdv := demoSection(face, th, "高级用法",
		"showSearch、自定义 actions 文案、footer 重载按钮、列表 250×300。",
		adv.Node())

	// ---------- custom-item.tsx ----------
	custom := wire(kit.NewTransfer(), "custom-item")
	custom.SetDataSource(searchData)
	custom.SetControlled(true)
	custom.SetTargetKeys(searchTarget)
	custom.SetRender(func(it kit.TransferItem) string {
		return it.Title + " - " + it.Description
	})
	secCustom := demoSection(face, th, "自定义渲染行数据",
		"render 返回 title - description 字符串。",
		custom.Node())

	// ---------- actions.tsx ----------
	actData := mockData(20)
	var actTarget []string
	for _, it := range actData {
		var k int
		fmt.Sscanf(it.Key, "%d", &k)
		if k > 10 {
			actTarget = append(actTarget, it.Key)
		}
	}
	act := wire(kit.NewTransfer(), "actions")
	act.SetDataSource(actData)
	act.SetControlled(true)
	act.SetTargetKeys(actTarget)
	act.SetActions("to right", "to left")
	// demo loading on move: brief loading via SetActionLoading
	act.SetOnChange(func(keys []string, dir kit.TransferDirection, move []string) {
		act.SetTargetKeys(keys)
		if dir == kit.TransferRight {
			act.SetActionLoading(true, false)
		} else {
			act.SetActionLoading(false, true)
		}
		if status != nil {
			*status = fmt.Sprintf("transfer actions loading %s move=%v", dir, move)
		}
		// clear loading immediately for gallery (no real async host timer here)
		act.SetActionLoading(false, false)
	})
	c.trackTicker(act)
	secAct := demoSection(face, th, "自定义操作按钮",
		"actions 文案 + 操作钮 loading（Ticker）。Gallery 中 loading 瞬时演示。",
		act.Node())

	// ---------- large-data.tsx (pagination) ----------
	pageData := mockData(80)
	var pageTarget []string
	for i, it := range pageData {
		if i%2 == 0 {
			pageTarget = append(pageTarget, it.Key)
		}
	}
	paged := wire(kit.NewTransfer(), "pagination")
	paged.SetDataSource(pageData)
	paged.SetControlled(true)
	paged.SetTargetKeys(pageTarget)
	paged.SetPagination(true)
	paged.SetRender(func(it kit.TransferItem) string { return it.Title })
	secPage := demoSection(face, th, "分页",
		"pagination：默认 pageSize=10 simple；列表宽 listWidthLG=250。",
		paged.Node())

	// ---------- table-transfer.tsx ----------
	tableData := mockData(20)
	tags := []string{"cat", "dog", "bird"}
	for i := range tableData {
		tableData[i].Description = tags[i%3]
	}
	tbl := wire(kit.NewTransfer(), "table")
	tbl.SetDataSource(tableData)
	tbl.SetControlled(true)
	tbl.SetShowSearch(true)
	tbl.SetShowSelectAll(false)
	tbl.SetListWidth(360)
	tbl.SetListHeight(280)
	tbl.SetListBody(func(p kit.TransferListBodyProps) core.Node {
		// lightweight table stand-in: header + rows with checkbox
		header := primitive.Row()
		header.Gap = 12
		for _, h := range []string{"", "Name", "Tag", "Description"} {
			tx := kit.NewText(h)
			tx.SetFace(face)
			if th != nil {
				tx.SetStyle(kit.Style{Text: th.Color(core.TokenColorTextSecondary)})
			}
			header.AddChild(tx.Node())
		}
		colBody := primitive.Column(header)
		colBody.Gap = 4
		sel := map[string]bool{}
		for _, k := range p.SelectedKeys {
			sel[k] = true
		}
		for _, it := range p.FilteredItems {
			row := primitive.Row()
			row.Gap = 12
			row.CrossAlign = core.CrossCenter
			cb := kit.NewCheckbox("")
			cb.SetFace(face)
			cb.SetChecked(sel[it.Key])
			cb.SetDisabled(p.Disabled || it.Disabled)
			key := it.Key
			was := sel[it.Key]
			cb.SetOnChange(func(next bool) {
				if p.OnItemSelect != nil {
					p.OnItemSelect(key, next)
				}
			})
			_ = was
			name := kit.NewText(it.Title)
			name.SetFace(face)
			tag := kit.NewText(it.Description)
			tag.SetFace(face)
			desc := kit.NewText("description of " + it.Title)
			desc.SetFace(face)
			row.AddChild(cb.Node())
			row.AddChild(name.Node())
			row.AddChild(tag.Node())
			row.AddChild(desc.Node())
			colBody.AddChild(row)
		}
		if len(p.FilteredItems) == 0 {
			empty := kit.NewText("暂无数据")
			empty.SetFace(face)
			colBody.AddChild(empty.Node())
		}
		return colBody
	})
	secTable := demoSection(face, th, "表格穿梭框",
		"ListBody 自定义列表体（表格行 + 勾选）；showSelectAll=false。",
		tbl.Node())

	// ---------- status (P0 status chrome) ----------
	stErr := wire(kit.NewTransfer(), "status-error")
	stErr.SetStatus(kit.TransferStatusError)
	stWarn := wire(kit.NewTransfer(), "status-warning")
	stWarn.SetStatus(kit.TransferStatusWarning)
	stWarn.SetShowSearch(true)
	secStatus := demoSection(face, th, "自定义状态",
		"status=error / warning 边框语义色。",
		col(stErr.Node(), stWarn.Node()))

	// Lifecycle (#9)
	life := wire(kit.NewTransfer(), "lifecycle")
	life.SetDataSource(mockData(8))
	life.SetTargetKeys([]string{"1", "3"})
	life.SetShowSearch(true)
	life.SetTitles("Source", "Target")
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (ShowSearch/DataSource) then chromeChange (TargetKeys)；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeTransfer, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeTransfer); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinT := wire(kit.NewTransfer(), "skin")
	skinT.SetDataSource(mockData(6))
	if f, ok := skinT.Node().(*primitive.Flex); ok {
		f.SkinType = kit.TypeTransfer
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root Flex SkinType=kit.Transfer；Theme.Skin Override 可命中（default walk）。",
		skinT.Node())

	c.addPage("transfer", "Transfer",
		demoPage(face, "Transfer",
			"双栏穿梭选择框。P0 对齐 docs/antd/transfer.md §6：dataSource/targetKeys、showSearch、oneWay、pagination、ListBody、status。",
			secBasic, secOne, secSearch, secAdv, secCustom, secAct, secPage, secTable, secStatus, secLife, secSkin))
}
