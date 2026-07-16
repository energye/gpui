//go:build linux && !nogpu

package main

import (
	"fmt"
	"os"
	"strings"
)

// featureConfig is one isolatable stress configuration.
// Env overrides always win over tier/probe defaults so you can bisect issues.
type featureConfig struct {
	Tier         string
	NameCN       string
	ParticleN    int
	Region       float64 // non-fullscreen fraction of window for particle stage
	Solid        bool
	Blend        bool
	Glow         bool
	Mesh         bool
	Atlas        bool
	Text         bool
	Layer        bool
	Trails       bool
	BlendCircles int // advanced-blend circle count (0=auto)
	AllowLowFPS  bool
	Expect       string

	// Isolation / matrix fields
	ProbeID         string
	ProbeClass      string // gate | stress | trap
	PerCircleBlend  bool   // regression trap: per-circle SetBlendMode
	ResizeOscillate bool   // recreate surface size every N frames
	PathSubmitHeavy bool   // many path fills, skip dense base mesh
	MultiLayer      int    // nested translucent layers
	AltClear        bool   // alternate clear color each frame
	GrowN           bool   // grow particle count over time
	MaxCPUPct       float64
	MaxJitter       float64 // if >0, FAIL when fps_max-fps_min exceeds (steady)
	MinParticleN    int
	MemSoakSec      int
	BisectHint      string

	// Dig axes: Skia-facing paths that historically hide correctness/perf bugs.
	Clip     bool // nested ClipRect / ClipPath
	Grad     bool // linear/radial/sweep gradient RT tiles
	Filter   bool // blur / drop-shadow filter tiles
	Dash     bool // dashed strokes
	EvenOdd  bool // EvenOdd vs NonZero fill rule
	Xform    bool // transform stack churn
	MeshWave bool // dense wavy vertex mesh (tessellation quality)
	ImagePx  bool // ImageBuf write + DrawImage
	TextBi   bool // bilingual CJK+Latin text glyph sample
	BlendSep bool // separable blend panel (Multiply/Screen/Overlay RT)
}

func tierDefaults(tier string) featureConfig {
	tier = strings.ToUpper(strings.TrimSpace(tier))
	switch tier {
	case "", "L0", "BASE":
		return featureConfig{
			Tier: "L0", NameCN: "基线实心粒子",
			ParticleN: 500, Region: 0.65,
			Solid:  true,
			Expect: "非全屏舞台内彩色实心粒子运动；应稳定≈60fps，cpu_fb=0",
		}
	case "L1", "ALPHA":
		return featureConfig{
			Tier: "L1", NameCN: "半透明叠压",
			ParticleN: 900, Region: 0.65,
			Solid: true, Blend: true,
			Expect: "全舞台彩色粒子+半透明叠压；Screen/Multiply 层批高级混合",
		}
	case "L2", "GLOW":
		return featureConfig{
			Tier: "L2", NameCN: "光晕有界RT",
			ParticleN: 1200, Region: 0.65,
			Solid: true, Blend: true, Glow: true,
			Expect: "粒子+有界 glow RT（隔帧 Export）；查滤镜/读回成本",
		}
	case "L3", "UI":
		return featureConfig{
			Tier: "L3", NameCN: "文本+图集+层",
			ParticleN: 1600, Region: 0.68,
			Solid: true, Blend: true, Glow: true, Mesh: true, Atlas: true, Text: true, Layer: true,
			Expect: "在 L2 上叠加 mesh 批、atlas 精灵、半透明层与中文说明",
		}
	case "L4", "SINK", "KITCHEN":
		return featureConfig{
			Tier: "L4", NameCN: "厨房水槽全开",
			ParticleN: 3000, Region: 0.72,
			Solid: true, Blend: true, Glow: true, Mesh: true, Atlas: true, Text: true, Layer: true, Trails: true,
			AllowLowFPS: false,
			Expect:      "非全屏厨房水槽：粒子/混合/光晕/mesh/atlas/层/拖尾全开，用于挖提交与内存问题",
		}
	default:
		// Maybe a probe id used as tier.
		if p, ok := probeByID(tier); ok {
			return applyProbe(featureConfig{}, p)
		}
		return tierDefaults("L0")
	}
}

