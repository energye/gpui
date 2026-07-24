package kit

import (
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Typography defaults — docs/antd/typography.md §6.2 / §6.10
const (
	DefaultTypographyFontSize     = 14.0
	DefaultTitleFontSizeH1        = 38.0
	DefaultTitleFontSizeH2        = 30.0
	DefaultTitleFontSizeH3        = 24.0
	DefaultTitleFontSizeH4        = 20.0
	DefaultTitleFontSizeH5        = 16.0
	DefaultTypographyActionGap    = 4.0 // marginXXS
	DefaultTypographyActionIcon   = 14.0
	DefaultTypographyCodeRadius   = 3.0
	DefaultTypographyCopyFeedback = 3.0 // seconds
	DefaultTypographyExpandLabel  = "展开"
	DefaultTypographyCollapseLab  = "收起"
)

// TypographyKind is Text / Title / Paragraph / Link.
type TypographyKind int

const (
	TypographyText TypographyKind = iota
	TypographyTitle
	TypographyParagraph
	TypographyLink
)

// TypographyType is semantic text color (antd type).
type TypographyType int

const (
	TypographyTypeDefault TypographyType = iota
	TypographyTypeSecondary
	TypographyTypeSuccess
	TypographyTypeWarning
	TypographyTypeDanger
)

// TypographyActionsPlacement places copy/edit/expand relative to content.
type TypographyActionsPlacement int

const (
	TypographyActionsEnd TypographyActionsPlacement = iota
	TypographyActionsStart
)

// Typography is Ant Design Typography (Text / Title / Paragraph / Link).
//
//	host Flex (row) — omitted when no actions/chrome and not editing
//	  ├─ actions? (start)
//	  ├─ content chrome? (code/mark/keyboard)
//	  │    └─ Text | EditableText
//	  └─ actions? (end)
//
// Product contract: docs/antd/typography.md §6 (P0 DoD).
//
// Compatibility: Root is the content *primitive.Text so simple labels may still
// set Root.FontSize; Node() returns the outer host when actions/chrome exist.
type Typography struct {
	// Root is the content text node (always non-nil after rebuild).
	Root *primitive.Text

	host    *typographyHost
	chrome  *primitive.Decorated
	editor  *primitive.EditableText
	actions *primitive.Flex

	// Kind selects Text/Title/Paragraph/Link rendering.
	Kind TypographyKind
	// Value is the string content.
	Value string
	// Level is Title importance 1..5 (default 1).
	Level int
	// Type is semantic color.
	Type TypographyType
	// Disabled greys out and blocks interactions.
	Disabled bool

	// Text decorations (antd boolean props).
	Strong    bool
	Code      bool
	Mark      bool
	Delete    bool
	Underline bool
	Italic    bool
	Keyboard  bool

	// Copyable
	Copyable  bool
	CopyText  string // empty → Value
	CopyIcon  string // empty → default glyph
	OnCopy    func(text string)
	copyOK    bool
	copyTimer float64

	// Editable
	Editable   bool
	editing    bool
	editingSet bool // true after SetEditing (controlled)
	editDraft  string
	editBackup string
	OnChange   func(value string)
	OnStart    func()
	OnEnd      func()
	OnCancel   func()

	// Ellipsis
	Ellipsis        bool
	EllipsisRows    int // 0 → 1 for Text; Paragraph may set higher
	Expandable      bool
	Collapsible     bool
	expanded        bool
	expandedSet     bool // true after SetExpanded (controlled)
	DefaultExpanded bool
	EllipsisMiddle  bool
	Suffix          string
	SymbolExpand    string
	SymbolCollapse  string
	OnExpand        func(expanded bool)
	OnEllipsis      func(ellipsis bool)
	isEllipsis      bool

	// ActionsPlacement default end.
	ActionsPlacement TypographyActionsPlacement

	// Layout
	MaxWidth  float64
	FontSize  float64 // 0 → kind default
	Face      text.Face
	Theme     *core.Theme
	Style     Style
	AriaLabel string
	OnClick   func()

	// Secondary is a legacy alias for Type=Secondary (kept for call sites).
	// Prefer SetType(TypographyTypeSecondary).
	Secondary bool

	life      tickerLifecycle
	boundTree *core.Tree
}

// Text / Title / Paragraph / Link type aliases (antd subcomponents).
type (
	Text      = Typography
	Title     = Typography
	Paragraph = Typography
	Link      = Typography
)

// typographyHost owns mount lifecycle + Escape while editing.
type typographyHost struct {
	primitive.Flex
	ty *Typography
}

func (h *typographyHost) TypeID() string { return TypeTypography }

func (h *typographyHost) OnMount() {
	if h == nil || h.ty == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.ty.boundTree = t
		h.ty.life.attach(t, h.ty, h.ty.needsTicker())
	}
}

