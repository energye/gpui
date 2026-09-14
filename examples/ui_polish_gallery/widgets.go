// Gallery live widgets: one P0 representative per section per W1 page.
//
// Each entry builds fresh kit nodes so the GPU window draws the real
// component, not placeholder strips. Scene places one node per section;
// selftest layouts every page headless before any window opens.
package main

import (
	"fmt"

	"github.com/energye/gpui/examples/pfkit"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/kit/border-beam"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/kit/divider"
	"github.com/energye/gpui/ui/kit/flex"
	floatbutton "github.com/energye/gpui/ui/kit/float-button"
	"github.com/energye/gpui/ui/kit/grid"
	"github.com/energye/gpui/ui/kit/icon"
	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/kit/masonry"
	"github.com/energye/gpui/ui/kit/progress"
	"github.com/energye/gpui/ui/kit/skeleton"
	"github.com/energye/gpui/ui/kit/space"
	"github.com/energye/gpui/ui/kit/spin"
	"github.com/energye/gpui/ui/kit/splitter"
	"github.com/energye/gpui/ui/kit/statistic"
	"github.com/energye/gpui/ui/kit/tag"
	"github.com/energye/gpui/ui/kit/timeline"
	"github.com/energye/gpui/ui/kit/tooltip"
	"github.com/energye/gpui/ui/kit/popover"
	"github.com/energye/gpui/ui/kit/typography"
	"github.com/energye/gpui/ui/kit/watermark"
	"github.com/energye/gpui/ui/rendering"
)

func box(w, h float64, r, g, b float64) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, r, g, b, 1)
}

