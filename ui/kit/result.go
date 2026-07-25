package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// ResultStatus is the Ant Design Result status set.
type ResultStatus string

const (
	ResultInfo    ResultStatus = "info"
	ResultSuccess ResultStatus = "success"
	ResultWarning ResultStatus = "warning"
	ResultError   ResultStatus = "error"
	Result403     ResultStatus = "403"
	Result404     ResultStatus = "404"
	Result500     ResultStatus = "500"
)

// Result is an Ant Design Result status page.
//
// Product contract: docs/antd/result.md §6. P0 covers status, title, subTitle,
// default/custom icon, exception statuses, extra actions, body content, token
// metrics, and static a11y semantics.
type Result struct {
	Root *primitive.Flex

	Status   ResultStatus
	Title    string
	SubTitle string

	Extra []core.Node
	Body  []core.Node

	customIcon core.Node
	iconName   string
	iconNone   bool
	iconNode   core.Node

	titleNode    *primitive.Text
	subTitleNode *primitive.Text
	extraNode    *primitive.Flex
	bodyNode     *primitive.Decorated

	Loading   bool
	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	spinPhase float64
	boundTree *core.Tree
}

// NewResult creates a Result with Ant defaults: status=info and empty content.
func NewResult() *Result {
	r := &Result{Status: ResultInfo}
	r.rebuild()
	return r
}

// Node returns the root core.Node for tree attachment.
func (r *Result) Node() core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	return r.Root
}

// IconNode returns the current icon/image node.
func (r *Result) IconNode() core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	return r.iconNode
}

// TitleNode returns the title text node when present.
func (r *Result) TitleNode() *primitive.Text {
	if r.Root == nil {
		r.rebuild()
	}
	return r.titleNode
}

// SubTitleNode returns the subtitle text node when present.
func (r *Result) SubTitleNode() *primitive.Text {
	if r.Root == nil {
		r.rebuild()
	}
	return r.subTitleNode
}

// ExtraNode returns the extra action row when present.
func (r *Result) ExtraNode() *primitive.Flex {
	if r.Root == nil {
		r.rebuild()
	}
	return r.extraNode
}

// BodyNode returns the body container when present.
func (r *Result) BodyNode() *primitive.Decorated {
	if r.Root == nil {
		r.rebuild()
	}
	return r.bodyNode
}

// SetStatus updates the Result status. Empty or unknown values fall back to info.
func (r *Result) SetStatus(s ResultStatus) {
	r.Status = normalizeResultStatus(s)
	r.rebuild()
}

// SetTitle updates title text.
func (r *Result) SetTitle(s string) {
	r.Title = s
	r.rebuild()
}

// SetSubTitle updates subtitle text.
func (r *Result) SetSubTitle(s string) {
	r.SubTitle = s
	r.rebuild()
}

// SetExtra replaces the action area.
func (r *Result) SetExtra(nodes ...core.Node) {
	r.Extra = append(r.Extra[:0], nodes...)
	r.rebuild()
}

// SetBody replaces the body content area.
func (r *Result) SetBody(nodes ...core.Node) {
	r.Body = append(r.Body[:0], nodes...)
	r.rebuild()
}

// SetIcon replaces the status icon with a custom node. Nil restores default.
func (r *Result) SetIcon(node core.Node) {
	r.customIcon = node
	r.iconName = ""
	r.iconNone = false
	r.rebuild()
}

// SetIconName replaces the status icon with a named primitive icon. Empty restores default.
func (r *Result) SetIconName(name string) {
	r.iconName = name
	r.customIcon = nil
	r.iconNone = false
	r.rebuild()
}

// SetIconNone hides the icon, matching antd icon={null}.
func (r *Result) SetIconNone(v bool) {
	r.iconNone = v
	if v {
		r.customIcon = nil
		r.iconName = ""
	}
	r.rebuild()
}

// SetLoading toggles the optional loading ticker for spinning icon nodes.
func (r *Result) SetLoading(v bool) {
	if r.Loading == v {
		return
	}
	r.Loading = v
	r.rebuild()
	if r.boundTree != nil {
		if v {
			r.boundTree.AddTicker(r)
		} else {
			r.boundTree.RemoveTicker(r)
		}
	}
}

// AttachTicker registers the Result loading spinner for demand-frame ANIMATING.
func (r *Result) AttachTicker(t *core.Tree) {
	if r == nil || t == nil {
		return
	}
	r.boundTree = t
	t.BindTicker(r, r.Loading)
}