func (h *typographyHost) OnUnmount() {
	if h != nil && h.ty != nil {
		h.ty.life.unmount()
		h.ty.boundTree = nil
	}
}

func (h *typographyHost) HandleKey(ev *core.KeyEvent) {
	if h == nil || h.ty == nil || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !h.ty.isEditing() {
		return
	}
	if ev.Key == "Escape" || ev.Key == "Esc" {
		h.ty.cancelEdit()
		ev.Handled = true
	}
}

// NewTypography creates Typography.Text with Ant defaults.
func NewTypography(value string) *Typography {
	t := &Typography{
		Kind:  TypographyText,
		Value: value,
		Level: 1,
	}
	t.rebuild()
	return t
}

// NewText is NewTypography (Typography.Text).
func NewText(value string) *Typography { return NewTypography(value) }

// NewTitle creates Typography.Title (level 1–5; out of range → 5).
func NewTitle(value string, level int) *Typography {
	t := &Typography{
		Kind:  TypographyTitle,
		Value: value,
		Level: clampTitleLevel(level),
	}
	t.rebuild()
	return t
}

// NewParagraph creates Typography.Paragraph (multi-line body).
func NewParagraph(value string) *Typography {
	t := &Typography{
		Kind:         TypographyParagraph,
		Value:        value,
		EllipsisRows: 8, // wrap budget for dense UI (not ellipsis unless SetEllipsis)
	}
	t.rebuild()
	return t
}

// NewLink creates Typography.Link (primary color, focusable, clickable).
func NewLink(value string) *Typography {
	t := &Typography{
		Kind:  TypographyLink,
		Value: value,
	}
	t.rebuild()
	return t
}

// Node returns the root core.Node.
func (t *Typography) Node() core.Node {
	if t == nil {
		return nil
	}
	if t.Root == nil {
		t.rebuild()
	}
	if t.host != nil {
		return t.host
	}
	return t.Root
}

