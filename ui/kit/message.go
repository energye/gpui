package kit

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Message defaults — docs/antd/message.md §6.2 / §6.10.
const (
	DefaultMessageDuration       = 3.0
	DefaultMessageTop            = 8.0
	DefaultMessageStackThreshold = 3
	DefaultMessageIconSize       = 16.0
	DefaultMessageMaxWidth       = 520.0
)

// MessageType is the Ant message level.
type MessageType string

const (
	MessageInfo    MessageType = "info"
	MessageSuccess MessageType = "success"
	MessageError   MessageType = "error"
	MessageWarning MessageType = "warning"
	MessageLoading MessageType = "loading"
)

// MessageConfig is the Go-side config object for message.open(config).
// Duration uses Ant seconds. DurationSet distinguishes omitted duration from
// explicit duration=0 (sticky until Destroy/close).
type MessageConfig struct {
	Type         MessageType
	Content      string
	Key          string
	IconName     string
	Duration     float64
	DurationSet  bool
	PauseOnHover bool
	OnClick      func()
	OnClose      func()
	Style        Style
}

// MessageSnapshot is an immutable test/debug view of one active message.
type MessageSnapshot struct {
	Key      string
	Type     MessageType
	Content  string
	IconName string
	Duration float64
	Elapsed  float64
}

// MessageHandle is the thenable returned from Open and sugar methods.
type MessageHandle struct {
	parent *Message
	key    string
	closed bool
	then   []func()
}

// Key returns the message key associated with this handle.
func (h *MessageHandle) Key() string {
	if h == nil {
		return ""
	}
	return h.key
}

