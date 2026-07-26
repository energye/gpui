package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Card component tokens — prepareComponentToken / genCardStyle.
// docs/antd/card.md §6.2 · components/card/style/index.ts
const (
	// DefaultCardBodyPadding is bodyPadding (paddingLG).
	DefaultCardBodyPadding = 24.0
	// DefaultCardBodyPaddingSM is bodyPaddingSM (fixed 12).
	DefaultCardBodyPaddingSM = 12.0
	// DefaultCardHeaderPadding is headerPadding (paddingLG).
	DefaultCardHeaderPadding = 24.0
	// DefaultCardHeaderPaddingSM is headerPaddingSM (fixed 12).
	DefaultCardHeaderPaddingSM = 12.0
	// DefaultCardHeaderFontSize is headerFontSize (fontSizeLG).
	DefaultCardHeaderFontSize = 16.0
	// DefaultCardHeaderFontSizeSM is headerFontSizeSM (fontSize).
	DefaultCardHeaderFontSizeSM = 14.0
	// DefaultCardHeaderHeight ≈ fontSizeLG*lineHeightLG + padding*2.
	DefaultCardHeaderHeight = 56.0
	// DefaultCardHeaderHeightSM ≈ fontSize*lineHeight + paddingXS*2.
	DefaultCardHeaderHeightSM = 30.0
	// DefaultCardRadius is borderRadiusLG.
	DefaultCardRadius = 8.0
	// DefaultCardLineWidth is lineWidth.
	DefaultCardLineWidth = 1.0
	// DefaultCardFontSize is fontSize.
	DefaultCardFontSize = 14.0
	// DefaultCardActionsPadV is actionsLiMargin vertical (paddingSM).
	DefaultCardActionsPadV = 12.0
	// DefaultCardActionsIconSize is cardActionsIconSize (fontSize).
	DefaultCardActionsIconSize = 14.0
	// DefaultCardMetaTitleFont is Meta title (fontSizeLG).
	DefaultCardMetaTitleFont = 16.0
	// DefaultCardMetaAvatarPad is Meta avatar paddingInlineEnd (padding).
	DefaultCardMetaAvatarPad = 16.0
	// DefaultCardMetaSectionGap is Meta section item gap (marginXS).
	DefaultCardMetaSectionGap = 8.0
	// DefaultCardGridPadding is cardPaddingBase (paddingLG).
	DefaultCardGridPadding = 24.0
	// DefaultCardGridWidthFrac is genCardGridStyle width 33.33%.
	DefaultCardGridWidthFrac = 1.0 / 3.0
	// DefaultCardLoadingRows is antd Skeleton paragraph rows under loading.
	DefaultCardLoadingRows = 4
	// DefaultCardFocusRingOutset approximates Ant focus-visible outset.
	DefaultCardFocusRingOutset = 1.5
)

// CardSize is antd Card size (medium|small). medium is kit middle.
type CardSize int

const (
	// CardMiddle is antd medium/default size.
	CardMiddle CardSize = iota
	// CardSmall is compact size.
	CardSmall
)

// CardVariant is antd variant (outlined|borderless).
type CardVariant int

const (
	// CardOutlined draws a 1px border (default).
	CardOutlined CardVariant = iota
	// CardBorderless has no border.
	CardBorderless
)

// CardType is antd type (default|inner).
type CardType int

const (
	// CardTypeDefault is the normal outer card.
	CardTypeDefault CardType = iota
	// CardTypeInner is nested inner chrome (fillAlter head).
	CardTypeInner
)

// Card is Ant Design Card (data display container).
//
//	Pressable? (hoverable / OnClick)
//	  └─ Decorated (root chrome)
//	       └─ Column
//	            header? · cover? · body · actions?
//
// Product contract: docs/antd/card.md §6 (P0 DoD).
type Card struct {
	press *primitive.Pressable
	Root  *primitive.Decorated

	headerDec  *primitive.Decorated
	titleLab   *primitive.Text
	extraSlot  *primitive.Slot
	coverSlot  *primitive.Slot
	bodyDec    *primitive.Decorated
	bodySlot   *primitive.Slot
	actionsRow *primitive.Flex
	skeleton   *Skeleton

	// Title is the header title string (used when TitleNode is nil).
	Title string
	// TitleNode overrides Title when non-nil.
	TitleNode core.Node
	Extra     core.Node
	Cover     core.Node
	content   core.Node
	Actions   []core.Node
	// Grids when non-empty place Card.Grid children in a wrapping body (contain-grid).
	Grids []*CardGrid

	Size      CardSize
	Variant   CardVariant
	Type      CardType
	Loading   bool
	Hoverable bool
	Disabled  bool
	// Width forces preferred outer width when > 0.
	Width float64

	OnClick   func()
	AriaLabel string

	Face  text.Face
	Theme *core.Theme
	Style Style

	// lastHovered mirrors press hover for tests / chrome.
	lastHovered bool
	boundTree   *core.Tree
}