// ContentNode returns the content text (or editor when editing).
func (t *Typography) ContentNode() core.Node {
	if t == nil {
		return nil
	}
	if t.isEditing() && t.editor != nil {
		return t.editor
	}
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// ChromeNode returns code/mark/keyboard Decorated chrome when present.
func (t *Typography) ChromeNode() core.Node {
	if t == nil {
		return nil
	}
	if t.chrome != nil {
		return t.chrome
	}
	return t.ContentNode()
}

// ValueOf returns the current string.
func (t *Typography) ValueOf() string {
	if t == nil {
		return ""
	}
	return t.Value
}

// SetValue updates content text.
func (t *Typography) SetValue(v string) {
	if t == nil {
		return
	}
	t.Value = v
	if t.isEditing() && t.editor != nil {
		t.editor.SetValue(v)
		return
	}
	t.applyContent()
}

// SetKind switches Text/Title/Paragraph/Link.
func (t *Typography) SetKind(k TypographyKind) {
	if t == nil || t.Kind == k {
		return
	}
	t.Kind = k
	t.rebuild()
}

// SetLevel sets Title level 1..5.
func (t *Typography) SetLevel(level int) {
	if t == nil {
		return
	}
	level = clampTitleLevel(level)
	if t.Level == level {
		return
	}
	t.Level = level
	t.applyChrome()
}

// SetType sets semantic color type.
func (t *Typography) SetType(ty TypographyType) {
	if t == nil {
		return
	}
	t.Type = ty
	t.Secondary = ty == TypographyTypeSecondary
	t.applyChrome()
}

// SetSecondary is a legacy alias for type=secondary.
func (t *Typography) SetSecondary(v bool) {
	if t == nil {
		return
	}
	t.Secondary = v
	if v {
		t.Type = TypographyTypeSecondary
	} else if t.Type == TypographyTypeSecondary {
		t.Type = TypographyTypeDefault
	}
	t.applyChrome()
}

// SetDisabled toggles disabled.
func (t *Typography) SetDisabled(d bool) {
	if t == nil {
		return
	}
	t.Disabled = d
	t.applyChrome()
	t.wireActions()
}

// SetStrong / SetCode / SetMark / SetDelete / SetUnderline / SetItalic / SetKeyboard.

func (t *Typography) SetStrong(v bool) {
	if t == nil {
		return
	}
	t.Strong = v
	t.applyChrome()
}

func (t *Typography) SetCode(v bool) {
	if t == nil {
		return
	}
	if t.Code == v {
		return
	}
	t.Code = v
	t.rebuild()
}

func (t *Typography) SetMark(v bool) {
	if t == nil {
		return
	}
	if t.Mark == v {
		return
	}
	t.Mark = v
	t.rebuild()
}

func (t *Typography) SetDelete(v bool) {
	if t == nil {
		return
	}
	t.Delete = v
	t.applyChrome()
}

func (t *Typography) SetUnderline(v bool) {
	if t == nil {
		return
	}
	t.Underline = v
	t.applyChrome()
}

func (t *Typography) SetItalic(v bool) {
	if t == nil {
		return
	}
	t.Italic = v
	t.applyChrome()
}

func (t *Typography) SetKeyboard(v bool) {
	if t == nil {
		return
	}
	if t.Keyboard == v {
		return
	}
	t.Keyboard = v
	t.rebuild()
}

// SetCopyable enables copy action.
func (t *Typography) SetCopyable(v bool) {
	if t == nil {
		return
	}
	if t.Copyable == v {
		return
	}
	t.Copyable = v
	t.rebuild()
}

// SetCopyText sets clipboard payload (empty → Value).
func (t *Typography) SetCopyText(s string) {
	if t == nil {
		return
	}
	t.CopyText = s
}

// SetCopyIcon sets copy icon registry name.
func (t *Typography) SetCopyIcon(name string) {
	if t == nil {
		return
	}
	t.CopyIcon = name
	if t.Copyable {
		t.rebuild()
	}
}

// SetEditable enables edit action.
func (t *Typography) SetEditable(v bool) {
	if t == nil {
		return
	}
	if t.Editable == v {
		return
	}
	t.Editable = v
	if !v && t.isEditing() {
		t.editing = false
		t.editingSet = false
	}
	t.rebuild()
}

// SetEditing sets controlled editing state.
func (t *Typography) SetEditing(v bool) {
	if t == nil {
		return
	}
	t.editingSet = true
	if t.editing == v {
		return
	}
	if v {
		t.beginEdit()
	} else if t.isEditing() {
		// controlled exit without commit
		t.editing = false
		t.rebuild()
	}
}

// IsEditing reports whether the control is in edit mode.
func (t *Typography) IsEditing() bool { return t != nil && t.isEditing() }

// SetEllipsis enables overflow ellipsis.
func (t *Typography) SetEllipsis(on bool) {
	if t == nil {
		return
	}
	t.Ellipsis = on
	t.applyContent()
}

// SetEllipsisRows sets max lines when ellipsis/wrap is active.
func (t *Typography) SetEllipsisRows(n int) {
	if t == nil {
		return
	}
	t.EllipsisRows = n
	t.applyContent()
}

// SetMaxLines is an alias of SetEllipsisRows (legacy Paragraph API).
func (t *Typography) SetMaxLines(n int) { t.SetEllipsisRows(n) }

// SetExpandable enables expand control when ellipsis overflows.
func (t *Typography) SetExpandable(v bool) {
	if t == nil {
		return
	}
	if t.Expandable == v {
		return
	}
	t.Expandable = v
	t.rebuild()
}

// SetCollapsible allows collapsing after expand (antd expandable='collapsible').
func (t *Typography) SetCollapsible(v bool) {
	if t == nil {
		return
	}
	t.Collapsible = v
	t.wireActions()
}

// SetExpanded sets controlled expanded state.
func (t *Typography) SetExpanded(v bool) {
	if t == nil {
		return
	}
	t.expandedSet = true
	if t.expanded == v {
		t.applyContent()
		t.wireActions()
		return
	}
	t.expanded = v
	t.applyContent()
	t.wireActions()
}

// Expanded reports current expanded state.
func (t *Typography) Expanded() bool {
	if t == nil {
		return false
	}
	return t.resolvedExpanded()
}

// SetDefaultExpanded sets non-controlled initial expanded.
func (t *Typography) SetDefaultExpanded(v bool) {
	if t == nil || t.expandedSet {
		return
	}
	t.DefaultExpanded = v
	t.expanded = v
	t.applyContent()
}

// SetEllipsisMiddle enables middle-ellipsis (start…end).
func (t *Typography) SetEllipsisMiddle(v bool) {
	if t == nil {
		return
	}
	t.EllipsisMiddle = v
	t.applyContent()
}

// SetSuffix sets ellipsis suffix (appended after truncated text).
func (t *Typography) SetSuffix(s string) {
	if t == nil {
		return
	}
	t.Suffix = s
	t.applyContent()
}

// SetExpandSymbol sets expand/collapse labels (empty → 展开/收起).
func (t *Typography) SetExpandSymbol(expand, collapse string) {
	if t == nil {
		return
	}
	t.SymbolExpand = expand
	t.SymbolCollapse = collapse
	t.wireActions()
}

// SetActionsPlacement sets start/end for the action bar.
func (t *Typography) SetActionsPlacement(p TypographyActionsPlacement) {
	if t == nil {
		return
	}
	if t.ActionsPlacement == p {
		return
	}
	t.ActionsPlacement = p
	t.rebuild()
}

// SetMaxWidth constrains content width (needed for ellipsis).
func (t *Typography) SetMaxWidth(w float64) {
	if t == nil {
		return
	}
	t.MaxWidth = w
	t.applyContent()
}

// SetFontSize overrides resolved font size (0 → kind default).
func (t *Typography) SetFontSize(px float64) {
	if t == nil {
		return
	}
	t.FontSize = px
	t.applyChrome()
}

// SetFace sets the font face.
func (t *Typography) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	if t.Root != nil {
		t.Root.Face = face
		t.Root.MarkNeedsLayout()
	}
	if t.editor != nil {
		t.editor.Face = face
		t.editor.MarkNeedsLayout()
	}
}

