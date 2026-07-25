package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Empty component tokens — genSharedEmptyStyle / mergeToken.
// docs/antd/empty.md §6.2 · components/empty/style/index.ts
//
// emptyImgHeight    = controlHeightLG * 2.5  (40*2.5=100)
// emptyImgHeightMD  = controlHeightLG         (40, simple/normal)
// emptyImgHeightSM  = controlHeightLG * 0.875 (35)
// marginInline / image marginBottom = antd marginXS=8
// footer marginTop = antd margin=16
// simple root marginBlock = antd marginXL=32
const (
	// DefaultEmptyDescription is locale Empty.description (en_US).
	DefaultEmptyDescription = "No data"
	// DefaultEmptyFontSize is fontSize (14).
	DefaultEmptyFontSize = 14.0
	// DefaultEmptyImgHeight is emptyImgHeight (controlHeightLG×2.5).
	DefaultEmptyImgHeight = 100.0
	// DefaultEmptyImgHeightMD is emptyImgHeightMD for simple/normal.
	DefaultEmptyImgHeightMD = 40.0
	// DefaultEmptyImgHeightSM is emptyImgHeightSM (small context).
	DefaultEmptyImgHeightSM = 35.0
	// DefaultEmptyImageMarginBottom is image margin-bottom (antd marginXS).
	DefaultEmptyImageMarginBottom = 8.0
	// DefaultEmptyMarginInline is root margin-inline (antd marginXS).
	DefaultEmptyMarginInline = 8.0
	// DefaultEmptyFooterMarginTop is footer margin-top (antd margin).
	DefaultEmptyFooterMarginTop = 16.0
	// DefaultEmptyNormalMarginBlock is empty-normal margin-block (antd marginXL).
	DefaultEmptyNormalMarginBlock = 32.0
	// DefaultEmptyOpacityImage is opacityImage (default 1).
	DefaultEmptyOpacityImage = 1.0
	// DefaultEmptyFocusRingOutset approximates Ant focus-visible outset (footer kids).
	DefaultEmptyFocusRingOutset = 1.5
	// DefaultEmptyImgAspect is default illustration width/height ratio (~184/152).
	DefaultEmptyImgAspect = 184.0 / 152.0
	// DefaultEmptySimpleImgAspect is simple illustration ratio (~64/41).
	DefaultEmptySimpleImgAspect = 64.0 / 41.0
)

// EmptyImageKind is the built-in illustration preset (antd PRESENTED_IMAGE_*).
type EmptyImageKind int

const (
	// EmptyImageDefault is Empty.PRESENTED_IMAGE_DEFAULT.
	EmptyImageDefault EmptyImageKind = iota
	// EmptyImageSimple is Empty.PRESENTED_IMAGE_SIMPLE (empty-normal metrics).
	EmptyImageSimple
)

// EmptyClassNames holds shallow semantic class tags (antd classNames).
type EmptyClassNames struct {
	Root        string
	Image       string
	Description string
	Footer      string
}

// Empty is Ant Design Empty (empty-state placeholder).
//
//	Decorated root
//	  └─ Column (CrossCenter)
//	       image · description? · footer?
//
// Product contract: docs/antd/empty.md §6 (P0 DoD).
// Root pointer stays stable across rebuild when possible (ClearChildren).
type Empty struct {
	Root *primitive.Decorated
	col  *primitive.Flex

	imageHost *primitive.Decorated
	descLab   *primitive.Text
	footer    *primitive.Decorated

	// Description is the string description when DescriptionNode is nil.
	// Empty string after SetDescription hides the description (antd falsy).
	Description string
	// DescriptionNode overrides Description when non-nil.
	DescriptionNode core.Node
	descriptionSet  bool
	descriptionNone bool // HideDescription / description=false

	// Image is the built-in kind when no custom ImageNode / ImageSrc.
	Image EmptyImageKind
	// ImageNode is a custom illustration node (overrides Image / ImageSrc).
	ImageNode core.Node
	// ImageSrc is a string image source label (antd image=string).
	ImageSrc string

	// imageHeight overrides resolved image height when > 0 (styles.image.height).
	imageHeight    float64
	imageHeightSet bool

	// Children are footer action nodes (antd children).
	Children []core.Node

	// Semantic shallow styles (style-class P0).
	Style            Style // root
	ImageStyle       Style
	DescriptionStyle Style
	FooterStyle      Style
	ClassNames       EmptyClassNames

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme

	// cached resolved colors for L2 tests
	descColor render.RGBA
}

