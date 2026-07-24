package kit

import (
	"sync"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Icon defaults — https://ant.design/components/icon
// Product size is ~1em; kit uses a fixed logical default (docs/antd/icon.md §6.2).
const (
	// DefaultIconSize is the unresolved Size fallback (antd style.fontSize ≈ 1em product default).
	DefaultIconSize = 16.0
	// iconSpinRPS is turns per second while Spin is true.
	iconSpinRPS = 1.0
)

// IconPainter is the antd `component` equivalent: custom glyph in [0,size]².
// primary/secondary are resolved colors (secondary may be transparent).
type IconPainter = primitive.IconPaintFn

// Icon is the product Icon control (docs/antd/icon.md §6).
//
// Structure:
//
//	iconHost (mount/unmount ticker)
//	  └─ primitive.Icon  (glyph / painter, rotate + spin phase)
//
// hit == layout == paint; decorative by default (HitDefer, not in Tab).
type Icon struct {
	Root  *iconHost
	Glyph *primitive.Icon

	Name      string
	Size      float64 // 0 → DefaultIconSize
	Color     render.RGBA
	Rotate    float64 // degrees
	Spin      bool
	Theme     *core.Theme
	ClassName string
	AriaLabel string
	// Decorative defaults true (aria-hidden equivalent). false + AriaLabel → meaningful img.
	Decorative bool
	Disabled   bool

	// Two-tone (antd twoToneColor).
	twoTonePrimary   render.RGBA
	twoToneSecondary render.RGBA
	twoToneSet       bool

	painter  IconPainter
	registry *primitive.IconRegistry

	spinPhase float64
	life      tickerLifecycle
	boundTree *core.Tree
}

// iconHost owns mount lifecycle for spin Ticker (demand-frame ANIMATING).
type iconHost struct {
	primitive.RepaintBoundary
	ic *Icon
}

func (h *iconHost) TypeID() string { return TypeIcon }

func (h *iconHost) OnMount() {
	if h == nil || h.ic == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.ic.boundTree = t
		h.ic.life.attach(t, h.ic, h.ic.Spin)
	}
}

func (h *iconHost) OnUnmount() {
	if h != nil && h.ic != nil {
		h.ic.life.unmount()
		h.ic.boundTree = nil
	}
}

// NewIcon creates a kit icon by registry name (antd named icon).
func NewIcon(name string) *Icon {
	ic := &Icon{
		Name:       name,
		Decorative: true,
	}
	ic.rebuild()
	return ic
}

// Node returns the root node.
func (ic *Icon) Node() core.Node {
	if ic == nil {
		return nil
	}
	if ic.Root == nil {
		ic.rebuild()
	}
	return ic.Root
}

// ChromeNode returns the glyph node (primitive.Icon).
func (ic *Icon) ChromeNode() core.Node {
	if ic == nil {
		return nil
	}
	if ic.Glyph == nil {
		ic.rebuild()
	}
	return ic.Glyph
}

