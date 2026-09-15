package icon

import (
	"math"
	"sync"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// DefaultIconSize is the edge length used when Size is 0 (antd §6.2.1).
const DefaultIconSize = 16.0

// spinPeriodSec is one full revolution.
const spinPeriodSec = 1.0

// Painter draws a custom glyph in a size×size box (antd component mapping).
// primary/secondary carry the two-tone colors; single-tone glyphs use base().
type Painter func(pc *rendering.PaintContext, size float64, primary, secondary render.RGBA)

// Def describes one registered glyph.
type Def struct {
	// TwoTone marks glyphs with a primary/secondary split.
	TwoTone bool
	// Tag tracks the registering source (iconfont override order).
	Tag string
}

// Registry maps icon names to glyph defs.
type Registry struct {
	mu    sync.RWMutex
	icons map[string]Def
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry { return &Registry{icons: map[string]Def{}} }

// Register adds or replaces name.
func (r *Registry) Register(name string, d Def) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	r.icons[name] = d
	r.mu.Unlock()
}

// Lookup returns the def for name.
func (r *Registry) Lookup(name string) (Def, bool) {
	if r == nil {
		return Def{}, false
	}
	r.mu.RLock()
	d, ok := r.icons[name]
	r.mu.RUnlock()
	return d, ok
}

// Names returns all registered names.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	out := make([]string, 0, len(r.icons))
	for n := range r.icons {
		out = append(out, n)
	}
	r.mu.RUnlock()
	return out
}

// p0Glyphs is the built-in P0 registry (docs/antd/icon.md §1.2).
// Geometry lives in glyph.go; flags here mark two-tone splits.
var p0Glyphs = []struct {
	name    string
	twoTone bool
}{
	{"check", false},
	{"close", false},
	{"info-circle", false},
	{"warning", false},
	{"home", false},
	{"setting", false},
	{"smile", true},
	{"sync", false},
	{"loading", false},
	{"heart", true},
	{"star", false},
	{"search", false},
	{"plus", false},
	{"minus", false},
	{"edit", false},
	{"delete", false},
	{"left", false},
	{"right", false},
	{"up", false},
	{"down", false},
	{"file-text", false},
	{"close-circle", false},
	{"check-circle", true},
	{"exclamation-circle", false},
	{"poweroff", false},
	{"download", false},
	{"ellipsis", false},
	{"ant-design", false},
}

// Global is the default registry preloaded with the P0 set.
var Global = NewRegistry()

// sourceOrder tracks RegisterIconSource override order (later wins).
var sourceOrder = struct {
	mu    sync.Mutex
	order []string
}{}

func init() {
	for _, g := range p0Glyphs {
		Global.Register(g.name, Def{TwoTone: g.twoTone, Tag: "p0"})
	}
}

// Register adds a glyph to the global registry.
func Register(name string, d Def) { Global.Register(name, d) }

// RegisterIconSource merges an offline iconfont source into the global
// registry (antd createFromIconfontCN mapping, no network). Later sources
// override earlier ones on name collision (antd scriptUrl[] order).
func RegisterIconSource(sourceID string, icons map[string]Def) {
	if sourceID == "" || len(icons) == 0 {
		return
	}
	for n, d := range icons {
		if d.Tag == "" {
			d.Tag = sourceID
		}
		Global.Register(n, d)
	}
	sourceOrder.mu.Lock()
	sourceOrder.order = append(sourceOrder.order, sourceID)
	sourceOrder.mu.Unlock()
}

// IconfontOptions selects offline sources (antd scriptUrl[] mapping).
type IconfontOptions struct {
	Sources []string
}

// IconfontFamily is an offline icon family (no remote script fetch).
type IconfontFamily struct {
	mu       sync.RWMutex
	sources  []string
	icons    map[string]Def
	painters map[string]Painter
}

// CreateFromIconfont builds an offline family.
func CreateFromIconfont(opts IconfontOptions) *IconfontFamily {
	return &IconfontFamily{
		sources:  append([]string(nil), opts.Sources...),
		icons:    map[string]Def{},
		painters: map[string]Painter{},
	}
}

// Register adds a glyph to the family.
func (f *IconfontFamily) Register(typeName string, d Def) {
	if f == nil || typeName == "" {
		return
	}
	f.mu.Lock()
	f.icons[typeName] = d
	f.mu.Unlock()
}

// RegisterPainter adds a custom painter to the family.
func (f *IconfontFamily) RegisterPainter(typeName string, p Painter) {
	if f == nil || typeName == "" || p == nil {
		return
	}
	f.mu.Lock()
	f.painters[typeName] = p
	f.mu.Unlock()
}