// NewEmpty creates an Empty with antd defaults
// (image=DEFAULT, description=locale "No data", no footer).
func NewEmpty() *Empty {
	e := &Empty{Image: EmptyImageDefault}
	e.rebuild()
	return e
}

// Node returns the mount root (stable Decorated).
func (e *Empty) Node() core.Node {
	if e == nil {
		return nil
	}
	if e.Root == nil {
		e.rebuild()
	}
	return e.Root
}

// ChromeNode returns the visual shell (Decorated).
func (e *Empty) ChromeNode() core.Node { return e.Node() }

// ImageNodeHost returns the image container (tests / semantic).
func (e *Empty) ImageNodeHost() core.Node {
	if e == nil {
		return nil
	}
	return e.imageHost
}

// DescriptionNodeHost returns the description text node when string-backed.
func (e *Empty) DescriptionNodeHost() core.Node {
	if e == nil {
		return nil
	}
	if e.descLab != nil {
		return e.descLab
	}
	return nil
}

// FooterNode returns the footer container (nil when no children).
func (e *Empty) FooterNode() core.Node {
	if e == nil {
		return nil
	}
	return e.footer
}

// --- L2 metrics ---

// FontSize returns body description font size.
func (e *Empty) FontSize() float64 {
	if e != nil && e.DescriptionStyle.FontSize > 0 {
		return e.DescriptionStyle.FontSize
	}
	if e != nil && e.Style.FontSize > 0 {
		return e.Style.FontSize
	}
	th := e.theme()
	if fs := th.SizeOr(core.TokenFontSize, 0); fs > 0 {
		return fs
	}
	return DefaultEmptyFontSize
}

// ImageHeight returns the resolved image box height.
func (e *Empty) ImageHeight() float64 {
	if e != nil && e.imageHeightSet && e.imageHeight > 0 {
		return e.imageHeight
	}
	if e != nil && e.ImageStyle.Height > 0 {
		return e.ImageStyle.Height
	}
	return e.defaultImgHeight()
}

func (e *Empty) defaultImgHeight() float64 {
	if e != nil && e.Image == EmptyImageSimple {
		return DefaultEmptyImgHeightMD
	}
	th := e.theme()
	chLG := th.SizeOr(core.TokenControlHeightLG, 40)
	if e != nil && e.Image == EmptyImageSimple {
		return chLG
	}
	if chLG > 0 {
		return chLG * 2.5
	}
	return DefaultEmptyImgHeight
}

// ImageMarginBottom returns image margin-bottom.
func (e *Empty) ImageMarginBottom() float64 {
	return DefaultEmptyImageMarginBottom
}

// FooterMarginTop returns footer margin-top.
func (e *Empty) FooterMarginTop() float64 {
	return DefaultEmptyFooterMarginTop
}

// MarginInline returns root horizontal margin.
func (e *Empty) MarginInline() float64 {
	return DefaultEmptyMarginInline
}

// NormalMarginBlock returns empty-normal vertical margin (simple image).
func (e *Empty) NormalMarginBlock() float64 {
	if e != nil && e.Image == EmptyImageSimple && e.ImageNode == nil && e.ImageSrc == "" {
		return DefaultEmptyNormalMarginBlock
	}
	return 0
}

// DescriptionColor returns the resolved description text color.
func (e *Empty) DescriptionColor() render.RGBA {
	if e == nil {
		return render.RGBA{}
	}
	if e.DescriptionStyle.hasText() {
		return e.DescriptionStyle.Text
	}
	if e.descColor.A > 0.01 {
		return e.descColor
	}
	return e.theme().Color(core.TokenColorTextSecondary)
}

