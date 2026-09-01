//go:build linux

package platform

import (
	"fmt"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
)

// 第1层 协议层（Q1-A 共用）：ibus 标准协议常量、单选探针、FlagNoAutoStart 封装
// fcitx5 通过 /org/freedesktop/IBus 兼容节点复用同一套 ibus 协议，故本层两引擎共用。

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

	ibusCaps  uint32 = 1<<0 | 1<<3 | 1<<5 // 41: PREEDIT|FOCUS|SURROUNDING
	fcitxCaps uint32 = 16

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

func x11ImeDebug(format string, args ...any) {
	if os.Getenv("GPUI_IME_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "[ime-x11] "+format+"\n", args...)
}

// x11ProbeOrder Q2 单选：只探用户配置的一家，绝不 fallback 双探
// 规则：GTK_IM_MODULE > QT_IM_MODULE > XMODIFIERS，含 ibus→只探 ibus，含 fcitx→只探 fcitx5，空值单探 ibus
func x11ProbeOrder() []string {
	gtk := strings.ToLower(os.Getenv("GTK_IM_MODULE"))
	qt := strings.ToLower(os.Getenv("QT_IM_MODULE"))
	xmod := strings.ToLower(os.Getenv("XMODIFIERS"))
	check := func(v, src string) (bool, []string) {
		if strings.Contains(v, "ibus") {
			x11ImeDebug("probe order: ibus (%s=%q) single", src, v)
			return true, []string{"ibus"}
		}
		if strings.Contains(v, "fcitx") {
			x11ImeDebug("probe order: fcitx5 (%s=%q) single", src, v)
			return true, []string{"fcitx5"}
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
	x11ImeDebug("probe order: ibus (default GTK=%q QT=%q XMOD=%q) single", gtk, qt, xmod)
	return []string{"ibus"}
}

// x11ProbeOrderStrict 供单测严格校验单选
func x11ProbeOrderStrict() []string { return x11ProbeOrder() }

func isSyntheticPath(p dbus.ObjectPath) bool {
	return strings.HasPrefix(string(p), syntheticFcitxPrefix)
}

// dbusFlagNoAutoStart 阻断对未拥有名字的 StartServiceByName 激活
const dbusFlagNoAutoStart = dbus.FlagNoAutoStart
