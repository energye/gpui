//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTour() {
	tour := kit.NewTour(kit.TourStep{Title: "Step1", Body: "demo"})
	c.add("tour", "Tour", "Data Display · Tour", tour.Node())
}
