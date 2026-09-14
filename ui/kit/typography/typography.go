// Package typography implements the typography control (docs/antd/typography.md §6).
//
// Composition over new frameworks: the widget owns a rendering.RenderBox
// node (Node) and reuses ui/rendering for draw, ui/theme for color.
// No new event or frame system; hit == layout == paint on the host node.
package typography

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Geometry fallbacks when theme tokens are zero (docs/antd/typography.md §6.2).
const (
	DefaultBodyFontSize = 14.0
	TitleLevel1Size     = 38.0
	TitleLevel2Size     = 30.0
	TitleLevel3Size     = 24.0
	TitleLevel4Size     = 20.0
	TitleLevel5Size     = 16.0
	ActionIconSize      = 14.0
	ActionGap           = 4.0
	CodeRadius          = 3.0
	ContainerRadius     = 6.0
	LineWidth           = 1.0
	FocusRingOutset     = 1.5
)

// MarkBg is the gold[2] mark wash (antd v4 compat, §6.2.1 component constant).
func MarkBg() render.RGBA { return render.Hex("#ffe58f") }

// EllipsisMark is the truncation glyph.
const EllipsisMark = "…"

// Kind selects Text/Title/Paragraph/Link (§6.10).
type Kind int

const (
	KindText Kind = iota
	KindTitle
	KindParagraph
	KindLink
)

// TextType selects the semantic color (§6.3 type).
type TextType int

const (
	TypeDefault TextType = iota
	TypeSecondary
	TypeSuccess
	TypeWarning
	TypeDanger
)

// Placement selects actions start/end (§6.3 actions).
type Placement string

const (
	PlacementStart Placement = "start"
	PlacementEnd   Placement = "end"
)

// Style is an optional business paint override. Zero value disables.
type Style struct {
	Text          render.RGBA
	UseText       bool
	Bg            render.RGBA
	UseBg         bool
}

// Typography is the typography widget (docs/antd/typography.md §6.10).
//
// It owns a rendering.RenderBox host: put Node() in the tree, drive layout
// through Layout, read Effective* for assertions. Four kinds share one type;
// NewText/NewTitle/NewParagraph/NewLink are sugar.
type Typography struct {
	value    string
	kind     Kind
	level    int
	textType TextType
	disabled bool

	strong    bool
	code      bool
	mark      bool
	deleted   bool
	underline bool
	italic    bool
	keyboard  bool

	copyable  bool
	copyText  string
	copyIcon  string
	lastCopy  string
	OnCopy    func(text string)

	editable   bool
	editing    bool
	editingSet bool
	pending    string
	OnChange   func(value string)
	OnStart    func()
	OnEnd      func()
	OnCancel   func()

	ellipsis           bool
	ellipsisRows       int
	expandable         bool
	collapsible        bool
	expanded           bool
	expandedSet        bool
	internalExpanded   bool
	defaultExpanded    bool
	ellipsisMiddle     bool
	suffix             string
	expandText         string
	collapseText       string
	expandTextSet      bool
	collapseTextSet    bool
	OnExpand           func(expanded bool)
	OnEllipsis         func(ellipsis bool)
	lastEllipsized     bool
	lastEllipsisFired  bool
	lastDisplay        string
	lastLines          int

	actionsPlacement Placement

	maxWidth float64
	fontSize float64
	face     text.Face
	provider *theme.Provider
	override *theme.Tokens
	style    Style

	OnClick   func()
	ariaLabel string
	focused   bool

	node     *rendering.RenderBox
	lastSize rendering.Size
	lastMaxW float64
}

// NewTypography creates a Text node (kind=Text).
func NewTypography(value string) *Typography {
	t := &Typography{value: value, kind: KindText, level: 1, ellipsisRows: 1, actionsPlacement: PlacementEnd, expandText: "展开", collapseText: "收起"}
	t.node = rendering.NewRenderBox()
	t.node.SetRepaintBoundary(true)
	t.node.SetRelayoutBoundary(true)
	self := t
	t.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) { self.paint(pc, size) }
	// Intrinsic size up front so Node().Size() is non-zero without a
	// caller Layout (Layout/Node contract); later Sets relayout as usual.
	t.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return t
}

// NewText is sugar for NewTypography.
func NewText(value string) *Typography { return NewTypography(value) }

// NewTitle creates a Title node (level 1..5, default 1).
func NewTitle(value string, level int) *Typography {
	t := NewTypography(value)
	t.kind = KindTitle
	t.SetLevel(level)
	// Kind switch changes the size ladder; refresh the intrinsic size so
	// Node().Size() is Title-correct without waiting for a caller Layout.
	t.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return t
}

// NewParagraph creates a Paragraph node.
func NewParagraph(value string) *Typography {
	t := NewTypography(value)
	t.kind = KindParagraph
	return t
}

