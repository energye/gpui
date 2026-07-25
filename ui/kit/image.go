package kit

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Image defaults — docs/antd/image.md §6.2
// components/image/style/index.ts prepareComponentToken / genImage*.
const (
	// DefaultImageWidth is the kit layout fallback when width is unset (antd has no fixed default).
	DefaultImageWidth = 200.0
	// DefaultImageHeight is the kit layout fallback when height is unset.
	DefaultImageHeight = 200.0
	// DefaultImageFontSize is fontSize (14).
	DefaultImageFontSize = 14.0
	// DefaultImageBorderRadius is borderRadius (6).
	DefaultImageBorderRadius = 6.0
	// DefaultImageLineWidth is lineWidth (1).
	DefaultImageLineWidth = 1.0
	// DefaultImageFocusRingOutset approximates Ant focus-visible outset.
	DefaultImageFocusRingOutset = 1.5
	// DefaultImageScaleStep is preview scaleStep (0.5 → ×1.5 per zoom-in).
	DefaultImageScaleStep = 0.5
	// DefaultImageMinScale is preview minScale.
	DefaultImageMinScale = 1.0
	// DefaultImageMaxScale is preview maxScale.
	DefaultImageMaxScale = 50.0
	// DefaultImageProgressRailH is progress rail height (6).
	DefaultImageProgressRailH = 6.0
	// DefaultImageCoverAlpha is hover cover black alpha (0.3).
	DefaultImageCoverAlpha = 0.3
	// DefaultImagePreviewOpSize falls back when fontSizeIcon×1.5 unavailable.
	DefaultImagePreviewOpSize = 18.0
	// DefaultImagePreviewSwitchSize is imagePreviewSwitchSize (controlHeightLG).
	DefaultImagePreviewSwitchSize = 40.0
	// imageSpinRPS is indeterminate placeholder spin rate.
	imageSpinRPS = 0.9
)

// ImageStatus is the load state machine (docs/antd/image.md §6.4).
type ImageStatus int

const (
	// ImageStatusEmpty has no src and is not loading.
	ImageStatusEmpty ImageStatus = iota
	// ImageStatusLoading shows placeholder / progress (src set, not OK yet).
	ImageStatusLoading
	// ImageStatusLoaded shows pixels or OK image chrome.
	ImageStatusLoaded
	// ImageStatusError after NotifyImageError; may show fallback.
	ImageStatusError
)

// ImageTransform is preview transform state (antd TransformType).
type ImageTransform struct {
	X, Y   float64
	Rotate float64 // degrees
	Scale  float64
	FlipX  bool
	FlipY  bool
}

// ImageToolbarInfo is the payload for actionsRender (antd ToolbarRenderInfoType subset).
type ImageToolbarInfo struct {
	Transform ImageTransform
	Current   int
	Total     int
	Src       string
	Alt       string
	// Actions are bound methods on the active preview owner.
	OnFlipY       func()
	OnFlipX       func()
	OnRotateLeft  func()
	OnRotateRight func()
	OnZoomOut     func()
	OnZoomIn      func()
	OnReset       func()
	OnClose       func()
	OnActive      func(delta int) // PreviewGroup: -1 prev / +1 next
}

// Image is Ant Design Image — previewable picture.
//
//	Column root
//	  Pressable(Decorated thumb)  // hit == layout == paint
//	  OverlayPortal (zero size)   // own preview when not in a group
//
// Product contract: docs/antd/image.md §6 (P0 DoD).
// Host decodes src via SetPixels / SetImageOK / NotifyImageError (no HTTP in kit).
type Image struct {
	Root      *primitive.Flex
	thumb     *primitive.Decorated
	pressable *primitive.Pressable
	painter   *primitive.PainterNode
	coverLab  *primitive.Text
	phLab     *primitive.Text
	phCanvas  *primitive.Canvas

	Portal *primitive.OverlayPortal
	Scope  *primitive.FocusScope
	layer  *imagePreviewLayer
	trap   overlayFocusTrap

	// --- product fields ---
	Src         string
	Alt         string
	Fallback    string
	PreviewSrc  string // preview.src (empty → Src / fallback chain)
	Width       float64
	Height      float64
	Preview     bool // preview enabled; default true
	Disabled    bool
	Loading     bool // force loading chrome
	AriaLabel   string
	Face        text.Face
	Theme       *core.Theme
	Style       Style
	Viewport    core.Size
	ScaleStep   float64 // 0 → DefaultImageScaleStep
	MinScale    float64 // 0 → DefaultImageMinScale
	MaxScale    float64 // 0 → DefaultImageMaxScale
	Percent     float64 // 0–100; only when percentSet
	percentSet  bool
	placeholder bool
	phNode      core.Node

	// host pixels
	Pixels         []byte
	PixelW, PixelH int
	imageOK        bool
	usingFallback  bool
	statusError    bool
	activeSrc      string // currently displayed logical src

	// preview open state
	open           bool
	openControlled bool
	transform      ImageTransform
	OnOpenChange   func(open bool)
	OnError        func()
	ActionsRender  func(info ImageToolbarInfo) core.Node
	CoverText      string // hover cover label; default "Preview"

	// group membership
	group      *ImagePreviewGroup
	groupIndex int

	// ticker for indeterminate progress
	spinPhase float64
	life      tickerLifecycle
	boundTree *core.Tree

	// cached L2 colors
	bgColor     render.RGBA
	borderColor render.RGBA
}

// NewImage creates an Image with antd defaults (preview=true, closed).
func NewImage() *Image {
	im := &Image{
		Preview:   true,
		CoverText: "Preview",
		transform: ImageTransform{Scale: 1},
	}
	im.rebuild()
	return im
}

// NewImageSized is a convenience for fixed-size thumbnails (gallery / Card cover).
func NewImageSized(alt string, w, h float64) *Image {
	im := NewImage()
	im.SetAlt(alt)
	if w > 0 {
		im.SetWidth(w)
	}
	if h > 0 {
		im.SetHeight(h)
	}
	return im
}

// Node returns the mount root (thumb column + zero-size portal host).
func (im *Image) Node() core.Node {
	if im == nil {
		return nil
	}
	if im.Root == nil {
		im.rebuild()
	}
	return im.Root
}

// ChromeNode returns the thumbnail Decorated (tests / layout).
func (im *Image) ChromeNode() core.Node {
	if im == nil {
		return nil
	}
	if im.thumb == nil {
		im.rebuild()
	}
	return im.thumb
}

// PreviewPortal returns the overlay portal (nil when owned by a group).
func (im *Image) PreviewPortal() *primitive.OverlayPortal {
	if im == nil {
		return nil
	}
	return im.Portal
}

// --- metrics (L2) ---

