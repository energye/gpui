//go:build linux && !nogpu

package main

import "github.com/energye/gpui/ui/kit"

// TextArea is covered on the Input page (antd Input.TextArea demos).
// Keep a thin tab that deep-links the same control family for catalog completeness.
func (c *catalogCtx) registerTextArea() {
	face, th := c.face, c.theme
	ta := kit.NewTextArea("See Input page for full TextArea demos (rows / maxLength / autoSize).", 4)
	ta.SetFace(face)
	if th != nil {
		ta.SetTheme(th)
	}
	ta.SetFixedSize(420, 0)
	*c.tickers = append(*c.tickers, ta)
	c.add("textarea", "TextArea",
		"Data Entry · TextArea — full demos on the Input page (textarea.tsx / autosize-textarea.tsx).",
		ta.Node())
}
