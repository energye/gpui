//go:build linux && !nogpu

// Package main — api_coverage_app
//
// Goal: exercise **every** public render.Context API inside realistic product
// scenes (not UI cosplay). Tracks coverage, fails if any API never runs.
// Also runs host lifecycle (minimize / ForceRecoverHealthy) to reproduce
// device-lost / OOM / present issues under full API load.
//
// Scenes (Skia / Flutter / Ant Design host duties mapped to API clusters):
//
//	G_STATE     paint state, transform stack, stroke/dash/fill rule
//	G_PATH      path builders + SVG path + boolean
//	G_SHAPE     rect/rrect/circle/ellipse/arc/poly/point/line
//	G_BRUSH     linear/radial/sweep/custom/pattern brushes
//	G_CLIP      clip rect/path/rrect, ClipOp, mask
//	G_LAYER     PushLayer / Backdrop / MaskLayer / blend modes
//	G_FILTER    Blur/Shadow/Grayscale/Invert/ColorMatrix/FilterGraph
//	G_IMAGE     DrawImage* / Nine / Quad / Atlas / GPUTexture / Export
//	G_TEXT      fonts, string, wrap, stroke string, shaped glyphs, text path
//	G_MESH      vertices, mesh, write pixels, set pixel
//	G_FRAME     damage, present full/auto/damage, flush GPU, shared encoder
//	G_IO        EncodePNG/JPEG, SavePNG, Image, AsMask
//
//	GPUI_ANIM_SECONDS=20 go run ./examples/api_coverage_app
//	GPUI_FORCE_LOST_AFTER=80 /tmp/api_coverage_app
//	GPUI_SELFTEST_LIFECYCLE=1 ... /tmp/api_coverage_app
//	GPUI_COVERAGE_STRICT=1  → exit 1 if any public API never hit
package main

import (
	"fmt"
	"sort"
	"sync"
)

// Coverage tracks which public Context APIs were exercised this process.
type Coverage struct {
	mu   sync.Mutex
	hit  map[string]int
	want []string
}

func NewCoverage(apis []string) *Coverage {
	c := &Coverage{hit: make(map[string]int, len(apis)), want: append([]string(nil), apis...)}
	for _, a := range apis {
		c.hit[a] = 0
	}
	return c
}

func (c *Coverage) Hit(name string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.hit[name]++
	c.mu.Unlock()
}

func (c *Coverage) Report() (covered, total int, missing []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	total = len(c.want)
	for _, a := range c.want {
		if c.hit[a] > 0 {
			covered++
		} else {
			missing = append(missing, a)
		}
	}
	sort.Strings(missing)
	return covered, total, missing
}

func (c *Coverage) SummaryLine() string {
	cov, tot, miss := c.Report()
	s := fmt.Sprintf("coverage=%d/%d (%.1f%%)", cov, tot, 100*float64(cov)/float64(max1(tot)))
	if len(miss) > 0 && len(miss) <= 12 {
		s += " missing=" + fmt.Sprint(miss)
	} else if len(miss) > 12 {
		s += fmt.Sprintf(" missing=%d (first=%v…)", len(miss), miss[:8])
	}
	return s
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// AllPublicContextAPIs is the checklist of render.Context exported methods
// that a full product host may call. Keep in sync with `rg 'func (c *Context)'`.
var AllPublicContextAPIs = []string{
	// frame / present
	"BeginFrame", "BeginGPUFrame", "MarkFullRedraw", "Invalidate", "PlanPresent",
	"PresentFrameFull", "PresentFrameAuto", "PresentFrameDamage", "PresentFrameDamageRects",
	"PresentFrame", "FlushGPU", "FlushGPUWithView", "FlushGPUWithViewDamage", "FlushGPUWithViewDamageRects",
	"SetDamageTracking", "TrackDamageRect", "FrameDamage", "FrameDamageUnion", "ResetFrameDamage",
	"SetSharedEncoder", "CreateSharedEncoder", "SubmitSharedEncoder",
	// lifecycle
	"Close", "DropGPURenderContext", "Resize", "ResizeTarget", "Width", "Height", "PixelWidth", "PixelHeight",
	"DeviceScale", "SetDeviceScale", "Clear", "ClearWithColor",
	// state
	"Push", "Pop", "SetAntiAlias", "AntiAlias", "SetDither", "Dither",
	"SetPipelineMode", "PipelineMode", "SetRasterizerMode", "RasterizerMode",
	"SetEffectSurface", "SetTextMode", "TextMode", "SetLCDLayout",
	"SetColor", "SetRGB", "SetRGBA", "SetHexColor",
	"SetFillBrush", "SetStrokeBrush", "FillBrush", "StrokeBrush",
	"SetFillPattern", "SetStrokePattern", "CreateImagePattern",
	"SetLineWidth", "SetLineCap", "SetLineJoin", "SetFillRule", "SetMiterLimit",
	"SetStroke", "GetStroke", "SetDash", "SetDashOffset", "ClearDash", "IsDashed",
	"SetBlendMode",
	// transform
	"Identity", "Translate", "Scale", "Rotate", "RotateAbout", "Shear", "Transform", "SetTransform", "GetTransform",
	"TransformPoint", "InvertY",
	// path
	"MoveTo", "LineTo", "QuadraticTo", "CubicTo", "ClosePath", "ClearPath", "NewSubPath",
	"SetPath", "AppendPath", "DrawPath", "FillPath", "StrokePath", "GetCurrentPoint",
	"Fill", "Stroke", "FillPreserve", "StrokePreserve",
	// shapes
	"DrawPoint", "DrawLine", "DrawRectangle", "DrawRoundedRectangle", "DrawRoundedRectangleXY",
	"DrawCircle", "DrawEllipse", "DrawArc", "DrawEllipticalArc", "DrawRegularPolygon",
	// clip / mask
	"Clip", "ClipPreserve", "ClipRect", "ClipRoundRect", "ResetClip",
	"ClipRectOp", "ClipPathOp", "SetMask", "GetMask", "ClearMask", "ApplyMask", "InvertMask", "AsMask",
	// layers
	"PushLayer", "PopLayer", "PushBackdropLayer", "PushMaskLayer", "LayerPoolStats", "ResetLayerPoolStats",
	// filters
	"ApplyBlur", "ApplyBlurXY", "ApplyDropShadow", "ApplyGrayscale", "ApplyInvert", "ApplyColorMatrix",
	"ApplyImageFilterGraph", "GPUFilterTexture",
	// image
	"DrawImage", "DrawImageEx", "DrawImageRounded", "DrawImageCircular", "DrawImageNine", "DrawImageQuad",
	"DrawAtlas", "ExportImageBuf", "DrawGPUTexture", "DrawGPUTextureWithOpacity", "DrawGPUTextureWithOpacityUV",
	"DrawGPUTextureBase", "CreateOffscreenTexture",
	// text
	"SetFont", "Font", "LoadFontFace", "LoadFontFaceWithVariations", "FontVariationAxes",
	"DrawString", "DrawStringAnchored", "DrawStringWrapped", "StrokeString", "StrokeStringAnchored",
	"MeasureString", "MeasureMultilineString", "WordWrap", "DrawShapedGlyphs", "TextPath",
	"SetTextDecoration", "TextDecoration",
	// mesh / pixels
	"DrawVertices", "DrawMesh", "SetPixel", "WritePixels", "FillRectCPU",
	// io / stats
	"Image", "SavePNG", "EncodePNG", "EncodeJPEG",
	"RenderPathStats", "ResetRenderPathStats", "LastCPUFallbackReason", "MemDigCmdBufs", "GPURenderContext",
}
