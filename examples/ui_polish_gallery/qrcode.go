//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerQRCode() {
	qr := kit.NewQRCode("gpui")
	c.add("qrcode", "QRCode", "Data Display · QRCode", qr.Node())
}