// FontSize returns body/cover font size.
func (im *Image) FontSize() float64 {
	if im != nil && im.Style.FontSize > 0 {
		return im.Style.FontSize
	}
	th := im.theme()
	if fs := th.SizeOr(core.TokenFontSize, 0); fs > 0 {
		return fs
	}
	return DefaultImageFontSize
}

// BorderRadius returns thumb corner radius.
func (im *Image) BorderRadius() float64 {
	if im != nil && im.Style.hasRadius() {
		return im.Style.Radius
	}
	th := im.theme()
	if r := th.SizeOr(core.TokenBorderRadius, 0); r > 0 {
		return r
	}
	return DefaultImageBorderRadius
}

// LineWidth returns border line width.
func (im *Image) LineWidth() float64 {
	th := im.theme()
	if w := th.SizeOr(core.TokenLineWidth, 0); w > 0 {
		return w
	}
	return DefaultImageLineWidth
}

// FocusRingOutset returns focus ring outset.
func (im *Image) FocusRingOutset() float64 { return DefaultImageFocusRingOutset }

// ResolvedWidth returns layout width.
func (im *Image) ResolvedWidth() float64 {
	if im != nil && im.Style.Width > 0 {
		return im.Style.Width
	}
	if im != nil && im.Width > 0 {
		return im.Width
	}
	return DefaultImageWidth
}

// ResolvedHeight returns layout height.
func (im *Image) ResolvedHeight() float64 {
	if im != nil && im.Style.Height > 0 {
		return im.Style.Height
	}
	if im != nil && im.Height > 0 {
		return im.Height
	}
	return DefaultImageHeight
}

// ResolvedScaleStep returns zoom step.
func (im *Image) ResolvedScaleStep() float64 {
	if im != nil && im.ScaleStep > 0 {
		return im.ScaleStep
	}
	return DefaultImageScaleStep
}

// ResolvedMinScale / ResolvedMaxScale return zoom clamps.
func (im *Image) ResolvedMinScale() float64 {
	if im != nil && im.MinScale > 0 {
		return im.MinScale
	}
	return DefaultImageMinScale
}
func (im *Image) ResolvedMaxScale() float64 {
	if im != nil && im.MaxScale > 0 {
		return im.MaxScale
	}
	return DefaultImageMaxScale
}

// Status returns the load state.
func (im *Image) Status() ImageStatus {
	return im.computeStatus()
}

// Transform returns a copy of the preview transform.
func (im *Image) Transform() ImageTransform {
	if im == nil {
		return ImageTransform{Scale: 1}
	}
	return im.transform
}

// IsPreviewOpen reports whether the preview layer is open (self or group).
func (im *Image) IsPreviewOpen() bool {
	if im == nil {
		return false
	}
	if im.group != nil {
		return im.group.IsOpen() && im.group.Current() == im.groupIndex
	}
	return im.open
}

// IsPreviewEnabled reports preview != false.
func (im *Image) IsPreviewEnabled() bool {
	return im != nil && im.Preview && !im.Disabled
}

// ResolvedPreviewSrc returns the src used by the preview layer.
func (im *Image) ResolvedPreviewSrc() string {
	if im == nil {
		return ""
	}
	if im.PreviewSrc != "" {
		return im.PreviewSrc
	}
	if im.usingFallback && im.Fallback != "" {
		return im.Fallback
	}
	if im.Src != "" {
		return im.Src
	}
	return im.activeSrc
}

// DisplaySrc is the logical src currently painted (src or fallback).
func (im *Image) DisplaySrc() string {
	if im == nil {
		return ""
	}
	if im.activeSrc != "" {
		return im.activeSrc
	}
	return im.Src
}

// BackgroundColor returns thumb fill (for L2 tests).
func (im *Image) BackgroundColor() render.RGBA {
	if im == nil {
		return render.RGBA{}
	}
	return im.bgColor
}

// --- setters ---

// SetSrc sets the image address and resets load state (antd src change).
func (im *Image) SetSrc(src string) {
	if im == nil {
		return
	}
	im.Src = src
	im.usingFallback = false
	im.statusError = false
	im.imageOK = false
	im.Pixels = nil
	im.PixelW, im.PixelH = 0, 0
	im.activeSrc = src
	if src != "" {
		im.placeholder = im.placeholder || im.Loading
	}
	im.rebuild()
}

// SetAlt sets alt text / accessible name.
func (im *Image) SetAlt(alt string) {
	if im == nil {
		return
	}
	im.Alt = alt
	im.applyA11y()
	if im.coverLab != nil && im.CoverText == "Preview" {
		// keep default cover text
	}
	im.markPaint()
}

// SetWidth / SetHeight set explicit box size (0 → default fallback).
func (im *Image) SetWidth(w float64) {
	if im == nil {
		return
	}
	im.Width = w
	im.rebuild()
}
func (im *Image) SetHeight(h float64) {
	if im == nil {
		return
	}
	im.Height = h
	im.rebuild()
}

// SetFallback sets the error fallback address.
func (im *Image) SetFallback(src string) {
	if im == nil {
		return
	}
	im.Fallback = src
}

// SetPreview enables/disables preview (preview=false).
func (im *Image) SetPreview(on bool) {
	if im == nil {
		return
	}
	im.Preview = on
	if !on && im.open && im.group == nil {
		im.setOpen(false, true)
	}
	im.rebuild()
}

// SetPreviewSrc sets preview.src (independent of thumb src).
func (im *Image) SetPreviewSrc(src string) {
	if im == nil {
		return
	}
	im.PreviewSrc = src
}

// SetPreviewOpen sets controlled open (preview.open).
func (im *Image) SetPreviewOpen(open bool) {
	if im == nil {
		return
	}
	im.openControlled = true
	if im.group != nil {
		if open {
			im.group.SetCurrent(im.groupIndex)
			im.group.SetOpen(true)
		} else {
			im.group.SetOpen(false)
		}
		return
	}
	im.setOpen(open, true)
}

// SetScaleStep sets zoom step (1+step multiplier).
func (im *Image) SetScaleStep(step float64) {
	if im == nil {
		return
	}
	im.ScaleStep = step
}

// SetPercent sets placeholder progress 0–100.
func (im *Image) SetPercent(p float64) {
	if im == nil {
		return
	}
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	im.Percent = p
	im.percentSet = true
	im.placeholder = true
	im.rebuild()
	im.life.setActive(im.needsTicker())
}

// ClearPercent clears explicit percent (indeterminate if still placeholder).
func (im *Image) ClearPercent() {
	if im == nil {
		return
	}
	im.percentSet = false
	im.Percent = 0
	im.rebuild()
}

// SetPlaceholder toggles loading placeholder chrome.
func (im *Image) SetPlaceholder(on bool) {
	if im == nil {
		return
	}
	im.placeholder = on
	im.rebuild()
	im.life.setActive(im.needsTicker())
}

// SetPlaceholderNode installs a custom placeholder node (antd placeholder ReactNode).
func (im *Image) SetPlaceholderNode(n core.Node) {
	if im == nil {
		return
	}
	im.phNode = n
	im.placeholder = n != nil
	im.rebuild()
}