// Tick advances the loading spinner. Implements core.Ticker when Loading.
func (r *Result) Tick(dt float64) bool {
	if r == nil || !r.Loading {
		return false
	}
	r.spinPhase += dt * 1.4
	for r.spinPhase >= 1 {
		r.spinPhase -= 1
	}
	if ic, ok := r.iconNode.(*primitive.Icon); ok {
		ic.SpinPhase = r.spinPhase
		ic.MarkNeedsPaint()
	} else if r.Root != nil {
		r.Root.MarkNeedsPaint()
	}
	return r.Loading
}

// SetAriaLabel sets the result region accessible label.
func (r *Result) SetAriaLabel(name string) {
	r.AriaLabel = name
	if r.Root != nil {
		r.Root.Base().Label = name
	}
}

// SetFace sets the font face used by text nodes.
func (r *Result) SetFace(face text.Face) {
	r.Face = face
	r.Style.Face = face
	r.rebuild()
}

// SetTheme applies a theme and rebuilds token-derived chrome.
func (r *Result) SetTheme(th *core.Theme) {
	r.Theme = th
	r.rebuild()
}

// SetStyle applies shallow root/text overrides.
func (r *Result) SetStyle(st Style) {
	r.Style = st
	if st.Face != nil {
		r.Face = st.Face
	}
	r.rebuild()
}

func (r *Result) rebuild() {
	if r == nil {
		return
	}
	r.Status = normalizeResultStatus(r.Status)
	th := DefaultTheme()
	if r.Theme != nil {
		th = r.Theme
	}
	tok := resultTokens(th)
	if r.Style.FontSize > 0 {
		tok.subTitleFontSize = r.Style.FontSize
	}

	children := make([]core.Node, 0, 5)
	r.titleNode, r.subTitleNode, r.extraNode, r.bodyNode = nil, nil, nil, nil

	r.iconNode = r.buildIcon(th, tok)
	if r.iconNode != nil {
		iconWrap := primitive.Column(r.iconNode)
		iconWrap.CrossAlign = core.CrossCenter
		iconWrap.Padding = primitive.EdgeInsets{Bottom: tok.iconMarginBottom}
		children = append(children, iconWrap)
	}

	if r.Title != "" {
		title := primitive.NewText(r.Title)
		title.FontSize = tok.titleFontSize
		title.Face = r.Face
		title.Color = th.Color(core.TokenColorText)
		if r.Style.hasText() {
			title.Color = r.Style.Text
		}
		title.MaxLines = 0
		title.MaxWidth = 720
		titleWrap := primitive.Column(title)
		titleWrap.CrossAlign = core.CrossCenter
		titleWrap.Padding = primitive.EdgeInsets{Top: tok.titleMarginBlock, Bottom: tok.titleMarginBlock}
		r.titleNode = title
		children = append(children, titleWrap)
	}

	if r.SubTitle != "" {
		sub := primitive.NewText(r.SubTitle)
		sub.FontSize = tok.subTitleFontSize
		sub.Face = r.Face
		sub.Color = th.Color(core.TokenColorTextSecondary)
		sub.MaxLines = 0
		sub.MaxWidth = 720
		r.subTitleNode = sub
		children = append(children, sub)
	}

	if len(r.Extra) > 0 {
		extra := primitive.Row(r.Extra...)
		extra.Gap = tok.extraItemGap
		extra.CrossAlign = core.CrossCenter
		extra.MainAlign = core.MainCenter
		extra.Padding = primitive.EdgeInsets{Top: tok.extraMarginTop}
		r.extraNode = extra
		children = append(children, extra)
	}

	if len(r.Body) > 0 {
		bodyContent := primitive.Column(r.Body...)
		bodyContent.Gap = tok.bodyGap
		body := primitive.NewDecorated(bodyContent)
		body.Padding = primitive.EdgeInsets{
			Left:   tok.bodyPaddingInline,
			Top:    tok.bodyPaddingBlock,
			Right:  tok.bodyPaddingInline,
			Bottom: tok.bodyPaddingBlock,
		}
		body.Background = th.Color(core.TokenColorFillSecondary)
		body.Radius = 0
		body.BorderWidth = 0
		body.Hit = core.HitDefer
		bodyWrap := primitive.Column(body)
		bodyWrap.CrossAlign = core.CrossCenter
		bodyWrap.Padding = primitive.EdgeInsets{Top: tok.bodyMarginTop}
		r.bodyNode = body
		children = append(children, bodyWrap)
	}

	root := primitive.Column(children...)
	root.CrossAlign = core.CrossCenter
	root.Padding = primitive.EdgeInsets{
		Left:   tok.rootPaddingInline,
		Top:    tok.rootPaddingBlock,
		Right:  tok.rootPaddingInline,
		Bottom: tok.rootPaddingBlock,
	}
	root.Base().Role = "status"
	root.Base().Label = r.AriaLabel
	r.Root = root
	if r.boundTree != nil && r.Loading {
		r.boundTree.AddTicker(r)
	}
}