// NewLink creates a Link node.
func NewLink(value string) *Typography {
	t := NewTypography(value)
	t.kind = KindLink
	return t
}

// Value returns the text.
func (t *Typography) Value() string {
	if t == nil {
		return ""
	}
	return t.value
}

// SetValue changes the text (layout).
func (t *Typography) SetValue(s string) {
	if t == nil || t.value == s {
		return
	}
	t.value = s
	t.relayout()
}

// Kind returns the kind.
func (t *Typography) Kind() Kind {
	if t == nil {
		return KindText
	}
	return t.kind
}

// SetKind switches Text/Title/Paragraph/Link (layout).
func (t *Typography) SetKind(k Kind) {
	if t == nil {
		return
	}
	if k != KindText && k != KindTitle && k != KindParagraph && k != KindLink {
		k = KindText
	}
	if t.kind == k {
		return
	}
	t.kind = k
	t.relayout()
}

// Level returns the title level (1..5).
func (t *Typography) Level() int {
	if t == nil || t.level < 1 || t.level > 5 {
		return 1
	}
	return t.level
}

// SetLevel sets 1..5 (clamped, layout).
func (t *Typography) SetLevel(l int) {
	if t == nil {
		return
	}
	if l < 1 {
		l = 1
	}
	if l > 5 {
		l = 5
	}
	if t.level == l {
		return
	}
	t.level = l
	t.relayout()
}

// Type returns the semantic type.
func (t *Typography) Type() TextType {
	if t == nil {
		return TypeDefault
	}
	return t.textType
}

// SetType sets the semantic color (paint only).
func (t *Typography) SetType(v TextType) {
	if t == nil {
		return
	}
	if v < TypeDefault || v > TypeDanger {
		v = TypeDefault
	}
	if t.textType == v {
		return
	}
	t.textType = v
	t.markPaint()
}

// SetDisabled toggles disabled (paint + interaction).
func (t *Typography) SetDisabled(b bool) {
	if t == nil || t.disabled == b {
		return
	}
	t.disabled = b
	t.markPaint()
}

// Disabled reports disabled.
func (t *Typography) Disabled() bool { return t != nil && t.disabled }

// SetStrong toggles bold (paint only for headless; weight probe).
func (t *Typography) SetStrong(b bool) {
	if t == nil || t.strong == b {
		return
	}
	t.strong = b
	t.markPaint()
}

// Strong reports strong.
func (t *Typography) Strong() bool { return t != nil && t.strong }

// SetCode toggles code chrome (layout for padding, paint for wash).
func (t *Typography) SetCode(b bool) {
	if t == nil || t.code == b {
		return
	}
	t.code = b
	t.relayout()
}

// Code reports code.
func (t *Typography) Code() bool { return t != nil && t.code }

// SetMark toggles mark wash (paint only).
func (t *Typography) SetMark(b bool) {
	if t == nil || t.mark == b {
		return
	}
	t.mark = b
	t.markPaint()
}

// Mark reports mark.
func (t *Typography) Mark() bool { return t != nil && t.mark }

// SetDelete toggles strikethrough (paint only).
func (t *Typography) SetDelete(b bool) {
	if t == nil || t.deleted == b {
		return
	}
	t.deleted = b
	t.markPaint()
}

// Deleted reports delete.
func (t *Typography) Deleted() bool { return t != nil && t.deleted }

// SetUnderline toggles underline (paint only).
func (t *Typography) SetUnderline(b bool) {
	if t == nil || t.underline == b {
		return
	}
	t.underline = b
	t.markPaint()
}

// Underline reports underline.
func (t *Typography) Underline() bool { return t != nil && t.underline }

// SetItalic toggles italic (paint only).
func (t *Typography) SetItalic(b bool) {
	if t == nil || t.italic == b {
		return
	}
	t.italic = b
	t.markPaint()
}

// Italic reports italic.
func (t *Typography) Italic() bool { return t != nil && t.italic }

// SetKeyboard toggles kbd chrome (layout for padding).
func (t *Typography) SetKeyboard(b bool) {
	if t == nil || t.keyboard == b {
		return
	}
	t.keyboard = b
	t.relayout()
}

// Keyboard reports keyboard.
func (t *Typography) Keyboard() bool { return t != nil && t.keyboard }

// FontWeight returns 600 when strong else 400 (§6.5 probe).
func (t *Typography) FontWeight() float64 {
	if t != nil && t.strong {
		return 600
	}
	return 400
}

// SetCopyable toggles the copy action (layout).
func (t *Typography) SetCopyable(b bool) {
	if t == nil || t.copyable == b {
		return
	}
	t.copyable = b
	t.relayout()
}

// Copyable reports copyable.
func (t *Typography) Copyable() bool { return t != nil && t.copyable }

