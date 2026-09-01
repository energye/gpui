//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	dbusServiceIBus   = "org.freedesktop.IBus"
	dbusPathIBusBus   = "/org/freedesktop/IBus"
	dbusIfaceIBus     = "org.freedesktop.IBus"
	dbusIfaceIBusCtx  = "org.freedesktop.IBus.InputContext"
	dbusServiceFcitx5 = "org.fcitx.Fcitx5"
	dbusPathFcitx5IM  = "/org/fcitx/Fcitx5/InputMethod"
	dbusIfaceFcitx5IM = "org.fcitx.Fcitx5.InputMethod"
	dbusServiceFcitx  = "org.fcitx.Fcitx"
	dbusPathFcitxIM   = "/org/fcitx/Fcitx/InputMethod"
	dbusIfaceFcitxIM  = "org.fcitx.Fcitx.InputMethod"
	dbusServiceDBus   = "org.freedesktop.DBus"

	ibusCaps  uint32 = 1<<0 | 1<<3 | 1<<5 // 41: PREEDIT|FOCUS|SURROUNDING (ibustypes.h)
	fcitxCaps uint32 = 16                 // CAPACITY_PREEDIT|SURROUNDING

	defaultCursorW = 2
	defaultCursorH = 16

	syntheticFcitxPrefix = "/org/fcitx/Fcitx/InputContext_"
)

var dbusMatchRules = []string{
	"type='signal',sender='org.freedesktop.IBus'",
	"type='signal',sender='org.fcitx.Fcitx5'",
	"type='signal',sender='org.fcitx.Fcitx'",
	"type='signal',sender='org.freedesktop.DBus',interface='org.freedesktop.DBus',member='NameOwnerChanged'",
}

// x11Ime 是 X11 D-Bus IME 的 S1+S2+S3 实现：会话总线 + 上下文 + 会话与锚点
// S1: 单 Conn 复用 + AddMatch + 无守护降级 + GPUI_IME_DEBUG
// S2: 探针顺序 + CreateInputContext 双引擎 + SetCapabilities + 异步化 + per-window Destroy
// S3: FocusIn/Out幂等 + SetCursorLocation/Rect + SetSurroundingText 4000 + SetContentType + XTranslateCoordinates
// S5: 信号归一 + AttrList
type x11Ime struct {
	conn       *dbus.Conn
	engine     string
	objectPath dbus.ObjectPath
	imeDirty   bool
	focused    bool
	mu         sync.Mutex
	host       *x11Host
	lastRect   Rect
	hasRect    bool
	purpose    ContentPurpose
	closed     bool
	composing  bool
	lastText   string
	lastCursor int
	lastAnchor int
	sigCh      chan *dbus.Signal
	sigStop    chan struct{}
	// B9: last preedit segments for debug / future DrawPreedit
	lastSegs []ImeSegment
	// B2/B4 退避：探测全失败时记录，下次 ensureReprobe 需间隔
	lastProbeFail time.Time
}

// 进程内单 dbus.Conn 复用（S1 核心：多窗口恒为 1 连接）。
var (
	x11SharedMu   sync.Mutex
	x11SharedConn *dbus.Conn
	x11SharedErr  error
	x11SharedOnce sync.Once
)

// S6: 全局 IME 集合用于 NameOwnerChanged 热切
var (
	x11ImesMu sync.Mutex
	x11Imes   = make(map[*x11Ime]struct{})
)

func x11RegisterIme(im *x11Ime) {
	if im == nil {
		return
	}
	x11ImesMu.Lock()
	x11Imes[im] = struct{}{}
	x11ImesMu.Unlock()
}

func x11UnregisterIme(im *x11Ime) {
	if im == nil {
		return
	}
	x11ImesMu.Lock()
	delete(x11Imes, im)
	empty := len(x11Imes) == 0
	x11ImesMu.Unlock()
	if empty {
		x11SharedMu.Lock()
		c := x11SharedConn
		x11SharedMu.Unlock()
		if c != nil {
			// Clean all 4 global matches (was [:2] leaked fcitx+NameOwnerChanged)
			for _, rule := range dbusMatchRules {
				_ = c.BusObject().Call(dbusServiceDBus+".RemoveMatch", 0, rule).Err
			}
		}
	}
}

func x11MarkDirtyForOwnerChange(name, oldOwner, newOwner string) {
	if name != dbusServiceIBus && name != dbusServiceFcitx5 && name != dbusServiceFcitx {
		return
	}
	x11ImesMu.Lock()
	imes := make([]*x11Ime, 0, len(x11Imes))
	for im := range x11Imes {
		imes = append(imes, im)
	}
	x11ImesMu.Unlock()

	type pendingDestroy struct {
		eng string
		obj dbus.ObjectPath
		c   *dbus.Conn
	}
	var toDestroy []pendingDestroy
	var toFocusOut []*x11Ime
	for _, im := range imes {
		im.mu.Lock()
		if oldOwner != "" && newOwner == "" {
			x11ImeDebug("NameOwnerChanged %s %q->%q imeDirty=true FocusOut", name, oldOwner, newOwner)
			im.imeDirty = true
			wasFocused := im.focused
			im.focused = false
			im.composing = false
			oldObj := im.objectPath
			oldEng := im.engine
			oldConn := im.conn
			im.objectPath = ""
			im.engine = ""
			if oldObj != "" {
				toDestroy = append(toDestroy, pendingDestroy{eng: oldEng, obj: oldObj, c: oldConn})
			}
			// stop old signal loop tied to dead daemon
			ch := im.sigCh
			stop := im.sigStop
			im.sigCh = nil
			im.sigStop = nil
			im.mu.Unlock()
			if ch != nil && oldConn != nil {
				oldConn.RemoveSignal(ch)
				close(ch)
			}
			if stop != nil {
				close(stop)
			}
			if wasFocused {
				toFocusOut = append(toFocusOut, im)
			}
		} else if oldOwner == "" && newOwner != "" {
			x11ImeDebug("NameOwnerChanged %s %q->%q imeDirty=true", name, oldOwner, newOwner)
			im.imeDirty = true
			im.mu.Unlock()
		} else {
			im.mu.Unlock()
		}
	}
	for _, d := range toDestroy {
		// Best-effort destroy on old conn (may already be dead, log anyway)
		if d.c != nil && d.obj != "" && !isSyntheticPath(d.obj) {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			if d.eng == "ibus" {
				o := d.c.Object(dbusServiceIBus, d.obj)
				_ = o.CallWithContext(ctx, dbusIfaceIBusCtx+".Destroy", 0).Err
			} else {
				for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
					o := d.c.Object(svc, d.obj)
					_ = o.CallWithContext(ctx, dbusIfaceFcitx5IM+".DestroyIC", 0).Err
				}
			}
			cancel()
			x11ImeDebug("destroy leaked ctx %s engine=%s on daemon gone", d.obj, d.eng)
		}
	}
	for _, im := range toFocusOut {
		im.callFocusOut()
	}
}

