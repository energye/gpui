// Package render provides a simple 2D graphics library for Go.
//
// # Overview
//
// render is a Pure Go 2D graphics library inspired by fogleman/gg and designed
// to integrate with the GoGPU ecosystem. It provides an immediate-mode drawing
// API similar to HTML Canvas, with both software and GPU rendering backends.
//
// # Quick Start
//
//	import "github.com/energye/gpui/render"
//
//	// Create a drawing context (dc = drawing context convention)
//	dc := render.NewContext(512, 512)
//
//	// Draw shapes
//	dc.SetRGB(1, 0, 0)
//	dc.DrawCircle(256, 256, 100)
//	dc.Fill()
//
//	// Save to PNG
//	dc.SavePNG("output.png")
//
// # API Compatibility
//
// The API is designed to be compatible with fogleman/gg for easy migration.
// Most fogleman/gg code should work with minimal changes (package name render).
//
// # Renderers
//
// The library includes both software and GPU-accelerated renderers:
//   - Software rasterizer for broad compatibility
//   - GPU renderer via gogpu/wgpu for high performance
//
// # Architecture
//
// The library is organized into:
//   - Public API: Context, Path, Paint, Matrix, Point
//   - Internal: raster (scanline), path (tessellation), blend (compositing)
//   - Renderers: software, gpu (wgpu)
//
// # Coordinate System
//
// Uses standard computer graphics coordinates:
//   - Origin (0,0) at top-left
//   - X increases right
//   - Y increases down
//   - Angles in radians, 0 is right, increases counter-clockwise
//
// # R2 CPU渐变冻结（S16/W2，见 vertices.go）
//
// 老 DrawVertices/DrawMesh 签名不动；新 DrawVerticesEx/DrawMeshEx 带
// VertDrawOptions/VertDrawResult（Degraded 降级标记）与哨兵错
// ErrVertsNonFinite/ErrVertsBadIndex。CPU 为预乘重心真渐变，与 GPU 只差抗锯齿。
//
// # Performance
//
// The software renderer prioritizes correctness.
// For performance-critical applications, use the GPU renderer.
package render
