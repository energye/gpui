//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerImage() {
	// Image — docs/antd/image.md §6.8 P0
	// https://ant.design/components/image
	// demos: basic / placeholder / fallback / preview-group /
	//        preview-group-visible / previewSrc / controlled-preview / toolbarRender
	//
	// P1 not shown: imageRender, mask, style-class, nested, true HTTP decode.

	face, th := c.face, c.theme
	status := c.status

	wire := func(im *kit.Image) *kit.Image {
		im.SetFace(face)
		if th != nil {
			im.SetTheme(th)
		}
		return im
	}
	fill := func(im *kit.Image, r, g, b byte) {
		im.SetPixels(4, 4, imageSolidRGBA(4, 4, r, g, b, 255))
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewImage())
	basic.SetWidth(200)
	basic.SetHeight(150)
	basic.SetAlt("basic")
	basic.SetSrc("https://zos.alipayobjects.com/rmsportal/jkjgkEfvpUPVyRjUImniVslZfWPnJuuZ.png")
	fill(basic, 180, 160, 140)
	basic.SetOnOpenChange(func(open bool) {
		if status != nil {
			*status = fmt.Sprintf("Image basic preview open=%v", open)
		}
	})
	secBasic := demoSection(face, th, "基本用法",
		"basic.tsx：width=200 + src；点击打开预览。",
		basic.Node())

	// ---------- placeholder.tsx ----------
	phInd := wire(kit.NewImage())
	phInd.SetWidth(120)
	phInd.SetHeight(120)
	phInd.SetPlaceholder(true)

	phPct := wire(kit.NewImage())
	phPct.SetWidth(120)
	phPct.SetHeight(120)
	phPct.SetPercent(50)

	phDone := wire(kit.NewImage())
	phDone.SetWidth(120)
	phDone.SetHeight(120)
	phDone.SetSrc("loaded.png")
	fill(phDone, 100, 160, 220)

	phRow := primitive.Row(phInd.Node(), phPct.Node(), phDone.Node())
	phRow.Gap = 16
	secPH := demoSection(face, th, "渐进加载",
		"placeholder.tsx：不确定进度 / percent=50 / 加载完成。Ticker 驱动不确定动画。",
		phRow)

	// ---------- fallback.tsx ----------
	fb := wire(kit.NewImage())
	fb.SetWidth(200)
	fb.SetHeight(150)
	fb.SetAlt("basic image")
	fb.SetSrc("error")
	fb.SetFallback("data:image/png;base64,fallback")
	fb.SetOnError(func() {
		if status != nil {
			*status = "Image onError → fallback"
		}
	})
	fb.NotifyImageError()
	fb.SetPixels(4, 4, imageSolidRGBA(4, 4, 90, 90, 100, 255))
	secFB := demoSection(face, th, "容错处理",
		"fallback.tsx：src 失败 → fallback 地址 + onError。",
		fb.Node())

	// ---------- preview-group.tsx ----------
	g1a := wire(kit.NewImageSized("svg image", 120, 90))
	g1a.SetSrc("https://gw.alipayobjects.com/zos/rmsportal/KDpgvguMpGfqaHPjicRK.svg")
	fill(g1a, 220, 100, 80)
	g1b := wire(kit.NewImageSized("svg image", 120, 90))
	g1b.SetSrc("https://gw.alipayobjects.com/zos/antfincdn/aPkFc8Sj7n/method-draw-image.svg")
	fill(g1b, 80, 160, 200)
	group := kit.NewImagePreviewGroup()
	group.SetFace(face)
	if th != nil {
		group.SetTheme(th)
	}
	group.OnChange = func(cur, prev int) {
		if status != nil {
			*status = fmt.Sprintf("PreviewGroup onChange %d → %d", prev, cur)
		}
	}
	group.Add(g1a, g1b)
	secGroup := demoSection(face, th, "多张图片预览",
		"preview-group.tsx：PreviewGroup 子图；切换触发 onChange。",
		group.Node())

	// ---------- preview-group-visible.tsx ----------
	albumThumb := wire(kit.NewImageSized("webp image", 160, 100))
	albumThumb.SetSrc("photo-1.webp")
	fill(albumThumb, 140, 170, 120)
	album := kit.NewImagePreviewGroup()
	album.SetFace(face)
	if th != nil {
		album.SetTheme(th)
	}
	album.SetItems("photo-1.webp", "photo-2.webp", "photo-3.webp")
	album.Add(albumThumb)
	album.OnOpenChange = func(open bool, cur int) {
		if status != nil {
			*status = fmt.Sprintf("Album open=%v current=%d", open, cur)
		}
	}
	secAlbum := demoSection(face, th, "相册模式",
		"preview-group-visible.tsx：items[] 三图相册；点缩略图预览并左右切换。",
		album.Node())

	// ---------- previewSrc.tsx ----------
	ps := wire(kit.NewImage())
	ps.SetWidth(160)
	ps.SetHeight(120)
	ps.SetAlt("basic image")
	ps.SetSrc("thumb-blur.png")
	ps.SetPreviewSrc("full-sharp.png")
	fill(ps, 160, 140, 200)
	secPS := demoSection(face, th, "自定义预览图片",
		"previewSrc.tsx：缩略 blur src，预览层用 preview.src。",
		ps.Node())

	// ---------- controlled-preview.tsx ----------
	ctrl := wire(kit.NewImage())
	ctrl.SetWidth(160)
	ctrl.SetHeight(1)
	ctrl.SetSrc("blur.png")
	ctrl.SetPreviewSrc("sharp.png")
	fill(ctrl, 100, 100, 100)
	openBtn := c.trackBtn(kit.NewButton("show image preview"))
	openBtn.SetType(kit.ButtonPrimary)
	openBtn.SetOnClick(func() {
		ctrl.SetPreviewOpen(true)
		if status != nil {
			*status = "controlled preview open"
		}
	})
	ctrl.SetOnOpenChange(func(open bool) {
		if status != nil {
			*status = fmt.Sprintf("controlled open=%v", open)
		}
	})
	ctrlCol := primitive.Column(openBtn.Node(), ctrl.Node())
	ctrlCol.Gap = 8
	secCtrl := demoSection(face, th, "受控的预览",
		"controlled-preview.tsx：按钮 SetPreviewOpen(true)；onOpenChange 回写。",
		ctrlCol)

	// ---------- toolbarRender.tsx ----------
	t0 := wire(kit.NewImageSized("image-0", 120, 90))
	t0.SetSrc("a.svg")
	fill(t0, 200, 120, 80)
	t1 := wire(kit.NewImageSized("image-1", 120, 90))
	t1.SetSrc("b.svg")
	fill(t1, 80, 140, 200)
	tbGroup := kit.NewImagePreviewGroup()
	tbGroup.SetFace(face)
	if th != nil {
		tbGroup.SetTheme(th)
	}
	tbGroup.SetActionsRender(func(info kit.ImageToolbarInfo) core.Node {
		mk := func(label string, fn func()) core.Node {
			b := kit.NewButton(label)
			b.SetFace(face)
			b.SetType(kit.ButtonText)
			b.SetOnClick(fn)
			return b.Node()
		}
		row := primitive.Row(
			mk("‹", func() {
				if info.OnActive != nil {
					info.OnActive(-1)
				}
			}),
			mk("›", func() {
				if info.OnActive != nil {
					info.OnActive(1)
				}
			}),
			mk("zoom+", info.OnZoomIn),
			mk("zoom−", info.OnZoomOut),
			mk("↺", info.OnRotateLeft),
			mk("↻", info.OnRotateRight),
			mk("flipX", info.OnFlipX),
			mk("flipY", info.OnFlipY),
			mk("reset", info.OnReset),
			mk("close", info.OnClose),
		)
		row.Gap = 4
		shell := primitive.NewDecorated(row)
		shell.Padding = primitive.All(8)
		shell.Radius = 100
		shell.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.35}
		return shell
	})
	tbGroup.OnChange = func(cur, prev int) {
		if status != nil {
			*status = fmt.Sprintf("toolbar group %d→%d", prev, cur)
		}
	}
	tbGroup.Add(t0, t1)
	secTB := demoSection(face, th, "自定义工具栏",
		"toolbarRender.tsx：ActionsRender 自定义 prev/next/zoom/rotate/flip/reset。",
		tbGroup.Node())

	page := demoPage(face, "Image 图片",
		"可预览的图片。P0：src/fallback/placeholder·percent/preview open·src/PreviewGroup items·onChange/工具栏。\n"+
			"P1：imageRender、mask/cover 高级、nested、真 HTTP 解码、semantic styles 深度。",
		secBasic, secPH, secFB, secGroup, secAlbum, secPS, secCtrl, secTB)
	c.addPage("image", "Image", page)
}

func imageSolidRGBA(w, h int, r, g, b, a byte) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pix[i*4+0] = r
		pix[i*4+1] = g
		pix[i*4+2] = b
		pix[i*4+3] = a
	}
	return pix
}