// SetCopyText pins the clipboard payload (empty uses Value).
func (t *Typography) SetCopyText(s string) {
	if t == nil || t.copyText == s {
		return
	}
	t.copyText = s
}

// CopyText returns the pinned payload ("" means use Value).
func (t *Typography) CopyText() string {
	if t == nil {
		return ""
	}
	return t.copyText
}

// SetCopyIcon pins the copy icon name (empty is default).
func (t *Typography) SetCopyIcon(name string) {
	if t == nil || t.copyIcon == name {
		return
	}
	t.copyIcon = name
	t.markPaint()
}

// CopyIcon returns the icon name.
func (t *Typography) CopyIcon() string {
	if t == nil {
		return ""
	}
	return t.copyIcon
}

// CopyPayload resolves copyText or Value.
func (t *Typography) CopyPayload() string {
	if t == nil {
		return ""
	}
	if t.copyText != "" {
		return t.copyText
	}
	return t.value
}

// CopiedText returns the last copied payload (host clipboard stand-in).
func (t *Typography) CopiedText() string {
	if t == nil {
		return ""
	}
	return t.lastCopy
}

// Copy simulates the copy action: writes payload once, fires OnCopy once.
// Disabled or non-copyable swallows.
func (t *Typography) Copy() bool {
	if t == nil || t.disabled || !t.copyable {
		return false
	}
	t.lastCopy = t.CopyPayload()
	if t.OnCopy != nil {
		t.OnCopy(t.lastCopy)
	}
	t.markPaint()
	return true
}

// PressCopy is an alias of Copy (pointer/keyboard entry).
func (t *Typography) PressCopy() bool { return t.Copy() }

// SetEditable toggles the edit action (layout).
func (t *Typography) SetEditable(b bool) {
	if t == nil || t.editable == b {
		return
	}
	t.editable = b
	t.relayout()
}

// Editable reports editable.
func (t *Typography) Editable() bool { return t != nil && t.editable }

// SetEditing sets controlled editing (paint).
func (t *Typography) SetEditing(b bool) {
	if t == nil {
		return
	}
	t.editing = b
	t.editingSet = true
	t.markPaint()
}

// IsEditing reports editing (controlled wins).
func (t *Typography) IsEditing() bool { return t != nil && t.editing }

// Editing is an alias of IsEditing.
func (t *Typography) Editing() bool { return t.IsEditing() }

// SetPendingEdit stages the edit buffer.
func (t *Typography) SetPendingEdit(s string) {
	if t == nil {
		return
	}
	t.pending = s
}

// PendingEdit returns the staged buffer.
func (t *Typography) PendingEdit() string {
	if t == nil {
		return ""
	}
	return t.pending
}

// StartEdit enters edit mode (Enter path setup). Disabled swallows.
func (t *Typography) StartEdit() bool {
	if t == nil || t.disabled || !t.editable || t.editing {
		return false
	}
	t.editing = true
	t.pending = t.value
	if t.OnStart != nil {
		t.OnStart()
	}
	t.markPaint()
	return true
}

// SubmitEdit commits newVal (Enter): value updates, OnChange + OnEnd fire.
func (t *Typography) SubmitEdit(newVal string) bool {
	if t == nil || t.disabled || !t.editable || !t.editing {
		return false
	}
	t.value = newVal
	t.pending = newVal
	t.editing = false
	if t.OnChange != nil {
		t.OnChange(newVal)
	}
	if t.OnEnd != nil {
		t.OnEnd()
	}
	t.relayout()
	return true
}

// CancelEdit aborts (Esc): value kept, OnCancel fires.
func (t *Typography) CancelEdit() bool {
	if t == nil || !t.editable || !t.editing {
		return false
	}
	t.pending = t.value
	t.editing = false
	if t.OnCancel != nil {
		t.OnCancel()
	}
	t.markPaint()
	return true
}

// PressKey handles Enter/Escape in edit mode (a11y keyboard path).
func (t *Typography) PressKey(key string) bool {
	if t == nil || !t.editing {
		return false
	}
	switch key {
	case "Enter", "\n":
		return t.SubmitEdit(t.pending)
	case "Escape", "Esc":
		return t.CancelEdit()
	}
	return false
}

// SetEllipsis toggles overflow ellipsis (layout).
func (t *Typography) SetEllipsis(b bool) {
	if t == nil || t.ellipsis == b {
		return
	}
	t.ellipsis = b
	t.relayout()
}

// Ellipsis reports ellipsis.
func (t *Typography) Ellipsis() bool { return t != nil && t.ellipsis }

// SetEllipsisRows sets max rows (<=1 single line, layout).
func (t *Typography) SetEllipsisRows(n int) {
	if t == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if t.ellipsisRows == n {
		return
	}
	t.ellipsisRows = n
	t.relayout()
}