// NewIcon creates an icon bound to this family.
func (f *IconfontFamily) NewIcon(typeName string) *Icon {
	ic := NewIcon(typeName)
	ic.family = f
	ic.syncNode()
	return ic
}

func (f *IconfontFamily) lookupDef(name string) (Def, bool) {
	if f == nil {
		return Def{}, false
	}
	f.mu.RLock()
	d, ok := f.icons[name]
	f.mu.RUnlock()
	return d, ok
}

func (f *IconfontFamily) lookupPainter(name string) (Painter, bool) {
	if f == nil {
		return nil, false
	}
	f.mu.RLock()
	p, ok := f.painters[name]
	f.mu.RUnlock()
	return p, ok
}

// global two-tone default (antd setTwoToneColor mapping).
var twoToneGlobal = struct {
	mu sync.RWMutex
	c  render.RGBA
}{c: render.Hex("#1677ff")}

// SetTwoToneColorGlobal sets the process-wide two-tone primary.
func SetTwoToneColorGlobal(c render.RGBA) {
	twoToneGlobal.mu.Lock()
	twoToneGlobal.c = c
	twoToneGlobal.mu.Unlock()
}

// GetTwoToneColorGlobal returns the process-wide two-tone primary.
func GetTwoToneColorGlobal() render.RGBA {
	twoToneGlobal.mu.RLock()
	defer twoToneGlobal.mu.RUnlock()
	return twoToneGlobal.c
}

// Icon is the Icon widget (docs/antd/icon.md §6.10, mapped to current pkgs).
//
// It owns a rendering.RenderBox node: put Node() in the tree, drive Tick via
// a scheduler.TickerRegistry, and read Effective* for assertions.
type Icon struct {
	name         string
	size         float64
	color        render.RGBA
	rotate       float64
	spin         bool
	phase        float64
	reduceMotion bool

	twoTonePrimary   render.RGBA
	twoToneSecondary render.RGBA
	hasSecondary     bool

	painter    Painter
	provider   *theme.Provider
	override   *theme.Tokens
	ariaLabel  string
	decorative bool
	disabled   bool
	className  string
	registry   *Registry
	family     *IconfontFamily

	node     *rendering.RenderBox
	attached *scheduler.TickerRegistry
}

// NewIcon creates an icon for name (unknown names render a placeholder).
func NewIcon(name string) *Icon {
	ic := &Icon{name: name, decorative: true}
	ic.node = rendering.NewRenderBox()
	ic.node.SetRepaintBoundary(true)
	paint := ic
	ic.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size.Width)
	}
	ic.syncNode()
	return ic
}

// Name returns the icon name.
func (ic *Icon) Name() string { return ic.name }

// SetName changes the glyph.
func (ic *Icon) SetName(s string) {
	if ic == nil || ic.name == s {
		return
	}
	ic.name = s
	ic.syncNode()
}

// SetSize sets the edge length (<=0 selects DefaultIconSize).
func (ic *Icon) SetSize(s float64) {
	if ic == nil {
		return
	}
	ic.size = s
	ic.syncNode()
}

// EffectiveSize returns the laid-out edge length.
func (ic *Icon) EffectiveSize() float64 {
	if ic == nil || ic.size <= 0 {
		return DefaultIconSize
	}
	return ic.size
}

// SetColor sets the single-tone color (A==0 clears to theme).
func (ic *Icon) SetColor(c render.RGBA) {
	if ic == nil {
		return
	}
	ic.color = c
	ic.dirty()
}

// SetRotate sets the static angle in degrees.
func (ic *Icon) SetRotate(deg float64) {
	if ic == nil {
		return
	}
	ic.rotate = deg
	ic.dirty()
}

// Rotate returns the static angle in degrees.
func (ic *Icon) Rotate() float64 {
	if ic == nil {
		return 0
	}
	return ic.rotate
}

// SetSpin enables rotation animation.
func (ic *Icon) SetSpin(b bool) {
	if ic == nil {
		return
	}
	ic.spin = b
	ic.dirty()
}

// Spin reports whether animation is enabled.
func (ic *Icon) Spin() bool { return ic != nil && ic.spin }

// SetReduceMotion freezes spin phase (accessibility).
func (ic *Icon) SetReduceMotion(b bool) {
	if ic == nil {
		return
	}
	ic.reduceMotion = b
}

// Phase returns the spin phase in [0,1).
func (ic *Icon) Phase() float64 {
	if ic == nil {
		return 0
	}
	return ic.phase
}

// EffectiveAngle returns rotate + phase*360 in degrees (§6.5).
func (ic *Icon) EffectiveAngle() float64 {
	if ic == nil {
		return 0
	}
	return ic.rotate + ic.phase*360
}