// contentForPage builds fresh live nodes, one per section (length matches
// Pages() sections; overview returns nil). Each wrapper is laid out up
// front (loose 912px content width) so tree embedding via Node() already
// sees content size — same contract as Button's intrinsic pre-layout.
// Without this, wrappers that compute size in Layout (alert, tag, ...) Wik
// would place 0x0 nodes.
func contentForPage(name string) []rendering.RenderObject {
	face14 := wrkit.FaceAt(14)
	loose := rendering.Loose(912, 800)
	flexRow := func(nodes ...rendering.RenderObject) rendering.RenderObject {
		f := flex.NewFlex(nodes...)
		f.Layout(loose)
		return f.Node()
	}
	flexCol := func(nodes ...rendering.RenderObject) rendering.RenderObject {
		f := flex.NewFlex(nodes...)
		f.SetVertical(true)
		f.Layout(loose)
		return f.Node()
	}
	mkBtn := func(label string, fn func(b *button.Button)) *button.Button {
		b := button.NewButton(label)
		b.SetTextFace(face14)
		if fn != nil {
			fn(b)
		}
		b.Layout(loose)
		track(b.Node(), &tracked{btn: b, label: label})
		for _, ch := range b.Node().Children() {
			track(ch, &tracked{btn: b, label: label})
		}
		return b
	}
	switch name {
	case "button":
		mk := func(label string, fn func(b *button.Button)) rendering.RenderObject {
			return mkBtn(label, fn).Node()
		}
		return []rendering.RenderObject{
			flexRow(mk("Primary", func(b *button.Button) { b.SetType(button.ButtonPrimary) }), mk("Default", nil), mk("Dashed", func(b *button.Button) { b.SetType(button.ButtonDashed) }), mk("Text", func(b *button.Button) { b.SetType(button.ButtonText) }), mk("Link", func(b *button.Button) { b.SetType(button.ButtonLink) })),
			flexRow(mk("Small", func(b *button.Button) { b.SetSize(button.ButtonSmall) }), mk("Middle", nil), mk("Large", func(b *button.Button) { b.SetSize(button.ButtonLarge) })),
			flexRow(mk("Disabled", func(b *button.Button) { b.SetDisabled(true) }), mk("Disabled", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetDisabled(true) })),
			flexRow(mk("Loading", func(b *button.Button) { b.SetLoading(true) }), mk("Loading", func(b *button.Button) { b.SetLoadingConfig(button.LoadingConfig{Icon: "custom-spin"}) })),
			flexRow(mk("Search", func(b *button.Button) { b.SetIcon("search") }), mk("Search", func(b *button.Button) { b.SetIcon("search"); b.SetIconPlacement(button.IconEnd) })),
			mk("EndIcon", func(b *button.Button) { b.SetIcon("search"); b.SetIconPlacement(button.IconEnd) }),
			flexRow(mk("Cancel", nil), mk("More", nil), mk("Submit", func(b *button.Button) { b.SetType(button.ButtonPrimary) })),
			flexRow(mk("Ghost", func(b *button.Button) { b.SetGhost(true) }), mk("Ghost", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetGhost(true) })),
			flexRow(mk("Danger", func(b *button.Button) { b.SetDanger(true) }), mk("Danger", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetDanger(true) })),
			mk("Block Button", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetBlock(true) }),
			flexRow(mk("Solid", func(b *button.Button) { b.SetColor(button.ColorPrimary); b.SetVariant(button.VariantSolid) }), mk("Outlined", func(b *button.Button) { b.SetColor(button.ColorPrimary); b.SetVariant(button.VariantOutlined) }), mk("Dashed", func(b *button.Button) { b.SetVariant(button.VariantDashed) }), mk("Filled", func(b *button.Button) { b.SetColor(button.ColorPrimary); b.SetVariant(button.VariantFilled) }), mk("Text", func(b *button.Button) { b.SetVariant(button.VariantText) }), mk("Link", func(b *button.Button) { b.SetVariant(button.VariantLink) })),
		}
	case "alert":
		mk := func(title string, fn func(a *alert.Alert)) *alert.Alert {
			a := alert.NewAlert(title)
			a.SetTextFace(face14)
			if fn != nil {
				fn(a)
			}
			a.Layout(loose)
			track(a.Node(), &tracked{al: a, label: title})
			return a
		}
		// Gallery sections: Basic / FourTypes / Filled / Closable /
		// Description / Icon / Banner / LoopBanner / Action. FourTypes packs
		// all four semantic types; Action packs single + double real buttons
		// so the window shows complete actual components, not placeholders.
		basic := mk("Success Text", func(a *alert.Alert) { a.SetType(alert.AlertSuccess) })
		f1 := mk("Success", func(a *alert.Alert) { a.SetType(alert.AlertSuccess); a.SetShowIcon(true) })
		f2 := mk("Info", func(a *alert.Alert) { a.SetType(alert.AlertInfo); a.SetShowIcon(true) })
		f3 := mk("Warning", func(a *alert.Alert) { a.SetType(alert.AlertWarning); a.SetShowIcon(true) })
		f4 := mk("Error", func(a *alert.Alert) { a.SetType(alert.AlertError); a.SetShowIcon(true) })
		fourTypes := flexCol(f1.Node(), f2.Node(), f3.Node(), f4.Node())
		filled := mk("Filled Info", func(a *alert.Alert) { a.SetType(alert.AlertInfo); a.SetVariant(alert.AlertFilled); a.SetShowIcon(true) })
		closable := mk("Closable Warning", func(a *alert.Alert) { a.SetType(alert.AlertWarning); a.SetShowIcon(true); a.SetClosable(true) })
		desc := mk("Success Title", func(a *alert.Alert) { a.SetType(alert.AlertSuccess); a.SetDescription("Detailed description with two lines"); a.SetShowIcon(true) })
		iconRow := mk("Icon Info", func(a *alert.Alert) { a.SetType(alert.AlertInfo); a.SetShowIcon(true); a.SetDescription("Helper with icon"); a.SetClosable(true) })
		banner := mk("Banner Announcement", func(a *alert.Alert) { a.SetBanner(true) })
		loopText := rendering.NewRenderText("Very long scrolling announcement for loop-banner marquee slot")
		loopText.FontSize = 14
		loopText.R, loopText.G, loopText.B, loopText.A = 0, 0, 0, 0.88
		loopText.SetFace(face14)
		loop := mk("loop", func(a *alert.Alert) { a.SetBanner(true); a.SetTitleNode(loopText) })
		singleAct := mkBtn("UNDO", nil)
		single := mk("Action Single", func(a *alert.Alert) { a.SetType(alert.AlertInfo); a.SetShowIcon(true); a.SetAction(singleAct.Node()) })
		accept := mkBtn("Accept", nil)
		decline := mkBtn("Decline", nil)
		pair := flex.NewFlex(accept.Node(), decline.Node())
		pair.SetVertical(true)
		pair.Layout(loose)
		double := mk("Action Double", func(a *alert.Alert) { a.SetType(alert.AlertWarning); a.SetShowIcon(true); a.SetDescription("With double-row action"); a.SetAction(pair.Node()) })
		actionCol := flexCol(single.Node(), double.Node())
		return []rendering.RenderObject{
			basic.Node(),
			fourTypes,
			filled.Node(),
			closable.Node(),
			desc.Node(),
			iconRow.Node(),
			banner.Node(),
			loop.Node(),
			actionCol,
		}
	case "border-beam":
		mkBeam := func(fn func(b *border_beam.BorderBeam)) rendering.RenderObject {
			b := border_beam.NewBorderBeam(box(160, 36, 0.97, 0.97, 0.97))
			if fn != nil {
				fn(b)
			}
			b.Layout(loose)
			return b.Node()
		}
		return []rendering.RenderObject{
			mkBeam(nil),
			mkBeam(func(b *border_beam.BorderBeam) { b.SetShowOnHover(true) }),
			mkBeam(func(b *border_beam.BorderBeam) { b.SetBorderRadius(8) }),
			mkBeam(func(b *border_beam.BorderBeam) {
				b.SetColorStops(
					border_beam.BorderBeamColorStop{Color: render.RGBA{R: 1, G: 0, B: 0, A: 1}, Percent: 0},
					border_beam.BorderBeamColorStop{Color: render.RGBA{R: 0, G: 0, B: 1, A: 1}, Percent: 1},
				)
			}),
			flexCol(mkBeam(func(b *border_beam.BorderBeam) { b.SetDuration(3) }), mkBeam(func(b *border_beam.BorderBeam) { b.SetDuration(12) })),
			flexCol(mkBeam(func(b *border_beam.BorderBeam) { b.SetSize(56) }), mkBeam(func(b *border_beam.BorderBeam) { b.SetSize(160) })),
			mkBeam(func(b *border_beam.BorderBeam) { b.SetLineWidth(2) }),
		}
	case "divider":
		mk := func(title string, fn func(d *divider.Divider)) rendering.RenderObject {
			var d *divider.Divider
			if title == "" {
				d = divider.NewDivider()
			} else {
				d = divider.NewDividerWithTitle(title)
			}
			d.SetFace(face14)
			if fn != nil {
				fn(d)
			}
			d.Layout(loose)
			return d.Node()
		}
		mkV := func(fn func(d *divider.Divider)) rendering.RenderObject {
			d := divider.NewDivider()
			if fn != nil {
				fn(d)
			}
			d.Layout(loose)
			return d.Node()
		}
		return []rendering.RenderObject{
			flexCol(mk("", nil), mk("", func(d *divider.Divider) { d.SetDashed(true) })),
			flexCol(mk("Center", nil), mk("Start", func(d *divider.Divider) { d.SetTitlePlacement(divider.Start) }), mk("End", func(d *divider.Divider) { d.SetTitlePlacement(divider.End) })),
			flexCol(mk("Small", func(d *divider.Divider) { d.SetSize(divider.Small) }), mk("Medium", func(d *divider.Divider) { d.SetSize(divider.Medium) }), mk("Large", func(d *divider.Divider) { d.SetSize(divider.Large) })),
			mk("Plain Body", func(d *divider.Divider) { d.SetPlain(true) }),
			flexRow(mkV(nil), mkV(func(d *divider.Divider) { d.SetVertical(true) }), mkV(func(d *divider.Divider) { d.SetVertical(true); d.SetDashed(true) })),
			flexCol(mk("", nil), mk("", func(d *divider.Divider) { d.SetDashed(true) }), mk("", func(d *divider.Divider) { d.SetVariant(divider.Dotted) })),
			mk("Style", nil),
		}
	case "flex":
		mkFlex := func(fn func(f *flex.Flex)) rendering.RenderObject {
			f := flex.NewFlex(box(60, 24, 0.3, 0.4, 0.8), box(60, 24, 0.8, 0.3, 0.3))
			if fn != nil {
				fn(f)
			}
			f.Layout(loose)
			return f.Node()
		}
		return []rendering.RenderObject{
			mkFlex(nil),
			flexCol(mkFlex(func(f *flex.Flex) { f.SetJustify(flex.FlexJustifyCenter); f.SetAlign(flex.FlexAlignCenter) }), mkFlex(func(f *flex.Flex) { f.SetJustify(flex.FlexJustifySpaceBetween) })),
			flexCol(mkFlex(func(f *flex.Flex) { f.SetGap(8) }), mkFlex(func(f *flex.Flex) { f.SetGap(16) }), mkFlex(func(f *flex.Flex) { f.SetGap(24) })),
			mkFlex(func(f *flex.Flex) { f.SetWrap(true) }),
			mkFlex(func(f *flex.Flex) { f.SetVertical(true); f.SetJustify(flex.FlexJustifyCenter); f.SetAlign(flex.FlexAlignCenter) }),
		}
	case "float-button":
		mk := func(fn func(b *floatbutton.FloatButton)) rendering.RenderObject {
			b := floatbutton.NewFloatButton()
			b.SetTextFace(face14)
			if fn != nil {
				fn(b)
			}
			b.Layout(loose)
			track(b.Node(), &tracked{fbtn: b, label: b.Content() + "/" + b.Tooltip()})
			return b.Node()
		}
		mkGroup := func(fns ...func(b *floatbutton.FloatButton)) rendering.RenderObject {
			var kids []*floatbutton.FloatButton
			for _, fn := range fns {
				b := floatbutton.NewFloatButton()
				b.SetTextFace(face14)
				if fn != nil {
					fn(b)
				}
				b.Layout(loose)
				kids = append(kids, b)
			}
			g := floatbutton.NewFloatButtonGroup(kids...)
			g.Layout(loose)
			return g.Node()
		}
		return []rendering.RenderObject{
			mk(nil),
			flexRow(mk(nil), mk(func(b *floatbutton.FloatButton) { b.SetType(floatbutton.ButtonTypePrimary) })),
			flexRow(mk(nil), mk(func(b *floatbutton.FloatButton) { b.SetShape(floatbutton.FloatButtonShapeSquare) })),
			flexRow(mk(func(b *floatbutton.FloatButton) { b.SetContent("Help") }), mk(func(b *floatbutton.FloatButton) { b.SetType(floatbutton.ButtonTypePrimary); b.SetContent("Help") })),
			flexRow(mk(func(b *floatbutton.FloatButton) { b.SetTooltip("Tip") }), mk(func(b *floatbutton.FloatButton) { b.SetTooltip("Tip"); b.SetDisabled(true) }), mk(func(b *floatbutton.FloatButton) { b.SetTooltip("Tip"); b.SetLoading(true) })),
			mkGroup(nil, nil),
			mkGroup(func(b *floatbutton.FloatButton) { b.SetTooltip("Menu") }, nil),
			mkGroup(nil, nil),
			mkGroup(nil, nil),
		}
	case "grid":
		mkRow := func(cols []*grid.Col, fn func(r *grid.Row)) rendering.RenderObject {
			r := grid.NewRow(cols...)
			if fn != nil {
				fn(r)
			}
			r.Layout(loose)
			return r.Node()
		}
		mkCol := func(w, h float64, r, g, b float64, fn func(c *grid.Col)) *grid.Col {
			c := grid.NewCol(box(w, h, r, g, b))
			if fn != nil {
				fn(c)
			}
			return c
		}
		return []rendering.RenderObject{
			mkRow([]*grid.Col{mkCol(120, 24, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(12) }), mkCol(120, 24, 0.4, 0.3, 0.2, func(c *grid.Col) { c.SetSpan(12) })}, nil),
			mkRow([]*grid.Col{mkCol(120, 24, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(8) }), mkCol(120, 24, 0.4, 0.3, 0.2, func(c *grid.Col) { c.SetSpan(8) })}, func(r *grid.Row) { r.SetGutter(16) }),
			mkRow([]*grid.Col{mkCol(120, 24, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(6); c.SetOffset(6) })}, nil),
			mkRow([]*grid.Col{mkCol(120, 24, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(8); c.SetPush(8) }), mkCol(120, 24, 0.4, 0.3, 0.2, func(c *grid.Col) { c.SetSpan(8); c.SetPull(8) })}, nil),
			mkRow([]*grid.Col{mkCol(120, 24, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(6) }), mkCol(120, 24, 0.4, 0.3, 0.2, func(c *grid.Col) { c.SetFlexAuto() })}, func(r *grid.Row) { r.SetJustify(grid.RowJustifyCenter) }),
			mkRow([]*grid.Col{mkCol(120, 16, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(6) }), mkCol(120, 32, 0.4, 0.3, 0.2, func(c *grid.Col) { c.SetSpan(6) })}, func(r *grid.Row) { r.SetAlign(grid.RowAlignMiddle) }),
			mkRow([]*grid.Col{mkCol(120, 24, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(6); c.SetOrder(2) }), mkCol(120, 24, 0.4, 0.3, 0.2, func(c *grid.Col) { c.SetSpan(6); c.SetOrder(1) })}, nil),
			mkRow([]*grid.Col{mkCol(120, 24, 0.2, 0.3, 0.4, func(c *grid.Col) { c.SetSpan(6) }), mkCol(60, 24, 0.4, 0.3, 0.2, func(c *grid.Col) { c.SetFlexNumber(1) })}, nil),
		}
	case "layout":
		mkLay := func(sections ...layout.LayoutSection) rendering.RenderObject {
			l := layout.NewLayout(sections...)
			l.Layout(loose)
			return l.Node()
		}
		basicH := layout.NewHeader(box(300, 12, 0.3, 0.4, 0.8))
		basicC := layout.NewContent(box(300, 28, 0.95, 0.95, 0.95))
		basicF := layout.NewFooter(box(300, 12, 0.8, 0.8, 0.8))
		topH := layout.NewHeader(box(300, 12, 0.3, 0.4, 0.8))
		topC := layout.NewContent(box(300, 28, 0.95, 0.95, 0.95))
		topF := layout.NewFooter(box(300, 12, 0.8, 0.8, 0.8))
		sideS := layout.NewSider(box(80, 40, 0.2, 0.3, 0.4))
		sideC := layout.NewContent(box(220, 40, 0.95, 0.95, 0.95))
		side2H := layout.NewHeader(box(300, 12, 0.3, 0.4, 0.8))
		side2S := layout.NewSider(box(80, 40, 0.2, 0.3, 0.4))
		side2C := layout.NewContent(box(220, 40, 0.95, 0.95, 0.95))
		colS := layout.NewSider(box(80, 40, 0.2, 0.3, 0.4))
		colS.SetCollapsible(true)
		colS.SetCollapsed(true)
		colS.Layout(loose)
		track(colS.Node(), &tracked{sider: colS, label: "side-collapsed"})
		colC := layout.NewContent(box(220, 40, 0.95, 0.95, 0.95))
		trigS := layout.NewSider(box(80, 40, 0.2, 0.3, 0.4))
		trigS.SetCollapsible(true)
		trigS.SetTrigger(box(40, 12, 0.9, 0.6, 0.1))
		trigS.Layout(loose)
		track(trigS.Node(), &tracked{sider: trigS, label: "side-trigger"})
		trigC := layout.NewContent(box(220, 40, 0.95, 0.95, 0.95))
		ovS := layout.NewSider(box(80, 40, 0.2, 0.3, 0.4))
		ovS.SetCollapsed(true)
		ovS.SetCollapsedWidth(0)
		ovC := layout.NewContent(box(300, 40, 0.95, 0.95, 0.95))
		respS := layout.NewSider(box(80, 40, 0.2, 0.3, 0.4))
		respS.SetBreakpoint(layout.LayoutBreakpointMD)
		respC := layout.NewContent(box(220, 40, 0.95, 0.95, 0.95))
		return []rendering.RenderObject{
			mkLay(basicH, basicC, basicF),
			mkLay(topH, topC, topF),
			mkLay(topH, layout.NewLayout(sideS, sideC)),
			mkLay(side2H, layout.NewLayout(side2S, side2C)),
			mkLay(colS, colC),
			mkLay(trigS, trigC),
			mkLay(ovS, ovC),
			mkLay(respS, respC),
		}
	case "masonry":
		mkMas := func(fn func(m *masonry.Masonry)) rendering.RenderObject {
			m := masonry.NewMasonry(
				masonry.MasonryItem{Key: "a", Content: box(80, 28, 0.3, 0.4, 0.8)},
				masonry.MasonryItem{Key: "b", Content: box(80, 40, 0.8, 0.3, 0.3)},
				masonry.MasonryItem{Key: "c", Content: box(80, 56, 0.2, 0.6, 0.3)},
				masonry.MasonryItem{Key: "d", Content: box(80, 20, 0.9, 0.6, 0.1)},
				masonry.MasonryItem{Key: "e", Content: box(80, 48, 0.5, 0.3, 0.7)},
				masonry.MasonryItem{Key: "f", Content: box(80, 32, 0.2, 0.7, 0.7)},
			)
			if fn != nil {
				fn(m)
			}
			m.Layout(loose)
			return m.Node()
		}
		return []rendering.RenderObject{
			mkMas(nil),
			mkMas(func(m *masonry.Masonry) { m.SetViewportWidth(500) }),
			mkMas(func(m *masonry.Masonry) { m.SetGutter(8, 8) }),
			mkMas(func(m *masonry.Masonry) { m.SetColumns(2) }),
		}
	case "progress":
		mk := func(p float64, fn func(pr *progress.Progress)) rendering.RenderObject {
			pr := progress.NewProgress(p)
			pr.SetTextFace(face14)
			if fn != nil {
				fn(pr)
			}
			pr.Layout(loose)
			return pr.Node()
		}
		return []rendering.RenderObject{
			flexCol(mk(30, nil), mk(50, nil), mk(70, func(pr *progress.Progress) { pr.SetStatus(progress.StatusException) }), mk(100, nil)),
			flexCol(mk(75, func(pr *progress.Progress) { pr.SetType(progress.TypeCircle) }), mk(70, func(pr *progress.Progress) { pr.SetType(progress.TypeCircle); pr.SetStatus(progress.StatusException) })),
			mk(30, func(pr *progress.Progress) { pr.SetSize(progress.SizeSmall) }),
			flexCol(mk(30, func(pr *progress.Progress) { pr.SetType(progress.TypeCircle); pr.SetSize(progress.SizeSmall) }), mk(60, func(pr *progress.Progress) { pr.SetType(progress.TypeCircle); pr.SetSize(progress.SizeSmall) })),
			flexCol(mk(30, func(pr *progress.Progress) { pr.SetType(progress.TypeCircle) }), mk(60, func(pr *progress.Progress) { pr.SetType(progress.TypeCircle) }), mk(90, func(pr *progress.Progress) { pr.SetType(progress.TypeCircle) })),
			mk(50, nil),
			mk(40, func(pr *progress.Progress) { pr.SetFormat(func(p, s float64) string { return "40%" }) }),
			mk(70, func(pr *progress.Progress) { pr.SetType(progress.TypeDashboard) }),
		}
	case "skeleton":
		mk := func(fn func(s *skeleton.Skeleton)) rendering.RenderObject {
			s := skeleton.NewSkeleton()
			if fn != nil {
				fn(s)
			}
			s.Layout(loose)
			return s.Node()
		}
		return []rendering.RenderObject{
			mk(nil),
			mk(func(s *skeleton.Skeleton) { s.SetAvatar(true) }),
			mk(func(s *skeleton.Skeleton) { s.SetActive(true) }),
			flexCol(mk(func(s *skeleton.Skeleton) { s.SetAvatar(true) }), mk(func(s *skeleton.Skeleton) { s.SetAvatar(false); s.SetTitle(false) })),
			mk(func(s *skeleton.Skeleton) { s.SetLoading(false); s.SetContent(box(200, 24, 0.9, 0.9, 0.9)) }),
			mk(func(s *skeleton.Skeleton) { s.SetParagraphRows(4) }),
			mk(nil),
			mk(nil),
		}
	case "space":
		mkSp := func(fn func(s *space.Space)) rendering.RenderObject {
			s := space.NewSpace(box(50, 20, 0.3, 0.4, 0.8), box(50, 20, 0.8, 0.3, 0.3), box(50, 20, 0.2, 0.6, 0.3))
			if fn != nil {
				fn(s)
			}
			s.Layout(loose)
			return s.Node()
		}
		return []rendering.RenderObject{
			mkSp(nil),
			mkSp(func(s *space.Space) { s.SetVertical(true) }),
			flexCol(mkSp(func(s *space.Space) { s.SetSize(space.SpaceSizeSmall) }), mkSp(func(s *space.Space) { s.SetSize(space.SpaceSizeLarge) })),
			mkSp(func(s *space.Space) { s.SetAlign(space.SpaceAlignCenter) }),
			mkSp(func(s *space.Space) { s.SetWrap(true) }),
			mkSp(func(s *space.Space) { s.SetSeparator(func() rendering.RenderObject { return box(2, 20, 0.6, 0.6, 0.6) }) }),
			func() rendering.RenderObject {
				c := space.NewSpaceCompact(box(60, 24, 0.3, 0.4, 0.8), box(60, 24, 0.8, 0.3, 0.3))
				c.Layout(loose)
				return c.Node()
			}(),
			func() rendering.RenderObject {
				c := space.NewSpaceCompact(box(60, 24, 0.3, 0.4, 0.8), box(60, 24, 0.8, 0.3, 0.3), box(60, 24, 0.2, 0.6, 0.3))
				c.Layout(loose)
				return c.Node()
			}(),
		}
	case "spin":
		mk := func(fn func(s *spin.Spin)) rendering.RenderObject {
			s := spin.NewSpin(nil)
			s.SetTextFace(face14)
			if fn != nil {
				fn(s)
			}
			s.Layout(loose)
			return s.Node()
		}
		return []rendering.RenderObject{
			mk(nil),
			flexRow(mk(func(s *spin.Spin) { s.SetSize(spin.SpinSmall) }), mk(nil), mk(func(s *spin.Spin) { s.SetSize(spin.SpinLarge) })),
			mk(func(s *spin.Spin) { s.SetContent(box(200, 40, 0.9, 0.9, 0.9)) }),
			flexRow(mk(func(s *spin.Spin) { s.SetDescription("Loading...") }), mk(func(s *spin.Spin) { s.SetSize(spin.SpinLarge); s.SetDescription("tip text") })),
			mk(func(s *spin.Spin) { s.SetDelay(500); s.SetDescription("Loading...") }),
			mk(func(s *spin.Spin) { s.SetDescription("Custom") }),
			mk(func(s *spin.Spin) { s.SetPercent(50) }),
			mk(func(s *spin.Spin) { s.SetDescription("Styled") }),
		}
	case "splitter":
		mkSplit := func(fn func(s *splitter.Splitter)) rendering.RenderObject {
			sp := splitter.NewSplitter(
				splitter.NewSplitterPanel(box(120, 32, 0.9, 0.2, 0.2)),
				splitter.NewSplitterPanel(box(120, 32, 0.2, 0.4, 0.9)),
			)
			if fn != nil {
				fn(sp)
			}
			sp.Layout(loose)
			track(sp.Node(), &tracked{split: sp, label: "split"})
			return sp.Node()
		}
		return []rendering.RenderObject{
			mkSplit(nil),
			mkSplit(func(s *splitter.Splitter) { s.SetPanelSizesPx([]float64{100, 200}) }),
			mkSplit(func(s *splitter.Splitter) { s.SetOrientation(splitter.Vertical) }),
			func() rendering.RenderObject {
				p1 := splitter.NewSplitterPanel(box(120, 32, 0.9, 0.2, 0.2))
				p1.SetCollapsible(true)
				p2 := splitter.NewSplitterPanel(box(120, 32, 0.2, 0.4, 0.9))
				p2.SetCollapsible(true)
				sp := splitter.NewSplitter(p1, p2)
				sp.Layout(loose)
				return sp.Node()
			}(),
			func() rendering.RenderObject {
				p1 := splitter.NewSplitterPanel(box(120, 32, 0.9, 0.2, 0.2))
				p1.SetCollapsible(true)
				p2 := splitter.NewSplitterPanel(box(120, 32, 0.2, 0.4, 0.9))
				p2.SetCollapsible(true)
				sp := splitter.NewSplitter(p1, p2)
				sp.SetCollapsibleIcons(box(12, 12, 0.2, 0.2, 0.2), box(12, 12, 0.2, 0.2, 0.2))
				sp.Layout(loose)
				return sp.Node()
			}(),
			mkSplit(nil),
			func() rendering.RenderObject {
				inner := splitter.NewSplitter(
					splitter.NewSplitterPanel(box(60, 32, 0.2, 0.6, 0.3)),
					splitter.NewSplitterPanel(box(60, 32, 0.9, 0.6, 0.1)),
				)
				inner.Layout(loose)
				sp := splitter.NewSplitter(
					splitter.NewSplitterPanel(box(120, 32, 0.9, 0.2, 0.2)),
					splitter.NewSplitterPanel(inner.Node()),
				)
				sp.Layout(loose)
				return sp.Node()
			}(),
			mkSplit(func(s *splitter.Splitter) { s.SetLazy(true) }),
		}
	case "statistic":
		mk := func(title string, v any, fn func(s *statistic.Statistic)) rendering.RenderObject {
			s := statistic.NewStatistic()
			s.SetTitle(title)
			s.SetValue(v)
			s.SetTextFace(face14)
			if fn != nil {
				fn(s)
			}
			s.Layout(loose)
			return s.Node()
		}
		return []rendering.RenderObject{
			flexCol(mk("Active Users", 112893, nil), mk("Account Balance (CNY)", 112893.0, func(s *statistic.Statistic) { s.SetPrecision(2) })),
			flexCol(mk("Feedback", 1128, func(s *statistic.Statistic) { s.SetPrefix("like"); s.SetSuffix(""); _ = s }), mk("Unmerged", "93 / 100", nil)),
			mk("Animated", 112893, func(s *statistic.Statistic) { s.SetFormatter(func(v any) string { return "112,893" }) }),
			flexCol(mk("Active", "11.28", func(s *statistic.Statistic) { s.SetPrefix("up"); s.SetSuffix("%") }), mk("Idle", "9.30", func(s *statistic.Statistic) { s.SetPrefix("down"); s.SetSuffix("%") })),
			flexCol(mk("Countdown", "01:01:01", nil), mk("Countup", "01:01", nil)),
			mk("Monthly Active Users", "93,241", func(s *statistic.Statistic) { s.SetPrefix("up"); s.SetSuffix("users") }),
		}
	case "tag":
		mk := func(label string, fn func(t *tag.Tag)) rendering.RenderObject {
			t := tag.NewTag(label)
			t.SetFace(face14)
			if fn != nil {
				fn(t)
			}
			t.Layout(loose)
			track(t.Node(), &tracked{tg: t, label: label})
			return t.Node()
		}
		mkCheck := func(label string, checked bool) rendering.RenderObject {
			c := tag.NewCheckableTag(label)
			c.SetFace(face14)
			c.SetChecked(checked)
			c.Layout(loose)
			track(c.Node(), &tracked{ctg: c, label: label})
			return c.Node()
		}
		return []rendering.RenderObject{
			flexRow(mk("Tag 1", nil), mk("Tag 2", nil), mk("Link", nil)),
			flexRow(mk("red", func(t *tag.Tag) { t.SetColor("red") }), mk("blue", func(t *tag.Tag) { t.SetColor("blue") }), mk("green", func(t *tag.Tag) { t.SetColor("green") }), mk("orange", func(t *tag.Tag) { t.SetColor("orange") }), mk("red solid", func(t *tag.Tag) { t.SetColor("red solid") }), mk("#f50", func(t *tag.Tag) { t.SetColor("#f50") })),
			flexRow(mk("Deletable", func(t *tag.Tag) { t.SetClosable(true) }), mk("Red", func(t *tag.Tag) { t.SetColor("red"); t.SetClosable(true) })),
			flexRow(mkCheck("Unchecked", false), mkCheck("Checked", true), mkCheck("Movies", false), mkCheck("Books", true)),
			flexRow(mk("Anim", nil), mk("Anim2", func(t *tag.Tag) { t.SetClosable(true) })),
			flexRow(mk("Icon tag", func(t *tag.Tag) { t.SetIcon("star") }), mk("Color icon", func(t *tag.Tag) { t.SetColor("blue"); t.SetIcon("star") })),
			flexRow(mk("success", func(t *tag.Tag) { t.SetColor("success") }), mk("processing", func(t *tag.Tag) { t.SetColor("processing") }), mk("error", func(t *tag.Tag) { t.SetColor("error") }), mk("warning", func(t *tag.Tag) { t.SetColor("warning") })),
			flexRow(mk("Apple", nil), mk("Pear", nil), mk("Orange", func(t *tag.Tag) { t.SetColor("orange") })),
		}
	case "timeline":
		mkTl := func(items []timeline.TimelineItem, fn func(t *timeline.Timeline)) rendering.RenderObject {
			t := timeline.NewTimeline(items...)
			t.SetFace(face14)
			if fn != nil {
				fn(t)
			}
			t.Layout(loose)
			return t.Node()
		}
		base4 := []timeline.TimelineItem{{Content: "Create services"}, {Content: "Solve issues"}, {Content: "Technical testing"}, {Content: "Network solved"}}
		return []rendering.RenderObject{
			mkTl(base4, nil),
			mkTl([]timeline.TimelineItem{{Content: "Outlined dot"}, {Content: "Outlined second"}}, func(t *timeline.Timeline) { t.SetVariant(timeline.TimelineOutlined) }),
			mkTl([]timeline.TimelineItem{{Content: "Recording", Loading: true}, {Content: "Pending item"}}, func(t *timeline.Timeline) { t.SetReverse(true) }),
			mkTl(base4, func(t *timeline.Timeline) { t.SetMode(timeline.TimelineModeAlternate) }),
			mkTl([]timeline.TimelineItem{{Content: "Step one"}, {Content: "Step two"}, {Content: "Step three"}}, func(t *timeline.Timeline) { t.SetOrientation(timeline.TimelineHorizontal) }),
			mkTl([]timeline.TimelineItem{{Content: "Named icon dot", Icon: "star"}, {Content: "Plain dot"}}, nil),
			mkTl(base4, func(t *timeline.Timeline) { t.SetMode(timeline.TimelineModeEnd) }),
			mkTl([]timeline.TimelineItem{{Title: "2015-09-01", Content: "Create services site"}, {Title: "2015-09-02", Content: "Solve network problems"}}, nil),
		}
	case "typography":
		mk := func(tp *typography.Typography) rendering.RenderObject {
			tp.SetFace(face14)
			tp.Layout(loose)
			track(tp.Node(), &tracked{typo: tp, label: tp.Value()})
			return tp.Node()
		}
		mkFn := func(tp *typography.Typography, fn func(t *typography.Typography)) rendering.RenderObject {
			tp.SetFace(face14)
			if fn != nil {
				fn(tp)
			}
			tp.Layout(loose)
			track(tp.Node(), &tracked{typo: tp, label: tp.Value()})
			return tp.Node()
		}
		return []rendering.RenderObject{
			flexCol(mk(typography.NewText("Ant Design Text basic")), mk(typography.NewTitle("Typography Title", 1)), mk(typography.NewParagraph("Paragraph line for layout and paint")), mk(typography.NewLink("Ant Design Link"))),
			flexCol(mk(typography.NewTitle("Typography Title", 1)), mk(typography.NewTitle("Typography Title", 2)), mk(typography.NewTitle("Typography Title", 3)), mk(typography.NewTitle("Typography Title", 4)), mk(typography.NewTitle("Typography Title", 5))),
			flexCol(mkFn(typography.NewText("secondary text"), func(t *typography.Typography) { t.SetType(typography.TypeSecondary) }), mkFn(typography.NewText("success text"), func(t *typography.Typography) { t.SetType(typography.TypeSuccess) }), mkFn(typography.NewText("warning text"), func(t *typography.Typography) { t.SetType(typography.TypeWarning) }), mkFn(typography.NewText("danger text"), func(t *typography.Typography) { t.SetType(typography.TypeDanger) }), mk(typography.NewLink("Ant Design Link"))),
			mkFn(typography.NewText("Editable old text"), func(t *typography.Typography) { t.SetEditable(true) }),
			mkFn(typography.NewText("Copy me please"), func(t *typography.Typography) { t.SetCopyable(true) }),
			mk(typography.NewText("Ellipsis long text that should truncate in narrow rows.")),
			mk(typography.NewText("Controlled ellipsis full text.")),
			mk(typography.NewText("Middle ellipsis head ... tail suffix")),
		}
	case "icon":
		mkIcon := func(name string) rendering.RenderObject {
			ic := icon.NewIcon(name)
			ic.Layout(loose)
			return ic.Node()
		}
		return []rendering.RenderObject{
			mkIcon("check"),
			mkIcon("star"),
			mkIcon("home"),
			mkIcon("setting"),
			mkIcon("search"),
		}
	case "watermark":
		mk := func(content string, fn func(w *watermark.Watermark)) rendering.RenderObject {
			w := watermark.NewWatermark(box(280, 48, 0.96, 0.96, 0.96))
			w.SetContent(content)
			w.SetFace(face14)
			if fn != nil {
				fn(w)
			}
			w.Layout(loose)
			return w.Node()
		}
		return []rendering.RenderObject{
			mk("Ant Design", nil),
			mk("Ant Design", func(w *watermark.Watermark) { w.SetContentStrings("Ant Design", "Happy Working") }),
			mk("Image mark", nil),
			mk("Custom", func(w *watermark.Watermark) { w.SetRotate(-15); w.SetFontSize(18) }),
			mk("Modal", nil),
		}
	case "tooltip":
		mkTip := func(title, trigger string, fn func(t *tooltip.Tooltip)) *tooltip.Tooltip {
			t := tooltip.NewTooltip(title)
			t.SetTriggerLabel(trigger)
			t.SetTextFace(face14)
			// Gallery is instant: no ticker drives delays in the window.
			t.SetMouseEnterDelay(0)
			t.SetMouseLeaveDelay(0)
			if fn != nil {
				fn(t)
			}
			t.Layout(loose)
			track(t.Node(), &tracked{tip: t, label: trigger + "/" + title})
			return t
		}
		return []rendering.RenderObject{
			mkTip("prompt text", "Hover me", nil).Node(),
			flexRow(mkTip("hover tip", "Hover", nil).Node(), mkTip("focus tip", "Focus", func(t *tooltip.Tooltip) { t.SetTrigger(tooltip.TriggerFocus) }).Node(), mkTip("click tip", "Click", func(t *tooltip.Tooltip) { t.SetTrigger(tooltip.TriggerClick) }).Node()),
			flexRow(mkTip("tip top", "top", nil).Node(), mkTip("tip bottom", "bottom", func(t *tooltip.Tooltip) { t.SetPlacement(tooltip.Bottom) }).Node(), mkTip("tip left", "left", func(t *tooltip.Tooltip) { t.SetPlacement(tooltip.Left) }).Node(), mkTip("tip right", "right", func(t *tooltip.Tooltip) { t.SetPlacement(tooltip.Right) }).Node()),
			flexRow(mkTip("arrow on", "Arrow", nil).Node(), mkTip("no arrow", "NoArrow", func(t *tooltip.Tooltip) { t.SetArrow(false) }).Node(), mkTip("center arrow", "Center", func(t *tooltip.Tooltip) { t.SetArrowConfig(true, true) }).Node()),
			mkTip("edge tip", "Edge", func(t *tooltip.Tooltip) { t.SetAutoAdjustOverflow(true) }).Node(),
			flexRow(mkTip("red tip", "red", func(t *tooltip.Tooltip) { t.SetColor("red") }).Node(), mkTip("green tip", "green", func(t *tooltip.Tooltip) { t.SetColor("green") }).Node(), mkTip("hex tip", "#ff5500", func(t *tooltip.Tooltip) { t.SetColor("#ff5500") }).Node()),
			flexRow(mkTip("disabled tip", "Disabled", func(t *tooltip.Tooltip) { t.SetDisabled(true) }).Node(), mkTip("", "Empty title", nil).Node()),
			mkTip("custom title node", "Custom", func(t *tooltip.Tooltip) { t.SetTriggerNode(box(60, 24, 0.3, 0.4, 0.8)) }).Node(),
		}
	case "popover":
		mkPop := func(title, content, trigger string, fn func(p *popover.Popover)) *popover.Popover {
			p := popover.NewPopover(trigger)
			p.SetTitle(title)
			p.SetContent(content)
			p.SetTextFace(face14)
			if fn != nil {
				fn(p)
			}
			p.Layout(loose)
			track(p.Node(), &tracked{pop: p, label: trigger + "/" + title})
			return p
		}
		return []rendering.RenderObject{
			mkPop("Title", "Content here", "Hover me", nil).Node(),
			flexRow(mkPop("Hover title", "Hover content", "Hover", nil).Node(), mkPop("Focus title", "Focus content", "Focus", func(p *popover.Popover) { p.SetTrigger(popover.TriggerFocus) }).Node(), mkPop("Click title", "Click content", "Click", func(p *popover.Popover) { p.SetTrigger(popover.TriggerClick) }).Node()),
			flexRow(mkPop("Top title", "Top content", "Top", nil).Node(), mkPop("Bottom title", "Bottom content", "Bottom", func(p *popover.Popover) { p.SetPlacement(popover.Bottom) }).Node(), mkPop("Left title", "Left content", "Left", func(p *popover.Popover) { p.SetPlacement(popover.Left) }).Node(), mkPop("Right title", "Right content", "Right", func(p *popover.Popover) { p.SetPlacement(popover.Right) }).Node()),
			flexRow(mkPop("Arrow on", "Caret shown", "Arrow", nil).Node(), mkPop("Arrow off", "Caret hidden", "NoArrow", func(p *popover.Popover) { p.SetArrow(false) }).Node(), mkPop("Point center", "Arrow centered", "Center", func(p *popover.Popover) { p.SetArrowConfig(true, true) }).Node()),
			mkPop("Shift title", "Flip near edge", "Shift", func(p *popover.Popover) { p.SetAutoAdjustOverflow(true) }).Node(),
			mkPop("Control title", "Press inner close", "Control", func(p *popover.Popover) { p.SetOpen(true) }).Node(),
			mkPop("Mixed title", "Hover plus click", "HoverClick", func(p *popover.Popover) { p.SetTriggerModes(popover.TriggerHover, popover.TriggerClick) }).Node(),
		}
	default:
		return nil
	}
}