// EllipsisRows returns rows (0/1 single line).
func (t *Typography) EllipsisRows() int {
	if t == nil {
		return 1
	}
	return t.ellipsisRows
}

// EffectiveRows resolves 0/1 to single line.
func (t *Typography) EffectiveRows() int {
	if t == nil || t.ellipsisRows <= 1 {
		return 1
	}
	return t.ellipsisRows
}

// SetExpandable toggles the expand affordance (layout).
func (t *Typography) SetExpandable(b bool) {
	if t == nil || t.expandable == b {
		return
	}
	t.expandable = b
	t.relayout()
}

// Expandable reports expandable.
func (t *Typography) Expandable() bool { return t != nil && t.expandable }

// SetCollapsible toggles收起 after expand (paint).
func (t *Typography) SetCollapsible(b bool) {
	if t == nil || t.collapsible == b {
		return
	}
	t.collapsible = b
	t.markPaint()
}

// Collapsible reports collapsible.
func (t *Typography) Collapsible() bool { return t != nil && t.collapsible }

// SetExpanded sets controlled expanded (layout + OnExpand on change).
func (t *Typography) SetExpanded(b bool) {
	if t == nil {
		return
	}
	old := t.IsExpanded()
	t.expanded = b
	t.expandedSet = true
	if old != b {
		if t.OnExpand != nil {
			t.OnExpand(b)
		}
		t.relayout()
		return
	}
	t.markPaint()
}

// SetDefaultExpanded sets the uncontrolled initial value.
func (t *Typography) SetDefaultExpanded(b bool) {
	if t == nil {
		return
	}
	t.defaultExpanded = b
	if !t.expandedSet {
		t.internalExpanded = b
		t.markPaint()
	}
}

// IsExpanded reports expanded (controlled wins, §6.3 priority).
func (t *Typography) IsExpanded() bool {
	if t == nil {
		return false
	}
	if t.expandedSet {
		return t.expanded
	}
	return t.internalExpanded
}

// Expanded is an alias of IsExpanded.
func (t *Typography) Expanded() bool { return t.IsExpanded() }

// ToggleExpand flips expand state (click on 展开/收起). Disabled swallows.
func (t *Typography) ToggleExpand() bool {
	if t == nil || t.disabled || !t.ellipsis || !t.expandable {
		return false
	}
	next := !t.IsExpanded()
	if t.expandedSet {
		if t.OnExpand != nil {
			t.OnExpand(next)
		}
		return true
	}
	t.internalExpanded = next
	if t.OnExpand != nil {
		t.OnExpand(next)
	}
	t.relayout()
	return true
}

// SetEllipsisMiddle toggles head+tail truncation (layout).
func (t *Typography) SetEllipsisMiddle(b bool) {
	if t == nil || t.ellipsisMiddle == b {
		return
	}
	t.ellipsisMiddle = b
	t.relayout()
}

// EllipsisMiddle reports middle mode.
func (t *Typography) EllipsisMiddle() bool { return t != nil && t.ellipsisMiddle }

// SetSuffix sets the ellipsis tail/suffix (layout).
func (t *Typography) SetSuffix(s string) {
	if t == nil || t.suffix == s {
		return
	}
	t.suffix = s
	t.relayout()
}

// Suffix returns suffix.
func (t *Typography) Suffix() string {
	if t == nil {
		return ""
	}
	return t.suffix
}

// SetExpandSymbol pins 展开/收起文案 (default 展开/收起, layout).
func (t *Typography) SetExpandSymbol(expand, collapse string) {
	if t == nil {
		return
	}
	t.expandText = expand
	t.collapseText = collapse
	t.expandTextSet = true
	t.collapseTextSet = true
	t.relayout()
}

// ExpandSymbol returns the collapsed affordance text (default 展开).
func (t *Typography) ExpandSymbol() string {
	if t == nil || t.expandText == "" {
		return "展开"
	}
	return t.expandText
}

// CollapseSymbol returns the expanded affordance text (default 收起).
func (t *Typography) CollapseSymbol() string {
	if t == nil || t.collapseText == "" {
		return "收起"
	}
	return t.collapseText
}

// SetActionsPlacement sets start/end (default end, paint).
func (t *Typography) SetActionsPlacement(p Placement) {
	if t == nil {
		return
	}
	if p != PlacementStart && p != PlacementEnd {
		p = PlacementEnd
	}
	if t.actionsPlacement == p {
		return
	}
	t.actionsPlacement = p
	t.markPaint()
}

// ActionsPlacement returns start/end.
func (t *Typography) ActionsPlacement() Placement {
	if t == nil || t.actionsPlacement == "" {
		return PlacementEnd
	}
	return t.actionsPlacement
}

// SetMaxWidth caps the content width (layout; 0 clears).
func (t *Typography) SetMaxWidth(w float64) {
	if t == nil {
		return
	}
	if w < 0 {
		w = 0
	}
	if t.maxWidth == w {
		return
	}
	t.maxWidth = w
	t.relayout()
}

