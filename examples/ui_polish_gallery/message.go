//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerMessage() {
	if c.msgHost == nil {
		c.msgHost = kit.NewMessage()
	}
	c.msgHost.SetFace(c.face)
	c.trackTicker(c.msgHost)

	btn := func(label string, primary bool, fn func()) *kit.Button {
		b := c.trackBtn(kit.NewButton(label))
		if primary {
			b.SetType(kit.ButtonPrimary)
		}
		b.SetOnClick(fn)
		return b
	}

	hooks := btn("Display normal message", true, func() {
		c.msgHost.Info("Hello, Ant Design!")
		*c.status = "message hooks"
	})

	otherRow := spaceWrap(8,
		btn("Success", false, func() {
			c.msgHost.Success("This is a success message")
			*c.status = "message success"
		}).Node(),
		btn("Error", false, func() {
			c.msgHost.Error("This is an error message")
			*c.status = "message error"
		}).Node(),
		btn("Warning", false, func() {
			c.msgHost.Warning("This is a warning message")
			*c.status = "message warning"
		}).Node(),
	)

	duration := btn("Customized display duration", false, func() {
		c.msgHost.Success("This is a prompt message for success, and it will disappear in 10 seconds", 10)
		*c.status = "message duration=10"
	})

	stackIndex := 0
	stackOpen := btn("Open the message box", true, func() {
		c.msgHost.SetStack(true)
		c.msgHost.SetStackThreshold(3)
		stackIndex++
		body := fmt.Sprintf("Message %d: This is a stacked message.", stackIndex)
		if stackIndex%2 == 0 {
			body = fmt.Sprintf("Message %d: This is a slightly longer stacked message.", stackIndex)
		}
		c.msgHost.Open(kit.MessageConfig{
			Type:        kit.MessageInfo,
			Content:     body,
			Duration:    0,
			DurationSet: true,
		})
		*c.status = "message stack"
	})
	stackDestroy := btn("Destroy all", false, func() {
		c.msgHost.Destroy()
		*c.status = "message destroy"
	})
	stackRow := spaceWrap(8, stackOpen.Node(), stackDestroy.Node())

	loading := btn("Display a loading indicator", false, func() {
		c.msgHost.Loading("Action in progress..", 0)
		*c.status = "message loading"
	})

	thenable := btn("Display sequential messages", false, func() {
		c.msgHost.Loading("Action in progress..", 2.5).Then(func() {
			c.msgHost.Success("Loading finished", 2.5).Then(func() {
				c.msgHost.Info("Loading finished", 2.5)
			})
		})
		*c.status = "message thenable"
	})

	styleObj := btn("Object style", false, func() {
		c.msgHost.Open(kit.MessageConfig{
			Type:    kit.MessageSuccess,
			Content: "This is a message with object styles",
			Style: kit.Style{
				Background: render.Hex("#f6ffed"),
				Border:     render.Hex("#95de64"),
				Text:       render.Hex("#237804"),
				Radius:     16,
			},
		})
		*c.status = "message object style"
	})
	styleFn := btn("Function style", true, func() {
		c.msgHost.Open(kit.MessageConfig{
			Type:    kit.MessageError,
			Content: "This is a message with function styles",
			Style: kit.Style{
				Background: render.Hex("#fff2f0"),
				Border:     render.Hex("#ffccc7"),
				Text:       render.Hex("#cf1322"),
				Radius:     16,
			},
		})
		*c.status = "message function style"
	})
	styleRow := spaceWrap(8, styleObj.Node(), styleFn.Node())

	update := btn("Open the message box", true, func() {
		c.msgHost.Open(kit.MessageConfig{
			Key:         "updatable",
			Type:        kit.MessageLoading,
			Content:     "Loading...",
			Duration:    0,
			DurationSet: true,
		}).Then(func() {
			c.msgHost.Open(kit.MessageConfig{
				Key:         "updatable",
				Type:        kit.MessageSuccess,
				Content:     "Loaded!",
				Duration:    2,
				DurationSet: true,
			})
		})
		// Trigger the update path directly so the gallery demonstrates key replacement
		// without a separate timer primitive in this page.
		c.msgHost.Open(kit.MessageConfig{
			Key:         "updatable",
			Type:        kit.MessageSuccess,
			Content:     "Loaded!",
			Duration:    2,
			DurationSet: true,
		})
		*c.status = "message update"
	})

	// Lifecycle (#9)
	lifeHost := kit.NewMessage()
	lifeHost.SetFace(c.face)
	if c.theme != nil {
		lifeHost.SetTheme(c.theme)
	}
	lifeHost.SetDuration(3)
	lifeHost.SetTop(64)
	c.trackTicker(lifeHost)
	lifeBtn := btn("Open on lifecycle host", true, func() {
		lifeHost.Info("Lifecycle host message")
		*c.status = "message lifecycle host"
	})
	lifeRow := spaceWrap(8, lifeBtn.Node(), lifeHost.Node())

	// Skin (#6)
	baseSkin := c.theme.Skin
	c.theme.Skin = core.Override(baseSkin, kit.TypeMessage, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if p := baseSkin.Painter(kit.TypeMessage); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeMessage); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinBtn := btn("Open skin message", false, func() {
		c.msgHost.Success("Item Decorated SkinType=kit.Message")
		*c.status = "message skin demo"
	})

	page := demoPage(c.face, "Message", "反馈 · docs/antd/message.md §6 P0",
		demoSection(c.face, c.theme, "Hooks 调用（推荐）",
			"message.useMessage 主路径：contextHolder 挂在 app root，按钮触发 info。",
			hooks.Node()),
		demoSection(c.face, c.theme, "其他提示类型",
			"success / error / warning 类型图标与 Token 语义色。",
			otherRow),
		demoSection(c.face, c.theme, "修改延时",
			"duration=10 秒，自动关闭由 Message Ticker 驱动。",
			duration.Node()),
		demoSection(c.face, c.theme, "堆叠",
			"stack threshold=3；超过阈值后折叠展示最新消息。",
			stackRow),
		demoSection(c.face, c.theme, "加载中",
			"loading 类型使用 Canvas spinner，duration=0 常驻直到 Destroy。",
			loading.Node()),
		demoSection(c.face, c.theme, "Promise 接口",
			"MessageHandle.Then 映射 thenable 主路径。",
			thenable.Node()),
		demoSection(c.face, c.theme, "自定义语义结构样式",
			"浅 Style 覆盖 root/icon/title 的主视觉；semantic 深度见 coverage P1。",
			styleRow),
		demoSection(c.face, c.theme, "更新消息内容",
			"同 key=open 后替换内容与类型，仍保持一条消息。",
			update.Node()),
		demoSection(c.face, c.theme, "Lifecycle (#9)",
			"独立 host：structureChange（Duration/Top）后 ensureBuilt 懒构建 Portal。",
			lifeRow),
		demoSection(c.face, c.theme, "Skin painter (#6)",
			"消息项 Decorated.SkinType=kit.Message 已注册；Override 委托 base painter。",
			skinBtn.Node()),
	)

	c.addPage("message", "Message", page)
}