// ImageKind returns the active built-in kind (custom Node/src still reports last kind).
func (e *Empty) ImageKind() EmptyImageKind {
	if e == nil {
		return EmptyImageDefault
	}
	return e.Image
}

// IsSimple reports PRESENTED_IMAGE_SIMPLE active (no custom override).
func (e *Empty) IsSimple() bool {
	return e != nil && e.Image == EmptyImageSimple && e.ImageNode == nil && e.ImageSrc == ""
}

// HasDescription reports whether description is shown.
func (e *Empty) HasDescription() bool {
	if e == nil || e.descriptionNone {
		return false
	}
	if e.DescriptionNode != nil {
		return true
	}
	if e.descriptionSet {
		return e.Description != ""
	}
	return true // default locale
}

// ResolvedDescription returns the effective string description ("" when hidden/node).
func (e *Empty) ResolvedDescription() string {
	if e == nil || e.descriptionNone {
		return ""
	}
	if e.DescriptionNode != nil {
		return ""
	}
	if e.descriptionSet {
		return e.Description
	}
	return DefaultEmptyDescription
}

// HasFooter reports whether footer children are present.
func (e *Empty) HasFooter() bool {
	return e != nil && len(e.Children) > 0
}

// --- setters ---

// SetDescription sets the string description. Empty string hides (antd falsy).
func (e *Empty) SetDescription(s string) {
	if e == nil {
		return
	}
	e.descriptionSet = true
	e.descriptionNone = s == ""
	e.Description = s
	e.DescriptionNode = nil
	e.rebuild()
}

// SetDescriptionNode sets a custom description node (overrides string).
func (e *Empty) SetDescriptionNode(n core.Node) {
	if e == nil {
		return
	}
	e.descriptionSet = true
	e.descriptionNone = n == nil
	e.DescriptionNode = n
	if n != nil {
		e.Description = ""
	}
	e.rebuild()
}

// HideDescription hides description (antd description={false}).
func (e *Empty) HideDescription() {
	if e == nil {
		return
	}
	e.descriptionSet = true
	e.descriptionNone = true
	e.Description = ""
	e.DescriptionNode = nil
	e.rebuild()
}

// SetImage selects a built-in illustration (clears custom Node/src).
func (e *Empty) SetImage(kind EmptyImageKind) {
	if e == nil {
		return
	}
	e.Image = kind
	e.ImageNode = nil
	e.ImageSrc = ""
	e.rebuild()
}

// SetImageNode sets a custom illustration node.
func (e *Empty) SetImageNode(n core.Node) {
	if e == nil {
		return
	}
	e.ImageNode = n
	if n != nil {
		e.ImageSrc = ""
	}
	e.rebuild()
}

// SetImageSrc sets a string image source (antd image=string).
func (e *Empty) SetImageSrc(src string) {
	if e == nil {
		return
	}
	e.ImageSrc = src
	if src != "" {
		e.ImageNode = nil
	}
	e.rebuild()
}

// SetImageHeight sets styles.image.height (0 clears override → kind default).
func (e *Empty) SetImageHeight(h float64) {
	if e == nil {
		return
	}
	e.imageHeight = h
	e.imageHeightSet = true
	e.rebuild()
}

// SetChildren sets footer action nodes.
func (e *Empty) SetChildren(kids ...core.Node) {
	if e == nil {
		return
	}
	e.Children = append([]core.Node(nil), kids...)
	e.rebuild()
}

// SetStyle sets root shallow style overrides.
func (e *Empty) SetStyle(st Style) {
	if e == nil {
		return
	}
	e.Style = st
	e.rebuild()
}

// SetImageStyle sets image semantic style overrides.
func (e *Empty) SetImageStyle(st Style) {
	if e == nil {
		return
	}
	e.ImageStyle = st
	e.rebuild()
}