// MaxWidth returns the cap (0 none).
func (t *Typography) MaxWidth() float64 {
	if t == nil {
		return 0
	}
	return t.maxWidth
}

// SetFontSize pins the size (0 selects kind default, layout).
func (t *Typography) SetFontSize(s float64) {
	if t == nil {
		return
	}
	if s < 0 {
		s = 0
	}
	if t.fontSize == s {
		return
	}
	t.fontSize = s
	t.relayout()
}

// FontSize returns the pinned size (0 default).
func (t *Typography) FontSize() float64 {
	if t == nil {
		return 0
	}
	return t.fontSize
}

// SetFace sets the text face (paint only; heuristic layout keeps size).
func (t *Typography) SetFace(f text.Face) {
	if t == nil {
		return
	}
	t.face = f
	t.markPaint()
}

// SetProvider selects the theme source (paint).
func (t *Typography) SetProvider(p *theme.Provider) {
	if t == nil {
		return
	}
	t.provider = p
	t.markPaint()
}

// SetTheme pins exact tokens (nil clears to provider, paint).
func (t *Typography) SetTheme(tok *theme.Tokens) {
	if t == nil {
		return
	}
	t.override = tok
	t.markPaint()
}

// Theme returns the effective tokens.
func (t *Typography) Theme() theme.Tokens { return t.themeTokens() }

// SetStyle installs the business override (paint).
func (t *Typography) SetStyle(s Style) {
	if t == nil {
		return
	}
	t.style = s
	t.markPaint()
}

// Style returns the override.
func (t *Typography) Style() Style {
	if t == nil {
		return Style{}
	}
	return t.style
}

// SetOnClick sets the click handler.
func (t *Typography) SetOnClick(fn func()) {
	if t == nil {
		return
	}
	t.OnClick = fn
}

// Click simulates activation. Disabled swallows.
func (t *Typography) Click() bool {
	if t == nil || t.disabled {
		return false
	}
	if t.OnClick != nil {
		t.OnClick()
	}
	return true
}

// SetAriaLabel pins the accessible name.
func (t *Typography) SetAriaLabel(s string) {
	if t == nil || t.ariaLabel == s {
		return
	}
	t.ariaLabel = s
	t.markPaint()
}

// AriaLabel returns explicit name else value (text is name).
func (t *Typography) AriaLabel() string {
	if t == nil {
		return ""
	}
	if t.ariaLabel != "" {
		return t.ariaLabel
	}
	return t.value
}

// Role returns link for Link, button for action rows, "" for plain text.
func (t *Typography) Role() string {
	if t == nil {
		return ""
	}
	if t.kind == KindLink {
		return "link"
	}
	if t.HasActions() {
		return "button"
	}
	return ""
}

// Focusable is Link or any action when enabled.
func (t *Typography) Focusable() bool {
	if t == nil || t.disabled {
		return false
	}
	if t.kind == KindLink {
		return true
	}
	return t.HasActions()
}

// Focus sets keyboard focus (paint only).
func (t *Typography) Focus() {
	if t == nil || !t.Focusable() {
		return
	}
	t.focused = true
	t.markPaint()
}

// Blur clears focus (paint only).
func (t *Typography) Blur() {
	if t == nil || !t.focused {
		return
	}
	t.focused = false
	t.markPaint()
}

// Focused reports focus.
func (t *Typography) Focused() bool { return t != nil && t.focused && t.Focusable() }

// FocusRingVisible requires focusable + focused (must paint ring).
func (t *Typography) FocusRingVisible() bool { return t.Focused() }

// HasActions reports copy/edit/expand affordances.
func (t *Typography) HasActions() bool {
	if t == nil {
		return false
	}
	if t.copyable || t.editable {
		return true
	}
	return t.ellipsis && t.expandable
}

// ActionNames returns accessible names for copy/edit/expand (中文默认).
func (t *Typography) ActionNames() []string {
	if t == nil {
		return nil
	}
	var out []string
	if t.copyable {
		out = append(out, "复制")
	}
	if t.editable {
		out = append(out, "编辑")
	}
	if t.ellipsis && t.expandable {
		if t.IsExpanded() {
			out = append(out, t.CollapseSymbol())
		} else {
			out = append(out, t.ExpandSymbol())
		}
	}
	return out
}

