//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerPagination() {
	// Pagination — antd demos §6.8 P0:
	// basic / align / more / changer / jump / mini / simple / controlled
	// https://ant.design/components/pagination · components/pagination/demo/*.tsx
	//
	// P1 not shown: total / all / itemRender / style-class / wireframe / component-token.

	face, th := c.face, c.theme
	wire := func(p *kit.Pagination, tag string) *kit.Pagination {
		p.SetFace(face)
		p.SetTheme(th)
		p.SetOnChange(func(page, pageSize int) {
			*c.status = fmt.Sprintf("pagination %s → page=%d size=%d", tag, page, pageSize)
		})
		return p
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStretch
		f.MainAlign = core.MainStart
		return f
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewPagination(), "basic")
	basic.SetTotal(50)
	basic.SetDefaultCurrent(1)
	secBasic := demoSection(face, th, "Basic",
		"defaultCurrent=1, total=50 (5 pages @ pageSize=10).",
		basic.Node())

	// ---------- align.tsx ----------
	alignStart := wire(kit.NewPagination(), "align-start")
	alignStart.SetTotal(50)
	alignStart.SetAlign(kit.PaginationAlignStart)
	alignCenter := wire(kit.NewPagination(), "align-center")
	alignCenter.SetTotal(50)
	alignCenter.SetAlign(kit.PaginationAlignCenter)
	alignEnd := wire(kit.NewPagination(), "align-end")
	alignEnd.SetTotal(50)
	alignEnd.SetAlign(kit.PaginationAlignEnd)
	secAlign := demoSection(face, th, "Align",
		"align start / center / end (antd 5.19+).",
		col(alignStart.Node(), alignCenter.Node(), alignEnd.Node()))

	// ---------- more.tsx ----------
	more := wire(kit.NewPagination(), "more")
	more.SetTotal(500)
	more.SetDefaultCurrent(6)
	secMore := demoSection(face, th, "More",
		"Many pages with ellipsis jump (defaultCurrent=6, total=500).",
		more.Node())

	// ---------- changer.tsx ----------
	changer := wire(kit.NewPagination(), "changer")
	changer.SetTotal(500)
	changer.SetDefaultCurrent(3)
	changer.SetShowSizeChanger(true)
	changer.SetOnShowSizeChange(func(current, size int) {
		*c.status = fmt.Sprintf("pagination changer size → current=%d size=%d", current, size)
	})
	changerDis := wire(kit.NewPagination(), "changer-disabled")
	changerDis.SetTotal(500)
	changerDis.SetDefaultCurrent(3)
	changerDis.SetShowSizeChanger(true)
	changerDis.SetDisabled(true)
	secChanger := demoSection(face, th, "Changer",
		"showSizeChanger + onShowSizeChange; second row disabled.",
		col(changer.Node(), changerDis.Node()))

	// ---------- jump.tsx ----------
	jump := wire(kit.NewPagination(), "jump")
	jump.SetTotal(500)
	jump.SetDefaultCurrent(2)
	jump.SetShowQuickJumper(true)
	jumpDis := wire(kit.NewPagination(), "jump-disabled")
	jumpDis.SetTotal(500)
	jumpDis.SetDefaultCurrent(2)
	jumpDis.SetShowQuickJumper(true)
	jumpDis.SetDisabled(true)
	secJump := demoSection(face, th, "Jump",
		"showQuickJumper — type page number and press Enter.",
		col(jump.Node(), jumpDis.Node()))

	// ---------- mini.tsx (size) ----------
	showTotal := func(total, start, end int) string {
		return fmt.Sprintf("Total %d items", total)
	}
	mkSizeRow := func(sz kit.PaginationSize, tag string, disabled bool) core.Node {
		a := wire(kit.NewPagination(), tag+"-a")
		a.SetTotal(50)
		a.SetSize(sz)
		b := wire(kit.NewPagination(), tag+"-b")
		b.SetTotal(50)
		b.SetSize(sz)
		b.SetShowSizeChanger(true)
		b.SetShowQuickJumper(true)
		c2 := wire(kit.NewPagination(), tag+"-c")
		c2.SetTotal(50)
		c2.SetSize(sz)
		c2.SetShowTotal(showTotal)
		d := wire(kit.NewPagination(), tag+"-d")
		d.SetTotal(50)
		d.SetSize(sz)
		d.SetDisabled(disabled)
		d.SetShowTotal(showTotal)
		d.SetShowSizeChanger(true)
		d.SetShowQuickJumper(true)
		return col(a.Node(), b.Node(), c2.Node(), d.Node())
	}
	secSize := demoSection(face, th, "Size",
		"size small / large with sizeChanger, jumper, showTotal, disabled.",
		col(
			kit.NewDivider().Node(),
			mkSizeRow(kit.PaginationSmall, "small", true),
			kit.NewDivider().Node(),
			mkSizeRow(kit.PaginationLarge, "large", true),
		))

	// ---------- simple.tsx ----------
	simple := wire(kit.NewPagination(), "simple")
	simple.SetTotal(50)
	simple.SetDefaultCurrent(2)
	simple.SetSimple(true)
	simpleRO := wire(kit.NewPagination(), "simple-ro")
	simpleRO.SetTotal(50)
	simpleRO.SetDefaultCurrent(2)
	simpleRO.SetSimple(true)
	simpleRO.SetSimpleReadOnly(true)
	simpleDis := wire(kit.NewPagination(), "simple-dis")
	simpleDis.SetTotal(50)
	simpleDis.SetDefaultCurrent(2)
	simpleDis.SetSimple(true)
	simpleDis.SetDisabled(true)
	secSimple := demoSection(face, th, "Simple",
		"simple pager; readOnly current; disabled.",
		col(simple.Node(), simpleRO.Node(), simpleDis.Node()))

	// ---------- controlled.tsx ----------
	controlled := wire(kit.NewPagination(), "controlled")
	controlled.SetTotal(50)
	controlled.SetCurrent(3)
	controlled.SetOnChange(func(page, pageSize int) {
		controlled.SetCurrent(page)
		*c.status = fmt.Sprintf("pagination controlled → page=%d size=%d", page, pageSize)
	})
	secControlled := demoSection(face, th, "Controlled",
		"current + onChange (parent SetCurrent).",
		controlled.Node())

	page := demoPage(face,
		"Pagination",
		"A long list can be divided into several pages using Pagination, and only one page will be loaded at a time.",
		secBasic, secAlign, secMore, secChanger, secJump, secSize, secSimple, secControlled,
	)
	c.addPage("pagination", "Pagination", page)
}
