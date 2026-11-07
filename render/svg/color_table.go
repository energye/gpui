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
	"image/color"
	"strings"

	"github.com/energye/gpui/render"
)

// OpenType SVG-table glyph rendering (OT 1.9.1): one document holds several
// glyph descriptions as elements with id "glyph<N>"; the host renders one
// element with <use> semantics into a coordinate space where user units are
// font design units, y is down, and the origin is the baseline pen position.
// The initial em-square viewport must NOT be clipped, so no viewport mapping
// and no root clipping happen here — the caller owns the transform.

// TableGlyphOptions carries the host bindings for SVG-table glyph rendering.
type TableGlyphOptions struct {
	// CurrentColor resolves currentColor inside the document: the host's
	// text foreground color. Zero keeps the CSS initial (black).
	CurrentColor color.Color

	// Palette provides the CPAL palette colors referenced via
	// var(--colorN, fallback): index 0 maps to --color0, and so on.
	Palette []color.Color

	// ViewportW/ViewportH is the em-square viewport in document user units
	// (the font's unitsPerEm twice) for percentage length resolution.
	ViewportW, ViewportH float64
}

// RenderTableGlyph renders the element with the given id (e.g. "glyph123")
// with <use> semantics per the OpenType SVG-table contract. Returns false
// when the id does not resolve (fail-closed: nothing is drawn).
func (d *Document) RenderTableGlyph(dc *render.Context, id string, opts TableGlyphOptions) bool {
	tgt := d.ElementByID(id)
	if tgt == nil {
		return false
	}
	d.setColorVariables(paletteVarTokens(opts.Palette))
	rootP := rootProps()
	if opts.CurrentColor != nil {
		rootP.color = cssColorToken(opts.CurrentColor)
	}
	st := &renderState{
		doc: d,
		rs:  RefSpace{ViewportW: opts.ViewportW, ViewportH: opts.ViewportH, FontSize: rootP.fontSize},
	}
	// <use> shadow-tree semantics with no use-attributes: the target's own
	// attrs evaluate against the root props.
	renderElement(dc, tgt, rootP, st)
	return true
}

// setColorVariables installs the CSS custom properties (color variables)
// var() references resolve against; nil clears them.
func (d *Document) setColorVariables(vars map[string]string) {
	d.colorVars = vars
}

// paletteVarTokens converts a CPAL palette into --colorN tokens
// (OT 1.9.1 SVG table: names are "--color" + zero-padded-less decimal).
func paletteVarTokens(pal []color.Color) map[string]string {
	if len(pal) == 0 {
		return nil
	}
	vars := make(map[string]string, len(pal))
	for i, c := range pal {
		vars[fmt.Sprintf("--color%d", i)] = cssColorToken(c)
	}
	return vars
}

// cssColorToken formats a color as a "#RRGGBBAA" token parseColor accepts.
func cssColorToken(c color.Color) string {
	r, g, b, a := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x%02x", r>>8, g>>8, b>>8, a>>8)
}

// resolveVarTokens resolves a CSS var() color reference against the
// document's color variables (OT 1.9.1 SVG table palette contract: var()
// only appears as the first item of a color value). Non-var values pass
// through unchanged. An unknown name falls back to the second var()
// argument; no fallback resolves to "" (invalid paint: fail-closed, the
// element paints nothing).
func resolveVarTokens(doc *Document, v string) string {
	trim := strings.TrimSpace(v)
	if len(trim) < 5 || !strings.EqualFold(trim[:4], "var(") {
		return v
	}
	end := strings.LastIndex(trim, ")")
	if end < 0 {
		return v // malformed: leave it for parseColor to reject
	}
	inner := trim[4:end]
	name, fallback := inner, ""
	if comma := strings.Index(inner, ","); comma >= 0 {
		name = inner[:comma]
		fallback = strings.TrimSpace(inner[comma+1:])
	}
	name = strings.TrimSpace(name)
	if doc != nil && doc.colorVars != nil {
		if c, ok := doc.colorVars[name]; ok {
			return c
		}
	}
	return fallback
}