// Angle is an alias of EffectiveAngle (acceptance shorthand).
func (ic *Icon) Angle() float64 { return ic.EffectiveAngle() }

// SetTwoToneColor sets the two-tone primary (secondary derived).
func (ic *Icon) SetTwoToneColor(p render.RGBA) {
	if ic == nil {
		return
	}
	ic.twoTonePrimary = p
	ic.hasSecondary = false
	ic.dirty()
}

// SetTwoToneColors sets both two-tone colors.
func (ic *Icon) SetTwoToneColors(p, s render.RGBA) {
	if ic == nil {
		return
	}
	ic.twoTonePrimary = p
	ic.twoToneSecondary = s
	ic.hasSecondary = true
	ic.dirty()
}

// TwoToneColors returns the effective primary/secondary pair.
func (ic *Icon) TwoToneColors() (render.RGBA, render.RGBA) {
	if ic != nil && ic.hasSecondary {
		return ic.twoTonePrimary, ic.twoToneSecondary
	}
	p := GetTwoToneColorGlobal()
	if ic != nil && ic.twoTonePrimary.A > 0 {
		p = ic.twoTonePrimary
	}
	s := render.RGBA{R: p.R, G: p.G, B: p.B, A: p.A * 0.35}
	if s.A <= 0 && p.A > 0 {
		s.A = 0.35
	}
	return p, s
}

// SetPainter sets a custom glyph (non-nil wins over name).
func (ic *Icon) SetPainter(p Painter) {
	if ic == nil {
		return
	}
	ic.painter = p
	ic.dirty()
}

// SetProvider selects the theme source (nil selects process default).
func (ic *Icon) SetProvider(p *theme.Provider) {
	if ic == nil {
		return
	}
	ic.provider = p
	ic.dirty()
}

// SetTheme pins exact tokens (nil clears to provider).
func (ic *Icon) SetTheme(t *theme.Tokens) {
	if ic == nil {
		return
	}
	ic.override = t
	ic.dirty()
}

