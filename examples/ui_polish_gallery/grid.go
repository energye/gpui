//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerGrid() {
	// Grid — docs/antd/grid.md §6.8 P0
	// https://ant.design/components/grid
	// demos: basic / gutter / offset / sort / flex / flex-align / flex-order / flex-stretch

	// antd demo col fill colors (alternating).
	colBg := func(i int) render.RGBA {
		if i%2 == 0 {
			return render.Hex("#1677ff")
		}
		return render.Hex("#1677ffbf")
	}
	// Content block inside Col (matches antd demo height/padding vibe).
	mkCell := func(label string, i int, h float64) core.Node {
		tx := kit.NewText(label)
		tx.SetFace(c.face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
		d := primitive.NewDecorated(tx.Node())
		d.Background = colBg(i)
		d.Padding = primitive.Symmetric(0, 8)
		if h > 0 {
			d.Height = h
			d.MinHeight = h
		} else {
			d.MinHeight = 32
		}
		d.ExpandWidth = true
		d.StretchChild = true
		return d
	}
	mkCol := func(span int, label string, i int) *kit.Col {
		col := kit.NewCol(mkCell(label, i, 32))
		if span >= 0 {
			col.SetSpan(span)
		}
		return col
	}
	rowOf := func(cols ...*kit.Col) *kit.Row {
		nodes := make([]core.Node, 0, len(cols))
		for _, col := range cols {
			if col != nil {
				nodes = append(nodes, col.Node())
			}
		}
		r := kit.NewRow(nodes...)
		return r
	}
	stackRows := func(rows ...*kit.Row) core.Node {
		col := primitive.Column()
		col.Gap = 8
		col.CrossAlign = core.CrossStretch
		col.MainAlign = core.MainStart
		for _, r := range rows {
			if r != nil {
				col.AddChild(r.Node())
			}
		}
		return col
	}
	playground := func(body core.Node) core.Node {
		d := primitive.NewDecorated(body)
		d.ExpandWidth = true
		d.StretchChild = true
		d.Radius = 0
		d.Padding = primitive.All(0)
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		return d
	}

	// ---------- basic.tsx ----------
	basic := stackRows(
		rowOf(mkCol(24, "col", 0)),
		rowOf(mkCol(12, "col-12", 0), mkCol(12, "col-12", 1)),
		rowOf(mkCol(8, "col-8", 0), mkCol(8, "col-8", 1), mkCol(8, "col-8", 2)),
		rowOf(mkCol(6, "col-6", 0), mkCol(6, "col-6", 1), mkCol(6, "col-6", 2), mkCol(6, "col-6", 3)),
	)
	secBasic := demoSection(c.face, c.theme, "基础栅格",
		"antd basic.tsx：24 / 12+12 / 8×3 / 6×4。",
		playground(basic))

	// ---------- gutter.tsx ----------
	gutterCell := func(label string, i int) *kit.Col {
		// gutter demo wraps content with padding style — our Col padH handles gutter.
		tx := kit.NewText(label)
		tx.SetFace(c.face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
		inner := primitive.NewDecorated(tx.Node())
		inner.Background = render.Hex("#0092ff")
		inner.Padding = primitive.Symmetric(8, 0)
		inner.ExpandWidth = true
		col := kit.NewCol(inner)
		col.SetSpan(6)
		return col
	}
	gutterH := rowOf(gutterCell("col-6", 0), gutterCell("col-6", 1), gutterCell("col-6", 2), gutterCell("col-6", 3))
	gutterH.SetGutter(16)
	gutterV := rowOf(
		gutterCell("col-6", 0), gutterCell("col-6", 1), gutterCell("col-6", 2), gutterCell("col-6", 3),
		gutterCell("col-6", 4), gutterCell("col-6", 5), gutterCell("col-6", 6), gutterCell("col-6", 7),
	)
	gutterV.SetGutterHV(16, 24)
	gutterBody := primitive.Column(
		func() core.Node {
			d := kit.NewDivider()
			d.SetTitle("Horizontal")
			d.SetTitlePlacement(kit.DividerTitleStart)
			d.SetFace(c.face)
			return d.Node()
		}(),
		gutterH.Node(),
		func() core.Node {
			d := kit.NewDivider()
			d.SetTitle("Vertical")
			d.SetTitlePlacement(kit.DividerTitleStart)
			d.SetFace(c.face)
			return d.Node()
		}(),
		gutterV.Node(),
	)
	gutterBody.Gap = 8
	gutterBody.CrossAlign = core.CrossStretch
	secGutter := demoSection(c.face, c.theme, "区块间隔",
		"antd gutter.tsx：gutter=16 水平；gutter=[16,24] 垂直行距。",
		playground(gutterBody))

	// ---------- offset.tsx ----------
	off1a, off1b := mkCol(8, "col-8", 0), mkCol(8, "col-8", 1)
	off1b.SetOffset(8)
	off2a, off2b := mkCol(6, "col-6 col-offset-6", 0), mkCol(6, "col-6 col-offset-6", 1)
	off2a.SetOffset(6)
	off2b.SetOffset(6)
	off3 := mkCol(12, "col-12 col-offset-6", 0)
	off3.SetOffset(6)
	secOffset := demoSection(c.face, c.theme, "左右偏移",
		"antd offset.tsx：offset 占位左空格。",
		playground(stackRows(
			rowOf(off1a, off1b),
			rowOf(off2a, off2b),
			rowOf(off3),
		)))

	// ---------- sort.tsx (push/pull) ----------
	sA := mkCol(18, "col-18 col-push-6", 0)
	sA.SetPush(6)
	sB := mkCol(6, "col-6 col-pull-18", 1)
	sB.SetPull(18)
	secSort := demoSection(c.face, c.theme, "栅格排序",
		"antd sort.tsx：push / pull 视觉换位。",
		playground(rowOf(sA, sB).Node()))

	// ---------- flex.tsx (justify) ----------
	mkSpan4 := func(i int) *kit.Col { return mkCol(4, "col-4", i) }
	justifyRow := func(j kit.RowJustify, title string) core.Node {
		r := rowOf(mkSpan4(0), mkSpan4(1), mkSpan4(2), mkSpan4(3))
		r.SetJustify(j)
		host := primitive.NewDecorated(r.Node())
		host.Background = render.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 0.08}
		host.ExpandWidth = true
		host.StretchChild = true
		d := kit.NewDivider()
		d.SetTitle(title)
		d.SetTitlePlacement(kit.DividerTitleStart)
		d.SetFace(c.face)
		col := primitive.Column(d.Node(), host)
		col.Gap = 4
		col.CrossAlign = core.CrossStretch
		return col
	}
	flexBody := primitive.Column(
		justifyRow(kit.RowJustifyStart, "sub-element align left"),
		justifyRow(kit.RowJustifyCenter, "sub-element align center"),
		justifyRow(kit.RowJustifyEnd, "sub-element align right"),
		justifyRow(kit.RowJustifySpaceBetween, "sub-element monospaced arrangement"),
		justifyRow(kit.RowJustifySpaceAround, "sub-element align full"),
		justifyRow(kit.RowJustifySpaceEvenly, "sub-element align evenly"),
	)
	flexBody.Gap = 8
	flexBody.CrossAlign = core.CrossStretch
	secFlex := demoSection(c.face, c.theme, "排版",
		"antd flex.tsx：Row.justify start/center/end/space-between/around/evenly。",
		playground(flexBody))

	// ---------- flex-align.tsx ----------
	alignRow := func(a kit.RowAlign, j kit.RowJustify, title string, hs []float64) core.Node {
		cols := make([]*kit.Col, len(hs))
		for i, h := range hs {
			cols[i] = kit.NewCol(mkCell(fmt.Sprintf("col-4"), i, h))
			cols[i].SetSpan(4)
		}
		r := rowOf(cols...)
		r.SetAlign(a)
		r.SetJustify(j)
		host := primitive.NewDecorated(r.Node())
		host.Background = render.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 0.08}
		host.ExpandWidth = true
		host.StretchChild = true
		d := kit.NewDivider()
		d.SetTitle(title)
		d.SetTitlePlacement(kit.DividerTitleStart)
		d.SetFace(c.face)
		col := primitive.Column(d.Node(), host)
		col.Gap = 4
		col.CrossAlign = core.CrossStretch
		return col
	}
	alignBody := primitive.Column(
		alignRow(kit.RowAlignTop, kit.RowJustifyCenter, "Align Top", []float64{100, 50, 120, 80}),
		alignRow(kit.RowAlignMiddle, kit.RowJustifySpaceAround, "Align Middle", []float64{100, 50, 120, 80}),
		alignRow(kit.RowAlignBottom, kit.RowJustifySpaceBetween, "Align Bottom", []float64{100, 50, 120, 80}),
	)
	alignBody.Gap = 8
	alignBody.CrossAlign = core.CrossStretch
	secAlign := demoSection(c.face, c.theme, "对齐",
		"antd flex-align.tsx：align top / middle / bottom。",
		playground(alignBody))

	// ---------- flex-order.tsx ----------
	orderCols := make([]*kit.Col, 4)
	for i := 0; i < 4; i++ {
		orderCols[i] = mkCol(6, fmt.Sprintf("%d col-order-%d", i+1, 4-i), i)
		orderCols[i].SetOrder(4 - i)
	}
	secOrder := demoSection(c.face, c.theme, "排序",
		"antd flex-order.tsx：order 4→1 视觉逆序。",
		playground(rowOf(orderCols...).Node()))

	// ---------- flex-stretch.tsx ----------
	fs1a, fs1b := kit.NewCol(mkCell("2 / 5", 0, 32)), kit.NewCol(mkCell("3 / 5", 1, 32))
	fs1a.SetFlexNumber(2)
	fs1b.SetFlexNumber(3)
	fs2a, fs2b := kit.NewCol(mkCell("100px", 0, 32)), kit.NewCol(mkCell("Fill Rest", 1, 32))
	fs2a.SetFlexString("100px")
	fs2b.SetFlexAuto()
	fs3a, fs3b := kit.NewCol(mkCell("1 1 200px", 0, 32)), kit.NewCol(mkCell("0 1 300px", 1, 32))
	fs3a.SetFlexString("1 1 200px")
	fs3b.SetFlexString("0 1 300px")
	fs4a, fs4b := kit.NewCol(mkCell("none", 0, 32)), kit.NewCol(mkCell("auto with no-wrap", 1, 32))
	fs4a.SetFlexNone()
	fs4b.SetFlexAuto()
	fs4 := rowOf(fs4a, fs4b)
	fs4.SetWrap(false)
	stretchBody := primitive.Column(
		func() core.Node {
			d := kit.NewDivider()
			d.SetTitle("Percentage columns")
			d.SetTitlePlacement(kit.DividerTitleStart)
			d.SetFace(c.face)
			return d.Node()
		}(),
		rowOf(fs1a, fs1b).Node(),
		func() core.Node {
			d := kit.NewDivider()
			d.SetTitle("Fill rest")
			d.SetTitlePlacement(kit.DividerTitleStart)
			d.SetFace(c.face)
			return d.Node()
		}(),
		rowOf(fs2a, fs2b).Node(),
		func() core.Node {
			d := kit.NewDivider()
			d.SetTitle("Raw flex style")
			d.SetTitlePlacement(kit.DividerTitleStart)
			d.SetFace(c.face)
			return d.Node()
		}(),
		rowOf(fs3a, fs3b).Node(),
		fs4.Node(),
	)
	stretchBody.Gap = 8
	stretchBody.CrossAlign = core.CrossStretch
	secStretch := demoSection(c.face, c.theme, "Flex 填充",
		"antd flex-stretch.tsx：flex 数字 / auto / 定宽 / 三值 / wrap=false。",
		playground(stretchBody))

	page := primitive.Column(
		secBasic, secGutter, secOffset, secSort,
		secFlex, secAlign, secOrder, secStretch,
	)
	page.Gap = 16
	page.MainAlign = core.MainStart
	page.CrossAlign = core.CrossStretch
	page.Padding = primitive.All(12)

	c.addPage("grid", "Grid", page)
}
