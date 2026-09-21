package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

// QRCodeInstance owns the encoded matrices plus status/cover state.
type QRCodeInstance struct {
	props     QRCodeProps
	ctx       scope.Ctx
	gen       QRCodeGenerateConfig
	mu        sync.Mutex
	values    []string
	matrix    [][]bool
	used      QRErrorLevel
	encodeErr error
	status    QRCodeStatus
	spin      float64

	ctlStatus bool
	mounted   bool

	onRefresh func()
}

// QRCodeMatrix is a snapshot copy of the module matrix.
type QRCodeMatrix [][]bool

func newQRCodeInstance(ctx scope.Ctx, props QRCodeProps, gen QRCodeGenerateConfig) *QRCodeInstance {
	if gen == nil {
		gen = DefaultQRCodeGenerateConfig()
	}
	in := &QRCodeInstance{props: props, ctx: ctx.Normalize(), gen: gen}
	in.status = props.Status
	if in.status == "" {
		in.status = QRCodeStatusActive
	}
	in.reencodeLocked()
	return in
}

// reencodeLocked encodes the first value (P0 single path) and records
// every input for the P1 multi path. Caller must hold in.mu.
func (in *QRCodeInstance) reencodeLocked() {
	in.values = ResolveQRCodeValues(in.props)
	in.matrix = nil
	in.encodeErr = nil
	in.used = in.props.ErrorLevel
	if in.used == "" {
		in.used = QRErrorLevelM
	}
	if len(in.values) == 0 {
		return
	}
	boost := true
	if in.props.BoostLevelSet {
		boost = in.props.BoostLevel
	}
	m, used, err := EncodeQRCodeValue(in.gen, in.values[0], in.used, boost)
	in.matrix, in.used, in.encodeErr = m, used, err
}

// Mount marks the host live.
func (in *QRCodeInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots, re-encoding only when the encode
// inputs (values, level, boost) changed. Theme/status-only updates skip
// the encode, which dominates Update cost on long content.
func (in *QRCodeInstance) Update(ctx scope.Ctx, next QRCodeProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	if !sameQRCodeEncodeInput(in.props, next) {
		in.props = next
		in.reencodeLocked()
	} else {
		in.props = next
	}
	if !in.ctlStatus {
		in.status = next.Status
		if in.status == "" {
			in.status = QRCodeStatusActive
		}
	}
}

// sameQRCodeEncodeInput reports whether two prop sets encode identically.
func sameQRCodeEncodeInput(a, b QRCodeProps) bool {
	if a.ErrorLevel != b.ErrorLevel || a.BoostLevel != b.BoostLevel ||
		a.BoostLevelSet != b.BoostLevelSet || a.ValuesSet != b.ValuesSet ||
		a.Value != b.Value || len(a.Values) != len(b.Values) {
		return false
	}
	for i := range a.Values {
		if a.Values[i] != b.Values[i] {
			return false
		}
	}
	return true
}

// Unmount marks the host dead.
func (in *QRCodeInstance) Unmount() {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.mounted = false
}

// SetState runs f under the lock (sole state mutation gate).
func (in *QRCodeInstance) SetState(f func(*QRCodeInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

// Generate returns the bound implementation.
func (in *QRCodeInstance) Generate() QRCodeGenerateConfig {
	if in == nil {
		return DefaultQRCodeGenerateConfig()
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.gen
}

// Locale resolves props locale over Ctx locale.
func (in *QRCodeInstance) Locale() string {
	if in == nil {
		return "en-US"
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.ctx.Locale != "" {
		return in.ctx.Locale
	}
	return "en-US"
}

// SetOnRefresh registers the expired-refresh callback.
func (in *QRCodeInstance) SetOnRefresh(fn func()) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onRefresh = fn
	if in.props.OnRefresh != nil {
		// Props callback wins when both are set.
		in.onRefresh = in.props.OnRefresh
	}
}

// SetValue drives the value from outside and re-encodes.
func (in *QRCodeInstance) SetValue(v string) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.props.Value = v
	in.props.Values = nil
	in.props.ValuesSet = false
	in.reencodeLocked()
}

// SetStatus drives controlled status from outside.
func (in *QRCodeInstance) SetStatus(s QRCodeStatus) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctlStatus = true
	in.status = s
}

// Status returns the current cover state.
func (in *QRCodeInstance) Status() QRCodeStatus {
	if in == nil {
		return QRCodeStatusActive
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.status
}

// SetCoverStatus flips active/expired/loading/scanned (uncontrolled).
func (in *QRCodeInstance) SetCoverStatus(s QRCodeStatus) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.ctlStatus {
		in.status = s
	}
}

// Matrix returns a snapshot copy of the first-value matrix.
// Empty value yields nil (antd renders null, never crashes).
func (in *QRCodeInstance) Matrix() [][]bool {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.matrix) == 0 {
		return nil
	}
	out := make([][]bool, len(in.matrix))
	for i, row := range in.matrix {
		out[i] = append([]bool(nil), row...)
	}
	return out
}

// Modules reports the matrix edge (0 when empty). Always >= 21 live.
func (in *QRCodeInstance) Modules() int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return len(in.matrix)
}