// SetName changes the icon registry key.
func (ic *Icon) SetName(name string) {
	if ic == nil {
		return
	}
	ic.Name = name
	if ic.Glyph != nil {
		ic.Glyph.Name = name
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetSize sets edge length in logical px. 0 → DefaultIconSize on next rebuild/sync.
func (ic *Icon) SetSize(s float64) {
	if ic == nil {
		return
	}
	ic.Size = s
	ic.syncGlyphMetrics()
	if ic.Glyph != nil {
		ic.Glyph.MarkNeedsLayout()
	}
	if ic.Root != nil {
		ic.Root.MarkNeedsLayout()
	}
}

// SetColor overrides theme text color when A>0.
func (ic *Icon) SetColor(c render.RGBA) {
	if ic == nil {
		return
	}
	ic.Color = c
	ic.syncGlyphPaint()
	if ic.Glyph != nil {
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetRotate sets static rotation in degrees (antd rotate).
func (ic *Icon) SetRotate(deg float64) {
	if ic == nil {
		return
	}
	ic.Rotate = deg
	if ic.Glyph != nil {
		ic.Glyph.RotateDeg = deg
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetSpin enables continuous rotation via Tree ticker (antd spin).
func (ic *Icon) SetSpin(v bool) {
	if ic == nil {
		return
	}
	ic.Spin = v
	ic.life.setActive(v)
	if ic.boundTree != nil {
		ic.boundTree.BindTicker(ic, v)
	}
	if ic.Glyph != nil {
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetTwoToneColor sets the two-tone primary color (secondary derived).
func (ic *Icon) SetTwoToneColor(primary render.RGBA) {
	if ic == nil {
		return
	}
	ic.twoTonePrimary = primary
	ic.twoToneSecondary = render.RGBA{}
	ic.twoToneSet = true
	ic.syncGlyphPaint()
	if ic.Glyph != nil {
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetTwoToneColors sets primary and secondary two-tone colors.
func (ic *Icon) SetTwoToneColors(primary, secondary render.RGBA) {
	if ic == nil {
		return
	}
	ic.twoTonePrimary = primary
	ic.twoToneSecondary = secondary
	ic.twoToneSet = true
	ic.syncGlyphPaint()
	if ic.Glyph != nil {
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetPainter sets a custom paint callback (antd component). Non-nil wins over name.
func (ic *Icon) SetPainter(fn IconPainter) {
	if ic == nil {
		return
	}
	ic.painter = fn
	if ic.Glyph != nil {
		ic.Glyph.PaintCustom = fn
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetTheme sets an explicit theme override.
func (ic *Icon) SetTheme(th *core.Theme) {
	if ic == nil {
		return
	}
	ic.Theme = th
	ic.syncGlyphPaint()
	if ic.Glyph != nil {
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetAriaLabel marks a meaningful image (Role=img). Empty keeps decorative semantics.
func (ic *Icon) SetAriaLabel(name string) {
	if ic == nil {
		return
	}
	ic.AriaLabel = name
	ic.applyA11y()
}

// SetDecorative toggles decorative vs meaningful. Default true.
func (ic *Icon) SetDecorative(v bool) {
	if ic == nil {
		return
	}
	ic.Decorative = v
	ic.applyA11y()
}

// SetDisabled tints with disabled text token when true.
func (ic *Icon) SetDisabled(v bool) {
	if ic == nil {
		return
	}
	ic.Disabled = v
	ic.syncGlyphPaint()
	if ic.Glyph != nil {
		ic.Glyph.MarkNeedsPaint()
	}
}

// SetClassName stores a semantic class hook (no CSS engine).
func (ic *Icon) SetClassName(c string) {
	if ic != nil {
		ic.ClassName = c
	}
}

// SetRegistry overrides the glyph registry (default GlobalIcons).
func (ic *Icon) SetRegistry(r *primitive.IconRegistry) {
	if ic == nil {
		return
	}
	ic.registry = r
	if ic.Glyph != nil {
		ic.Glyph.Registry = r
		ic.Glyph.MarkNeedsPaint()
	}
}

// Known reports whether Name is registered (false for empty name with painter-only).
func (ic *Icon) Known() bool {
	if ic == nil || ic.Name == "" {
		return ic != nil && ic.painter != nil
	}
	reg := ic.registry
	if reg == nil {
		reg = primitive.GlobalIcons
	}
	_, ok := reg.Lookup(ic.Name)
	return ok
}

// EffectiveSize returns resolved edge length.
func (ic *Icon) EffectiveSize() float64 {
	if ic == nil {
		return DefaultIconSize
	}
	if ic.Size > 0 {
		return ic.Size
	}
	return DefaultIconSize
}

// EffectiveColor returns the color used for monochrome glyphs.
func (ic *Icon) EffectiveColor() render.RGBA {
	if ic == nil {
		return render.RGBA{}
	}
	th := ic.theme()
	if ic.Disabled {
		return th.Color(core.TokenColorDisabledText)
	}
	if ic.Color.A > 0 {
		return ic.Color
	}
	return th.Color(core.TokenColorText)
}

// EffectiveAngle returns rotate + spinPhase*360 in degrees.
func (ic *Icon) EffectiveAngle() float64 {
	if ic == nil {
		return 0
	}
	return ic.Rotate + ic.spinPhase*360
}

// AttachTicker registers spin animation on the demand-frame loop.
func (ic *Icon) AttachTicker(t *core.Tree) {
	if ic == nil || t == nil {
		return
	}
	ic.boundTree = t
	ic.life.attach(t, ic, ic.Spin)
}

// Tick advances spin phase. Implements core.Ticker.
func (ic *Icon) Tick(dt float64) (still bool) {
	if ic == nil || !ic.Spin {
		return false
	}
	var nt *core.Tree
	if ic.Root != nil {
		nt = ic.Root.Tree()
	}
	if !ic.life.stillMounted(nt) {
		return false
	}
	// reduced-motion: freeze phase.
	if tree := ic.boundTree; tree != nil {
		if tree.Clock() != nil && tree.Clock().ReduceMotion {
			return true
		}
	} else if nt != nil && nt.Clock() != nil && nt.Clock().ReduceMotion {
		return true
	}
	ic.spinPhase += dt * iconSpinRPS
	if ic.spinPhase >= 1 {
		ic.spinPhase -= float64(int(ic.spinPhase))
	}
	if ic.Glyph != nil {
		ic.Glyph.SpinPhase = ic.spinPhase
		ic.Glyph.MarkNeedsPaint()
	} else if ic.Root != nil {
		ic.Root.MarkNeedsPaint()
	}
	return true
}

// SpinPhase returns the current [0,1) spin phase (tests).
func (ic *Icon) SpinPhase() float64 {
	if ic == nil {
		return 0
	}
	return ic.spinPhase
}

func (ic *Icon) theme() *core.Theme {
	var n core.Node
	if ic.Root != nil {
		n = ic.Root
	} else if ic.Glyph != nil {
		n = ic.Glyph
	}
	return themeOf(ic.Theme, n)
}

func (ic *Icon) applyA11y() {
	if ic == nil || ic.Root == nil {
		return
	}
	b := ic.Root.Base()
	if ic.AriaLabel != "" && !ic.Decorative {
		b.Role = "img"
		b.Label = ic.AriaLabel
	} else if ic.AriaLabel != "" {
		// Meaningful even if Decorative left true: label wins.
		b.Role = "img"
		b.Label = ic.AriaLabel
		ic.Decorative = false
	} else {
		b.Role = ""
		b.Label = ""
	}
}

func (ic *Icon) syncGlyphMetrics() {
	if ic == nil || ic.Glyph == nil {
		return
	}
	ic.Glyph.Size = ic.EffectiveSize()
	if ic.Root != nil {
		// Host sizes to child via Fit/layout of single child.
		ic.Root.MarkNeedsLayout()
	}
}

func (ic *Icon) syncGlyphPaint() {
	if ic == nil || ic.Glyph == nil {
		return
	}
	ic.Glyph.Color = ic.EffectiveColor()
	ic.Glyph.RotateDeg = ic.Rotate
	ic.Glyph.SpinPhase = ic.spinPhase
	ic.Glyph.PaintCustom = ic.painter
	ic.Glyph.Registry = ic.registry
	if ic.twoToneSet {
		ic.Glyph.TwoTonePrimary = ic.twoTonePrimary
		ic.Glyph.TwoToneSecondary = ic.twoToneSecondary
	} else if ic.isTwoToneName() {
		// Global two-tone only for two-tone registry glyphs (antd setTwoToneColor).
		ic.Glyph.TwoTonePrimary = GetTwoToneColorGlobal()
		ic.Glyph.TwoToneSecondary = render.RGBA{}
	} else {
		ic.Glyph.TwoTonePrimary = render.RGBA{}
		ic.Glyph.TwoToneSecondary = render.RGBA{}
	}
}

// isTwoToneName reports whether the registry def is marked two-tone.
func (ic *Icon) isTwoToneName() bool {
	if ic == nil || ic.Name == "" || ic.painter != nil {
		return false
	}
	reg := ic.registry
	if reg == nil {
		reg = primitive.GlobalIcons
	}
	def, ok := reg.Lookup(ic.Name)
	return ok && def.TwoTone
}

func (ic *Icon) rebuild() {
	if ic == nil {
		return
	}
	sz := ic.EffectiveSize()
	g := primitive.NewIcon(ic.Name)
	g.Size = sz
	g.Hit = core.HitDefer
	ic.Glyph = g

	host := &iconHost{ic: ic}
	host.Init(host)
	host.Hit = core.HitDefer
	host.SetRepaintBoundary(true)
	host.AddChild(g)
	ic.Root = host

	ic.syncGlyphPaint()
	ic.applyA11y()
}

// ── Global two-tone (antd setTwoToneColor / getTwoToneColor) ──────────────

var (
	twoToneMu     sync.RWMutex
	twoToneGlobal render.RGBA
	twoToneInited bool
)

// SetTwoToneColorGlobal sets the process-wide two-tone primary (antd setTwoToneColor).
func SetTwoToneColorGlobal(c render.RGBA) {
	twoToneMu.Lock()
	defer twoToneMu.Unlock()
	twoToneGlobal = c
	twoToneInited = true
}

// GetTwoToneColorGlobal returns the global two-tone primary.
// Default: Theme primary blue-ish seed when unset (resolved lazily as opaque primary-like).
func GetTwoToneColorGlobal() render.RGBA {
	twoToneMu.RLock()
	defer twoToneMu.RUnlock()
	if twoToneInited && twoToneGlobal.A > 0 {
		return twoToneGlobal
	}
	// Fallback without Theme pointer: Ant default primary-ish.
	return render.RGBA{R: 0.09, G: 0.42, B: 0.93, A: 1} // ~#1677ff
}

// ── Offline iconfont family (antd createFromIconfontCN) ───────────────────

// IconfontOptions configures an offline iconfont family.
// Sources mirror antd scriptUrl[] load order: later sources override same type names.
type IconfontOptions struct {
	// Sources are logical source IDs (not network URLs). Register icons per source
	// with RegisterIconSource or family.Register.
	Sources []string
	// Extra is reserved (antd extraCommonProps) — unused in P0.
}

// IconfontFamily is the offline createFromIconfontCN product.
type IconfontFamily struct {
	sources []string
	reg     *primitive.IconRegistry
}

var (
	iconSourcesMu sync.RWMutex
	iconSources   = map[string]map[string]primitive.IconDef{}
)

// RegisterIconSource registers offline icons under a source ID (antd multi-scriptUrl shard).
func RegisterIconSource(sourceID string, icons map[string]primitive.IconDef) {
	if sourceID == "" || icons == nil {
		return
	}
	iconSourcesMu.Lock()
	defer iconSourcesMu.Unlock()
	cp := make(map[string]primitive.IconDef, len(icons))
	for k, v := range icons {
		cp[k] = v
	}
	iconSources[sourceID] = cp
}

// CreateFromIconfont builds an offline iconfont family (antd createFromIconfontCN).
// Does **not** fetch remote scriptUrl; merge registered sources in order.
// Built-in GlobalIcons names remain available; source types overlay same names later-wins.
func CreateFromIconfont(opts IconfontOptions) *IconfontFamily {
	f := &IconfontFamily{
		sources: append([]string(nil), opts.Sources...),
	}
	f.rebuildReg()
	return f
}

func (f *IconfontFamily) rebuildReg() {
	if f == nil {
		return
	}
	// Fresh registry with builtins, then overlay sources in order (later wins).
	f.reg = primitive.NewIconRegistry()
	iconSourcesMu.RLock()
	defer iconSourcesMu.RUnlock()
	for _, src := range f.sources {
		if m, ok := iconSources[src]; ok {
			for name, def := range m {
				f.reg.Register(name, def)
			}
		}
	}
}

// Register adds/replaces a type name on this family (and its first source if any).
func (f *IconfontFamily) Register(typeName string, def primitive.IconDef) {
	if f == nil || typeName == "" {
		return
	}
	if f.reg == nil {
		f.rebuildReg()
	}
	f.reg.Register(typeName, def)
	if len(f.sources) > 0 {
		src := f.sources[len(f.sources)-1]
		iconSourcesMu.Lock()
		if iconSources[src] == nil {
			iconSources[src] = map[string]primitive.IconDef{}
		}
		iconSources[src][typeName] = def
		iconSourcesMu.Unlock()
	}
}

// NewIcon creates an Icon bound to this family's registry (antd <IconFont type=… />).
func (f *IconfontFamily) NewIcon(typeName string) *Icon {
	if f == nil {
		return NewIcon(typeName)
	}
	if f.reg == nil {
		f.rebuildReg()
	}
	ic := NewIcon(typeName)
	ic.SetRegistry(f.reg)
	return ic
}

// Registry returns the merged family registry.
func (f *IconfontFamily) Registry() *primitive.IconRegistry {
	if f == nil {
		return primitive.GlobalIcons
	}
	if f.reg == nil {
		f.rebuildReg()
	}
	return f.reg
}
