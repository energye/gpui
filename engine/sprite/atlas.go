//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package sprite

import (
	"math"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/renderconv"
	"github.com/energye/gpui/render"
)

// Per-sprite sampling reuses render's two existing enums (P3.7 batch 4, D6):
// render.InterpolationMode picks the resampling quality and render.MipmapMode
// picks the level path. The engine no longer owns a third filter enum — the
// old AtlasFilter/AtlasFilter* set was a duplicate of the render vocabulary
// (bicubic included, 1:1). Both zero values are load-bearing: Filter 0 is
// «unset» (render normalizes it to bilinear, the historic road) and Mipmap 0
// is MipmapNone (single-level path, bit-identical to every pre-batch-4 sprite).
const (
	// FilterUnset is the zero quality: render maps it to bilinear (old road).
	FilterUnset = render.InterpolationMode(0)
)

// validFilter reports whether m is a settable quality (never guess a bad one).
func validFilter(m render.InterpolationMode) bool {
	switch m {
	case FilterUnset, render.InterpNearest, render.InterpBilinear, render.InterpBicubic:
		return true
	}
	return false
}

// validMipmap reports whether m is a settable level path.
func validMipmap(m render.MipmapMode) bool {
	switch m {
	case render.MipmapNone, render.MipmapNearest, render.MipmapLinear:
		return true
	}
	return false
}

// FilterName returns the stable log/data name of one sampling quality.
func FilterName(m render.InterpolationMode) string {
	switch m {
	case FilterUnset:
		return "default"
	case render.InterpNearest:
		return "nearest"
	case render.InterpBilinear:
		return "bilinear"
	case render.InterpBicubic:
		return "bicubic"
	default:
		return "unknown"
	}
}

// ParseFilter maps "", "default", "nearest", "bilinear", "bicubic" to a
// sampling quality. Anything else is a core InvalidArg error, never a guess
// (the vocabulary the atlas case data uses stayed intact through the merge).
func ParseFilter(str string) (render.InterpolationMode, error) {
	switch str {
	case "", "default":
		return FilterUnset, nil
	case "nearest":
		return render.InterpNearest, nil
	case "bilinear":
		return render.InterpBilinear, nil
	case "bicubic":
		return render.InterpBicubic, nil
	default:
		return FilterUnset, core.InvalidArg("sprite.ParseFilter", str)
	}
}

// MipmapName returns the stable log/data name of one level path.
func MipmapName(m render.MipmapMode) string {
	switch m {
	case render.MipmapNone:
		return "none"
	case render.MipmapNearest:
		return "nearest"
	case render.MipmapLinear:
		return "linear"
	default:
		return "unknown"
	}
}

// ParseMipmap maps "", "none", "nearest", "linear" to a level path. Anything
// else errors: an unset field means «stay on the single-level road», not «pick
// something nicer» — that is what keeps every existing sprite bit-identical.
func ParseMipmap(str string) (render.MipmapMode, error) {
	switch str {
	case "", "none":
		return render.MipmapNone, nil
	case "nearest":
		return render.MipmapNearest, nil
	case "linear":
		return render.MipmapLinear, nil
	default:
		return render.MipmapNone, core.InvalidArg("sprite.ParseMipmap", str)
	}
}

// AtlasSprite is one sub-rect of a large image with rotation, tint, flip,
// pivot, and per-sprite filter: which atlas it comes from, the source
// block in image pixels, the destination box in world units, the opacity,
// the rotation about the pivot, the mirror before rotation, the pivot
// offset from the Dst origin, the multiply tint, and the sampling.
// Tag is a debug key for tests and logs only.
//
// Perspective (P3.7 batch 1c) switches the sprite to the projective leg:
// Corners carry the projected screen corners (TL/TR/BR/BL, user space of
// the target canvas — the camera exit fills them) and Weights the per-corner
// homogeneous w (= focal+depth); Dst/Rot/Flip/Pivot are then ignored, no
// half-affine-half-perspective mix. Zero value keeps the affine road.
// 同 Godot Sprite3D 的职责划分：投影由引擎算好，render 只收顶点级数据。
type AtlasSprite struct {
	Image   core.AssetID
	Src     core.Rect
	Dst     core.Rect
	Opacity float64
	Rot     float64
	FlipX   bool
	FlipY   bool
	Pivot   core.Vec2
	Tint    core.Color
	Filter  render.InterpolationMode
	// Mipmap is the per-sprite level path (P3.7 batch 4): the zero value
	// MipmapNone keeps the historic single-level draw bit-identical.
	Mipmap render.MipmapMode
	Tag    string

	Perspective bool
	Corners     [4]core.Vec2
	Weights     [4]float64
}

