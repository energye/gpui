package kit

import (
	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Public prim re-exports for examples/kit_f0_* windows. Product code in
// this package imports internal/prim directly; examples import kit only.

type (
	// PrimDirection follows scope.Direction for Row RTL.
	PrimDirection = prim.Direction
	// PrimCrossAlign selects cross-axis placement.
	PrimCrossAlign = prim.CrossAlign
	// PrimLimits carries constrained-box min/max edges.
	PrimLimits = prim.Limits
	// PrimOffstage keeps measured size while hidden.
	PrimOffstage = prim.Offstage
	// PrimFlexSpec is one flex child spec.
	PrimFlexSpec = prim.FlexSpec
	// PrimFlexResult is computed flex geometry.
	PrimFlexResult = prim.FlexResult
	// PrimStackSpec is one stack layer spec.
	PrimStackSpec = prim.StackSpec
	// PrimStackResult is computed stack geometry.
	PrimStackResult = prim.StackResult
	// PrimWrapResult is computed flow geometry.
	PrimWrapResult = prim.WrapResult
	// PrimViewportBox wires a virtual list under a viewport.
	PrimViewportBox = prim.ViewportBox
	// PrimDecorSpec is the resolved decoration.
	PrimDecorSpec = prim.DecorSpec
	// PrimDecorProps carries decoration overrides.
	PrimDecorProps = prim.DecorProps
	// PrimOvalClipSpec describes an oval clip box.
	PrimOvalClipSpec = prim.OvalClipSpec
	// PrimPainter draws custom content.
	PrimPainter = prim.Painter
	// PrimTextStyle is the inheritable text look.
	PrimTextStyle = prim.TextStyle
	// PrimLabelProps configures single-style text.
	PrimLabelProps = prim.LabelProps
	// PrimSpan is one styled run.
	PrimSpan = prim.Span
	// PrimRichProps configures multi-span text.
	PrimRichProps = prim.RichProps
	// PrimPictureState names the async image phase.
	PrimPictureState = prim.PictureState
	// PrimPictureSpec resolves placeholder geometry.
	PrimPictureSpec = prim.PictureSpec
	// PrimIconSpec resolves icon size and color.
	PrimIconSpec = prim.IconSpec
	// PrimTextStyleScope carries one inherited style level.
	PrimTextStyleScope = prim.TextStyleScope
)

const (
	// PrimDirLTR lays Row children left to right.
	PrimDirLTR = prim.DirLTR
	// PrimDirRTL lays Row children right to left.
	PrimDirRTL = prim.DirRTL
	// PrimPicturePlaceholder shows the fill color, no pixels yet.
	PrimPicturePlaceholder = prim.PicturePlaceholder
	// PrimPictureReady shows decoded pixels.
	PrimPictureReady = prim.PictureReady
	// PrimPictureError shows the error chrome.
	PrimPictureError = prim.PictureError
	// PrimCrossStart packs at the cross start.
	PrimCrossStart = prim.CrossStart
	// PrimCrossCenter centers on the cross axis.
	PrimCrossCenter = prim.CrossCenter
	// PrimCrossStretch documents full-cross fill.
	PrimCrossStretch = prim.CrossStretch
)

// Layout facades.
func PrimPadOuter(child rendering.Size, l, t, r, b float64) rendering.Size {
	return prim.PadOuter(child, l, t, r, b)
}

// PrimPadChildConstraints shrinks parent constraints by insets.
func PrimPadChildConstraints(parent rendering.Constraints, l, t, r, b float64) rendering.Constraints {
	return prim.PadChildConstraints(parent, l, t, r, b)
}

// PrimNewPadBox wraps child with uniform pad.
func PrimNewPadBox(child rendering.RenderObject, pad float64) *rendering.RenderBox {
	return prim.NewPadBox(child, pad)
}

// PrimCenter centers the child.
func PrimCenter(child rendering.RenderObject) *rendering.RenderAlignBox {
	return prim.Center(child)
}

// PrimAlignBox aligns the child fractionally.
func PrimAlignBox(child rendering.RenderObject, ax, ay float64) *rendering.RenderAlignBox {
	return prim.AlignBox(child, ax, ay)
}

// PrimNewSizedBox builds a fixed-size host.
func PrimNewSizedBox(w, h float64, child rendering.RenderObject) *rendering.RenderBox {
	return prim.NewSizedBox(w, h, child)
}

// PrimConstrainSize tightens a child size into limits.
func PrimConstrainSize(l PrimLimits, parent rendering.Constraints, child rendering.Size) rendering.Size {
	return prim.ConstrainSize(l, parent, child)
}

// PrimAspectFit fits the largest ratio box inside parent.
func PrimAspectFit(parent rendering.Constraints, ratio float64) rendering.Size {
	return prim.AspectFit(parent, ratio)
}

// PrimFixedSpec wraps a measured child.
func PrimFixedSpec(w, h float64) PrimFlexSpec { return prim.FixedSpec(w, h) }

// PrimExpandedSpec must fill its share.
func PrimExpandedSpec(flex int) PrimFlexSpec { return prim.ExpandedSpec(flex) }

// PrimFlexibleSpec may be smaller than its share.
func PrimFlexibleSpec(flex int) PrimFlexSpec { return prim.FlexibleSpec(flex) }

// PrimSpacerSpec is empty flex space.
func PrimSpacerSpec(flex int) PrimFlexSpec { return prim.SpacerSpec(flex) }

// PrimFlexLayout computes Row/Column geometry.
func PrimFlexLayout(horizontal bool, mainMax, crossMax, spacing float64, dir PrimDirection, cross PrimCrossAlign, items []PrimFlexSpec) PrimFlexResult {
	return prim.FlexLayout(horizontal, mainMax, crossMax, spacing, dir, cross, items)
}

// PrimStackedSpec wraps a normal layer.
func PrimStackedSpec(w, h float64) PrimStackSpec { return prim.StackedSpec(w, h) }

// PrimPositionedSpec wraps an explicit layer.
func PrimPositionedSpec(w, h, x, y, pw, ph float64) PrimStackSpec {
	return prim.PositionedSpec(w, h, x, y, pw, ph)
}

// PrimStackLayout computes stack geometry.
func PrimStackLayout(parent rendering.Constraints, layers []PrimStackSpec) PrimStackResult {
	return prim.StackLayout(parent, layers)
}

// PrimIndexedOuter sizes a page switcher to the largest page.
func PrimIndexedOuter(parent rendering.Constraints, pages []rendering.Size) rendering.Size {
	return prim.IndexedOuter(parent, sizesToRendering(pages))
}

func sizesToRendering(in []rendering.Size) []rendering.Size { return in }

// PrimWrapLayout flows children into runs.
func PrimWrapLayout(parent rendering.Constraints, spacing, runSpacing float64, cross PrimCrossAlign, children []rendering.Size) PrimWrapResult {
	return prim.WrapLayout(parent, spacing, runSpacing, cross, children)
}

// PrimNewViewportBox builds a scrollable fixed-extent host.
func PrimNewViewportBox(count int, extent float64, builder rendering.ItemBuilder) *PrimViewportBox {
	return prim.NewViewportBox(count, extent, builder)
}

// PrimNewVariableViewportBox builds a variable-height host.
func PrimNewVariableViewportBox(count int, fallback float64, extentAt rendering.ItemExtentFunc, builder rendering.ItemBuilder) *PrimViewportBox {
	return prim.NewVariableViewportBox(count, fallback, extentAt, builder)
}

// Decor facades.
func PrimBlendSrcOver(dst, src [3]float64, opacity float64) [3]float64 {
	return prim.BlendSrcOver(dst, src, opacity)
}

// PrimRRectContains reports rounded-rect containment.
func PrimRRectContains(x, y, w, h, radius float64) bool {
	return prim.RRectContains(x, y, w, h, radius)
}

// PrimOvalContains reports ellipse containment.
func PrimOvalContains(x, y, w, h float64) bool { return prim.OvalContains(x, y, w, h) }

// PrimGradientSample samples a two-stop gradient.
func PrimGradientSample(from, to theme.Color, t float64) theme.Color {
	return prim.GradientSample(from, to, t)
}

// PrimGradientMonotonic checks midpoint_claim.
func PrimGradientMonotonic(from, mid, to theme.Color) bool {
	return prim.GradientMonotonic(from, mid, to)
}

// PrimColored builds a solid box.
func PrimColored(w, h float64, c theme.Color) *rendering.RenderColorBox {
	return prim.Colored(w, h, c)
}

// PrimResolveDecor merges decoration levels.
func PrimResolveDecor(props PrimDecorProps, ctheme *PrimDecorSpec, seed theme.Tokens) PrimDecorSpec {
	return prim.ResolveDecor(props, ctheme, seed)
}

// PrimNewDecoratedBox builds the decorated box.
func PrimNewDecoratedBox(w, h float64, spec PrimDecorSpec, child rendering.RenderObject) *rendering.RenderClipRRect {
	return prim.NewDecoratedBox(w, h, spec, child)
}

// PrimNewClipRect clips to a hard rectangle.
func PrimNewClipRect(child rendering.RenderObject) *rendering.RenderClipRRect {
	return prim.NewClipRect(child)
}

// PrimNewClipRRect clips to a rounded rectangle.
func PrimNewClipRRect(radius float64, child rendering.RenderObject) *rendering.RenderClipRRect {
	return prim.NewClipRRect(radius, child)
}

// PrimNewOpacity wraps group opacity.
func PrimNewOpacity(opacity float64, children ...rendering.RenderObject) *rendering.RenderOpacity {
	return prim.NewOpacity(opacity, children...)
}

// PrimNewTransformed wraps rotation and scale.
func PrimNewTransformed(rotation, sx, sy float64, children ...rendering.RenderObject) *rendering.RenderTransform {
	return prim.NewTransformed(rotation, sx, sy, children...)
}

// PrimFittedScale computes the fit scale.
func PrimFittedScale(parentW, parentH, childW, childH float64) float64 {
	return prim.FittedScale(parentW, parentH, childW, childH)
}

// PrimNewCustomPaint builds a custom-paint box.
func PrimNewCustomPaint(w, h float64, painter PrimPainter, boundary bool, child rendering.RenderObject) *rendering.RenderBox {
	return prim.NewCustomPaint(w, h, painter, boundary, child)
}

// Content facades.
func PrimResolveTextStyle(seed theme.Tokens, chain ...PrimTextStyle) PrimTextStyle {
	return prim.ResolveTextStyle(seed, chain...)
}

// PrimNewLabel builds single-style text.
func PrimNewLabel(p PrimLabelProps) *rendering.RenderText { return prim.NewLabel(p) }

// PrimNewRichLabel builds multi-span text.
func PrimNewRichLabel(p PrimRichProps) *rendering.RenderText { return prim.NewRichLabel(p) }

// PrimEstimateSize measures text with the no-face heuristic.
func PrimEstimateSize(s string, fontSize, approxW, lineMult, maxWidth float64, maxLines int, ellipsis bool) rendering.Size {
	return prim.EstimateSize(s, fontSize, approxW, lineMult, maxWidth, maxLines, ellipsis)
}

// PrimResolvePicture fills placeholder geometry.
func PrimResolvePicture(w, h float64, seed theme.Tokens) PrimPictureSpec {
	return prim.ResolvePicture(w, h, seed)
}

// PrimNewPicture builds the placeholder image node.
func PrimNewPicture(spec PrimPictureSpec) *rendering.RenderImage {
	return prim.NewPicture(spec)
}

// PrimPicturePhase maps the node state to the facade phase.
func PrimPicturePhase(im *rendering.RenderImage) PrimPictureState {
	return prim.PicturePhase(im)
}

// PrimResolveIcon resolves icon size and color from Ctx.
func PrimResolveIcon(ctx ScopeCtx, seed theme.Tokens) PrimIconSpec {
	return prim.ResolveIcon(ctx, seed)
}

// PrimIconBoxSize reports the square icon box.
func PrimIconBoxSize(spec PrimIconSpec) rendering.Size {
	return prim.IconBoxSize(spec)
}