// SetTheme sets an explicit theme override.
func (t *Typography) SetTheme(th *core.Theme) {
	if t == nil {
		return
	}
	t.Theme = th
	t.applyChrome()
}

// SetStyle sets optional visual overrides.
func (t *Typography) SetStyle(st Style) {
	if t == nil {
		return
	}
	t.Style = st
	t.applyChrome()
}

// SetAriaLabel sets accessible name for the text/link.
func (t *Typography) SetAriaLabel(name string) {
	if t == nil {
		return
	}
	t.AriaLabel = name
	t.applyA11y()
}

// SetOnClick sets click handler (Link/Text).
func (t *Typography) SetOnClick(fn func()) {
	if t == nil {
		return
	}
	t.OnClick = fn
	t.rebuild()
}

// ResolvedFontSize returns the effective point size (§6.2).
func (t *Typography) ResolvedFontSize() float64 {
	if t == nil {
		return DefaultTypographyFontSize
	}
	return t.resolvedFontSize()
}

// ResolvedColor returns the effective text color.
func (t *Typography) ResolvedColor() render.RGBA {
	if t == nil {
		return render.RGBA{}
	}
	return t.resolvedColor(t.theme())
}

// Tick drives copy-success feedback.
func (t *Typography) Tick(dt float64) bool {
	if t == nil {
		return false
	}
	if t.boundTree == nil {
		return false
	}
	if !t.copyOK {
		return false
	}
	t.copyTimer -= dt
	if t.copyTimer <= 0 {
		t.copyOK = false
		t.copyTimer = 0
		t.wireActions()
		return false
	}
	return true
}

// ── internals ──────────────────────────────────────────────────────────────

func (t *Typography) theme() *core.Theme {
	var n core.Node
	if t.host != nil {
		n = t.host
	} else if t.Root != nil {
		n = t.Root
	}
	return themeOf(t.Theme, n)
}

func (t *Typography) needsHost() bool {
	return t.Copyable || t.Editable || t.Expandable || t.Code || t.Mark || t.Keyboard ||
		t.OnClick != nil || t.Kind == TypographyLink || t.isEditing()
}

func (t *Typography) needsTicker() bool { return t.copyOK }

func (t *Typography) isEditing() bool {
	if t.editingSet {
		return t.editing
	}
	return t.editing
}

func (t *Typography) resolvedExpanded() bool {
	if t.expandedSet {
		return t.expanded
	}
	return t.expanded
}

func (t *Typography) resolvedFontSize() float64 {
	if t.FontSize > 0 {
		return t.FontSize
	}
	if t.Style.FontSize > 0 {
		return t.Style.FontSize
	}
	th := t.theme()
	switch t.Kind {
	case TypographyTitle:
		return titleFontSize(t.Level, th)
	default:
		return th.SizeOr(core.TokenFontSize, DefaultTypographyFontSize)
	}
}

func titleFontSize(level int, th *core.Theme) float64 {
	level = clampTitleLevel(level)
	// Prefer Theme tokens when present; fall back to Ant defaults.
	keys := []string{
		"", // 1-based
		"fontSizeHeading1",
		"fontSizeHeading2",
		"fontSizeHeading3",
		"fontSizeHeading4",
		"fontSizeHeading5",
	}
	defaults := []float64{
		0,
		DefaultTitleFontSizeH1,
		DefaultTitleFontSizeH2,
		DefaultTitleFontSizeH3,
		DefaultTitleFontSizeH4,
		DefaultTitleFontSizeH5,
	}
	if th != nil {
		if v := th.Size(keys[level]); v > 0 {
			return v
		}
	}
	return defaults[level]
}

func clampTitleLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 5 {
		return 5
	}
	return level
}

