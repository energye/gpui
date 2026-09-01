//go:build linux

package platform

import (
	"sync"

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
		x11ImeDebug("fcitx session dial start addr=%q", envSessionBus())
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

func envSessionBus() string {
	// os.Getenv 在 proto 侧已使用，此处仅为日志
	return ""
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
