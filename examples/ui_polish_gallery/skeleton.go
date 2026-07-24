//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSkeleton() {
	sk := kit.NewSkeleton(200, 16)
	sk.SetActive(true)
	*c.tickers = append(*c.tickers, sk)
	c.add("skeleton", "Skeleton", "Feedback · Skeleton", sk.Node())
}