func loadConfig() featureConfig {
	// Explicit probe wins over tier.
	if id := envString("GPUI_PROBE", ""); id != "" {
		if p, ok := probeByID(id); ok {
			cfg := applyProbe(featureConfig{}, p)
			return applyEnvOverrides(cfg)
		}
		// fall through with tier = id
	}

	tier := envString("GPUI_TIER", envString("GPUI_SCENARIO", "L0"))
	cfg := tierDefaults(tier)
	if cfg.ProbeID == "" {
		cfg.ProbeID = cfg.Tier
	}
	if cfg.ProbeClass == "" {
		// L0/L1 gate, L2+ stress by default
		switch cfg.Tier {
		case "L0", "L1":
			cfg.ProbeClass = string(classGate)
		default:
			if strings.HasPrefix(cfg.Tier, "P_") {
				// already set by applyProbe
			} else {
				cfg.ProbeClass = string(classStress)
			}
		}
	}
	return applyEnvOverrides(cfg)
}

func applyEnvOverrides(cfg featureConfig) featureConfig {
	if v := envInt("GPUI_PARTICLE_N", -1); v > 0 {
		cfg.ParticleN = v
	}
	if v := envFloat("GPUI_REGION", -1); v > 0.2 && v <= 1.0 {
		cfg.Region = v
	}

	apply := func(key string, cur *bool) {
		if _, ok := os.LookupEnv(key); ok {
			*cur = envBool(key, *cur)
		}
	}
	apply("GPUI_ENABLE_SOLID", &cfg.Solid)
	apply("GPUI_ENABLE_BLEND", &cfg.Blend)
	apply("GPUI_ENABLE_GLOW", &cfg.Glow)
	apply("GPUI_ENABLE_MESH", &cfg.Mesh)
	apply("GPUI_ENABLE_ATLAS", &cfg.Atlas)
	apply("GPUI_ENABLE_TEXT", &cfg.Text)
	apply("GPUI_ENABLE_LAYER", &cfg.Layer)
	apply("GPUI_ENABLE_TRAILS", &cfg.Trails)
	apply("GPUI_ENABLE_CLIP", &cfg.Clip)
	apply("GPUI_ENABLE_GRAD", &cfg.Grad)
	apply("GPUI_ENABLE_FILTER", &cfg.Filter)
	apply("GPUI_ENABLE_DASH", &cfg.Dash)
	apply("GPUI_ENABLE_EVENODD", &cfg.EvenOdd)
	apply("GPUI_ENABLE_XFORM", &cfg.Xform)
	apply("GPUI_ENABLE_MESH_WAVE", &cfg.MeshWave)
	apply("GPUI_ENABLE_IMAGE_PX", &cfg.ImagePx)
	apply("GPUI_ENABLE_TEXT_BI", &cfg.TextBi)
	apply("GPUI_ENABLE_BLEND_SEP", &cfg.BlendSep)
	if _, ok := os.LookupEnv("GPUI_ENABLE_PER_CIRCLE_BLEND"); ok {
		cfg.PerCircleBlend = envBool("GPUI_ENABLE_PER_CIRCLE_BLEND", cfg.PerCircleBlend)
	}
	if _, ok := os.LookupEnv("GPUI_ENABLE_RESIZE"); ok {
		cfg.ResizeOscillate = envBool("GPUI_ENABLE_RESIZE", cfg.ResizeOscillate)
	}
	if _, ok := os.LookupEnv("GPUI_ENABLE_PATH_SUBMIT"); ok {
		cfg.PathSubmitHeavy = envBool("GPUI_ENABLE_PATH_SUBMIT", cfg.PathSubmitHeavy)
	}
	if _, ok := os.LookupEnv("GPUI_ENABLE_ALT_CLEAR"); ok {
		cfg.AltClear = envBool("GPUI_ENABLE_ALT_CLEAR", cfg.AltClear)
	}
	if _, ok := os.LookupEnv("GPUI_ENABLE_GROW_N"); ok {
		cfg.GrowN = envBool("GPUI_ENABLE_GROW_N", cfg.GrowN)
	}
	if v := envInt("GPUI_MULTI_LAYER", -1); v >= 0 {
		cfg.MultiLayer = v
	}
	if v := envFloat("GPUI_MAX_CPU", -1); v > 0 {
		cfg.MaxCPUPct = v
	}

	if _, ok := os.LookupEnv("GPUI_ALLOW_LOW_FPS"); ok {
		cfg.AllowLowFPS = envBool("GPUI_ALLOW_LOW_FPS", cfg.AllowLowFPS)
	}
	if cfg.ParticleN < 10 {
		cfg.ParticleN = 10
	}
	if cfg.ParticleN > 50000 {
		cfg.ParticleN = 50000
	}
	// Density floor: never silently gut content below probe MinN via env mistakes
	// (explicit GPUI_PARTICLE_N still wins if user set it above).
	if cfg.MinParticleN > 0 && envInt("GPUI_PARTICLE_N", -1) < 0 && cfg.ParticleN < cfg.MinParticleN {
		cfg.ParticleN = cfg.MinParticleN
	}
	// at least something drawable
	if !cfg.Solid && !cfg.Mesh && !cfg.Atlas {
		cfg.Solid = true
	}
	if v := envInt("GPUI_BLEND_CIRCLES", -1); v >= 0 {
		cfg.BlendCircles = v
	} else if cfg.BlendCircles <= 0 {
		if cfg.Blend {
			cfg.BlendCircles = 96
		} else {
			cfg.BlendCircles = 0
		}
	}
	return cfg
}