// NewCard creates a Card with antd defaults (outlined, middle). Title may be empty.
func NewCard(title string) *Card {
	c := &Card{
		Title:   title,
		Size:    CardMiddle,
		Variant: CardOutlined,
		Type:    CardTypeDefault,
	}
	c.rebuild()
	return c
}

// Node returns the mount root (Pressable when hoverable/clickable, else Decorated).

// ensureBuilt materializes the control tree if missing (#9).
func (c *Card) ensureBuilt() {
	if c == nil {
		return
	}
	if c.Root == nil {
		c.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (c *Card) structureChange() {
	if c == nil {
		return
	}
	c.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (c *Card) chromeChange() {
	if c == nil {
		return
	}
	c.ensureBuilt()
	c.rebuild()
}

func (c *Card) Node() core.Node {
	if c == nil {
		return nil
	}
	c.ensureBuilt()
	if c.press != nil {
		return c.press
	}
	return c.Root
}

// ChromeNode returns the Decorated chrome (always the visual shell).
func (c *Card) ChromeNode() core.Node {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// BodyPadding returns resolved body inset (uniform) for the current size/type.
func (c *Card) BodyPadding() float64 {
	if c == nil {
		return DefaultCardBodyPadding
	}
	if c.Type == CardTypeInner {
		// inner body: padding (16) vertical uses TokenPadding; horizontal bodyPadding.
		// Tests assert horizontal body pad; return size-based body pad.
	}
	if c.Size == CardSmall {
		return DefaultCardBodyPaddingSM
	}
	return c.bodyPadResolved()
}

// HeaderFontSize returns resolved header title size.
func (c *Card) HeaderFontSize() float64 {
	if c == nil {
		return DefaultCardHeaderFontSize
	}
	if c.Type == CardTypeInner || c.Size == CardSmall {
		return c.headerFontResolved()
	}
	return c.headerFontResolved()
}

// HeaderMinHeight returns resolved header min height.
func (c *Card) HeaderMinHeight() float64 {
	if c == nil {
		return DefaultCardHeaderHeight
	}
	return c.headerMinHResolved()
}

// Radius returns resolved corner radius.
func (c *Card) Radius() float64 {
	if c == nil {
		return DefaultCardRadius
	}
	return c.radiusResolved()
}

// LineWidth returns resolved border width (0 when borderless).
func (c *Card) LineWidth() float64 {
	if c == nil {
		return DefaultCardLineWidth
	}
	if c.Variant == CardBorderless {
		return 0
	}
	return c.lineWResolved()
}

// IsOutlined reports variant=outlined.
func (c *Card) IsOutlined() bool {
	return c != nil && c.Variant == CardOutlined
}

// IsHovered reports pointer hover (hoverable cards).
func (c *Card) IsHovered() bool {
	return c != nil && c.lastHovered
}

// SetTitle sets the string title and rebuilds.
func (c *Card) SetTitle(s string) {
	if c == nil {
		return
	}
	c.Title = s
	c.TitleNode = nil
	c.rebuild()
}

// SetTitleNode sets a custom title node (overrides Title string).
func (c *Card) SetTitleNode(n core.Node) {
	if c == nil {
		return
	}
	c.TitleNode = n
	c.rebuild()
}

// SetExtra sets the header extra area.
func (c *Card) SetExtra(n core.Node) {
	if c == nil {
		return
	}
	c.Extra = n
	if c.extraSlot != nil {
		c.extraSlot.SetChild(n)
		return
	}
	c.rebuild()
}

// SetCover sets the cover node.
func (c *Card) SetCover(n core.Node) {
	if c == nil {
		return
	}
	c.Cover = n
	c.rebuild()
}

// SetContent sets body children (ignored visually while Loading).
func (c *Card) SetContent(n core.Node) {
	if c == nil {
		return
	}
	c.content = n
	if c.Loading {
		c.rebuild()
		return
	}
	if c.bodySlot != nil && len(c.Grids) == 0 {
		c.bodySlot.SetChild(n)
		return
	}
	c.rebuild()
}

// SetActions sets bottom action nodes (equal flex share).
func (c *Card) SetActions(actions ...core.Node) {
	if c == nil {
		return
	}
	c.Actions = append([]core.Node(nil), actions...)
	c.rebuild()
}

// SetGrids installs Card.Grid children (contain-grid body).
func (c *Card) SetGrids(grids ...*CardGrid) {
	if c == nil {
		return
	}
	c.Grids = append([]*CardGrid(nil), grids...)
	c.rebuild()
}

// SetSize sets middle|small.
func (c *Card) SetSize(sz CardSize) {
	if c == nil {
		return
	}
	c.Size = sz
	c.rebuild()
}

// SetVariant sets outlined|borderless.
func (c *Card) SetVariant(v CardVariant) {
	if c == nil {
		return
	}
	c.Variant = v
	c.applyChrome()
}

// SetType sets default|inner.
func (c *Card) SetType(t CardType) {
	if c == nil {
		return
	}
	c.Type = t
	c.rebuild()
}

// SetLoading toggles body skeleton (Ticker via Skeleton mount).
func (c *Card) SetLoading(v bool) {
	if c == nil {
		return
	}
	c.Loading = v
	c.rebuild()
}

// SetHoverable enables hover elevation feedback.
func (c *Card) SetHoverable(v bool) {
	if c == nil {
		return
	}
	c.Hoverable = v
	c.rebuild()
}

// SetDisabled toggles disabled chrome (kit extension).
func (c *Card) SetDisabled(v bool) {
	if c == nil {
		return
	}
	c.Disabled = v
	if c.press != nil {
		c.press.SetDisabled(v)
	}
	c.applyChrome()
}

// SetWidth forces outer width (0 = content).
func (c *Card) SetWidth(w float64) {
	if c == nil {
		return
	}
	c.Width = w
	if c.Root != nil {
		c.Root.Width = w
		c.Root.MarkNeedsLayout()
		return
	}
	c.rebuild()
}

// SetOnClick sets click handler (enables Pressable even without Hoverable).
func (c *Card) SetOnClick(fn func()) {
	if c == nil {
		return
	}
	c.OnClick = fn
	c.rebuild()
}

// SetTheme sets theme override.
func (c *Card) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.rebuild()
}

// SetFace sets font face for text chrome.
func (c *Card) SetFace(face text.Face) {
	if c == nil {
		return
	}
	c.Face = face
	if c.titleLab != nil {
		c.titleLab.Face = face
		c.titleLab.MarkNeedsPaint()
	}
}

// SetStyle sets optional visual overrides.
func (c *Card) SetStyle(st Style) {
	if c == nil {
		return
	}
	c.Style = st
	c.applyChrome()
}

// SetAriaLabel sets accessible name.
func (c *Card) SetAriaLabel(s string) {
	if c == nil {
		return
	}
	c.AriaLabel = s
	c.applyA11y()
}

// AttachTicker registers loading skeleton animation (also auto OnMount of Skeleton).
func (c *Card) AttachTicker(t *core.Tree) {
	if c == nil || t == nil {
		return
	}
	c.boundTree = t
	if c.skeleton != nil {
		c.skeleton.AttachTicker(t)
	}
}

// Tick forwards to skeleton when loading (implements optional host loops).
func (c *Card) Tick(dt float64) bool {
	if c == nil || !c.Loading || c.skeleton == nil {
		return false
	}
	return c.skeleton.Tick(dt)
}

func (c *Card) theme() *core.Theme {
	var n core.Node
	if c.Root != nil {
		n = c.Root
	}
	return themeOf(c.Theme, n)
}

func (c *Card) bodyPadResolved() float64 {
	th := c.theme()
	if c.Size == CardSmall {
		return DefaultCardBodyPaddingSM
	}
	return th.SizeOr(core.TokenPaddingLG, DefaultCardBodyPadding)
}

func (c *Card) headerPadResolved() float64 {
	th := c.theme()
	if c.Size == CardSmall {
		return DefaultCardHeaderPaddingSM
	}
	return th.SizeOr(core.TokenPaddingLG, DefaultCardHeaderPadding)
}

func (c *Card) headerFontResolved() float64 {
	th := c.theme()
	if c.Type == CardTypeInner {
		return th.SizeOr(core.TokenFontSize, DefaultCardFontSize)
	}
	if c.Size == CardSmall {
		return th.SizeOr(core.TokenFontSize, DefaultCardHeaderFontSizeSM)
	}
	return th.SizeOr(core.TokenFontSizeLG, DefaultCardHeaderFontSize)
}

func (c *Card) headerMinHResolved() float64 {
	if c.Size == CardSmall {
		return DefaultCardHeaderHeightSM
	}
	return DefaultCardHeaderHeight
}

func (c *Card) radiusResolved() float64 {
	if c.Style.hasRadius() {
		return c.Style.Radius
	}
	return c.theme().SizeOr(core.TokenBorderRadiusLG, DefaultCardRadius)
}

func (c *Card) lineWResolved() float64 {
	return c.theme().SizeOr(core.TokenLineWidth, DefaultCardLineWidth)
}

func (c *Card) fontResolved() float64 {
	if c.Style.FontSize > 0 {
		return c.Style.FontSize
	}
	return c.theme().SizeOr(core.TokenFontSize, DefaultCardFontSize)
}

func (c *Card) hasHeader() bool {
	if c.TitleNode != nil {
		return true
	}
	if c.Title != "" {
		return true
	}
	return c.Extra != nil
}

func (c *Card) needsPressable() bool {
	return c.Hoverable || c.OnClick != nil
}

func (c *Card) rebuild() {
	th := c.theme()
	radius := c.radiusResolved()
	lineW := c.lineWResolved()
	bodyPad := c.bodyPadResolved()
	headPad := c.headerPadResolved()
	headFont := c.headerFontResolved()
	headMinH := c.headerMinHResolved()

	col := primitive.Column()
	col.Gap = 0
	col.CrossAlign = core.CrossStretch
	col.ExpandMax = true

	// --- header ---
	c.headerDec = nil
	c.titleLab = nil
	c.extraSlot = nil
	if c.hasHeader() {
		var titleNode core.Node
		if c.TitleNode != nil {
			titleNode = c.TitleNode
		} else {
			c.titleLab = primitive.NewText(c.Title)
			c.titleLab.FontSize = headFont
			c.titleLab.Face = c.Face
			if c.Style.hasText() {
				c.titleLab.Color = c.Style.Text
			} else {
				c.titleLab.Color = th.Color(core.TokenColorText)
			}
			// antd fontWeightStrong on head title
			titleNode = c.titleLab
		}
		c.extraSlot = primitive.NewSlot("card-extra", c.Extra)
		row := primitive.Row(titleNode, primitive.Spacer(), c.extraSlot)
		row.CrossAlign = core.CrossCenter
		row.Gap = th.SizeOr(core.TokenMarginSM, 8)
		row.ExpandMax = true

		c.headerDec = primitive.NewDecorated(row)
		c.headerDec.Padding = primitive.EdgeInsets{Left: headPad, Right: headPad}
		c.headerDec.MinHeight = headMinH
		c.headerDec.ExpandWidth = true
		c.headerDec.BorderWidth = lineW
		c.headerDec.BorderColor = th.Color(core.TokenColorBorderSecondary)
		// only bottom border: draw as full border then same bg — approximate with
		// a thin bottom divider child instead for hit==paint clarity.
		// Use a column: row + divider line.
		div := primitive.NewDecorated()
		div.Height = lineW
		div.ExpandWidth = true
		div.Background = th.Color(core.TokenColorBorderSecondary)
		div.Hit = core.HitDefer
		headCol := primitive.Column(row, div)
		headCol.Gap = 0
		headCol.CrossAlign = core.CrossStretch
		headCol.ExpandMax = true
		c.headerDec = primitive.NewDecorated(headCol)
		c.headerDec.Padding = primitive.EdgeInsets{
			Left: headPad, Right: headPad,
			Top: headPad * 0.5, Bottom: 0, // vertical rhythm ≈ minHeight via content
		}
		// Prefer min height via MinHeight on decorated.
		c.headerDec.MinHeight = headMinH
		c.headerDec.ExpandWidth = true
		c.headerDec.Hit = core.HitDefer
		if c.Type == CardTypeInner {
			c.headerDec.Background = th.Color(core.TokenColorFillSecondary)
		} else {
			c.headerDec.Background = render.RGBA{} // transparent over root
		}
		// Top radius only when first child — applied on root, head is square inside.
		col.AddChild(c.headerDec)
	}

	// --- cover ---
	c.coverSlot = nil
	if c.Cover != nil {
		c.coverSlot = primitive.NewSlot("card-cover", c.Cover)
		col.AddChild(c.coverSlot)
	}

	// --- body ---
	c.skeleton = nil
	c.bodySlot = nil
	var bodyChild core.Node
	containGrid := len(c.Grids) > 0
	if c.Loading {
		// antd: Skeleton loading active paragraph={{ rows: 4 }} title={false}
		sk := NewSkeleton()
		sk.SetTitle(false)
		sk.SetParagraphRows(DefaultCardLoadingRows)
		sk.SetActive(true)
		sk.SetTheme(c.Theme)
		c.skeleton = sk
		bodyChild = sk.Node()
	} else if containGrid {
		gridRow := primitive.Row()
		gridRow.Wrap = true
		gridRow.Gap = 0
		gridRow.CrossAlign = core.CrossStart
		gridRow.ExpandMax = true
		for _, g := range c.Grids {
			if g == nil {
				continue
			}
			gridRow.AddChild(g.Node())
		}
		bodyChild = gridRow
	} else if c.content != nil {
		c.bodySlot = primitive.NewSlot("card-body", c.content)
		bodyChild = c.bodySlot
	}

	c.bodyDec = nil
	if bodyChild != nil || c.Loading {
		if bodyChild == nil {
			bodyChild = primitive.NewSlot("card-body-empty", nil)
		}
		c.bodyDec = primitive.NewDecorated(bodyChild)
		c.bodyDec.ExpandWidth = true
		c.bodyDec.Hit = core.HitDefer
		if containGrid && !c.Loading {
			c.bodyDec.Padding = primitive.EdgeInsets{}
		} else if c.Type == CardTypeInner {
			// padding: `${padding} ${bodyPadding}` → 16 vertical, 24 horizontal (middle)
			v := th.SizeOr(core.TokenPadding, 16)
			h := bodyPad
			if c.Size == CardSmall {
				h = DefaultCardBodyPaddingSM
			}
			c.bodyDec.Padding = primitive.EdgeInsets{Top: v, Bottom: v, Left: h, Right: h}
		} else {
			c.bodyDec.Padding = primitive.All(bodyPad)
		}
		col.AddChild(c.bodyDec)
	}

	// --- actions ---
	c.actionsRow = nil
	if len(c.Actions) > 0 {
		actPadV := th.SizeOr(core.TokenPaddingSM, DefaultCardActionsPadV)
		n := len(c.Actions)
		row := primitive.Row()
		row.CrossAlign = core.CrossCenter
		row.ExpandMax = true
		row.Gap = 0
		split := th.Color(core.TokenColorBorderSecondary)
		for i, a := range c.Actions {
			if a == nil {
				continue
			}
			// Each action equal flex; optional Pressable wrap if not already interactive.
			cellInner := a
			cell := primitive.NewDecorated(cellInner)
			cell.Padding = primitive.EdgeInsets{Top: actPadV, Bottom: actPadV}
			cell.Hit = core.HitDefer
			// Center content.
			wrap := primitive.NewFlexible(1, cell)
			wrap.FillChild = true
			row.AddChild(wrap)
			if i < n-1 {
				sep := primitive.NewDecorated()
				sep.Width = lineW
				sep.ExpandWidth = false
				sep.Background = split
				sep.Hit = core.HitDefer
				// Stretch height via MinHeight later; use flexible height from row.
				sep.MinHeight = actPadV * 2
				row.AddChild(sep)
			}
		}
		actDec := primitive.NewDecorated(row)
		actDec.ExpandWidth = true
		actDec.Background = th.Color(core.TokenColorBgContainer)
		// Top divider
		topDiv := primitive.NewDecorated()
		topDiv.Height = lineW
		topDiv.ExpandWidth = true
		topDiv.Background = split
		topDiv.Hit = core.HitDefer
		actCol := primitive.Column(topDiv, actDec)
		actCol.Gap = 0
		actCol.CrossAlign = core.CrossStretch
		actCol.ExpandMax = true
		c.actionsRow = row
		col.AddChild(actCol)
	}

	c.Root = primitive.NewDecorated(col)
	c.Root.SkinType = TypeCard
	c.Root.Radius = radius
	c.Root.ExpandWidth = c.Width <= 0 && c.Style.Width <= 0
	if c.Width > 0 {
		c.Root.Width = c.Width
	} else if c.Style.Width > 0 {
		c.Root.Width = c.Style.Width
	}
	c.Root.Hit = core.HitBlock
	c.applyChrome()
	c.applyA11y()

	// Pressable shell for hover / click.
	c.press = nil
	if c.needsPressable() {
		p := primitive.NewPressable(c.Root)
		p.Focusable = c.OnClick != nil
		p.ShowFocusRing = c.OnClick != nil
		p.FocusRingOutset = DefaultCardFocusRingOutset
		p.FocusRingRadius = radius
		p.EnableRipple = false
		p.SetDisabled(c.Disabled)
		p.Click = func() {
			if c.Disabled || c.OnClick == nil {
				return
			}
			c.OnClick()
		}
		p.OnStateChange = func() {
			c.lastHovered = p.State.Hovered
			c.applyChrome()
		}
		c.press = p
	} else {
		c.lastHovered = false
	}

	if c.boundTree != nil && c.skeleton != nil {
		c.skeleton.AttachTicker(c.boundTree)
	}
}

func (c *Card) applyA11y() {
	if c == nil || c.Root == nil {
		return
	}
	// region / group role for container
	c.Root.Base().Role = "group"
	name := c.AriaLabel
	if name == "" {
		name = c.Title
	}
	c.Root.Base().Label = name
	if c.press != nil {
		c.press.Base().Role = "group"
		c.press.Base().Label = name
	}
}

func (c *Card) applyChrome() {
	if c == nil || c.Root == nil {
		return
	}
	th := c.theme()
	radius := c.radiusResolved()
	lineW := c.lineWResolved()

	bg := th.Color(core.TokenColorBgContainer)
	if c.Style.hasBG() {
		bg = c.Style.Background
	}
	border := th.Color(core.TokenColorBorderSecondary)
	if c.Style.hasBorder() {
		border = c.Style.Border
	}

	c.Root.Radius = radius
	c.Root.Background = bg

	switch {
	case c.Disabled:
		c.Root.Background = th.Color(core.TokenColorDisabledBg)
		if c.Variant == CardBorderless {
			c.Root.BorderWidth = 0
		} else {
			c.Root.BorderWidth = lineW
			c.Root.BorderColor = th.Color(core.TokenColorBorder)
		}
		if c.titleLab != nil {
			c.titleLab.Color = th.Color(core.TokenColorDisabledText)
			c.titleLab.MarkNeedsPaint()
		}
	case c.Hoverable && c.lastHovered && !c.Disabled:
		// antd hoverable: borderColor transparent + boxShadowCard (approx).
		c.Root.BorderWidth = lineW
		c.Root.BorderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.06} // soft edge ≈ shadow
		// Slight lift via secondary fill wash.
		c.Root.Background = bg
	case c.Variant == CardBorderless:
		c.Root.BorderWidth = 0
		c.Root.BorderColor = render.RGBA{}
	default:
		c.Root.BorderWidth = lineW
		c.Root.BorderColor = border
	}

	if c.headerDec != nil && c.Type == CardTypeInner && !c.Disabled {
		c.headerDec.Background = th.Color(core.TokenColorFillSecondary)
	}
	c.Root.MarkNeedsPaint()
}

// ---------------------------------------------------------------------------
// Card.Meta
// ---------------------------------------------------------------------------

// CardMeta is Ant Design Card.Meta (avatar + title + description).
type CardMeta struct {
	Root *primitive.Decorated

	Avatar      core.Node
	Title       string
	TitleNode   core.Node
	Description string
	DescNode    core.Node

	Face  text.Face
	Theme *core.Theme
	Style Style
}

// NewCardMeta creates an empty Meta; set fields then Node().
func NewCardMeta() *CardMeta {
	m := &CardMeta{}
	m.rebuild()
	return m
}

// Node returns root.
func (m *CardMeta) Node() core.Node {
	if m == nil {
		return nil
	}
	if m.Root == nil {
		m.rebuild()
	}
	return m.Root
}

// SetAvatar sets the leading avatar node.
func (m *CardMeta) SetAvatar(n core.Node) {
	if m == nil {
		return
	}
	m.Avatar = n
	m.rebuild()
}

// SetTitle sets title string.
func (m *CardMeta) SetTitle(s string) {
	if m == nil {
		return
	}
	m.Title = s
	m.TitleNode = nil
	m.rebuild()
}

// SetTitleNode sets custom title node.
func (m *CardMeta) SetTitleNode(n core.Node) {
	if m == nil {
		return
	}
	m.TitleNode = n
	m.rebuild()
}

// SetDescription sets description string.
func (m *CardMeta) SetDescription(s string) {
	if m == nil {
		return
	}
	m.Description = s
	m.DescNode = nil
	m.rebuild()
}

// SetDescriptionNode sets custom description node.
func (m *CardMeta) SetDescriptionNode(n core.Node) {
	if m == nil {
		return
	}
	m.DescNode = n
	m.rebuild()
}

// SetFace sets font.
func (m *CardMeta) SetFace(face text.Face) {
	if m == nil {
		return
	}
	m.Face = face
	m.rebuild()
}

// SetTheme sets theme.
func (m *CardMeta) SetTheme(th *core.Theme) {
	if m == nil {
		return
	}
	m.Theme = th
	m.rebuild()
}

func (m *CardMeta) theme() *core.Theme {
	var n core.Node
	if m.Root != nil {
		n = m.Root
	}
	return themeOf(m.Theme, n)
}

func (m *CardMeta) rebuild() {
	th := m.theme()
	row := primitive.Row()
	row.CrossAlign = core.CrossStart
	row.Gap = 0

	if m.Avatar != nil {
		avWrap := primitive.NewDecorated(m.Avatar)
		avWrap.Padding = primitive.EdgeInsets{Right: th.SizeOr(core.TokenPadding, DefaultCardMetaAvatarPad)}
		avWrap.Hit = core.HitDefer
		row.AddChild(avWrap)
	}

	section := primitive.Column()
	section.Gap = th.SizeOr(core.TokenMarginXS, DefaultCardMetaSectionGap)
	section.CrossAlign = core.CrossStart

	if m.TitleNode != nil {
		section.AddChild(m.TitleNode)
	} else if m.Title != "" {
		t := primitive.NewText(m.Title)
		t.FontSize = th.SizeOr(core.TokenFontSizeLG, DefaultCardMetaTitleFont)
		t.Face = m.Face
		if m.Style.hasText() {
			t.Color = m.Style.Text
		} else {
			t.Color = th.Color(core.TokenColorText)
		}
		section.AddChild(t)
	}

	if m.DescNode != nil {
		section.AddChild(m.DescNode)
	} else if m.Description != "" {
		d := primitive.NewText(m.Description)
		d.FontSize = th.SizeOr(core.TokenFontSize, DefaultCardFontSize)
		d.Face = m.Face
		d.Color = th.Color(core.TokenColorTextSecondary)
		section.AddChild(d)
	}

	if len(section.Base().Children()) > 0 {
		flex := primitive.NewFlexible(1, section)
		flex.FillChild = true
		row.AddChild(flex)
	}

	m.Root = primitive.NewDecorated(row)
	m.Root.Hit = core.HitDefer
	// antd margin: -marginXXS 0 → ignore for kit (no negative margin layout)
	if m.Style.hasBG() {
		m.Root.Background = m.Style.Background
	}
}

// ---------------------------------------------------------------------------
// Card.Grid
// ---------------------------------------------------------------------------

// CardGrid is Ant Design Card.Grid (equal cell inside contain-grid body).
type CardGrid struct {
	Root *primitive.Decorated

	content   core.Node
	Hoverable bool
	// WidthFrac is 0..1 share of card width (0 → DefaultCardGridWidthFrac 1/3).
	WidthFrac float64
	// WidthPx forces pixel width when > 0 (overrides frac in fixed layouts).
	WidthPx float64

	Disabled bool
	OnClick  func()

	Face  text.Face
	Theme *core.Theme
	Style Style

	press       *primitive.Pressable
	lastHovered bool
}

// NewCardGrid creates a grid cell (hoverable default true).
func NewCardGrid() *CardGrid {
	g := &CardGrid{Hoverable: true}
	g.rebuild()
	return g
}

// Node returns mount root.
func (g *CardGrid) Node() core.Node {
	if g == nil {
		return nil
	}
	if g.Root == nil {
		g.rebuild()
	}
	if g.press != nil {
		return g.press
	}
	return g.Root
}

// ChromeNode returns Decorated chrome.
func (g *CardGrid) ChromeNode() core.Node {
	if g == nil {
		return nil
	}
	if g.Root == nil {
		g.rebuild()
	}
	return g.Root
}

// SetContent sets cell body.
func (g *CardGrid) SetContent(n core.Node) {
	if g == nil {
		return
	}
	g.content = n
	g.rebuild()
}

// SetHoverable toggles hover chrome (antd default true).
func (g *CardGrid) SetHoverable(v bool) {
	if g == nil {
		return
	}
	g.Hoverable = v
	g.rebuild()
}

// SetWidthFrac sets width fraction (e.g. 0.25 for 25%).
func (g *CardGrid) SetWidthFrac(f float64) {
	if g == nil {
		return
	}
	g.WidthFrac = f
	g.rebuild()
}

// SetWidthPx sets fixed width in px.
func (g *CardGrid) SetWidthPx(px float64) {
	if g == nil {
		return
	}
	g.WidthPx = px
	g.rebuild()
}

// SetOnClick sets click handler.
func (g *CardGrid) SetOnClick(fn func()) {
	if g == nil {
		return
	}
	g.OnClick = fn
	g.rebuild()
}

// SetTheme sets theme.
func (g *CardGrid) SetTheme(th *core.Theme) {
	if g == nil {
		return
	}
	g.Theme = th
	g.rebuild()
}

// SetFace sets font.
func (g *CardGrid) SetFace(face text.Face) {
	if g == nil {
		return
	}
	g.Face = face
}

func (g *CardGrid) theme() *core.Theme {
	var n core.Node
	if g.Root != nil {
		n = g.Root
	}
	return themeOf(g.Theme, n)
}

func (g *CardGrid) padResolved() float64 {
	return g.theme().SizeOr(core.TokenPaddingLG, DefaultCardGridPadding)
}

func (g *CardGrid) rebuild() {
	th := g.theme()
	pad := g.padResolved()
	lineW := th.SizeOr(core.TokenLineWidth, DefaultCardLineWidth)
	split := th.Color(core.TokenColorBorderSecondary)

	var child core.Node = g.content
	if child == nil {
		child = primitive.NewSlot("card-grid-empty", nil)
	}
	// Center text-ish content by default (demo grid-card uses textAlign center).
	inner := primitive.NewDecorated(child)
	inner.Padding = primitive.All(pad)
	inner.Hit = core.HitDefer

	g.Root = primitive.NewDecorated(inner)
	g.Root.Radius = 0
	g.Root.BorderWidth = 0
	// Approximate box-shadow grid lines with 1px borders on end/bottom.
	g.Root.BorderWidth = lineW
	g.Root.BorderColor = split
	g.Root.Background = th.Color(core.TokenColorBgContainer)
	g.Root.Hit = core.HitBlock

	if g.WidthPx > 0 {
		g.Root.Width = g.WidthPx
	} else if g.Style.Width > 0 {
		g.Root.Width = g.Style.Width
	} else {
		// Frac is applied by parent when known; store preferred via MinWidth hint only.
		// Parent contain-grid Row wraps; without % layout we use WidthFrac * 300 default
		// when WidthFrac set, else ~1/3 of 300.
		frac := g.WidthFrac
		if frac <= 0 {
			frac = DefaultCardGridWidthFrac
		}
		// Prefer ExpandWidth false; use a reasonable demo default width so layout works
		// without percentage engine. Callers (gallery) can SetWidthPx.
		if g.Root.Width == 0 {
			g.Root.Width = 300 * frac // soft default; gallery sets explicit
		}
	}

	g.press = nil
	if g.Hoverable || g.OnClick != nil {
		p := primitive.NewPressable(g.Root)
		p.Focusable = g.OnClick != nil
		p.ShowFocusRing = g.OnClick != nil
		p.EnableRipple = false
		p.SetDisabled(g.Disabled)
		p.Click = func() {
			if g.Disabled || g.OnClick == nil {
				return
			}
			g.OnClick()
		}
		p.OnStateChange = func() {
			g.lastHovered = p.State.Hovered
			g.applyChrome()
		}
		g.press = p
	}
	g.applyChrome()
}

func (g *CardGrid) applyChrome() {
	if g == nil || g.Root == nil {
		return
	}
	th := g.theme()
	lineW := th.SizeOr(core.TokenLineWidth, DefaultCardLineWidth)
	split := th.Color(core.TokenColorBorderSecondary)
	bg := th.Color(core.TokenColorBgContainer)
	if g.Style.hasBG() {
		bg = g.Style.Background
	}
	g.Root.Background = bg
	g.Root.BorderWidth = lineW
	if g.Hoverable && g.lastHovered && !g.Disabled {
		g.Root.BorderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.08}
	} else {
		g.Root.BorderColor = split
	}
	g.Root.MarkNeedsPaint()
}
