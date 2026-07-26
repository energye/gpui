//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerCarousel() {
	// Carousel — docs/antd/carousel.md §6.8 P0
	// https://ant.design/components/carousel
	// demos: basic / placement / autoplay / fade / arrows / dot-duration
	//
	// P1 not shown: pixel-level scrollx/fade motion, custom arrows, slick extra
	// Settings, semantic classNames, ConfigProvider global, debug token demo.

	face, th := c.face, c.theme
	status := c.status

	wire := func(car *kit.Carousel) *kit.Carousel {
		car.SetFace(face)
		if th != nil {
			car.SetTheme(th)
		}
		c.trackTicker(car)
		return car
	}

	slide := func(label string) core.Node {
		lab := kit.NewText(label)
		lab.SetFace(face)
		lab.SetFontSize(18)
		lab.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
		box := primitive.NewDecorated(lab.Node())
		box.Background = render.Hex("#364d79")
		box.Height = kit.DefaultCarouselStageHeight
		box.StretchChild = true
		box.CenterContent = true
		box.Hit = core.HitDefer
		// Center label roughly via padding (demo rhythm matches antd 160px stage).
		box.Padding = primitive.EdgeInsets{Top: 64}
		return box
	}
	four := func() []core.Node {
		return []core.Node{slide("1"), slide("2"), slide("3"), slide("4")}
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewCarousel(four()...))
	basic.SetAfterChange(func(cur int) {
		if status != nil {
			*status = fmt.Sprintf("Carousel afterChange → %d", cur)
		}
	})
	secBasic := demoSection(face, th, "基本",
		"basic.tsx：默认 dots=bottom、infinite；afterChange 回调。",
		basic.Node())

	// ---------- placement.tsx ----------
	placements := []struct {
		name string
		p    kit.CarouselDotPlacement
	}{
		{"top", kit.CarouselDotTop},
		{"bottom", kit.CarouselDotBottom},
		{"start", kit.CarouselDotStart},
		{"end", kit.CarouselDotEnd},
	}
	placeCol := primitive.Column()
	placeCol.Gap = 12
	placeCol.CrossAlign = core.CrossStretch
	for _, pl := range placements {
		car := wire(kit.NewCarousel(four()...))
		car.SetDotPlacement(pl.p)
		lab := kit.NewText("dotPlacement=" + pl.name)
		lab.SetFace(face)
		row := primitive.Column(lab.Node(), car.Node())
		row.Gap = 4
		row.CrossAlign = core.CrossStretch
		placeCol.AddChild(row)
	}
	secPlace := demoSection(face, th, "位置",
		"placement.tsx：top / bottom / start / end（start/end 纵向）。",
		placeCol)

	// ---------- autoplay.tsx ----------
	ap := wire(kit.NewCarousel(four()...))
	ap.SetAutoplay(true)
	secAP := demoSection(face, th, "自动切换",
		"autoplay.tsx：autoplay 默认间隔 3000ms。",
		ap.Node())

	// ---------- fade.tsx ----------
	fade := wire(kit.NewCarousel(four()...))
	fade.SetEffect(kit.CarouselFade)
	secFade := demoSection(face, th, "渐显",
		"fade.tsx：effect=fade（P0 切换可瞬时；flag 对齐）。",
		fade.Node())

	// ---------- arrows.tsx ----------
	arH := wire(kit.NewCarousel(four()...))
	arH.SetArrows(true)
	arH.SetInfinite(false)
	arV := wire(kit.NewCarousel(four()...))
	arV.SetArrows(true)
	arV.SetInfinite(false)
	arV.SetDotPlacement(kit.CarouselDotStart)
	arCol := primitive.Column(arH.Node(), arV.Node())
	arCol.Gap = 16
	arCol.CrossAlign = core.CrossStretch
	secArrows := demoSection(face, th, "切换箭头",
		"arrows.tsx：arrows + infinite=false；横向与 start 纵向。",
		arCol)

	// ---------- dot-duration.tsx ----------
	dur := wire(kit.NewCarousel(four()...))
	dur.SetAutoplay(true)
	dur.SetDotDuration(true)
	dur.SetAutoplaySpeed(5000)
	secDur := demoSection(face, th, "进度条",
		"dot-duration.tsx：autoplay={{ dotDuration: true }} + autoplaySpeed=5000。",
		dur.Node())

	// Extra P0 surfaces: draggable (no dedicated official demo page).
	drag := wire(kit.NewCarousel(four()...))
	drag.SetDraggable(true)
	drag.SetArrows(true)
	secDrag := demoSection(face, th, "拖拽（P0）",
		"draggable=true：舞台拖拽超过阈值切换；可与 arrows 并用。",
		drag.Node())

	// Lifecycle (#9)
	life := wire(kit.NewCarousel(four()...))
	life.SetArrows(true)
	life.SetDots(true)
	life.SetSpeed(300)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Slides → Arrows/Dots → Speed；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6) — Root 是 Stack；TypeID=kit.Carousel 已注册，Override 证明挂接点存在。
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeCarousel, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeCarousel); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinCar := wire(kit.NewCarousel(four()...))
	skinCar.SetArrows(true)
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"TypeID=kit.Carousel 已注册；Theme.Skin Override 可挂接（Root Stack 走默认子节点绘制）。",
		skinCar.Node())

	page := demoPage(face, "Carousel 走马灯",
		"Ant Design Carousel — docs/antd/carousel.md §6.8 P0。"+
			"一组轮播区域：dots / arrows / autoplay / fade / placement / dotDuration。Also #9 lifecycle + #6 Skin.",
		secBasic, secPlace, secAP, secFade, secArrows, secDur, secDrag, secLife, secSkin)

	c.addPage("carousel", "Carousel", page)
}
