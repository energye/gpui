//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// 第2层 引擎层 fcitx：会话总线 + 生命周期（S1 期间保持基线不动）
// 会话总线地址来自 DBUS_SESSION_BUS_ADDRESS，会话单例复用

var (
	x11FcitxMu   sync.Mutex
	x11FcitxConn *dbus.Conn
	x11FcitxErr  error
	x11FcitxOnce sync.Once
)

// sharedFcitxConn 会话总线单例（fcitx5 基线）
func sharedFcitxConn() (*dbus.Conn, error) {
	x11FcitxOnce.Do(func() {
		x11ImeDebug("fcitx session dial start addr=%q", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
		c, err := dbus.SessionBus()
		if err != nil {
			x11ImeDebug("fcitx session dial/hello failed: %v", err)
			x11FcitxErr = err
			return
		}
		x11ImeDebug("fcitx session hello ok")
		for _, rule := range dbusMatchRules {
			if err := c.BusObject().Call(dbusServiceDBus+".AddMatch", 0, rule).Err; err != nil {
				x11ImeDebug("fcitx session AddMatch %q failed: %v", rule, err)
			} else {
				x11ImeDebug("fcitx session AddMatch %q ok", rule)
			}
		}
		x11FcitxMu.Lock()
		x11FcitxConn = c
		x11FcitxMu.Unlock()
		go x11GlobalNameOwnerLoop(c)
		go x11BusWatchLoop(c)
	})
	x11FcitxMu.Lock()
	defer x11FcitxMu.Unlock()
	return x11FcitxConn, x11FcitxErr
}

func resetFcitxSessionForTest() {
	x11FcitxMu.Lock()
	if x11FcitxConn != nil {
		_ = x11FcitxConn.Close()
	}
	x11FcitxConn = nil
	x11FcitxErr = nil
	x11FcitxMu.Unlock()
	x11FcitxOnce = sync.Once{}
}

func (e *fcitxEngine) Name() string { return "fcitx5" }
func (e *fcitxEngine) Caps() uint32 { return fcitxCaps }

func (e *fcitxEngine) CreateInputContext(conn *dbus.Conn, timeout time.Duration) (dbus.ObjectPath, error) {
	if conn == nil {
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
		obj := conn.Object(t.service, dbus.ObjectPath(t.path))
		var path dbus.ObjectPath
		err := obj.CallWithContext(ctx, t.iface+".CreateInputContext", dbus.FlagNoAutoStart, appName, appID).Store(&path)
		cancel()
		if err == nil && path != "" {
			return path, nil
		}
		if err != nil {
			x11ImeDebug("fcitx %s failed: %v", t.service, err)
		}
	}
	return "", fmt.Errorf("fcitx CreateInputContext all failed")
}

func (e *fcitxEngine) SetCapabilities(conn *dbus.Conn, obj dbus.ObjectPath, caps uint32) error {
	if conn == nil {
		return fmt.Errorf("nil conn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
		o := conn.Object(svc, obj)
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
	o := conn.Object(dbusServiceFcitx5, obj)
	if err := o.CallWithContext(ctx, "SetCapacity", 0, caps).Err; err == nil {
		return nil
	}
	return fmt.Errorf("SetCapacity failed for %s", obj)
}

func (e *fcitxEngine) Destroy(conn *dbus.Conn, obj dbus.ObjectPath) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
		o := conn.Object(svc, obj)
		_ = o.CallWithContext(ctx, dbusIfaceFcitx5IM+".DestroyIC", 0).Err
		_ = o.CallWithContext(ctx, "DestroyIC", 0).Err
	}
	x11ImeDebug("DestroyIC fcitx %s", obj)
	return nil
}

func (e *fcitxEngine) FocusIn(conn *dbus.Conn, obj dbus.ObjectPath) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx, "org.fcitx.Fcitx-0"} {
		o := conn.Object(svc, obj)
		err := o.CallWithContext(ctx, dbusIfaceFcitxIM+".FocusIn", 0).Err
		if err == nil {
			return nil
		}
		err = o.CallWithContext(ctx, "FocusIn", 0).Err
		if err == nil {
			return nil
		}
	}
	return nil
}