func (r *Result) buildIcon(th *core.Theme, tok resultTokenSet) core.Node {
	if r.iconNone {
		return nil
	}
	if r.customIcon != nil {
		return r.customIcon
	}
	if r.iconName != "" {
		ic := primitive.NewIcon(r.iconName)
		ic.Size = tok.iconFontSize
		ic.Color = resultStatusColor(th, r.Status)
		ic.SpinPhase = r.spinPhase
		return ic
	}
	if isResultException(r.Status) {
		return newResultExceptionImage(r.Status, th, tok)
	}
	ic := primitive.NewIcon(resultIconName(r.Status))
	ic.Size = tok.iconFontSize
	ic.Color = resultStatusColor(th, r.Status)
	ic.SpinPhase = r.spinPhase
	return ic
}

type resultTokenSet struct {
	rootPaddingBlock  float64
	rootPaddingInline float64
	iconFontSize      float64
	iconMarginBottom  float64
	titleFontSize     float64
	titleMarginBlock  float64
	subTitleFontSize  float64
	extraMarginTop    float64
	extraItemGap      float64
	bodyMarginTop     float64
	bodyPaddingBlock  float64
	bodyPaddingInline float64
	bodyGap           float64
	imageWidth        float64
	imageHeight       float64
}

func resultTokens(th *core.Theme) resultTokenSet {
	if th == nil {
		th = DefaultTheme()
	}
	padding := th.SizeOr(core.TokenPadding, 16)
	paddingXS := th.SizeOr(core.TokenPaddingXS, 4)
	paddingLG := th.SizeOr(core.TokenPaddingLG, 24)
	fontSize := th.SizeOr(core.TokenFontSize, 14)
	fontSizeLG := th.SizeOr(core.TokenFontSizeLG, 16)
	marginXS := th.SizeOr(core.TokenMarginXS, 4)
	return resultTokenSet{
		rootPaddingBlock:  paddingLG * 2,
		rootPaddingInline: padding * 2,
		iconFontSize:      fontSizeLG * 4.5,
		iconMarginBottom:  paddingLG,
		titleFontSize:     fontSizeLG * 1.5,
		titleMarginBlock:  marginXS,
		subTitleFontSize:  fontSize,
		extraMarginTop:    paddingLG,
		extraItemGap:      paddingXS * 2,
		bodyMarginTop:     paddingLG,
		bodyPaddingBlock:  paddingLG,
		bodyPaddingInline: padding * 2.5,
		bodyGap:           paddingXS * 2,
		imageWidth:        250,
		imageHeight:       295,
	}
}

func normalizeResultStatus(s ResultStatus) ResultStatus {
	switch s {
	case ResultSuccess, ResultWarning, ResultError, Result403, Result404, Result500:
		return s
	default:
		return ResultInfo
	}
}

func isResultException(s ResultStatus) bool {
	return s == Result403 || s == Result404 || s == Result500
}

func resultIconName(s ResultStatus) string {
	switch s {
	case ResultSuccess:
		return "check"
	case ResultError:
		return "close"
	default:
		return "info"
	}
}

func resultStatusColor(th *core.Theme, s ResultStatus) render.RGBA {
	if th == nil {
		th = DefaultTheme()
	}
	switch s {
	case ResultSuccess:
		return th.Color(core.TokenColorSuccess)
	case ResultWarning:
		return th.Color(core.TokenColorWarning)
	case ResultError:
		return th.Color(core.TokenColorError)
	default:
		return th.Color(core.TokenColorPrimary)
	}
}

func newResultExceptionImage(status ResultStatus, th *core.Theme, tok resultTokenSet) *primitive.Canvas {
	bg := th.Color(core.TokenColorBgLayout)
	fg := resultStatusColor(th, status)
	split := th.Color(core.TokenColorSplit)
	return primitive.NewCanvas(tok.imageWidth, tok.imageHeight, func(pc *core.PaintContext, size core.Size) {
		w, h := size.Width, size.Height
		pc.FillLocalRect(w*0.16, h*0.18, w*0.68, h*0.58, bg)
		pc.FillLocalRect(w*0.20, h*0.22, w*0.60, h*0.06, split)
		pc.FillLocalRect(w*0.24, h*0.34, w*0.28, h*0.05, split)
		pc.FillLocalRect(w*0.24, h*0.45, w*0.44, h*0.05, split)
		pc.FillLocalRect(w*0.24, h*0.56, w*0.35, h*0.05, split)
		pc.FillLocalRect(w*0.62, h*0.34, w*0.12, h*0.27, fg)
		pc.FillLocalRect(w*0.50, h*0.68, w*0.34, h*0.04, fg)
	})
}