func (ic *Icon) themeTokens() theme.Tokens {
	if ic != nil && ic.override != nil {
		return *ic.override
	}
	if ic != nil && ic.provider != nil {
		return ic.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// EffectiveColor returns the single-tone color (§6.2.2 priority).
func (ic *Icon) EffectiveColor() render.RGBA {
	tok := ic.themeTokens()
	if ic != nil && ic.disabled {
		return themeToRGBA(tok.ColorTextDisabled)
	}
	if ic != nil && ic.color.A > 0 {
		return ic.color
	}
	return themeToRGBA(tok.ColorText)
}

// SetAriaLabel makes the icon meaningful (empty keeps it decorative).
func (ic *Icon) SetAriaLabel(s string) {
	if ic == nil {
		return
	}
	ic.ariaLabel = s
}

// AriaLabel returns the accessible name ("" means decorative).
func (ic *Icon) AriaLabel() string {
	if ic == nil {
		return ""
	}
	return ic.ariaLabel
}

// Role returns "img" for named icons, "" for decorative ones.
func (ic *Icon) Role() string {
	if ic != nil && ic.ariaLabel != "" {
		return "img"
	}
	return ""
}

// SetDecorative sets the decorative flag (default true).
func (ic *Icon) SetDecorative(b bool) {
	if ic == nil {
		return
	}
	ic.decorative = b
}

// Decorative reports the decorative flag.
func (ic *Icon) Decorative() bool { return ic == nil || ic.decorative }

// Focusable is always false: pure Icon never takes Tab (hosts own clicks).
func (ic *Icon) Focusable() bool { return false }

// SetDisabled toggles the disabled appearance.
func (ic *Icon) SetDisabled(b bool) {
	if ic == nil {
		return
	}
	ic.disabled = b
	ic.dirty()
}

// Disabled reports the disabled flag.
func (ic *Icon) Disabled() bool { return ic != nil && ic.disabled }

// SetClassName stores the semantic hook (no CSS engine).
func (ic *Icon) SetClassName(s string) {
	if ic == nil {
		return
	}
	ic.className = s
}

// ClassName returns the stored hook.
func (ic *Icon) ClassName() string {
	if ic == nil {
		return ""
	}
	return ic.className
}

// SetRegistry selects the glyph source (nil selects Global).
func (ic *Icon) SetRegistry(r *Registry) {
	if ic == nil {
		return
	}
	ic.registry = r
	ic.dirty()
}

func (ic *Icon) effectiveRegistry() *Registry {
	if ic != nil && ic.registry != nil {
		return ic.registry
	}
	return Global
}

func (ic *Icon) resolveDef() (Def, bool) {
	if ic == nil {
		return Def{}, false
	}
	if d, ok := ic.family.lookupDef(ic.name); ok {
		return d, true
	}
	return ic.effectiveRegistry().Lookup(ic.name)
}

// DefTag returns the registering source tag ("" when unknown).
func (ic *Icon) DefTag() string {
	d, ok := ic.resolveDef()
	if !ok {
		return ""
	}
	return d.Tag
}

// Known reports whether name resolves (or a painter wins).
func (ic *Icon) Known() bool {
	if ic == nil {
		return false
	}
	if ic.painter != nil {
		return true
	}
	if _, ok := ic.family.lookupPainter(ic.name); ok {
		return true
	}
	_, ok := ic.resolveDef()
	return ok
}

// Node returns the tree node (layout/paint/hit through it).
func (ic *Icon) Node() rendering.RenderObject {
	if ic == nil {
		return nil
	}
	ic.syncNode()
	return ic.node
}

// Layout sizes the node to EffectiveSize under constraints.
func (ic *Icon) Layout(c rendering.Constraints) rendering.Size {
	if ic == nil {
		return rendering.Size{}
	}
	ic.syncNode()
	return ic.node.Layout(c)
}

// Attach registers the spin ticker.
func (ic *Icon) Attach(reg *scheduler.TickerRegistry) {
	if ic == nil || reg == nil {
		return
	}
	if ic.attached != nil && ic.attached != reg {
		ic.attached.Remove(ic)
	}
	ic.attached = reg
	reg.Add(ic)
}

// Detach unregisters the spin ticker.
func (ic *Icon) Detach() {
	if ic == nil || ic.attached == nil {
		return
	}
	ic.attached.Remove(ic)
	ic.attached = nil
}

// Tick advances spin phase (scheduler.Ticker). Idle icons auto-drop.
func (ic *Icon) Tick(dt float64) bool {
	if ic == nil {
		return false
	}
	if !ic.spin || ic.reduceMotion {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	ic.phase = math.Mod(ic.phase+dt/spinPeriodSec, 1)
	if ic.phase < 0 {
		ic.phase++
	}
	ic.dirty()
	return true
}

// WantsFrame reports frame demand (scheduler.FrameWanter).
func (ic *Icon) WantsFrame() bool {
	return ic != nil && ic.spin && !ic.reduceMotion
}

func (ic *Icon) syncNode() {
	if ic == nil || ic.node == nil {
		return
	}
	s := ic.EffectiveSize()
	ic.node.FixedWidth = s
	ic.node.FixedHeight = s
}

func (ic *Icon) dirty() {
	if ic == nil || ic.node == nil {
		return
	}
	ic.syncNode()
	ic.node.MarkNeedsPaint()
}

func (ic *Icon) paint(pc *rendering.PaintContext, size float64) {
	if pc == nil || size <= 0 {
		return
	}
	base := ic.EffectiveColor()
	primary, secondary := ic.TwoToneColors()
	cx, cy := size/2, size/2
	pc.Save()
	pc.RotateAbout(ic.EffectiveAngle()*math.Pi/180, cx, cy)
	if ic.painter != nil {
		ic.painter(pc, size, primary, secondary)
		pc.RestoreCanvas()
		return
	}
	if p, ok := ic.family.lookupPainter(ic.name); ok && p != nil {
		p(pc, size, primary, secondary)
		pc.RestoreCanvas()
		return
	}
	d, ok := ic.resolveDef()
	if !ok {
		drawPlaceholder(pc, size, base)
		pc.RestoreCanvas()
		return
	}
	drawGlyph(pc, ic.name, size, base, primary, secondary, d.TwoTone)
	pc.RestoreCanvas()
}

// PaintGlyph draws one registered glyph in a size×size box whose top-left
// is (x,y) in pc local space, rotated angleDeg about its center. Button
// leading icons/spinners reuse this so they match Icon pixel-for-pixel.
// Unknown names draw the placeholder (never blank, never a black bar).
func PaintGlyph(pc *rendering.PaintContext, name string, x, y, size float64, c render.RGBA, angleDeg float64) {
	if pc == nil || pc.DC == nil || size <= 0 {
		return
	}
	child := pc.WithOrigin(pc.OriginX+x, pc.OriginY+y)
	child.Save()
	if angleDeg != 0 {
		child.RotateAbout(angleDeg*math.Pi/180, size/2, size/2)
	}
	if d, ok := Global.Lookup(name); ok {
		drawGlyph(child, name, size, c, c, c, d.TwoTone)
	} else {
		// Family painters / unknown: placeholder keeps ink visible.
		drawPlaceholder(child, size, c)
	}
	child.RestoreCanvas()
}