func (e *fcitxEngine) FocusOut(conn *dbus.Conn, obj dbus.ObjectPath) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx, "org.fcitx.Fcitx-0"} {
		o := conn.Object(svc, obj)
		err := o.CallWithContext(ctx, "FocusOut", 0).Err
		if err == nil {
			return nil
		}
	}
	return nil
}

func (e *fcitxEngine) SetCursorLocation(conn *dbus.Conn, obj dbus.ObjectPath, x, y, w, h int) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
		o := conn.Object(svc, obj)
		err := o.CallWithContext(ctx, dbusIfaceFcitx5IM+".SetCursorRect", 0, int32(x), int32(y), int32(w), int32(h)).Err
		if err == nil {
			return nil
		}
		err = o.CallWithContext(ctx, "SetCursorRect", 0, int32(x), int32(y), int32(w), int32(h)).Err
		if err == nil {
			return nil
		}
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetCursorLocation", 0, int32(x), int32(y), int32(w), int32(h)).Err
		if err == nil {
			return nil
		}
	}
	x11ImeDebug("fcitx SetCursorRect(%d,%d,%d,%d) err fallback", x, y, w, h)
	return nil
}

func (e *fcitxEngine) SetSurroundingText(conn *dbus.Conn, obj dbus.ObjectPath, text string, cursor, anchor int) error {
	if conn == nil || obj == "" {
		return nil
	}
	t, c, a := x11TruncateSurrounding(text, cursor, anchor)
	x11ImeDebug("SetSurroundingText len=%d->%d cursor=%d->%d anchor=%d->%d", len(text), len(t), cursor, c, anchor, a)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
		o := conn.Object(svc, obj)
		err := o.CallWithContext(ctx, dbusIfaceFcitx5IM+".SetSurroundingText", 0, t, uint32(c), uint32(a)).Err
		if err == nil {
			return nil
		}
		err = o.CallWithContext(ctx, "SetSurroundingText", 0, t, uint32(c), uint32(a)).Err
		if err == nil {
			return nil
		}
	}
	x11ImeDebug("fcitx SetSurroundingText err fallback")
	return nil
}

func (e *fcitxEngine) SetContentType(conn *dbus.Conn, obj dbus.ObjectPath, purpose ContentPurpose) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
		o := conn.Object(svc, obj)
		err := o.CallWithContext(ctx, "SetContentType", 0, uint32(purpose)).Err
		if err == nil {
			return nil
		}
		err = o.CallWithContext(ctx, dbusIfaceFcitxIM+".SetContentType", 0, uint32(purpose)).Err
		if err == nil {
			return nil
		}
	}
	x11ImeDebug("fcitx SetContentType purpose=%d err fallback", purpose)
	return nil
}

func (e *fcitxEngine) ProcessKeyEvent(conn *dbus.Conn, obj dbus.ObjectPath, keysym, keycode, state, xTime uint32, isPress bool) (bool, error) {
	if conn == nil || obj == "" {
		return false, fmt.Errorf("nil conn")
	}
	t := xTime
	if t == 0 {
		t = uint32(time.Now().UnixNano() / 1e6 & 0xffffffff)
	}
	isRelease := !isPress
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceFcitx5, obj)
	var handled bool
	err := o.CallWithContext(ctx, dbusIfaceFcitxIM+".ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state), t, isRelease).Store(&handled)
	if err != nil {
		err = o.CallWithContext(ctx, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state), t, isRelease).Store(&handled)
	}
	if err != nil {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel2()
		err = o.CallWithContext(ctx2, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
	}
	x11ImeDebug("fcitx ProcessKeyEvent uuuub (%#x,%d,%d,%d,%v)->%v err=%v", keysym, keycode, state, t, isRelease, handled, err)
	return handled, err
}