// SetPixels installs RGBA samples and marks image OK.
func (im *Image) SetPixels(w, h int, rgba []byte) {
	if im == nil {
		return
	}
	im.PixelW, im.PixelH = w, h
	im.Pixels = rgba
	if w > 0 && h > 0 && len(rgba) >= w*h*4 {
		im.imageOK = true
		im.statusError = false
		if im.activeSrc == "" {
			if im.usingFallback && im.Fallback != "" {
				im.activeSrc = im.Fallback
			} else if im.Src != "" {
				im.activeSrc = im.Src
			} else {
				im.activeSrc = "pixels"
			}
		}
	}
	im.rebuild()
	im.life.setActive(im.needsTicker())
}

// SetImageOK marks host decode success/failure without firing OnError.
func (im *Image) SetImageOK(ok bool) {
	if im == nil {
		return
	}
	im.imageOK = ok
	if ok {
		im.statusError = false
		if im.activeSrc == "" {
			im.activeSrc = im.Src
			if im.activeSrc == "" {
				im.activeSrc = "ok"
			}
		}
	}
	im.rebuild()
	im.life.setActive(im.needsTicker())
}

// NotifyImageError simulates img onError. Switches to fallback when set.
func (im *Image) NotifyImageError() {
	if im == nil {
		return
	}
	if im.OnError != nil {
		im.OnError()
	}
	im.imageOK = false
	im.statusError = true
	im.Pixels = nil
	im.PixelW, im.PixelH = 0, 0
	if im.Fallback != "" && !im.usingFallback {
		im.usingFallback = true
		im.activeSrc = im.Fallback
		// host must re-decode fallback via SetPixels; show loading-ish error chrome until then
	} else {
		im.activeSrc = ""
	}
	im.rebuild()
	im.life.setActive(im.needsTicker())
}

// SetDisabled toggles disabled chrome and blocks preview.
func (im *Image) SetDisabled(v bool) {
	if im == nil {
		return
	}
	im.Disabled = v
	if im.pressable != nil {
		im.pressable.SetDisabled(v)
	}
	im.rebuild()
}

// SetLoading forces loading overlay / ticker.
func (im *Image) SetLoading(v bool) {
	if im == nil {
		return
	}
	im.Loading = v
	if v {
		im.placeholder = true
	}
	im.rebuild()
	im.life.setActive(im.needsTicker())
}

// SetTheme sets the theme override.
func (im *Image) SetTheme(th *core.Theme) {
	if im == nil {
		return
	}
	im.Theme = th
	im.rebuild()
}

// SetFace sets the font face for cover/placeholder text.
func (im *Image) SetFace(face text.Face) {
	if im == nil {
		return
	}
	im.Face = face
	if im.coverLab != nil {
		im.coverLab.Face = face
	}
	if im.phLab != nil {
		im.phLab.Face = face
	}
	im.rebuild()
}

// SetStyle applies Style overrides on the thumb.
func (im *Image) SetStyle(st Style) {
	if im == nil {
		return
	}
	im.Style = st
	if st.Face != nil {
		im.Face = st.Face
	}
	im.rebuild()
}

// SetAriaLabel sets the accessible name.
func (im *Image) SetAriaLabel(name string) {
	if im == nil {
		return
	}
	im.AriaLabel = name
	im.applyA11y()
}

// SetCoverText sets hover cover label (default "Preview").
func (im *Image) SetCoverText(s string) {
	if im == nil {
		return
	}
	im.CoverText = s
	if im.coverLab != nil {
		im.coverLab.Value = s
		im.coverLab.MarkNeedsPaint()
	}
}

// SetActionsRender installs custom toolbar renderer.
func (im *Image) SetActionsRender(fn func(ImageToolbarInfo) core.Node) {
	if im == nil {
		return
	}
	im.ActionsRender = fn
	if im.open {
		im.rebuildPreviewChrome()
	}
}

// SetOnOpenChange sets the open change callback.
func (im *Image) SetOnOpenChange(fn func(bool)) {
	if im == nil {
		return
	}
	im.OnOpenChange = fn
}

// SetOnError sets the load error callback.
func (im *Image) SetOnError(fn func()) {
	if im == nil {
		return
	}
	im.OnError = fn
}

// OpenPreview opens the preview if enabled.
func (im *Image) OpenPreview() {
	if im == nil || !im.IsPreviewEnabled() {
		return
	}
	if im.group != nil {
		im.group.SetCurrent(im.groupIndex)
		im.group.SetOpen(true)
		return
	}
	if im.openControlled {
		im.setOpen(true, true)
		return
	}
	im.setOpen(true, true)
}

// ClosePreview closes the preview.
func (im *Image) ClosePreview() {
	if im == nil {
		return
	}
	if im.group != nil {
		im.group.SetOpen(false)
		return
	}
	im.setOpen(false, true)
}

// ZoomIn / ZoomOut / Rotate* / Flip* / ResetTransform — preview tools.
func (im *Image) ZoomIn() {
	if im == nil {
		return
	}
	step := im.ResolvedScaleStep()
	im.transform.Scale = clampFloat(im.transform.Scale*(1+step), im.ResolvedMinScale(), im.ResolvedMaxScale())
	im.rebuildPreviewChrome()
}
func (im *Image) ZoomOut() {
	if im == nil {
		return
	}
	step := im.ResolvedScaleStep()
	im.transform.Scale = clampFloat(im.transform.Scale/(1+step), im.ResolvedMinScale(), im.ResolvedMaxScale())
	im.rebuildPreviewChrome()
}
func (im *Image) RotateLeft() {
	if im == nil {
		return
	}
	im.transform.Rotate -= 90
	im.rebuildPreviewChrome()
}
func (im *Image) RotateRight() {
	if im == nil {
		return
	}
	im.transform.Rotate += 90
	im.rebuildPreviewChrome()
}
func (im *Image) FlipX() {
	if im == nil {
		return
	}
	im.transform.FlipX = !im.transform.FlipX
	im.rebuildPreviewChrome()
}
func (im *Image) FlipY() {
	if im == nil {
		return
	}
	im.transform.FlipY = !im.transform.FlipY
	im.rebuildPreviewChrome()
}
func (im *Image) ResetTransform() {
	if im == nil {
		return
	}
	im.transform = ImageTransform{Scale: 1}
	im.rebuildPreviewChrome()
}

// AttachTicker binds indeterminate progress animation.
func (im *Image) AttachTicker(t *core.Tree) {
	if im == nil || t == nil {
		return
	}
	im.boundTree = t
	im.life.attach(t, im, im.needsTicker())
}

