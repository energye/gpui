package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// antd combination.tsx: Get Started at bottom-right with large gap above it.
func TestComboGalleryExact_GetStartedBottomRight(t *testing.T) {
	const imgW, imgH, cardW = 273.0, 273.0, 620.0
	flexImg := primitive.NewDecorated(nil)
	flexImg.Width, flexImg.Height = imgW, imgH

	quote := kit.NewTitle("“antd is an enterprise-class UI design language and React UI library.”", 3)
	quote.SetEllipsisRows(3)
	quoteBox := primitive.NewDecorated(quote.Node())
	quoteBox.Width = 250

	btn := kit.NewButton("Get Started")
	btn.SetType(kit.ButtonPrimary)

	inner := kit.NewFlex(quoteBox, btn.Node())
	inner.SetVertical(true)
	inner.SetAlign(kit.FlexAlignEnd)
	inner.SetJustify(kit.FlexJustifySpaceBetween)

	pad := primitive.NewDecorated(inner.Node())
	pad.Padding = primitive.All(32)
	pad.Width = cardW - imgW
	pad.Height = imgH
	pad.StretchChild = true

	row := kit.NewFlex(flexImg, pad)
	row.SetJustify(kit.FlexJustifyStart)
	row.SetAlign(kit.FlexAlignStart)

	card := primitive.NewDecorated(row.Node())
	card.Width = cardW
	_ = card.Layout(core.Loose(800, 400))

	// panes flush
	if pad.Base().Offset().X < imgW-1 || pad.Base().Offset().X > imgW+1 {
		t.Fatalf("right x=%v want %v", pad.Base().Offset().X, imgW)
	}
	innerH := inner.Root.Base().Size().Height
	// content area = 273 - 64 = 209
	if innerH < 200 || innerH > 220 {
		t.Fatalf("inner h=%v want ~209", innerH)
	}
	kids := inner.Root.Children()
	titleH := kids[0].Base().Size().Height
	btnY := kids[1].Base().Offset().Y
	btnH := kids[1].Base().Size().Height
	gap := btnY - titleH
	if gap < 60 {
		t.Fatalf("gap between title and button too small: %v (titleH=%v btnY=%v) — want large space-between gap", gap, titleH, btnY)
	}
	if btnY+btnH < innerH-2 {
		t.Fatalf("button not at bottom y=%v h=%v innerH=%v", btnY, btnH, innerH)
	}
	innerW := inner.Root.Base().Size().Width
	if kids[1].Base().Offset().X+kids[1].Base().Size().Width < innerW-2 {
		t.Fatalf("button not right-aligned")
	}
}