func (t *Typography) resolvedColor(th *core.Theme) render.RGBA {
	if t.Style.hasText() {
		return t.Style.Text
	}
	if t.Disabled {
		return th.Color(core.TokenColorDisabledText)
	}
	ty := t.Type
	if t.Secondary && ty == TypographyTypeDefault {
		ty = TypographyTypeSecondary
	}
	switch ty {
	case TypographyTypeSecondary:
		return th.Color(core.TokenColorTextSecondary)
	case TypographyTypeSuccess:
		return th.Color(core.TokenColorSuccess)
	case TypographyTypeWarning:
		return th.Color(core.TokenColorWarning)
	case TypographyTypeDanger:
		return th.Color(core.TokenColorError)
	default:
		if t.Kind == TypographyLink {
			return th.Color(core.TokenColorPrimary)
		}
		if t.Strong {
			// heading-leaning emphasis
			c := th.Color(core.TokenColorText)
			return c
		}
		return th.Color(core.TokenColorText)
	}
}

func (t *Typography) rebuild() {
	th := t.theme()
	fs := t.resolvedFontSize()

	// Content text
	if t.Root == nil {
		t.Root = primitive.NewText(t.Value)
	}
	t.Root.Face = t.Face
	if t.Style.Face != nil {
		t.Root.Face = t.Style.Face
	}
	t.Root.FontSize = fs
	t.applyContent()
	t.applyChrome()

	// Editor (created lazily in beginEdit)
	if t.isEditing() {
		t.ensureEditor()
	} else {
		t.editor = nil
	}

	if !t.needsHost() {
		// Drop host if present; simple text is Root alone.
		if t.host != nil && t.boundTree != nil {
			// keep boundTree; host may unmount when parent replaces Node()
		}
		t.host = nil
		t.chrome = nil
		t.actions = nil
		t.applyA11y()
		return
	}

	content := t.buildContentNode(th)
	acts := t.buildActions(th)

	var kids []core.Node
	if t.ActionsPlacement == TypographyActionsStart && acts != nil {
		kids = append(kids, acts)
	}
	kids = append(kids, content)
	if t.ActionsPlacement != TypographyActionsStart && acts != nil {
		kids = append(kids, acts)
	}

	// Reuse host so mounted trees keep the same root identity.
	if t.host == nil {
		host := &typographyHost{ty: t}
		host.Axis = core.AxisHorizontal
		host.CrossAlign = core.CrossCenter
		host.MainAlign = core.MainStart
		host.Gap = DefaultTypographyActionGap
		host.Init(host)
		host.Hit = core.HitDefer
		t.host = host
	} else {
		t.host.ty = t
		t.host.Axis = core.AxisHorizontal
		t.host.CrossAlign = core.CrossCenter
		t.host.MainAlign = core.MainStart
		t.host.Gap = DefaultTypographyActionGap
		t.host.ClearChildren()
	}
	for _, k := range kids {
		if k != nil {
			t.host.AddChild(k)
		}
	}
	t.actions = acts
	t.host.MarkNeedsLayout()
	t.applyA11y()
}

func (t *Typography) buildContentNode(th *core.Theme) core.Node {
	var inner core.Node
	if t.isEditing() {
		t.ensureEditor()
		inner = t.editor
	} else {
		inner = t.Root
	}

	// Clickable: Link or OnClick (not while editing/disabled).
	if !t.isEditing() && !t.Disabled && (t.Kind == TypographyLink || t.OnClick != nil) {
		p := primitive.NewPressable(inner)
		p.ShowFocusRing = true
		p.EnableRipple = false
		p.FocusRingOutset = 1.5
		p.Click = func() {
			if t.Disabled {
				return
			}
			if t.OnClick != nil {
				t.OnClick()
			}
		}
		// Link uses pointer cursor (Pressable default).
		if t.Kind != TypographyLink && t.OnClick == nil {
			p.Focusable = false
		}
		inner = p
	}

	if t.Code || t.Mark || t.Keyboard {
		dec := primitive.NewDecorated(inner)
		dec.Radius = DefaultTypographyCodeRadius
		dec.Hit = core.HitDefer
		padH, padV := 0.4*t.resolvedFontSize()*0.5, 0.15*t.resolvedFontSize()
		if t.Code || t.Keyboard {
			dec.Padding = primitive.EdgeInsets{
				Left: padH * 2, Right: padH * 2, Top: padV, Bottom: padV,
			}
			dec.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
			dec.BorderColor = render.RGBA{R: 100.0 / 255, G: 100.0 / 255, B: 100.0 / 255, A: 0.2}
			if t.Code {
				dec.Background = render.RGBA{R: 150.0 / 255, G: 150.0 / 255, B: 150.0 / 255, A: 0.1}
			} else {
				// keyboard slightly lighter + thicker bottom (visual kbd)
				dec.Background = render.RGBA{R: 150.0 / 255, G: 150.0 / 255, B: 150.0 / 255, A: 0.06}
				dec.BorderWidth = 1
			}
		}
		if t.Mark {
			// antd gold[2]
			dec.Background = render.Hex("#FFE58F")
			dec.Padding = primitive.EdgeInsets{}
			dec.BorderWidth = 0
		}
		t.chrome = dec
		return dec
	}
	t.chrome = nil
	return inner
}

