//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

// Package svg renders static SVG documents on top of
// [github.com/energye/gpui/render.Context].
//
// The semantic anchor is Skia's SVG module (SkSVGDOM): behavior questions
// (preserveAspectRatio, units, inheritance, reference resolution) follow
// its answers, with the resvg support matrix used as a coverage checklist.
// P3.8 batch 1 (evidence/P3.8/) upgraded the former icon subset to the
// document-semantics base; paint servers (gradients/patterns), clipPath,
// mask, text and filters land in the following batches.
//
// # Supported SVG Elements
//
//   - <svg> (root and nested: new viewport with viewBox,
//     preserveAspectRatio, overflow) with width, height
//   - <path> with d attribute (full SVG path command set)
//   - <circle> cx, cy, r
//   - <rect> x, y, width, height, rx, ry
//   - <ellipse> cx, cy, rx, ry
//   - <line> x1, y1, x2, y2
//   - <polygon> points
//   - <polyline> points
//   - <g> grouping with inherited properties and transform
//   - <defs> reference container (collected, never rendered directly)
//   - <symbol> template instantiated by <use> (own viewBox viewport)
//   - <use> href/xlink:href instantiation (x, y, width, height)
//   - <style> minimal selector set (tag and single-class rules)
//
// # Supported Attributes
//
//   - Paint: fill, stroke (color | none | currentColor), color;
//     url(#id) references are validated — an unresolved or not-yet-supported
//     target paints nothing and warns (never falls back to black)
//   - Opacity: opacity (element-level), fill-opacity, stroke-opacity
//   - Fill rule: fill-rule (clip-rule carried until clipPath lands)
//   - Stroke: stroke-width, stroke-linecap, stroke-linejoin,
//     stroke-miterlimit, stroke-dasharray (odd lists repeated), stroke-dashoffset
//   - Geometry units: px/pt/pc/in/cm/mm/q, em/ex (inherited font-size),
//     % (viewport axis: x/width→width, y/height→height, radii→normalized
//     diagonal) — see [Length]
//   - Text/units state: font-size (relative units resolve against the
//     inherited value)
//   - transform (translate, rotate, scale, matrix, skewX, skewY)
//   - class + style attributes and <style> rules, cascaded presentation
//     attribute < tag rule < class rule < inline style
//   - preserveAspectRatio (nine alignments × meet/slice, default
//     xMidYMid meet) and overflow (visible disables the viewport clip)
//
// # Dimensions
//
// Root width/height percentages resolve against the viewBox (legacy root
// convention). Child geometry lengths are full [Length] values resolved
// against the referencing viewport.
//
// # Not supported (named refusal)
//
// Animation (SMIL), script, foreignObject, cross-file use references, and
// elements whose batches have not landed (text, filters, gradients,
// clipPath, mask, image, pattern) are skipped with a warning in
// [Document.Warnings]; the affected paints render nothing (fail-closed).
// Parse still succeeds — fault-tolerant rendering, resvg alike; strict
// callers inspect [Document.Warnings].
//
// # Usage
//
//	img, err := svg.Render(svgBytes, 64, 64)
//
//	// With color override (theming):
//	img, err := svg.RenderWithColor(svgBytes, 64, 64, color.White)
//
//	// Parse once, render many times:
//	doc, err := svg.Parse(svgBytes)
//	img1 := doc.Render(16, 16)
//	img2 := doc.Render(32, 32)
//	img3 := doc.RenderWithColor(24, 24, themeColor)
package svg
