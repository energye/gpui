//go:build linux && !nogpu

package main

import (
	"log"
	"strings"
)

// scenarioSpec describes one long-soak complex render matrix entry.
// See docs/MEM_ANIM_LONGSOAK_PLAN.md.
type scenarioSpec struct {
	ID            string
	Name          string
	Flags         FeatureFlags
	Stress        bool
	Lite          bool
	Density       int  // extra high-density draws per frame (0=off)
	AllowLowFPS   bool // high-density / stress may drop below 60
	DamagePresent bool // use PresentFrameAuto + partial redraw (S19)
}

func allScenarios() map[string]scenarioSpec {
	base := func(mods ...string) FeatureFlags {
		f := FeatureFlags{HUD: true, Background: true}
		set := map[string]*bool{
			"glow": &f.GlowOrbs, "cards": &f.Cards, "paths": &f.Paths, "dash": &f.DashStroke,
			"clip": &f.Clip, "layer": &f.Layer, "backdrop": &f.Backdrop, "mask": &f.Mask,
			"image": &f.Image, "text": &f.Text, "filter": &f.Filter, "xform": &f.Transform,
			"blend": &f.Blend, "verts": &f.Vertices, "pixels": &f.Pixels, "poly": &f.Polygon,
			"grad": &f.Gradient, "pattern": &f.Pattern, "advblend": &f.AdvBlend,
			"rrectclip": &f.RRectClip, "textlcd": &f.TextLCD, "damage": &f.Damage,
			"scroll": &f.ScrollUI, "mesh3d": &f.Mesh3D,
		}
		for _, m := range mods {
			if p, ok := set[m]; ok {
				*p = true
			}
		}
		return f
	}
	all := FeatureFlags{
		Background: true, GlowOrbs: true, Cards: true, Paths: true, DashStroke: true,
		Clip: true, Layer: true, Backdrop: true, Mask: true, Image: true, Text: true,
		Filter: true, Transform: true, Blend: true, Vertices: true, Pixels: true, Polygon: true, HUD: true,
	}
	allPlus3D := all
	allPlus3D.Mesh3D = true
	// Gap modules combined (S21). Intentionally excludes Damage present-path so
	// continuous full present still validates gap APIs without Idle/flash risk.
	// Keep to gap APIs only (no layer/cards/text already covered by S01–S14).
	gapAll := base("grad", "pattern", "advblend", "rrectclip", "textlcd", "scroll")
	return map[string]scenarioSpec{
		"S01": {ID: "S01", Name: "BaselineHUD", Flags: base()},
		"S02": {ID: "S02", Name: "GlowField", Flags: base("glow")},
		"S03": {ID: "S03", Name: "CardUI", Flags: base("cards", "text")},
		"S04": {ID: "S04", Name: "PathDash", Flags: base("paths", "dash", "poly")},
		"S05": {ID: "S05", Name: "ClipStack", Flags: base("clip", "glow")},
		"S06": {ID: "S06", Name: "LayerOpacity", Flags: base("layer", "text")},
		"S07": {ID: "S07", Name: "BackdropPanel", Flags: base("glow", "backdrop", "text")},
		"S08": {ID: "S08", Name: "ImageMaskAtlas", Flags: base("image", "mask")},
		"S09": {ID: "S09", Name: "TextStyles", Flags: base("text")},
		"S10": {ID: "S10", Name: "FilterFX", Flags: base("filter", "cards")},
		"S11": {ID: "S11", Name: "MeshBlendXform", Flags: base("verts", "blend", "xform")},
		"S12": {ID: "S12", Name: "FullComposite", Flags: all},
		"S13": {ID: "S13", Name: "HighDensity", Flags: base("glow", "paths", "poly", "text"), Density: 1200, AllowLowFPS: true},
		"S14": {ID: "S14", Name: "StressEveryFrame", Flags: all, Stress: true, AllowLowFPS: true},
		// --- Skia gap extensions (S15+) ---
		"S15": {ID: "S15", Name: "GradientPattern", Flags: base("grad", "pattern")},
		"S16": {ID: "S16", Name: "AdvancedBlend", Flags: base("advblend")},
		"S17": {ID: "S17", Name: "ClipRRectEvenOdd", Flags: base("rrectclip", "paths", "dash", "text")},
		"S18": {ID: "S18", Name: "TextLCDShape", Flags: base("textlcd")},
		"S19": {ID: "S19", Name: "DamagePartialPresent", Flags: FeatureFlags{HUD: true, Damage: true, Text: true, ScrollUI: true}, DamagePresent: true},
		"S20": {ID: "S20", Name: "ScrollModalUI", Flags: base("scroll", "layer", "text", "cards")},
		"S21": {ID: "S21", Name: "SkiaGapComposite", Flags: gapAll, Lite: true, AllowLowFPS: true},
		// 3D gradient / rotation pressure (Skia-class mesh animation gate)
		"S22": {ID: "S22", Name: "Mesh3DGradient", Flags: base("mesh3d", "text"), Density: 0},
		"S23": {ID: "S23", Name: "Mesh3DFullComposite", Flags: allPlus3D, Lite: false, AllowLowFPS: false},
	}
}

func applyScenario(id string) (scenarioSpec, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if id == "" {
		return scenarioSpec{}, false
	}
	s, ok := allScenarios()[id]
	if !ok {
		log.Printf("unknown GPUI_SCENARIO=%q — use S01..S23", id)
		return scenarioSpec{}, false
	}
	Features = s.Flags
	return s, true
}

func scenarioListOrdered() []scenarioSpec {
	ids := []string{
		"S01", "S02", "S03", "S04", "S05", "S06", "S07", "S08", "S09", "S10",
		"S11", "S12", "S13", "S14", "S15", "S16", "S17", "S18", "S19", "S20", "S21", "S22", "S23",
	}
	m := allScenarios()
	out := make([]scenarioSpec, 0, len(ids))
	for _, id := range ids {
		out = append(out, m[id])
	}
	return out
}
