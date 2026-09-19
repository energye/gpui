package float_button

import (
	"strconv"
	"strings"
	"sync"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// P1 extras (docs/antd/float-button.md §6.8): badge overlay, href/target/
// htmlType desktop mapping, semantic classNames/styles hooks, tooltip full
// props staging, draggable offsets, ConfigProvider-style global defaults,
// and FloatButton.BackTop. Everything lives in this package so shared
// packages stay untouched.

// DefaultBadgeOverflowCount caps the badge count text (antd overflowCount).
const DefaultBadgeOverflowCount = 99

// DefaultBackTopVisibilityHeight shows BackTop at scrollY >= 400 (§6.10).
const DefaultBackTopVisibilityHeight = 400.0

// DefaultBackTopDurationMs is the BackTop return-to-top time (§6.10).
const DefaultBackTopDurationMs = 450.0

// SetBadgeCount shows a count overlay (P1 badge; hides until >0 by default).
func (b *FloatButton) SetBadgeCount(n int) {
	if b == nil {
		return
	}
	if b.hasBadge && b.badgeCount == n {
		return
	}
	b.hasBadge = true
	b.badgeCount = n
	b.dirty()
}

// BadgeCount returns the badge count setting.
func (b *FloatButton) BadgeCount() int {
	if b == nil {
		return 0
	}
	return b.badgeCount
}

// HasBadgeCount reports whether a count overlay is configured.
func (b *FloatButton) HasBadgeCount() bool { return b != nil && b.hasBadge }

// SetBadgeDot toggles the dot overlay (P1 badge).
func (b *FloatButton) SetBadgeDot(v bool) {
	if b == nil || b.badgeDot == v {
		return
	}
	b.badgeDot = v
	b.dirty()
}

// BadgeDot reports the dot overlay flag.
func (b *FloatButton) BadgeDot() bool { return b != nil && b.badgeDot }

// SetBadgeOverflowCount caps the count text (<=0 falls back to 99).
func (b *FloatButton) SetBadgeOverflowCount(n int) {
	if b == nil {
		return
	}
	b.badgeOverflow = n
	b.dirty()
}

// BadgeOverflowCount returns the effective overflow cap.
func (b *FloatButton) BadgeOverflowCount() int {
	if b == nil || b.badgeOverflow <= 0 {
		return DefaultBadgeOverflowCount
	}
	return b.badgeOverflow
}

// SetBadgeShowZero shows the count overlay even when count == 0.
func (b *FloatButton) SetBadgeShowZero(v bool) {
	if b == nil || b.badgeShowZero == v {
		return
	}
	b.badgeShowZero = v
	b.dirty()
}

// BadgeShowZero reports the show-zero switch.
func (b *FloatButton) BadgeShowZero() bool { return b != nil && b.badgeShowZero }

// ClearBadge removes count and dot overlays.
func (b *FloatButton) ClearBadge() {
	if b == nil || (!b.hasBadge && !b.badgeDot) {
		return
	}
	b.hasBadge = false
	b.badgeCount = 0
	b.badgeDot = false
	b.dirty()
}

// BadgeVisible reports whether the overlay paints (dot or positive count).
func (b *FloatButton) BadgeVisible() bool {
	if b == nil {
		return false
	}
	if b.badgeDot {
		return true
	}
	if !b.hasBadge {
		return false
	}
	return b.badgeCount != 0 || b.badgeShowZero
}

// BadgeText is the painted count label ("" for dot/hidden; "99+" on overflow).
func (b *FloatButton) BadgeText() string {
	if b == nil || b.badgeDot || !b.hasBadge {
		return ""
	}
	n := b.badgeCount
	if n == 0 && !b.badgeShowZero {
		return ""
	}
	if n < 0 {
		n = 0
	}
	if ov := b.BadgeOverflowCount(); n > ov {
		return strconv.Itoa(ov) + "+"
	}
	return strconv.Itoa(n)
}

// BadgeBackground is the overlay fill (error token, §6.5 badge red).
func (b *FloatButton) BadgeBackground() render.RGBA {
	return themeToRGBA(b.themeTokens().ColorError)
}

// paintBadge draws the count/dot overlay at the top-right corner. Count text
// needs textFace; without a face the red shell still paints and no black bar
// is ever drawn (DrawString no-ops on a nil face). Reads only the frozen
// snapshot (R2-6) — never live badge fields on raster.
func (b *FloatButton) paintBadge(pc *rendering.PaintContext, size rendering.Size, S FloatSnap) {
	if b == nil || !S.BadgeVisible {
		return
	}
	bg := S.BadgeBg
	if S.BadgeDot {
		rendering.FillCircle(pc, size.Width-5, 5, 5, bg.R, bg.G, bg.B, bg.A)
		return
	}
	label := S.BadgeLabel
	if label == "" {
		return
	}
	cx, cy := size.Width-7, 7.0
	bw, bh := 16.0, 16.0
	if S.TextFace != nil {
		if w, h := text.Measure(label, S.TextFace); w > 0 && h > 0 {
			bw = w + 8
			if bw < 16 {
				bw = 16
			}
			bh = h*0.75 + 6
			if bh < 14 {
				bh = 14
			}
			if bh > 18 {
				bh = 18
			}
		}
	}
	rendering.FillRoundRect(pc, cx-bw/2, cy-bh/2, bw, bh, bh/2, bg.R, bg.G, bg.B, bg.A)
	if S.TextFace == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(cx, cy)
	pc.DC.SetFont(S.TextFace)
	pc.DC.SetRGBA(1, 1, 1, 1)
	pc.DC.DrawStringAnchored(label, ax, ay, 0.5, 0.5)
}

// FloatHtmlType maps the native button type (desktop: host Form interprets).
type FloatHtmlType string

const (
	FloatHtmlButton FloatHtmlType = "button"
	FloatHtmlSubmit FloatHtmlType = "submit"
	FloatHtmlReset  FloatHtmlType = "reset"
)

// SetHref sets the P1 link address (desktop: OnNavigate, not <a> nav).
func (b *FloatButton) SetHref(s string) {
	if b == nil || b.href == s {
		return
	}
	b.href = s
	b.dirty()
}

// Href returns the link address ("" = plain button).
func (b *FloatButton) Href() string {
	if b == nil {
		return ""
	}
	return b.href
}

// IsLink reports href presence (target only effective then, P1).
func (b *FloatButton) IsLink() bool { return b != nil && b.href != "" }

// SetTarget sets the P1 href target (effective only when Href != "").
func (b *FloatButton) SetTarget(s string) {
	if b == nil {
		return
	}
	s = strings.TrimSpace(s)
	if b.target == s {
		return
	}
	b.target = s
	b.dirty()
}

// Target returns the href target.
func (b *FloatButton) Target() string {
	if b == nil {
		return ""
	}
	return b.target
}

// SetHtmlType sets submit/reset/button (default button, P1 Form hook).
func (b *FloatButton) SetHtmlType(t FloatHtmlType) {
	if b == nil {
		return
	}
	switch t {
	case FloatHtmlSubmit, FloatHtmlReset, FloatHtmlButton:
	default:
		t = FloatHtmlButton
	}
	if b.htmlType == t {
		return
	}
	b.htmlType = t
	b.dirty()
}

// HtmlTypeName returns the native type (default button).
func (b *FloatButton) HtmlTypeName() FloatHtmlType {
	if b == nil || b.htmlType == "" {
		return FloatHtmlButton
	}
	return b.htmlType
}

// SetOnNavigate sets the P1 href callback (fired once per Click after OnClick).
func (b *FloatButton) SetOnNavigate(fn func(href, target string)) {
	if b == nil {
		return
	}
	b.onNavigate = fn
}

// FloatSemanticKey names one style hook (P1 classNames/styles object form).
type FloatSemanticKey string

const (
	FloatSemanticRoot    FloatSemanticKey = "root"
	FloatSemanticIcon    FloatSemanticKey = "icon"
	FloatSemanticContent FloatSemanticKey = "content"
	FloatSemanticBadge   FloatSemanticKey = "badge"
)

// FloatSemanticNodes lists every semantic hook (testdata mirrors this list).
var FloatSemanticNodes = []FloatSemanticKey{
	FloatSemanticRoot,
	FloatSemanticIcon,
	FloatSemanticContent,
	FloatSemanticBadge,
}

// FloatSemanticStyle is one semantic paint override (P1 styles object form).
type FloatSemanticStyle struct {
	Bg      render.RGBA
	UseBg   bool
	Text    render.RGBA
	UseText bool
}

// SetClassName pins one semantic class hook (P1 classNames; paint-neutral).
func (b *FloatButton) SetClassName(k FloatSemanticKey, v string) {
	if b == nil {
		return
	}
	if b.classNames == nil {
		b.classNames = map[FloatSemanticKey]string{}
	}
	b.classNames[k] = v
}

// ClassName reads one semantic class hook.
func (b *FloatButton) ClassName(k FloatSemanticKey) string {
	if b == nil || b.classNames == nil {
		return ""
	}
	return b.classNames[k]
}

// SetClassNames replaces all semantic class hooks (nil clears).
func (b *FloatButton) SetClassNames(m map[FloatSemanticKey]string) {
	if b == nil {
		return
	}
	if m == nil {
		b.classNames = nil
		return
	}
	b.classNames = map[FloatSemanticKey]string{}
	for k, v := range m {
		b.classNames[k] = v
	}
}

// SetSemanticStyle installs one semantic paint override (P1 styles).
func (b *FloatButton) SetSemanticStyle(k FloatSemanticKey, s FloatSemanticStyle) {
	if b == nil {
		return
	}
	if b.semStyles == nil {
		b.semStyles = map[FloatSemanticKey]FloatSemanticStyle{}
	}
	b.semStyles[k] = s
	b.dirty()
}

// SemanticStyle reads one semantic paint override.
func (b *FloatButton) SemanticStyle(k FloatSemanticKey) (FloatSemanticStyle, bool) {
	return b.semStyle(k)
}

func (b *FloatButton) semStyle(k FloatSemanticKey) (FloatSemanticStyle, bool) {
	if b == nil || b.semStyles == nil {
		return FloatSemanticStyle{}, false
	}
	s, ok := b.semStyles[k]
	return s, ok
}

// FloatTooltipPlacement stages full TooltipProps positioning (P1); the P0
// string bubble shows on hover regardless of placement.
type FloatTooltipPlacement string

const (
	FloatTooltipTop    FloatTooltipPlacement = "top"
	FloatTooltipLeft   FloatTooltipPlacement = "left"
	FloatTooltipRight  FloatTooltipPlacement = "right"
	FloatTooltipBottom FloatTooltipPlacement = "bottom"
)

// SetTooltipPlacement stages the bubble direction (invalid falls back to top).
func (b *FloatButton) SetTooltipPlacement(p FloatTooltipPlacement) {
	if b == nil {
		return
	}
	switch p {
	case FloatTooltipLeft, FloatTooltipRight, FloatTooltipBottom:
	default:
		p = FloatTooltipTop
	}
	if b.tipPlacement == p {
		return
	}
	b.tipPlacement = p
	b.dirty()
}

// TooltipPlacement returns the staged bubble direction (default top).
func (b *FloatButton) TooltipPlacement() FloatTooltipPlacement {
	if b == nil {
		return FloatTooltipTop
	}
	switch b.tipPlacement {
	case FloatTooltipLeft, FloatTooltipRight, FloatTooltipBottom:
		return b.tipPlacement
	default:
		return FloatTooltipTop
	}
}

// SetTooltipDelayMs stages the bubble delay (negative clamps to 0).
func (b *FloatButton) SetTooltipDelayMs(ms int) {
	if b == nil {
		return
	}
	if ms < 0 {
		ms = 0
	}
	b.tipDelayMs = ms
}

// TooltipDelayMs returns the staged bubble delay.
func (b *FloatButton) TooltipDelayMs() int {
	if b == nil {
		return 0
	}
	return b.tipDelayMs
}

// SetDraggable toggles host-mapped dragging (P1 draggable staging).
func (b *FloatButton) SetDraggable(v bool) {
	if b == nil || b.draggable == v {
		return
	}
	b.draggable = v
	b.dirty()
}

// Draggable reports the drag switch.
func (b *FloatButton) Draggable() bool { return b != nil && b.draggable }

// DragBy moves a draggable button (false when off/disabled; group layout
// owns member offsets, so dragging targets standalone buttons).
func (b *FloatButton) DragBy(dx, dy float64) bool {
	if b == nil || !b.draggable || b.disabled {
		return false
	}
	b.dragX += dx
	b.dragY += dy
	if b.node != nil {
		b.node.SetOffset(rendering.Point{X: b.dragX, Y: b.dragY})
	}
	b.dirty()
	return true
}

// DragOffset returns the accumulated drag position.
func (b *FloatButton) DragOffset() (x, y float64) {
	if b == nil {
		return 0, 0
	}
	return b.dragX, b.dragY
}

// ClearDrag resets the drag position.
func (b *FloatButton) ClearDrag() {
	if b == nil || (b.dragX == 0 && b.dragY == 0) {
		return
	}
	b.dragX, b.dragY = 0, 0
	if b.node != nil {
		b.node.SetOffset(rendering.Point{})
	}
	b.dirty()
}

// FloatGlobalConfig mirrors ConfigProvider float-button defaults (P1,
// button-owned staging so shared packages stay untouched).
type FloatGlobalConfig struct {
	Type        ButtonType
	HasType     bool
	Shape       FloatButtonShape
	HasShape    bool
	Disabled    bool
	HasDisabled bool
	Loading     bool
	HasLoading  bool
	Icon        string
	HasIcon     bool
	HtmlType    FloatHtmlType
	HasHtmlType bool
}

var floatGlobalMu sync.RWMutex
var floatGlobal FloatGlobalConfig

// SetFloatGlobalConfig installs ConfigProvider-style defaults (P1).
func SetFloatGlobalConfig(c FloatGlobalConfig) {
	floatGlobalMu.Lock()
	floatGlobal = c
	floatGlobalMu.Unlock()
}

// GetFloatGlobalConfig returns the global defaults snapshot.
func GetFloatGlobalConfig() FloatGlobalConfig {
	floatGlobalMu.RLock()
	defer floatGlobalMu.RUnlock()
	return floatGlobal
}

// ResetFloatGlobalConfig clears global defaults to NewFloatButton baselines.
func ResetFloatGlobalConfig() {
	floatGlobalMu.Lock()
	floatGlobal = FloatGlobalConfig{}
	floatGlobalMu.Unlock()
}

func (b *FloatButton) applyGlobalDefaults() {
	if b == nil {
		return
	}
	floatGlobalMu.RLock()
	g := floatGlobal
	floatGlobalMu.RUnlock()
	if g.HasType {
		if g.Type != ButtonTypePrimary {
			b.typ = ButtonTypeDefault
		} else {
			b.typ = ButtonTypePrimary
		}
	}
	if g.HasShape {
		if g.Shape != FloatButtonShapeSquare {
			b.shape = FloatButtonShapeCircle
		} else {
			b.shape = FloatButtonShapeSquare
		}
	}
	if g.HasDisabled {
		b.disabled = g.Disabled
	}
	if g.HasLoading {
		b.loading = g.Loading
	}
	if g.HasIcon {
		b.icon = g.Icon
	}
	if g.HasHtmlType {
		b.htmlType = g.HtmlType
	}
}

// FloatBackTop is the return-to-top button (docs/antd/float-button.md §6.10,
// P1): a single button plus host scroll observation. The host feeds scroll
// positions via SetScrollY; Click fires onClick and returns to top
// (durationMs guides the host animation, applied by the host).
type FloatBackTop struct {
	btn              *FloatButton
	visibilityHeight float64
	durationMs       float64
	scrollY          float64
	onClick          func()
}

// NewFloatBackTop creates a BackTop (icon up, hidden until scrollY >= 400).
func NewFloatBackTop() *FloatBackTop {
	t := &FloatBackTop{
		btn:              NewFloatButton(),
		visibilityHeight: DefaultBackTopVisibilityHeight,
		durationMs:       DefaultBackTopDurationMs,
	}
	t.btn.SetIcon("up")
	return t
}

// Button returns the styled button (type/shape/disabled/loading live here).
func (t *FloatBackTop) Button() *FloatButton {
	if t == nil {
		return nil
	}
	return t.btn
}

// SetVisibilityHeight sets the show threshold (<=0 falls back to 400).
func (t *FloatBackTop) SetVisibilityHeight(v float64) {
	if t == nil {
		return
	}
	if v <= 0 {
		v = DefaultBackTopVisibilityHeight
	}
	t.visibilityHeight = v
}

// VisibilityHeight returns the show threshold.
func (t *FloatBackTop) VisibilityHeight() float64 {
	if t == nil || t.visibilityHeight <= 0 {
		return DefaultBackTopVisibilityHeight
	}
	return t.visibilityHeight
}

// SetDurationMs sets the return-to-top time (<=0 falls back to 450).
func (t *FloatBackTop) SetDurationMs(v float64) {
	if t == nil {
		return
	}
	if v <= 0 {
		v = DefaultBackTopDurationMs
	}
	t.durationMs = v
}

// DurationMs returns the return-to-top time.
func (t *FloatBackTop) DurationMs() float64 {
	if t == nil || t.durationMs <= 0 {
		return DefaultBackTopDurationMs
	}
	return t.durationMs
}

// SetScrollY feeds the host scroll position (negative clamps to 0).
func (t *FloatBackTop) SetScrollY(y float64) {
	if t == nil {
		return
	}
	if y < 0 {
		y = 0
	}
	t.scrollY = y
}

// ScrollY returns the last fed scroll position.
func (t *FloatBackTop) ScrollY() float64 {
	if t == nil {
		return 0
	}
	return t.scrollY
}

// Visible reports whether BackTop shows (scrollY >= visibilityHeight).
func (t *FloatBackTop) Visible() bool {
	if t == nil {
		return false
	}
	return t.scrollY >= t.VisibilityHeight()
}

// SetOnClick sets the BackTop click callback.
func (t *FloatBackTop) SetOnClick(fn func()) {
	if t == nil {
		return
	}
	t.onClick = fn
}

// Click activates BackTop (false when hidden/disabled/loading); on success
// it fires onClick and returns scrollY to top.
func (t *FloatBackTop) Click() bool {
	if t == nil || !t.Visible() {
		return false
	}
	if t.btn == nil || t.btn.Disabled() || t.btn.Loading() {
		return false
	}
	if t.onClick != nil {
		t.onClick()
	}
	t.scrollY = 0
	return true
}

// Role returns the accessible role.
func (t *FloatBackTop) Role() string { return "button" }

// Focusable reports Tab reachability (hidden/disabled BackTop is skipped).
func (t *FloatBackTop) Focusable() bool {
	return t != nil && t.Visible() && t.btn != nil && t.btn.Focusable()
}

// Focus takes keyboard focus; false when hidden/disabled.
func (t *FloatBackTop) Focus() bool {
	if t == nil || !t.Visible() || t.btn == nil {
		return false
	}
	return t.btn.Focus()
}

// Blur releases keyboard focus.
func (t *FloatBackTop) Blur() {
	if t == nil || t.btn == nil {
		return
	}
	t.btn.Blur()
}

// Focused reports keyboard focus.
func (t *FloatBackTop) Focused() bool {
	return t != nil && t.btn != nil && t.btn.Focused()
}

// KeyActivate handles Enter/Space while focused (false when not activatable).
func (t *FloatBackTop) KeyActivate(key string) bool {
	if t == nil || !t.Visible() || t.btn == nil || !t.btn.Focused() {
		return false
	}
	switch key {
	case "Enter", "Space", " ":
		return t.Click()
	}
	return false
}

// SetTextFace forwards the paint-only font face to the button.
func (t *FloatBackTop) SetTextFace(f text.Face) {
	if t == nil || t.btn == nil {
		return
	}
	t.btn.SetTextFace(f)
}

// SetProvider forwards the theme source to the button.
func (t *FloatBackTop) SetProvider(p *theme.Provider) {
	if t == nil || t.btn == nil {
		return
	}
	t.btn.SetProvider(p)
}

// SetTheme pins exact tokens on the button (nil clears to provider).
func (t *FloatBackTop) SetTheme(tok *theme.Tokens) {
	if t == nil || t.btn == nil {
		return
	}
	t.btn.SetTheme(tok)
}

// Node returns the tree node (host hides it while !Visible).
func (t *FloatBackTop) Node() rendering.RenderObject {
	if t == nil || t.btn == nil {
		return nil
	}
	return t.btn.Node()
}

// Layout sizes the button (zero size while hidden).
func (t *FloatBackTop) Layout(c rendering.Constraints) rendering.Size {
	if t == nil || t.btn == nil {
		return rendering.Size{}
	}
	if !t.Visible() {
		return rendering.Size{}
	}
	return t.btn.Layout(c)
}
