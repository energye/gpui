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
// # R4 图集扩展冻结（S30/W4，2.2 的 render 底，前置 S24 R3，见 vertices.go）
//
// 老 DrawAtlas 签名不动，只读老字段；新字段零值即老路，逐位一致。
// 新 DrawAtlasEx 带 AtlasDrawOptions/AtlasDrawResult 与哨兵错
// ErrAtlasNonFinite/ErrAtlasUnsupportedFilter。AtlasSprite 加
// Rot/FlipX/FlipY/PivotX/PivotY/Tint/Filter：绕轴心先翻转后旋转，
// Tint 零结构体即白不透明（直射相乘含 A，显卡走逐顶点 premul 颜色，
// 与 CPU 真采样同真值，只差采样舍入）；Filter 零值即 Bilinear，单图选
// Nearest/Bilinear/Bicubic。rot/flip/tint/filter 走显卡四角，
// CPU 与显卡同拆法只差采样舍入。GOGPU_RENDER_MODE=cpu 强制取 CPU 真值，
// 离屏对比即未来 game_sprite--case=rot 窗的依据（窗随 P2 建）。
//
// # Performance
//
// The software renderer prioritizes correctness.
// For performance-critical applications, use the GPU renderer.
package render
