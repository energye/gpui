//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerNotification() {
	if c.ntfHost == nil {
		c.ntfHost = kit.NewNotification()
	}
	c.ntfHost.SetFace(c.face)
	c.trackTicker(c.ntfHost)

	btn := func(label string, primary bool, fn func()) *kit.Button {
		b := c.trackBtn(kit.NewButton(label))
		if primary {
			b.SetType(kit.ButtonPrimary)
		}
		b.SetOnClick(fn)
		return b
	}

	// ── Hooks 调用（推荐） ──────────────────────────────────────────
	hooksRow := spaceWrap(8,
		btn("topLeft", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title:       "Notification topLeft",
				Description: "Hello, Ant Design!",
				Placement:   kit.NotificationTopLeft,
			})
			*c.status = "notification hooks topLeft"
		}).Node(),
		btn("topRight", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title:       "Notification topRight",
				Description: "Hello, Ant Design!",
				Placement:   kit.NotificationTopRight,
			})
			*c.status = "notification hooks topRight"
		}).Node(),
		btn("bottomLeft", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title:       "Notification bottomLeft",
				Description: "Hello, Ant Design!",
				Placement:   kit.NotificationBottomLeft,
			})
			*c.status = "notification hooks bottomLeft"
		}).Node(),
		btn("bottomRight", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title:       "Notification bottomRight",
				Description: "Hello, Ant Design!",
				Placement:   kit.NotificationBottomRight,
			})
			*c.status = "notification hooks bottomRight"
		}).Node(),
	)

	// ── 自动关闭的延时 ──────────────────────────────────────────────
	duration := btn("Open the notification box", true, func() {
		c.ntfHost.Open(kit.NotificationConfig{
			Title:       "Notification Title",
			Description: "I will never close automatically. This is a purposely very very long description that has many many characters and words.",
			Duration:    0,
			DurationSet: true,
		})
		*c.status = "notification duration=0"
	})

	// ── 带有图标的通知提醒框 ────────────────────────────────────────
	withIcon := spaceWrap(8,
		btn("Success", false, func() {
			c.ntfHost.Success(kit.NotificationConfig{
				Title:       "Notification Title",
				Description: "This is the content of the notification. This is the content of the notification. This is the content of the notification.",
			})
			*c.status = "notification success"
		}).Node(),
		btn("Info", false, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title:       "Notification Title",
				Description: "This is the content of the notification. This is the content of the notification. This is the content of the notification.",
			})
			*c.status = "notification info"
		}).Node(),
		btn("Warning", false, func() {
			c.ntfHost.Warning(kit.NotificationConfig{
				Title:       "Notification Title",
				Description: "This is the content of the notification. This is the content of the notification. This is the content of the notification.",
			})
			*c.status = "notification warning"
		}).Node(),
		btn("Error", false, func() {
			c.ntfHost.Error(kit.NotificationConfig{
				Title:       "Notification Title",
				Description: "This is the content of the notification. This is the content of the notification. This is the content of the notification.",
			})
			*c.status = "notification error"
		}).Node(),
	)

	// ── 自定义按钮 ──────────────────────────────────────────────────
	withBtn := btn("Open the notification box", true, func() {
		key := fmt.Sprintf("open-%d", c.ntfHost.Count()+1)
		c.ntfHost.Open(kit.NotificationConfig{
			Key:   key,
			Title: "Notification Title",
			Description: "A function will be be called after the notification is closed " +
				"(automatically after the \"duration\" time or manually).",
			Duration:    0,
			DurationSet: true,
			Actions: []kit.NotificationAction{
				{Label: "Destroy All", OnClick: func() { c.ntfHost.Destroy() }},
				{Label: "Confirm", Primary: true, OnClick: func() { c.ntfHost.Destroy(key) }},
			},
			OnClose: func() { *c.status = "notification closed" },
		})
		*c.status = "notification with-btn"
	})

	// ── 自定义图标 ──────────────────────────────────────────────────
	customIcon := btn("Open the notification box", true, func() {
		c.ntfHost.Open(kit.NotificationConfig{
			Title:       "Notification Title",
			Description: "This is the content of the notification. This is the content of the notification. This is the content of the notification.",
			IconName:    "star",
			Style:       kit.Style{Text: render.Hex("#108ee9")},
		})
		*c.status = "notification custom-icon"
	})

	// ── 位置 ────────────────────────────────────────────────────────
	placementRow := spaceWrap(8,
		btn("top", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title: "Notification top", Description: "This is the content of the notification.",
				Placement: kit.NotificationTop,
			})
			*c.status = "notification top"
		}).Node(),
		btn("bottom", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title: "Notification bottom", Description: "This is the content of the notification.",
				Placement: kit.NotificationBottom,
			})
			*c.status = "notification bottom"
		}).Node(),
		btn("topLeft", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title: "Notification topLeft", Description: "This is the content of the notification.",
				Placement: kit.NotificationTopLeft,
			})
			*c.status = "notification topLeft"
		}).Node(),
		btn("topRight", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title: "Notification topRight", Description: "This is the content of the notification.",
				Placement: kit.NotificationTopRight,
			})
			*c.status = "notification topRight"
		}).Node(),
		btn("bottomLeft", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title: "Notification bottomLeft", Description: "This is the content of the notification.",
				Placement: kit.NotificationBottomLeft,
			})
			*c.status = "notification bottomLeft"
		}).Node(),
		btn("bottomRight", true, func() {
			c.ntfHost.Info(kit.NotificationConfig{
				Title: "Notification bottomRight", Description: "This is the content of the notification.",
				Placement: kit.NotificationBottomRight,
			})
			*c.status = "notification bottomRight"
		}).Node(),
	)

	// ── 更新消息内容 ────────────────────────────────────────────────
	update := btn("Open the notification box", true, func() {
		const key = "updatable"
		c.ntfHost.Open(kit.NotificationConfig{
			Key: key, Title: "Notification Title", Description: "description.",
		})
		c.ntfHost.Open(kit.NotificationConfig{
			Key: key, Title: "New Title", Description: "New description.",
		})
		*c.status = "notification update"
	})

	// ── 堆叠 ────────────────────────────────────────────────────────
	stackOpen := btn("Open the notification box", true, func() {
		c.ntfHost.SetStack(true)
		c.ntfHost.SetStackThreshold(3)
		c.ntfHost.Open(kit.NotificationConfig{
			Title:       "Notification Title",
			Description: "This is the content of the notification.\nThis is the content of the notification.",
			Duration:    0,
			DurationSet: true,
		})
		*c.status = fmt.Sprintf("notification stack count=%d", c.ntfHost.Count())
	})
	stackDestroy := btn("Destroy all", false, func() {
		c.ntfHost.Destroy()
		*c.status = "notification destroy"
	})
	stackRow := spaceWrap(8, stackOpen.Node(), stackDestroy.Node())

	page := demoPage(c.face, "Notification", "反馈 · docs/antd/notification.md §6 P0",
		demoSection(c.face, c.theme, "Hooks 调用（推荐）",
			"useNotification 主路径：contextHolder 挂 app root；四角 placement 打开 info。",
			hooksRow),
		demoSection(c.face, c.theme, "自动关闭的延时",
			"duration=0 常驻，直到手动 Destroy/关闭。",
			duration.Node()),
		demoSection(c.face, c.theme, "带有图标的通知提醒框",
			"success / info / warning / error 类型图标与 Token 语义色。",
			withIcon),
		demoSection(c.face, c.theme, "自定义按钮",
			"actions：Destroy All / Confirm；onClose 回调。",
			withBtn.Node()),
		demoSection(c.face, c.theme, "自定义图标",
			"IconName + Style.Text 着色（smile → star 占位）。",
			customIcon.Node()),
		demoSection(c.face, c.theme, "位置",
			"placement：top / bottom / topLeft / topRight / bottomLeft / bottomRight。",
			placementRow),
		demoSection(c.face, c.theme, "更新消息内容",
			"同 key open 两次：替换 title/description，不新增。",
			update.Node()),
		demoSection(c.face, c.theme, "堆叠",
			"stack threshold=3；超过阈值折叠为最新一条 +N。",
			stackRow),
	)
	c.addPage("notification", "Notification", page)
}
