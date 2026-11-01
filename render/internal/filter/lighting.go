//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

// P3.4 batch 2 CPU filters: point-source diffuse lighting (Skia
// LightingImageFilter — source alpha as the height map) and Tile
// (SkImageFilters::Tile). Straight-domain color math where noted; the
// premul storage invariant is preserved on write.

package filter

import (
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// PointDiffuseFilter lights the image with a point source: the source alpha
// channel is the height map (Skia semantics), the surface normal comes from
// central differences scaled by surfaceScale, and the result replaces RGB
// with lightColor·kd·max(N·L, 0)·srcAlpha while keeping the source alpha.
type PointDiffuseFilter struct {
	lightPos     [3]float32 // pixels: x, y over the image, z above the plane
	lightColor   [3]float32 // straight 0-1
	kd           float32
	surfaceScale float32
}

// NewPointDiffuseFilter builds a point-diffuse lighting filter.
func NewPointDiffuseFilter(lightPos [3]float32, color render.RGBA, kd, surfaceScale float32) *PointDiffuseFilter {
	return &PointDiffuseFilter{
		lightPos:     lightPos,
		lightColor:   [3]float32{float32(color.R), float32(color.G), float32(color.B)},
		kd:           kd,
		surfaceScale: surfaceScale,
	}
}

// ExpandBounds is identity.
func (f *PointDiffuseFilter) ExpandBounds(bounds image.Rectangle) image.Rectangle {
	return bounds
}

// Apply runs the diffuse lighting over the bounds.
func (f *PointDiffuseFilter) Apply(src, dst *render.Pixmap, bounds scene.Rect) {
	w, h := src.Width(), src.Height()
	srcData, dstData := src.Data(), dst.Data()
	ss := f.surfaceScale
	for y := int(bounds.MinY); y < int(bounds.MaxY); y++ {
		for x := int(bounds.MinX); x < int(bounds.MaxX); x++ {
			i := (y*w + x) * 4
			a := float32(srcData[i+3]) / 255.0
			aL := alphaAt(srcData, w, h, x-1, y)
			aR := alphaAt(srcData, w, h, x+1, y)
			aT := alphaAt(srcData, w, h, x, y-1)
			aB := alphaAt(srcData, w, h, x, y+1)
			// Surface normal from central alpha differences (straight-space
			// heights 0-1; premul alpha scales the height uniformly, which a
			// constant factor absorbs into the normal length).
			nx := -(aR - aL) * ss
			ny := -(aB - aT) * ss
			nz := float32(1.0)
			inv := float32(1.0) / sqrt3(nx*nx+ny*ny+nz*nz)
			nx, ny, nz = nx*inv, ny*inv, nz*inv
			lx := f.lightPos[0] - float32(x)
			ly := f.lightPos[1] - float32(y)
			lz := f.lightPos[2]
			if lz >= 0 && lz < 0.001 {
				lz = 0.001 // zero-division guard only; negative z stays (light below the plane → kd≤0)
			}
			lil := 1.0 / sqrt3(lx*lx+ly*ly+lz*lz)
			lx, ly, lz = lx*lil, ly*lil, lz*lil
			kd := nx*lx + ny*ly + nz*lz
			if kd < 0 {
				kd = 0
			}
			lit := [3]float32{
				f.lightColor[0] * f.kd * kd * a,
				f.lightColor[1] * f.kd * kd * a,
				f.lightColor[2] * f.kd * kd * a,
			}
			dstData[i] = u8c(lit[0])
			dstData[i+1] = u8c(lit[1])
			dstData[i+2] = u8c(lit[2])
			dstData[i+3] = srcData[i+3]
		}
	}
}

// TileFilter repeats the source tile rect across the whole output
// (SkImageFilters::Tile semantics: content outside the rect repeats it).
type TileFilter struct {
	srcRect [4]float32 // pixels: x0, y0, x1, y1
}

// NewTileFilter builds a tile filter over the source rect.
func NewTileFilter(srcRect [4]float32) *TileFilter { return &TileFilter{srcRect: srcRect} }

// ExpandBounds is identity.
func (f *TileFilter) ExpandBounds(bounds image.Rectangle) image.Rectangle {
	return bounds
}

// Apply tiles the source rect across the bounds.
func (f *TileFilter) Apply(src, dst *render.Pixmap, bounds scene.Rect) {
	w, h := src.Width(), src.Height()
	srcData, dstData := src.Data(), dst.Data()
	x0, y0 := f.srcRect[0], f.srcRect[1]
	x1, y1 := f.srcRect[2], f.srcRect[3]
	tw, th := x1-x0, y1-y0
	if tw <= 0 || th <= 0 {
		return
	}
	for y := int(bounds.MinY); y < int(bounds.MaxY); y++ {
		for x := int(bounds.MinX); x < int(bounds.MaxX); x++ {
			sx := x0 + modF(float32(x), tw)
			sy := y0 + modF(float32(y), th)
			sxi, syi := int(sx), int(sy)
			if sxi < 0 || sxi >= w || syi < 0 || syi >= h {
				continue
			}
			si := (syi*w + sxi) * 4
			oi := (y*w + x) * 4
			dstData[oi], dstData[oi+1], dstData[oi+2], dstData[oi+3] =
				srcData[si], srcData[si+1], srcData[si+2], srcData[si+3]
		}
	}
}

func alphaAt(data []uint8, w, h, x, y int) float32 {
	if x < 0 {
		x = 0
	}
	if x >= w {
		x = w - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= h {
		y = h - 1
	}
	return float32(data[(y*w+x)*4+3]) / 255.0
}

func sqrt3(v float32) float32 {
	if v <= 0 {
		return 0.0001
	}
	// Newton iterations; math.Sqrt is fine too but keep the file dependency
	// surface identical to the GPU branch.
	g := v
	for i := 0; i < 8; i++ {
		g = (g + v/g) * 0.5
	}
	return g
}

func u8c(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

func modF(a, b float32) float32 {
	m := a - b*float32(int(a/b))
	if m < 0 {
		m += b
	}
	return m
}
