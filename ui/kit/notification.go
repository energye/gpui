package kit

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Notification defaults — docs/antd/notification.md §6.2 / §6.10
// and components/notification/style prepareComponentToken.
const (
	DefaultNotificationDuration       = 4.5
	DefaultNotificationTop            = 24.0
	DefaultNotificationBottom         = 24.0
	DefaultNotificationWidth          = 384.0
	DefaultNotificationStackThreshold = 3
	DefaultNotificationIconSize       = 24.0
	DefaultNotificationCloseSize      = 22.0
	DefaultNotificationPadV           = 16.0 // paddingMD
	DefaultNotificationPadH           = 24.0 // paddingLG / paddingContentHorizontalLG
	DefaultNotificationGap            = 16.0 // margin (item spacing)
	DefaultNotificationEdge           = 24.0 // marginLG
	DefaultNotificationRadius         = 8.0  // borderRadiusLG
	DefaultNotificationTitleFont      = 16.0 // fontSizeLG
	DefaultNotificationFont           = 14.0
	DefaultNotificationFocusOutset    = 1.5
	DefaultNotificationCloseAria      = "Close"
)

// NotificationPlacement is antd placement.
type NotificationPlacement string

const (
	NotificationTop         NotificationPlacement = "top"
	NotificationTopLeft     NotificationPlacement = "topLeft"
	NotificationTopRight    NotificationPlacement = "topRight"
	NotificationBottom      NotificationPlacement = "bottom"
	NotificationBottomLeft  NotificationPlacement = "bottomLeft"
	NotificationBottomRight NotificationPlacement = "bottomRight"
)

// NotificationType is the Ant notification level (icon sugar).
type NotificationType string

const (
	NotificationInfo    NotificationType = "info"
	NotificationSuccess NotificationType = "success"
	NotificationError   NotificationType = "error"
	NotificationWarning NotificationType = "warning"
)

// NotificationAction is one actions[] button (antd actions / legacy btn).
type NotificationAction struct {
	Label   string
	Primary bool
	OnClick func()
}

// NotificationConfig is the Go-side config for notification.open(config).
// Duration uses Ant seconds. DurationSet distinguishes omitted duration from
// explicit duration=0 (sticky until Destroy/close). Closable defaults true when
// ClosableSet is false.
type NotificationConfig struct {
	Type        NotificationType
	Title       string
	Description string
	Key         string
	IconName    string
	Duration    float64
	DurationSet bool
	Placement   NotificationPlacement
	Closable    bool
	ClosableSet bool
	Actions     []NotificationAction
	OnClick     func()
	OnClose     func()
	Style       Style
	Role        string // "alert" (default) | "status"
}

// NotificationSnapshot is an immutable test/debug view of one active notice.
type NotificationSnapshot struct {
	Key         string
	Type        NotificationType
	Title       string
	Description string
	IconName    string
	Placement   NotificationPlacement
	Duration    float64
	Elapsed     float64
	Closable    bool
	ActionCount int
}

// NotificationHandle is returned from Open and type sugar methods.
type NotificationHandle struct {
	parent *Notification
	key    string
	closed bool
	then   []func()
}

// Key returns the notice key associated with this handle.
func (h *NotificationHandle) Key() string {
	if h == nil {
		return ""
	}
	return h.key
}

// Then runs fn after this notice closes.
func (h *NotificationHandle) Then(fn func()) *NotificationHandle {
	if h == nil || fn == nil {
		return h
	}
	if h.closed {
		fn()
		return h
	}
	h.then = append(h.then, fn)
	return h
}

func (h *NotificationHandle) resolve() {
	if h == nil || h.closed {
		return
	}
	h.closed = true
	thens := append([]func(){}, h.then...)
	h.then = nil
	for _, fn := range thens {
		if fn != nil {
			fn()
		}
	}
}

type notificationItem struct {
	key         string
	typ         NotificationType
	title       string
	description string
	iconName    string
	placement   NotificationPlacement
	duration    float64
	elapsed     float64
	closable    bool
	actions     []NotificationAction
	onClick     func()
	onClose     func()
	style       Style
	role        string
	handle      *NotificationHandle
	seq         int
}

