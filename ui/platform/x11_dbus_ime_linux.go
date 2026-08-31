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
	x11ImesMu.Unlock()
}

func x11MarkDirtyForOwnerChange(name, oldOwner, newOwner string) {
	// 仅关心 ibus/fcitx 三名
	if name != "org.freedesktop.IBus" && name != "org.fcitx.Fcitx5" && name != "org.fcitx.Fcitx" {
		return
	}
	x11ImesMu.Lock()
	defer x11ImesMu.Unlock()
	for im := range x11Imes {
		im.mu.Lock()
		// 有→空：FocusOut+EndComposing 置 nil 逻辑
		if oldOwner != "" && newOwner == "" {
			x11ImeDebug("NameOwnerChanged %s %q->%q imeDirty=true FocusOut", name, oldOwner, newOwner)
			// 标记脏并清 focused/composing
			im.imeDirty = true
			wasFocused := im.focused
			im.focused = false
			im.composing = false
			im.mu.Unlock()
			if wasFocused {
				im.callFocusOut()
			}
			// 清路径以便下次懒重探
			im.mu.Lock()
			// 保留 engine 供日志，但清 path 以触发重探
			im.objectPath = ""
			im.mu.Unlock()
		} else if oldOwner == "" && newOwner != "" {
			x11ImeDebug("NameOwnerChanged %s %q->%q imeDirty=true", name, oldOwner, newOwner)
			im.imeDirty = true
			im.mu.Unlock()
		} else {
			im.mu.Unlock()
		}
	}
}

