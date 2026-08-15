// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build !nogpu

// Command compute_clip is a visual demo exercising Vello compute-pipeline clip
// layers.
//
// The scene is described once as SceneElements (gpu.RenderSceneAuto): the
// engine automatically renders it through the GPU compute pipeline when
// available and falls back to the CPU reference otherwise — no manual
// CPU/GPU branching in the example.
//
// Scene layout:
//   - White background
//   - Green rectangle (full canvas)
//   - BeginClip: rounded-rectangle-approximated clip (centered, ~200x150)
//   - 8 colored circles inside clip (clipped to the rounded rect)
//   - Blue rectangle inside clip (partially clipped)
//   - EndClip
//   - Yellow star outside clip (fully visible, NOT clipped)
//
// Output:
//
//	tmp/compute_clip.png  — rendered image (selected backend)
package main

import (
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/energye/gpui/render/internal/gpu"
	"github.com/energye/gpui/render/internal/gpu/tilecompute"
)

const (
	canvasWidth  = 400
	canvasHeight = 300
)

var bgColor = [4]uint8{255, 255, 255, 255} // White background.

func main() {
	fmt.Println("Vello Compute Pipeline — Clip Layers Demo")
	fmt.Println("==========================================")
	fmt.Println()

	// Enable debug logging for GPU init diagnostics.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// Scene is described once; the backend (GPU compute / CPU reference) is
	// selected automatically by gpu.RenderSceneAuto.
	elements := buildClipScene()

	tilesX := (canvasWidth + tilecompute.TileWidth - 1) / tilecompute.TileWidth
	tilesY := (canvasHeight + tilecompute.TileHeight - 1) / tilecompute.TileHeight
	fmt.Printf("Scene: %d element(s) (draws + clips)\n", len(elements))
	fmt.Printf("Canvas: %dx%d (%dx%d tiles)\n\n", canvasWidth, canvasHeight, tilesX, tilesY)

	res := gpu.RenderSceneAuto(canvasWidth, canvasHeight, bgColor, elements)
	fmt.Printf("Render (%s)... %v done\n", res.Backend, res.Duration.Round(100*time.Microsecond))
	if res.GPUError != nil {
		fmt.Printf("  (GPU attempted but failed: %v — fell back to CPU)\n", res.GPUError)
	}
	fmt.Println()

	// Ensure tmp directory exists.
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create tmp/: %v\n", err)
		os.Exit(1)
	}

	// Save the rendered image.
	if err := savePNG(res.Img, "tmp/compute_clip.png"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: save image: %v\n", err)
		os.Exit(1)
	}

	// --- Diagnostics: check specific pixels ---
	printDiagnostics(res.Img)

	fmt.Println("\nOutput:")
	fmt.Println("  Image:   tmp/compute_clip.png")
	fmt.Printf("  Backend: %s\n", res.Backend)
}

// buildClipScene creates a scene that exercises clip layers.
//
// Layout:
//  1. Green background rectangle (full canvas)
//  2. BeginClip with rounded-rect-approximated path (centered ~200x150)
//  3. 8 colored circles inside clip
//  4. Blue rectangle inside clip (partially outside clip bounds)
//  5. EndClip
//  6. Yellow 5-point star outside clip (fully visible)
func buildClipScene() []tilecompute.SceneElement {
	// Clip region: centered rounded rectangle (~200x150).
	// Approximated by a rectangle with rounded corners using cubic beziers.
	clipPath := roundedRectLines(100, 75, 300, 225, 20)

	return []tilecompute.SceneElement{
		// 1. Green background (full canvas).
		{
			Type:     tilecompute.ElementDraw,
			Lines:    rectLines(0, 0, canvasWidth, canvasHeight),
			Color:    [4]uint8{60, 180, 60, 255},
			FillRule: tilecompute.FillRuleNonZero,
		},
		// 2. BeginClip: rounded rectangle.
		{
			Type:      tilecompute.ElementBeginClip,
			Lines:     clipPath,
			BlendMode: 0x8003, // CLIP_BLEND_MODE (simple clip)
			Alpha:     1.0,
		},
		// 3-10. Eight colored circles inside the clip region.
		circleElement(140, 110, 22, [4]uint8{220, 40, 40, 220}),  // red
		circleElement(200, 110, 28, [4]uint8{40, 40, 220, 220}),  // blue
		circleElement(260, 110, 20, [4]uint8{220, 40, 220, 220}), // magenta
		circleElement(170, 155, 30, [4]uint8{40, 200, 200, 220}), // cyan
		circleElement(230, 155, 26, [4]uint8{220, 180, 40, 220}), // orange
		circleElement(140, 200, 24, [4]uint8{180, 40, 220, 220}), // purple
		circleElement(200, 200, 32, [4]uint8{40, 220, 40, 220}),  // green
		circleElement(260, 200, 18, [4]uint8{220, 220, 40, 220}), // yellow
		// 11. Blue rectangle inside clip (extends beyond clip bounds to test clipping).
		{
			Type:     tilecompute.ElementDraw,
			Lines:    rectLines(80, 130, 320, 170),
			Color:    [4]uint8{30, 60, 220, 180},
			FillRule: tilecompute.FillRuleNonZero,
		},
		// 12. EndClip.
		{Type: tilecompute.ElementEndClip},
		// 13. Yellow 5-point star OUTSIDE clip (fully visible, not clipped).
		{
			Type:     tilecompute.ElementDraw,
			Lines:    starLines(50, 50, 35),
			Color:    [4]uint8{230, 200, 0, 255},
			FillRule: tilecompute.FillRuleEvenOdd,
		},
		// 14. Another star on the right side.
		{
			Type:     tilecompute.ElementDraw,
			Lines:    starLines(350, 50, 35),
			Color:    [4]uint8{230, 200, 0, 255},
			FillRule: tilecompute.FillRuleEvenOdd,
		},
	}
}