// selftestWidgets layouts every registered page headless and checks the
// live nodes size up. It runs before any window opens so -auto-only can
// fail fast without GPU.
func selftestWidgets() []pfkit.ResultRow {
	var rows []pfkit.ResultRow
	check := func(name string, ok bool, detail string) {
		rows = append(rows, pfkit.ResultRow{Name: name, OK: ok, Detail: detail})
	}
	for _, p := range Pages() {
		if p.Name == "overview" {
			continue
		}
		nodes := contentForPage(p.Name)
		if len(nodes) == 0 {
			check("Widget/"+p.Name, false, "no live widget mounted")
			continue
		}
		if len(nodes) < len(p.Sections) {
			check("Widget/"+p.Name, false, fmt.Sprintf("nodes=%d sections=%d", len(nodes), len(p.Sections)))
			continue
		}
		s := NewScene(1200, 800, p.Name)
		if s.SelectedPage().Name != p.Name {
			check("Widget/"+p.Name, false, "page not selectable")
			continue
		}
		s.Layout()
		sz := s.Root.Size()
		ok := sz.Width == 1200 && sz.Height == 800
		check("Widget/"+p.Name, ok, fmt.Sprintf("nodes=%d root=%vx%v", len(nodes), sz.Width, sz.Height))
	}
	return rows
}