// SetDescriptionStyle sets description semantic style overrides.
func (e *Empty) SetDescriptionStyle(st Style) {
	if e == nil {
		return
	}
	e.DescriptionStyle = st
	e.rebuild()
}

// SetFooterStyle sets footer semantic style overrides.
func (e *Empty) SetFooterStyle(st Style) {
	if e == nil {
		return
	}
	e.FooterStyle = st
	e.rebuild()
}

// SetClassNames sets shallow semantic class tags.
func (e *Empty) SetClassNames(cn EmptyClassNames) {
	if e == nil {
		return
	}
	e.ClassNames = cn
	e.rebuild()
}

// SetFace sets the font face.
func (e *Empty) SetFace(face text.Face) {
	if e == nil {
		return
	}
	e.Face = face
	e.rebuild()
}

// SetTheme sets the theme and rebuilds.
func (e *Empty) SetTheme(th *core.Theme) {
	if e == nil {
		return
	}
	e.Theme = th
	e.rebuild()
}

// SetAriaLabel sets an accessible name on the root (optional).
func (e *Empty) SetAriaLabel(s string) {
	if e == nil {
		return
	}
	e.AriaLabel = s
	if e.Root != nil {
		e.Root.Base().Label = s
	}
}

func (e *Empty) theme() *core.Theme {
	if e != nil && e.Theme != nil {
		return e.Theme
	}
	return DefaultTheme()
}