func x11ImeDebug(format string, args ...any) {
	if os.Getenv("GPUI_IME_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "[ime-x11] "+format+"\n", args...)
}

// sharedDBusConn 返回进程级单例会话总线连接。
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
		for _, rule := range []string{
			"type='signal',sender='org.freedesktop.IBus'",
			"type='signal',sender='org.fcitx.Fcitx5'",
			"type='signal',sender='org.fcitx.Fcitx'",
			"type='signal',sender='org.freedesktop.DBus',interface='org.freedesktop.DBus',member='NameOwnerChanged'",
		} {
			if err := c.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err; err != nil {
				x11ImeDebug("dbus AddMatch %q failed: %v", rule, err)
			} else {
				x11ImeDebug("dbus AddMatch %q ok", rule)
			}
		}
		x11ImeDebug("dbus match ok")
		x11SharedMu.Lock()
		x11SharedConn = c
		x11SharedMu.Unlock()
		// S6: 启动全局 NameOwnerChanged 监听
		go x11GlobalNameOwnerLoop(c)
		// S6: Bus 断开退避重拨（200ms→2s）
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
		if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" {
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
	// Godbus 的 Signals 通道关闭代表 Bus 断开
	// 这里用一个独立的通道监听 conn 的关闭
	// 由于 godbus 不直接暴露关闭信号，我们通过轮询 NameHasOwner 失败来判断
	// 简化：监听 conn 的错误通道（若有）或定期检查
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if conn.Connected() {
			continue
		}
		x11ImeDebug("bus disconnect detected, backoff reconnect")
		// 指数退避 200ms→2s 重拨
		backoff := 200 * time.Millisecond
		for {
			time.Sleep(backoff)
			if backoff < 2*time.Second {
				backoff *= 2
				if backoff > 2*time.Second {
					backoff = 2 * time.Second
				}
			}
			c, err := dbus.SessionBus()
			if err != nil {
				x11ImeDebug("bus reconnect dial failed: %v backoff %v", err, backoff)
				continue
			}
			x11ImeDebug("bus reconnect ok")
			x11SharedMu.Lock()
			x11SharedConn = c
			x11SharedErr = nil
			x11SharedMu.Unlock()
			// 重新订阅
			for _, rule := range []string{
				"type='signal',sender='org.freedesktop.IBus'",
				"type='signal',sender='org.fcitx.Fcitx5'",
				"type='signal',sender='org.fcitx.Fcitx'",
				"type='signal',sender='org.freedesktop.DBus',interface='org.freedesktop.DBus',member='NameOwnerChanged'",
			} {
				_ = c.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err
			}
			go x11GlobalNameOwnerLoop(c)
			go x11BusWatchLoop(c)
			// 标记所有 IME 脏，下次输入懒重探
			x11ImesMu.Lock()
			for im := range x11Imes {
				im.mu.Lock()
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
	err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, name).Store(&has)
	if err != nil {
		return false, err
	}
	return has, nil
}

// x11ProbeOrder 按 GTK_IM_MODULE/QT_IM_MODULE/XMODIFIERS 定探测顺序
// 优先级 GTK > QT > XMODIFIERS，含 ibus 则 ibus 优先，含 fcitx 则 fcitx 优先，空值默认 ibus
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

// imeForX11 供 x11_linux.go 调用：有总线且有守护时返回 x11Ime，
// 探测全异步化，建窗不阻塞（500ms 超时 per engine）。
func imeForX11(h *x11Host) IME {
	conn, err := sharedDBusConn()
	if err != nil || conn == nil {
		x11ImeDebug("imeForX11 no bus: %v", err)
		return nil
	}
	hasIbus, _ := dbusHasOwner(conn, "org.freedesktop.IBus")
	hasFcitx5, _ := dbusHasOwner(conn, "org.fcitx.Fcitx5")
	hasFcitx, _ := dbusHasOwner(conn, "org.fcitx.Fcitx")
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
	// S2 异步探测：后台建 InputContext，前台立即返回不卡建窗
	go im.asyncProbe()
	return im
}

// S6: 懒重探，脏标记或无对象时按 S2 顺序同步重探
func (im *x11Ime) ensureReprobe() {
	if im == nil {
		return
	}
	im.mu.Lock()
	dirty := im.imeDirty
	need := dirty || im.objectPath == ""
	im.mu.Unlock()
	if !need {
		return
	}
	x11ImeDebug("lazy reprobe triggered dirty=%v path=%q", dirty, im.ObjectPath())
	// 同步重探，500ms 超时
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
			_ = im.setCapabilitiesIbus(obj, 41)
		} else {
			_ = im.setCapabilitiesFcitx(obj, 16)
		}
		im.mu.Lock()
		if im.closed {
			im.mu.Unlock()
			im.destroyObject(obj, eng)
			return
		}
		im.engine = eng
		im.objectPath = obj
		im.imeDirty = false
		// 补 FocusIn+Cursor+Surrounding
		hasRect := im.hasRect
		rect := im.lastRect
		im.mu.Unlock()
		x11ImeDebug("lazy reprobe success engine=%s path=%s", eng, obj)
		im.callFocusIn()
		_ = im.setCapabilitiesIbus(obj, 41)
		if hasRect {
			im.callSetCursorLocation(rect)
		}
		if im.lastText != "" {
			im.callSetSurroundingText(im.lastText, im.lastCursor, im.lastAnchor)
		}
		im.startSignalLoop()
		return
	}
	x11ImeDebug("lazy reprobe all failed")
	im.mu.Lock()
	im.imeDirty = false
	im.mu.Unlock()
}

