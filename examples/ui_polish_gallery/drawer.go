//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerDrawer() {
	basic := c.newGalleryDrawer("Basic Drawer", simpleDrawerContent("Some contents...", 3))
	basicFooter := primitive.Row(c.trackBtn(kit.NewButton("Cancel")).Node(), c.trackBtn(kit.NewButton("Submit")).Node())
	basicFooter.Gap = 8
	basicFooter.MainAlign = core.MainEnd
	basic.SetFooter(basicFooter)

	placement := c.newGalleryDrawer("Basic Drawer", simpleDrawerContent("Some contents...", 3))
	placement.SetClosable(false)

	resizable := c.newGalleryDrawer("Resizable Drawer", simpleDrawerContent("Drag the edge to resize the drawer", 2))
	resizable.SetSizePx(256)
	resizable.SetResizable(true)
	resizable.SetResizeBounds(200, 560)

	loading := c.newGalleryDrawer("Loading Drawer", loadingDrawerBody(c))
	loading.SetDestroyOnHidden(true)

	extra := c.newGalleryDrawer("Drawer with extra actions", simpleDrawerContent("Some contents...", 3))
	extra.SetSizePx(500)
	extraActions := primitive.Row(c.trackBtn(kit.NewButton("Cancel")).Node(), c.trackBtn(kit.NewButton("OK")).Node())
	extraActions.Gap = 8
	extra.SetExtra(extraActions)

	form := c.newGalleryDrawer("Create a new account", drawerFormBody())
	form.SetSizePx(720)
	formActions := primitive.Row(c.trackBtn(kit.NewButton("Cancel")).Node(), c.trackBtn(kit.NewButton("Submit")).Node())
	formActions.Gap = 8
	form.SetExtra(formActions)

	profile := c.newGalleryDrawer("User Profile", drawerProfileBody())
	profile.SetSizePx(640)
	profile.SetClosable(false)

	drawers := []core.Node{
		basic.Node(),
		placement.Node(),
		resizable.Node(),
		loading.Node(),
		extra.Node(),
		form.Node(),
		profile.Node(),
	}
	c.trackTicker(loading)

	page := demoPage(c.face, "Drawer", "Feedback / Drawer",
		demoSection(c.face, c.theme, "Basic Drawer", "", drawerOpenRow(c, basic, "Open")),
		demoSection(c.face, c.theme, "Custom Placement", "", drawerPlacementRow(c, placement)),
		demoSection(c.face, c.theme, "Resizable", "", drawerOpenRow(c, resizable, "Open Drawer")),
		demoSection(c.face, c.theme, "Loading", "", drawerLoadingRow(c, loading)),
		demoSection(c.face, c.theme, "Extra Actions", "", drawerPlacementRow(c, extra)),
		demoSection(c.face, c.theme, "Form In Drawer", "", drawerOpenRow(c, form, "New account")),
		demoSection(c.face, c.theme, "User Profile", "", drawerOpenRow(c, profile, "View Profile")),
	)
	if col, ok := page.(*primitive.Flex); ok {
		for _, n := range drawers {
			col.AddChild(n)
		}
	}
	c.addPage("drawer", "Drawer", page)
}

func (c *catalogCtx) newGalleryDrawer(title string, body core.Node) *kit.Drawer {
	d := kit.NewDrawer(title)
	d.SetFace(c.face)
	d.SetContent(body)
	d.OnClose = func() {
		*c.status = title + " closed"
	}
	d.OnOpenChange = func(open bool) {
		if open {
			*c.status = title + " open"
		}
	}
	return d
}

func drawerOpenRow(c *catalogCtx, d *kit.Drawer, label string) core.Node {
	open := c.trackBtn(kit.NewButton(label))
	open.SetType(kit.ButtonPrimary)
	open.SetOnClick(func() { d.SetOpen(true) })
	return spaceWrap(8, open.Node())
}

func drawerPlacementRow(c *catalogCtx, d *kit.Drawer) core.Node {
	row := primitive.Row()
	row.Gap = 8
	for _, tc := range []struct {
		label string
		p     kit.DrawerPlacement
	}{
		{"top", kit.DrawerPlacementTop},
		{"right", kit.DrawerPlacementRight},
		{"bottom", kit.DrawerPlacementBottom},
		{"left", kit.DrawerPlacementLeft},
	} {
		btn := c.trackBtn(kit.NewButton(tc.label))
		btn.SetOnClick(func(p kit.DrawerPlacement) func() {
			return func() {
				d.SetPlacement(p)
				d.SetOpen(true)
			}
		}(tc.p))
		row.AddChild(btn.Node())
	}
	return row
}

func drawerLoadingRow(c *catalogCtx, d *kit.Drawer) core.Node {
	open := c.trackBtn(kit.NewButton("Open Drawer"))
	open.SetType(kit.ButtonPrimary)
	open.SetOnClick(func() {
		d.SetLoading(true)
		d.SetOpen(true)
	})
	done := c.trackBtn(kit.NewButton("Finish Loading"))
	done.SetOnClick(func() { d.SetLoading(false) })
	return spaceWrap(8, open.Node(), done.Node())
}

func simpleDrawerContent(value string, count int) core.Node {
	col := primitive.Column()
	col.Gap = 8
	for i := 0; i < count; i++ {
		col.AddChild(kit.NewText(value).Node())
	}
	return col
}

func loadingDrawerBody(c *catalogCtx) core.Node {
	reload := c.trackBtn(kit.NewButton("Reload"))
	reload.SetType(kit.ButtonPrimary)
	return primitive.Column(reload.Node(), kit.NewText("Some contents...").Node(), kit.NewText("Some contents...").Node())
}

func drawerFormBody() core.Node {
	form := kit.NewForm(nil)
	form.BindInput("name", kit.NewInput("Please enter user name"), true, "Name")
	form.BindInput("url", kit.NewInput("Please enter url"), true, "Url")
	form.BindInput("owner", kit.NewInput("Please select an owner"), true, "Owner")
	form.BindInput("type", kit.NewInput("Please choose the type"), true, "Type")
	form.BindInput("description", kit.NewTextArea("please enter url description", 4).Input, true, "Description")
	return form.Node()
}

func drawerProfileBody() core.Node {
	col := primitive.Column()
	col.Gap = 12
	for _, s := range []string{
		"Personal",
		"Full Name: Lily",
		"Account: AntDesign@example.com",
		"City: HangZhou",
		"Company",
		"Position: Programmer",
		"Department: XTech",
		"Contacts",
		"Email: AntDesign@example.com",
		"GitHub: github.com/ant-design/ant-design",
	} {
		col.AddChild(kit.NewText(s).Node())
	}
	return col
}