func x11ImeDebug(format string, args ...any) {
	if os.Getenv("GPUI_IME_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "[ime-x11] "+format+"\n", args...)
}

func sharedDBusConn() (*dbus.Conn, error) {
	x11SharedOnce.Do(func() {
		x11ImeDebug("dbus dial start addr=%q", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
		c, err := dbus.SessionBus()
		if err != nil {
			x11ImeDebug("dbus dial/hello failed: %v", err)
			x11SharedErr = err
			return
		}
		x11ImeDebug("dbus hello ok")
		for _, rule := range dbusMatchRules {
			if err := c.BusObject().Call(dbusServiceDBus+".AddMatch", 0, rule).Err; err != nil {
				x11ImeDebug("dbus AddMatch %q failed: %v", rule, err)
			} else {
				x11ImeDebug("dbus AddMatch %q ok", rule)
			}
		}
		x11ImeDebug("dbus match ok")
		x11SharedMu.Lock()
		x11SharedConn = c
		x11SharedMu.Unlock()
		go x11GlobalNameOwnerLoop(c)
		go x11BusWatchLoop(c)
	})
	x11SharedMu.Lock()
	defer x11SharedMu.Unlock()
	return x11SharedConn, x11SharedErr
}

func x11GlobalNameOwnerLoop(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	ch := make(chan *dbus.Signal, 10)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	for sig := range ch {
		// Stale loop guard: if shared conn rotated, exit old loop (B4 leak)
		x11SharedMu.Lock()
		cur := x11SharedConn
		x11SharedMu.Unlock()
		if cur != conn {
			return
		}
		if sig.Name != dbusServiceDBus+".NameOwnerChanged" {
			continue
		}
		if len(sig.Body) < 3 {
			continue
		}
		name, _ := sig.Body[0].(string)
		oldOwner, _ := sig.Body[1].(string)
		newOwner, _ := sig.Body[2].(string)
		x11MarkDirtyForOwnerChange(name, oldOwner, newOwner)
	}
}

func x11BusWatchLoop(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// Stale loop guard: if shared conn already rotated, this loop is orphaned (B4)
		x11SharedMu.Lock()
		cur := x11SharedConn
		x11SharedMu.Unlock()
		if cur != conn {
			return
		}
		if conn.Connected() {
			continue
		}
		x11ImeDebug("bus disconnect detected, backoff reconnect")
		backoff := 200 * time.Millisecond
		for {
			time.Sleep(backoff)
			if backoff < 2*time.Second {
				backoff *= 2
				if backoff > 2*time.Second {
					backoff = 2 * time.Second
				}
			}
			// Another waiter may have already reconnected
			x11SharedMu.Lock()
			if x11SharedConn != conn {
				x11SharedMu.Unlock()
				return
			}
			x11SharedMu.Unlock()
			c, err := dbus.SessionBus()
			if err != nil {
				x11ImeDebug("bus reconnect dial failed: %v backoff %v", err, backoff)
				continue
			}
			x11ImeDebug("bus reconnect ok")
			x11SharedMu.Lock()
			// Double-check still stale
			if x11SharedConn != conn {
				x11SharedMu.Unlock()
				_ = c.Close()
				return
			}
			x11SharedConn = c
			x11SharedErr = nil
			x11SharedMu.Unlock()
			for _, rule := range dbusMatchRules {
				_ = c.BusObject().Call(dbusServiceDBus+".AddMatch", 0, rule).Err
			}
			go x11GlobalNameOwnerLoop(c)
			go x11BusWatchLoop(c)
			x11ImesMu.Lock()
			for im := range x11Imes {
				im.mu.Lock()
				im.conn = c // B3: propagate new conn to existing IMEs
				im.imeDirty = true
				im.mu.Unlock()
			}
			x11ImesMu.Unlock()
			return
		}
	}
}

func dbusHasOwner(conn *dbus.Conn, name string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("nil conn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	var has bool
	err := conn.BusObject().CallWithContext(ctx, dbusServiceDBus+".NameHasOwner", 0, name).Store(&has)
	if err != nil {
		return false, err
	}
	return has, nil
}

func x11ProbeOrder() []string {
	gtk := strings.ToLower(os.Getenv("GTK_IM_MODULE"))
	qt := strings.ToLower(os.Getenv("QT_IM_MODULE"))
	xmod := strings.ToLower(os.Getenv("XMODIFIERS"))
	check := func(v, src string) (bool, []string) {
		if strings.Contains(v, "ibus") {
			x11ImeDebug("probe order: ibus (%s=%q)", src, v)
			return true, []string{"ibus", "fcitx5"}
		}
		if strings.Contains(v, "fcitx") {
			x11ImeDebug("probe order: fcitx5 (%s=%q)", src, v)
			return true, []string{"fcitx5", "ibus"}
		}
		return false, nil
	}
	if ok, ord := check(gtk, "GTK_IM_MODULE"); ok {
		return ord
	}
	if ok, ord := check(qt, "QT_IM_MODULE"); ok {
		return ord
	}
	if ok, ord := check(xmod, "XMODIFIERS"); ok {
		return ord
	}
	x11ImeDebug("probe order: ibus (default GTK=%q QT=%q XMOD=%q)", gtk, qt, xmod)
	return []string{"ibus", "fcitx5"}
}

func x11ClientName() string {
	exe := os.Args[0]
	if exe == "" {
		exe = "gpui"
	}
	base := filepath.Base(exe)
	if base == "" || base == "." {
		base = "gpui"
	}
	return "gpui:" + base
}

func x11AppName() string {
	exe := os.Args[0]
	if exe == "" {
		return "gpui"
	}
	base := filepath.Base(exe)
	if base == "" || base == "." {
		return "gpui"
	}
	return base
}

func isSyntheticPath(p dbus.ObjectPath) bool {
	return strings.HasPrefix(string(p), syntheticFcitxPrefix)
}

// imeForX11 供 x11_linux.go 调用：有总线且有守护时返回 x11Ime，
// 探测全异步化，建窗不阻塞（500ms 超时 per engine）。
func imeForX11(h *x11Host) IME {
	conn, err := sharedDBusConn()
	if err != nil || conn == nil {
		x11ImeDebug("imeForX11 no bus: %v", err)
		return nil
	}
	hasIbus, _ := dbusHasOwner(conn, dbusServiceIBus)
	hasFcitx5, _ := dbusHasOwner(conn, dbusServiceFcitx5)
	hasFcitx, _ := dbusHasOwner(conn, dbusServiceFcitx)
	if !hasIbus && !hasFcitx5 && !hasFcitx {
		x11ImeDebug("imeForX11 no daemon owner (ibus=%v fcitx5=%v fcitx=%v) degrade nil", hasIbus, hasFcitx5, hasFcitx)
		return nil
	}
	x11ImeDebug("imeForX11 created conn=%p hasIbus=%v hasFcitx5=%v hasFcitx=%v", conn, hasIbus, hasFcitx5, hasFcitx)
	im := &x11Ime{
		conn: conn,
		host: h,
	}
	x11RegisterIme(im)
	go im.asyncProbe()
	return im
}

// ensureReprobe 懒重探，脏标记或无对象时按 S2 顺序同步重探
func (im *x11Ime) ensureReprobe() {
	if im == nil {
		return
	}
	im.mu.Lock()
	need := im.imeDirty || im.objectPath == ""
	if need && !im.lastProbeFail.IsZero() && time.Since(im.lastProbeFail) < 500*time.Millisecond {
		im.mu.Unlock()
		x11ImeDebug("lazy reprobe throttled lastFail=%v", im.lastProbeFail)
		return
	}
	im.mu.Unlock()
	if !need {
		return
	}
	x11ImeDebug("lazy reprobe triggered dirty=%v path=%q", im.imeDirty, im.ObjectPath())
	order := x11ProbeOrder()
	for _, eng := range order {
		var obj dbus.ObjectPath
		var err error
		if eng == "ibus" {
			obj, err = im.tryCreateIbus(500 * time.Millisecond)
		} else {
			obj, err = im.tryCreateFcitx5(500 * time.Millisecond)
		}
		if err != nil {
			x11ImeDebug("lazy reprobe %s failed: %v", eng, err)
			continue
		}
		x11ImeDebug("lazy reprobe %s ok %s", eng, obj)
		if eng == "ibus" {
			_ = im.setCapabilitiesIbus(obj, ibusCaps)
		} else {
			_ = im.setCapabilitiesFcitx(obj, fcitxCaps)
		}
		im.mu.Lock()
		if im.closed {
			im.mu.Unlock()
			im.destroyObject(obj, eng)
			return
		}
		oldObj := im.objectPath
		oldEng := im.engine
		im.engine = eng
		im.objectPath = obj
		im.imeDirty = false
		im.lastProbeFail = time.Time{}
		hasRect := im.hasRect
		rect := im.lastRect
		lastText := im.lastText
		lastCursor := im.lastCursor
		lastAnchor := im.lastAnchor
		im.mu.Unlock()
		if oldObj != "" && oldObj != obj {
			im.destroyObject(oldObj, oldEng)
		}
		x11ImeDebug("lazy reprobe success engine=%s path=%s", eng, obj)
		im.callFocusIn()
		if hasRect {
			im.callSetCursorLocation(rect)
		}
		if lastText != "" {
			im.callSetSurroundingText(lastText, lastCursor, lastAnchor)
		}
		im.startSignalLoop()
		return
	}
	x11ImeDebug("lazy reprobe all failed")
	im.mu.Lock()
	im.imeDirty = false
	im.lastProbeFail = time.Now()
	im.mu.Unlock()
}

func (im *x11Ime) asyncProbe() {
	order := x11ProbeOrder()
	x11ImeDebug("asyncProbe start order=%v", order)
	for _, eng := range order {
		var obj dbus.ObjectPath
		var err error
		if eng == "ibus" {
			obj, err = im.tryCreateIbus(500 * time.Millisecond)
		} else {
			obj, err = im.tryCreateFcitx5(500 * time.Millisecond)
		}
		if err != nil {
			x11ImeDebug("CreateInputContext %s failed: %v", eng, err)
			continue
		}
		x11ImeDebug("CreateInputContext %s ok objectPath=%s", eng, obj)
		if eng == "ibus" {
			if err := im.setCapabilitiesIbus(obj, ibusCaps); err != nil {
				x11ImeDebug("SetCapabilities ibus failed: %v", err)
			} else {
				x11ImeDebug("SetCapabilities 41 ok")
			}
		} else {
			if err := im.setCapabilitiesFcitx(obj, fcitxCaps); err != nil {
				x11ImeDebug("SetCapacity fcitx5 failed: %v", err)
			} else {
				x11ImeDebug("SetCapacity ok")
			}
		}
		im.mu.Lock()
		if im.closed {
			im.mu.Unlock()
			im.destroyObject(obj, eng)
			return
		}
		im.engine = eng
		im.objectPath = obj
		x11ImeDebug("engine=%s objectPath=%s", eng, obj)
		im.mu.Unlock()
		im.startSignalLoop()
		return
	}
	x11ImeDebug("probe all failed, degrade")
	im.mu.Lock()
	im.engine = ""
	im.objectPath = ""
	im.mu.Unlock()
}

func (im *x11Ime) tryCreateIbus(timeout time.Duration) (dbus.ObjectPath, error) {
	if im == nil || im.conn == nil {
		return "", fmt.Errorf("nil conn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	clientName := x11ClientName()
	x11ImeDebug("ibus CreateInputContext(s) client_name=%q", clientName)
	obj := im.conn.Object(dbusServiceIBus, dbusPathIBusBus)
	var path dbus.ObjectPath
	err := obj.CallWithContext(ctx, dbusIfaceIBus+".CreateInputContext", 0, clientName).Store(&path)
	if err != nil {
		x11ImeDebug("ibus single param failed, try dual: %v", err)
		ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
		defer cancel2()
		if err2 := obj.CallWithContext(ctx2, dbusIfaceIBus+".CreateInputContext", 0, clientName, clientName).Store(&path); err2 != nil {
			return "", err2
		}
	}
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	return path, nil
}

func (im *x11Ime) tryCreateFcitx5(timeout time.Duration) (dbus.ObjectPath, error) {
	if im == nil || im.conn == nil {
		return "", fmt.Errorf("nil conn")
	}
	appName := x11AppName()
	appID := "gpui"
	try := []struct {
		service string
		path    string
		iface   string
	}{
		{dbusServiceFcitx5, dbusPathFcitx5IM, dbusIfaceFcitx5IM},
		{dbusServiceFcitx, dbusPathFcitxIM, dbusIfaceFcitxIM},
	}
	for _, t := range try {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		x11ImeDebug("fcitx CreateInputContext(ss) %s %s appname=%q appid=%q", t.service, t.path, appName, appID)
		obj := im.conn.Object(t.service, dbus.ObjectPath(t.path))
		var path dbus.ObjectPath
		err := obj.CallWithContext(ctx, t.iface+".CreateInputContext", 0, appName, appID).Store(&path)
		cancel()
		if err == nil && path != "" {
			return path, nil
		}
		if err != nil {
			x11ImeDebug("fcitx %s failed: %v", t.service, err)
		}
	}
	for _, svc := range []string{"org.fcitx.Fcitx-0", dbusServiceFcitx} {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		x11ImeDebug("fcitx CreateICv3(si) %s /inputmethod appname=%q", svc, appName)
		obj := im.conn.Object(svc, "/inputmethod")
		var icid int32
		var ok bool
		var v0, v1, v2, v3 uint32
		err := obj.CallWithContext(ctx, dbusIfaceFcitxIM+".CreateICv3", 0, appName, int32(0)).Store(&icid, &ok, &v0, &v1, &v2, &v3)
		cancel()
		if err == nil {
			x11ImeDebug("fcitx CreateICv3 ok id=%d ok=%v -> synthetic path would be fake, treat as unsupported (hole 1)", icid, ok)
			// Hole 1: old fcitx CreateICv3 returns an integer id, not an object path.
			// Fabricating /org/fcitx/Fcitx/InputContext_<id> makes 9 later calls silently skip.
			// Honest fallback: report failure so probe can try ibus compat (which is real on this host).
			continue
		}
		x11ImeDebug("fcitx CreateICv3 %s failed: %v", svc, err)
		ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
		var id int32
		err2 := obj.CallWithContext(ctx2, dbusIfaceFcitxIM+".CreateICv3", 0, appName, int32(0)).Store(&id)
		cancel2()
		if err2 == nil {
			x11ImeDebug("fcitx CreateICv3 second try id=%d -> also synthetic, not usable", id)
			continue
		}
	}
	return "", fmt.Errorf("fcitx CreateInputContext all failed")
}

func (im *x11Ime) setCapabilitiesIbus(obj dbus.ObjectPath, caps uint32) error {
	if im == nil || im.conn == nil {
		return fmt.Errorf("nil conn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	o := im.conn.Object(dbusServiceIBus, obj)
	err := o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetCapabilities", 0, caps).Err
	if err != nil {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel2()
		if err2 := o.CallWithContext(ctx2, dbusIfaceIBusCtx+".SetCapability", 0, caps).Err; err2 == nil {
			return nil
		}
		return err
	}
	return nil
}

func (im *x11Ime) setCapabilitiesFcitx(obj dbus.ObjectPath, caps uint32) error {
	if im == nil || im.conn == nil {
		return fmt.Errorf("nil conn")
	}
	if isSyntheticPath(obj) {
		x11ImeDebug("fcitx old IC synthetic path %s skip SetCapacity", obj)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
		o := im.conn.Object(svc, obj)
		for _, meth := range []string{
			dbusIfaceFcitx5IM + ".SetCapacity",
			dbusIfaceFcitxIM + ".SetCapacity",
			dbusIfaceFcitx5IM + ".SetCapability",
		} {
			if err := o.CallWithContext(ctx, meth, 0, caps).Err; err == nil {
				return nil
			}
		}
	}
	o := im.conn.Object(dbusServiceFcitx5, obj)
	if err := o.CallWithContext(ctx, "SetCapacity", 0, caps).Err; err == nil {
		return nil
	}
	return fmt.Errorf("SetCapacity failed for %s", obj)
}

func (im *x11Ime) destroyObject(obj dbus.ObjectPath, engine string) {
	if im == nil || im.conn == nil || obj == "" {
		return
	}
	if isSyntheticPath(obj) {
		x11ImeDebug("DestroyIC fcitx old synthetic %s (no-op)", obj)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if engine == "ibus" {
		o := im.conn.Object(dbusServiceIBus, obj)
		err := o.CallWithContext(ctx, dbusServiceIBus+".Service.Destroy", 0).Err
		if err != nil {
			err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".Destroy", 0).Err
		}
		if err != nil {
			err = o.CallWithContext(ctx, "Destroy", 0).Err
		}
		x11ImeDebug("Destroy ibus %s err=%v", obj, err)
	} else {
		for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
			o := im.conn.Object(svc, obj)
			_ = o.CallWithContext(ctx, dbusIfaceFcitx5IM+".DestroyIC", 0).Err
			_ = o.CallWithContext(ctx, "DestroyIC", 0).Err
		}
		x11ImeDebug("DestroyIC fcitx %s", obj)
	}
}

// Close 由 x11Host.destroy 调用，per-window Destroy
func (im *x11Ime) Close() {
	if im == nil {
		return
	}
	x11UnregisterIme(im)
	im.stopSignalLoop()
	im.mu.Lock()
	if im.closed {
		im.mu.Unlock()
		return
	}
	im.closed = true
	obj := im.objectPath
	eng := im.engine
	im.objectPath = ""
	im.engine = ""
	im.mu.Unlock()
	if obj != "" {
		im.destroyObject(obj, eng)
	}
}

// S5: 信号循环

func (im *x11Ime) startSignalLoop() {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		return
	}
	im.stopSignalLoop()
	ch := make(chan *dbus.Signal, 20)
	im.mu.Lock()
	im.sigCh = ch
	im.sigStop = make(chan struct{})
	im.mu.Unlock()
	im.conn.Signal(ch)
	go im.signalLoop()
	x11ImeDebug("signal loop started for %s engine=%s", im.ObjectPath(), im.Engine())
}

func (im *x11Ime) stopSignalLoop() {
	if im == nil {
		return
	}
	im.mu.Lock()
	ch := im.sigCh
	stop := im.sigStop
	im.sigCh = nil
	im.sigStop = nil
	im.mu.Unlock()
	if ch != nil && im.conn != nil {
		im.conn.RemoveSignal(ch)
		close(ch)
	}
	if stop != nil {
		close(stop)
	}
}

func (im *x11Ime) signalLoop() {
	if im == nil {
		return
	}
	im.mu.Lock()
	ch := im.sigCh
	stop := im.sigStop
	im.mu.Unlock()
	if ch == nil {
		return
	}
	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return
			}
			im.handleSignal(sig)
		case <-stop:
			return
		}
	}
}

func (im *x11Ime) handleSignal(sig *dbus.Signal) {
	if im == nil || sig == nil {
		return
	}
	path := im.ObjectPath()
	if sig.Path != path {
		if !isSyntheticPath(path) {
			return
		}
		if sig.Sender != "org.fcitx.Fcitx-0" && sig.Sender != dbusServiceFcitx5 && sig.Sender != dbusServiceFcitx {
			return
		}
	}
	x11ImeDebug("signal %s %s path=%s body=%v", sig.Sender, sig.Name, sig.Path, sig.Body)
	switch sig.Name {
	case dbusIfaceIBusCtx + ".UpdatePreeditText", "UpdatePreeditText":
		if len(sig.Body) >= 3 {
			if v, ok := sig.Body[0].(dbus.Variant); ok {
				text, segs := DecodeIBusVariant(v)
				cursor, _ := sig.Body[1].(uint32)
				visible, _ := sig.Body[2].(bool)
				x11ImeDebug("UpdatePreeditText text=%q cursor=%d visible=%v segs=%v", text, cursor, visible, segs)
				if !visible || text == "" {
					im.pushPreedit("", 0, false, segs)
				} else {
					im.pushPreedit(text, int(cursor), true, segs)
				}
			}
		}
	case dbusIfaceIBusCtx + ".CommitText", "CommitText":
		if len(sig.Body) >= 1 {
			if v, ok := sig.Body[0].(dbus.Variant); ok {
				text, _ := DecodeIBusVariant(v)
				x11ImeDebug("CommitText %q", text)
				im.pushCommit(text)
			}
		}
	case dbusIfaceIBusCtx + ".DeleteSurroundingText", "DeleteSurroundingText":
		if len(sig.Body) >= 2 {
			var offset int32
			var n uint32
			switch o := sig.Body[0].(type) {
			case int32:
				offset = o
			case int:
				offset = int32(o)
			}
			switch n2 := sig.Body[1].(type) {
			case uint32:
				n = n2
			case uint:
				n = uint32(n2)
			case int:
				n = uint32(n2)
			}
			x11ImeDebug("DeleteSurroundingText offset=%d n=%d", offset, n)
			im.pushDeleteSurrounding(int(offset), int(n))
		}
	case dbusIfaceIBusCtx + ".HidePreeditText", "HidePreeditText":
		x11ImeDebug("HidePreeditText")
		im.pushPreedit("", 0, false, nil)
	case "org.fcitx.Fcitx.InputMethod.UpdatePreedit", "UpdatePreedit", "org.fcitx.Fcitx5.InputContext.UpdatePreedit":
		if len(sig.Body) >= 2 {
			if text, ok := sig.Body[0].(string); ok {
				var cursor int32
				switch c := sig.Body[1].(type) {
				case int32:
					cursor = c
				case int:
					cursor = int32(c)
				case uint32:
					cursor = int32(c)
				}
				clean, segs := parseFcitxPreedit(text)
				x11ImeDebug("fcitx UpdatePreedit %q cursor=%d segs=%v", clean, cursor, segs)
				im.pushPreedit(clean, int(cursor), clean != "", segs)
			}
		}
	case "CommitString", "org.fcitx.Fcitx.InputMethod.CommitString", "org.fcitx.Fcitx5.InputContext.CommitString":
		if len(sig.Body) >= 1 {
			if text, ok := sig.Body[0].(string); ok {
				x11ImeDebug("fcitx CommitString %q", text)
				im.pushCommit(text)
			}
		}
	}
}

func (im *x11Ime) pushPreedit(text string, cursor int, visible bool, segs ...[]ImeSegment) {
	if im == nil || im.host == nil {
		return
	}
	var segList []ImeSegment
	if len(segs) > 0 {
		segList = segs[0]
	}
	if !visible || text == "" {
		im.mu.Lock()
		im.composing = false
		im.lastSegs = nil
		im.mu.Unlock()
		im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: "", IMEStart: -1, IMEEnd: -1})
		im.host.WakeUp()
		x11ImeDebug("push Preedit end segs=%v", segList)
		return
	}
	im.mu.Lock()
	im.composing = true
	im.lastSegs = segList
	im.mu.Unlock()
	start := cursor
	if start < 0 || start > len(text) {
		start = -1
	}
	im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: text, IMEStart: start, IMEEnd: start})
	im.host.WakeUp()
	x11ImeDebug("push Preedit %q cursor=%d segs=%v", text, cursor, segList)
}