func (t *Typography) buildActions(th *core.Theme) *primitive.Flex {
	if t.isEditing() {
		return nil
	}
	var kids []core.Node
	if t.Expandable && (t.isEllipsis || t.resolvedExpanded()) {
		kids = append(kids, t.makeExpandAction(th))
	}
	if t.Editable && !t.Disabled {
		kids = append(kids, t.makeIconAction("edit", "编辑", th, func() {
			if t.Disabled {
				return
			}
			t.beginEdit()
		}))
	}
	if t.Copyable && !t.Disabled {
		label := "复制"
		icon := t.CopyIcon
		if icon == "" {
			icon = "copy"
		}
		if t.copyOK {
			icon = "check"
			label = "复制成功"
		}
		kids = append(kids, t.makeIconAction(icon, label, th, func() {
			t.doCopy()
		}))
	}
	if len(kids) == 0 {
		return nil
	}
	row := primitive.Row(kids...)
	row.Gap = DefaultTypographyActionGap
	row.CrossAlign = core.CrossCenter
	return row
}

func (t *Typography) makeExpandAction(th *core.Theme) core.Node {
	expanded := t.resolvedExpanded()
	label := t.SymbolExpand
	if label == "" {
		label = DefaultTypographyExpandLabel
	}
	if expanded {
		if t.Collapsible || true {
			// antd collapsible shows 收起; expandable-only stays 展开 until collapsed via control.
			lab := t.SymbolCollapse
			if lab == "" {
				lab = DefaultTypographyCollapseLab
			}
			if t.Collapsible {
				label = lab
			} else {
				// expandable once: still allow collapse in kit for controlled demos
				label = lab
			}
		}
	}
	tx := primitive.NewText(label)
	tx.FontSize = th.SizeOr(core.TokenFontSize, DefaultTypographyFontSize)
	tx.Face = t.Face
	tx.Color = th.Color(core.TokenColorPrimary)
	p := primitive.NewPressable(tx)
	p.EnableRipple = false
	p.ShowFocusRing = true
	p.FocusRingOutset = 1.5
	p.Click = func() {
		if t.Disabled {
			return
		}
		next := !t.resolvedExpanded()
		if t.expandedSet {
			// controlled: only notify
			if t.OnExpand != nil {
				t.OnExpand(next)
			}
			return
		}
		t.expanded = next
		t.applyContent()
		t.rebuild()
		if t.OnExpand != nil {
			t.OnExpand(next)
		}
	}
	p.Base().Role = "button"
	p.Base().Label = label
	return p
}

func (t *Typography) makeIconAction(iconName, aria string, th *core.Theme, click func()) core.Node {
	// Prefer registry icon; fall back to text glyph for unknown names.
	var child core.Node
	if _, ok := primitive.GlobalIcons.Lookup(iconName); ok || iconName == "check" || iconName == "copy" || iconName == "edit" {
		// Ensure copy/edit exist as simple painters.
		ensureTypographyIcons()
		ic := primitive.NewIcon(iconName)
		ic.Size = DefaultTypographyActionIcon
		ic.Color = th.Color(core.TokenColorPrimary)
		child = ic
	} else {
		tx := primitive.NewText(aria)
		tx.FontSize = DefaultTypographyActionIcon
		tx.Color = th.Color(core.TokenColorPrimary)
		child = tx
	}
	p := primitive.NewPressable(child)
	p.EnableRipple = false
	p.ShowFocusRing = true
	p.FocusRingOutset = 1.5
	p.Click = click
	p.Base().Role = "button"
	p.Base().Label = aria
	return p
}

func (t *Typography) wireActions() {
	// cheapest path: rebuild host children when actions visibility changes
	if t.needsHost() {
		t.rebuild()
	}
}