// printDiagnostics prints pixel values at key locations to verify clipping.
func printDiagnostics(img *image.RGBA) {
	type probe struct {
		x, y int
		desc string
	}
	probes := []probe{
		{200, 150, "center of clip region (circles + blue rect)"},
		{50, 50, "yellow star (outside clip, fully visible)"},
		{350, 50, "yellow star right (outside clip)"},
		{10, 10, "top-left corner (green bg, outside clip)"},
		{200, 10, "top center (green bg, outside clip)"},
		{95, 150, "left edge of clip (boundary)"},
		{305, 150, "right edge of clip (boundary)"},
		{200, 70, "just above clip region (green bg)"},
		{200, 230, "just below clip region (green bg)"},
		{140, 110, "red circle center (inside clip)"},
		{260, 200, "yellow circle center (inside clip)"},
	}

	fmt.Println("Diagnostic pixels:")
	for _, p := range probes {
		c := img.RGBAAt(p.x, p.y)
		fmt.Printf("  (%3d,%3d) %-45s RGBA=(%3d,%3d,%3d,%3d)\n",
			p.x, p.y, p.desc, c.R, c.G, c.B, c.A)
	}
}

// --- Shape builders ---

// circleElement creates a SceneElement for a filled circle.
func circleElement(cx, cy, radius float32, col [4]uint8) tilecompute.SceneElement {
	return tilecompute.SceneElement{
		Type:     tilecompute.ElementDraw,
		Lines:    tilecompute.FlattenFill(circleCubics(cx, cy, radius)),
		Color:    col,
		FillRule: tilecompute.FillRuleNonZero,
	}
}

func rectLines(x0, y0, x1, y1 float32) []tilecompute.LineSoup {
	return []tilecompute.LineSoup{
		{PathIx: 0, P0: [2]float32{x0, y0}, P1: [2]float32{x1, y0}},
		{PathIx: 0, P0: [2]float32{x1, y0}, P1: [2]float32{x1, y1}},
		{PathIx: 0, P0: [2]float32{x1, y1}, P1: [2]float32{x0, y1}},
		{PathIx: 0, P0: [2]float32{x0, y1}, P1: [2]float32{x0, y0}},
	}
}

