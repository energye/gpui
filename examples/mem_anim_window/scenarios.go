//go:build linux && !nogpu

package main

import (
	"log"
	"strings"
)

// scenarioSpec describes one long-soak complex render matrix entry.
// See docs/MEM_ANIM_LONGSOAK_PLAN.md.
type scenarioSpec struct {
	ID          string
	Name        string
	Flags       FeatureFlags
	Stress      bool
	Lite        bool
	Density     int  // extra high-density draws per frame (0=off)
	AllowLowFPS bool // high-density / stress may drop below 60
}

func allScenarios() map[string]scenarioSpec {
	base := func(mods ...string) FeatureFlags {
		f := FeatureFlags{HUD: true, Background: true}
		set := map[string]*bool{
			"glow": &f.GlowOrbs, "cards": &f.Cards, "paths": &f.Paths, "dash": &f.DashStroke,
			"clip": &f.Clip, "layer": &f.Layer, "backdrop": &f.Backdrop, "mask": &f.Mask,
			"image": &f.Image, "text": &f.Text, "filter": &f.Filter, "xform": &f.Transform,
			"blend": &f.Blend, "verts": &f.Vertices, "pixels": &f.Pixels, "poly": &f.Polygon,
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
	}
}

func applyScenario(id string) (scenarioSpec, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if id == "" {
		return scenarioSpec{}, false
	}
	s, ok := allScenarios()[id]
	if !ok {
		log.Printf("unknown GPUI_SCENARIO=%q — use S01..S14", id)
		return scenarioSpec{}, false
	}
	Features = s.Flags
	return s, true
}

func scenarioListOrdered() []scenarioSpec {
	ids := []string{"S01", "S02", "S03", "S04", "S05", "S06", "S07", "S08", "S09", "S10", "S11", "S12", "S13", "S14"}
	m := allScenarios()
	out := make([]scenarioSpec, 0, len(ids))
	for _, id := range ids {
		out = append(out, m[id])
	}
	return out
}