// Notification is an Ant-style app-level notification API plus its portal holder.
//
// Structure:
//
//	OverlayPortal
//	  └─ NotificationLayer (alert live region)
//	       └─ per-placement Column
//	            └─ Pressable
//	                 └─ Decorated (width 384)
//	                      └─ Row(icon + Column(title/desc/actions) + close?)
//
// Hit, layout and paint share the same Decorated/Pressable box. Timed close uses
// the Tree ticker; there is no private frame loop.
type Notification struct {
	Portal   *primitive.OverlayPortal
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size

	Duration       float64
	Top            float64
	Bottom         float64
	Placement      NotificationPlacement
	MaxCount       int
	Stack          bool
	StackThreshold int
	Closable       bool
	Style          Style

	layer *notificationLayer
	items []notificationItem
	seq   int

	boundTree  *core.Tree
	itemOffset map[string]core.Point
}

// NewNotification creates an Ant-style notification API/holder.
func NewNotification() *Notification {
	n := &Notification{
		Duration:       DefaultNotificationDuration,
		Top:            DefaultNotificationTop,
		Bottom:         DefaultNotificationBottom,
		Placement:      NotificationTopRight,
		StackThreshold: DefaultNotificationStackThreshold,
		Closable:       true,
	}
	n.rebuild()
	return n
}

// Node returns the portal node to mount at app/root level (contextHolder).