func (t *Typography) themeTokens() theme.Tokens {
	if t != nil && t.override != nil {
		return *t.override
	}
	if t != nil && t.provider != nil {
		return t.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// EffectiveFontSize resolves Title ladder else body 14 (§6.2).
func (t *Typography) EffectiveFontSize() float64 {
	if t != nil && t.fontSize > 0 {
		return t.fontSize
	}
	if t != nil && t.kind == KindTitle {
		switch t.Level() {
		case 1:
			return TitleLevel1Size
		case 2:
			return TitleLevel2Size
		case 3:
			return TitleLevel3Size
		case 4:
			return TitleLevel4Size
		case 5:
			return TitleLevel5Size
		}
		return TitleLevel1Size
	}
	tok := t.themeTokens()
	if tok.FontSize > 0 {
		return tok.FontSize
	}
	return DefaultBodyFontSize
}

// EffectiveLineHeight returns fontSize*lineHeight (§6.2 FontHeight).
func (t *Typography) EffectiveLineHeight() float64 {
	fs := t.EffectiveFontSize()
	tok := t.themeTokens()
	lh := tok.LineHeight
	if lh <= 0 {
		lh = 1.5714285714285714
	}
	return fs * lh
}

// EffectiveColor resolves type/disabled/link through Theme (§6.2.2).
func (t *Typography) EffectiveColor() render.RGBA {
	tok := t.themeTokens()
	if t != nil && t.style.UseText {
		return t.style.Text
	}
	if t != nil && t.disabled {
		return themeToRGBA(tok.ColorTextDisabled)
	}
	if t != nil && t.kind == KindLink && t.textType == TypeDefault {
		return themeToRGBA(tok.ColorLink)
	}
	if t == nil {
		return themeToRGBA(tok.ColorText)
	}
	switch t.textType {
	case TypeSecondary:
		return themeToRGBA(tok.ColorTextSecondary)
	case TypeSuccess:
		return themeToRGBA(tok.ColorSuccess)
	case TypeWarning:
		return themeToRGBA(tok.ColorWarning)
	case TypeDanger:
		return themeToRGBA(tok.ColorError)
	default:
		if t.kind == KindTitle {
			return themeToRGBA(tok.ColorTextHeading)
		}
		return themeToRGBA(tok.ColorText)
	}
}

// CodeBg resolves the code wash (theme fill, no brand).
func (t *Typography) CodeBg() render.RGBA {
	tok := t.themeTokens()
	if c := themeToRGBA(tok.ColorFillSecondary); c.A > 0 {
		return c
	}
	return render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
}

// TextWidth estimates the advance of s (ascii 0.6em, wide 1em).
func (t *Typography) TextWidth(s string) float64 {
	fs := t.EffectiveFontSize()
	var w float64
	for _, r := range s {
		if r < 128 {
			w += 0.6 * fs
		} else {
			w += 1.0 * fs
		}
	}
	return w
}

// ChromePadW is the code/kbd horizontal chrome.
func (t *Typography) ChromePadW() float64 {
	if t != nil && (t.code || t.keyboard) {
		return 8
	}
	return 0
}

// ActionCount counts copy/edit/expand slots.
func (t *Typography) ActionCount() int {
	if t == nil {
		return 0
	}
	n := 0
	if t.copyable {
		n++
	}
	if t.editable {
		n++
	}
	if t.ellipsis && t.expandable {
		n++
	}
	return n
}

// ActionWidth reserves icon 14 + gap 4 per action.
func (t *Typography) ActionWidth() float64 {
	return float64(t.ActionCount()) * (ActionIconSize + ActionGap)
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func (t *Typography) effectiveMaxForDisplay() float64 {
	if t == nil {
		return rendering.Unbounded
	}
	if t.maxWidth > 0 {
		return t.maxWidth
	}
	if t.lastMaxW > 0 {
		return t.lastMaxW
	}
	return rendering.Unbounded
}

func (t *Typography) availFor(maxW float64) float64 {
	avail := maxW - t.ChromePadW() - t.ActionWidth()
	if avail < 0 {
		avail = 0
	}
	return avail
}

// IsEllipsized reports whether value overflows rows at the display width.
func (t *Typography) IsEllipsized() bool {
	if t == nil || !t.ellipsis {
		return false
	}
	maxW := t.effectiveMaxForDisplay()
	if maxW <= 0 || maxW >= rendering.Unbounded/2 {
		return false
	}
	avail := t.availFor(maxW)
	rows := t.EffectiveRows()
	fullW := t.TextWidth(t.value)
	return fullW > avail*float64(rows)+0.5
}

// DisplayText returns the painted string (binary-cut ellipsis, §6.4).
func (t *Typography) DisplayText() string {
	if t == nil {
		return ""
	}
	if !t.ellipsis {
		return t.value
	}
	if t.IsExpanded() {
		return t.value
	}
	maxW := t.effectiveMaxForDisplay()
	if maxW <= 0 || maxW >= rendering.Unbounded/2 {
		return t.value
	}
	avail := t.availFor(maxW)
	rows := t.EffectiveRows()
	fullW := t.TextWidth(t.value)
	if fullW <= avail*float64(rows)+0.5 {
		return t.value
	}
	if t.ellipsisMiddle {
		return t.truncateMiddle(avail, rows)
	}
	return t.truncateEnd(avail, rows)
}

func (t *Typography) expandPart() string {
	if t == nil || !t.expandable || t.IsExpanded() {
		return ""
	}
	return " " + t.ExpandSymbol()
}

func (t *Typography) truncateEnd(avail float64, rows int) string {
	runes := []rune(t.value)
	capW := avail * float64(rows)
	lo, hi := 0, len(runes)
	widthOf := func(n int) float64 {
		cand := string(runes[:n]) + EllipsisMark + t.suffix + t.expandPart()
		return t.TextWidth(cand)
	}
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if widthOf(mid) <= capW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo]) + EllipsisMark + t.suffix + t.expandPart()
}

func (t *Typography) truncateMiddle(avail float64, rows int) string {
	runes := []rune(t.value)
	capW := avail * float64(rows)
	var tail string
	if t.suffix != "" {
		tail = t.suffix
	} else if len(runes) > 4 {
		tail = string(runes[len(runes)-4:])
	} else {
		tail = string(runes)
	}
	overhead := t.TextWidth(EllipsisMark+tail) + t.TextWidth(t.expandPart())
	budget := capW - overhead
	if budget < 0 {
		budget = 0
	}
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if t.TextWidth(string(runes[:mid])) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	head := string(runes[:lo])
	// Fixed tail wins over head on overlap: clamp head so head+tail fits.
	for lo > 0 && t.TextWidth(head+EllipsisMark+tail+t.expandPart()) > capW {
		lo--
		head = string(runes[:lo])
	}
	return head + EllipsisMark + tail + t.expandPart()
}

// LineCount returns painted rows (rows cap when collapsed).
func (t *Typography) LineCount() int {
	if t == nil {
		return 1
	}
	maxW := t.effectiveMaxForDisplay()
	if maxW <= 0 || maxW >= rendering.Unbounded/2 {
		return 1
	}
	avail := t.availFor(maxW)
	if avail <= 0 {
		return t.EffectiveRows()
	}
	fullW := t.TextWidth(t.DisplayText())
	if fullW <= 0 {
		return 1
	}
	lines := int(math.Ceil(fullW / avail))
	if lines < 1 {
		lines = 1
	}
	if t.ellipsis && !t.IsExpanded() && lines > t.EffectiveRows() {
		lines = t.EffectiveRows()
	}
	return lines
}

// PreferredSize is the content size before constraints.
func (t *Typography) PreferredSize(maxW float64) rendering.Size {
	if t == nil {
		return rendering.Size{}
	}
	fs := t.EffectiveFontSize()
	lineH := t.EffectiveLineHeight()
	fullW := t.TextWidth(t.value)
	prefW := fullW + t.ChromePadW() + t.ActionWidth()
	if maxW <= 0 || maxW >= rendering.Unbounded/2 {
		return rendering.Size{Width: prefW, Height: lineH}
	}
	if prefW <= maxW+0.5 {
		return rendering.Size{Width: prefW, Height: lineH}
	}
	avail := t.availFor(maxW)
	if avail <= 0 {
		avail = maxW
	}
	linesNeeded := int(math.Ceil(prefW / maxW))
	if linesNeeded < 1 {
		linesNeeded = 1
	}
	_ = fs
	if t.ellipsis && !t.IsExpanded() {
		rows := t.EffectiveRows()
		lines := linesNeeded
		if lines > rows {
			lines = rows
		}
		return rendering.Size{Width: maxW, Height: lineH * float64(lines)}
	}
	return rendering.Size{Width: maxW, Height: lineH * float64(linesNeeded)}
}

// Node returns the tree node (layout/paint/hit through it).
func (t *Typography) Node() rendering.RenderObject {
	if t == nil {
		return nil
	}
	return t.node
}

// ContentNode returns the text content node (same host, single box).
func (t *Typography) ContentNode() rendering.RenderObject { return t.Node() }

// ChromeNode returns the code/mark/kbd shell (same host, single box).
func (t *Typography) ChromeNode() rendering.RenderObject { return t.Node() }

// LaidOut returns the last laid-out size.
func (t *Typography) LaidOut() rendering.Size {
	if t == nil {
		return rendering.Size{}
	}
	return t.lastSize
}

// Layout sizes the host under constraints (Exact wins via Tighten).
func (t *Typography) Layout(c rendering.Constraints) rendering.Size {
	if t == nil || t.node == nil {
		return rendering.Size{}
	}
	effMax := c.MaxWidth
	if t.maxWidth > 0 && t.maxWidth < effMax {
		effMax = t.maxWidth
	}
	t.lastMaxW = effMax
	pref := t.PreferredSize(effMax)
	out := c.Tighten(pref)
	t.lastSize = out
	t.node.FixedWidth = out.Width
	t.node.FixedHeight = out.Height
	sz := t.node.Layout(c)
	t.lastSize = sz
	t.lastDisplay = t.DisplayText()
	ell := t.IsEllipsized()
	if ell != t.lastEllipsized || !t.lastEllipsisFired {
		t.lastEllipsized = ell
		t.lastEllipsisFired = true
		if t.OnEllipsis != nil {
			t.OnEllipsis(ell)
		}
	}
	lines := t.LineCount()
	t.lastLines = lines
	return sz
}

// LastDisplay returns the last laid-out string (tests may use DisplayText).
func (t *Typography) LastDisplay() string {
	if t == nil {
		return ""
	}
	if t.lastDisplay != "" {
		return t.lastDisplay
	}
	return t.DisplayText()
}

// LastLines returns the last laid-out row count.
func (t *Typography) LastLines() int {
	if t == nil {
		return 1
	}
	if t.lastLines > 0 {
		return t.lastLines
	}
	return t.LineCount()
}

func (t *Typography) relayout() {
	if t == nil || t.node == nil {
		return
	}
	t.node.MarkNeedsLayout()
}

func (t *Typography) markPaint() {
	if t == nil || t.node == nil {
		return
	}
	t.node.MarkNeedsPaint()
}

func (t *Typography) paint(pc *rendering.PaintContext, size rendering.Size) {
	if t == nil || pc == nil || pc.DC == nil {
		return
	}
	w, h := size.Width, size.Height
	if w <= 0 || h <= 0 {
		return
	}
	tok := t.themeTokens()
	if t.style.UseBg {
		rendering.FillRect(pc, 0, 0, w, h, t.style.Bg.R, t.style.Bg.G, t.style.Bg.B, t.style.Bg.A)
	} else if t.mark {
		mb := MarkBg()
		rendering.FillRect(pc, 0, 0, w, h, mb.R, mb.G, mb.B, mb.A)
	}
	if t.code {
		bg := t.CodeBg()
		rendering.FillRoundRect(pc, 0, 0, w, h, CodeRadius, bg.R, bg.G, bg.B, bg.A)
	}
	if t.keyboard {
		bg := t.CodeBg()
		rendering.FillRoundRect(pc, 0, 0, w, h, CodeRadius, bg.R, bg.G, bg.B, bg.A)
		lc := themeToRGBA(tok.ColorBorder)
		rendering.StrokeRoundRect(pc, 0, 0, w, h, CodeRadius, LineWidth, lc.R, lc.G, lc.B, lc.A)
	}
	col := t.EffectiveColor()
	disp := t.DisplayText()
	if disp == "" {
		disp = t.value
	}
	fs := t.EffectiveFontSize()
	if t.face != nil {
		pc.DC.SetFont(t.face)
	}
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
	x := 0.0
	if t.code || t.keyboard {
		x = 4
	}
	if t.actionsPlacement == PlacementStart {
		x += t.ActionWidth()
	}
	baseline := h/2 + fs*0.35
	if baseline < fs*0.8 {
		baseline = fs * 0.8
	}
	ax, ay := pc.Abs(x, baseline)
	// Keep the draw call behind a non-empty check only; without a loaded
	// face DrawString is a safe no-op and the chrome above carries pixels.
	if strings.TrimSpace(disp) != "" || disp != "" {
		pc.DC.DrawString(disp, ax, ay)
	}
	tw := t.TextWidth(disp)
	if t.underline {
		y := baseline + 2
		rendering.StrokeLine(pc, x, y, x+tw, y, 1, col.R, col.G, col.B, col.A)
	}
	if t.deleted {
		y := baseline - fs*0.25
		rendering.StrokeLine(pc, x, y, x+tw, y, 1, col.R, col.G, col.B, col.A)
	}
	if t.FocusRingVisible() {
		rc := themeToRGBA(tok.ColorPrimary)
		rendering.StrokeRoundRect(pc, -FocusRingOutset, -FocusRingOutset, w+2*FocusRingOutset, h+2*FocusRingOutset, ContainerRadius, 2, rc.R, rc.G, rc.B, rc.A)
	}
	// Action slots: small squares so copy/edit/expand presence is paint-visible.
	n := t.ActionCount()
	if n > 0 {
		ax0 := w - float64(n)*(ActionIconSize+ActionGap) + ActionGap
		if t.actionsPlacement == PlacementStart {
			ax0 = 0
		}
		ay0 := h/2 - ActionIconSize/2
		ac := themeToRGBA(tok.ColorTextSecondary)
		for i := 0; i < n; i++ {
			rendering.FillRect(pc, ax0+float64(i)*(ActionIconSize+ActionGap), ay0, 6, 6, ac.R, ac.G, ac.B, ac.A)
		}
	}
}