func (im *x11Ime) pushCommit(text string) {
	if im == nil || im.host == nil {
		return
	}
	im.mu.Lock()
	im.composing = false
	im.mu.Unlock()
	im.host.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: text, IMEStart: -1, IMEEnd: -1})
	im.host.WakeUp()
	x11ImeDebug("push Commit %q", text)
}

func (im *x11Ime) pushDeleteSurrounding(offset, n int) {
	if im == nil || im.host == nil {
		return
	}
	// IBus DeleteSurroundingText(offset, n) is rune-based [caret+offset, caret+offset+n).
	// Wayland/composition expects before/after byte counts (-before, +after).
	// Translate via last surrounding text so multi-byte (emoji/CJK) byte lengths are correct.
	beforeBytes, afterBytes := 0, 0
	im.mu.Lock()
	text := im.lastText
	cursorByte := im.lastCursor
	im.mu.Unlock()
	if text != "" {
		// rune caret from byte cursor (cursor is at rune boundary)
		runeCaret := 0
		for i := 0; i < cursorByte; {
			_, sz := x11DecodeRune(text[i:])
			if sz == 0 {
				break
			}
			runeCaret++
			i += sz
		}
		startRune := runeCaret + offset
		endRune := startRune + n
		// Clamp
		totalRunes := 0
		for i := 0; i < len(text); {
			_, sz := x11DecodeRune(text[i:])
			if sz == 0 {
				break
			}
			totalRunes++
			i += sz
		}
		if startRune < 0 {
			startRune = 0
		}
		if endRune > totalRunes {
			endRune = totalRunes
		}
		if startRune < endRune {
			beforeEnd := endRune
			if beforeEnd > runeCaret {
				beforeEnd = runeCaret
			}
			beforeStart := startRune
			if beforeStart < 0 {
				beforeStart = 0
			}
			if beforeEnd > beforeStart {
				beforeBytes = byteLenForRunes(text, beforeStart, beforeEnd)
			}
			afterStart := startRune
			if afterStart < runeCaret {
				afterStart = runeCaret
			}
			afterEnd := endRune
			if afterEnd > afterStart {
				afterBytes = byteLenForRunes(text, afterStart, afterEnd)
			}
		}
	} else {
		// No surrounding context (fallback): assume ASCII 1:1
		runeCaretOff := 0 // unknown, treat offset relative to caret
		startRune := runeCaretOff + offset
		endRune := startRune + n
		_ = endRune
		if offset < 0 {
			if offset+n <= 0 {
				beforeBytes = n
			} else {
				beforeBytes = -offset
				afterBytes = offset + n
			}
		} else if offset == 0 {
			afterBytes = n
		} else {
			// gap before delete: Wayland can't represent gap, delete from caret (over-delete gap)
			afterBytes = offset + n
		}
		if beforeBytes < 0 {
			beforeBytes = 0
		}
		if afterBytes < 0 {
			afterBytes = 0
		}
	}
	im.host.pushIME(Event{Type: EventIME, IMEKind: 3, IMEStart: -beforeBytes, IMEEnd: afterBytes})
	im.host.WakeUp()
	x11ImeDebug("push DeleteSurrounding ibus offset=%d n=%d -> before=%d after=%d", offset, n, beforeBytes, afterBytes)
}

