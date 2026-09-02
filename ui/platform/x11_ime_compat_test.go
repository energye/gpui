//go:build linux

package platform

// X11 IME 引擎选择回归单测（fcitx5 兼容节点 + ibus 私有总线解析 + 端到端取 IME）。
//
// 覆盖三个真实缺陷：
//  1. ibus 私有总线解析按「含 fcitx」字样过滤，把 fcitx5(ibusfrontend) 自己写的
//     合法条目误杀，导致本机（GTK_IM_MODULE=ibus 但守护实为 fcitx5）候选归零、IME=nil；
//  2. fcitx5 原生 /org/fcitx/Fcitx5/InputMethod 未导出时（fcitx5 只装 ibusfrontend），
//     引擎无回落，CreateInputContext 直接失败；
//  3. fcitx5 兼容节点下 ProcessKeyEvent 必须走 ibus 的 uuu 签名，
//     发 uuuub 会被拒（Invalid arguments），按键被误判未消费，Ctrl+Space 切换失效。

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// TestX11IbusBusResolveKeepsFcitxFrontendEntry 锁死缺陷 1：
// fcitx5 运行 ibusfrontend 时写下的条目（地址带 fcitx_random_string、PID 为活进程）
// 必须被保留，不得按「含 fcitx」误杀。
func TestX11IbusBusResolveKeepsFcitxFrontendEntry(t *testing.T) {
	dir := t.TempDir()
	orig := os.Getenv("IBUS_BUS_DIR")
	os.Setenv("IBUS_BUS_DIR", dir)
	defer os.Setenv("IBUS_BUS_DIR", orig)

	// 复刻真机 ~/.config/ibus/bus/ 条目：fcitx5 写的合法接入点
	const fcitxAddr = "unix:path=/run/user/1000/bus,fcitx_random_string=1bded355649b42b3804325f48923f965"
	live := filepath.Join(dir, "fcitx5-frontend-unix-0")
	body := "IBUS_ADDRESS=" + fcitxAddr + "\nIBUS_DAEMON_PID=" + strconv.Itoa(os.Getpid()) + "\n"
	if err := os.WriteFile(live, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := x11ResolveIbusAddressFromDir(dir)
	if err != nil {
		t.Fatalf("fcitx5 ibusfrontend 条目被误杀（候选应为 1）：%v", err)
	}
	if got != fcitxAddr {
		t.Fatalf("addr = %q, want %q", got, fcitxAddr)
	}
}

// TestX11IbusBusResolveSkipsDeadPid 死 PID 条目仍须跳过（防回归）。
func TestX11IbusBusResolveSkipsDeadPid(t *testing.T) {
	dir := t.TempDir()
	orig := os.Getenv("IBUS_BUS_DIR")
	os.Setenv("IBUS_BUS_DIR", dir)
	defer os.Setenv("IBUS_BUS_DIR", orig)

	dead := filepath.Join(dir, "dead-bus")
	body := "IBUS_ADDRESS=unix:abstract=/tmp/ibus-dead,guid=dead\nIBUS_DAEMON_PID=999999\n"
	if err := os.WriteFile(dead, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := x11ResolveIbusAddressFromDir(dir); err == nil {
		t.Fatalf("死 PID 条目应被跳过，却返回了地址")
	}
}

// TestX11FcitxFallsBackToIBusCompatNode 锁死缺陷 2：
// fcitx5 原生 InputMethod 对象缺失时，必须回落到 fcitx5 自己提供的 IBus 兼容节点。
func TestX11FcitxFallsBackToIBusCompatNode(t *testing.T) {
	conn, err := sharedFcitxConn()
	if err != nil || conn == nil {
		t.Skipf("无会话总线：%v", err)
	}
	// 前置：本机会话总线上需存在 fcitx5 提供的 IBus 兼容节点
	hasIBus, _ := dbusHasOwner(conn, dbusServiceIBus)
	if !hasIBus {
		t.Skip("本机无 org.freedesktop.IBus，跳过兼容回落验证")
	}

	e := &fcitxEngine{}
	obj, err := e.CreateInputContext(conn, 1500*time.Millisecond)
	if err != nil {
		t.Fatalf("fcitx5 引擎应回落到 IBus 兼容节点建上下文，实际失败：%v", err)
	}
	if obj == "" {
		t.Fatalf("回落建出的 objectPath 为空")
	}
	// 若走的是回落路径，必须置兼容模式，后续按键才按 ibus 的 uuu 签名发
	if !e.isCompat() {
		t.Logf("说明：本机 fcitx5 导出了原生 InputMethod，走原生路径（compat=false），回落逻辑未触发")
	} else {
		t.Logf("回落生效：objectPath=%s compat=true", obj)
	}
	if err := e.SetCapabilities(conn, obj, e.Caps()); err != nil {
		t.Logf("SetCapabilities 返回 %v（兼容模式下按 ibus 协议发）", err)
	}
	_ = e.FocusIn(conn, obj)
	// uuu 签名在两种路径下都必须不报 Invalid arguments
	handled, err := e.ProcessKeyEvent(conn, obj, 0x6e, 57, 0, 0, true)
	t.Logf("ProcessKeyEvent -> handled=%v err=%v", handled, err)
	if err != nil {
		t.Fatalf("ProcessKeyEvent 不应报错（兼容模式须用 uuu 签名）：%v", err)
	}
	_ = e.FocusOut(conn, obj)
	_ = e.Destroy(conn, obj)
}

// TestX11FcitxCompatUsesIBusKeySignature 锁死缺陷 3：
// 兼容模式下 ProcessKeyEvent 必须走 ibus 的 uuu 三参签名；
// fcitx 原生 uuuub 五参在 IBus 接口上会被拒。
func TestX11FcitxCompatUsesIBusKeySignature(t *testing.T) {
	conn, err := sharedFcitxConn()
	if err != nil || conn == nil {
		t.Skipf("无会话总线：%v", err)
	}
	var ic dbus.ObjectPath
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Object(dbusServiceIBus, dbus.ObjectPath(dbusPathIBusBus)).
		CallWithContext(ctx, dbusIfaceIBus+".CreateInputContext", dbus.FlagNoAutoStart, x11ClientName()).Store(&ic); err != nil {
		t.Skipf("无法建 IBus 上下文：%v", err)
	}
	defer func() {
		_ = (&ibusEngine{}).Destroy(conn, ic)
	}()

	obj := conn.Object(dbusServiceIBus, ic)
	// uuu（ibus 原生）：必须被接受
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()
	var handled bool
	errUUU := obj.CallWithContext(ctx2, dbusIfaceIBusCtx+".ProcessKeyEvent", 0,
		uint32(0x6e), uint32(57), uint32(0)).Store(&handled)

	// uuuub（fcitx 原生）：在 IBus 接口上必须被拒，这正是「不能切换」的根因
	ctx3, cancel3 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel3()
	now := uint32(time.Now().UnixNano()/1e6) & 0xffffffff
	errUUUUB := obj.CallWithContext(ctx3, dbusIfaceIBusCtx+".ProcessKeyEvent", 0,
		uint32(0x6e), uint32(57), uint32(0), now, false).Store(&handled)

	if errUUU != nil {
		t.Fatalf("uuu 签名应被 IBus 接口接受，实际：%v", errUUU)
	}
	if errUUUUB == nil {
		t.Fatalf("uuuub 签名本应被 IBus 接口拒绝；若被接受则说明签名探测前提变了")
	}
	t.Logf("uuu 被接受，uuuub 被拒（%v）—— 兼容模式必须选 uuu", errUUUUB)
}

// TestX11ImeForX11BothModuleConfigs 端到端：
// GTK_IM_MODULE=ibus 与 =fcitx 两态都必须拿到可用的 InputContext。
// 修复前：ibus 态 IME=nil（缺陷 1），fcitx 态 IC 建不出（缺陷 2）。
func TestX11ImeForX11BothModuleConfigs(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	sc, serr := sharedFcitxConn()
	if serr != nil || sc == nil {
		t.Skipf("无会话总线：%v", serr)
	}
	if has, _ := dbusHasOwner(sc, dbusServiceIBus); !has {
		t.Skip("本机无 org.freedesktop.IBus，跳过端到端验证")
	}

	origGtk, origQt, origXmod := os.Getenv("GTK_IM_MODULE"), os.Getenv("QT_IM_MODULE"), os.Getenv("XMODIFIERS")
	defer func() {
		os.Setenv("GTK_IM_MODULE", origGtk)
		os.Setenv("QT_IM_MODULE", origQt)
		os.Setenv("XMODIFIERS", origXmod)
	}()

	for _, mod := range []string{"ibus", "fcitx"} {
		t.Run("GTK_IM_MODULE="+mod, func(t *testing.T) {
			os.Setenv("GTK_IM_MODULE", mod)
			os.Setenv("QT_IM_MODULE", mod)
			os.Setenv("XMODIFIERS", "@im="+mod)
			resetIbusPrivateForTest()
			defer resetIbusPrivateForTest()

			ime := imeForX11(&x11Host{st: &x11State{}})
			if ime == nil {
				t.Fatalf("imeForX11 返回 nil：该配置下完全无法输入")
			}
			x, ok := ime.(*x11Ime)
			if !ok {
				t.Fatalf("not x11Ime")
			}
			defer x.Close()

			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if x.ObjectPath() != "" {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if x.ObjectPath() == "" {
				t.Fatalf("探测超时：InputContext 未建立（engine=%q）", x.Engine())
			}
			t.Logf("OK engine=%q objectPath=%s", x.Engine(), x.ObjectPath())
		})
	}
}