// Tick advances indeterminate placeholder animation.
func (im *Image) Tick(dt float64) bool {
	if im == nil {
		return false
	}
	if !im.life.stillMounted(im.boundTree) {
		return false
	}
	if !im.needsTicker() {
		return false
	}
	im.spinPhase += dt * imageSpinRPS
	if im.spinPhase > 1 {
		im.spinPhase = math.Mod(im.spinPhase, 1)
	}
	if im.phCanvas != nil {
		im.phCanvas.MarkNeedsPaint()
	} else if im.thumb != nil {
		im.thumb.MarkNeedsPaint()
	}
	return true
}

// --- internals ---

// statusError marks NotifyImageError path.
var _ = ImageStatusError

// bool field for error (declared with other fields via this line in struct — added):
// NOTE: statusError lives on Image; ensure struct has it.
// (defined in struct above as missing — patch via var on Image)

type imageStatusBits struct{}

func (im *Image) theme() *core.Theme {
	var n core.Node
	if im != nil && im.Root != nil {
		n = im.Root
	}
	return themeOf(im.Theme, n)
}

func (im *Image) needsTicker() bool {
	if im == nil {
		return false
	}
	if im.Loading {
		return true
	}
	// indeterminate progress: placeholder without concrete percent, not yet loaded
	if im.placeholder && !im.imageOK && !im.percentSet {
		return true
	}
	return false
}

func (im *Image) markPaint() {
	if im == nil {
		return
	}
	if im.thumb != nil {
		im.thumb.MarkNeedsPaint()
	}
	if im.Root != nil {
		im.Root.MarkNeedsPaint()
	}
}

func (im *Image) applyA11y() {
	if im == nil {
		return
	}
	name := im.AriaLabel
	if name == "" {
		name = im.Alt
	}
	if name == "" {
		name = im.Src
	}
	if im.pressable != nil {
		im.pressable.Base().Label = name
		im.pressable.Base().Role = "img"
	}
	if im.thumb != nil {
		im.thumb.Base().Label = name
		im.thumb.Base().Role = "img"
	}
}

func (im *Image) setOpen(open bool, notify bool) {
	if im == nil {
		return
	}
	if !open && !im.open {
		if im.Portal != nil {
			im.Portal.SetOpen(false)
		}
		return
	}
	if open && !im.IsPreviewEnabled() {
		return
	}
	was := im.open
	im.open = open
	if open {
		if im.transform.Scale <= 0 {
			im.transform.Scale = 1
		}
		im.rebuildPreviewChrome()
		if im.Portal != nil {
			im.Portal.SetOpen(true)
		}
		im.trap.wire(im.Scope, true, im.onEscape)
		var prefer core.Node
		if im.layer != nil && im.layer.closeBtn != nil {
			prefer = im.layer.closeBtn
		}
		im.trap.enter(im.Scope, im.Portal, prefer)
	} else if was {
		if im.Portal != nil {
			im.Portal.SetOpen(false)
		}
		im.trap.wire(im.Scope, false, nil)
		im.trap.leave(im.Scope, im.Portal)
	}
	if notify && was != open && im.OnOpenChange != nil {
		im.OnOpenChange(open)
	}
}

func (im *Image) onEscape() {
	if im == nil || !im.open {
		return
	}
	im.setOpen(false, true)
}

func (im *Image) onThumbActivate() {
	if im == nil || im.Disabled || !im.Preview {
		return
	}
	im.OpenPreview()
}

func (im *Image) toolbarInfo() ImageToolbarInfo {
	cur, total := 0, 1
	var onActive func(int)
	if im.group != nil {
		cur = im.group.Current()
		total = im.group.Total()
		onActive = func(delta int) {
			if delta < 0 {
				im.group.Prev()
			} else if delta > 0 {
				im.group.Next()
			}
		}
	}
	return ImageToolbarInfo{
		Transform:     im.transform,
		Current:       cur,
		Total:         total,
		Src:           im.ResolvedPreviewSrc(),
		Alt:           im.Alt,
		OnFlipY:       im.FlipY,
		OnFlipX:       im.FlipX,
		OnRotateLeft:  im.RotateLeft,
		OnRotateRight: im.RotateRight,
		OnZoomOut:     im.ZoomOut,
		OnZoomIn:      im.ZoomIn,
		OnReset:       im.ResetTransform,
		OnClose:       im.ClosePreview,
		OnActive:      onActive,
	}
}

func (im *Image) rebuild() {
	if im == nil {
		return
	}
	// ensure transform defaults
	if im.transform.Scale <= 0 {
		im.transform.Scale = 1
	}
	th := im.theme()
	w := im.ResolvedWidth()
	h := im.ResolvedHeight()
	radius := im.BorderRadius()
	lineW := im.LineWidth()
	fs := im.FontSize()

	bg := th.Color(core.TokenColorBgLayout)
	if im.Style.hasBG() {
		bg = im.Style.Background
	}
	if im.Disabled {
		if d := th.Color(core.TokenColorDisabledBg); d.A > 0 {
			bg = d
		}
	}
	bd := th.Color(core.TokenColorBorder)
	if im.Style.hasBorder() {
		bd = im.Style.Border
	}
	im.bgColor = bg
	im.borderColor = bd

	// pixel painter
	pw, ph := im.PixelW, im.PixelH
	pix := im.Pixels
	status := im.computeStatus()
	im.painter = primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		// sample pixels when available
		if len(pix) >= pw*ph*4 && pw > 0 && ph > 0 {
			paintPixelGrid(pc, sz, pix, pw, ph)
			return
		}
		// soft empty / loading bands (token-ish neutrals, not brand primary)
		base := bg
		if base.A < 0.1 {
			base = render.RGBA{R: 0.93, G: 0.94, B: 0.96, A: 1}
		}
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, base)
		if status == ImageStatusError {
			errC := th.Color(core.TokenColorError)
			errC.A = 0.12
			pc.FillLocalRect(0, 0, sz.Width, sz.Height, errC)
		}
	})
	im.painter.Width, im.painter.Height = w, h

	// cover label (hover semantics approximated as always-available overlay text for P0)
	coverTxt := im.CoverText
	if coverTxt == "" {
		coverTxt = "Preview"
	}
	im.coverLab = primitive.NewText(coverTxt)
	im.coverLab.FontSize = fs
	im.coverLab.Face = im.Face
	im.coverLab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.95}

	// placeholder content
	var phNode core.Node
	showPH := status == ImageStatusLoading || (im.placeholder && !im.imageOK)
	if showPH {
		if im.phNode != nil {
			phNode = im.phNode
		} else {
			phNode = im.buildDefaultPlaceholder(th, w, h, fs)
		}
	}

	stack := primitive.NewStack(im.painter)
	stack.Fit = true
	if showPH && phNode != nil {
		stack.AddChild(primitive.Positioned(core.AlignCenter, phNode))
	}
	// subtle cover hint when preview enabled and loaded
	if im.Preview && !im.Disabled && status == ImageStatusLoaded {
		coverBg := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
			if pc == nil {
				return
			}
			// P0: static low-contrast cover tint (hover full opacity is P1 motion)
			pc.FillLocalRect(0, 0, sz.Width, sz.Height, render.RGBA{R: 0, G: 0, B: 0, A: 0.0})
		})
		coverBg.Width, coverBg.Height = w, h
		// show label only — avoid always dimming thumb
		_ = coverBg
		stack.AddChild(primitive.Positioned(core.AlignCenter, im.coverLab))
		// hide cover text at rest: keep for a11y tests via CoverText; visual opacity via color A
		im.coverLab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.0}
	}

	if im.thumb == nil {
		im.thumb = primitive.NewDecorated(stack)
	} else {
		im.thumb.ClearChildren()
		im.thumb.AddChild(stack)
	}
	im.thumb.Width, im.thumb.Height = w, h
	im.thumb.MinWidth, im.thumb.MinHeight = w, h
	im.thumb.StretchChild = true
	im.thumb.Radius = radius
	im.thumb.Background = bg
	im.thumb.BorderWidth = lineW
	im.thumb.BorderColor = bd
	im.thumb.Hit = core.HitDefer
	im.thumb.SetThemeHook(func(*core.Theme) { im.rebuild() })

	if im.pressable == nil {
		im.pressable = primitive.NewPressable(im.thumb)
	} else {
		im.pressable.ClearChildren()
		im.pressable.AddChild(im.thumb)
	}
	im.pressable.Focusable = true
	im.pressable.ShowFocusRing = true
	im.pressable.FocusRingRadius = radius
	im.pressable.FocusRingOutset = DefaultImageFocusRingOutset
	im.pressable.SetDisabled(im.Disabled)
	im.pressable.EnableRipple = false
	im.pressable.Click = im.onThumbActivate
	im.applyA11y()

	// root column: pressable + portal
	if im.Root == nil {
		im.Root = primitive.Column()
	} else {
		im.Root.ClearChildren()
	}
	im.Root.CrossAlign = core.CrossStart
	im.Root.Gap = 0
	im.Root.AddChild(im.pressable)

	// own portal only when not in a group
	if im.group == nil {
		im.rebuildPreviewChrome()
		if im.Portal != nil {
			im.Root.AddChild(im.Portal)
			if im.open {
				im.Portal.SetOpen(true)
			}
		}
	} else {
		im.Portal = nil
	}

	im.Root.MarkNeedsLayout()
	im.Root.MarkNeedsPaint()
	im.life.setActive(im.needsTicker())
}