// UsedLevel reports the effective error level after boost.
func (in *QRCodeInstance) UsedLevel() QRErrorLevel {
	if in == nil {
		return QRErrorLevelM
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.used
}

// EncodeError reports the last encode failure, if any.
func (in *QRCodeInstance) EncodeError() error {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.encodeErr
}

// Values returns the resolved encode inputs.
func (in *QRCodeInstance) Values() []string {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return append([]string(nil), in.values...)
}

// HasCover reports whether a cover overlays the matrix.
func (in *QRCodeInstance) HasCover() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.status != QRCodeStatusActive
}

// HasIcon reports whether a center icon is configured.
func (in *QRCodeInstance) HasIcon() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.props.Icon != ""
}

// ClickRefresh fires onRefresh exactly once per click (expired only).
func (in *QRCodeInstance) ClickRefresh() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if in.status != QRCodeStatusExpired {
		in.mu.Unlock()
		return false
	}
	cb := in.onRefresh
	if cb == nil {
		cb = in.props.OnRefresh
	}
	in.mu.Unlock()
	if cb == nil {
		return false
	}
	cb()
	return true
}

// Tick advances the loading spinner (host drives the clock).
func (in *QRCodeInstance) Tick(dt float64) {
	if in == nil || dt <= 0 {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.spin += dt * 360
	for in.spin >= 360 {
		in.spin -= 360
	}
}

// SpinAngle reports the loading spinner angle in degrees.
func (in *QRCodeInstance) SpinAngle() float64 {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.spin
}

// CoverText renders the default cover copy for the current status.
// StatusRender hook wins when set (P0 string form).
func (in *QRCodeInstance) CoverText() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	locale := "en-US"
	if in.ctx.Locale != "" {
		locale = in.ctx.Locale
	}
	expired, _, scanned := ResolveQRCodeLocale(locale)
	info := QRCodeStatusInfo{Status: in.status, Expired: expired, Scanned: scanned}
	if refreshCB := in.refreshLocked(); refreshCB != nil {
		info.OnRefresh = refreshCB
	}
	_, refresh, _ := ResolveQRCodeLocale(locale)
	info.Refresh = refresh
	if in.props.StatusRender != nil {
		return in.props.StatusRender(info)
	}
	switch in.status {
	case QRCodeStatusExpired:
		return expired + " " + refresh
	case QRCodeStatusScanned:
		return scanned
	case QRCodeStatusLoading:
		return ""
	default:
		return ""
	}
}

func (in *QRCodeInstance) refreshLocked() func() {
	if in.onRefresh != nil {
		return in.onRefresh
	}
	return in.props.OnRefresh
}

// HolderContent renders static-call holder copy through Ctx.
func (in *QRCodeInstance) HolderContent() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	ctx := in.ctx
	in.mu.Unlock()
	if ctx.HolderRender == nil {
		return "qrcode holder"
	}
	return ctx.HolderRender("qrcode")
}