func (c featureConfig) featuresSummary() string {
	on := make([]string, 0, 10)
	if c.Solid {
		on = append(on, "solid")
	}
	if c.Blend {
		if c.PerCircleBlend {
			on = append(on, "blend_per_circle")
		} else {
			on = append(on, "blend")
		}
	}
	if c.Glow {
		on = append(on, "glow")
	}
	if c.Mesh {
		on = append(on, "mesh")
	}
	if c.Atlas {
		on = append(on, "atlas")
	}
	if c.Text {
		on = append(on, "text")
	}
	if c.Layer {
		on = append(on, "layer")
	}
	if c.Trails {
		on = append(on, "trails")
	}
	if c.ResizeOscillate {
		on = append(on, "resize")
	}
	if c.PathSubmitHeavy {
		on = append(on, "path_submit")
	}
	if c.MultiLayer > 1 {
		on = append(on, fmt.Sprintf("layers=%d", c.MultiLayer))
	}
	if c.AltClear {
		on = append(on, "alt_clear")
	}
	if c.GrowN {
		on = append(on, "grow_n")
	}
	if c.Clip {
		on = append(on, "clip")
	}
	if c.Grad {
		on = append(on, "grad")
	}
	if c.Filter {
		on = append(on, "filter")
	}
	if c.Dash {
		on = append(on, "dash")
	}
	if c.EvenOdd {
		on = append(on, "evenodd")
	}
	if c.Xform {
		on = append(on, "xform")
	}
	if c.MeshWave {
		on = append(on, "mesh_wave")
	}
	if c.ImagePx {
		on = append(on, "image_px")
	}
	if c.TextBi {
		on = append(on, "text_bi")
	}
	if c.BlendSep {
		on = append(on, "blend_sep")
	}
	if c.MaxJitter > 0 {
		on = append(on, fmt.Sprintf("jit<%.0f", c.MaxJitter))
	}
	if len(on) == 0 {
		return "none"
	}
	return strings.Join(on, ",")
}

func (c featureConfig) String() string {
	id := c.ProbeID
	if id == "" {
		id = c.Tier
	}
	return fmt.Sprintf("%s n=%d region=%.2f feats=%s class=%s", id, c.ParticleN, c.Region, c.featuresSummary(), c.ProbeClass)
}
