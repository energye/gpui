//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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

// dbusGetNameOwner 返回名字的「唯一归属名」（如 ":1.2066"），无主时返回 ""。
//
// 这是「输入法框架热切」的身份真源：环境变量（GTK/QT/XMOD）说的是用户「声明」要哪家，
// 而唯一归属名说的是总线上「实际」是谁在干活。两者分属不同事实，切换时必须比对后者——
// fcitx5 跑 ibusfrontend 时自己会占 org.freedesktop.IBus，此时声明=ibus、实际=fcitx5，
// 单看声明无法判断连接是否仍对得上同一个服务方。
func dbusGetNameOwner(conn *dbus.Conn, name string) string {
	if conn == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	var owner string
	if err := conn.BusObject().CallWithContext(ctx, dbusServiceDBus+".GetNameOwner", dbus.FlagNoAutoStart, name).Store(&owner); err != nil {
		return ""
	}
	return owner
}

// x11ImeOwner 汇总当前总线上各输入法名字的归属，供身份比对。
type x11ImeOwner struct {
	IBus    string // org.freedesktop.IBus   的唯一归属名
	Fcitx5  string // org.fcitx.Fcitx5 的唯一归属名
	Fcitx   string // org.fcitx.Fcitx（老名）的唯一归属名
	IBusPID uint32 // 占 IBus 名的进程 PID，用于日志溯源
}

// Any 报告总线上是否还有任一输入法框架活着。
func (o x11ImeOwner) Any() bool { return o.IBus != "" || o.Fcitx5 != "" || o.Fcitx != "" }

// snapshotOwner 对当前引擎所关心的名字做一次归属快照。
func snapshotOwner(conn *dbus.Conn) x11ImeOwner {
	var o x11ImeOwner
	if conn == nil {
		return o
	}
	o.IBus = dbusGetNameOwner(conn, dbusServiceIBus)
	o.Fcitx5 = dbusGetNameOwner(conn, dbusServiceFcitx5)
	o.Fcitx = dbusGetNameOwner(conn, dbusServiceFcitx)
	if o.IBus != "" {
		var pid uint32
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if err := conn.BusObject().CallWithContext(ctx, dbusServiceDBus+".GetConnectionUnixProcessID", 0, o.IBus).Store(&pid); err == nil {
			o.IBusPID = pid
		}
	}
	return o
}

// x11EffectiveOwner 取得「当前总线上谁活着」的可靠快照，并给出可用于探测的连接。
//
// 为什么不能只看传入的连接：程序可能正握着一条**已经死了的私有总线连接**
// （真 ibus-daemon 一退，它那条 unix:abstract 私有总线随之消失）。此时在这条
// 死连接上做归属查询，结果必然是「谁都没有」，于是程序看不见**新出现在会话
// 总线上的 fcitx5**——即「从 ibus 切到 fcitx5 用不了」的根因。
//
// 策略：优先用当前连接；当它不可用或查不到任何人时，补查会话总线并合并结果
// （名字归属在会话总线上可见：真 ibus-daemon 也在会话总线占名，只是服务对象
// 挂在私有总线上）。返回合并后的快照，以及建议拿去探测的连接。
func x11EffectiveOwner(curConn *dbus.Conn) (x11ImeOwner, *dbus.Conn) {
	merged := snapshotOwner(curConn)
	best := curConn
	if merged.Any() && curConn != nil && curConn.Connected() {
		return merged, best
	}
	// 当前连接不可用或查不到人 → 补查会话总线（fcitx5 只出现在那里）
	if sess, err := sharedFcitxConn(); err == nil && sess != nil && sess != curConn {
		alt := snapshotOwner(sess)
		if alt.Any() {
			merged, best = alt, sess
			x11ImeDebug("effective owner: current bus unusable, session bus has IBus=%s Fcitx5=%s", alt.IBus, alt.Fcitx5)
		}
	}
	return merged, best
}

// x11ProbeOrderForOwner 在 Q2 单选基础上，按「实测谁活着」校正目标框架。
//
// 为什么需要它：用户在运行时切换输入法框架（如停 fcitx5、起 ibus）**不会**改写
// 本进程已读到的 GTK/QT/XMOD 环境变量。若死守声明去单选，切换后会一直去探一个
// 已经没了名字的框架，永远探不到新框架——即「程序感知不到用户切了输入法」。
//
// 纪律：仍然**只返回一个目标**（不双探、不 fallback 到另一家再兜底），
// 只是在「声明的那家已无主、而实测有别家活着」时，把目标换成实测活着的这家。
// 声明的那家还活着时严格按声明走，不改用户配置意图。
func x11ProbeOrderForOwner(conn *dbus.Conn) []string {
	return x11ProbeOrderForOwnerFrom(x11ProbeOrder(), snapshotOwner(conn))
}

// snapshotOwnerOnSessionBus 在会话总线上做一次归属快照，失败返回空快照。
//
// 建窗时的目标校必须用会话总线：候选总线（私有地址文件）此刻可能解析不出来
// （本机 fcitx5 跑 ibusfrontend 时会把 ~/.config/ibus/bus/ 写成空值），
// 而真 ibus 也会在会话总线上占名，所以在会话总线上查才查得全。
func snapshotOwnerOnSessionBus() x11ImeOwner {
	sess, err := sharedFcitxConn()
	if err != nil || sess == nil {
		return x11ImeOwner{}
	}
	return snapshotOwner(sess)
}