func x11DecodeRune(s string) (r rune, sz int) {
	if len(s) == 0 {
		return 0, 0
	}
	b := s[0]
	if b < 0x80 {
		return rune(b), 1
	}
	if b < 0xE0 {
		if len(s) < 2 {
			return rune(b), 1
		}
		return rune(b&0x1F)<<6 | rune(s[1]&0x3F), 2
	}
	if b < 0xF0 {
		if len(s) < 3 {
			return rune(b), 1
		}
		return rune(b&0x0F)<<12 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F), 3
	}
	if len(s) < 4 {
		return rune(b), 1
	}
	return rune(b&0x07)<<18 | rune(s[1]&0x3F)<<12 | rune(s[2]&0x3F)<<6 | rune(s[3]&0x3F), 4
}

func byteLenForRunes(text string, startRune, endRune int) int {
	if startRune >= endRune || startRune < 0 {
		return 0
	}
	i, rIdx := 0, 0
	startByte, endByte := -1, -1
	for i < len(text) {
		if rIdx == startRune {
			startByte = i
		}
		if rIdx == endRune {
			endByte = i
			break
		}
		_, sz := x11DecodeRune(text[i:])
		if sz == 0 {
			break
		}
		i += sz
		rIdx++
	}
	if startByte < 0 {
		startByte = len(text)
	}
	if endByte < 0 {
		endByte = len(text)
	}
	if endByte < startByte {
		return 0
	}
	return endByte - startByte
}