// roundedRectLines creates line segments approximating a rounded rectangle.
// Uses cubic bezier curves for the corners, flattened to line segments.
func roundedRectLines(x0, y0, x1, y1, radius float32) []tilecompute.LineSoup {
	// Clamp radius to half the smaller dimension.
	maxR := float32(math.Min(float64(x1-x0), float64(y1-y0))) / 2
	if radius > maxR {
		radius = maxR
	}

	const kappa float32 = 0.5522847498 // 4*(sqrt(2)-1)/3 for circular arcs
	k := radius * kappa

	// Corner arcs as cubic beziers, then flatten.
	// Top-right corner.
	trArc := tilecompute.FlattenFill([]tilecompute.CubicBezier{{
		P0: [2]float32{x1 - radius, y0},
		P1: [2]float32{x1 - radius + k, y0},
		P2: [2]float32{x1, y0 + radius - k},
		P3: [2]float32{x1, y0 + radius},
	}})
	// Bottom-right corner.
	brArc := tilecompute.FlattenFill([]tilecompute.CubicBezier{{
		P0: [2]float32{x1, y1 - radius},
		P1: [2]float32{x1, y1 - radius + k},
		P2: [2]float32{x1 - radius + k, y1},
		P3: [2]float32{x1 - radius, y1},
	}})
	// Bottom-left corner.
	blArc := tilecompute.FlattenFill([]tilecompute.CubicBezier{{
		P0: [2]float32{x0 + radius, y1},
		P1: [2]float32{x0 + radius - k, y1},
		P2: [2]float32{x0, y1 - radius + k},
		P3: [2]float32{x0, y1 - radius},
	}})
	// Top-left corner.
	tlArc := tilecompute.FlattenFill([]tilecompute.CubicBezier{{
		P0: [2]float32{x0, y0 + radius},
		P1: [2]float32{x0, y0 + radius - k},
		P2: [2]float32{x0 + radius - k, y0},
		P3: [2]float32{x0 + radius, y0},
	}})

	// Assemble: top edge, TR arc, right edge, BR arc, bottom edge, BL arc, left edge, TL arc.
	lines := make([]tilecompute.LineSoup, 0, 4+len(trArc)+len(brArc)+len(blArc)+len(tlArc))

	// Top edge.
	lines = append(lines, tilecompute.LineSoup{
		PathIx: 0,
		P0:     [2]float32{x0 + radius, y0},
		P1:     [2]float32{x1 - radius, y0},
	})
	// Top-right arc.
	lines = append(lines, trArc...)
	// Right edge.
	lines = append(lines, tilecompute.LineSoup{
		PathIx: 0,
		P0:     [2]float32{x1, y0 + radius},
		P1:     [2]float32{x1, y1 - radius},
	})
	// Bottom-right arc.
	lines = append(lines, brArc...)
	// Bottom edge.
	lines = append(lines, tilecompute.LineSoup{
		PathIx: 0,
		P0:     [2]float32{x1 - radius, y1},
		P1:     [2]float32{x0 + radius, y1},
	})
	// Bottom-left arc.
	lines = append(lines, blArc...)
	// Left edge.
	lines = append(lines, tilecompute.LineSoup{
		PathIx: 0,
		P0:     [2]float32{x0, y1 - radius},
		P1:     [2]float32{x0, y0 + radius},
	})
	// Top-left arc.
	lines = append(lines, tlArc...)

	return lines
}

func circleCubics(cx, cy, r float32) []tilecompute.CubicBezier {
	const kappa float32 = 0.5522847498
	k := r * kappa
	return []tilecompute.CubicBezier{
		{P0: [2]float32{cx + r, cy}, P1: [2]float32{cx + r, cy + k}, P2: [2]float32{cx + k, cy + r}, P3: [2]float32{cx, cy + r}},
		{P0: [2]float32{cx, cy + r}, P1: [2]float32{cx - k, cy + r}, P2: [2]float32{cx - r, cy + k}, P3: [2]float32{cx - r, cy}},
		{P0: [2]float32{cx - r, cy}, P1: [2]float32{cx - r, cy - k}, P2: [2]float32{cx - k, cy - r}, P3: [2]float32{cx, cy - r}},
		{P0: [2]float32{cx, cy - r}, P1: [2]float32{cx + k, cy - r}, P2: [2]float32{cx + r, cy - k}, P3: [2]float32{cx + r, cy}},
	}
}

func starLines(cx, cy, r float32) []tilecompute.LineSoup {
	// 5-point star vertices (tip at top).
	var vertices [5][2]float32
	for i := 0; i < 5; i++ {
		angle := -math.Pi/2 + float64(i)*2*math.Pi/5
		vertices[i] = [2]float32{
			cx + r*float32(math.Cos(angle)),
			cy + r*float32(math.Sin(angle)),
		}
	}

	// Star order: 0 -> 2 -> 4 -> 1 -> 3 -> 0 (skip one vertex).
	order := [6]int{0, 2, 4, 1, 3, 0}
	lines := make([]tilecompute.LineSoup, 5)
	for i := 0; i < 5; i++ {
		lines[i] = tilecompute.LineSoup{
			PathIx: 0,
			P0:     vertices[order[i]],
			P1:     vertices[order[i+1]],
		}
	}
	return lines
}

func savePNG(img *image.RGBA, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}