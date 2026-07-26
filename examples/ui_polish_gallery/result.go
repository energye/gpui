//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerResult() {
	// Result — docs/antd/result.md §6.8 P0
	// https://ant.design/components/result
	// demos: success / info / warning / 403 / 404 / 500 / error / customIcon
	//
	// P1 not shown: semantic classNames/styles depth, _semantic.tsx,
	// style-class.tsx, ConfigProvider global defaults, pixel-perfect motion/hash.

	face, th := c.face, c.theme
	status := c.status

	wire := func(r *kit.Result) *kit.Result {
		r.SetFace(face)
		if th != nil {
			r.SetTheme(th)
		}
		return r
	}
	action := func(label string, primary bool) core.Node {
		b := c.trackBtn(kit.NewButton(label))
		if primary {
			b.SetType(kit.ButtonPrimary)
		}
		b.SetOnClick(func() {
			if status != nil {
				*status = "Result · " + label
			}
		})
		return b.Node()
	}

	success := wire(kit.NewResult())
	success.SetStatus(kit.ResultSuccess)
	success.SetTitle("Successfully Purchased Cloud Server ECS!")
	success.SetSubTitle("Order number: 2017182818828182881 Cloud server configuration takes 1-5 minutes, please wait.")
	success.SetExtra(action("Go Console", true), action("Buy Again", false))
	secSuccess := demoSection(face, th, "Success",
		"success.tsx：success 状态、长 subTitle、primary/default 操作按钮。",
		success.Node())

	info := wire(kit.NewResult())
	info.SetTitle("Your operation has been executed")
	info.SetExtra(action("Go Console", true))
	secInfo := demoSection(face, th, "Info",
		"info.tsx：默认 status=info，单个 primary 操作按钮。",
		info.Node())

	warning := wire(kit.NewResult())
	warning.SetStatus(kit.ResultWarning)
	warning.SetTitle("There are some problems with your operation.")
	warning.SetExtra(action("Go Console", true))
	secWarning := demoSection(face, th, "Warning",
		"warning.tsx：warning 状态与操作区。",
		warning.Node())

	exceptions := primitive.Column()
	exceptions.Gap = 16
	exceptions.CrossAlign = core.CrossStretch
	for _, cfg := range []struct {
		st  kit.ResultStatus
		sub string
	}{
		{kit.Result403, "Sorry, you are not authorized to access this page."},
		{kit.Result404, "Sorry, the page you visited does not exist."},
		{kit.Result500, "Sorry, something went wrong."},
	} {
		r := wire(kit.NewResult())
		r.SetStatus(cfg.st)
		r.SetTitle(string(cfg.st))
		r.SetSubTitle(cfg.sub)
		r.SetExtra(action("Back Home", true))
		exceptions.AddChild(r.Node())
	}
	secExceptions := demoSection(face, th, "403 / 404 / 500",
		"异常页主路径：异常图 250×295、标题、说明和 Back Home 操作。",
		exceptions)

	errRes := wire(kit.NewResult())
	errRes.SetStatus(kit.ResultError)
	errRes.SetTitle("Submission Failed")
	errRes.SetSubTitle("Please check and modify the following information before resubmitting.")
	errRes.SetExtra(action("Go Console", true), action("Buy Again", false))
	body := primitive.Column()
	body.Gap = 8
	for _, line := range []string{
		"The content you submitted has the following error:",
		"x Your account has been frozen. Thaw immediately >",
		"x Your account is not yet eligible to apply. Apply Unlock >",
	} {
		t := primitive.NewText(line)
		t.Face = face
		body.AddChild(t)
	}
	errRes.SetBody(body.Children()...)
	secError := demoSection(face, th, "Error",
		"error.tsx：error 状态、操作区和 body 内容区。",
		errRes.Node())

	custom := wire(kit.NewResult())
	custom.SetIconName("star")
	custom.SetTitle("Great, we have done all the operations!")
	custom.SetExtra(action("Next", true))
	secCustom := demoSection(face, th, "自定义 icon",
		"customIcon.tsx：SetIconName 替换默认状态图标。",
		custom.Node())

	// Lifecycle (#9)
	life := wire(kit.NewResult())
	life.SetStatus(kit.ResultInfo)
	life.SetTitle("Lifecycle")
	life.SetSubTitle("structureChange: Status → Title → SubTitle")
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Status → Title → SubTitle；ensureBuilt 懒构建 Flex root。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeResult, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "result-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypeResult); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeResult); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinR := wire(kit.NewResult())
	skinR.SetStatus(kit.ResultSuccess)
	skinR.SetTitle("Skin Result")
	if root, ok := skinR.Node().(*primitive.Decorated); ok {
		root.Base().Key = "result-skin-demo"
	} else if f, ok := skinR.Node().(*primitive.Flex); ok && f != nil {
		f.Base().Key = "result-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Flex.SkinType=kit.Result。Key=result-skin-demo 可命中 Override。",
		skinR.Node())

	page := demoPage(face, "Result 结果",
		"用于反馈一系列操作任务的处理结果。P0 + #9 lifecycle + #6 Skin。",
		secSuccess, secInfo, secWarning, secExceptions, secError, secCustom, secLife, secSkin)
	c.addPage("result", "Result", page)
}