// Then runs fn after this message closes. It returns the same handle so callers
// can register multiple close continuations.
func (h *MessageHandle) Then(fn func()) *MessageHandle {
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

func (h *MessageHandle) resolve() {
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

type messageItem struct {
	key      string
	typ      MessageType
	content  string
	iconName string
	duration float64
	elapsed  float64
	onClick  func()
	onClose  func()
	style    Style
	handle   *MessageHandle
	seq      int
}

// Message is an Ant-style app-level message API plus its portal holder.
//
// Structure:
//
//	OverlayPortal
//	  └─ MessageLayer (status live region)
//	       └─ Column(top-center)
//	            └─ Pressable
//	                 └─ Decorated
//	                      └─ Row(icon/spinner + Text)
//
// Hit, layout and paint share the same Decorated/Pressable box. Timed close and
// loading spinner use the Tree ticker; there is no private frame loop.
type Message struct {
	Portal   *primitive.OverlayPortal
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size

	Duration       float64
	Top            float64
	MaxCount       int
	Stack          bool
	StackThreshold int
	Style          Style

	layer *messageLayer
	items []messageItem
	seq   int

	boundTree *core.Tree
	spinPhase float64
}

// MessageHost is a compatibility alias for older app code. New code should use
// Message / NewMessage.
type MessageHost = Message

// NewMessage creates an Ant-style message API/holder.
func NewMessage() *Message {
	m := &Message{
		Duration:       DefaultMessageDuration,
		Top:            DefaultMessageTop,
		StackThreshold: DefaultMessageStackThreshold,
	}
	m.rebuild()
	return m
}

// NewMessageHost is kept as a constructor alias for existing root mounting code.
func NewMessageHost() *MessageHost { return NewMessage() }

// Node returns the portal node to mount at app/root level.

// ensureBuilt materializes the control tree if missing (#9).
func (m *Message) ensureBuilt() {
	if m == nil {
		return
	}
	if m.Portal == nil {
		m.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (m *Message) structureChange() {
	if m == nil {
		return
	}
	m.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (m *Message) chromeChange() {
	if m == nil {
		return
	}
	m.ensureBuilt()
	m.rebuild()
}

func (m *Message) Node() core.Node {
	if m == nil {
		return nil
	}
	m.ensureBuilt()
	return m.Portal
}

// SetTheme sets an explicit theme override.
func (m *Message) SetTheme(th *core.Theme) {
	if m == nil {
		return
	}
	m.Theme = th
	m.refresh()
}

// SetFace sets the message content font face.
func (m *Message) SetFace(face text.Face) {
	if m == nil {
		return
	}
	m.Face = face
	m.refresh()
}

// SetStyle applies default visual overrides for new and existing messages.
func (m *Message) SetStyle(st Style) {
	if m == nil {
		return
	}
	m.Style = st
	m.refresh()
}

// SetDuration sets the default auto-close duration in seconds. 0 makes new
// messages sticky unless a config overrides duration.
func (m *Message) SetDuration(seconds float64) {
	if m == nil {
		return
	}
	if seconds < 0 {
		seconds = DefaultMessageDuration
	}
	m.Duration = seconds
}

// SetTop sets the top offset in logical pixels.
func (m *Message) SetTop(px float64) {
	if m == nil {
		return
	}
	if px < 0 {
		px = 0
	}
	m.Top = px
	m.markLayer()
}

// SetMaxCount caps active messages. n<=0 disables the cap.
func (m *Message) SetMaxCount(n int) {
	if m == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	m.MaxCount = n
	m.enforceMaxCount()
	m.refresh()
}

// SetStack toggles Ant 6.4 stack mode.
func (m *Message) SetStack(enabled bool) {
	if m == nil {
		return
	}
	m.Stack = enabled
	m.refresh()
}

// SetStackThreshold sets the stack collapse threshold; values <1 resolve to 1.
func (m *Message) SetStackThreshold(n int) {
	if m == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	m.StackThreshold = n
	m.refresh()
}

// Open implements message.open(config). Same key updates the existing message.
func (m *Message) Open(cfg MessageConfig) *MessageHandle {
	if m == nil {
		return nil
	}
	if m.Portal == nil {
		m.rebuild()
	}
	if cfg.Type == "" {
		cfg.Type = MessageInfo
	}
	if cfg.Key == "" {
		m.seq++
		cfg.Key = fmt.Sprintf("msg-%d", m.seq)
	}
	dur := m.Duration
	if dur < 0 {
		dur = DefaultMessageDuration
	}
	if cfg.DurationSet {
		dur = cfg.Duration
	}
	if dur < 0 {
		dur = DefaultMessageDuration
	}
	handle := &MessageHandle{parent: m, key: cfg.Key}
	next := messageItem{
		key:      cfg.Key,
		typ:      cfg.Type,
		content:  cfg.Content,
		iconName: cfg.IconName,
		duration: dur,
		onClick:  cfg.OnClick,
		onClose:  cfg.OnClose,
		style:    cfg.Style,
		handle:   handle,
		seq:      m.seq,
	}
	if idx := m.indexOf(cfg.Key); idx >= 0 {
		old := m.items[idx]
		next.seq = old.seq
		m.items[idx] = next
		m.refresh()
		m.syncTicker()
		return handle
	}
	m.items = append(m.items, next)
	m.enforceMaxCount()
	m.refresh()
	m.syncTicker()
	return handle
}

// Info pushes an info message.
func (m *Message) Info(content string, duration ...float64) *MessageHandle {
	return m.openSugar(MessageInfo, content, duration...)
}

// Success pushes a success message.
func (m *Message) Success(content string, duration ...float64) *MessageHandle {
	return m.openSugar(MessageSuccess, content, duration...)
}

// Error pushes an error message.
func (m *Message) Error(content string, duration ...float64) *MessageHandle {
	return m.openSugar(MessageError, content, duration...)
}

// Warning pushes a warning message.
func (m *Message) Warning(content string, duration ...float64) *MessageHandle {
	return m.openSugar(MessageWarning, content, duration...)
}

// Loading pushes a loading message. The default duration still follows Ant (3s);
// pass 0 for sticky loading.
func (m *Message) Loading(content string, duration ...float64) *MessageHandle {
	return m.openSugar(MessageLoading, content, duration...)
}

func (m *Message) openSugar(typ MessageType, content string, duration ...float64) *MessageHandle {
	cfg := MessageConfig{Type: typ, Content: content}
	if len(duration) > 0 {
		cfg.Duration = duration[0]
		cfg.DurationSet = true
	}
	return m.Open(cfg)
}

// Destroy clears all messages, or only the supplied key(s).
func (m *Message) Destroy(keys ...string) {
	if m == nil || len(m.items) == 0 {
		return
	}
	if len(keys) == 0 {
		for len(m.items) > 0 {
			m.closeAt(0)
		}
		m.refresh()
		m.syncTicker()
		return
	}
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	for i := 0; i < len(m.items); {
		if keySet[m.items[i].key] {
			m.closeAt(i)
			continue
		}
		i++
	}
	m.refresh()
	m.syncTicker()
}

// Count returns the number of active messages.
func (m *Message) Count() int {
	if m == nil {
		return 0
	}
	return len(m.items)
}

// Items returns active messages in display order (oldest first).
func (m *Message) Items() []MessageSnapshot {
	if m == nil {
		return nil
	}
	out := make([]MessageSnapshot, 0, len(m.items))
	for _, it := range m.items {
		out = append(out, snapshotOf(it))
	}
	return out
}

// SpinPhase reports the loading spinner phase, mainly for deterministic tests.
func (m *Message) SpinPhase() float64 {
	if m == nil {
		return 0
	}
	return m.spinPhase
}

// VisibleItems returns the items currently rendered after stack collapse.
func (m *Message) VisibleItems() []MessageSnapshot {
	if m == nil {
		return nil
	}
	src := m.visibleItems()
	out := make([]MessageSnapshot, 0, len(src))
	for _, it := range src {
		out = append(out, snapshotOf(it))
	}
	return out
}

// Sync expires timed messages once. It is kept for older one-shot tests; normal
// apps should call AttachTicker and let the tree demand-frame loop drive expiry.
func (m *Message) Sync() {
	if m == nil {
		return
	}
	m.Tick(0)
	m.refresh()
}

// AttachTicker binds timed close and loading spinner to the demand-frame loop.
func (m *Message) AttachTicker(t *core.Tree) {
	if m == nil || t == nil {
		return
	}
	m.boundTree = t
	t.BindTicker(m, m.needsTicker())
}

// Tick advances durations and loading spinner. Implements core.Ticker.
func (m *Message) Tick(dt float64) bool {
	if m == nil || len(m.items) == 0 {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	changed := false
	for i := 0; i < len(m.items); {
		if m.items[i].duration > 0 {
			m.items[i].elapsed += dt
			if m.items[i].elapsed >= m.items[i].duration {
				m.closeAt(i)
				changed = true
				continue
			}
		}
		i++
	}
	if m.hasLoading() {
		m.spinPhase += dt * 1.4
		if m.spinPhase >= 1 {
			m.spinPhase -= float64(int(m.spinPhase))
		}
		m.markLayer()
	}
	if changed {
		m.refresh()
	}
	m.syncTicker()
	return m.needsTicker()
}

func (m *Message) theme() *core.Theme {
	var n core.Node
	if m != nil && m.Portal != nil {
		n = m.Portal
	}
	return themeOf(m.Theme, n)
}

func (m *Message) rebuild() {
	if m.layer == nil {
		m.layer = &messageLayer{host: m}
		m.layer.Init(m.layer)
		m.layer.Hit = core.HitDefer
		m.layer.Base().Role = "status"
	}
	if m.Portal == nil {
		m.Portal = primitive.NewOverlayPortal(m.layer)
		m.Portal.ID = ""
		m.Portal.ZOrder = OverlayZMessage
	} else {
		m.Portal.Content = m.layer
		m.Portal.ZOrder = OverlayZMessage
	}
	m.Portal.SetThemeHook(func(*core.Theme) { m.refresh() })
	m.refresh()
}

func (m *Message) refresh() {
	if m == nil {
		return
	}
	if m.layer == nil || m.Portal == nil {
		m.rebuild()
		return
	}
	m.layer.ClearChildren()
	col := primitive.Column()
	col.Gap = 8
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossCenter
	for _, it := range m.visibleItems() {
		card := m.buildItem(&it)
		col.AddChild(card)
	}
	if m.Stack && len(m.items) > m.stackThreshold() && len(m.items) > 0 {
		more := primitive.NewText(fmt.Sprintf("+%d", len(m.items)-1))
		more.FontSize = 12
		more.Face = m.Face
		more.Color = m.theme().Color(core.TokenColorTextSecondary)
		col.AddChild(more)
	}
	m.layer.AddChild(col)
	m.layer.Base().Label = m.statusLabel()
	m.Portal.SetOpen(len(m.items) > 0)
	m.markLayer()
}

func (m *Message) buildItem(it *messageItem) core.Node {
	th := m.theme()
	fontSize := th.SizeOr(core.TokenFontSize, 14)
	if it.style.FontSize > 0 {
		fontSize = it.style.FontSize
	} else if m.Style.FontSize > 0 {
		fontSize = m.Style.FontSize
	}
	txt := primitive.NewText(it.content)
	txt.FontSize = fontSize
	txt.Face = m.Face
	txt.Color = th.Color(core.TokenColorText)
	if m.Style.Text.A > 0 {
		txt.Color = m.Style.Text
	}
	if it.style.Text.A > 0 {
		txt.Color = it.style.Text
	}
	row := primitive.Row()
	row.Gap = th.SizeOr(core.TokenMarginSM, 8)
	row.CrossAlign = core.CrossCenter
	row.MainAlign = core.MainCenter
	if icon := m.buildIcon(it, th); icon != nil {
		row.AddChild(icon)
	}
	row.AddChild(txt)

	dec := primitive.NewDecorated(row)
	dec.SkinType = TypeMessage
	dec.Padding = primitive.Symmetric(12, 9)
	dec.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	if m.Style.hasRadius() {
		dec.Radius = m.Style.Radius
	}
	if it.style.hasRadius() {
		dec.Radius = it.style.Radius
	}
	dec.Background = th.Color(core.TokenColorBgContainer)
	if m.Style.Background.A > 0 {
		dec.Background = m.Style.Background
	}
	if it.style.Background.A > 0 {
		dec.Background = it.style.Background
	}
	dec.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	dec.BorderColor = th.Color(core.TokenColorBorderSecondary)
	if m.Style.Border.A > 0 {
		dec.BorderColor = m.Style.Border
	}
	if it.style.Border.A > 0 {
		dec.BorderColor = it.style.Border
	}
	dec.SetCenterContent(true)
	dec.MinHeight = th.SizeOr(core.TokenControlHeight, 32) + 6
	dec.MinWidth = 80

	press := primitive.NewPressable(dec)
	press.Focusable = false
	press.ShowFocusRing = false
	press.EnableRipple = false
	press.FocusRingRadius = dec.Radius
	press.Base().Role = "status"
	press.Base().Label = it.content
	if it.onClick != nil {
		key := it.key
		press.Click = func() {
			if idx := m.indexOf(key); idx >= 0 && m.items[idx].onClick != nil {
				m.items[idx].onClick()
			}
		}
	} else {
		press.Cursor = core.CursorDefault
	}
	return press
}

func (m *Message) buildIcon(it *messageItem, th *core.Theme) core.Node {
	name := it.iconName
	if name == "" {
		name = defaultMessageIcon(it.typ)
	}
	col := messageTypeColor(th, it.typ)
	if it.style.Text.A > 0 {
		col = it.style.Text
	}
	size := DefaultMessageIconSize
	if it.typ == MessageLoading {
		return primitive.NewCanvas(size, size, func(pc *core.PaintContext, sz core.Size) {
			m.paintLoading(pc, sz, col)
		})
	}
	ic := primitive.NewIcon(name)
	ic.Size = size
	ic.Color = col
	return ic
}

func (m *Message) paintLoading(pc *core.PaintContext, sz core.Size, col render.RGBA) {
	if pc == nil {
		return
	}
	track := render.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * 0.25}
	stroke := 2.0
	cx, cy := sz.Width/2, sz.Height/2
	r := sz.Width/2 - stroke
	if r < 1 {
		r = 1
	}
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	start := -math.Pi/2 + m.spinPhase*2*math.Pi
	end := start + 2*math.Pi*0.7
	pts := make([]float64, 0, 82)
	for i := 0; i <= 40; i++ {
		a := start + (end-start)*float64(i)/40
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

func (m *Message) visibleItems() []messageItem {
	if m == nil || len(m.items) == 0 {
		return nil
	}
	if m.Stack && len(m.items) > m.stackThreshold() {
		return []messageItem{m.items[len(m.items)-1]}
	}
	out := make([]messageItem, len(m.items))
	copy(out, m.items)
	return out
}

func (m *Message) stackThreshold() int {
	if m == nil || m.StackThreshold < 1 {
		return DefaultMessageStackThreshold
	}
	return m.StackThreshold
}

func (m *Message) indexOf(key string) int {
	if m == nil || key == "" {
		return -1
	}
	for i := range m.items {
		if m.items[i].key == key {
			return i
		}
	}
	return -1
}

func (m *Message) closeAt(i int) {
	if m == nil || i < 0 || i >= len(m.items) {
		return
	}
	it := m.items[i]
	m.items = append(m.items[:i], m.items[i+1:]...)
	if it.onClose != nil {
		it.onClose()
	}
	if it.handle != nil {
		it.handle.resolve()
	}
}

func (m *Message) enforceMaxCount() {
	if m == nil || m.MaxCount <= 0 {
		return
	}
	for len(m.items) > m.MaxCount {
		m.closeAt(0)
	}
}

func (m *Message) hasLoading() bool {
	if m == nil {
		return false
	}
	for _, it := range m.items {
		if it.typ == MessageLoading {
			return true
		}
	}
	return false
}

func (m *Message) needsTicker() bool {
	if m == nil {
		return false
	}
	if m.hasLoading() {
		return true
	}
	for _, it := range m.items {
		if it.duration > 0 {
			return true
		}
	}
	return false
}

func (m *Message) syncTicker() {
	if m != nil && m.boundTree != nil {
		m.boundTree.BindTicker(m, m.needsTicker())
	}
}

func (m *Message) markLayer() {
	if m == nil || m.layer == nil {
		return
	}
	m.layer.MarkNeedsLayout()
	m.layer.MarkNeedsPaint()
}

func (m *Message) statusLabel() string {
	if m == nil || len(m.items) == 0 {
		return ""
	}
	return m.items[len(m.items)-1].content
}

func snapshotOf(it messageItem) MessageSnapshot {
	return MessageSnapshot{
		Key:      it.key,
		Type:     it.typ,
		Content:  it.content,
		IconName: resolvedMessageIcon(it),
		Duration: it.duration,
		Elapsed:  it.elapsed,
	}
}

func resolvedMessageIcon(it messageItem) string {
	if it.iconName != "" {
		return it.iconName
	}
	return defaultMessageIcon(it.typ)
}

func defaultMessageIcon(typ MessageType) string {
	switch typ {
	case MessageSuccess:
		return "check"
	case MessageError:
		return "close"
	case MessageWarning:
		return "info"
	case MessageLoading:
		return "loading"
	default:
		return "info"
	}
}

func messageTypeColor(th *core.Theme, typ MessageType) render.RGBA {
	if th == nil {
		th = DefaultTheme()
	}
	switch typ {
	case MessageSuccess:
		return th.Color(core.TokenColorSuccess)
	case MessageError:
		return th.Color(core.TokenColorError)
	case MessageWarning:
		return th.Color(core.TokenColorWarning)
	default:
		return th.Color(core.TokenColorPrimary)
	}
}

type messageLayer struct {
	core.NodeBase
	host *Message
}

func (l *messageLayer) TypeID() string { return TypeMessage }

func (l *messageLayer) Layout(c core.Constraints) core.Size {
	var portal *primitive.OverlayPortal
	var vp core.Size
	top := DefaultMessageTop
	if l.host != nil {
		portal = l.host.Portal
		vp = l.host.Viewport
		top = l.host.Top
	}
	vw, vh := resolveOverlayViewport(vp, portal, c.MaxWidth, c.MaxHeight)
	maxW := DefaultMessageMaxWidth
	if maxW > vw-16 {
		maxW = vw - 16
	}
	if maxW < 80 {
		maxW = 80
	}
	for _, child := range l.Children() {
		sz := child.Layout(core.Loose(maxW, vh))
		x := (vw - sz.Width) / 2
		if x < 0 {
			x = 0
		}
		child.Base().SetOffset(core.Point{X: x, Y: top})
	}
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *messageLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *messageLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