// --- helpers for verification ---

func (im *x11Ime) Engine() string {
	if im == nil {
		return ""
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.engine
}

func (im *x11Ime) ObjectPath() dbus.ObjectPath {
	if im == nil {
		return ""
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.objectPath
}

// --- IME 接口（S1/S2/S3/S6）---

func (im *x11Ime) EnableIME(rect Rect) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	im.mu.Lock()
	wasFocused := im.focused
	im.focused = true
	im.lastRect = rect
	im.hasRect = true
	purpose := im.purpose
	lastText := im.lastText
	lastCursor := im.lastCursor
	lastAnchor := im.lastAnchor
	hasSurrounding := lastText != "" || lastCursor != 0 || lastAnchor != 0
	im.mu.Unlock()
	if wasFocused {
		x11ImeDebug("EnableIME focused guard skip rect=%v", rect)
		return
	}
	engine, path := im.Engine(), im.ObjectPath()
	im.mu.Lock()
	dirty := im.imeDirty
	im.mu.Unlock()
	x11ImeDebug("EnableIME rect=%v engine=%s path=%s purpose=%d dirty=%v", rect, engine, path, purpose, dirty)
	im.callFocusIn()
	im.callSetCursorLocation(rect)
	im.callSetContentType(purpose)
	if hasSurrounding {
		im.callSetSurroundingText(lastText, lastCursor, lastAnchor)
	}
}

func (im *x11Ime) UpdateCursorRect(rect Rect) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	im.mu.Lock()
	same := im.hasRect && im.lastRect == rect
	im.lastRect = rect
	im.hasRect = true
	composing := im.composing
	focused := im.focused
	im.mu.Unlock()
	if same {
		x11ImeDebug("UpdateCursorRect skip same rect=%v", rect)
		return
	}
	if !focused {
		x11ImeDebug("UpdateCursorRect preheat (not focused) rect=%v", rect)
		return
	}
	if !composing {
		x11ImeDebug("UpdateCursorRect preheat (not composing) rect=%v", rect)
		return
	}
	engine, composingVal := im.Engine(), composing
	im.mu.Lock()
	dirty := im.imeDirty
	im.mu.Unlock()
	x11ImeDebug("UpdateCursorRect rect=%v engine=%s composing=%v dirty=%v", rect, engine, composingVal, dirty)
	im.callSetCursorLocation(rect)
}