func (e *Empty) rebuild() {
	if e == nil {
		return
	}
	th := e.theme()
	e.descColor = th.Color(core.TokenColorTextSecondary)
	if e.descColor.A < 0.05 {
		e.descColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}

	// Root (stable)
	if e.Root == nil {
		e.Root = primitive.NewDecorated()
		e.Root.Hit = core.HitDefer
	} else {
		e.Root.ClearChildren()
	}
	e.Root.Base().Role = "group"
	if e.AriaLabel != "" {
		e.Root.Base().Label = e.AriaLabel
	} else {
		e.Root.Base().Label = ""
	}
	if e.ClassNames.Root != "" {
		e.Root.Base().Key = e.ClassNames.Root
	}

	// apply root style
	e.Root.BorderWidth = 0
	e.Root.Background = render.RGBA{}
	e.Root.BorderColor = render.RGBA{}
	e.Root.Radius = 0
	if e.Style.hasBG() {
		e.Root.Background = e.Style.Background
	}
	if e.Style.hasBorder() {
		e.Root.BorderColor = e.Style.Border
		e.Root.BorderWidth = 1
	}
	if e.Style.hasRadius() {
		e.Root.Radius = e.Style.Radius
	}

	// outer margins
	mi := DefaultEmptyMarginInline
	if g := th.SizeOr(core.TokenMarginSM, 0); g >= 8 {
		mi = g
	}
	mb := 0.0
	mt := 0.0
	if e.IsSimple() {
		mb = DefaultEmptyNormalMarginBlock
		mt = DefaultEmptyNormalMarginBlock
	}
	e.Root.Padding = primitive.EdgeInsets{Left: mi, Right: mi, Top: mt, Bottom: mb}

	// column
	if e.col == nil {
		e.col = primitive.Column()
		e.col.CrossAlign = core.CrossCenter
		e.col.MainAlign = core.MainStart
	} else {
		e.col.ClearChildren()
	}
	e.col.Gap = 0
	e.col.CrossAlign = core.CrossCenter

	// --- image ---
	imgH := e.ImageHeight()
	imgW := imgH * DefaultEmptyImgAspect
	if e.IsSimple() {
		imgW = imgH * DefaultEmptySimpleImgAspect
	}
	if e.ImageStyle.Width > 0 {
		imgW = e.ImageStyle.Width
	}

	var imgChild core.Node
	switch {
	case e.ImageNode != nil:
		imgChild = e.ImageNode
	case e.ImageSrc != "":
		// string image: labeled placeholder box (real decode is P1)
		alt := e.ResolvedDescription()
		if alt == "" {
			alt = "empty"
		}
		if e.AriaLabel != "" {
			alt = e.AriaLabel
		}
		im := NewImage(alt, imgW, imgH)
		im.Face = e.Face
		im.Theme = th
		im.SetSrc(e.ImageSrc)
		// img role for string images
		if n := im.Node(); n != nil {
			n.Base().Role = "img"
			n.Base().Label = alt
		}
		imgChild = im.Node()
	default:
		imgChild = e.buildBuiltinImage(th, imgW, imgH)
	}

	if e.imageHost == nil {
		e.imageHost = primitive.NewDecorated()
		e.imageHost.Hit = core.HitDefer
	} else {
		e.imageHost.ClearChildren()
	}
	e.imageHost.Width = imgW
	e.imageHost.Height = imgH
	e.imageHost.StretchChild = true
	e.imageHost.CenterContent = true
	if e.ClassNames.Image != "" {
		e.imageHost.Base().Key = e.ClassNames.Image
	}
	// image styles
	e.imageHost.Background = render.RGBA{}
	e.imageHost.BorderWidth = 0
	e.imageHost.Radius = 0
	if e.ImageStyle.hasBG() {
		e.imageHost.Background = e.ImageStyle.Background
	}
	if e.ImageStyle.hasBorder() {
		e.imageHost.BorderColor = e.ImageStyle.Border
		e.imageHost.BorderWidth = 1
	}
	if e.ImageStyle.hasRadius() {
		e.imageHost.Radius = e.ImageStyle.Radius
	}
	if imgChild != nil {
		e.imageHost.AddChild(imgChild)
	}
	// margin-bottom via wrapper padding
	imgWrap := primitive.NewDecorated(e.imageHost)
	imgWrap.Hit = core.HitDefer
	imgWrap.Padding = primitive.EdgeInsets{Bottom: DefaultEmptyImageMarginBottom}
	e.col.AddChild(imgWrap)

	// --- description ---
	e.descLab = nil
	if e.HasDescription() {
		var descNode core.Node
		if e.DescriptionNode != nil {
			descNode = e.DescriptionNode
		} else {
			txt := e.ResolvedDescription()
			lab := primitive.NewText(txt)
			lab.FontSize = e.FontSize()
			lab.Face = e.Face
			if e.DescriptionStyle.Face != nil {
				lab.Face = e.DescriptionStyle.Face
			} else if e.Style.Face != nil {
				lab.Face = e.Style.Face
			}
			col := e.descColor
			if e.DescriptionStyle.hasText() {
				col = e.DescriptionStyle.Text
			} else if e.Style.hasText() {
				col = e.Style.Text
			}
			lab.Color = col
			e.descLab = lab
			if e.ClassNames.Description != "" {
				lab.Base().Key = e.ClassNames.Description
			}
			descNode = lab
		}
		e.col.AddChild(descNode)
	}

	// --- footer ---
	e.footer = nil
	if len(e.Children) > 0 {
		row := primitive.Row()
		row.Gap = 8
		row.MainAlign = core.MainCenter
		row.CrossAlign = core.CrossCenter
		for _, c := range e.Children {
			if c != nil {
				row.AddChild(c)
			}
		}
		ft := primitive.NewDecorated(row)
		ft.Hit = core.HitDefer
		ft.Padding = primitive.EdgeInsets{Top: DefaultEmptyFooterMarginTop}
		if e.ClassNames.Footer != "" {
			ft.Base().Key = e.ClassNames.Footer
		}
		if e.FooterStyle.hasBG() {
			ft.Background = e.FooterStyle.Background
		}
		if e.FooterStyle.hasBorder() {
			ft.BorderColor = e.FooterStyle.Border
			ft.BorderWidth = 1
		}
		if e.FooterStyle.hasRadius() {
			ft.Radius = e.FooterStyle.Radius
		}
		e.footer = ft
		e.col.AddChild(ft)
	}

	e.Root.AddChild(e.col)

	// theme hook for live theme switch
	e.Root.SetThemeHook(func(th *core.Theme) {
		if th != nil {
			e.Theme = th
		}
		e.rebuild()
	})
}