func finiteAtlasFloat(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteAtlasRect(r core.Rect) bool {
	return finiteAtlasFloat(r.X) && finiteAtlasFloat(r.Y) &&
		finiteAtlasFloat(r.W) && finiteAtlasFloat(r.H)
}

func finiteAtlasColor(c core.Color) bool {
	return finiteAtlasFloat(c.R) && finiteAtlasFloat(c.G) &&
		finiteAtlasFloat(c.B) && finiteAtlasFloat(c.A)
}

// NewAtlasSprite builds an AtlasSprite. Image must be non-empty, Src/Dst
// coordinates, Opacity, Rot, Pivot, and Tint components must be finite,
// Filter must be a settable choice. Zero-size rects are allowed here and
// skipped later by the render draw, mirroring the R4 skip. Tag stays empty;
// the caller sets it for debugging.
func NewAtlasSprite(image core.AssetID, src, dst core.Rect, opacity, rot float64, pivot core.Vec2, tint core.Color, filter render.InterpolationMode, flipX, flipY bool) (AtlasSprite, error) {
	const op = "sprite.NewAtlasSprite"
	if image.Empty() {
		return AtlasSprite{}, core.InvalidArg(op, "image")
	}
	if !finiteAtlasRect(src) {
		return AtlasSprite{}, core.InvalidArg(op, "src")
	}
	if !finiteAtlasRect(dst) {
		return AtlasSprite{}, core.InvalidArg(op, "dst")
	}
	if !finiteAtlasFloat(opacity) {
		return AtlasSprite{}, core.InvalidArg(op, "opacity")
	}
	if !finiteAtlasFloat(rot) {
		return AtlasSprite{}, core.InvalidArg(op, "rot")
	}
	if !finiteAtlasFloat(pivot.X) || !finiteAtlasFloat(pivot.Y) {
		return AtlasSprite{}, core.InvalidArg(op, "pivot")
	}
	if !finiteAtlasColor(tint) {
		return AtlasSprite{}, core.InvalidArg(op, "tint")
	}
	if !validFilter(filter) {
		return AtlasSprite{}, core.InvalidArg(op, "filter")
	}
	return AtlasSprite{Image: image, Src: src, Dst: dst, Opacity: opacity, Rot: rot, Pivot: pivot, Tint: tint, Filter: filter, FlipX: flipX, FlipY: flipY}, nil
}

// Validate reports whether s is drawable (finite numbers, known filter,
// non-empty image). Zero-size rects are valid here; they are skipped by
// Skippable and by the render draw, never an error.
func (s AtlasSprite) Validate() error {
	const op = "sprite.AtlasSprite.Validate"
	if s.Image.Empty() {
		return core.InvalidArg(op, "image")
	}
	if !finiteAtlasRect(s.Src) {
		return core.InvalidArg(op, "src")
	}
	if !finiteAtlasRect(s.Dst) {
		return core.InvalidArg(op, "dst")
	}
	if !finiteAtlasFloat(s.Opacity) {
		return core.InvalidArg(op, "opacity")
	}
	if !finiteAtlasFloat(s.Rot) {
		return core.InvalidArg(op, "rot")
	}
	if !finiteAtlasFloat(s.Pivot.X) || !finiteAtlasFloat(s.Pivot.Y) {
		return core.InvalidArg(op, "pivot")
	}
	if !finiteAtlasColor(s.Tint) {
		return core.InvalidArg(op, "tint")
	}
	if !validFilter(s.Filter) {
		return core.InvalidArg(op, "filter")
	}
	if !validMipmap(s.Mipmap) {
		return core.InvalidArg(op, "mipmap")
	}
	if s.Perspective {
		for i := range s.Corners {
			if !finiteAtlasFloat(s.Corners[i].X) || !finiteAtlasFloat(s.Corners[i].Y) {
				return core.InvalidArg(op, "corners")
			}
		}
		for _, w := range s.Weights {
			if !finiteAtlasFloat(w) || w <= 0 {
				return core.InvalidArg(op, "weights")
			}
		}
	}
	return nil
}

// Skippable reports whether the render draw skips s without drawing,
// matching the R4 skip: empty source or zero destination size. Negative
// destination size is kept (mirrored draw at the render edge).
func (s AtlasSprite) Skippable() bool {
	return s.Src.W <= 0 || s.Src.H <= 0 || s.Dst.W == 0 || s.Dst.H == 0
}

// HasTint reports whether s carries an explicit tint. The zero color
// means no tint (white opaque at the render edge); any other value,
// including explicit white, multiplies the texels.
func (s AtlasSprite) HasTint() bool { return s.Tint != (core.Color{}) }

// PivotPoint returns the absolute pivot in world units: Dst origin plus
// the pivot offset. Rotating about the feet keeps this point fixed, so a
// turning body stays glued at the feet.
func (s AtlasSprite) PivotPoint() core.Vec2 {
	return core.V2(s.Dst.X+s.Pivot.X, s.Dst.Y+s.Pivot.Y)
}

// FeetPivot returns the pivot offset that pins the rotation center at the
// feet: bottom-edge center of dst in Dst units. Use it as Pivot so a turn
// never lifts the feet.
func FeetPivot(dst core.Rect) core.Vec2 { return core.V2(dst.W/2, dst.H) }

// ToRender converts s to the render draw at the boundary, field for field.
// Bad sprites return a core InvalidArg error and no guess; skippable
// sprites convert fine and are skipped later by DrawAtlasEx.
func (s AtlasSprite) ToRender() (render.AtlasSprite, error) {
	if err := s.Validate(); err != nil {
		return render.AtlasSprite{}, err
	}
	out := render.AtlasSprite{
		SrcX: s.Src.X, SrcY: s.Src.Y, SrcW: s.Src.W, SrcH: s.Src.H,
		DstX: s.Dst.X, DstY: s.Dst.Y, DstW: s.Dst.W, DstH: s.Dst.H,
		Opacity: s.Opacity, Rot: s.Rot, FlipX: s.FlipX, FlipY: s.FlipY,
		PivotX: s.Pivot.X, PivotY: s.Pivot.Y,
		Tint: renderconv.ColorToRender(s.Tint),
		// Both axes ride straight through: render owns the vocabularies, and its
		// zero values already mean «historic road» (batch 4 merged the enums).
		Filter: s.Filter,
		Mipmap: s.Mipmap,
	}
	if s.Perspective {
		out.Perspective = true
		for i := range s.Corners {
			out.Corners[i] = renderconv.Vec2ToRenderPoint(s.Corners[i])
		}
		out.Weights = s.Weights
	}
	return out, nil
}

// AtlasToRender converts items to the render draws at the boundary,
// preserving order and count (lossless): the caller draws the result with
// the existing DrawAtlasEx, old road untouched. The input slice is never
// mutated; the result is a fresh slice. Empty input returns nil with nil
// error. The first bad sprite aborts with its InvalidArg error and nil
// result, never a partial draw.
func AtlasToRender(items []AtlasSprite) ([]render.AtlasSprite, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]render.AtlasSprite, len(items))
	for i := range items {
		rs, err := items[i].ToRender()
		if err != nil {
			return nil, err
		}
		out[i] = rs
	}
	return out, nil
}