func (im *x11Ime) SetContentType(purpose ContentPurpose) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	im.mu.Lock()
	im.purpose = purpose
	focused := im.focused
	path := im.objectPath
	dirty := im.imeDirty
	im.mu.Unlock()
	engine := im.Engine()
	x11ImeDebug("SetContentType purpose=%d engine=%s focused=%v dirty=%v", purpose, engine, focused, dirty)
	if !focused || path == "" {
		return
	}
	im.callSetContentType(purpose)
}

func (im *x11Ime) SetComposing(text string, cursor int) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	// SetComposing here reports surrounding text (InputRouter.pushSurrounding).
	// Do NOT flip the IME composing flag here — composing state is driven
	// ONLY by D-Bus signals (pushPreedit/pushCommit) to keep F-D3 anchor
	// reporting honest (real report only when IME preedit is active).
	im.mu.Lock()
	im.lastText = text
	im.lastCursor = cursor
	im.lastAnchor = cursor
	rect := im.lastRect
	hasRect := im.hasRect
	purpose := im.purpose
	engine := im.engine
	dirty := im.imeDirty
	im.mu.Unlock()
	x11ImeDebug("SetComposing len=%d cur=%d engine=%s dirty=%v", len(text), cursor, engine, dirty)
	if purpose == PurposePassword {
		x11ImeDebug("SetComposing skip password purpose")
		return
	}
	im.callSetSurroundingText(text, cursor, cursor)
	// Cursor rect is driven by UpdateCursorRect (which checks im.composing);
	// do not force a cursor update here to avoid preheat-period spurious reports.
	_ = rect
	_ = hasRect
}

func (im *x11Ime) Commit(text string) {
	if im == nil {
		return
	}
	x11ImeDebug("Commit len=%d engine=%s", len(text), im.Engine())
	im.mu.Lock()
	im.composing = false
	im.mu.Unlock()
	// If app explicitly commits, forward as IME commit so editor's AddText path runs
	if text != "" && im.host != nil {
		im.host.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: text})
		im.host.WakeUp()
	}
}

func (im *x11Ime) DisableIME() {
	if im == nil {
		return
	}
	im.mu.Lock()
	wasFocused := im.focused
	im.focused = false
	im.composing = false
	im.mu.Unlock()
	if !wasFocused {
		return
	}
	x11ImeDebug("DisableIME engine=%s", im.Engine())
	im.callFocusOut()
}

// --- S3 helpers ---

func (im *x11Ime) callFocusIn() {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("FocusIn skip no object")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	obj := string(im.ObjectPath())
	eng := im.Engine()
	var err error
	if eng == "ibus" {
		o := im.conn.Object(dbusServiceIBus, dbus.ObjectPath(obj))
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".FocusIn", 0).Err
		if err != nil {
			err = o.CallWithContext(ctx, "FocusIn", 0).Err
		}
	} else {
		for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx, "org.fcitx.Fcitx-0"} {
			o := im.conn.Object(svc, dbus.ObjectPath(obj))
			err = o.CallWithContext(ctx, dbusIfaceFcitxIM+".FocusIn", 0).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, "FocusIn", 0).Err
			if err == nil {
				break
			}
		}
		if isSyntheticPath(dbus.ObjectPath(obj)) {
			err = nil
			x11ImeDebug("FocusIn fcitx synthetic no-op %s", obj)
		}
	}
	x11ImeDebug("FocusIn %s err=%v", obj, err)
}

func (im *x11Ime) callFocusOut() {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("FocusOut skip no object")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	obj := string(im.ObjectPath())
	eng := im.Engine()
	var err error
	if eng == "ibus" {
		o := im.conn.Object(dbusServiceIBus, dbus.ObjectPath(obj))
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".FocusOut", 0).Err
		if err != nil {
			err = o.CallWithContext(ctx, "FocusOut", 0).Err
		}
	} else {
		for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx, "org.fcitx.Fcitx-0"} {
			o := im.conn.Object(svc, dbus.ObjectPath(obj))
			err = o.CallWithContext(ctx, "FocusOut", 0).Err
			if err == nil {
				break
			}
		}
		if isSyntheticPath(dbus.ObjectPath(obj)) {
			err = nil
		}
	}
	x11ImeDebug("FocusOut %s err=%v", obj, err)
	// F-D7 / P10: FocusOut must clear preedit; push IMECompose empty so editor's composingRange is cleared
	if im.host != nil {
		im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: "", IMEStart: -1, IMEEnd: -1})
		im.host.WakeUp()
	}
	x11ImeDebug("EndComposing after FocusOut")
}