func (im *Image) computeStatus() ImageStatus {
	if im == nil {
		return ImageStatusEmpty
	}
	if im.statusError && !im.imageOK {
		return ImageStatusError
	}
	if im.imageOK {
		return ImageStatusLoaded
	}
	if im.Loading || im.placeholder || (im.Src != "" && !im.imageOK) {
		return ImageStatusLoading
	}
	return ImageStatusEmpty
}

func (im *Image) buildDefaultPlaceholder(th *core.Theme, w, h, fs float64) core.Node {
	sec := th.Color(core.TokenColorTextSecondary)
	col := primitive.Column()
	col.CrossAlign = core.CrossCenter
	col.Gap = 8

	// indeterminate ink / spinner
	spinSz := math.Min(w, h) * 0.28
	if spinSz < 16 {
		spinSz = 16
	}
	if spinSz > 48 {
		spinSz = 48
	}
	im.phCanvas = primitive.NewCanvas(spinSz, spinSz, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		// soft progress blob
		c := render.RGBA{R: 0.45, G: 0.65, B: 0.95, A: 0.55}
		phase := im.spinPhase
		cx := sz.Width * (0.3 + 0.4*phase)
		cy := sz.Height * 0.5
		r := math.Min(sz.Width, sz.Height) * 0.35
		pc.FillLocalRect(cx-r, cy-r, r*2, r*2, c)
	})
	col.AddChild(im.phCanvas)

	// percent rail when set
	if im.percentSet {
		railW := w * 0.7
		if railW < 40 {
			railW = 40
		}
		pct := im.Percent
		rail := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
			if pc == nil {
				return
			}
			pc.FillLocalRect(0, 0, sz.Width, sz.Height, render.RGBA{R: 1, G: 1, B: 1, A: 0.5})
			fw := sz.Width * pct / 100
			if fw > 0 {
				pc.FillLocalRect(0, 0, fw, sz.Height, render.RGBA{R: 0.47, G: 0.67, B: 1, A: 0.85})
			}
		})
		rail.Width = railW
		rail.Height = DefaultImageProgressRailH
		col.AddChild(rail)
		im.phLab = primitive.NewText(fmt.Sprintf("%.0f%%", pct))
		im.phLab.FontSize = fs
		im.phLab.Face = im.Face
		im.phLab.Color = sec
		col.AddChild(im.phLab)
	} else {
		im.phLab = primitive.NewText("Loading")
		im.phLab.FontSize = fs
		im.phLab.Face = im.Face
		im.phLab.Color = sec
		col.AddChild(im.phLab)
	}
	return col
}