func (e *Empty) buildBuiltinImage(th *core.Theme, w, h float64) core.Node {
	// Approximate antd empty SVG with token-colored geometry (P0 近似).
	bg := th.Color(core.TokenColorBgContainer)
	if bg.A < 0.05 {
		bg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	fillSec := th.Color(core.TokenColorFillSecondary)
	if fillSec.A < 0.02 {
		fillSec = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	fillQuat := th.Color(core.TokenColorTextQuaternary)
	if fillQuat.A < 0.02 {
		fillQuat = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	// panel-ish fill (antd colorFillTertiary blend)
	panel := render.RGBA{R: 0.96, G: 0.96, B: 0.96, A: 1}
	if c := th.Color(core.TokenColorBgLayout); c.A > 0.05 {
		panel = c
	}
	detail := fillSec
	border := fillQuat
	shadow := fillSec

	simple := e != nil && e.Image == EmptyImageSimple
	paint := func(pc *core.PaintContext, sz core.Size) {
		if pc == nil || sz.Width <= 0 || sz.Height <= 0 {
			return
		}
		// opacityImage
		_ = DefaultEmptyOpacityImage
		if simple {
			// simple: ellipse shadow + open box
			cx, cy := sz.Width*0.5, sz.Height*0.78
			rx, ry := sz.Width*0.48, sz.Height*0.14
			// approximate ellipse with round rect
			pc.FillLocalRoundRect(cx-rx, cy-ry, rx*2, ry*2, ry, shadow)
			// box body
			bx, by := sz.Width*0.14, sz.Height*0.18
			bw, bh := sz.Width*0.72, sz.Height*0.55
			pc.FillLocalRoundRect(bx, by+bh*0.25, bw, bh*0.75, 2, detail)
			pc.StrokeLocalRoundRect(bx, by+bh*0.25, bw, bh*0.75, 2, 1, border)
			// lid
			pc.StrokeLocalLine(bx, by+bh*0.25, bx+bw*0.18, by, 1, border)
			pc.StrokeLocalLine(bx+bw, by+bh*0.25, bx+bw*0.82, by, 1, border)
			pc.StrokeLocalLine(bx+bw*0.18, by, bx+bw*0.82, by, 1, border)
			return
		}
		// default: document panel + soft shadow
		cx, cy := sz.Width*0.48, sz.Height*0.88
		rx, ry := sz.Width*0.42, sz.Height*0.08
		pc.FillLocalRoundRect(cx-rx, cy-ry, rx*2, ry*2, ry, shadow)
		// panel
		pw, ph := sz.Width*0.48, sz.Height*0.62
		px := (sz.Width - pw) * 0.38
		py := sz.Height * 0.12
		pc.FillLocalRoundRect(px, py, pw, ph, 4, panel)
		pc.StrokeLocalRoundRect(px, py, pw, ph, 4, 1, border)
		// inner lines
		pad := pw * 0.12
		pc.FillLocalRoundRect(px+pad, py+ph*0.12, pw-pad*2, ph*0.22, 2, detail)
		pc.FillLocalRoundRect(px+pad, py+ph*0.42, pw-pad*2, ph*0.06, 1, detail)
		pc.FillLocalRoundRect(px+pad, py+ph*0.54, pw-pad*2, ph*0.06, 1, detail)
		// tray under panel
		tx := px - pw*0.18
		tw := pw * 1.36
		ty := py + ph*0.72
		thh := ph * 0.28
		pc.FillLocalRoundRect(tx, ty, tw, thh, 3, detail)
		_ = bg
	}
	c := primitive.NewCanvas(w, h, paint)
	// decorative: no role/label
	c.Base().Role = ""
	c.Base().Label = ""
	c.Hit = core.HitDefer
	return c
}