func (im *x11Ime) callSetCursorLocation(rect Rect) {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("SetCursorLocation skip no object rect=%v", rect)
		return
	}
	px, py, pw, ph := im.translateRect(rect)
	x11ImeDebug("SetCursorLocation ii ii rect=%v -> phys x=%d y=%d w=%d h=%d", rect, px, py, pw, ph)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	obj := im.ObjectPath()
	eng := im.Engine()
	var err error
	if eng == "ibus" {
		o := im.conn.Object(dbusServiceIBus, obj)
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetCursorLocation", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
		if err != nil {
			err = o.CallWithContext(ctx, "SetCursorLocation", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
		}
		x11ImeDebug("ibus SetCursorLocation(%d,%d,%d,%d) err=%v", px, py, pw, ph, err)
	} else {
		if isSyntheticPath(obj) {
			x11ImeDebug("SetCursorRect fcitx synthetic skip %v", rect)
			return
		}
		for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
			o := im.conn.Object(svc, obj)
			err = o.CallWithContext(ctx, dbusIfaceFcitx5IM+".SetCursorRect", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, "SetCursorRect", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetCursorLocation", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
			if err == nil {
				break
			}
		}
		x11ImeDebug("fcitx SetCursorRect(%d,%d,%d,%d) err=%v", px, py, pw, ph, err)
	}
}

func (im *x11Ime) callSetSurroundingText(text string, cursor, anchor int) {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("SetSurroundingText skip no object len=%d", len(text))
		return
	}
	t, c, a := x11TruncateSurrounding(text, cursor, anchor)
	x11ImeDebug("SetSurroundingText len=%d->%d cursor=%d->%d anchor=%d->%d", len(text), len(t), cursor, c, anchor, a)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	obj := im.ObjectPath()
	eng := im.Engine()
	var err error
	if eng == "ibus" {
		// IBusText wire is (sa{sv}sv) [name, props, text, AttrList]; plain s is wrong (hole 3).
		// Use ibusTextPayload/ibusFullPayload style so godbus emits (sa{sv}sv) variant.
		ibusText := ibusFullPayload{
			Name:  ibusTextName,
			Props: map[string]dbus.Variant{},
			Text:  t,
			Attrs: dbus.MakeVariant([]interface{}{}),
		}
		v := dbus.MakeVariant(ibusText)
		// Fallback to simple text payload if full struct rejected (defensive)
		o := im.conn.Object(dbusServiceIBus, obj)
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetSurroundingText", 0, v, uint32(c), uint32(a)).Err
		if err != nil {
			v2 := dbus.MakeVariant(ibusTextPayload{Text: t, Attrs: nil})
			err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetSurroundingText", 0, v2, uint32(c), uint32(a)).Err
		}
		if err != nil {
			err = o.CallWithContext(ctx, "SetSurroundingText", 0, v, uint32(c), uint32(a)).Err
		}
		x11ImeDebug("ibus SetSurroundingText err=%v variant=%s", err, v.Signature())
	} else {
		if isSyntheticPath(obj) {
			x11ImeDebug("SetSurroundingText fcitx synthetic skip")
			return
		}
		for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
			o := im.conn.Object(svc, obj)
			err = o.CallWithContext(ctx, dbusIfaceFcitx5IM+".SetSurroundingText", 0, t, uint32(c), uint32(a)).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, "SetSurroundingText", 0, t, uint32(c), uint32(a)).Err
			if err == nil {
				break
			}
		}
		x11ImeDebug("fcitx SetSurroundingText err=%v", err)
	}
}

func (im *x11Ime) callSetContentType(purpose ContentPurpose) {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("SetContentType skip no object purpose=%d", purpose)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	obj := im.ObjectPath()
	eng := im.Engine()
	var err error
	if eng == "ibus" {
		o := im.conn.Object(dbusServiceIBus, obj)
		type ibusContentType struct {
			Purpose uint32
			Hints   uint32
		}
		v := dbus.MakeVariant(ibusContentType{Purpose: uint32(purpose), Hints: 0})
		err = o.CallWithContext(ctx, dbusServiceDBus+".Properties.Set", 0, dbusIfaceIBusCtx, "ContentType", v).Err
		if err != nil {
			err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetContentType", 0, uint32(purpose), uint32(0)).Err
		}
		if err != nil {
			err = o.CallWithContext(ctx, "SetContentType", 0, uint32(purpose), uint32(0)).Err
		}
		if err != nil {
			err = o.CallWithContext(ctx, dbusServiceDBus+".Properties.Set", 0, dbusIfaceIBusCtx, "ContentType", dbus.MakeVariant(uint32(purpose))).Err
		}
		x11ImeDebug("ibus SetContentType purpose=%d err=%v", purpose, err)
	} else {
		if isSyntheticPath(obj) {
			x11ImeDebug("SetContentType fcitx synthetic skip purpose=%d", purpose)
			return
		}
		for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
			o := im.conn.Object(svc, obj)
			err = o.CallWithContext(ctx, "SetContentType", 0, uint32(purpose)).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, dbusIfaceFcitxIM+".SetContentType", 0, uint32(purpose)).Err
			if err == nil {
				break
			}
		}
		x11ImeDebug("fcitx SetContentType purpose=%d err=%v", purpose, err)
	}
}

// translateRect 逻辑 rect -> 物理根窗口坐标
func (im *x11Ime) translateRect(r Rect) (int, int, int, int) {
	if im == nil || im.host == nil || im.host.st == nil {
		return int(r.X), int(r.Y), int(r.W), int(r.H)
	}
	st := im.host.st
	scale := st.scale
	if scale <= 0 {
		scale = 1
	}
	w := r.W
	h := r.H
	if w <= 0 {
		w = defaultCursorW
	}
	if h <= 0 {
		h = defaultCursorH
	}
	px := int(r.X * scale)
	py := int(r.Y * scale)
	pw := int(w * scale)
	ph := int(h * scale)
	if pw < 1 {
		pw = defaultCursorW
	}
	if st.display != 0 && st.window != 0 && st.root != 0 {
		if x, y, ok := x11TranslateToRoot(st, px, py); ok {
			px, py = x, y
		}
		// B6: RandR 多显修正（查询 monitor 几何，预留 per-monitor scale）
		px, py = x11RandRAdjust(st, px, py)
	}
	return px, py, pw, ph
}

// WantsSurrounding reports that X11 D-Bus needs surrounding on every edit.
func (im *x11Ime) WantsSurrounding() bool { return true }

