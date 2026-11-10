//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package svg

import (
	"fmt"
	"image"
	"image/color"

	"github.com/energye/gpui/render"
)

// Render parses SVG XML data and renders it to an RGBA image at the specified size.
// The SVG viewBox is scaled to fit the target width and height.
func Render(data []byte, width, height int) (*image.RGBA, error) {
	doc, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return doc.Render(width, height), nil
}

// RenderWithColor parses SVG XML data and renders it with all fill and stroke
// colors replaced by the given color. This is useful for theming icon SVGs.
// Colors set to "none" are preserved (transparent areas stay transparent).
func RenderWithColor(data []byte, width, height int, c color.Color) (*image.RGBA, error) {
	doc, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return doc.RenderWithColor(width, height, c), nil
}

// Render renders the document to an RGBA image at the specified size.
// The SVG viewBox is scaled to fit the target dimensions.
func (d *Document) Render(width, height int) *image.RGBA {
	// 批 4f：SVG 烘焙是离屏 CPU 场景——WithCPUOnly 钉死软件光栅化（含
	// CPU SDF 解析覆盖），烘焙结果不再随进程的 GPU 会话漂移（窗口进程与
	// 纯 CPU 进程的烘焙逐位一致，conformance/resvg 对比结论对生产生效）。
	dc := render.NewContext(width, height, render.WithCPUOnly())
	defer dc.Close()
	d.RenderTo(dc, 0, 0, float64(width), float64(height))
	return dc.Image().(*image.RGBA)
}

// RenderWithColor renders the document with all fill and stroke colors
// replaced by the given color. Colors set to "none" are preserved.
func (d *Document) RenderWithColor(width, height int, c color.Color) *image.RGBA {
	dc := newSVGOffscreen(width, height)
	defer dc.Close()
	d.RenderToWithColor(dc, 0, 0, float64(width), float64(height), c)
	return dc.Image().(*image.RGBA)
}

// RenderTo renders the document into an existing [render.Context] at the specified
// position and size. The SVG viewBox is scaled to fit (x, y, width, height).
func (d *Document) RenderTo(dc *render.Context, x, y, width, height float64) {
	d.renderInternal(dc, x, y, width, height, nil)
}

// RenderToWithColor renders the document into an existing [render.Context] with
// all non-"none" colors replaced by the given override color.
func (d *Document) RenderToWithColor(dc *render.Context, x, y, width, height float64, c color.Color) {
	d.renderInternal(dc, x, y, width, height, c)
}

// renderInternal is the shared rendering implementation.
func (d *Document) renderInternal(dc *render.Context, x, y, width, height float64, overrideColor color.Color) {
	if !d.hasViewBox && !d.hasSize {
		// Root <svg> with no width/height and no viewBox: size to the
		// content bounding box (usvg calculate_svg_bbox after
		// restore_viewbox — the 100% fallback resolves against the render
		// target, then the tree bbox redefines the size).
		if r, ok := d.measureContentBBox(); ok {
			d.ViewBox = ViewBox{MinX: 0, MinY: 0, Width: r.Max.X, Height: r.Max.Y}
			d.hasViewBox = true
		}
	}
	if d.hasViewBox && (d.ViewBox.Width <= 0 || d.ViewBox.Height <= 0) {
		// Declared but degenerate viewBox: nothing sensible to map.
		return
	}

	dc.Push()

	// Position at (x, y).
	if x != 0 || y != 0 {
		dc.Translate(x, y)
	}

	// Root viewport clip (overflow:hidden default); slice content outside
	// the viewport must not leak.
	if !d.OverflowVisible {
		dc.ClipRect(0, 0, width, height)
	}

	// Map the viewBox into the viewport with preserveAspectRatio semantics
	// (uniform scale + alignment). Without a viewBox, user units are px and
	// no mapping applies (SVG spec).
	if d.hasViewBox {
		dc.Transform(viewportTransform(width, height, d.ViewBox, d.Par))
	}

	rootP := evalProps(rootProps(), d.rootAttrs, d)
	// Percentages resolve against the viewBox when present (usvg
	// units.rs: state.view_box), not the render canvas.
	pw, ph := width, height
	if d.hasViewBox {
		pw, ph = d.ViewBox.Width, d.ViewBox.Height
	}
	state := &renderState{
		doc:           d,
		overrideColor: overrideColor,
		rs:            RefSpace{ViewportW: pw, ViewportH: ph, FontSize: rootP.fontSize},
		root:          rootP,
	}

	renderElements(dc, d.Elements, rootP, state)

	dc.Pop()
}

// String returns a short description of the document for debugging.
func (d *Document) String() string {
	return fmt.Sprintf("SVG(viewBox=%.0f,%.0f,%.0f,%.0f elements=%d)",
		d.ViewBox.MinX, d.ViewBox.MinY, d.ViewBox.Width, d.ViewBox.Height,
		len(d.Elements))
}