// ensureBuilt materializes the control tree if missing (#9).
func (n *Notification) ensureBuilt() {
	if n == nil {
		return
	}
	if n.Portal == nil {
		n.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (n *Notification) structureChange() {
	if n == nil {
		return
	}
	n.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (n *Notification) chromeChange() {
	if n == nil {
		return
	}
	n.ensureBuilt()
	n.rebuild()
}

func (n *Notification) Node() core.Node {
	if n == nil {
		return nil
	}
	n.ensureBuilt()
	return n.Portal
}

// SetTheme sets an explicit theme override.
func (n *Notification) SetTheme(th *core.Theme) {
	if n == nil {
		return
	}
	n.Theme = th
	n.refresh()
}

// SetFace sets the notification content font face.
func (n *Notification) SetFace(face text.Face) {
	if n == nil {
		return
	}
	n.Face = face
	n.refresh()
}

// SetStyle applies default visual overrides for new and existing notices.
func (n *Notification) SetStyle(st Style) {
	if n == nil {
		return
	}
	n.Style = st
	n.refresh()
}

// SetDuration sets the default auto-close duration in seconds. 0 makes new
// notices sticky unless a config overrides duration.
func (n *Notification) SetDuration(seconds float64) {
	if n == nil {
		return
	}
	if seconds < 0 {
		seconds = DefaultNotificationDuration
	}
	n.Duration = seconds
}

// SetPlacement sets the default placement for new notices.
func (n *Notification) SetPlacement(p NotificationPlacement) {
	if n == nil {
		return
	}
	n.Placement = normalizePlacement(p)
}

// SetTop sets the top edge inset in logical pixels (top* placements).
func (n *Notification) SetTop(px float64) {
	if n == nil {
		return
	}
	if px < 0 {
		px = 0
	}
	n.Top = px
	n.markLayer()
}

// SetBottom sets the bottom edge inset in logical pixels (bottom* placements).
func (n *Notification) SetBottom(px float64) {
	if n == nil {
		return
	}
	if px < 0 {
		px = 0
	}
	n.Bottom = px
	n.markLayer()
}

// SetMaxCount caps active notices. n<=0 disables the cap.
func (n *Notification) SetMaxCount(c int) {
	if n == nil {
		return
	}
	if c < 0 {
		c = 0
	}
	n.MaxCount = c
	n.enforceMaxCount()
	n.refresh()
}

// SetStack toggles Ant stack mode.
func (n *Notification) SetStack(enabled bool) {
	if n == nil {
		return
	}
	n.Stack = enabled
	n.refresh()
}

// SetStackThreshold sets the stack collapse threshold; values <1 resolve to 1.
func (n *Notification) SetStackThreshold(c int) {
	if n == nil {
		return
	}
	if c < 1 {
		c = 1
	}
	n.StackThreshold = c
	n.refresh()
}

// SetClosable sets the default closable flag for new notices.
func (n *Notification) SetClosable(on bool) {
	if n == nil {
		return
	}
	n.Closable = on
}

// Open implements notification.open(config). Same key updates the existing notice.
func (n *Notification) Open(cfg NotificationConfig) *NotificationHandle {
	if n == nil {
		return nil
	}
	if n.Portal == nil {
		n.rebuild()
	}
	if cfg.Key == "" {
		n.seq++
		cfg.Key = fmt.Sprintf("ntf-%d", n.seq)
	}
	dur := n.Duration
	if dur < 0 {
		dur = DefaultNotificationDuration
	}
	if cfg.DurationSet {
		dur = cfg.Duration
	}
	if dur < 0 {
		dur = DefaultNotificationDuration
	}
	placement := cfg.Placement
	if placement == "" {
		placement = n.Placement
	}
	placement = normalizePlacement(placement)
	closable := n.Closable
	if cfg.ClosableSet {
		closable = cfg.Closable
	}
	role := cfg.Role
	if role == "" {
		role = "alert"
	}
	handle := &NotificationHandle{parent: n, key: cfg.Key}
	next := notificationItem{
		key:         cfg.Key,
		typ:         cfg.Type,
		title:       cfg.Title,
		description: cfg.Description,
		iconName:    cfg.IconName,
		placement:   placement,
		duration:    dur,
		closable:    closable,
		actions:     append([]NotificationAction{}, cfg.Actions...),
		onClick:     cfg.OnClick,
		onClose:     cfg.OnClose,
		style:       cfg.Style,
		role:        role,
		handle:      handle,
		seq:         n.seq,
	}
	if idx := n.indexOf(cfg.Key); idx >= 0 {
		old := n.items[idx]
		next.seq = old.seq
		next.elapsed = 0
		n.items[idx] = next
		n.refresh()
		n.syncTicker()
		return handle
	}
	n.items = append(n.items, next)
	n.enforceMaxCount()
	n.refresh()
	n.syncTicker()
	return handle
}

// Info opens an info notification.
func (n *Notification) Info(cfg NotificationConfig) *NotificationHandle {
	cfg.Type = NotificationInfo
	return n.Open(cfg)
}

// Success opens a success notification.
func (n *Notification) Success(cfg NotificationConfig) *NotificationHandle {
	cfg.Type = NotificationSuccess
	return n.Open(cfg)
}

// Error opens an error notification.
func (n *Notification) Error(cfg NotificationConfig) *NotificationHandle {
	cfg.Type = NotificationError
	return n.Open(cfg)
}

// Warning opens a warning notification.
func (n *Notification) Warning(cfg NotificationConfig) *NotificationHandle {
	cfg.Type = NotificationWarning
	return n.Open(cfg)
}

// Destroy clears all notices, or only the supplied key(s).
func (n *Notification) Destroy(keys ...string) {
	if n == nil || len(n.items) == 0 {
		return
	}
	if len(keys) == 0 {
		for len(n.items) > 0 {
			n.closeAt(0)
		}
		n.refresh()
		n.syncTicker()
		return
	}
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	for i := 0; i < len(n.items); {
		if keySet[n.items[i].key] {
			n.closeAt(i)
			continue
		}
		i++
	}
	n.refresh()
	n.syncTicker()
}

// Count returns the number of active notices.
func (n *Notification) Count() int {
	if n == nil {
		return 0
	}
	return len(n.items)
}

// Items returns active notices in insertion order (oldest first).
func (n *Notification) Items() []NotificationSnapshot {
	if n == nil {
		return nil
	}
	out := make([]NotificationSnapshot, 0, len(n.items))
	for _, it := range n.items {
		out = append(out, snapshotOfNotification(it))
	}
	return out
}

// VisibleItems returns the items currently rendered after stack collapse.
func (n *Notification) VisibleItems() []NotificationSnapshot {
	if n == nil {
		return nil
	}
	src := n.visibleItems()
	out := make([]NotificationSnapshot, 0, len(src))
	for _, it := range src {
		out = append(out, snapshotOfNotification(it))
	}
	return out
}

// PlacementOf returns the placement of the notice with key, or empty.
func (n *Notification) PlacementOf(key string) NotificationPlacement {
	if n == nil {
		return ""
	}
	if idx := n.indexOf(key); idx >= 0 {
		return n.items[idx].placement
	}
	return ""
}

// ItemOffset returns the layout offset of the notice card with key after layout.
// ok is false when the key is not visible or not yet laid out.
func (n *Notification) ItemOffset(key string) (core.Point, bool) {
	if n == nil || key == "" || n.itemOffset == nil {
		return core.Point{}, false
	}
	off, ok := n.itemOffset[key]
	return off, ok
}

// AttachTicker binds timed close to the demand-frame loop.
func (n *Notification) AttachTicker(t *core.Tree) {
	if n == nil || t == nil {
		return
	}
	n.boundTree = t
	t.BindTicker(n, n.needsTicker())
}

// Tick advances durations. Implements core.Ticker.
func (n *Notification) Tick(dt float64) bool {
	if n == nil || len(n.items) == 0 {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	changed := false
	for i := 0; i < len(n.items); {
		if n.items[i].duration > 0 {
			n.items[i].elapsed += dt
			if n.items[i].elapsed >= n.items[i].duration {
				n.closeAt(i)
				changed = true
				continue
			}
		}
		i++
	}
	if changed {
		n.refresh()
	}
	n.syncTicker()
	return n.needsTicker()
}

func (n *Notification) theme() *core.Theme {
	var node core.Node
	if n != nil && n.Portal != nil {
		node = n.Portal
	}
	return themeOf(n.Theme, node)
}

func (n *Notification) rebuild() {
	if n.layer == nil {
		n.layer = &notificationLayer{host: n}
		n.layer.Init(n.layer)
		n.layer.Hit = core.HitDefer
		n.layer.Base().Role = "alert"
	}
	if n.Portal == nil {
		n.Portal = primitive.NewOverlayPortal(n.layer)
		n.Portal.ID = ""
		n.Portal.ZOrder = OverlayZNotification
	} else {
		n.Portal.Content = n.layer
		n.Portal.ZOrder = OverlayZNotification
	}
	n.Portal.SetThemeHook(func(*core.Theme) { n.refresh() })
	n.refresh()
}

func (n *Notification) refresh() {
	if n == nil {
		return
	}
	if n.layer == nil || n.Portal == nil {
		n.rebuild()
		return
	}
	n.layer.ClearChildren()
	visible := n.visibleItems()
	by := map[NotificationPlacement][]notificationItem{}
	order := []NotificationPlacement{
		NotificationTop, NotificationTopLeft, NotificationTopRight,
		NotificationBottom, NotificationBottomLeft, NotificationBottomRight,
	}
	for _, it := range visible {
		by[it.placement] = append(by[it.placement], it)
	}
	for _, pl := range order {
		list := by[pl]
		if len(list) == 0 {
			continue
		}
		col := primitive.Column()
		col.Gap = DefaultNotificationGap
		col.MainAlign = core.MainStart
		col.CrossAlign = core.CrossStretch
		for i := range list {
			col.AddChild(n.buildItem(&list[i]))
		}
		if n.Stack && len(n.items) > n.stackThreshold() && len(n.items) > 0 {
			more := primitive.NewText(fmt.Sprintf("+%d", len(n.items)-1))
			more.FontSize = 12
			more.Face = n.Face
			more.Color = n.theme().Color(core.TokenColorTextSecondary)
			col.AddChild(more)
		}
		g := &notificationGroup{host: n, placement: pl}
		g.Init(g)
		g.Hit = core.HitDefer
		g.AddChild(col)
		n.layer.AddChild(g)
	}
	n.layer.Base().Label = n.statusLabel()
	n.layer.Base().Role = n.layerRole()
	n.Portal.SetOpen(len(n.items) > 0)
	n.markLayer()
}

func (n *Notification) buildItem(it *notificationItem) core.Node {
	th := n.theme()
	fontSize := th.SizeOr(core.TokenFontSize, DefaultNotificationFont)
	if it.style.FontSize > 0 {
		fontSize = it.style.FontSize
	} else if n.Style.FontSize > 0 {
		fontSize = n.Style.FontSize
	}
	titleSize := th.SizeOr(core.TokenFontSizeLG, DefaultNotificationTitleFont)
	padV := th.SizeOr(core.TokenPadding, DefaultNotificationPadV)
	padH := th.SizeOr(core.TokenPaddingLG, DefaultNotificationPadH)
	radius := th.SizeOr(core.TokenBorderRadiusLG, DefaultNotificationRadius)
	if n.Style.hasRadius() {
		radius = n.Style.Radius
	}
	if it.style.hasRadius() {
		radius = it.style.Radius
	}
	iconGap := th.SizeOr(core.TokenMarginSM, 12)
	titleDescGap := th.SizeOr(core.TokenMarginXS, 8)
	if titleDescGap < 4 {
		titleDescGap = 8
	}

	section := primitive.Column()
	section.Gap = titleDescGap
	section.MainAlign = core.MainStart
	section.CrossAlign = core.CrossStretch
	if it.title != "" {
		title := primitive.NewText(it.title)
		title.FontSize = titleSize
		title.Face = n.Face
		title.Color = th.Color(core.TokenColorText)
		if n.Style.Text.A > 0 {
			title.Color = n.Style.Text
		}
		if it.style.Text.A > 0 {
			title.Color = it.style.Text
		}
		section.AddChild(title)
	}
	if it.description != "" {
		desc := primitive.NewText(it.description)
		desc.FontSize = fontSize
		desc.Face = n.Face
		desc.Color = th.Color(core.TokenColorTextSecondary)
		if desc.Color.A == 0 {
			desc.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
		}
		section.AddChild(desc)
	}
	if len(it.actions) > 0 {
		actRow := primitive.Row()
		actRow.Gap = th.SizeOr(core.TokenMarginXS, 8)
		if actRow.Gap < 4 {
			actRow.Gap = 8
		}
		actRow.MainAlign = core.MainEnd
		actRow.CrossAlign = core.CrossCenter
		for _, act := range it.actions {
			a := act
			btn := NewButton(a.Label)
			btn.SetFace(n.Face)
			btn.SetSize(ButtonSmall)
			if a.Primary {
				btn.SetType(ButtonPrimary)
			} else {
				btn.SetType(ButtonDefault)
				btn.SetVariant(ButtonVariantLink)
			}
			if a.OnClick != nil {
				btn.SetOnClick(a.OnClick)
			}
			actRow.AddChild(btn.Node())
		}
		section.AddChild(actRow)
	}

	body := primitive.Row()
	body.Gap = iconGap
	body.MainAlign = core.MainStart
	body.CrossAlign = core.CrossStart
	if icon := n.buildIcon(it, th); icon != nil {
		body.AddChild(icon)
	}
	flex := primitive.NewFlexible(1, section)
	flex.FillChild = true
	body.AddChild(flex)
	if it.closable {
		body.AddChild(n.buildClose(it, th, radius))
	}

	dec := primitive.NewDecorated(body)
	dec.SkinType = TypeNotification
	dec.Padding = primitive.EdgeInsets{Top: padV, Bottom: padV, Left: padH, Right: padH}
	dec.Radius = radius
	dec.Background = th.Color(core.TokenColorBgContainer)
	if n.Style.Background.A > 0 {
		dec.Background = n.Style.Background
	}
	if it.style.Background.A > 0 {
		dec.Background = it.style.Background
	}
	dec.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	dec.BorderColor = th.Color(core.TokenColorBorderSecondary)
	if dec.BorderColor.A == 0 {
		dec.BorderColor = th.Color(core.TokenColorBorder)
	}
	if n.Style.Border.A > 0 {
		dec.BorderColor = n.Style.Border
	}
	if it.style.Border.A > 0 {
		dec.BorderColor = it.style.Border
	}
	width := DefaultNotificationWidth
	if n.Style.Width > 0 {
		width = n.Style.Width
	}
	if it.style.Width > 0 {
		width = it.style.Width
	}
	dec.Width = width
	dec.ExpandWidth = true

	press := primitive.NewPressable(dec)
	press.Focusable = false
	press.ShowFocusRing = false
	press.EnableRipple = false
	press.FocusRingRadius = radius
	press.Base().Role = it.role
	if press.Base().Role == "" {
		press.Base().Role = "alert"
	}
	press.Base().Label = it.title
	if it.description != "" {
		if press.Base().Label != "" {
			press.Base().Label += " — " + it.description
		} else {
			press.Base().Label = it.description
		}
	}
	if it.onClick != nil {
		key := it.key
		press.Click = func() {
			if idx := n.indexOf(key); idx >= 0 && n.items[idx].onClick != nil {
				n.items[idx].onClick()
			}
		}
	} else {
		press.Cursor = core.CursorDefault
	}
	card := &notificationCard{key: it.key}
	card.Init(card)
	card.Hit = core.HitDefer
	card.AddChild(press)
	return card
}

func (n *Notification) buildIcon(it *notificationItem, th *core.Theme) core.Node {
	name := it.iconName
	if name == "" {
		name = defaultNotificationIcon(it.typ)
	}
	if name == "" {
		return nil
	}
	col := notificationTypeColor(th, it.typ)
	if it.style.Text.A > 0 && it.iconName != "" {
		// custom icon may carry style text as tint when set
		col = it.style.Text
	}
	size := DefaultNotificationIconSize
	ic := primitive.NewIcon(name)
	ic.Size = size
	ic.Color = col
	return ic
}

func (n *Notification) buildClose(it *notificationItem, th *core.Theme, radius float64) core.Node {
	x := primitive.NewText("×")
	x.FontSize = th.SizeOr(core.TokenFontSize, DefaultNotificationFont)
	x.Face = n.Face
	closeCol := th.Color(core.TokenColorTextTertiary)
	if closeCol.A == 0 {
		closeCol = th.Color(core.TokenColorTextSecondary)
	}
	if closeCol.A == 0 {
		closeCol = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	x.Color = closeCol
	cp := primitive.NewPressable(x)
	cp.Focusable = true
	cp.ShowFocusRing = true
	cp.FocusRingRadius = radius
	if cp.FocusRingRadius <= 0 {
		cp.FocusRingRadius = 4
	}
	cp.FocusRingOutset = DefaultNotificationFocusOutset
	cp.EnableRipple = false
	cp.Padding = primitive.EdgeInsets{Left: 8}
	key := it.key
	cp.Click = func() { n.Destroy(key) }
	cp.Base().Role = "button"
	cp.Base().Label = DefaultNotificationCloseAria
	return cp
}

func (n *Notification) visibleItems() []notificationItem {
	if n == nil || len(n.items) == 0 {
		return nil
	}
	if n.Stack && len(n.items) > n.stackThreshold() {
		return []notificationItem{n.items[len(n.items)-1]}
	}
	out := make([]notificationItem, len(n.items))
	copy(out, n.items)
	return out
}

func (n *Notification) stackThreshold() int {
	if n == nil || n.StackThreshold < 1 {
		return DefaultNotificationStackThreshold
	}
	return n.StackThreshold
}

func (n *Notification) indexOf(key string) int {
	if n == nil || key == "" {
		return -1
	}
	for i := range n.items {
		if n.items[i].key == key {
			return i
		}
	}
	return -1
}

func (n *Notification) closeAt(i int) {
	if n == nil || i < 0 || i >= len(n.items) {
		return
	}
	it := n.items[i]
	n.items = append(n.items[:i], n.items[i+1:]...)
	if it.onClose != nil {
		it.onClose()
	}
	if it.handle != nil {
		it.handle.resolve()
	}
}

func (n *Notification) enforceMaxCount() {
	if n == nil || n.MaxCount <= 0 {
		return
	}
	for len(n.items) > n.MaxCount {
		n.closeAt(0)
	}
}

func (n *Notification) needsTicker() bool {
	if n == nil {
		return false
	}
	for _, it := range n.items {
		if it.duration > 0 {
			return true
		}
	}
	return false
}

func (n *Notification) syncTicker() {
	if n != nil && n.boundTree != nil {
		n.boundTree.BindTicker(n, n.needsTicker())
	}
}

func (n *Notification) markLayer() {
	if n == nil || n.layer == nil {
		return
	}
	n.layer.MarkNeedsLayout()
	n.layer.MarkNeedsPaint()
}

func (n *Notification) statusLabel() string {
	if n == nil || len(n.items) == 0 {
		return ""
	}
	it := n.items[len(n.items)-1]
	if it.title != "" {
		return it.title
	}
	return it.description
}

func (n *Notification) layerRole() string {
	if n == nil || len(n.items) == 0 {
		return "alert"
	}
	role := n.items[len(n.items)-1].role
	if role == "" {
		return "alert"
	}
	return role
}

func snapshotOfNotification(it notificationItem) NotificationSnapshot {
	return NotificationSnapshot{
		Key:         it.key,
		Type:        it.typ,
		Title:       it.title,
		Description: it.description,
		IconName:    resolvedNotificationIcon(it),
		Placement:   it.placement,
		Duration:    it.duration,
		Elapsed:     it.elapsed,
		Closable:    it.closable,
		ActionCount: len(it.actions),
	}
}

func resolvedNotificationIcon(it notificationItem) string {
	if it.iconName != "" {
		return it.iconName
	}
	return defaultNotificationIcon(it.typ)
}

func defaultNotificationIcon(typ NotificationType) string {
	switch typ {
	case NotificationSuccess:
		return "check"
	case NotificationError:
		return "close"
	case NotificationWarning:
		return "info"
	case NotificationInfo:
		return "info"
	default:
		return ""
	}
}

func notificationTypeColor(th *core.Theme, typ NotificationType) render.RGBA {
	if th == nil {
		th = DefaultTheme()
	}
	switch typ {
	case NotificationSuccess:
		return th.Color(core.TokenColorSuccess)
	case NotificationError:
		return th.Color(core.TokenColorError)
	case NotificationWarning:
		return th.Color(core.TokenColorWarning)
	case NotificationInfo:
		return th.Color(core.TokenColorPrimary)
	default:
		return th.Color(core.TokenColorPrimary)
	}
}

func normalizePlacement(p NotificationPlacement) NotificationPlacement {
	switch p {
	case NotificationTop, NotificationTopLeft, NotificationTopRight,
		NotificationBottom, NotificationBottomLeft, NotificationBottomRight:
		return p
	default:
		return NotificationTopRight
	}
}

type notificationLayer struct {
	core.NodeBase
	host *Notification
}

func (l *notificationLayer) TypeID() string { return TypeNotification }

func (l *notificationLayer) Layout(c core.Constraints) core.Size {
	var portal *primitive.OverlayPortal
	var vp core.Size
	top := DefaultNotificationTop
	bottom := DefaultNotificationBottom
	edge := DefaultNotificationEdge
	if l.host != nil {
		portal = l.host.Portal
		vp = l.host.Viewport
		top = l.host.Top
		bottom = l.host.Bottom
		l.host.itemOffset = map[string]core.Point{}
	}
	vw, vh := resolveOverlayViewport(vp, portal, c.MaxWidth, c.MaxHeight)
	for _, child := range l.Children() {
		grp, ok := child.(*notificationGroup)
		if !ok || grp == nil {
			sz := child.Layout(core.Loose(DefaultNotificationWidth, vh))
			child.Base().SetOffset(core.Point{X: vw - sz.Width - edge, Y: top})
			continue
		}
		maxW := DefaultNotificationWidth
		if maxW > vw-edge*2 {
			maxW = vw - edge*2
		}
		if maxW < 80 {
			maxW = 80
		}
		sz := child.Layout(core.Loose(maxW, vh))
		x, y := placementOrigin(grp.placement, vw, vh, sz, top, bottom, edge)
		child.Base().SetOffset(core.Point{X: x, Y: y})
		if l.host != nil {
			recordNotificationOffsets(l.host, child, 0, 0)
		}
	}
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func recordNotificationOffsets(host *Notification, n core.Node, baseX, baseY float64) {
	if host == nil || n == nil {
		return
	}
	off := n.Base().Offset()
	absX, absY := baseX+off.X, baseY+off.Y
	if card, ok := n.(*notificationCard); ok && card.key != "" {
		host.itemOffset[card.key] = core.Point{X: absX, Y: absY}
		return
	}
	for _, c := range n.Children() {
		recordNotificationOffsets(host, c, absX, absY)
	}
}

func (l *notificationLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *notificationLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }

type notificationGroup struct {
	core.NodeBase
	host      *Notification
	placement NotificationPlacement
}

func (g *notificationGroup) TypeID() string { return "kit.NotificationGroup" }

func (g *notificationGroup) Layout(c core.Constraints) core.Size {
	var w, h float64
	y := 0.0
	gap := DefaultNotificationGap
	kids := g.Children()
	for i, child := range kids {
		sz := child.Layout(core.Loose(c.MaxWidth, c.MaxHeight))
		child.Base().SetOffset(core.Point{X: 0, Y: y})
		if sz.Width > w {
			w = sz.Width
		}
		y += sz.Height
		if i < len(kids)-1 {
			y += gap
		}
	}
	h = y
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	out := core.Size{Width: w, Height: h}
	g.SetSize(out)
	return out
}

func (g *notificationGroup) Paint(pc *core.PaintContext) { g.DefaultPaintChildren(pc) }

func (g *notificationGroup) HitTest(p core.Point) core.Node { return g.DefaultHitTest(p) }

// notificationCard wraps one notice so layout can key offsets for tests.
type notificationCard struct {
	core.NodeBase
	key string
}

func (c *notificationCard) TypeID() string { return "kit.NotificationCard" }

func (c *notificationCard) Layout(cs core.Constraints) core.Size {
	var w, h float64
	for _, child := range c.Children() {
		sz := child.Layout(cs)
		child.Base().SetOffset(core.Point{})
		if sz.Width > w {
			w = sz.Width
		}
		if sz.Height > h {
			h = sz.Height
		}
	}
	out := core.Size{Width: w, Height: h}
	c.SetSize(out)
	return out
}

func (c *notificationCard) Paint(pc *core.PaintContext) { c.DefaultPaintChildren(pc) }

func (c *notificationCard) HitTest(p core.Point) core.Node { return c.DefaultHitTest(p) }

func placementOrigin(p NotificationPlacement, vw, vh float64, sz core.Size, top, bottom, edge float64) (x, y float64) {
	switch p {
	case NotificationTop:
		x = (vw - sz.Width) / 2
		if x < 0 {
			x = 0
		}
		y = top
	case NotificationTopLeft:
		x = edge
		y = top
	case NotificationTopRight:
		x = vw - sz.Width - edge
		if x < 0 {
			x = 0
		}
		y = top
	case NotificationBottom:
		x = (vw - sz.Width) / 2
		if x < 0 {
			x = 0
		}
		y = vh - sz.Height - bottom
		if y < 0 {
			y = 0
		}
	case NotificationBottomLeft:
		x = edge
		y = vh - sz.Height - bottom
		if y < 0 {
			y = 0
		}
	case NotificationBottomRight:
		x = vw - sz.Width - edge
		if x < 0 {
			x = 0
		}
		y = vh - sz.Height - bottom
		if y < 0 {
			y = 0
		}
	default:
		x = vw - sz.Width - edge
		if x < 0 {
			x = 0
		}
		y = top
	}
	return x, y
}