// IsComposing reports whether a pre-edit session is active.
func (im *x11Ime) IsComposing() bool {
	if im == nil {
		return false
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.composing
}

func isX11ModifierKeysym(ks uint32) bool {
	switch ks {
	case 0xffe1, 0xffe2, // Shift L/R
		0xffe3, 0xffe4, // Control L/R
		0xffe9, 0xffea, // Alt L/R
		0xffeb, 0xffec: // Super/Meta L/R
		return true
	}
	return false
}

func isX11NavKeysym(ks uint32) bool {
	switch ks {
	case 0xff50, // Home
		0xff57, // End
		0xff55, // Page_Up
		0xff56, // Page_Down
		0xff51, // Left
		0xff52, // Up
		0xff53, // Right
		0xff54, // Down
		0xff09, // Tab
		0xff0d, // Return
		0xff1b: // Escape
		return true
	}
	return false
}

// ProcessKeyEvent S4：先走 D-Bus 判 consumed，再本地 Home/End 等分流
// keycode 为 X 硬件码，state 为 X 修饰位，isPress true=Press false=Release
// xTime 为 XKeyEvent.time（ms），fcitx 侧透传，0 时回退 time.Now（兼容旧测试）
// 返回 handled==true 则拦截不再本地插入，50ms 超时按未处理放行
// ibus 的 ProcessKeyEvent 通过 state 的 IBUS_RELEASE_MASK(1<<30) 区分释放，
// 实测 state=1<<30 可正常调通，故不再丢弃释放事件。
func (im *x11Ime) ProcessKeyEvent(keycode uint32, state uint32, isPress bool, xTime ...uint32) bool {
	var xTimeVal uint32
	if len(xTime) > 0 {
		xTimeVal = xTime[0]
	}
	im.mu.Lock()
	dirty := im.imeDirty
	im.mu.Unlock()
	im.ensureReprobe()
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("ProcessKeyEvent skip no object keycode=%d state=%d press=%v dirty=%v", keycode, state, isPress, dirty)
		return false
	}
	if isSyntheticPath(im.ObjectPath()) {
		x11ImeDebug("ProcessKeyEvent fcitx synthetic skip")
		return false
	}
	// Single lock acquisition for engine to avoid double locking.
	im.mu.Lock()
	eng := im.engine
	im.mu.Unlock()
	// IBUS_RELEASE_MASK = 1<<30, ibus 用它区分 press/release
	if eng == "ibus" && !isPress {
		state |= 1 << 30
	}
	var keysym uint32
	if im.host != nil && im.host.st != nil && im.host.st.keycodeToKeysym != nil && im.host.st.display != 0 {
		ks := xKeysymForState(im.host.st, uint(keycode), uint32(state))
		keysym = uint32(ks)
		x11ImeDebug("ProcessKeyEvent keysym=%#x keycode=%d state=%d", keysym, keycode, state)
	}
	obj := im.ObjectPath()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var handled bool
	var err error
	if eng == "ibus" {
		o := im.conn.Object(dbusServiceIBus, obj)
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
		if err != nil {
			err = o.CallWithContext(ctx, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
		}
		x11ImeDebug("ibus ProcessKeyEvent uuu (%#x,%d,%d)->%v err=%v", keysym, keycode, state, handled, err)
	} else {
		o := im.conn.Object(dbusServiceFcitx5, obj)
		t := xTimeVal
		if t == 0 {
			t = uint32(time.Now().UnixNano() / 1e6 & 0xffffffff)
		}
		isRelease := !isPress
		err = o.CallWithContext(ctx, dbusIfaceFcitxIM+".ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state), t, isRelease).Store(&handled)
		if err != nil {
			err = o.CallWithContext(ctx, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state), t, isRelease).Store(&handled)
		}
		if err != nil {
			ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel2()
			err = o.CallWithContext(ctx2, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
		}
		x11ImeDebug("fcitx ProcessKeyEvent uuuub (%#x,%d,%d,%d,%v)->%v err=%v", keysym, keycode, state, t, isRelease, handled, err)
	}
	if err != nil {
		x11ImeDebug("ProcessKeyEvent err %v -> pass-through", err)
		return false
	}
	// 修饰键（Shift/Ctrl/Alt/Meta）不拦截：需让本地感知修饰状态，否则 Shift+字母/方向 的后续组合会丢失修饰
	if handled && isX11ModifierKeysym(keysym) {
		x11ImeDebug("ProcessKeyEvent modifier %#x handled but not blocking (preserve local mods)", keysym)
		return false
	}
	// 英文非组合态：输入法已无 preedit，方向/翻页等导航键应直通本地编辑器（Shift+方向选区等快捷键才有效）
	if handled && !im.IsComposing() && isX11NavKeysym(keysym) {
		x11ImeDebug("ProcessKeyEvent nav %#x handled but not composing -> pass-through (english mode shortcut)", keysym)
		return false
	}
	return handled
}

// x11TruncateSurrounding 复用 textinput.TruncateSurrounding 语义：4000 居中，UTF8 边界安全
func x11TruncateSurrounding(text string, cursor, anchor int) (string, int, int) {
	if len(text)+1 <= 4000 {
		return text, cursor, anchor
	}
	budget := 3999
	c := cursor
	if c < 0 {
		c = 0
	}
	if c > len(text) {
		c = len(text)
	}
	half := budget / 2
	start := c - half
	if start < 0 {
		start = 0
	}
	end := start + budget
	if end > len(text) {
		end = len(text)
		start = end - budget
		if start < 0 {
			start = 0
		}
	}
	for start > 0 && start < len(text) && (text[start]&0xC0) == 0x80 {
		start--
	}
	for end > start && end < len(text) && (text[end]&0xC0) == 0x80 {
		end--
	}
	if end-start > budget {
		end = start + budget
		for end > start && end < len(text) && (text[end]&0xC0) == 0x80 {
			end--
		}
	}
	newCursor := c - start
	for newCursor > 0 && newCursor < len(text[start:end]) && (text[start+newCursor]&0xC0) == 0x80 {
		newCursor--
	}
	newAnchor := anchor - start
	if newAnchor < 0 {
		newAnchor = newCursor
	}
	if newAnchor > len(text[start:end]) {
		newAnchor = len(text[start:end])
	}
	for newAnchor > 0 && newAnchor < len(text[start:end]) && (text[start+newAnchor]&0xC0) == 0x80 {
		newAnchor--
	}
	if len(text[start:end])+1 > 4000 {
		// strict cap: trim to 3999 bytes at rune boundary
		end = start + 3999
		for end > start && end < len(text) && (text[end]&0xC0) == 0x80 {
			end--
		}
		return text[start:end], newCursor, newAnchor
	}
	return text[start:end], newCursor, newAnchor
}

func resetSharedDBusConnForTest() {
	x11SharedMu.Lock()
	if x11SharedConn != nil {
		_ = x11SharedConn.Close()
	}
	x11SharedConn = nil
	x11SharedErr = nil
	x11SharedMu.Unlock()
	x11SharedOnce = sync.Once{}
}