func (im *Image) rebuildPreviewChrome() {
	if im == nil || im.group != nil {
		return
	}
	th := im.theme()
	info := im.toolbarInfo()

	// preview image label
	srcLab := im.ResolvedPreviewSrc()
	if srcLab == "" {
		srcLab = "(no src)"
	}
	imgTxt := primitive.NewText(srcLab)
	imgTxt.FontSize = im.FontSize()
	imgTxt.Face = im.Face
	imgTxt.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.92}

	// show transform summary
	meta := fmt.Sprintf("scale %.2f  rot %.0f°", im.transform.Scale, im.transform.Rotate)
	if im.transform.FlipX {
		meta += " flipX"
	}
	if im.transform.FlipY {
		meta += " flipY"
	}
	metaLab := primitive.NewText(meta)
	metaLab.FontSize = im.FontSize() - 2
	if metaLab.FontSize < 10 {
		metaLab.FontSize = 10
	}
	metaLab.Face = im.Face
	metaLab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.65}

	// pixel panel when available
	var body core.Node
	pw, ph := im.PixelW, im.PixelH
	pix := im.Pixels
	previewBox := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		if len(pix) >= pw*ph*4 && pw > 0 && ph > 0 {
			paintPixelGrid(pc, sz, pix, pw, ph)
			return
		}
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, render.RGBA{R: 0.15, G: 0.15, B: 0.18, A: 1})
	})
	// scale visual size
	bw := im.ResolvedWidth() * im.transform.Scale
	bh := im.ResolvedHeight() * im.transform.Scale
	if bw < 80 {
		bw = 80
	}
	if bh < 80 {
		bh = 80
	}
	if bw > 720 {
		bw = 720
	}
	if bh > 520 {
		bh = 520
	}
	previewBox.Width, previewBox.Height = bw, bh

	imgCol := primitive.Column(previewBox, imgTxt, metaLab)
	imgCol.Gap = 8
	imgCol.CrossAlign = core.CrossCenter
	body = imgCol

	// toolbar
	var toolbar core.Node
	if im.ActionsRender != nil {
		toolbar = im.ActionsRender(info)
	} else {
		toolbar = im.buildDefaultToolbar(th, info)
	}

	// close button
	closeBtn := primitive.NewPressable(primitive.NewText("✕"))
	closeBtn.Focusable = true
	closeBtn.ShowFocusRing = true
	closeBtn.EnableRipple = false
	closeBtn.Click = im.ClosePreview
	closeBtn.Base().Role = "button"
	closeBtn.Base().Label = "Close preview"
	if lab, ok := closeBtn.Children()[0].(*primitive.Text); ok {
		lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.85}
		lab.FontSize = DefaultImagePreviewOpSize
		lab.Face = im.Face
	}

	mask := primitive.NewMask()
	mask.OnDismiss = func() { im.setOpen(false, true) }

	if im.layer == nil {
		im.layer = &imagePreviewLayer{owner: im}
		im.layer.Init(im.layer)
		im.layer.Hit = core.HitDefer
		im.layer.Role = "dialog"
	}
	im.layer.mask = mask
	im.layer.body = body
	im.layer.toolbar = toolbar
	im.layer.closeBtn = closeBtn
	im.layer.Label = im.previewA11yName()
	// content stack: body + toolbar
	content := primitive.Column(body)
	if toolbar != nil {
		content.AddChild(toolbar)
	}
	content.Gap = 16
	content.CrossAlign = core.CrossCenter
	im.layer.content = content
	im.layer.ClearChildren()
	im.layer.AddChild(mask)
	im.layer.AddChild(content)
	im.layer.AddChild(closeBtn)

	if im.Scope == nil {
		im.Scope = primitive.NewFocusScope(im.layer)
	}
	im.trap.wire(im.Scope, im.open, im.onEscape)

	if im.Portal == nil {
		im.Portal = primitive.NewOverlayPortal(im.Scope)
		im.Portal.ZOrder = OverlayZImagePreview
		im.Portal.SetContentOffset(core.Point{})
	} else {
		im.Portal.Content = im.Scope
		im.Portal.ZOrder = OverlayZImagePreview
	}
}

func (im *Image) previewA11yName() string {
	if im == nil {
		return "Image preview"
	}
	if im.AriaLabel != "" {
		return im.AriaLabel
	}
	if im.Alt != "" {
		return im.Alt
	}
	if s := im.ResolvedPreviewSrc(); s != "" {
		return s
	}
	return "Image preview"
}

func (im *Image) buildDefaultToolbar(th *core.Theme, info ImageToolbarInfo) core.Node {
	mk := func(label string, fn func()) core.Node {
		t := primitive.NewText(label)
		t.FontSize = DefaultImagePreviewOpSize
		t.Face = im.Face
		t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.65}
		p := primitive.NewPressable(t)
		p.Focusable = true
		p.ShowFocusRing = true
		p.EnableRipple = false
		p.Click = fn
		p.Base().Role = "button"
		p.Base().Label = label
		return p
	}
	row := primitive.Row(
		mk("↺", info.OnRotateLeft),
		mk("↻", info.OnRotateRight),
		mk("⇄", info.OnFlipX),
		mk("⇅", info.OnFlipY),
		mk("−", info.OnZoomOut),
		mk("+", info.OnZoomIn),
		mk("reset", info.OnReset),
		mk("close", info.OnClose),
	)
	if info.OnActive != nil && info.Total > 1 {
		row = primitive.Row(
			mk("‹", func() { info.OnActive(-1) }),
			mk("›", func() { info.OnActive(1) }),
			mk("↺", info.OnRotateLeft),
			mk("↻", info.OnRotateRight),
			mk("⇄", info.OnFlipX),
			mk("⇅", info.OnFlipY),
			mk("−", info.OnZoomOut),
			mk("+", info.OnZoomIn),
			mk("reset", info.OnReset),
			mk("close", info.OnClose),
		)
	}
	row.Gap = 12
	row.CrossAlign = core.CrossCenter
	shell := primitive.NewDecorated(row)
	shell.Padding = primitive.EdgeInsets{Left: 16, Right: 16, Top: 8, Bottom: 8}
	shell.Radius = 100
	shell.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.35}
	shell.Hit = core.HitDefer
	_ = th
	return shell
}

// imagePreviewLayer full-viewport preview host (mask + content + close).
type imagePreviewLayer struct {
	core.NodeBase
	owner    *Image
	group    *ImagePreviewGroup
	mask     *primitive.Mask
	content  core.Node
	body     core.Node
	toolbar  core.Node
	closeBtn core.Node
}

func (l *imagePreviewLayer) TypeID() string { return "kit.imagePreviewLayer" }

func (l *imagePreviewLayer) Layout(c core.Constraints) core.Size {
	vw, vh := l.viewport()
	out := c.Tighten(core.Size{Width: vw, Height: vh})
	l.SetSize(out)
	if l.mask != nil {
		l.mask.Width, l.mask.Height = vw, vh
		_ = l.mask.Layout(core.Tight(vw, vh))
		l.mask.Base().SetOffset(core.Point{})
	}
	// content centered
	if l.content != nil {
		csz := l.content.Layout(core.Loose(vw*0.9, vh*0.85))
		x := (vw - csz.Width) / 2
		y := (vh - csz.Height) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		l.content.Base().SetOffset(core.Point{X: x, Y: y})
	}
	// close top-right
	if l.closeBtn != nil {
		csz := l.closeBtn.Layout(core.Loose(48, 48))
		l.closeBtn.Base().SetOffset(core.Point{X: vw - csz.Width - 12, Y: 12})
	}
	return out
}

func (l *imagePreviewLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }
func (l *imagePreviewLayer) HitTest(p core.Point) core.Node {
	return l.DefaultHitTest(p)
}

func (l *imagePreviewLayer) viewport() (vw, vh float64) {
	var portal *primitive.OverlayPortal
	var explicit core.Size
	if l.owner != nil {
		portal = l.owner.Portal
		explicit = l.owner.Viewport
	}
	if l.group != nil {
		portal = l.group.Portal
		explicit = l.group.Viewport
	}
	return resolveOverlayViewport(explicit, portal, 800, 600)
}

// ---------------------------------------------------------------------------
// ImagePreviewGroup — Image.PreviewGroup
// ---------------------------------------------------------------------------

