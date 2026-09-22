package prim

import (
	"strings"
	"sync"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Official antd icon library (1:1): all 848 icons from
// @ant-design/icons-svg 4.6.0, viewBox 64 64 896 896.
// Data lives in icon_antd_paths.go (generated); this file parses
// once, caches, and paints through rendering.FillPath.

var (
	antdIndexOnce sync.Once
	antdIndex     map[string]*IconAntdEntry
	antdParseOnce sync.Once
	antdParsed    map[string][]antdParsedPath
)

type antdParsedPath struct {
	path *render.Path
	fill string
}

func buildAntdIndex() {
	antdIndex = make(map[string]*IconAntdEntry, len(IconAntdEntries))
	for i := range IconAntdEntries {
		e := &IconAntdEntries[i]
		antdIndex[e.Name+"|"+e.Theme] = e
	}
}

// AntdIconThemes lists the three official themes.
func AntdIconThemes() []string { return []string{"outlined", "filled", "twotone"} }

// AntdIconCount reports the official total (848).
func AntdIconCount() int { return len(IconAntdEntries) }

// LookupAntdIcon finds name+theme (theme case-insensitive, default outlined).
func LookupAntdIcon(name, iconTheme string) *IconAntdEntry {
	antdIndexOnce.Do(buildAntdIndex)
	th := strings.ToLower(strings.TrimSpace(iconTheme))
	if th == "" {
		th = "outlined"
	}
	if e, ok := antdIndex[name+"|"+th]; ok {
		return e
	}
	return nil
}

// IsKnownAntdIcon reports official coverage for name+theme.
func IsKnownAntdIcon(name, iconTheme string) bool { return LookupAntdIcon(name, iconTheme) != nil }

// AntdIconNames returns all official keys (Name|Theme) sorted by file order.
func AntdIconNames() []string {
	out := make([]string, 0, len(IconAntdEntries))
	for i := range IconAntdEntries {
		out = append(out, IconAntdEntries[i].Name+"|"+IconAntdEntries[i].Theme)
	}
	return out
}

func parsedAntdPaths() map[string][]antdParsedPath {
	antdParseOnce.Do(func() {
		antdIndexOnce.Do(buildAntdIndex)
		antdParsed = make(map[string][]antdParsedPath, len(IconAntdEntries))
		for i := range IconAntdEntries {
			e := &IconAntdEntries[i]
			key := e.Name + "|" + e.Theme
			var list []antdParsedPath
			for _, p := range e.Paths {
				parsed, err := render.ParseSVGPath(p.D)
				if err != nil || parsed == nil {
					continue
				}
				list = append(list, antdParsedPath{path: parsed, fill: p.Fill})
			}
			if len(list) > 0 {
				antdParsed[key] = list
			}
		}
	})
	return antdParsed
}

// PaintAntdIcon draws the official glyph in a size x size local box.
// Returns false when the icon is unknown (caller falls back).
// Colors: no-fill paths use main; primaryColor->main, secondaryColor->second.
//
// NOTE (E6 atlas attempt, reverted 2026-09-21): a pixel-level glyph atlas
// (rasterize once to ImageBuf, blit thereafter) FAILED pixel-identity:
// offscreen raster bytes are bit-correct, but the DrawImage blit shifts
// edge RGB (68->42 at equal alpha) — the FillPath-write and DrawImage-write
// paths disagree on alpha/gamma conventions somewhere inside the image
// pipeline. Fixing that is engine-wide image surgery, disproportionate to
// the need (human-speed scroll is already green, see window README).
// Kept: direct vector paint (correct everywhere). If the image pipeline
// conventions are ever unified, reintroduce the atlas behind this door.
func PaintAntdIcon(pc *rendering.PaintContext, size float64, name, iconTheme string, main, second theme.Color, hasSecond bool) bool {
	if pc == nil || size <= 0 {
		return false
	}
	th := strings.ToLower(strings.TrimSpace(iconTheme))
	if th == "" {
		th = "outlined"
	}
	return paintAntdVector(pc, size, name, th, main, second, hasSecond)
}

// paintAntdVector fills the parsed SVG paths through rendering.FillPath.
func paintAntdVector(pc *rendering.PaintContext, size float64, name, th string, main, second theme.Color, hasSecond bool) bool {
	list, ok := parsedAntdPaths()[name+"|"+th]
	if !ok || len(list) == 0 {
		return false
	}
	scale := size / 896.0
	base := render.Translate(0, 0).Multiply(render.Scale(scale, scale)).Multiply(render.Translate(-64, -64))
	for _, pp := range list {
		var r, g, b, a float64
		switch pp.fill {
		case "primaryColor":
			r, g, b, a = main.R, main.G, main.B, main.A
		case "secondaryColor":
			if hasSecond {
				r, g, b, a = second.R, second.G, second.B, second.A
			} else {
				r, g, b, a = main.R, main.G, main.B, 0.15
			}
		default:
			r, g, b, a = main.R, main.G, main.B, main.A
		}
		if a <= 0 {
			continue
		}
		tp := pp.path.Transform(base)
		rendering.FillPath(pc, tp, r, g, b, a)
	}
	return true
}