// frameworkIdentity 归一出「当前是哪几家在服务、各自是谁」的指纹，用于跨框架比对。
//
// 为什么需要它：只按「同一个名字的 owner 是否变了」比对（x11OwnerChanged）**漏掉了
// 换框架**这一档——fcitx5 原生只占 org.fcitx.Fcitx5，真 ibus 只占
// org.freedesktop.IBus，两者互不重叠，任何一个名字的 owner 都没有「从 A 变 B」，
// 于是被判成「没变」，程序继续抱着旧框架不放。
// 指纹把「哪几个名字有人 + 分别是谁」整体纳入，换框架就一定能比对出来。
func (o x11ImeOwner) frameworkIdentity() string {
	return "IBus=" + o.IBus + ";Fcitx5=" + o.Fcitx5 + ";Fcitx=" + o.Fcitx
}

// x11OwnerChanged 判定归属快照是否发生了「换了服务方」级别的变化。
func x11ProbeOrderForOwnerFrom(declared []string, cur x11ImeOwner) []string {
	if len(declared) == 0 {
		return declared
	}
	name := declared[0]
	alive := false
	switch name {
	case "ibus":
		alive = cur.IBus != ""
	case "fcitx5":
		alive = cur.Fcitx5 != "" || cur.Fcitx != ""
	}
	if alive {
		return declared
	}
	// 声明的那家没了：转向实测活着的框架，否则切换永远感知不到。
	switch {
	case cur.IBus != "":
		x11ImeDebug("probe order corrected: declared=%s gone, ibus alive (owner %s)", name, cur.IBus)
		return []string{"ibus"}
	case cur.Fcitx5 != "" || cur.Fcitx != "":
		x11ImeDebug("probe order corrected: declared=%s gone, fcitx5 alive (owner %s)", name, cur.Fcitx5)
		return []string{"fcitx5"}
	}
	return declared
}

// x11OwnerChanged 判定归属快照是否发生了「换了服务方」级别的变化。
//
// 语义分三档，对应三种现场：
//   - 守护消失（Any: 有→无）：服务方没了，IC 仍保留（B22，防重建导致 fcitx5 回落英文）。
//   - 守护上线（Any: 无→有）：服务方回来了，下次输入重探接管。
//   - 过户（某个具体名字的 owner 从 A 变成 B，且 A、B 都非空）：**换了框架**，
//     旧 IC 挂在一个已经不存在的服务方上，必须重建。
//
// 第三档是本函数存在的理由：只看 Any 会把「fcitx5 让位给 ibus」误判成「还在」，
// 从而复用一个死掉的 IC，表现为切换后新输入法接不上。
func x11OwnerChanged(before, after x11ImeOwner) bool {
	if before.IBus != "" && after.IBus != "" && before.IBus != after.IBus {
		return true
	}
	if before.Fcitx5 != "" && after.Fcitx5 != "" && before.Fcitx5 != after.Fcitx5 {
		return true
	}
	if before.Fcitx != "" && after.Fcitx != "" && before.Fcitx != after.Fcitx {
		return true
	}
	return false
}

// dbusFlagNoAutoStart 阻断对未拥有名字的 StartServiceByName 激活
const dbusFlagNoAutoStart = dbus.FlagNoAutoStart

// x11ServiceForEngine 返回引擎在总线上占的名字（有多个时返回主名）。
func x11ServiceForEngine(engine string) string {
	switch engine {
	case "ibus":
		return dbusServiceIBus
	case "fcitx5":
		return dbusServiceFcitx5
	}
	return ""
}

// x11ImeEngine 第2层引擎抽象：ibus 私有总线与 fcitx5 会话总线各自实现，统一层只调度
type x11ImeEngine interface {
	Name() string
	Caps() uint32
	// Bus 返回该引擎当前应使用的总线连接。
	//
	// force=true 表示已确认「换了输入法框架」，须作废缓存重新取连接：
	// ibus 守护换人后它监听的**总线地址本身会变**（实测 fcitx5 把 IBus 服务
	// 开在会话总线上，真 ibus-daemon 则另开 unix:abstract 私有总线，并把新
	// 地址写进 ~/.config/ibus/bus/）。沿用启动时缓存的那条连接，输入上下文
	// 永远建不出来，只能重启进程——这正是「切完输入法必须重启」的根因。
	Bus(force bool) (*dbus.Conn, error)
	CreateInputContext(conn *dbus.Conn, timeout time.Duration) (dbus.ObjectPath, error)
	SetCapabilities(conn *dbus.Conn, obj dbus.ObjectPath, caps uint32) error
	Destroy(conn *dbus.Conn, obj dbus.ObjectPath) error
	FocusIn(conn *dbus.Conn, obj dbus.ObjectPath) error
	FocusOut(conn *dbus.Conn, obj dbus.ObjectPath) error
	SetCursorLocation(conn *dbus.Conn, obj dbus.ObjectPath, x, y, w, h int) error
	SetSurroundingText(conn *dbus.Conn, obj dbus.ObjectPath, text string, cursor, anchor int) error
	SetContentType(conn *dbus.Conn, obj dbus.ObjectPath, purpose ContentPurpose) error
	ProcessKeyEvent(conn *dbus.Conn, obj dbus.ObjectPath, keysym, keycode, state, xTime uint32, isPress bool) (bool, error)
}

type ibusEngine struct{}

var x11Engines = map[string]x11ImeEngine{
	"ibus":   &ibusEngine{},
	"fcitx5": &fcitxEngine{},
}

func x11EngineForName(name string) x11ImeEngine {
	if e, ok := x11Engines[name]; ok {
		return e
	}
	return nil
}