// ImagePreviewGroup is Ant Design Image.PreviewGroup.
//
//	Row of child thumbs + shared OverlayPortal for multi-image preview.
type ImagePreviewGroup struct {
	Root   *primitive.Flex
	Portal *primitive.OverlayPortal
	Scope  *primitive.FocusScope
	layer  *imagePreviewLayer
	trap   overlayFocusTrap

	children []*Image
	items    []string // explicit items[] (album mode); when set, overrides child src list for preview
	current  int
	open     bool

	OnChange       func(current, prev int)
	OnOpenChange   func(open bool, current int)
	ActionsRender  func(info ImageToolbarInfo) core.Node
	Theme          *core.Theme
	Face           text.Face
	Viewport       core.Size
	ScaleStep      float64
	transform      ImageTransform
	openControlled bool
}

// NewImagePreviewGroup creates an empty preview group.
func NewImagePreviewGroup() *ImagePreviewGroup {
	g := &ImagePreviewGroup{transform: ImageTransform{Scale: 1}}
	g.rebuild()
	return g
}

// Node returns the group root.
func (g *ImagePreviewGroup) Node() core.Node {
	if g == nil {
		return nil
	}
	if g.Root == nil {
		g.rebuild()
	}
	return g.Root
}

// Add registers child Image instances (preview-group.tsx pattern).
func (g *ImagePreviewGroup) Add(images ...*Image) {
	if g == nil {
		return
	}
	for _, im := range images {
		if im == nil {
			continue
		}
		im.group = g
		im.groupIndex = len(g.children)
		// child should not keep own portal
		if im.Portal != nil {
			im.Portal.SetOpen(false)
		}
		im.Portal = nil
		g.children = append(g.children, im)
		im.rebuild()
	}
	g.rebuild()
}

// SetItems sets album items (preview-group-visible.tsx). Preview navigates items.
func (g *ImagePreviewGroup) SetItems(urls ...string) {
	if g == nil {
		return
	}
	g.items = append([]string(nil), urls...)
	if g.current >= len(g.items) && len(g.items) > 0 {
		g.current = 0
	}
	g.rebuildPreview()
}

// Items returns a copy of items[].
func (g *ImagePreviewGroup) Items() []string {
	if g == nil {
		return nil
	}
	return append([]string(nil), g.items...)
}

