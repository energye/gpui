//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

// P3.1 linear blend capability scene: three cards baked through the real
// render.Context (GPU accelerator live), each proving one leg of the
// BlendSpace contract. Palette here is the single source of truth shared with
// the pixel probes in main.go (P0.2 rule).
package main

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

const (
	cardW = 340.0
	cardH = 240.0
)

// Single source of truth for both the scene and the pixel assertions.
var (
	bgGray = 0.8 // destination card fill, sRGB-encoded 204
	fgGray = 0.5 // source fill, sRGB-encoded 128

	colCard = [3]float64{0.10, 0.11, 0.14}

	// wantSRGB: |128-204| = 76 — the sRGB-domain Difference result.
	wantSRGB = 76.0
	// wantLinear: decode(0.5)=0.2140, decode(0.8)=0.6038 → |Δ|=0.3898 →
	// encode → 167.6 — the linear-light Difference result (float reference;
	// the GPU dual-tex path is float end to end).
	wantLinear = 167.6
	// wantDegraded: an opaque Normal fill covers with the source bytes in
	// either domain — 128 pins the P3.1 fixed-function degradation contract.
	wantDegraded = 128.0

	tolSRGB     = 2.0 / 255
	tolLinear   = 3.0 / 255
	tolDegraded = 1.0 / 255
)

type p31Card struct {
	img          *render.ImageBuf
	w, h         int
	gpuOps       int
	cpuFallbacks int
}

var (
	bakeOnce sync.Once
	cards    [3]*p31Card
)

func warmGPU() {
	dc := render.NewContext(8, 8)
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})
	dc.SetRGB(0, 1, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	_ = dc.Fill()
	_ = dc.Image()
	_ = dc.Close()
}

// bakeCard runs the draw closure on a real render.Context (GPU accelerator
// registered) and captures the route stats — the same path a widget uses.
func bakeCard(w, h int, draw func(*render.Context)) *p31Card {
	dc := render.NewContext(w, h)
	draw(dc)
	stats := dc.RenderPathStats()
	img := dc.Image()
	_ = dc.Close()
	return &p31Card{
		img:          render.ImageBufFromImage(img),
		w:            w,
		h:            h,
		gpuOps:       stats.GPUOps,
		cpuFallbacks: stats.CPUFallbackOps,
	}
}

func fillCardBase(dc *render.Context) {
	dc.SetRGBA(bgGray, bgGray, bgGray, 1)
	dc.DrawRectangle(0, 0, cardW, cardH)
	_ = dc.Fill()
}

// drawCardA: Difference in the sRGB domain (dual-tex, params.linear=0).
func drawCardA(dc *render.Context) {
	fillCardBase(dc)
	dc.SetRGB(fgGray, fgGray, fgGray)
	dc.SetBlendMode(render.BlendDifference)
	dc.DrawRectangle(16, 24, cardW-32, cardH-40)
	_ = dc.Fill()
}

// drawCardB: the same content with BlendSpaceLinear — the dual-tex shader
// decodes cb/cs to linear light before B(Cb,Cs) and re-encodes co.
func drawCardB(dc *render.Context) {
	fillCardBase(dc)
	dc.SetRGB(fgGray, fgGray, fgGray)
	dc.SetBlendMode(render.BlendDifference)
	dc.SetBlendSpace(render.BlendSpaceLinear)
	dc.DrawRectangle(16, 24, cardW-32, cardH-40)
	_ = dc.Fill()
}

// drawCardC: a linear request on Normal — the fixed-function family degrades
// to the sRGB domain on every backend (P3.1 contract; P3.1b unlocks).
func drawCardC(dc *render.Context) {
	fillCardBase(dc)
	dc.SetRGB(fgGray, fgGray, fgGray)
	dc.SetBlendMode(render.BlendNormal)
	dc.SetBlendSpace(render.BlendSpaceLinear)
	dc.DrawRectangle(16, 24, cardW-32, cardH-40)
	_ = dc.Fill()
}

func ensureBaked() {
	bakeOnce.Do(func() {
		warmGPU()
		cards[0] = bakeCard(int(cardW), int(cardH), drawCardA)
		cards[1] = bakeCard(int(cardW), int(cardH), drawCardB)
		cards[2] = bakeCard(int(cardW), int(cardH), drawCardC)
	})
}

var cardAt = [][2]float64{{8, 60}, {372, 60}, {8, 330}}

func buildScene(shell *wrkit.ShellChrome) {
	drawBaked := func(idx int) func(*rendering.PaintContext, rendering.Size) {
		return func(pc *rendering.PaintContext, _ rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ensureBaked()
			c := cards[idx]
			if c == nil || c.img == nil {
				return
			}
			dc := pc.DC
			dc.SetRGBA(colCard[0], colCard[1], colCard[2], 1)
			dc.DrawRectangle(pc.OriginX, pc.OriginY, cardW, cardH)
			_ = dc.Fill()
			pc.DC.DrawImageEx(c.img, render.DrawImageOptions{
				X:             pc.OriginX + 1,
				Y:             pc.OriginY + 1,
				DstWidth:      cardW - 2,
				DstHeight:     cardH - 2,
				Interpolation: render.InterpNearest,
			})
		}
	}
	for i, title := range []string{
		"A Difference · sRGB 域（dual-tex linear=0）want 76",
		"B Difference · linear 域（dual-tex linear=1）want ≈168",
		"C Normal+Linear · 固定功能显式降级 want 128",
	} {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = drawBaked(i)
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+8)
	}
}

func cardsSummary() string {
	ensureBaked()
	s := ""
	for i, c := range cards {
		if c == nil {
			s += "nil;"
			continue
		}
		s += fmt.Sprintf("card%d gpu_ops=%d cpu_fb=%d;", i, c.gpuOps, c.cpuFallbacks)
	}
	return s
}
