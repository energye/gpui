//go:build linux

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// 第2层 引擎层 ibus：私有总线 + 生命周期
// 私有地址来自 ~/.config/ibus/bus/* 的 IBUS_ADDRESS，验 PID、跳 fcitx 污染，dialIbusPrivate 单例复用

var (
	x11IbusMu   sync.Mutex
	x11IbusConn *dbus.Conn
	x11IbusErr  error
	x11IbusOnce sync.Once
)

// x11ResolveIbusAddress 读 ~/.config/ibus/bus/* → IBUS_ADDRESS
// 规则：遍历文件，解析 IBUS_ADDRESS 与 IBUS_DAEMON_PID，跳过含 fcitx 的污染条目，验 PID 存活，取最新 mtime 的有效条目
func x11ResolveIbusAddress() (string, error) {
	dir := x11IbusBusDir()
	return x11ResolveIbusAddressFromDir(dir)
}

func x11IbusBusDir() string {
	if v := os.Getenv("IBUS_BUS_DIR"); v != "" {
		return v
	}
	home := os.Getenv("HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".config/ibus/bus")
}

func x11ResolveIbusAddressFromDir(dir string) (string, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil || len(entries) == 0 {
		return "", os.ErrNotExist
	}
	type cand struct {
		addr  string
		pid   string
		mtime int64
		path  string
	}
	var cands []cand
	for _, p := range entries {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			continue
		}
		addr, pid, hasAddr := x11ParseIbusBusFile(p)
		if !hasAddr || addr == "" {
			continue
		}
		// 跳污染：地址或文件内容含 fcitx
		if strings.Contains(strings.ToLower(addr), "fcitx") {
			x11ImeDebug("ibus bus skip polluted addr %q file %s", addr, p)
			continue
		}
		// 也检查文件原始内容是否含 fcitx_random_string
		if x11FileContainsFcitx(p) {
			x11ImeDebug("ibus bus skip fcitx file %s", p)
			continue
		}
		// 验 PID 存活
		if pid != "" && !x11PidAlive(pid) {
			x11ImeDebug("ibus bus skip dead pid %s file %s addr %q", pid, p, addr)
			continue
		}
		cands = append(cands, cand{addr: addr, pid: pid, mtime: fi.ModTime().UnixNano(), path: p})
	}
	if len(cands) == 0 {
		return "", os.ErrNotExist
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mtime > cands[j].mtime })
	best := cands[0]
	x11ImeDebug("ibus bus resolved %q pid %s file %s", best.addr, best.pid, best.path)
	return best.addr, nil
}

func x11ParseIbusBusFile(path string) (addr, pid string, hasAddr bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "IBUS_ADDRESS=") {
			v := strings.TrimPrefix(line, "IBUS_ADDRESS=")
			v = strings.Trim(v, "\"' ")
			addr = v
			hasAddr = true
		} else if strings.HasPrefix(line, "IBUS_DAEMON_PID=") {
			v := strings.TrimPrefix(line, "IBUS_DAEMON_PID=")
			v = strings.Trim(v, "\"' ")
			pid = v
		}
	}
	return addr, pid, hasAddr
}

func x11FileContainsFcitx(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := strings.ToLower(string(b))
	return strings.Contains(s, "fcitx")
}

func x11PidAlive(pidStr string) bool {
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil || pid <= 0 {
		return false
	}
	// 先用 kill 0 验存在
	if err := syscall.Kill(pid, 0); err == nil {
		return true
	}
	// 回退 /proc 判存在（kill 在容器内可能 EPERM）
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
		return true
	}
	return false
}

// dialIbusPrivate 私有总线拨号，单例复用，多窗口连接数恒为 1
func dialIbusPrivate() (*dbus.Conn, error) {
	x11IbusOnce.Do(func() {
		addr, err := x11ResolveIbusAddress()
		if err != nil {
			x11ImeDebug("ibus private no addr: %v", err)
			x11IbusErr = err
			return
		}
		x11ImeDebug("ibus private dial addr=%q", addr)
		c, err := dialWithTimeout(addr, 800)
		if err != nil {
			x11ImeDebug("ibus private dial failed: %v", err)
			x11IbusErr = err
			return
		}
		x11ImeDebug("ibus private dial ok addr=%q", addr)
		// 私有/回退总线订阅 NameOwnerChanged（与会话总线一致）及 ibus 信号
		for _, rule := range dbusMatchRules {
			ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
			err := c.BusObject().CallWithContext(ctx2, dbusServiceDBus+".AddMatch", 0, rule).Err
			cancel2()
			if err != nil {
				x11ImeDebug("ibus AddMatch %q note: %v", rule, err)
			} else {
				x11ImeDebug("ibus AddMatch %q ok", rule)
			}
		}
		x11IbusMu.Lock()
		x11IbusConn = c
		x11IbusMu.Unlock()
		go x11GlobalNameOwnerLoop(c)
		go x11BusWatchLoop(c)
	})
	x11IbusMu.Lock()
	defer x11IbusMu.Unlock()
	return x11IbusConn, x11IbusErr
}

func dialWithTimeout(addr string, ms int) (*dbus.Conn, error) {
	type res struct {
		c   *dbus.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := dbus.Connect(addr)
		ch <- res{c, err}
	}()
	select {
	case r := <-ch:
		return r.c, r.err
	case <-time.After(time.Duration(ms) * time.Millisecond):
		go func() {
			r := <-ch
			if r.c != nil {
				_ = r.c.Close()
			}
		}()
		return nil, fmt.Errorf("dial timeout %dms", ms)
	}
}

func resetIbusPrivateForTest() {
	x11IbusMu.Lock()
	if x11IbusConn != nil {
		_ = x11IbusConn.Close()
	}
	x11IbusConn = nil
	x11IbusErr = nil
	x11IbusMu.Unlock()
	x11IbusOnce = sync.Once{}
}