// asyncProbe 后台按顺序探 CreateInputContext 并 SetCapabilities
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
		// SetCapabilities
		if eng == "ibus" {
			if err := im.setCapabilitiesIbus(obj, 41); err != nil {
				x11ImeDebug("SetCapabilities ibus failed: %v", err)
			} else {
				x11ImeDebug("SetCapabilities 41 ok")
			}
		} else {
			if err := im.setCapabilitiesFcitx(obj, 16); err != nil {
				x11ImeDebug("SetCapacity fcitx5 failed: %v", err)
			} else {
				x11ImeDebug("SetCapacity ok")
			}
		}
		im.mu.Lock()
		// 避免 Close 后又被写
		if im.closed {
			im.mu.Unlock()
			// 已关闭，立即销毁刚建的上下文避免泄漏
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
	obj := im.conn.Object("org.freedesktop.IBus", "/org/freedesktop/IBus")
	var path dbus.ObjectPath
	// 首选单参 s（官方 src/ibusbus.c:ibus_bus_create_input_context），老版双参 ss 兼容探测放在失败后重试
	err := obj.CallWithContext(ctx, "org.freedesktop.IBus.CreateInputContext", 0, clientName).Store(&path)
	if err != nil {
		// 尝试双参兼容（老版）
		x11ImeDebug("ibus single param failed, try dual: %v", err)
		ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
		defer cancel2()
		err2 := obj.CallWithContext(ctx2, "org.freedesktop.IBus.CreateInputContext", 0, clientName, clientName).Store(&path)
		if err2 != nil {
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
	// 优先新服务/路径 (Fcitx5 5.0+)
	try := []struct {
		service string
		path    string
		iface   string
	}{
		{"org.fcitx.Fcitx5", "/org/fcitx/Fcitx5/InputMethod", "org.fcitx.Fcitx5.InputMethod"},
		{"org.fcitx.Fcitx", "/org/fcitx/Fcitx/InputMethod", "org.fcitx.Fcitx.InputMethod"},
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
	// 兼容旧 fcitx (Fcitx 4.x) 的 CreateICv3(si) -> int id，合成路径
	for _, svc := range []string{"org.fcitx.Fcitx-0", "org.fcitx.Fcitx"} {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		x11ImeDebug("fcitx CreateICv3(si) %s /inputmethod appname=%q", svc, appName)
		obj := im.conn.Object(svc, "/inputmethod")
		var icid int32
		var ok bool
		var v0, v1, v2, v3 uint32
		// 兼容两种签名: CreateICv3(si) -> (i b uuuu) 或 (i b ...)
		err := obj.CallWithContext(ctx, "org.fcitx.Fcitx.InputMethod.CreateICv3", 0, appName, int32(0)).Store(&icid, &ok, &v0, &v1, &v2, &v3)
		cancel()
		if err == nil {
			x11ImeDebug("fcitx CreateICv3 ok id=%d ok=%v", icid, ok)
			// 合成对象路径以保持每窗唯一，供 Destroy 时识别
			return dbus.ObjectPath(fmt.Sprintf("/org/fcitx/Fcitx/InputContext_%d", icid)), nil
		}
		x11ImeDebug("fcitx CreateICv3 %s failed: %v", svc, err)
		// 再试简化版 si -> i
		ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
		var id int32
		err2 := obj.CallWithContext(ctx2, "org.fcitx.Fcitx.InputMethod.CreateICv3", 0, appName, int32(0)).Store(&id)
		cancel2()
		if err2 == nil {
			return dbus.ObjectPath(fmt.Sprintf("/org/fcitx/Fcitx/InputContext_%d", id)), nil
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
	// ibus InputContext SetCapabilities(u)
	o := im.conn.Object("org.freedesktop.IBus", obj)
	// 兼容两种名：SetCapabilities / SetCapability
	err := o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.SetCapabilities", 0, caps).Err
	if err != nil {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel2()
		err2 := o.CallWithContext(ctx2, "org.freedesktop.IBus.InputContext.SetCapability", 0, caps).Err
		if err2 == nil {
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
	// 旧 fcitx CreateICv3 合成的路径无需 SetCapacity，直接成功
	if strings.HasPrefix(string(obj), "/org/fcitx/Fcitx/InputContext_") {
		x11ImeDebug("fcitx old IC synthetic path %s skip SetCapacity", obj)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// fcitx InputContext 在对象上，服务名为创建时的服务，尝试两套名
	for _, svc := range []string{"org.fcitx.Fcitx5", "org.fcitx.Fcitx"} {
		o := im.conn.Object(svc, obj)
		for _, meth := range []string{
			"org.fcitx.Fcitx5.InputContext.SetCapacity",
			"org.fcitx.Fcitx.InputContext.SetCapacity",
			"org.fcitx.Fcitx5.InputContext.SetCapability",
		} {
			err := o.CallWithContext(ctx, meth, 0, caps).Err
			if err == nil {
				return nil
			}
		}
	}
	// 最后尝试不带接口前缀的短名
	o := im.conn.Object("org.fcitx.Fcitx5", obj)
	err := o.CallWithContext(ctx, "SetCapacity", 0, caps).Err
	if err == nil {
		return nil
	}
	return err
}

func (im *x11Ime) destroyObject(obj dbus.ObjectPath, engine string) {
	if im == nil || im.conn == nil || obj == "" {
		return
	}
	// 旧 fcitx 合成路径无需真实 Destroy，记日志即算清理
	if strings.HasPrefix(string(obj), "/org/fcitx/Fcitx/InputContext_") {
		x11ImeDebug("DestroyIC fcitx old synthetic %s (no-op)", obj)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if engine == "ibus" {
			o := im.conn.Object("org.freedesktop.IBus", obj)
			err := o.CallWithContext(ctx, "org.freedesktop.IBus.Service.Destroy", 0).Err
			if err != nil {
				err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.Destroy", 0).Err
			}
			if err != nil {
				err = o.CallWithContext(ctx, "Destroy", 0).Err
			}
			x11ImeDebug("Destroy ibus %s err=%v", obj, err)
		} else {
			for _, svc := range []string{"org.fcitx.Fcitx5", "org.fcitx.Fcitx"} {
				o := im.conn.Object(svc, obj)
				_ = o.CallWithContext(ctx, "org.fcitx.Fcitx5.InputContext.DestroyIC", 0).Err
				_ = o.CallWithContext(ctx, "DestroyIC", 0).Err
			}
			x11ImeDebug("DestroyIC fcitx %s", obj)
		}
	}
	// RemoveMatch 仅在最后窗口时有意义，这里每窗都尝试一次，失败忽略
	for _, rule := range []string{
		"type='signal',sender='org.freedesktop.IBus'",
		"type='signal',sender='org.fcitx.Fcitx5'",
	} {
		_ = im.conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, rule).Err
	}
	x11ImeDebug("RemoveMatch tried")
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
	// 仅处理本 InputContext 的信号
	if sig.Path != im.ObjectPath() {
		// 对 fcitx 旧合成路径，Path 可能不同，需要放行所有 fcitx 信号
		if !strings.HasPrefix(string(im.ObjectPath()), "/org/fcitx/") {
			return
		}
		// 对于合成路径，放宽过滤：只要 sender 是 fcitx 且 member 匹配即处理
		if sig.Sender != "org.fcitx.Fcitx-0" && sig.Sender != "org.fcitx.Fcitx5" && sig.Sender != "org.fcitx.Fcitx" {
			return
		}
	}
	x11ImeDebug("signal %s %s path=%s body=%v", sig.Sender, sig.Name, sig.Path, sig.Body)
	switch sig.Name {
	// ibus
	case "org.freedesktop.IBus.InputContext.UpdatePreeditText", "UpdatePreeditText":
		if len(sig.Body) >= 3 {
			if v, ok := sig.Body[0].(dbus.Variant); ok {
				text, segs := DecodeIBusVariant(v)
				cursor, _ := sig.Body[1].(uint32)
				visible, _ := sig.Body[2].(bool)
				x11ImeDebug("UpdatePreeditText text=%q cursor=%d visible=%v segs=%v", text, cursor, visible, segs)
				if !visible || text == "" {
					im.pushPreedit("", 0, false)
				} else {
					// cursor_pos 是 byte 偏移？按 spec 是 uint32，传给 UpdateComposingText
					im.pushPreedit(text, int(cursor), true)
				}
			}
		}
	case "org.freedesktop.IBus.InputContext.CommitText", "CommitText":
		if len(sig.Body) >= 1 {
			if v, ok := sig.Body[0].(dbus.Variant); ok {
				text, _ := DecodeIBusVariant(v)
				x11ImeDebug("CommitText %q", text)
				im.pushCommit(text)
			}
		}
	case "org.freedesktop.IBus.InputContext.DeleteSurroundingText", "DeleteSurroundingText":
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
	case "org.freedesktop.IBus.InputContext.HidePreeditText", "HidePreeditText":
		x11ImeDebug("HidePreeditText")
		im.pushPreedit("", 0, false)
	// fcitx
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
				im.pushPreedit(clean, int(cursor), clean != "")
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

func (im *x11Ime) pushPreedit(text string, cursor int, visible bool) {
	if im == nil || im.host == nil {
		return
	}
	if !visible || text == "" {
		// EndComposing
		im.mu.Lock()
		im.composing = false
		im.mu.Unlock()
		// 推送空 preedit 结束
		im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: "", IMEStart: -1, IMEEnd: -1})
		im.host.WakeUp()
		x11ImeDebug("push Preedit end")
		return
	}
	im.mu.Lock()
	im.composing = true
	im.mu.Unlock()
	// Start/End 为 cursor 偏移，-1 表示末尾
	start := cursor
	if start < 0 || start > len(text) {
		start = -1
	}
	im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: text, IMEStart: start, IMEEnd: start})
	im.host.WakeUp()
	x11ImeDebug("push Preedit %q cursor=%d", text, cursor)
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
	// offset/n 按 code point，platform.Event 用 IMEStart=-before, IMEEnd=+after
	im.host.pushIME(Event{Type: EventIME, IMEKind: 3, IMEStart: offset, IMEEnd: n})
	im.host.WakeUp()
	x11ImeDebug("push DeleteSurrounding %d %d", offset, n)
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
	im.mu.Unlock()
	if wasFocused {
		x11ImeDebug("EnableIME focused guard skip rect=%v", rect)
		return
	}
	x11ImeDebug("EnableIME rect=%v engine=%s path=%s purpose=%d dirty=%v", rect, im.Engine(), im.ObjectPath(), purpose, im.imeDirty)
	// S3: FocusIn + Cursor + ContentType + Surrounding
	im.callFocusIn()
	im.callSetCursorLocation(rect)
	im.callSetContentType(purpose)
	// surrounding: need text, but EnableIME only has rect; surrounding will be pushed via SetComposing from UI
	// For S3 we also push an empty surrounding to prime the context
	im.callSetSurroundingText("", 0, 0)
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
	// S3: 仅 composing 时实发，非 composing 仅缓存（防回声）
	if !focused {
		x11ImeDebug("UpdateCursorRect preheat (not focused) rect=%v", rect)
		return
	}
	if !composing {
		x11ImeDebug("UpdateCursorRect preheat (not composing) rect=%v", rect)
		return
	}
	x11ImeDebug("UpdateCursorRect rect=%v engine=%s composing=%v dirty=%v", rect, im.Engine(), composing, im.imeDirty)
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
	im.mu.Unlock()
	x11ImeDebug("SetContentType purpose=%d engine=%s focused=%v dirty=%v", purpose, im.Engine(), focused, im.imeDirty)
	if !focused || im.ObjectPath() == "" {
		return
	}
	// 密码框禁组合：PurposePassword 时不做额外处理，仍需告知对端
	im.callSetContentType(purpose)
}

func (im *x11Ime) SetComposing(text string, cursor int) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	// S3: 更新 composing 状态 + 环绕文本
	composing := text != ""
	im.mu.Lock()
	im.composing = composing
	im.lastText = text
	im.lastCursor = cursor
	im.lastAnchor = cursor
	im.mu.Unlock()
	x11ImeDebug("SetComposing len=%d cur=%d engine=%s composing=%v dirty=%v", len(text), cursor, im.Engine(), composing, im.imeDirty)
	// 密码时禁组合：直接不发 surrounding
	if im.purpose == PurposePassword {
		x11ImeDebug("SetComposing skip password purpose")
		return
	}
	im.callSetSurroundingText(text, cursor, cursor)
	// composing 变化后若有缓存 rect，则补发一次 cursor
	if composing {
		im.mu.Lock()
		rect := im.lastRect
		has := im.hasRect
		im.mu.Unlock()
		if has {
			im.callSetCursorLocation(rect)
		}
	}
}

func (im *x11Ime) Commit(text string) {
	if im == nil {
		return
	}
	x11ImeDebug("Commit len=%d engine=%s", len(text), im.Engine())
	// Commit 后 composing 清零
	im.mu.Lock()
	im.composing = false
	im.mu.Unlock()
	// 可选：提交后更新 surrounding 为空或新文本，这里交由上层 SetComposing 清理
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
		o := im.conn.Object("org.freedesktop.IBus", dbus.ObjectPath(obj))
		err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.FocusIn", 0).Err
		if err != nil {
			err = o.CallWithContext(ctx, "FocusIn", 0).Err
		}
	} else {
		// fcitx
		for _, svc := range []string{"org.fcitx.Fcitx5", "org.fcitx.Fcitx", "org.fcitx.Fcitx-0"} {
			o := im.conn.Object(svc, dbus.ObjectPath(obj))
			err = o.CallWithContext(ctx, "org.fcitx.Fcitx.InputMethod.FocusIn", 0).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, "FocusIn", 0).Err
			if err == nil {
				break
			}
		}
		if strings.HasPrefix(obj, "/org/fcitx/Fcitx/InputContext_") {
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
		o := im.conn.Object("org.freedesktop.IBus", dbus.ObjectPath(obj))
		err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.FocusOut", 0).Err
		if err != nil {
			err = o.CallWithContext(ctx, "FocusOut", 0).Err
		}
	} else {
		for _, svc := range []string{"org.fcitx.Fcitx5", "org.fcitx.Fcitx", "org.fcitx.Fcitx-0"} {
			o := im.conn.Object(svc, dbus.ObjectPath(obj))
			err = o.CallWithContext(ctx, "FocusOut", 0).Err
			if err == nil {
				break
			}
		}
		if strings.HasPrefix(obj, "/org/fcitx/Fcitx/InputContext_") {
			err = nil
		}
	}
	x11ImeDebug("FocusOut %s err=%v", obj, err)
	// EndComposing 清理
	x11ImeDebug("EndComposing after FocusOut")
}

func (im *x11Ime) callSetCursorLocation(rect Rect) {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("SetCursorLocation skip no object rect=%v", rect)
		return
	}
	// 物理坐标：逻辑 rect * ScaleFactor → 根窗口
	px, py, pw, ph := im.translateRect(rect)
	x11ImeDebug("SetCursorLocation ii ii rect=%v -> phys x=%d y=%d w=%d h=%d", rect, px, py, pw, ph)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	obj := im.ObjectPath()
	eng := im.Engine()
	var err error
	if eng == "ibus" {
		o := im.conn.Object("org.freedesktop.IBus", obj)
		err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.SetCursorLocation", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
		if err != nil {
			err = o.CallWithContext(ctx, "SetCursorLocation", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
		}
		x11ImeDebug("ibus SetCursorLocation(%d,%d,%d,%d) err=%v", px, py, pw, ph, err)
	} else {
		// fcitx5 SetCursorRect
		if strings.HasPrefix(string(obj), "/org/fcitx/Fcitx/InputContext_") {
			x11ImeDebug("SetCursorRect fcitx synthetic skip %v", rect)
			return
		}
		for _, svc := range []string{"org.fcitx.Fcitx5", "org.fcitx.Fcitx"} {
			o := im.conn.Object(svc, obj)
			err = o.CallWithContext(ctx, "org.fcitx.Fcitx.InputContext.SetCursorRect", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, "SetCursorRect", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.SetCursorLocation", 0, int32(px), int32(py), int32(pw), int32(ph)).Err
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
	// 4000 居中截断
	t, c, a := x11TruncateSurrounding(text, cursor, anchor)
	x11ImeDebug("SetSurroundingText len=%d->%d cursor=%d->%d anchor=%d->%d", len(text), len(t), cursor, c, anchor, a)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	obj := im.ObjectPath()
	eng := im.Engine()
	var err error
	if eng == "ibus" {
		// ibus: SetSurroundingText(v IBusText, u cursor, u anchor)  v 含 IBusText{文本, 属性}
		// 属性可空，类型必为 v 包 IBusText，用 dbus.MakeVariant 包装文本
		v := dbus.MakeVariant(t)
		o := im.conn.Object("org.freedesktop.IBus", obj)
		err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.SetSurroundingText", 0, v, uint32(c), uint32(a)).Err
		if err != nil {
			err = o.CallWithContext(ctx, "SetSurroundingText", 0, v, uint32(c), uint32(a)).Err
		}
		x11ImeDebug("ibus SetSurroundingText err=%v", err)
	} else {
		if strings.HasPrefix(string(obj), "/org/fcitx/Fcitx/InputContext_") {
			x11ImeDebug("SetSurroundingText fcitx synthetic skip")
			return
		}
		for _, svc := range []string{"org.fcitx.Fcitx5", "org.fcitx.Fcitx"} {
			o := im.conn.Object(svc, obj)
			err = o.CallWithContext(ctx, "org.fcitx.Fcitx.InputContext.SetSurroundingText", 0, t, uint32(c), uint32(a)).Err
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
		o := im.conn.Object("org.freedesktop.IBus", obj)
		// 官方：org.freedesktop.DBus.Properties.Set("org.freedesktop.IBus.InputContext", "ContentType", v((uu)))
		type ibusContentType struct {
			Purpose uint32
			Hints   uint32
		}
		v := dbus.MakeVariant(ibusContentType{Purpose: uint32(purpose), Hints: 0})
		err = o.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Set", 0, "org.freedesktop.IBus.InputContext", "ContentType", v).Err
		if err != nil {
			// 回退直接方法
			err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.SetContentType", 0, uint32(purpose), uint32(0)).Err
		}
		if err != nil {
			err = o.CallWithContext(ctx, "SetContentType", 0, uint32(purpose), uint32(0)).Err
		}
		// 再试单参
		if err != nil {
			err = o.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Set", 0, "org.freedesktop.IBus.InputContext", "ContentType", dbus.MakeVariant(uint32(purpose))).Err
		}
		x11ImeDebug("ibus SetContentType purpose=%d err=%v", purpose, err)
	} else {
		if strings.HasPrefix(string(obj), "/org/fcitx/Fcitx/InputContext_") {
			x11ImeDebug("SetContentType fcitx synthetic skip purpose=%d", purpose)
			return
		}
		for _, svc := range []string{"org.fcitx.Fcitx5", "org.fcitx.Fcitx"} {
			o := im.conn.Object(svc, obj)
			err = o.CallWithContext(ctx, "SetContentType", 0, uint32(purpose)).Err
			if err == nil {
				break
			}
			err = o.CallWithContext(ctx, "org.fcitx.Fcitx.InputContext.SetContentType", 0, uint32(purpose)).Err
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
		// 回退：直接按逻辑值取整
		return int(r.X), int(r.Y), int(r.W), int(r.H)
	}
	st := im.host.st
	scale := st.scale
	if scale <= 0 {
		scale = 1
	}
	// w=2 h=行高：若传入的 W/H 为 0，按 2x行高 兜底
	w := r.W
	h := r.H
	if w <= 0 {
		w = 2
	}
	if h <= 0 {
		h = 16
	}
	// 逻辑 -> 物理
	px := int((r.X) * scale)
	py := int((r.Y) * scale)
	pw := int(w * scale)
	ph := int(h * scale)
	if pw < 1 {
		pw = 2
	}
	// XTranslateCoordinates：window -> root
	if st.display != 0 && st.window != 0 && st.root != 0 {
		if x, y, ok := x11TranslateToRoot(st, px, py); ok {
			px, py = x, y
		}
	}
	return px, py, pw, ph
}

// ProcessKeyEvent S4：先走 D-Bus 判 consumed，再本地 Home/End 等分流
// keycode 为 X 硬件码，state 为 X 修饰位，isPress true=Press false=Release
// 返回 handled==true 则拦截不再本地插入，50ms 超时按未处理放行
func (im *x11Ime) ProcessKeyEvent(keycode uint32, state uint32, isPress bool) bool {
	im.ensureReprobe()
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("ProcessKeyEvent skip no object keycode=%d state=%d press=%v dirty=%v", keycode, state, isPress, im.imeDirty)
		return false
	}
	// 合成路径无真实 InputContext，直接放行
	if strings.HasPrefix(string(im.ObjectPath()), "/org/fcitx/Fcitx/InputContext_") {
		x11ImeDebug("ProcessKeyEvent fcitx synthetic skip")
		return false
	}
	// keyval = XKeycodeToKeysym(dpy, keycode, Shift?1:0)  Xlib index0=裸键 index1=Shift
	var keysym uint32
	if im.host != nil && im.host.st != nil && im.host.st.keycodeToKeysym != nil && im.host.st.display != 0 {
		idx := 0
		if state&1 != 0 { // ShiftMask
			idx = 1
		}
		ks := im.host.st.keycodeToKeysym(im.host.st.display, uint(keycode), idx)
		keysym = uint32(ks)
		x11ImeDebug("ProcessKeyEvent keysym=%#x keycode=%d state=%d idx=%d", keysym, keycode, state, idx)
	}
	eng := im.Engine()
	obj := im.ObjectPath()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var handled bool
	var err error
	if eng == "ibus" {
		// ibus: ProcessKeyEvent(uuu keyval,keycode,state)->b
		o := im.conn.Object("org.freedesktop.IBus", obj)
		err = o.CallWithContext(ctx, "org.freedesktop.IBus.InputContext.ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
		if err != nil {
			err = o.CallWithContext(ctx, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
		}
		x11ImeDebug("ibus ProcessKeyEvent uuu (%#x,%d,%d)->%v err=%v", keysym, keycode, state, handled, err)
	} else {
		// fcitx5: ProcessKeyEvent(uuuub keyval,keycode,state,time,isRelease)->b
		o := im.conn.Object("org.fcitx.Fcitx5", obj)
		// time 用当前毫秒，isRelease 取反 isPress
		t := uint32(time.Now().UnixNano() / 1e6 & 0xffffffff)
		isRelease := !isPress
		// 尝试多套名
		err = o.CallWithContext(ctx, "org.fcitx.Fcitx.InputMethod.ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state), t, isRelease).Store(&handled)
		if err != nil {
			err = o.CallWithContext(ctx, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state), t, isRelease).Store(&handled)
		}
		if err != nil {
			// 再试不带 time/isRelease 的旧签
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
	return handled
}

// x11TruncateSurrounding 复用 textinput.TruncateSurrounding 语义：4000 居中，UTF8 边界安全
func x11TruncateSurrounding(text string, cursor, anchor int) (string, int, int) {
	if len(text)+1 <= 4000 {
		return text, cursor, anchor
	}
	budget := 3999
	// 以 cursor 为中心
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
	for end < len(text) && (text[end]&0xC0) == 0x80 {
		end++
	}
	if end-start > budget {
		end = start + budget
		for end < len(text) && (text[end]&0xC0) == 0x80 {
			end++
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