func (t *Typography) applyContent() {
	if t.Root == nil {
		return
	}
	display := t.Value
	if t.Suffix != "" && !t.resolvedExpanded() {
		// suffix is visual only with ellipsis; applied after truncate when possible
	}

	rows := t.EllipsisRows
	if rows <= 0 {
		if t.Kind == TypographyParagraph {
			rows = 0 // unconstrained wrap unless ellipsis
		} else {
			rows = 1
		}
	}

	expanded := t.resolvedExpanded()
	useEllipsis := t.Ellipsis && !expanded
	t.Root.MaxWidth = t.MaxWidth
	t.Root.Face = t.Face
	if t.Style.Face != nil {
		t.Root.Face = t.Style.Face
	}

	if useEllipsis && t.EllipsisMiddle && t.MaxWidth > 0 {
		// Middle ellipsis: pre-truncate string; disable primitive end-ellipsis.
		adv := t.Root // use layout-time advance via temporary measure
		_ = adv
		display = middleEllipsisString(t.Value, t.MaxWidth, t.resolvedFontSize(), t.Root.Face)
		if t.Suffix != "" {
			display = display + t.Suffix
		}
		t.Root.SetValue(display)
		t.Root.SetEllipsis(false)
		t.Root.SetMaxLines(1)
		t.flagEllipsis(display != t.Value && t.Value != "")
		return
	}

	if useEllipsis {
		t.Root.SetValue(t.Value)
		t.Root.SetEllipsis(true)
		if rows <= 0 {
			rows = 1
		}
		t.Root.SetMaxLines(rows)
		// suffix: append to value when single-line primitive can't host it separately
		if t.Suffix != "" {
			// Keep full value for copy; paint uses ellipsis. Suffix shown via expand label area in P1;
			// for P0, append suffix after value so it participates in measure when not overflowing hard.
			t.Root.SetValue(t.Value + t.Suffix)
		}
		t.flagEllipsis(true) // conservative; refined after layout if needed
		return
	}

	// Expanded or no ellipsis
	t.Root.SetValue(t.Value)
	t.Root.SetEllipsis(false)
	if t.Kind == TypographyParagraph && rows > 0 && !expanded {
		t.Root.SetMaxLines(rows)
	} else if expanded || rows <= 0 {
		t.Root.SetMaxLines(0)
	} else {
		t.Root.SetMaxLines(rows)
	}
	t.flagEllipsis(false)
}

func (t *Typography) flagEllipsis(v bool) {
	if t.isEllipsis == v {
		return
	}
	t.isEllipsis = v
	if t.OnEllipsis != nil {
		t.OnEllipsis(v)
	}
}

func (t *Typography) applyChrome() {
	if t.Root == nil {
		return
	}
	th := t.theme()
	fs := t.resolvedFontSize()
	t.Root.FontSize = fs
	t.Root.Color = t.resolvedColor(th)
	if t.editor != nil {
		t.editor.FontSize = fs
		t.editor.Color = t.resolvedColor(th)
		t.editor.Face = t.Face
	}

	var dec render.TextDecoration
	if t.Underline {
		dec |= render.TextDecorationUnderline
	}
	if t.Delete {
		dec |= render.TextDecorationLineThrough
	}
	t.Root.SetDecoration(dec)

	// Italic: best-effort via face; flag kept for API / future face axis.
	_ = t.Italic
	// Strong: slightly larger only when not Title (Title already bold-weight intent via size).
	if t.Strong && t.Kind != TypographyTitle && t.FontSize <= 0 {
		// keep size; color already heading-leaning
	}
	t.Root.MarkNeedsPaint()
}

func (t *Typography) applyA11y() {
	name := t.AriaLabel
	if name == "" {
		name = t.Value
	}
	if t.Root != nil {
		if t.Kind == TypographyLink {
			t.Root.Base().Role = "link"
		}
		if name != "" {
			t.Root.Base().Label = name
		}
	}
	if t.host != nil && t.Kind == TypographyLink {
		t.host.Base().Role = "link"
		if name != "" {
			t.host.Base().Label = name
		}
	}
}

func (t *Typography) ensureEditor() {
	if t.editor != nil {
		return
	}
	ed := primitive.NewEditableText()
	ed.Multiline = t.Kind == TypographyParagraph
	ed.FontSize = t.resolvedFontSize()
	ed.Face = t.Face
	ed.Color = t.resolvedColor(t.theme())
	ed.ShowFocusRing = true
	ed.SetValue(t.editDraft)
	ed.OnSubmit = func(v string) {
		t.commitEdit(v)
	}
	// live draft
	ed.OnChange = func(v string) {
		t.editDraft = v
	}
	t.editor = ed
}

func (t *Typography) beginEdit() {
	if t.Disabled || !t.Editable {
		return
	}
	if t.isEditing() {
		return
	}
	t.editBackup = t.Value
	t.editDraft = t.Value
	t.editing = true
	if !t.editingSet {
		// non-controlled
	}
	t.editor = nil
	t.rebuild()
	if t.editor != nil {
		if tr := t.tree(); tr != nil {
			t.boundTree = tr
			tr.SetFocus(t.editor)
			t.editor.AttachTicker(tr)
		}
	}
	if t.OnStart != nil {
		t.OnStart()
	}
}

