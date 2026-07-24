//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerButton() {
	// Button — Ant Design docs-style multi-section page
	// https://ant.design/components/button
	mkBtn := func(label string, typ kit.ButtonType) *kit.Button {
		b := kit.NewButton(label)
		b.SetType(typ)
		return c.trackBtn(b)
	}

	// Type
	bPrimary := mkBtn("Primary Button", kit.ButtonPrimary)
	bDefault := mkBtn("Default Button", kit.ButtonDefault)
	bDashed := mkBtn("Dashed Button", kit.ButtonDashed)
	bText := mkBtn("Text Button", kit.ButtonText)
	bLink := mkBtn("Link Button", kit.ButtonLink)
	secType := demoSection(c.face, c.theme, "Type",
		"There are primary button, default button, dashed button, text button and link button.",
		spaceWrap(8, bPrimary.Node(), bDefault.Node(), bDashed.Node(), bText.Node(), bLink.Node()))

	// Icon
	bIconSearch := mkBtn("Search", kit.ButtonPrimary)
	bIconSearch.SetIcon("search")
	bIconPlus := mkBtn("Add", kit.ButtonDefault)
	bIconPlus.SetIcon("plus")
	bIconOnly := mkBtn("Search", kit.ButtonDefault)
	bIconOnly.SetIcon("search")
	secIcon := demoSection(c.face, c.theme, "Icon",
		"Button components can contain an Icon (leading or end).",
		spaceWrap(8, bIconSearch.Node(), bIconPlus.Node(), bIconOnly.Node()))

	// Icon placement end
	bIconEnd := mkBtn("Search", kit.ButtonDefault)
	bIconEnd.SetIcon("search")
	bIconEnd.SetIconPlacement(kit.ButtonIconEnd)
	secIconEnd := demoSection(c.face, c.theme, "Icon Placement",
		"Icon at start (default) or end of the label.",
		spaceWrap(8, bIconSearch.Node(), bIconEnd.Node()))

	// Size
	bLarge := mkBtn("Large", kit.ButtonPrimary)
	bLarge.SetSize(kit.ButtonLarge)
	bMiddle := mkBtn("Middle", kit.ButtonPrimary)
	bMiddle.SetSize(kit.ButtonMiddle)
	bSmall := mkBtn("Small", kit.ButtonPrimary)
	bSmall.SetSize(kit.ButtonSmall)
	secSize := demoSection(c.face, c.theme, "Size",
		"Ant Design supports a default button size as well as a large and small size.",
		spaceWrap(8, bLarge.Node(), bMiddle.Node(), bSmall.Node()))

	// Disabled
	bDisP := mkBtn("Primary", kit.ButtonPrimary)
	bDisP.SetDisabled(true)
	bDisD := mkBtn("Default", kit.ButtonDefault)
	bDisD.SetDisabled(true)
	bDisH := mkBtn("Dashed", kit.ButtonDashed)
	bDisH.SetDisabled(true)
	bDisT := mkBtn("Text", kit.ButtonText)
	bDisT.SetDisabled(true)
	bDisL := mkBtn("Link", kit.ButtonLink)
	bDisL.SetDisabled(true)
	secDisabled := demoSection(c.face, c.theme, "Disabled",
		"To mark a button as disabled, add the disabled property to the Button.",
		spaceWrap(8, bDisP.Node(), bDisD.Node(), bDisH.Node(), bDisT.Node(), bDisL.Node()))

	// Loading
	bLoad1 := mkBtn("Loading", kit.ButtonPrimary)
	bLoad1.SetLoading(true)
	bLoad2 := mkBtn("Loading", kit.ButtonDefault)
	bLoad2.SetLoading(true)
	bLoad3 := mkBtn("Loading", kit.ButtonDashed)
	bLoad3.SetLoading(true)
	*c.tickers = append(*c.tickers, bLoad1, bLoad2, bLoad3)
	secLoading := demoSection(c.face, c.theme, "Loading",
		"A loading indicator can be added to a button by setting the loading property on the Button.",
		spaceWrap(8, bLoad1.Node(), bLoad2.Node(), bLoad3.Node()))

	// Multiple
	bM1 := mkBtn("Button 1", kit.ButtonDefault)
	bM2 := mkBtn("Button 2", kit.ButtonDefault)
	bM3 := mkBtn("Button 3", kit.ButtonDefault)
	secMultiple := demoSection(c.face, c.theme, "Multiple Buttons",
		"If you need several buttons, we recommend that you use Space to set the spacing.",
		spaceWrap(8, bM1.Node(), bM2.Node(), bM3.Node()))

	// Danger
	bDangP := mkBtn("Primary", kit.ButtonPrimary)
	bDangP.SetDanger(true)
	bDangD := mkBtn("Default", kit.ButtonDefault)
	bDangD.SetDanger(true)
	bDangH := mkBtn("Dashed", kit.ButtonDashed)
	bDangH.SetDanger(true)
	bDangT := mkBtn("Text", kit.ButtonText)
	bDangT.SetDanger(true)
	bDangL := mkBtn("Link", kit.ButtonLink)
	bDangL.SetDanger(true)
	secDanger := demoSection(c.face, c.theme, "Danger Buttons",
		"Danger buttons are used for actions with higher risk.",
		spaceWrap(8, bDangP.Node(), bDangD.Node(), bDangH.Node(), bDangT.Node(), bDangL.Node()))

	// Block
	bBlock := mkBtn("Primary Block Button", kit.ButtonPrimary)
	bBlock.SetBlock(true)
	bBlock2 := mkBtn("Default Block Button", kit.ButtonDefault)
	bBlock2.SetBlock(true)
	blockCol := primitive.Column(bBlock.Node(), bBlock2.Node())
	blockCol.Gap = 8
	blockCol.CrossAlign = core.CrossStretch
	secBlock := demoSection(c.face, c.theme, "Block Button",
		"block property will make the button fit to its parent width.",
		blockCol)

	// Ghost — Ant demos put ghost on a dark/complex surface so transparency is obvious.
	bGhostP := mkBtn("Primary", kit.ButtonPrimary)
	bGhostP.SetGhost(true)
	bGhostD := mkBtn("Default", kit.ButtonDefault)
	bGhostD.SetGhost(true)
	bGhostR := mkBtn("Danger", kit.ButtonPrimary)
	bGhostR.SetDanger(true)
	bGhostR.SetGhost(true)
	ghostRow := kit.NewSpace(bGhostP.Node(), bGhostD.Node(), bGhostR.Node())
	ghostRow.SetSizePx(8)
	ghostRow.SetWrap(true)
	ghostHost := primitive.NewDecorated(ghostRow.Node())
	ghostHost.Padding = primitive.All(16)
	ghostHost.Radius = 8
	// Ant docs use a dark band behind ghost buttons.
	ghostHost.Background = render.RGBA{R: 0.12, G: 0.14, B: 0.18, A: 1}
	ghostHost.BorderWidth = 0
	secGhost := demoSection(c.face, c.theme, "Ghost Button",
		"Ghost = transparent fill (use on dark / image backgrounds). Primary/Default/Danger differ by border & text color.",
		ghostHost)

	// Color & Variant — each variant looks different; Color changes the accent.
	mkVar := func(label string, v kit.ButtonVariant, c kit.ButtonColor) core.Node {
		b := mkBtn(label, kit.ButtonDefault)
		b.SetVariant(v)
		b.SetColor(c)
		return b.Node()
	}
	rowPrimary := spaceWrap(8,
		mkVar("Solid", kit.ButtonVariantSolid, kit.ButtonColorPrimary),
		mkVar("Outlined", kit.ButtonVariantOutlined, kit.ButtonColorPrimary),
		mkVar("Dashed", kit.ButtonVariantDashed, kit.ButtonColorPrimary),
		mkVar("Filled", kit.ButtonVariantFilled, kit.ButtonColorPrimary),
		mkVar("Text", kit.ButtonVariantText, kit.ButtonColorPrimary),
		mkVar("Link", kit.ButtonVariantLink, kit.ButtonColorPrimary),
	)
	rowDanger := spaceWrap(8,
		mkVar("Solid", kit.ButtonVariantSolid, kit.ButtonColorDanger),
		mkVar("Outlined", kit.ButtonVariantOutlined, kit.ButtonColorDanger),
		mkVar("Filled", kit.ButtonVariantFilled, kit.ButtonColorDanger),
		mkVar("Text", kit.ButtonVariantText, kit.ButtonColorDanger),
	)
	rowSuccess := spaceWrap(8,
		mkVar("Solid", kit.ButtonVariantSolid, kit.ButtonColorSuccess),
		mkVar("Outlined", kit.ButtonVariantOutlined, kit.ButtonColorSuccess),
		mkVar("Filled", kit.ButtonVariantFilled, kit.ButtonColorSuccess),
	)
	varCol := primitive.Column(
		sec(c.face, "color=primary · variant=…"),
		rowPrimary,
		sec(c.face, "color=danger"),
		rowDanger,
		sec(c.face, "color=success"),
		rowSuccess,
	)
	varCol.Gap = 10
	varCol.CrossAlign = core.CrossStart
	secVariant := demoSection(c.face, c.theme, "Color & Variant",
		"Solid=fill · Outlined=border · Dashed=dashed border · Filled=light wash · Text/Link=no chrome. Same Type「Primary」≈ Solid+primary.",
		varCol)

	c.items = append(c.items, ctlTab("btn", "Button"))
	c.contents["btn"] = demoPage(c.face, "Button",
		"To trigger an operation. Aligns Ant Design Button demos (type/size/icon/disabled/loading/danger/block/ghost/variant).",
		secType, secIcon, secIconEnd, secSize, secDisabled, secLoading, secMultiple, secDanger, secBlock, secGhost, secVariant)
}
