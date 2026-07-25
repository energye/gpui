//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSkeleton() {
	// Skeleton — docs/antd/skeleton.md §6.8 P0
	// https://ant.design/components/skeleton
	// demos: basic / complex / active / element / children / list / style-class / _semantic
	//
	// P1 not shown: title/paragraph/avatar full object forms, styles(info)=> depth,
	// pixel-perfect shimmer, browser-only APIs, debug/官网逐像素.

	face, th := c.face, c.theme
	status := c.status

	wire := func(s *kit.Skeleton) *kit.Skeleton {
		if th != nil {
			s.SetTheme(th)
		}
		c.trackTicker(s)
		return s
	}
	wireEl := func(t interface{ AttachTicker(*core.Tree) }) {
		c.trackTicker(t)
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewSkeleton())
	secBasic := demoSection(face, th, "基本",
		"basic.tsx：默认 title + 3 行 paragraph。",
		basic.Node())

	// ---------- complex.tsx ----------
	complex := wire(kit.NewSkeleton())
	complex.SetAvatar(true)
	complex.SetParagraphRows(4)
	secComplex := demoSection(face, th, "复杂的组合",
		"complex.tsx：avatar + title + paragraph rows=4。",
		complex.Node())

	// ---------- active.tsx ----------
	active := wire(kit.NewSkeleton())
	active.SetActive(true)
	secActive := demoSection(face, th, "动画效果",
		"active.tsx：active shimmer（Ticker；像素级 P1）。",
		active.Node())

	// ---------- element.tsx ----------
	btn := kit.NewSkeletonButton()
	btn.SetActive(true)
	if th != nil {
		btn.SetTheme(th)
	}
	wireEl(btn)
	avatar := kit.NewSkeletonAvatar()
	avatar.SetActive(true)
	if th != nil {
		avatar.SetTheme(th)
	}
	wireEl(avatar)
	input := kit.NewSkeletonInput()
	input.SetActive(true)
	if th != nil {
		input.SetTheme(th)
	}
	wireEl(input)
	row1 := primitive.Row(btn.Node(), avatar.Node(), input.Node())
	row1.Gap = 12
	row1.CrossAlign = core.CrossCenter

	blockBtn := kit.NewSkeletonButton()
	blockBtn.SetActive(true)
	blockBtn.SetBlock(true)
	if th != nil {
		blockBtn.SetTheme(th)
	}
	wireEl(blockBtn)
	blockInput := kit.NewSkeletonInput()
	blockInput.SetActive(true)
	blockInput.SetBlock(true)
	if th != nil {
		blockInput.SetTheme(th)
	}
	wireEl(blockInput)
	blockCol := primitive.Column(blockBtn.Node(), blockInput.Node())
	blockCol.Gap = 12
	blockCol.CrossAlign = core.CrossStretch

	img := kit.NewSkeletonImage()
	img.SetActive(true)
	if th != nil {
		img.SetTheme(th)
	}
	wireEl(img)
	nodeA := kit.NewSkeletonNode(nil)
	nodeA.SetActive(true)
	nodeA.SetStyle(kit.Style{Width: 160})
	if th != nil {
		nodeA.SetTheme(th)
	}
	wireEl(nodeA)
	icon := kit.NewIcon("info")
	icon.SetSize(40)
	if th != nil {
		icon.SetTheme(th)
	}
	nodeB := kit.NewSkeletonNode(icon.Node())
	nodeB.SetActive(true)
	if th != nil {
		nodeB.SetTheme(th)
	}
	wireEl(nodeB)
	row2 := primitive.Row(img.Node(), nodeA.Node(), nodeB.Node())
	row2.Gap = 12
	row2.CrossAlign = core.CrossCenter

	elCol := primitive.Column(row1, blockCol, row2)
	elCol.Gap = 16
	elCol.CrossAlign = core.CrossStretch
	secElement := demoSection(face, th, "按钮/头像/输入框/图像/自定义节点",
		"element.tsx：Skeleton.Button / Avatar / Input / Image / Node。",
		elCol)

	// ---------- children.tsx ----------
	// Desktop gallery: toggle instead of antd setTimeout(3s).
	childrenBody := primitive.Column()
	childrenBody.Gap = 8
	childrenBody.CrossAlign = core.CrossStart
	titleT := kit.NewText("Ant Design, a design language")
	titleT.SetFace(face)
	titleT.SetFontSize(16)
	bodyT := kit.NewParagraph("We supply a series of design principles, practical patterns and high quality design resources, to help people create their product prototypes beautifully and efficiently.")
	bodyT.SetFace(face)
	bodyT.SetEllipsisRows(6)
	childrenBody.AddChild(titleT.Node())
	childrenBody.AddChild(bodyT.Node())

	childSk := wire(kit.NewSkeleton())
	childSk.SetContent(childrenBody)
	childLoading := true
	childSk.SetLoading(true)
	showBtn := c.trackBtn(kit.NewButton("Show Content"))
	showBtn.SetOnClick(func() {
		childLoading = !childLoading
		childSk.SetLoading(childLoading)
		if childLoading {
			showBtn.SetLabel("Show Content")
			if status != nil {
				*status = "Skeleton · children loading"
			}
			return
		}
		showBtn.SetLabel("Show Skeleton")
		if status != nil {
			*status = "Skeleton · children content"
		}
	})
	childCol := primitive.Column(childSk.Node(), showBtn.Node())
	childCol.Gap = 16
	childCol.CrossAlign = core.CrossStart
	secChildren := demoSection(face, th, "包含子组件",
		"children.tsx：loading 切换骨架 / 真实内容（gallery 用 Toggle 代替 setTimeout）。",
		childCol)

	// ---------- list.tsx ----------
	listCol := primitive.Column()
	listCol.Gap = 16
	listCol.CrossAlign = core.CrossStretch
	listLoading := true
	var listItems []*kit.Skeleton
	for i := 0; i < 3; i++ {
		s := wire(kit.NewSkeleton())
		s.SetAvatar(true)
		s.SetActive(true)
		s.SetLoading(true)
		// content for loaded state
		meta := primitive.Column()
		meta.Gap = 4
		meta.CrossAlign = core.CrossStart
		mt := kit.NewText("ant design part")
		mt.SetFace(face)
		md := kit.NewParagraph("Ant Design, a design language for background applications, is refined by Ant UED Team.")
		md.SetFace(face)
		md.SetEllipsisRows(3)
		mc := kit.NewParagraph("We supply a series of design principles, practical patterns and high quality design resources.")
		mc.SetFace(face)
		mc.SetEllipsisRows(4)
		meta.AddChild(mt.Node())
		meta.AddChild(md.Node())
		meta.AddChild(mc.Node())
		s.SetContent(meta)
		listItems = append(listItems, s)
		listCol.AddChild(s.Node())
	}
	listSwitch := c.trackBtn(kit.NewButton("Show loaded list"))
	listSwitch.SetOnClick(func() {
		listLoading = !listLoading
		for _, s := range listItems {
			s.SetLoading(listLoading)
			s.SetActive(listLoading)
		}
		if listLoading {
			listSwitch.SetLabel("Show loaded list")
			if status != nil {
				*status = "Skeleton · list loading"
			}
		} else {
			listSwitch.SetLabel("Show skeleton list")
			if status != nil {
				*status = "Skeleton · list loaded"
			}
		}
	})
	listWrap := primitive.Column(listSwitch.Node(), listCol)
	listWrap.Gap = 12
	listWrap.CrossAlign = core.CrossStretch
	secList := demoSection(face, th, "列表",
		"list.tsx：3 条 avatar+active 骨架；切换显示真实列表项。",
		listWrap)

	// ---------- style-class.tsx ----------
	styleA := wire(kit.NewSkeleton())
	styleA.SetAvatar(true)
	styleA.SetParagraph(false)
	styleA.SetClassNames(kit.SkeletonClassNames{
		Root: "sk-root", Header: "sk-header", Avatar: "sk-avatar", Title: "sk-title",
	})
	styleA.SetStyles(kit.SkeletonStyles{
		Root:   kit.Style{Radius: 10},
		Avatar: kit.Style{Border: render.RGBA{R: 0.67, G: 0.67, B: 0.67, A: 1}},
		Title:  kit.Style{Border: render.RGBA{R: 0.67, G: 0.67, B: 0.67, A: 1}},
	})
	styleB := wire(kit.NewSkeleton())
	styleB.SetActive(true)
	styleB.SetClassNames(kit.SkeletonClassNames{
		Root: "sk-root-active", Paragraph: "sk-paragraph",
	})
	styleB.SetStyles(kit.SkeletonStyles{
		Root:  kit.Style{Border: render.RGBA{R: 0.9, G: 0.95, B: 1, A: 0.3}, Radius: 10},
		Title: kit.Style{Background: render.RGBA{R: 0.9, G: 0.95, B: 1, A: 0.5}, Height: 20, Radius: 20},
	})
	styleRow := primitive.Row(styleA.Node(), styleB.Node())
	styleRow.Gap = 16
	styleRow.CrossAlign = core.CrossStart
	secStyle := demoSection(face, th, "自定义语义结构的样式和类",
		"style-class.tsx：浅 classNames + styles（函数形态 P1）。",
		styleRow)

	// ---------- _semantic.tsx ----------
	sem := wire(kit.NewSkeleton())
	sem.SetAvatar(true)
	sem.SetParagraphRows(4)
	sem.SetClassNames(kit.SkeletonClassNames{
		Root: "root", Header: "header", Section: "section",
		Avatar: "avatar", Title: "title", Paragraph: "paragraph",
	})
	secSem := demoSection(face, th, "Semantic DOM",
		"_semantic.tsx：root / header / section / avatar / title / paragraph 语义钩子。",
		sem.Node())

	// ---------- round (extra P0 chrome) ----------
	round := wire(kit.NewSkeleton())
	round.SetRound(true)
	round.SetActive(true)
	secRound := demoSection(face, th, "圆角条块",
		"round=true：标题与段落胶囊化。",
		round.Node())

	page := demoPage(face, "Skeleton 骨架屏",
		"Feedback · docs/antd/skeleton.md §6 P0 — 在需要等待加载内容的位置提供占位图形组合。",
		secBasic, secComplex, secActive, secElement, secChildren, secList, secStyle, secSem, secRound)
	c.addPage("skeleton", "Skeleton", page)
}