func (t *Typography) commitEdit(v string) {
	if !t.isEditing() {
		return
	}
	t.Value = v
	t.editing = false
	if t.editingSet {
		// controlled: caller may keep editing true via SetEditing
	}
	t.editor = nil
	t.rebuild()
	if t.OnChange != nil {
		t.OnChange(v)
	}
	if t.OnEnd != nil {
		t.OnEnd()
	}
}

func (t *Typography) cancelEdit() {
	if !t.isEditing() {
		return
	}
	t.Value = t.editBackup
	t.editing = false
	t.editor = nil
	t.rebuild()
	if t.OnCancel != nil {
		t.OnCancel()
	}
}

func (t *Typography) doCopy() {
	if t.Disabled || !t.Copyable {
		return
	}
	payload := t.CopyText
	if payload == "" {
		payload = t.Value
	}
	if tr := t.tree(); tr != nil {
		if clip := tr.Clipboard(); clip != nil {
			_ = clip.WriteText(payload)
		} else {
			mc := core.NewMemoryClipboard()
			_ = mc.WriteText(payload)
			tr.SetClipboard(mc)
		}
	}
	if t.OnCopy != nil {
		t.OnCopy(payload)
	}
	t.copyOK = true
	t.copyTimer = DefaultTypographyCopyFeedback
	if tr := t.tree(); tr != nil {
		t.boundTree = tr
		t.life.attach(tr, t, true)
	}
	t.wireActions()
}

func (t *Typography) tree() *core.Tree {
	if t == nil {
		return nil
	}
	if t.boundTree != nil {
		return t.boundTree
	}
	if t.host != nil {
		if tr := t.host.Tree(); tr != nil {
			return tr
		}
	}
	if t.Root != nil {
		return t.Root.Tree()
	}
	return nil
}

// middleEllipsisString truncates s to fit maxW with an ellipsis in the middle.
func middleEllipsisString(s string, maxW, fontSize float64, face text.Face) string {
	if s == "" || maxW <= 0 {
		return s
	}
	adv := func(str string) float64 {
		if face != nil {
			return face.Advance(str)
		}
		return float64(utf8.RuneCountInString(str)) * fontSize * 0.5
	}
	if adv(s) <= maxW {
		return s
	}
	ell := "…"
	ellW := adv(ell)
	if ellW >= maxW {
		return ell
	}
	r := []rune(s)
	n := len(r)
	best := ell
	// Prefer keeping more of the end (common "…filename.ext" pattern).
	for keepEnd := 1; keepEnd < n; keepEnd++ {
		for keepStart := 0; keepStart < n-keepEnd; keepStart++ {
			cand := string(r[:keepStart]) + ell + string(r[n-keepEnd:])
			if adv(cand) <= maxW {
				if utf8.RuneCountInString(cand) >= utf8.RuneCountInString(best) {
					best = cand
				}
			} else {
				break
			}
		}
	}
	return best
}

func ensureTypographyIcons() {
	r := primitive.GlobalIcons
	if _, ok := r.Lookup("copy"); !ok {
		r.Register("copy", primitive.IconDef{
			Kind: primitive.IconCustom,
			Paint: func(pc *core.PaintContext, size float64, primary, secondary render.RGBA) {
				if pc == nil {
					return
				}
				// two overlapping rectangles
				s := size
				pc.StrokeLocalRoundRect(s*0.28, s*0.18, s*0.55, s*0.55, 1, 1.5, primary)
				pc.StrokeLocalRoundRect(s*0.18, s*0.32, s*0.55, s*0.55, 1, 1.5, primary)
			},
		})
	}
	if _, ok := r.Lookup("edit"); !ok {
		r.Register("edit", primitive.IconDef{
			Kind: primitive.IconCustom,
			Paint: func(pc *core.PaintContext, size float64, primary, secondary render.RGBA) {
				if pc == nil {
					return
				}
				s := size
				pc.StrokeLocalPolyline([]float64{
					s * 0.2, s * 0.72,
					s * 0.2, s * 0.55,
					s * 0.62, s * 0.18,
					s * 0.78, s * 0.34,
					s * 0.36, s * 0.72,
					s * 0.2, s * 0.72,
				}, 1.5, primary)
			},
		})
	}
}

// TypographyTestForceEllipsis marks overflow for tests so Expand action is shown.
func TypographyTestForceEllipsis(t *Typography, v bool) {
	if t == nil {
		return
	}
	t.flagEllipsis(v)
	t.rebuild()
}