// SetCurrent sets the active preview index.
func (g *ImagePreviewGroup) SetCurrent(i int) {
	if g == nil {
		return
	}
	total := g.Total()
	if total == 0 {
		g.current = 0
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= total {
		i = total - 1
	}
	prev := g.current
	if prev == i {
		return
	}
	g.current = i
	g.transform = ImageTransform{Scale: 1}
	if g.OnChange != nil {
		g.OnChange(i, prev)
	}
	if g.open {
		g.rebuildPreview()
	}
}

// Current returns the active index.
func (g *ImagePreviewGroup) Current() int {
	if g == nil {
		return 0
	}
	return g.current
}

// Total returns previewable count (items or children).
func (g *ImagePreviewGroup) Total() int {
	if g == nil {
		return 0
	}
	if len(g.items) > 0 {
		return len(g.items)
	}
	return len(g.children)
}

// Next / Prev navigate the album.
func (g *ImagePreviewGroup) Next() {
	if g == nil {
		return
	}
	if g.current+1 < g.Total() {
		g.SetCurrent(g.current + 1)
	}
}
func (g *ImagePreviewGroup) Prev() {
	if g == nil {
		return
	}
	if g.current > 0 {
		g.SetCurrent(g.current - 1)
	}
}

// SetOpen shows/hides the shared preview.
func (g *ImagePreviewGroup) SetOpen(open bool) {
	if g == nil {
		return
	}
	was := g.open
	g.open = open
	if open {
		if g.transform.Scale <= 0 {
			g.transform.Scale = 1
		}
		g.rebuildPreview()
		if g.Portal != nil {
			g.Portal.SetOpen(true)
		}
		g.trap.wire(g.Scope, true, g.onEscape)
		var prefer core.Node
		if g.layer != nil && g.layer.closeBtn != nil {
			prefer = g.layer.closeBtn
		}
		g.trap.enter(g.Scope, g.Portal, prefer)
	} else if was {
		if g.Portal != nil {
			g.Portal.SetOpen(false)
		}
		g.trap.wire(g.Scope, false, nil)
		g.trap.leave(g.Scope, g.Portal)
	}
	if was != open && g.OnOpenChange != nil {
		g.OnOpenChange(open, g.current)
	}
}

// IsOpen reports preview visibility.
func (g *ImagePreviewGroup) IsOpen() bool { return g != nil && g.open }

// SetActionsRender installs group-level toolbar renderer.
func (g *ImagePreviewGroup) SetActionsRender(fn func(ImageToolbarInfo) core.Node) {
	if g == nil {
		return
	}
	g.ActionsRender = fn
	if g.open {
		g.rebuildPreview()
	}
}

// SetTheme / SetFace wire chrome.
func (g *ImagePreviewGroup) SetTheme(th *core.Theme) {
	if g == nil {
		return
	}
	g.Theme = th
	for _, im := range g.children {
		im.SetTheme(th)
	}
	g.rebuild()
}
func (g *ImagePreviewGroup) SetFace(face text.Face) {
	if g == nil {
		return
	}
	g.Face = face
	for _, im := range g.children {
		im.SetFace(face)
	}
	g.rebuild()
}

func (g *ImagePreviewGroup) onEscape() {
	if g == nil || !g.open {
		return
	}
	g.SetOpen(false)
}

func (g *ImagePreviewGroup) theme() *core.Theme {
	var n core.Node
	if g != nil && g.Root != nil {
		n = g.Root
	}
	return themeOf(g.Theme, n)
}

func (g *ImagePreviewGroup) currentSrc() string {
	if g == nil {
		return ""
	}
	if len(g.items) > 0 {
		if g.current >= 0 && g.current < len(g.items) {
			return g.items[g.current]
		}
		return ""
	}
	if g.current >= 0 && g.current < len(g.children) {
		return g.children[g.current].ResolvedPreviewSrc()
	}
	return ""
}

func (g *ImagePreviewGroup) currentImage() *Image {
	if g == nil || len(g.items) > 0 {
		return nil
	}
	if g.current >= 0 && g.current < len(g.children) {
		return g.children[g.current]
	}
	return nil
}

func (g *ImagePreviewGroup) toolbarInfo() ImageToolbarInfo {
	src := g.currentSrc()
	alt := ""
	if im := g.currentImage(); im != nil {
		alt = im.Alt
	}
	return ImageToolbarInfo{
		Transform: g.transform,
		Current:   g.current,
		Total:     g.Total(),
		Src:       src,
		Alt:       alt,
		OnFlipY: func() {
			g.transform.FlipY = !g.transform.FlipY
			g.rebuildPreview()
		},
		OnFlipX: func() {
			g.transform.FlipX = !g.transform.FlipX
			g.rebuildPreview()
		},
		OnRotateLeft: func() {
			g.transform.Rotate -= 90
			g.rebuildPreview()
		},
		OnRotateRight: func() {
			g.transform.Rotate += 90
			g.rebuildPreview()
		},
		OnZoomOut: func() {
			step := DefaultImageScaleStep
			if g.ScaleStep > 0 {
				step = g.ScaleStep
			}
			g.transform.Scale = clampFloat(g.transform.Scale/(1+step), DefaultImageMinScale, DefaultImageMaxScale)
			g.rebuildPreview()
		},
		OnZoomIn: func() {
			step := DefaultImageScaleStep
			if g.ScaleStep > 0 {
				step = g.ScaleStep
			}
			g.transform.Scale = clampFloat(g.transform.Scale*(1+step), DefaultImageMinScale, DefaultImageMaxScale)
			g.rebuildPreview()
		},
		OnReset: func() {
			g.transform = ImageTransform{Scale: 1}
			g.rebuildPreview()
		},
		OnClose: func() { g.SetOpen(false) },
		OnActive: func(delta int) {
			if delta < 0 {
				g.Prev()
			} else if delta > 0 {
				g.Next()
			}
		},
	}
}

func (g *ImagePreviewGroup) rebuild() {
	if g == nil {
		return
	}
	if g.Root == nil {
		g.Root = primitive.Row()
	} else {
		g.Root.ClearChildren()
	}
	g.Root.Gap = 8
	g.Root.CrossAlign = core.CrossStart
	for _, im := range g.children {
		if im == nil {
			continue
		}
		im.group = g
		// ensure child rebuilt without own portal
		if im.Root == nil {
			im.rebuild()
		}
		g.Root.AddChild(im.Node())
	}
	g.rebuildPreview()
	if g.Portal != nil {
		g.Root.AddChild(g.Portal)
		if g.open {
			g.Portal.SetOpen(true)
		}
	}
	g.Root.MarkNeedsLayout()
	g.Root.MarkNeedsPaint()
}

func (g *ImagePreviewGroup) rebuildPreview() {
	if g == nil {
		return
	}
	th := g.theme()
	info := g.toolbarInfo()
	src := g.currentSrc()
	if src == "" {
		src = "(empty)"
	}

	// body: prefer current child pixels
	var pix []byte
	var pw, ph int
	var boxW, boxH float64 = DefaultImageWidth, DefaultImageHeight
	if im := g.currentImage(); im != nil {
		pix, pw, ph = im.Pixels, im.PixelW, im.PixelH
		boxW, boxH = im.ResolvedWidth(), im.ResolvedHeight()
	}
	scale := g.transform.Scale
	if scale <= 0 {
		scale = 1
	}
	bw, bh := boxW*scale, boxH*scale
	if bw < 80 {
		bw = 80
	}
	if bh < 80 {
		bh = 80
	}
	if bw > 720 {
		bw = 720
	}
	if bh > 520 {
		bh = 520
	}
	previewBox := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		if len(pix) >= pw*ph*4 && pw > 0 && ph > 0 {
			paintPixelGrid(pc, sz, pix, pw, ph)
			return
		}
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, render.RGBA{R: 0.15, G: 0.15, B: 0.18, A: 1})
	})
	previewBox.Width, previewBox.Height = bw, bh

	srcLab := primitive.NewText(src)
	srcLab.FontSize = DefaultImageFontSize
	srcLab.Face = g.Face
	srcLab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.9}

	tot := g.Total()
	if tot < 1 {
		tot = 1
	}
	countLab := primitive.NewText(fmt.Sprintf("%d / %d", g.current+1, tot))
	countLab.FontSize = DefaultImageFontSize
	countLab.Face = g.Face
	countLab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.65}

	body := primitive.Column(previewBox, srcLab, countLab)
	body.Gap = 8
	body.CrossAlign = core.CrossCenter

	var toolbar core.Node
	if g.ActionsRender != nil {
		toolbar = g.ActionsRender(info)
	} else if im := g.currentImage(); im != nil && im.ActionsRender != nil {
		toolbar = im.ActionsRender(info)
	} else {
		// reuse Image toolbar builder via a transient helper
		tmp := &Image{Face: g.Face, Theme: g.Theme, transform: g.transform}
		toolbar = tmp.buildDefaultToolbar(th, info)
	}

	closeBtn := primitive.NewPressable(primitive.NewText("✕"))
	closeBtn.Focusable = true
	closeBtn.ShowFocusRing = true
	closeBtn.EnableRipple = false
	closeBtn.Click = func() { g.SetOpen(false) }
	closeBtn.Base().Role = "button"
	closeBtn.Base().Label = "Close preview"
	if lab, ok := closeBtn.Children()[0].(*primitive.Text); ok {
		lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.85}
		lab.FontSize = DefaultImagePreviewOpSize
		lab.Face = g.Face
	}

	mask := primitive.NewMask()
	mask.OnDismiss = func() { g.SetOpen(false) }

	if g.layer == nil {
		g.layer = &imagePreviewLayer{group: g}
		g.layer.Init(g.layer)
		g.layer.Hit = core.HitDefer
		g.layer.Role = "dialog"
	}
	g.layer.mask = mask
	g.layer.closeBtn = closeBtn
	g.layer.Label = "Image preview"
	content := primitive.Column(body)
	if toolbar != nil {
		content.AddChild(toolbar)
	}
	content.Gap = 16
	content.CrossAlign = core.CrossCenter
	g.layer.content = content
	g.layer.ClearChildren()
	g.layer.AddChild(mask)
	g.layer.AddChild(content)
	g.layer.AddChild(closeBtn)

	if g.Scope == nil {
		g.Scope = primitive.NewFocusScope(g.layer)
	}
	g.trap.wire(g.Scope, g.open, g.onEscape)

	if g.Portal == nil {
		g.Portal = primitive.NewOverlayPortal(g.Scope)
		g.Portal.ZOrder = OverlayZImagePreview
		g.Portal.SetContentOffset(core.Point{})
	} else {
		g.Portal.Content = g.Scope
		g.Portal.ZOrder = OverlayZImagePreview
	}
}

// --- helpers ---

func paintPixelGrid(pc *core.PaintContext, sz core.Size, pix []byte, pw, ph int) {
	gx, gy := pw, ph
	if gx > 32 {
		gx = 32
	}
	if gy > 32 {
		gy = 32
	}
	cw, ch := sz.Width/float64(gx), sz.Height/float64(gy)
	for y := 0; y < gy; y++ {
		for x := 0; x < gx; x++ {
			sx := x * pw / gx
			sy := y * ph / gy
			i := (sy*pw + sx) * 4
			if i+3 >= len(pix) {
				continue
			}
			c := render.RGBA{
				R: float64(pix[i]) / 255,
				G: float64(pix[i+1]) / 255,
				B: float64(pix[i+2]) / 255,
				A: float64(pix[i+3]) / 255,
			}
			pc.FillLocalRect(float64(x)*cw, float64(y)*ch, cw+0.5, ch+0.5, c)
		}
	}
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
